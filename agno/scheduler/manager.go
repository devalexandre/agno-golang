package scheduler

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// ValidationError reports invalid schedule input (HTTP 422).
type ValidationError struct{ Msg string }

func (e *ValidationError) Error() string { return e.Msg }

func invalid(format string, args ...any) error {
	return &ValidationError{Msg: fmt.Sprintf(format, args...)}
}

// CreateRequest describes a new schedule. Exactly one of CronExpr,
// IntervalSeconds and RunAt may be set; none makes it on demand. Endpoint may
// be omitted when TargetType/TargetID name a component.
type CreateRequest struct {
	Name              string         `json:"name"`
	CronExpr          string         `json:"cron_expr"`
	IntervalSeconds   int64          `json:"interval_seconds"`
	RunAt             *int64         `json:"run_at"`
	Endpoint          string         `json:"endpoint"`
	Method            string         `json:"method"`
	Description       *string        `json:"description"`
	Payload           map[string]any `json:"payload"`
	Timezone          string         `json:"timezone"`
	TimeoutSeconds    *int           `json:"timeout_seconds"`
	MaxRetries        *int           `json:"max_retries"`
	RetryDelaySeconds *int           `json:"retry_delay_seconds"`
	TargetType        string         `json:"target_type"`
	TargetID          string         `json:"target_id"`
	Enabled           *bool          `json:"enabled"`
	UserID            *string        `json:"-"`
}

// UpdateRequest patches a schedule; nil fields are left unchanged.
type UpdateRequest struct {
	Name              *string         `json:"name"`
	CronExpr          *string         `json:"cron_expr"`
	IntervalSeconds   *int64          `json:"interval_seconds"`
	RunAt             *int64          `json:"run_at"`
	Endpoint          *string         `json:"endpoint"`
	Method            *string         `json:"method"`
	Description       *string         `json:"description"`
	Payload           *map[string]any `json:"payload"`
	Timezone          *string         `json:"timezone"`
	TimeoutSeconds    *int            `json:"timeout_seconds"`
	MaxRetries        *int            `json:"max_retries"`
	RetryDelaySeconds *int            `json:"retry_delay_seconds"`
}

// Manager is the CRUD API for schedules, with validation and next-run
// computation. It mirrors Python's ScheduleManager.
type Manager struct {
	Store Store
}

// NewManager returns a Manager over store.
func NewManager(store Store) *Manager { return &Manager{Store: store} }

func validateTiming(s *Schedule) error {
	set := 0
	if !IsOnDemand(s.CronExpr) {
		set++
		if len(s.CronExpr) > 128 || !ValidateCronExpr(s.CronExpr) {
			return invalid("Invalid cron expression: %s", s.CronExpr)
		}
	}
	if s.IntervalSeconds != 0 {
		set++
		if s.IntervalSeconds < 1 {
			return invalid("interval_seconds must be positive")
		}
	}
	if s.RunAt != nil {
		set++
	}
	if set > 1 {
		return invalid("only one of cron_expr, interval_seconds and run_at may be set")
	}
	if !ValidateTimezone(s.Timezone) {
		return invalid("Invalid timezone: %s", s.Timezone)
	}
	return nil
}

func validateLimits(s *Schedule) error {
	if s.TimeoutSeconds < 1 || s.TimeoutSeconds > 86400 {
		return invalid("timeout_seconds must be between 1 and 86400")
	}
	if s.MaxRetries < 0 || s.MaxRetries > 10 {
		return invalid("max_retries must be between 0 and 10")
	}
	if s.RetryDelaySeconds < 1 || s.RetryDelaySeconds > 3600 {
		return invalid("retry_delay_seconds must be between 1 and 3600")
	}
	if s.Description != nil && len(*s.Description) > 1024 {
		return invalid("description too long")
	}
	return nil
}

