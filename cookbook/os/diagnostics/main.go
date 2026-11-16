// Model-free diagnostics.
//
// Shows: AgentOS.RunDiagnostics checks the database (a live session read),
// authorization settings, component ids, knowledge wiring and required
// environment variables, without calling any model. Secrets are never
// reported: env checks report only "Set"/"Not set". The same report is served
// at GET /diagnostics (?required_env=A,B), which requires config:read when
// JWT authorization is on.
//
// Needs: nothing external.
//
//	OPENAI_API_KEY=x go run ./cookbook/os/diagnostics
//	curl "localhost:7777/diagnostics?required_env=OPENAI_API_KEY,DATABASE_URL"
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	agnoos "github.com/devalexandre/agno-golang/agno/os"
)

func main() {
	app, err := agnoos.NewAgentOS(agnoos.AgentOSOptions{
		OSID:     "diagnostics-os",
		Settings: &agnoos.AgentOSSettings{Host: "0.0.0.0", Port: 7777, EnableCORS: true, Timeout: time.Minute},
	})
	if err != nil {
		log.Fatal(err)
	}
	report := app.RunDiagnostics(context.Background(), "OPENAI_API_KEY")
	out, _ := json.MarshalIndent(report, "", "  ")
	fmt.Println(string(out))
	log.Fatal(app.Serve())
}
