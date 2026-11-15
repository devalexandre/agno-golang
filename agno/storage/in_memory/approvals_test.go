package inmemory

import (
	"testing"

	"github.com/devalexandre/agno-golang/agno/approval/approvaltest"
)

func TestInMemoryDBApprovalConformance(t *testing.T) {
	approvaltest.RunStoreConformance(t, New())
}
