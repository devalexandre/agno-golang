// Package approval implements approval records for human-in-the-loop runs,
// mirroring Python's agno.approval and agno.run.approval.
//
// Two approval types exist:
//
//   - required: a blocking record. It is created with status "pending" when a
//     run pauses on a tool marked toolkit.WithApproval(toolkit.ApprovalRequired);
//     Agent.ContinueRun refuses to continue until the record is approved or
//     rejected (see Resolve), and then applies the decision to the paused call.
//   - audit: a non-blocking trail. A record with status "approved" or
//     "rejected" is written after the HITL interaction resolves.
//
// Records are kept in a Store, set with AgentConfig.ApprovalStore. When it is
// not set, the agent DB is used: every storage backend implements Store (SQL
// backends through sqlapproval, document backends through kvstore, and the
// in-memory DB). InMemoryStore is a standalone in-process Store. The
// approvaltest package holds the conformance suite for new implementations.
package approval

import (
	"context"
	"errors"
	"sort"
	"sync"
	"time"

	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// Type is the approval type.
type Type string

const (
	TypeRequired Type = toolkit.ApprovalRequired
	TypeAudit    Type = toolkit.ApprovalAudit
)

// Status is the state of an approval record.
type Status string

const (
	StatusPending  Status = "pending"
	StatusApproved Status = "approved"
	StatusRejected Status = "rejected"
)

// Approval is an approval record (same fields as Python's approval dict).
type Approval struct {
	ID             string                   `json:"id"`
	RunID          string                   `json:"run_id"`
	SessionID      string                   `json:"session_id"`
	Status         Status                   `json:"status"`
	ApprovalType   Type                     `json:"approval_type"`
	PauseType      models.PauseType         `json:"pause_type,omitempty"`
	ToolName       string                   `json:"tool_name,omitempty"`
	ToolArgs       map[string]interface{}   `json:"tool_args,omitempty"`
	SourceType     string                   `json:"source_type,omitempty"` // agent | team | workflow
	AgentID        string                   `json:"agent_id,omitempty"`
	TeamID         string                   `json:"team_id,omitempty"`
	WorkflowID     string                   `json:"workflow_id,omitempty"`
	UserID         string                   `json:"user_id,omitempty"`
	SourceName     string                   `json:"source_name,omitempty"`
	Requirements   []*models.RunRequirement `json:"requirements,omitempty"`
	Context        map[string]interface{}   `json:"context,omitempty"`
	ResolvedBy     string                   `json:"resolved_by,omitempty"`
	ResolvedAt     int64                    `json:"resolved_at,omitempty"`
	ResolutionData map[string]interface{}   `json:"resolution_data,omitempty"`
	RunStatus      models.RunStatus         `json:"run_status,omitempty"`
	CreatedAt      int64                    `json:"created_at"`
	UpdatedAt      int64                    `json:"updated_at,omitempty"`
}

// Filter selects approvals in ListApprovals. Zero fields match everything.
type Filter struct {
	RunID        string
	SessionID    string
	Status       Status
	ApprovalType Type
	Limit        int
}

// Resolution is the decision applied by UpdateApproval.
type Resolution struct {
	Status         Status
	ResolvedBy     string
	ResolvedAt     int64
	ResolutionData map[string]interface{}
}

// ErrNotFound is returned when an approval does not exist.
var ErrNotFound = errors.New("approval not found")

// ErrStatusConflict is returned when an approval is not in the expected status
// (for example it was already resolved).
var ErrStatusConflict = errors.New("approval status conflict")

// Store persists approval records.
type Store interface {
	CreateApproval(ctx context.Context, a *Approval) error
	GetApproval(ctx context.Context, id string) (*Approval, error)
	// ListApprovals returns matching approvals, most recent first.
	ListApprovals(ctx context.Context, f Filter) ([]*Approval, error)
	// UpdateApproval resolves an approval when its status equals expected
	// (ErrStatusConflict otherwise).
	UpdateApproval(ctx context.Context, id string, expected Status, res Resolution) (*Approval, error)
	// UpdateRunStatus stamps run_status on all approvals of a run and returns how many changed.
	UpdateRunStatus(ctx context.Context, runID string, status models.RunStatus) (int, error)
}

// Resolve approves or rejects a pending approval. resolutionData may carry
// "values" (user input), "result" (external execution) or "note".
func Resolve(ctx context.Context, s Store, id string, status Status, resolvedBy string, resolutionData map[string]interface{}) (*Approval, error) {
	if status != StatusApproved && status != StatusRejected {
		return nil, errors.New("approval: status must be approved or rejected")
	}
	return s.UpdateApproval(ctx, id, StatusPending, Resolution{
		Status:         status,
		ResolvedBy:     resolvedBy,
		ResolvedAt:     time.Now().Unix(),
		ResolutionData: resolutionData,
	})
}

// InMemoryStore is a goroutine-safe in-memory Store.
type InMemoryStore struct {
	mu        sync.RWMutex
	approvals map[string]*Approval
	order     []string
}

// NewInMemoryStore creates an empty in-memory store.
func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{approvals: map[string]*Approval{}}
}

func clone(a *Approval) *Approval {
	c := *a
	return &c
}

// CreateApproval stores a record.
func (s *InMemoryStore) CreateApproval(_ context.Context, a *Approval) error {
	if a == nil || a.ID == "" {
		return errors.New("approval: id is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.approvals[a.ID]; !exists {
		s.order = append(s.order, a.ID)
	}
	s.approvals[a.ID] = clone(a)
	return nil
}

// GetApproval returns a record by id.
func (s *InMemoryStore) GetApproval(_ context.Context, id string) (*Approval, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	a, ok := s.approvals[id]
	if !ok {
		return nil, ErrNotFound
	}
	return clone(a), nil
}

// ListApprovals returns matching records, most recent first.
func (s *InMemoryStore) ListApprovals(_ context.Context, f Filter) ([]*Approval, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []*Approval
	for i := len(s.order) - 1; i >= 0; i-- {
		a := s.approvals[s.order[i]]
		if (f.RunID != "" && a.RunID != f.RunID) ||
			(f.SessionID != "" && a.SessionID != f.SessionID) ||
			(f.Status != "" && a.Status != f.Status) ||
			(f.ApprovalType != "" && a.ApprovalType != f.ApprovalType) {
			continue
		}
		out = append(out, clone(a))
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].CreatedAt > out[j].CreatedAt })
	if f.Limit > 0 && len(out) > f.Limit {
		out = out[:f.Limit]
	}
	return out, nil
}

// UpdateApproval resolves a record when it is in the expected status.
func (s *InMemoryStore) UpdateApproval(_ context.Context, id string, expected Status, res Resolution) (*Approval, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.approvals[id]
	if !ok {
		return nil, ErrNotFound
	}
	if expected != "" && a.Status != expected {
		return nil, ErrStatusConflict
	}
	a.Status = res.Status
	a.ResolvedBy = res.ResolvedBy
	a.ResolvedAt = res.ResolvedAt
	if res.ResolutionData != nil {
		a.ResolutionData = res.ResolutionData
	}
	a.UpdatedAt = time.Now().Unix()
	return clone(a), nil
}

// UpdateRunStatus stamps run_status on every approval of the run.
func (s *InMemoryStore) UpdateRunStatus(_ context.Context, runID string, status models.RunStatus) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for _, a := range s.approvals {
		if a.RunID == runID && a.RunStatus != status {
			a.RunStatus = status
			a.UpdatedAt = time.Now().Unix()
			n++
		}
	}
	return n, nil
}

var _ Store = (*InMemoryStore)(nil)
