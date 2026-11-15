// Remote AgentOS client: drive an AgentOS over its REST API and re-serve a
// remote agent over MCP without a local AgentOS.
//
// What it shows
//   - agentos.Client: List, Run, RunStream (SSE events), GetSessions
//   - agentos.RemoteComponent: a remote agent as a component.Component
//   - mcpserver used standalone: the remote agent is exposed as an MCP tool
//     on :7780/mcp, proxying calls to the remote AgentOS
//
// Try it
//
//	go run ./cookbook/os-example     # or any AgentOS on AGENTOS_URL
//	AGENTOS_URL=http://localhost:7777 go run ./cookbook/client/remote_agentos
//
// Requires: AGENTOS_URL (default http://localhost:7777); AGENTOS_SECURITY_KEY
// when the remote AgentOS sets one.
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/devalexandre/agno-golang/agno/client/agentos"
	"github.com/devalexandre/agno-golang/agno/os/component"
	"github.com/devalexandre/agno-golang/agno/os/mcpserver"
)

func main() {
	ctx := context.Background()
	base := os.Getenv("AGENTOS_URL")
	if base == "" {
		base = "http://localhost:7777"
	}
	client := agentos.New(base, os.Getenv("AGENTOS_SECURITY_KEY"))

	agents, err := client.List(ctx, component.KindAgent)
	if err != nil {
		log.Fatal(err)
	}
	if len(agents) == 0 {
		log.Fatal("the remote AgentOS has no agents")
	}
	for _, a := range agents {
		fmt.Printf("agent %s (%s): %s\n", a.ID, a.Name, a.Description)
	}
	target := agents[0].ID

	out, err := client.Run(ctx, component.KindAgent, target, agentos.RunParams{Message: "Say hello in one sentence.", UserID: "demo"})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("run %s: %s\n", out.RunID, out.Text())

	fmt.Print("stream: ")
	_, err = client.RunStream(ctx, component.KindAgent, target, agentos.RunParams{Message: "Count to five.", SessionID: out.SessionID},
		func(ev agentos.RunEvent) error {
			if ev.NormalizedEvent() == "RunContent" {
				fmt.Print(ev.Text())
			}
			return nil
		})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println()

	if sessions, err := client.GetSessions(ctx, agentos.SessionsQuery{Type: "agent", ComponentID: target}); err == nil {
		fmt.Printf("sessions: %v\n", sessions["meta"])
	}

	// Re-serve the remote agent over MCP, standalone.
	remote := agentos.NewRemote(client, component.KindAgent, target)
	_ = remote.Refresh(ctx)
	srv, err := mcpserver.New(component.NewRegistry(remote), &mcpserver.Config{
		Name:   "remote-proxy",
		Expose: []mcpserver.Exposure{{Component: remote, Name: "remote_agent"}},
	})
	if err != nil {
		log.Fatal(err)
	}
	log.Println("MCP proxy: http://localhost:7780/mcp")
	log.Fatal(http.ListenAndServe("localhost:7780", srv))
}
