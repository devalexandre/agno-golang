package sqlstore

import (
	"context"

	"github.com/devalexandre/agno-golang/agno/approval"
	"github.com/devalexandre/agno-golang/agno/models"
)

// Approval records (approval.Store), so an agent using this DB keeps
// human-in-the-loop approvals across restarts. The table is created on first use.

// CreateApproval stores an approval record.
func (s *SQLStore) CreateApproval(ctx context.Context, a *approval.Approval) error {
	return s.approvals.CreateApproval(ctx, a)
}

// GetApproval returns an approval record (approval.ErrNotFound when missing).
func (s *SQLStore) GetApproval(ctx context.Context, id string) (*approval.Approval, error) {
	return s.approvals.GetApproval(ctx, id)
}

// ListApprovals returns matching approval records, most recent first.
func (s *SQLStore) ListApprovals(ctx context.Context, f approval.Filter) ([]*approval.Approval, error) {
	return s.approvals.ListApprovals(ctx, f)
}

// UpdateApproval resolves an approval when its status equals expected.
func (s *SQLStore) UpdateApproval(ctx context.Context, id string, expected approval.Status, res approval.Resolution) (*approval.Approval, error) {
	return s.approvals.UpdateApproval(ctx, id, expected, res)
}

// UpdateRunStatus stamps run_status on every approval of the run.
func (s *SQLStore) UpdateRunStatus(ctx context.Context, runID string, status models.RunStatus) (int, error) {
	return s.approvals.UpdateRunStatus(ctx, runID, status)
}

var _ approval.Store = (*SQLStore)(nil)