func applyTargets(s *Schedule) {
	if tt, tid, ok := ParseRunEndpoint(s.Endpoint); ok {
		s.TargetType, s.TargetID = ptr(tt), ptr(tid)
	} else {
		s.TargetType, s.TargetID = nil, nil
	}
}

// Create validates and stores a new schedule.
func (m *Manager) Create(ctx context.Context, req CreateRequest) (*Schedule, error) {
	if err := ValidateName(req.Name); err != nil {
		return nil, invalid("%s", err.Error())
	}
	endpoint := req.Endpoint
	if endpoint == "" && req.TargetType != "" && req.TargetID != "" {
		switch req.TargetType {
		case "agent", "team", "workflow":
			endpoint = BuildRunEndpoint(req.TargetType, req.TargetID)
		default:
			return nil, invalid("target_type must be agent, team or workflow")
		}
	}
	if err := ValidateEndpoint(endpoint); err != nil {
		return nil, invalid("%s", err.Error())
	}
	method, err := NormalizeMethod(req.Method)
	if err != nil {
		return nil, invalid("%s", err.Error())
	}
	now := time.Now().Unix()
	s := &Schedule{
		ID: NewID(), UserID: req.UserID, Name: req.Name, Description: req.Description, Method: method,
		Endpoint: endpoint, Payload: req.Payload, CronExpr: req.CronExpr, IntervalSeconds: req.IntervalSeconds,
		RunAt: req.RunAt, Timezone: req.Timezone, TimeoutSeconds: 3600, RetryDelaySeconds: 60, Enabled: true,
		CreatedAt: now, UpdatedAt: ptr(now),
	}
	if s.Timezone == "" {
		s.Timezone = "UTC"
	}
	if req.TimeoutSeconds != nil {
		s.TimeoutSeconds = *req.TimeoutSeconds
	}
	if req.MaxRetries != nil {
		s.MaxRetries = *req.MaxRetries
	}
	if req.RetryDelaySeconds != nil {
		s.RetryDelaySeconds = *req.RetryDelaySeconds
	}
	if req.Enabled != nil {
		s.Enabled = *req.Enabled
	}
	if err := validateTiming(s); err != nil {
		return nil, err
	}
	if err := validateLimits(s); err != nil {
		return nil, err
	}
	applyTargets(s)
	if s.Enabled {
		if s.NextRunAt, err = ComputeNextRun(s, nil, false); err != nil {
			return nil, invalid("%s", err.Error())
		}
	}
	if err := m.Store.CreateSchedule(ctx, s); err != nil {
		return nil, err
	}
	return s, nil
}

// Get returns a schedule; userID scopes when non-nil.
func (m *Manager) Get(ctx context.Context, id string, userID *string) (*Schedule, error) {
	return m.Store.GetSchedule(ctx, id, userID)
}

// List lists schedules.
func (m *Manager) List(ctx context.Context, f ListFilter) ([]*Schedule, int, error) {
	return m.Store.ListSchedules(ctx, f)
}

