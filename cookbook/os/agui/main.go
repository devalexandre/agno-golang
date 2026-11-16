// AG-UI interface: serve an agent to AG-UI frontends (CopilotKit, Dojo, ...).
//
// What it shows
//   - AgentOSOptions.Protocols.AGUI mounts POST {prefix}/agui (RunAgentInput
//     in, AG-UI events out over SSE) and GET {prefix}/status
//   - text messages stream as TEXT_MESSAGE_* events, tool calls as
//     TOOL_CALL_*; a paused (human-in-the-loop) run ends with the pending tool
//     calls and is resumed by trailing "tool" messages on the same thread
//
// Try it
//
//	go run ./cookbook/os/agui
//	curl -N -X POST http://localhost:7779/chat/agui -H 'Content-Type: application/json' \
//	     -d '{"threadId":"t1","runId":"r1","messages":[{"id":"1","role":"user","content":"Hello!"}]}'
//	curl http://localhost:7779/chat/status
//
// Requires: a local Ollama (OLLAMA_HOST, default http://localhost:11434) with
// OLLAMA_MODEL (default llama3.2:latest).
package main

import (
	"context"
	"log"
	"os"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ollama"
	agentOS "github.com/devalexandre/agno-golang/agno/os"
	"github.com/devalexandre/agno-golang/agno/os/agui"
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
	chat, err := agent.NewAgent(agent.AgentConfig{Context: context.Background(), Name: "Chat", Description: "Friendly chat agent", Model: model})
	if err != nil {
		log.Fatal(err)
	}
	aos, err := agentOS.NewAgentOS(agentOS.AgentOSOptions{
		OSID:     "agui-os",
		Agents:   []*agent.Agent{chat},
		Settings: &agentOS.AgentOSSettings{Port: 7779, Host: "localhost", EnableCORS: true},
		Protocols: agentOS.ProtocolOptions{AGUI: &agentOS.AGUIOptions{
			Config:    agui.Config{Prefix: "/chat"},
			Component: component.FromAgent(chat),
		}},
	})
	if err != nil {
		log.Fatal(err)
	}
	log.Println("AG-UI endpoint: http://localhost:7779/chat/agui")
	if err := aos.Serve(); err != nil {
		log.Fatal(err)
	}
}
