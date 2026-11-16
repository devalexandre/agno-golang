// MCP server exposure: serve an AgentOS as an MCP server (streamable HTTP).
//
// What it shows
//   - AgentOSOptions.Protocols.MCP mounts the MCP server at /mcp
//   - DefaultTools: the 8 built-in tools (get_agentos_config, run_agent,
//     run_team, run_workflow, continue_run, cancel_run, get_sessions,
//     get_session_runs), scoped by tag with ExcludeTags
//   - Expose: the agent is also published as its own tool ("ask_assistant")
//   - a custom tool, bounded tool arguments, the server card and the
//     plain-HTTP tool runner
//
// Try it
//
//	go run ./cookbook/os/mcp_server
//	curl http://localhost:7777/mcp/server-card
//	curl -X POST http://localhost:7777/mcp/server/tools/ask_assistant/run \
//	     -H 'Content-Type: application/json' -d '{"arguments":{"message":"Hi!"}}'
//	npx @modelcontextprotocol/inspector   # connect to http://localhost:7777/mcp
//
// Requires: a local Ollama (OLLAMA_HOST, default http://localhost:11434) with
// OLLAMA_MODEL (default llama3.2:latest) pulled.
package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ollama"
	agentOS "github.com/devalexandre/agno-golang/agno/os"
	"github.com/devalexandre/agno-golang/agno/os/component"
	"github.com/devalexandre/agno-golang/agno/os/mcpserver"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

func main() {
	ctx := context.Background()
	model, err := ollama.NewOllamaChat(models.WithID(env("OLLAMA_MODEL", "llama3.2:latest")), models.WithBaseURL(env("OLLAMA_HOST", "http://localhost:11434")))
	if err != nil {
		log.Fatal(err)
	}
	assistant, err := agent.NewAgent(agent.AgentConfig{Context: ctx, Name: "Assistant", Description: "A concise, helpful assistant", Model: model})
	if err != nil {
		log.Fatal(err)
	}

	// A custom tool next to the built-ins.
	clock := mcpserver.CustomTool{
		Tool: &mcp.Tool{Name: "server_time", Description: "Current server time (RFC3339).",
			InputSchema: map[string]any{"type": "object"}},
		Handler: func(context.Context, *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: time.Now().Format(time.RFC3339)}}}, nil
		},
	}

	aos, err := agentOS.NewAgentOS(agentOS.AgentOSOptions{
		OSID:     "mcp-os",
		Agents:   []*agent.Agent{assistant},
		Settings: &agentOS.AgentOSSettings{Port: 7777, Host: "localhost", EnableCORS: true},
		Protocols: agentOS.ProtocolOptions{MCP: &mcpserver.Config{
			Instructions:     "Use ask_assistant for questions; get_agentos_config lists everything else.",
			DefaultTools:     true,
			ExcludeTags:      []string{mcpserver.TagSession}, // keep the session tools off
			Expose:           []mcpserver.Exposure{{Component: component.FromAgent(assistant), Name: "ask_assistant"}},
			Tools:            []mcpserver.CustomTool{clock},
			MaxMessageLength: 4000, // bounded tool arguments
		}},
	})
	if err != nil {
		log.Fatal(err)
	}
	log.Println("MCP endpoint: http://localhost:7777/mcp")
	if err := aos.Serve(); err != nil {
		log.Fatal(err)
	}
}
