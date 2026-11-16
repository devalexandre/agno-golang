// A2A server: expose AgentOS agents over the Agent2Agent protocol (v1.0 with
// v0.3 compatibility).
//
// What it shows
//   - AgentOSOptions.Protocols.A2A mounts /a2a/agents/{id} (JSON-RPC:
//     SendMessage / message/send, SendStreamingMessage / message/stream,
//     GetTask, CancelTask, ListTasks) and the agent card at
//     /a2a/agents/{id}/.well-known/agent-card.json
//   - an in-memory task store (swap Config.TaskStore for your own)
//
// Try it
//
//	go run ./cookbook/a2a/server
//	curl http://localhost:7778/a2a/agents/agent_<id>/.well-known/agent-card.json   # id printed at startup
//	curl -X POST http://localhost:7778/a2a/agents/<id> -H 'Content-Type: application/json' -d '{
//	  "jsonrpc":"2.0","id":1,"method":"SendMessage",
//	  "params":{"message":{"messageId":"m1","role":"ROLE_USER","parts":[{"text":"Tell me a joke"}]}}}'
//	go run ./cookbook/a2a/client   # A2A_AGENT_URL=http://localhost:7778/a2a/agents/<id>
//
// Requires: a local Ollama (OLLAMA_HOST, default http://localhost:11434) with
// OLLAMA_MODEL (default llama3.2:latest).
package main

import (
	"context"
	"log"
	"os"

	"github.com/devalexandre/agno-golang/agno/a2a"
	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ollama"
	agentOS "github.com/devalexandre/agno-golang/agno/os"
	"github.com/devalexandre/agno-golang/agno/os/a2aserver"
	"github.com/devalexandre/agno-golang/agno/os/component"
)

func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

func main() {
	model, err := ollama.NewOllamaChat(models.WithID(env("OLLAMA_MODEL", "llama3.2:latest")), models.WithBaseURL(env("OLLAMA_HOST", "http://localhost:11434")))
	if err != nil {
		log.Fatal(err)
	}
	jokes, err := agent.NewAgent(agent.AgentConfig{Context: context.Background(), Name: "Joker", Description: "Tells short, clean jokes", Model: model})
	if err != nil {
		log.Fatal(err)
	}
	aos, err := agentOS.NewAgentOS(agentOS.AgentOSOptions{
		OSID:     "a2a-os",
		Agents:   []*agent.Agent{jokes},
		Settings: &agentOS.AgentOSSettings{Port: 7778, Host: "localhost", EnableCORS: true},
		Protocols: agentOS.ProtocolOptions{A2A: &a2aserver.Config{
			Provider: &a2a.AgentProvider{Organization: "Example Org", URL: "https://example.com"},
		}},
	})
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("A2A agent: http://localhost:7778/a2a/agents/%s", component.FromAgent(jokes).ID())
	if err := aos.Serve(); err != nil {
		log.Fatal(err)
	}
}
