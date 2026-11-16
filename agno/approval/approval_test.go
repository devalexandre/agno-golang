package approval_test

import (
	"testing"

	"github.com/devalexandre/agno-golang/agno/approval"
	"github.com/devalexandre/agno-golang/agno/approval/approvaltest"
)

func TestInMemoryStoreConformance(t *testing.T) {
	approvaltest.RunStoreConformance(t, approval.NewInMemoryStore())
}
