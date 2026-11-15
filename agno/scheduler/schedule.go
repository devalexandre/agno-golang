// Package scheduler ports Agno's scheduler: persisted schedules (cron,
// fixed interval, one-shot or on-demand) that fire an agent, team, workflow or
// arbitrary AgentOS endpoint, with retries, run history, enable/disable,
// cancellation and a multi-worker safe poller.
package scheduler

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode"
)

// Run statuses recorded on ScheduleRun.Status.
const (
	RunStatusRunning   = "running"
	RunStatusSuccess   = "success"
	RunStatusFailed    = "failed"
	RunStatusPaused    = "paused"
	RunStatusCancelled = "cancelled"
	RunStatusTimeout   = "timeout"
)

// Schedule kinds, derived from which timing field is set.
const (
	KindCron     = "cron"
	KindInterval = "interval"
	KindOnce     = "once"
	KindOnDemand = "on_demand"
)

var (
	// ErrNotFound is returned when a schedule or run does not exist.
	ErrNotFound = errors.New("scheduler: not found")
	// ErrDuplicateName is returned when a schedule name is already taken for the owner.
	ErrDuplicateName = errors.New("scheduler: schedule name already exists")
	// ErrDisabled is returned when triggering a disabled schedule.
	ErrDisabled = errors.New("scheduler: schedule is disabled")
)

// RunEndpointRE matches a component run endpoint and captures type and id.
var RunEndpointRE = regexp.MustCompile(`^/(agents|teams|workflows)/([^/]+)/runs/?$`)

var namePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9 ._-]*$`)

// Schedule is a persisted scheduled job.
type Schedule struct {
	ID                string         `json:"id"`
	UserID            *string        `json:"user_id"`
	Name              string         `json:"name"`
	Description       *string        `json:"description"`
	Method            string         `json:"method"`
	Endpoint          string         `json:"endpoint"`
	Payload           map[string]any `json:"payload"`
	CronExpr          string         `json:"cron_expr"`
	IntervalSeconds   int64          `json:"interval_seconds,omitempty"`
	RunAt             *int64         `json:"run_at,omitempty"`
	Timezone          string         `json:"timezone"`
	TimeoutSeconds    int            `json:"timeout_seconds"`
	MaxRetries        int            `json:"max_retries"`
	RetryDelaySeconds int            `json:"retry_delay_seconds"`
	Enabled           bool           `json:"enabled"`
	NextRunAt         *int64         `json:"next_run_at"`
	LockedBy          *string        `json:"-"`
	LockedAt          *int64         `json:"-"`
	TargetType        *string        `json:"target_type"`
	TargetID          *string        `json:"target_id"`
	DisabledReason    *string        `json:"disabled_reason"`
	CreatedAt         int64          `json:"created_at"`
	UpdatedAt         *int64         `json:"updated_at"`
}

// Kind reports how the schedule is timed.
func (s *Schedule) Kind() string {
	switch {
	case !IsOnDemand(s.CronExpr):
		return KindCron
	case s.IntervalSeconds > 0:
		return KindInterval
	case s.RunAt != nil:
		return KindOnce
	default:
		return KindOnDemand
	}
}

// Clone returns a deep-enough copy (payload map is shallow-copied).
func (s *Schedule) Clone() *Schedule {
	if s == nil {
		return nil
	}
	c := *s
	if s.Payload != nil {
		c.Payload = make(map[string]any, len(s.Payload))
		for k, v := range s.Payload {
			c.Payload[k] = v
		}
	}
	return &c
}

// ScheduleRun records a single execution attempt of a schedule.
type ScheduleRun struct {
	ID           string         `json:"id"`
	ScheduleID   string         `json:"schedule_id"`
	UserID       *string        `json:"user_id"`
	Attempt      int            `json:"attempt"`
	TriggeredAt  *int64         `json:"triggered_at"`
	CompletedAt  *int64         `json:"completed_at"`
	Status       string         `json:"status"`
	StatusCode   *int           `json:"status_code"`
	RunID        *string        `json:"run_id"`
	SessionID    *string        `json:"session_id"`
	Error        *string        `json:"error"`
	Input        map[string]any `json:"input"`
	Output       map[string]any `json:"output"`
	Requirements []any          `json:"requirements"`
	CreatedAt    int64          `json:"created_at"`
}

// Clone returns a copy of the run.
func (r *ScheduleRun) Clone() *ScheduleRun {
	if r == nil {
		return nil
	}
	c := *r
	return &c
}

// ComputeNextRun computes the next fire time of s after the given epoch
// (now when nil). It returns nil for on-demand schedules and for one-shot
// schedules that already fired (fired=true).
func ComputeNextRun(s *Schedule, after *int64, fired bool) (*int64, error) {
	now := time.Now().Unix()
	switch s.Kind() {
	case KindCron:
		n, err := ComputeNextCronRun(s.CronExpr, s.Timezone, after)
		if err != nil {
			return nil, err
		}
		return &n, nil
	case KindInterval:
		base := now
		if after != nil {
			base = *after
		}
		n := maxInt64(base+s.IntervalSeconds, now+1)
		return &n, nil
	case KindOnce:
		if fired {
			return nil, nil
		}
		n := *s.RunAt
		return &n, nil
	}
	return nil, nil
}

// ValidateName validates a schedule name.
func ValidateName(name string) error {
	if len(name) == 0 || len(name) > 255 || !namePattern.MatchString(name) {
		return fmt.Errorf("name must start with alphanumeric and contain only alphanumeric, spaces, '.', '_', '-'")
	}
	return nil
}

// NormalizeMethod upper-cases and validates an HTTP method.
func NormalizeMethod(m string) (string, error) {
	if m == "" {
		return "POST", nil
	}
	m = strings.ToUpper(m)
	switch m {
	case "GET", "POST", "PUT", "PATCH", "DELETE":
		return m, nil
	}
	return "", fmt.Errorf("method must be GET, POST, PUT, PATCH, or DELETE")
}

// ValidateEndpoint validates an endpoint path.
func ValidateEndpoint(e string) error {
	if !strings.HasPrefix(e, "/") {
		return fmt.Errorf("endpoint must start with '/'")
	}
	if strings.Contains(e, "://") {
		return fmt.Errorf("endpoint must be a path, not a full URL")
	}
	if len(e) > 512 {
		return fmt.Errorf("endpoint too long")
	}
	for _, r := range e {
		if unicode.IsSpace(r) || unicode.IsControl(r) {
			return fmt.Errorf("endpoint must not contain whitespace or control characters")
		}
	}
	return nil
}

// BuildRunEndpoint builds the canonical run endpoint for a component
// (targetType is "agent", "team" or "workflow").
func BuildRunEndpoint(targetType, targetID string) string {
	return "/" + targetType + "s/" + targetID + "/runs"
}

// ParseRunEndpoint extracts (targetType, targetID) from a run endpoint.
func ParseRunEndpoint(endpoint string) (string, string, bool) {
	m := RunEndpointRE.FindStringSubmatch(endpoint)
	if m == nil {
		return "", "", false
	}
	return strings.TrimSuffix(m[1], "s"), m[2], true
}

var (
	runIDMu   sync.Mutex
	lastRunNs int64
)

// NewScheduleRunID returns a time-ordered unique run id ("sr_<ns>_<rand>").
func NewScheduleRunID() string {
	runIDMu.Lock()
	ns := time.Now().UnixNano()
	if ns <= lastRunNs {
		ns = lastRunNs + 1
	}
	lastRunNs = ns
	runIDMu.Unlock()
	return fmt.Sprintf("sr_%020d_%s", ns, randomHex(16))
}

// NewID returns a random 32-char hex identifier.
func NewID() string { return randomHex(16) }

func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func ptr[T any](v T) *T { return &v }

func strOr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
