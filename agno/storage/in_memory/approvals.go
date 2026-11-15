package inmemory

import (
	"context"

	"github.com/devalexandre/agno-golang/agno/approval"
	"github.com/devalexandre/agno-golang/agno/models"
)

// Approval records (approval.Store), so an agent using this DB keeps
// human-in-the-loop approvals across restarts.

// CreateApproval stores an approval record.
func (d *InMemoryDB) CreateApproval(ctx context.Context, a *approval.Approval) error {
	return d.approvals.CreateApproval(ctx, a)
}

// GetApproval returns an approval record (approval.ErrNotFound when missing).
func (d *InMemoryDB) GetApproval(ctx context.Context, id string) (*approval.Approval, error) {
	return d.approvals.GetApproval(ctx, id)
}

// ListApprovals returns matching approval records, most recent first.
func (d *InMemoryDB) ListApprovals(ctx context.Context, f approval.Filter) ([]*approval.Approval, error) {
	return d.approvals.ListApprovals(ctx, f)
}

// UpdateApproval resolves an approval when its status equals expected.
func (d *InMemoryDB) UpdateApproval(ctx context.Context, id string, expected approval.Status, res approval.Resolution) (*approval.Approval, error) {
	return d.approvals.UpdateApproval(ctx, id, expected, res)
}

// UpdateRunStatus stamps run_status on every approval of the run.
func (d *InMemoryDB) UpdateRunStatus(ctx context.Context, runID string, status models.RunStatus) (int, error) {
	return d.approvals.UpdateRunStatus(ctx, runID, status)
}

var _ approval.Store = (*InMemoryDB)(nil)
