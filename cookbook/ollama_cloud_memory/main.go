package main

import (
	"context"
	"os"
	"path/filepath"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/memory"
	"github.com/devalexandre/agno-golang/agno/memory/sqlite"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ollama"
	"github.com/pterm/pterm"
)

func main() {
	ctx := context.Background()

	pterm.DefaultBigText.WithLetters(
		pterm.NewLettersFromStringWithStyle("🤖", pterm.NewStyle(pterm.FgCyan)),
		pterm.NewLettersFromStringWithStyle(" Agno Golang", pterm.NewStyle(pterm.FgCyan)),
		pterm.NewLettersFromStringWithStyle(" + Memory", pterm.NewStyle(pterm.FgMagenta)),
	).Render()

	pterm.Println(pterm.FgGray.Sprint("Agent with persistent memory using Ollama Cloud"))
	pterm.Println()

	// Create Ollama Cloud model
	pterm.FgGray.Print("Initializing Ollama Cloud model... ")

	apiKey := os.Getenv("OLLAMA_API_KEY")
	if apiKey == "" {
		pterm.Println()
		pterm.FgRed.Println("✗ OLLAMA_API_KEY environment variable is required")
		os.Exit(1)
	}

	model, err := ollama.NewOllamaChat(
		models.WithID("kimi-k2:1t-cloud"),
		models.WithBaseURL("https://ollama.com"),
		models.WithAPIKey(apiKey),
	)
	if err != nil {
		pterm.Println()
		pterm.FgRed.Printf("✗ Failed to create Ollama Cloud model: %v\n", err)
		os.Exit(1)
	}

	pterm.FgGreen.Println("✓ Ready")

	// Setup memory database
	pterm.FgGray.Print("Setting up memory database... ")

	homeDir, err := os.UserHomeDir()
	if err != nil {
		pterm.Println()
		pterm.FgRed.Printf("✗ Failed to get home directory: %v\n", err)
		os.Exit(1)
	}

	memoryDir := filepath.Join(homeDir, ".agno_memory")
	if err := os.MkdirAll(memoryDir, 0755); err != nil {
		pterm.Println()
		pterm.FgRed.Printf("✗ Failed to create memory directory: %v\n", err)
		os.Exit(1)
	}

	dbPath := filepath.Join(memoryDir, "user_memories.db")

	db, err := sqlite.NewSqliteMemoryDb("user_memories", dbPath)
	if err != nil {
		pterm.Println()
		pterm.FgRed.Printf("✗ Failed to create memory database: %v\n", err)
		os.Exit(1)
	}

	pterm.FgGreen.Println("✓ Ready")

	// Create memory manager
	pterm.FgGray.Print("Creating memory manager... ")
	memoryManager := memory.NewMemory(model, db)
	pterm.FgGreen.Println("✓ Ready")

	// Create Agent with Memory Parameter (Python style!)
	pterm.FgGray.Print("Creating agent with memory... ")

	userID := "alice_thompson@example.com"

	agt, err := agent.NewAgent(agent.AgentConfig{
		Context:             ctx,
		Model:               model,
		Name:                "Memory Assistant",
		Instructions:        "You are a helpful AI assistant with persistent memory. Remember important user information and reference it naturally in conversations.",
		Memory:              memoryManager,
		UserID:              userID,
		EnableUserMemories:  true,
		EnableAgenticMemory: true,
	})
	if err != nil {
		pterm.Println()
		pterm.FgRed.Printf("✗ Failed to create agent: %v\n", err)
		os.Exit(1)
	}

	pterm.FgGreen.Println("✓ Ready")
	pterm.Println()

	// DEMO 1: Learning Personal Information
	pterm.FgCyan.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	pterm.FgCyan.Println("📝 DEMO 1: Learning Personal Information")
	pterm.FgCyan.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	pterm.Println()

	userInput1 := "My name is Alice Thompson. I'm a software engineer from San Francisco. " +
		"I love working with Go and building scalable systems. In my free time, I enjoy hiking and photography."

	pterm.FgYellow.Printf("👤 User: %s\n", userInput1)
	pterm.Println()

	resp1, err := agt.Run(userInput1)
	if err != nil {
		pterm.FgRed.Printf("✗ Error: %v\n", err)
		os.Exit(1)
	}

	pterm.FgGreen.Printf("🤖 Agent: %s\n", resp1.TextContent)
	pterm.Println()

	// DEMO 2: Recalling Stored Information
	pterm.FgCyan.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	pterm.FgCyan.Println("💭 DEMO 2: Recalling Stored Information")
	pterm.FgCyan.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	pterm.Println()

	userInput2 := "What was I telling you about my hobbies?"

	pterm.FgYellow.Printf("👤 User: %s\n", userInput2)
	pterm.Println()

	resp2, err := agt.Run(userInput2)
	if err != nil {
		pterm.FgRed.Printf("✗ Error: %v\n", err)
		os.Exit(1)
	}

	pterm.FgGreen.Printf("🤖 Agent: %s\n", resp2.TextContent)
	pterm.Println()

	// DEMO 3: Learning New Preferences
	pterm.FgCyan.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	pterm.FgCyan.Println("📚 DEMO 3: Learning New Preferences")
	pterm.FgCyan.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	pterm.Println()

	userInput3 := "I'm recently interested in machine learning and studying with agno!"

	pterm.FgYellow.Printf("👤 User: %s\n", userInput3)
	pterm.Println()

	resp3, err := agt.Run(userInput3)
	if err != nil {
		pterm.FgRed.Printf("✗ Error: %v\n", err)
		os.Exit(1)
	}

	pterm.FgGreen.Printf("🤖 Agent: %s\n", resp3.TextContent)
	pterm.Println()

	// Final Summary
	pterm.FgCyan.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	pterm.FgCyan.Println("📚 Final Memory Summary")
	pterm.FgCyan.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	pterm.Println()

	finalMemories, err := db.GetUserMemories(ctx, userID)
	if err == nil && len(finalMemories) > 0 {
		pterm.FgMagenta.Printf("✓ Agent stored %d total memories:\n", len(finalMemories))
		pterm.Println()
		for i, mem := range finalMemories {
			pterm.FgBlue.Printf("  Memory #%d: %s\n", i+1, mem.Memory)
		}
	} else {
		pterm.FgGray.Println("No memories were stored")
	}

	pterm.FgGreen.Println("\n✅ Demo Complete!")
	pterm.FgGray.Printf("Memory stored at: %s\n\n", dbPath)
}
