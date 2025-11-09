package main

import (
	"context"
	"log"
	"os"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ollama"
	agentOS "github.com/devalexandre/agno-golang/agno/os"
	"github.com/devalexandre/agno-golang/agno/tools"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

func main() {
	ctx := context.Background()

	// Get API key
	apiKey := os.Getenv("OLLAMA_API_KEY")
	if apiKey == "" {
		log.Fatal("OLLAMA_API_KEY environment variable is required")
	}

	// Create Ollama Cloud model
	ollamaModel, err := ollama.NewOllamaChat(
		models.WithID("kimi-k2:1t-cloud"),
		models.WithBaseURL("https://ollama.com"),
		models.WithAPIKey(apiKey),
	)
	if err != nil {
		log.Fatalf("Failed to create Ollama Cloud model: %v", err)
	}

	// Create Research Agent with DuckDuckGo search
	researcher, err := agent.NewAgent(agent.AgentConfig{
		Context:      ctx,
		Name:         "Researcher",
		Role:         "Research Specialist",
		Description:  "Expert at finding and analyzing information from the web",
		Instructions: "You are a research specialist. Search for accurate, up-to-date information using DuckDuckGo and provide well-structured summaries with sources.",
		Model:        ollamaModel,
		Tools: []toolkit.Tool{
			tools.NewDuckDuckGoTool(), // DuckDuckGo search
		},
		ShowToolsCall: true,
		Markdown:      true,
		Debug:         false,
	})
	if err != nil {
		log.Fatalf("Failed to create researcher agent: %v", err)
	}

	// Create Writing Assistant with file operations
	writer, err := agent.NewAgent(agent.AgentConfig{
		Context:      ctx,
		Name:         "Writer",
		Role:         "Content Writer",
		Description:  "Creative writer that produces engaging content",
		Instructions: "You are a professional content writer. Create clear, engaging, and well-structured content. Use markdown formatting for better readability.",
		Model:        ollamaModel,
		Tools: []toolkit.Tool{
			tools.NewFileTool(true), // Can read and write files
		},
		ShowToolsCall: true,
		Markdown:      true,
		Debug:         false,
	})
	if err != nil {
		log.Fatalf("Failed to create writer agent: %v", err)
	}

	// Create General Assistant
	assistant, err := agent.NewAgent(agent.AgentConfig{
		Context:      ctx,
		Name:         "Assistant",
		Role:         "AI Assistant",
		Description:  "Helpful general-purpose AI assistant",
		Instructions: "You are a helpful, friendly, and knowledgeable AI assistant. Provide clear and concise answers. Be professional yet approachable.",
		Model:        ollamaModel,
		Markdown:     true,
		Debug:        false,
	})
	if err != nil {
		log.Fatalf("Failed to create assistant agent: %v", err)
	}

	// Get security key from environment (optional)
	securityKey := os.Getenv("SECURITY_KEY")

	// Get port from environment or use default
	port := 8080
	if portEnv := os.Getenv("PORT"); portEnv != "" {
		log.Printf("Using port from PORT environment variable: %s", portEnv)
	}

	// Create AgentOS instance
	osInstance, err := agentOS.NewAgentOS(agentOS.AgentOSOptions{
		OSID:        "ollama-cloud-agentos",
		Description: agentOS.StringPtr("AgentOS powered by Ollama Cloud - kimi-k2:1t-cloud model"),
		Agents:      []*agent.Agent{researcher, writer, assistant},
		Settings: &agentOS.AgentOSSettings{
			Port:        port,
			Host:        "0.0.0.0", // Accept connections from any IP
			Debug:       false,
			EnableCORS:  true,
			SecurityKey: securityKey, // Optional - enables authentication
		},
	})
	if err != nil {
		log.Fatalf("Failed to create AgentOS: %v", err)
	}

	log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	log.Println("🚀 AgentOS with Ollama Cloud Starting...")
	log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	log.Printf("📍 Server: http://localhost:%d\n", port)
	log.Printf("🤖 Model: kimi-k2:1t-cloud (Ollama Cloud)\n")
	log.Printf("👥 Agents: 3 (Researcher, Writer, Assistant)\n")
	log.Printf("🔧 Tools: DuckDuckGo Search, File Operations\n")
	if securityKey != "" {
		log.Printf("🔒 Security: Enabled (authentication required)\n")
	} else {
		log.Printf("🔓 Security: Disabled (no authentication)\n")
	}
	log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	log.Println("")
	log.Println("📚 Available Endpoints:")
	log.Printf("  • GET    http://localhost:%d/             - API root\n", port)
	log.Printf("  • GET    http://localhost:%d/agents       - List all agents\n", port)
	log.Printf("  • POST   http://localhost:%d/agents/:id/runs - Execute agent\n", port)
	log.Printf("  • POST   http://localhost:%d/agents/:id/runs/:run_id/continue - Continue run\n", port)
	log.Printf("  • WS     ws://localhost:%d/workflows/ws   - WebSocket for workflows\n", port)
	log.Printf("  • GET    http://localhost:%d/sessions     - List sessions\n", port)
	log.Println("")
	log.Println("💡 Quick Test:")
	log.Printf("  curl -X POST http://localhost:%d/agents/$(curl -s http://localhost:%d/agents | jq -r '.[0].id')/runs \\\n", port, port)
	log.Println("    -F 'message=Tell me a joke' \\")
	log.Println("    -F 'stream=false'")
	log.Println("")
	log.Println("Press Ctrl+C to stop the server")
	log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	// Start the server
	if err := osInstance.Serve(); err != nil {
		log.Fatalf("Failed to start AgentOS: %v", err)
	}
}
