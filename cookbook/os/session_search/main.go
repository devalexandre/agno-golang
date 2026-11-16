// Session routes: pagination, advanced search and cursor-paginated runs.
//
// Shows: GET /sessions (paginated), GET /sessions/filter-schema, POST
// /sessions/search with an AND/OR/NOT filter expression, rename/patch/delete,
// and GET /sessions/{id}/runs?limit=N&before_run_index=M (cursor pages).
// Two databases are registered, so requests choose one with ?db_id=.
//
// Needs: nothing external; no model is called.
//
//	go run ./cookbook/os/session_search
//	curl "localhost:7777/sessions?db_id=main&type=agent&limit=5&page=1"
//	curl localhost:7777/sessions/filter-schema
//	curl -X POST "localhost:7777/sessions/search?db_id=main" -H 'Content-Type: application/json' \
//	  -d '{"filter":{"op":"AND","conditions":[{"op":"EQ","key":"user_id","value":"alice"},{"op":"CONTAINS","key":"session_name","value":"billing"}]}}'
//	curl -X POST "localhost:7777/sessions/s-1/rename?db_id=main" -H 'Content-Type: application/json' -d '{"session_name":"Renamed"}'
//	curl "localhost:7777/sessions/s-1/runs?db_id=main&limit=2"
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	agnoos "github.com/devalexandre/agno-golang/agno/os"
)

func main() {
	mainDB := agnoos.NewInMemoryDB("main")
	archive := agnoos.NewInMemoryDB("archive")

	// Seed a few sessions with runs.
	now := time.Now().Unix()
	names := []string{"Billing question", "Refund", "Billing follow-up"}
	for i, user := range []string{"alice", "bob", "alice"} {
		s := &agnoos.SessionRecord{
			SessionID: fmt.Sprintf("s-%d", i+1), SessionType: agnoos.SessionTypeAgent, AgentID: "support",
			UserID: user, SessionName: names[i], CreatedAt: now + int64(i), UpdatedAt: now + int64(i),
		}
		for r := 0; r < 3; r++ {
			s.Runs = append(s.Runs, &agnoos.RunRecord{
				RunID: fmt.Sprintf("%s-run-%d", s.SessionID, r), RunIndex: r, SessionID: s.SessionID,
				AgentID: "support", Status: "COMPLETED", RunInput: fmt.Sprintf("question %d", r),
				Content: fmt.Sprintf("answer %d", r), CreatedAt: now + int64(r),
			})
		}
		if err := mainDB.UpsertSession(context.Background(), s); err != nil {
			log.Fatal(err)
		}
	}

	app, err := agnoos.NewAgentOS(agnoos.AgentOSOptions{
		OSID:      "sessions-os",
		Databases: []agnoos.Database{mainDB, archive},
		Settings:  &agnoos.AgentOSSettings{Host: "0.0.0.0", Port: 7777, EnableCORS: true, Timeout: time.Minute},
	})
	if err != nil {
		log.Fatal(err)
	}
	log.Fatal(app.Serve())
}
