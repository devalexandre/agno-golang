// Storage: persistent approval records.
//
// Every storage backend implements approval.Store, so an agent whose DB is
// one of them keeps "required" approvals across restarts. No ApprovalStore
// config is needed: the agent uses its DB.
//
//   - SQL backends (postgres, mysql, singlestore, sqlite) use an
//     <prefix>_approvals table (agno_approvals for sqlite), with the same
//     columns as Python's approvals table. Resolution is a conditional UPDATE,
//     so two approvers cannot both resolve the same record.
//   - Document backends (redis, valkey, mongo, dynamo, firestore, gcs_json,
//     surrealdb, clickhouse) use an "approvals" collection.
//   - inmemory.New() keeps them in memory.
//
// This example uses a sqlite file and simulates two processes: one pauses the
// run, the other approves it and continues the run. Runs offline with a
// scripted stub model; no env vars or services needed.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/approval"
	"github.com/devalexandre/agno-golang/agno/storage/sqlite"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
	"github.com/devalexandre/agno-golang/cookbook/internal/toolcallstub"
)

type PayParams struct {
	Invoice string  `json:"invoice" description:"Invoice id" required:"true"`
	Amount  float64 `json:"amount" description:"Amount" required:"true"`
}

type PaymentsTool struct{ toolkit.Toolkit }

func (p *PaymentsTool) Pay(in PayParams) (interface{}, error) {
	return fmt.Sprintf("paid invoice %s (%.2f)", in.Invoice, in.Amount), nil
}

func newAgent(db *sqlite.SqliteStorage) *agent.Agent {
	tool := &PaymentsTool{Toolkit: toolkit.NewToolkit()}
	tool.Name, tool.Description = "Payments", "Payments"
	tool.RegisterWithOptions("Pay", "Pay an invoice", tool, tool.Pay, PayParams{}, toolkit.WithApproval(toolkit.ApprovalRequired))
	a, err := agent.NewAgent(agent.AgentConfig{
		Context:   context.Background(),
		Name:      "payments-agent",
		Model:     toolcallstub.New("Done.", "Payments_Pay", `{"invoice":"INV-7","amount":1200}`),
		Tools:     []toolkit.Tool{tool},
		DB:        db, // also the approval store
		SessionID: "finance-session",
	})
	if err != nil {
		log.Fatal(err)
	}
	return a
}

func openDB(file string) *sqlite.SqliteStorage {
	db, err := sqlite.NewSqliteStorage(sqlite.SqliteStorageConfig{TableName: "agno_sessions", DBFile: &file})
	if err != nil {
		log.Fatal(err)
	}
	if err := db.CreateTables(context.Background()); err != nil {
		log.Fatal(err)
	}
	return db
}

func main() {
	ctx := context.Background()
	dir, err := os.MkdirTemp("", "agno-approvals")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(dir)
	file := filepath.Join(dir, "agent.db")

	// Process 1: the run pauses; a pending approval is written to the DB.
	db1 := openDB(file)
	resp, err := newAgent(db1).Run("Pay invoice INV-7")
	if err != nil {
		log.Fatal(err)
	}
	runID := resp.RunID
	db1.Close()
	fmt.Println("process 1: run", resp.Status)

	// Process 2: an approver lists pending approvals and resolves one.
	db2 := openDB(file)
	defer db2.Close()
	pending, err := db2.ListApprovals(ctx, approval.Filter{Status: approval.StatusPending})
	if err != nil {
		log.Fatal(err)
	}
	for _, p := range pending {
		fmt.Printf("process 2: pending approval %s tool=%s args=%v\n", p.ID, p.ToolName, p.ToolArgs)
	}

	a := newAgent(db2)
	if _, err := a.ContinueRun(ctx, runID); errors.Is(err, agent.ErrApprovalPending) {
		fmt.Println("process 2: continue refused while pending")
	}
	if _, err := approval.Resolve(ctx, db2, pending[0].ID, approval.StatusApproved, "finance-lead", nil); err != nil {
		log.Fatal(err)
	}
	out, err := a.ContinueRun(ctx, runID)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("process 2: status=%s content=%q\n", out.Status, out.TextContent)

	rec, _ := db2.GetApproval(ctx, pending[0].ID)
	fmt.Printf("approval record: status=%s resolved_by=%s run_status=%s\n", rec.Status, rec.ResolvedBy, rec.RunStatus)
}
