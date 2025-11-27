package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ollama"
	"github.com/devalexandre/agno-golang/agno/storage/sqlite"
	"github.com/devalexandre/agno-golang/agno/utils"
)

func main() {
	ctx := context.Background()

	// Enable markdown
	utils.SetMarkdownMode(true)

	// Create Ollama model
	model, err := ollama.NewOllamaChat(
		models.WithID("llama3.2:latest"),
		models.WithBaseURL("http://localhost:11434"),
	)
	if err != nil {
		log.Fatal(err)
	}

	// Create SQLite storage for conversation history
	dbFile := "agent_storage.db"
	db, err := sqlite.NewSqliteStorage(sqlite.SqliteStorageConfig{
		TableName: "agno_sessions",
		DBFile:    &dbFile,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// Create agent with storage
	ag, err := agent.NewAgent(agent.AgentConfig{
		Context:   ctx,
		Model:     model,
		Name:      "PersonalAssistant",
		SessionID: "user_123_session_1",
		UserID:    "user_123",
		Instructions: `You are a helpful personal assistant! 📝
You can remember information from our conversations and help with various tasks.

Your capabilities:
- Remember user preferences and information
- Track conversation history
- Provide personalized responses based on past interactions
- Help with planning and organization`,
		Storage:  db,
		Markdown: true,
		Debug:    false,
	})
	if err != nil {
		log.Fatal(err)
	}

	// First interaction
	utils.InfoPanel("First Interaction:")
	response1, err := ag.Run(
		"My name is Alex and I love programming in Go. Can you remember that?",
		agent.WithAddHistoryToContext(true),
	)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("🤖 %s\n\n", response1.TextContent)

	time.Sleep(1 * time.Second)

	// Second interaction - agent should remember
	utils.InfoPanel("Second Interaction (with history):")
	response2, err := ag.Run(
		"What's my name and what do I like?",
		agent.WithAddHistoryToContext(true),
	)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("🤖 %s\n\n", response2.TextContent)

	time.Sleep(1 * time.Second)

	// Third interaction - continuing the conversation
	utils.InfoPanel("Third Interaction:")
	response3, err := ag.Run(
		"Can you suggest a Go project for me based on what you know about me?",
		agent.WithAddHistoryToContext(true),
	)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("🤖 %s\n\n", response3.TextContent)

	// Show storage info
	utils.SuccessPanel(fmt.Sprintf("Conversation history persisted to: %s", dbFile))
}