// Update patches a schedule, recomputing next_run_at when timing changes.
func (m *Manager) Update(ctx context.Context, id string, userID *string, req UpdateRequest) (*Schedule, error) {
	s, err := m.Store.GetSchedule(ctx, id, userID)
	if err != nil {
		return nil, err
	}
	timing := false
	if req.Name != nil {
		if err := ValidateName(*req.Name); err != nil {
			return nil, invalid("%s", err.Error())
		}
		s.Name = *req.Name
	}
	if req.Endpoint != nil {
		if err := ValidateEndpoint(*req.Endpoint); err != nil {
			return nil, invalid("%s", err.Error())
		}
		s.Endpoint = *req.Endpoint
		applyTargets(s)
	}
	if req.Method != nil {
		if s.Method, err = NormalizeMethod(*req.Method); err != nil {
			return nil, invalid("%s", err.Error())
		}
	}
	if req.Description != nil {
		s.Description = req.Description
	}
	if req.Payload != nil {
		s.Payload = *req.Payload
	}
	// Setting one timing field replaces the others.
	if req.CronExpr != nil {
		s.CronExpr, s.IntervalSeconds, s.RunAt, timing = *req.CronExpr, 0, nil, true
	}
	if req.IntervalSeconds != nil {
		if req.CronExpr == nil {
			s.CronExpr = ""
		}
		s.IntervalSeconds, s.RunAt, timing = *req.IntervalSeconds, nil, true
	}
	if req.RunAt != nil {
		if req.CronExpr == nil {
			s.CronExpr = ""
		}
		if req.IntervalSeconds == nil {
			s.IntervalSeconds = 0
		}
		s.RunAt, timing = req.RunAt, true
	}
	if req.Timezone != nil {
		s.Timezone, timing = *req.Timezone, true
	}
	if req.TimeoutSeconds != nil {
		s.TimeoutSeconds = *req.TimeoutSeconds
	}
	if req.MaxRetries != nil {
		s.MaxRetries = *req.MaxRetries
	}
	if req.RetryDelaySeconds != nil {
		s.RetryDelaySeconds = *req.RetryDelaySeconds
	}
	if err := validateTiming(s); err != nil {
		return nil, err
	}
	if err := validateLimits(s); err != nil {
		return nil, err
	}
	if timing && s.Enabled {
		if s.NextRunAt, err = ComputeNextRun(s, nil, false); err != nil {
			return nil, invalid("%s", err.Error())
		}
	}
	s.UpdatedAt = ptr(time.Now().Unix())
	if err := m.Store.UpdateSchedule(ctx, s); err != nil {
		return nil, err
	}
	return s, nil
}

// Delete removes a schedule and its run history.
func (m *Manager) Delete(ctx context.Context, id string, userID *string) error {
	if _, err := m.Store.GetSchedule(ctx, id, userID); err != nil {
		return err
	}
	return m.Store.DeleteSchedule(ctx, id)
}

// Enable enables a schedule and recomputes its next run.
func (m *Manager) Enable(ctx context.Context, id string, userID *string) (*Schedule, error) {
	s, err := m.Store.GetSchedule(ctx, id, userID)
	if err != nil {
		return nil, err
	}
	s.Enabled, s.DisabledReason = true, nil
	if s.NextRunAt, err = ComputeNextRun(s, nil, false); err != nil {
		return nil, invalid("%s", err.Error())
	}
	s.UpdatedAt = ptr(time.Now().Unix())
	return s, m.Store.UpdateSchedule(ctx, s)
}

// Disable disables a schedule; it will not be claimed until enabled.
func (m *Manager) Disable(ctx context.Context, id string, userID *string) (*Schedule, error) {
	s, err := m.Store.GetSchedule(ctx, id, userID)
	if err != nil {
		return nil, err
	}
	s.Enabled, s.NextRunAt = false, nil
	s.UpdatedAt = ptr(time.Now().Unix())
	return s, m.Store.UpdateSchedule(ctx, s)
}

// GetRuns lists a schedule's run history, newest first.
func (m *Manager) GetRuns(ctx context.Context, scheduleID string, userID *string, limit, page int) ([]*ScheduleRun, int, error) {
	if _, err := m.Store.GetSchedule(ctx, scheduleID, userID); err != nil {
		return nil, 0, err
	}
	return m.Store.ListRuns(ctx, scheduleID, userID, limit, page)
}

// GetRun returns one run belonging to scheduleID.
func (m *Manager) GetRun(ctx context.Context, scheduleID, runID string, userID *string) (*ScheduleRun, error) {
	r, err := m.Store.GetRun(ctx, runID, userID)
	if err != nil {
		return nil, err
	}
	if r.ScheduleID != scheduleID {
		return nil, ErrNotFound
	}
	return r, nil
}

// IsValidation reports whether err is a ValidationError.
func IsValidation(err error) bool {
	var v *ValidationError
	return errors.As(err, &v)
}
