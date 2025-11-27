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

	// Create storage for session management
	dbFile := "agent_sessions.db"
	db, err := sqlite.NewSqliteStorage(sqlite.SqliteStorageConfig{
		TableName: "agno_sessions",
		DBFile:    &dbFile,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// Session 1: First conversation
	utils.InfoPanel("=== Session 1: First Conversation ===")
	session1, err := agent.NewAgent(agent.AgentConfig{
		Context:   ctx,
		Model:     model,
		Name:      "ShoppingAssistant",
		SessionID: "shopping_session_001",
		UserID:    "customer_alex",
		Storage:   db,
		Instructions: `You are a helpful shopping assistant! 🛍️

Help customers find products, track their shopping cart, and provide recommendations.
Remember what they're interested in and provide personalized suggestions.`,
		Markdown: true,
		Debug:    false,
	})
	if err != nil {
		log.Fatal(err)
	}

	response1, err := session1.Run(
		"I'm looking for a new laptop for programming. What do you recommend?",
		agent.WithAddHistoryToContext(true),
	)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("🤖 %s\n\n", response1.TextContent)
	time.Sleep(1 * time.Second)

	response2, err := session1.Run(
		"I prefer something with at least 16GB RAM and good battery life.",
		agent.WithAddHistoryToContext(true),
	)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("🤖 %s\n\n", response2.TextContent)
	time.Sleep(1 * time.Second)

	// Session 2: Continue the same conversation later
	utils.InfoPanel("\n=== Session 2: Continuing Previous Conversation ===")
	session2, err := agent.NewAgent(agent.AgentConfig{
		Context:   ctx,
		Model:     model,
		Name:      "ShoppingAssistant",
		SessionID: "shopping_session_001", // Same session ID
		UserID:    "customer_alex",
		Storage:   db,
		Instructions: `You are a helpful shopping assistant! 🛍️

Help customers find products, track their shopping cart, and provide recommendations.
Remember what they're interested in and provide personalized suggestions.`,
		Markdown: true,
		Debug:    false,
	})
	if err != nil {
		log.Fatal(err)
	}

	// Agent should remember the previous conversation
	response3, err := session2.Run(
		"What was I looking for again? And what were my requirements?",
		agent.WithAddHistoryToContext(true),
	)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("🤖 %s\n\n", response3.TextContent)
	time.Sleep(1 * time.Second)

	// New session with same user
	utils.InfoPanel("\n=== Session 3: New Shopping Session (Same User) ===")
	session3, err := agent.NewAgent(agent.AgentConfig{
		Context:   ctx,
		Model:     model,
		Name:      "ShoppingAssistant",
		SessionID: "shopping_session_002", // Different session ID
		UserID:    "customer_alex",        // Same user
		Storage:   db,
		Instructions: `You are a helpful shopping assistant! 🛍️

Help customers find products, track their shopping cart, and provide recommendations.
Remember what they're interested in and provide personalized suggestions.`,
		Markdown: true,
		Debug:    false,
	})
	if err != nil {
		log.Fatal(err)
	}

	response4, err := session3.Run(
		"Now I need a wireless mouse. Any suggestions?",
		agent.WithAddHistoryToContext(true),
	)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("🤖 %s\n\n", response4.TextContent)

	// Show session statistics
	utils.SuccessPanel(fmt.Sprintf(`Session Management Demo Complete!

User: customer_alex
Sessions: shopping_session_001, shopping_session_002
Storage: %s

Each session maintains its own conversation history while sharing the same user ID.`, dbFile))
}
