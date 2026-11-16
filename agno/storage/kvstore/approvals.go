package kvstore

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"time"

	"github.com/devalexandre/agno-golang/agno/approval"
	"github.com/devalexandre/agno-golang/agno/models"
)

// Approval records (approval.Store), kept in the "approvals" collection.
//
// Status transitions are serialized within the process; backends without
// conditional writes cannot make them atomic across processes.

func approvalIndex(a *approval.Approval) map[string]string {
	return map[string]string{
		"run_id":        a.RunID,
		"session_id":    a.SessionID,
		"status":        string(a.Status),
		"approval_type": string(a.ApprovalType),
	}
}

// CreateApproval stores an approval record.
func (s *Store) CreateApproval(ctx context.Context, a *approval.Approval) error {
	if a == nil || a.ID == "" {
		return errors.New("approval: id is required")
	}
	if a.CreatedAt == 0 {
		a.CreatedAt = time.Now().Unix()
	}
	if a.UpdatedAt == 0 {
		a.UpdatedAt = a.CreatedAt
	}
	if ins, ok := s.backend.(Inserter); ok {
		doc, err := json.Marshal(a)
		if err != nil {
			return err
		}
		created, err := ins.Insert(ctx, CollApprovals, a.ID, doc, approvalIndex(a))
		if err != nil {
			return err
		}
		if !created {
			return errors.New("approval: " + a.ID + " already exists")
		}
		return nil
	}
	return s.put(ctx, CollApprovals, a.ID, a, approvalIndex(a))
}

// GetApproval returns an approval record (approval.ErrNotFound when missing).
func (s *Store) GetApproval(ctx context.Context, id string) (*approval.Approval, error) {
	var a approval.Approval
	ok, err := s.get(ctx, CollApprovals, id, &a)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, approval.ErrNotFound
	}
	return &a, nil
}

// ListApprovals returns matching approval records, most recent first.
func (s *Store) ListApprovals(ctx context.Context, f approval.Filter) ([]*approval.Approval, error) {
	where := map[string]string{}
	for k, v := range map[string]string{
		"run_id": f.RunID, "session_id": f.SessionID, "status": string(f.Status), "approval_type": string(f.ApprovalType),
	} {
		if v != "" {
			where[k] = v
		}
	}
	docs, err := s.backend.List(ctx, CollApprovals, where)
	if err != nil {
		return nil, err
	}
	var out []*approval.Approval
	for _, doc := range docs {
		var a approval.Approval
		if err := json.Unmarshal(doc, &a); err != nil {
			return nil, err
		}
		// The backend may ignore where; re-check every filter.
		if (f.RunID != "" && a.RunID != f.RunID) ||
			(f.SessionID != "" && a.SessionID != f.SessionID) ||
			(f.Status != "" && a.Status != f.Status) ||
			(f.ApprovalType != "" && a.ApprovalType != f.ApprovalType) {
			continue
		}
		out = append(out, &a)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].CreatedAt != out[j].CreatedAt {
			return out[i].CreatedAt > out[j].CreatedAt
		}
		return out[i].ID > out[j].ID
	})
	if f.Limit > 0 && len(out) > f.Limit {
		out = out[:f.Limit]
	}
	return out, nil
}

// UpdateApproval resolves an approval when its status equals expected.
func (s *Store) UpdateApproval(ctx context.Context, id string, expected approval.Status, res approval.Resolution) (*approval.Approval, error) {
	s.approvalMu.Lock()
	defer s.approvalMu.Unlock()
	a, err := s.GetApproval(ctx, id)
	if err != nil {
		return nil, err
	}
	if expected != "" && a.Status != expected {
		return nil, approval.ErrStatusConflict
	}
	a.Status, a.ResolvedBy, a.ResolvedAt = res.Status, res.ResolvedBy, res.ResolvedAt
	if res.ResolutionData != nil {
		a.ResolutionData = res.ResolutionData
	}
	a.UpdatedAt = time.Now().Unix()
	if err := s.put(ctx, CollApprovals, a.ID, a, approvalIndex(a)); err != nil {
		return nil, err
	}
	return a, nil
}

// UpdateRunStatus stamps run_status on every approval of the run.
func (s *Store) UpdateRunStatus(ctx context.Context, runID string, status models.RunStatus) (int, error) {
	s.approvalMu.Lock()
	defer s.approvalMu.Unlock()
	list, err := s.ListApprovals(ctx, approval.Filter{RunID: runID})
	if err != nil {
		return 0, err
	}
	n := 0
	for _, a := range list {
		if a.RunStatus == status {
			continue
		}
		a.RunStatus, a.UpdatedAt = status, time.Now().Unix()
		if err := s.put(ctx, CollApprovals, a.ID, a, approvalIndex(a)); err != nil {
			return n, err
		}
		n++
	}
	return n, nil
}

// DeleteApproval removes an approval record.
func (s *Store) DeleteApproval(ctx context.Context, id string) error {
	return s.backend.Delete(ctx, CollApprovals, id)
}

var _ approval.Store = (*Store)(nil)
