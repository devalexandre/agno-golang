// Example: let the agent manage user memories with MemoryTools.
//
// Env vars: none (no API key needed). Requires a local Ollama server with
// llama3.2 at http://localhost:11434. Memories are stored in a local SQLite
// file (memorytools_demo.db).
//
//	go run ./cookbook/tools/memorytools
package main

import (
	"context"
	"log"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/memory/sqlite"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ollama"
	"github.com/devalexandre/agno-golang/agno/tools/memorytools"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

func main() {
	ctx := context.Background()
	model, err := ollama.NewOllamaChat(models.WithID("llama3.2:latest"), models.WithBaseURL("http://localhost:11434"))
	if err != nil {
		log.Fatalf("Failed to create model: %v", err)
	}

	db, err := sqlite.NewSqliteMemoryDb("user_memories", "memorytools_demo.db")
	if err != nil {
		log.Fatalf("Failed to open memory db: %v", err)
	}

	mt, err := memorytools.NewMemoryTools(db, memorytools.WithUserID("john@example.com"))
	if err != nil {
		log.Fatal(err)
	}

	ag, err := agent.NewAgent(agent.AgentConfig{
		Context:       ctx,
		Name:          "Memory Keeper",
		Model:         model,
		Instructions:  mt.Instructions,
		Tools:         []toolkit.Tool{mt},
		ShowToolsCall: true,
		Markdown:      true,
	})
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	ag.PrintResponse("My name is John, I live in Lisbon and I love hiking. Please remember that.", true, true)
	ag.PrintResponse("What do you remember about me?", true, true)
}
