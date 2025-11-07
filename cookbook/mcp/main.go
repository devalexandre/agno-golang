package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ollama"
	"github.com/devalexandre/agno-golang/agno/tools/mcp"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

func main() {
	fmt.Println("=== MCP Agent Example (Following Python Pattern) ===")
	fmt.Println("This example follows the exact same pattern as Python agno MCPTools")
	fmt.Println()

	// Get current workspace folder
	workspaceFolder, err := os.Getwd()
	if err != nil {
		log.Fatalf("Failed to get current directory: %v", err)
	}

	// Use MCP filesystem server
	serverCommand := fmt.Sprintf("docker run --rm -i --mount type=bind,src=%s,dst=/workspace mcp/filesystem /workspace", workspaceFolder)

	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "filesystem":
			serverCommand = fmt.Sprintf("docker run --rm -i --mount type=bind,src=%s,dst=/workspace mcp/filesystem /workspace", workspaceFolder)
		case "sqlite":
			serverCommand = "mcp-server-sqlite --db-path /tmp/test.db"
		default:
			fmt.Println("Available servers: git, filesystem, sqlite")
			fmt.Printf("Usage: %s [server_type] [message]\n", filepath.Base(os.Args[0]))
			return
		}
	}

	fmt.Printf("🔗 Connecting to MCP server...\n")
	fmt.Printf("📞 Command: %s\n", serverCommand)

	// Create MCPTool instance - usando apenas ClientSession
	mcpTool, err := mcp.NewMCPTool("MCPTools", serverCommand)
	if err != nil {
		log.Fatalf("Failed to create MCP tool: %v", err)
	}
	defer mcpTool.Close()

	// Connect to MCP server - descobre ferramentas dinamicamente
	ctx := context.Background()
	if err := mcpTool.Connect(ctx); err != nil {
		log.Fatalf("Failed to connect to MCP server: %v", err)
	}

	// Create Ollama model
	model, err := ollama.NewOllamaChat(
		models.WithID("llama3.2:latest"),
		models.WithBaseURL("http://localhost:11434"),
	)
	if err != nil {
		log.Fatalf("Failed to create Ollama model: %v", err)
	}

	// Create agent with MCP tools - same as Python agent integration
	agentInstance, err := agent.NewAgent(agent.AgentConfig{
		Context: ctx,
		Name:    "MCP Assistant",
		Model:   model,
		Tools:   []toolkit.Tool{mcpTool}, // MCPTool implements toolkit.Tool
		Instructions: `You are an assistant with access to MCP tools. 

When you need to use Git tools, always provide:
- repo_path: "/workspace" (this is the mounted workspace in Docker)

For file operations, use appropriate paths relative to the workspace.

Be helpful and provide clear information about what you're doing.`,
		Debug:         false,
		ShowToolsCall: true,
	})
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	// Get message from command line or use default
	message := "List the files in the /workspace directory"
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "sqlite":
			message = "Create a table called 'users' with id and name columns, then insert a test user"
		case "git":
			message = "Show me the Git status of this repository"
		}
	}
	if len(os.Args) > 2 {
		message = os.Args[2]
	}

	fmt.Printf("💬 Question: %s\n", message)
	fmt.Println("🤖 Processing...")

	// Run the agent
	response, err := agentInstance.Run(message)
	if err != nil {
		log.Fatalf("Agent run failed: %v", err)
	}

	fmt.Printf("📋 Response:\n%s\n", response.TextContent)
}
