package main

import (
	"fmt"
	"log"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ollama"
	"github.com/devalexandre/agno-golang/agno/storage/sqlite"
)

func main() {
	fmt.Println("=== Session State Example ===\n")
	fmt.Println("This example demonstrates session state persistence across multiple conversations.")
	fmt.Println("Session state allows maintaining context between separate agent runs.\n")

	// 1. Setup storage for session persistence
	dbFile := "session_state.db"
	db, err := sqlite.NewSqliteStorage(sqlite.SqliteStorageConfig{
		TableName: "agno_sessions",
		DBFile:    &dbFile,
	})
	if err != nil {
		log.Fatalf("Failed to create storage: %v", err)
	}
	defer db.Close()

	// 2. Create cloud model
	fmt.Println("🤖 Setting up cloud LLM...")
	model, err := ollama.NewOllamaChat(
		models.WithID("kimi-k2:1t-cloud"),
		models.WithBaseURL("https://ollama.com"),
	)
	if err != nil {
		log.Fatalf("Failed to create model: %v", err)
	}

	// 3. Create agent with storage
	fmt.Println("🎯 Creating agent with session state...")
	ag, err := agent.NewAgent(agent.AgentConfig{
		Name:          "Session Assistant",
		Model:         model,
		Description:   "AI assistant that remembers session context",
		Instructions:  "You are a helpful assistant. Use session state to remember user preferences and context across conversations.",
		Storage:       db,
		Markdown:      true,
		ShowToolsCall: false,
	})
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	fmt.Println("\n✅ Agent created with session state support!")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")

	// Session 1: User introduces themselves
	sessionID := "user_123_session"
	fmt.Println("--- Session 1: Initial Conversation ---")

	// Initial session state
	sessionState := map[string]interface{}{
		"user_name":          "",
		"preferences":        map[string]interface{}{},
		"conversation_count": 0,
	}

	response1, err := ag.Run(
		"Hi! My name is Alex and I prefer technical explanations with code examples.",
		agent.WithSessionID(sessionID),
		agent.WithSessionState(sessionState),
	)
	if err != nil {
		log.Fatalf("Agent error: %v", err)
	}

	fmt.Printf("\n👤 User: Hi! My name is Alex and I prefer technical explanations with code examples.\n")
	fmt.Printf("🤖 Assistant: %s\n", response1.TextContent)

	// Update session state
	sessionState["user_name"] = "Alex"
	sessionState["preferences"] = map[string]interface{}{
		"style":        "technical",
		"include_code": true,
	}
	sessionState["conversation_count"] = 1

	// Session 2: Ask a question (same session)
	fmt.Println("\n--- Session 2: Question with Context ---")

	response2, err := ag.Run(
		"Can you explain what a channel is?",
		agent.WithSessionID(sessionID),
		agent.WithSessionState(sessionState),
		agent.WithAddSessionStateToContext(true), // Include session state in context
	)
	if err != nil {
		log.Fatalf("Agent error: %v", err)
	}

	fmt.Printf("\n👤 User: Can you explain what a channel is?\n")
	fmt.Printf("🤖 Assistant: %s\n", response2.TextContent)

	// Update conversation count
	sessionState["conversation_count"] = 2
	sessionState["last_topic"] = "Go channels"

	// Session 3: Follow-up question
	fmt.Println("\n--- Session 3: Follow-up with Memory ---")

	response3, err := ag.Run(
		"What about buffered versions?",
		agent.WithSessionID(sessionID),
		agent.WithSessionState(sessionState),
		agent.WithAddHistoryToContext(true), // Include conversation history
		agent.WithAddSessionStateToContext(true),
	)
	if err != nil {
		log.Fatalf("Agent error: %v", err)
	}

	fmt.Printf("\n👤 User: What about buffered versions?\n")
	fmt.Printf("🤖 Assistant: %s\n", response3.TextContent)

	// Session 4: New session (different user)
	fmt.Println("\n--- Session 4: New User (Different Session) ---")

	newSessionID := "user_456_session"
	newSessionState := map[string]interface{}{
		"user_name": "Jordan",
		"preferences": map[string]interface{}{
			"style":        "simple",
			"include_code": false,
		},
		"conversation_count": 0,
	}

	response4, err := ag.Run(
		"What are Go channels?",
		agent.WithSessionID(newSessionID),
		agent.WithSessionState(newSessionState),
	)
	if err != nil {
		log.Fatalf("Agent error: %v", err)
	}

	fmt.Printf("\n👤 User (Jordan): What are Go channels?\n")
	fmt.Printf("🤖 Assistant: %s\n", response4.TextContent)

	// Session 5: Return to original session
	fmt.Println("\n--- Session 5: Back to Alex's Session ---")

	sessionState["conversation_count"] = 3

	response5, err := ag.Run(
		"Remember what we discussed? Can you summarize?",
		agent.WithSessionID(sessionID),
		agent.WithSessionState(sessionState),
		agent.WithAddHistoryToContext(true),
		agent.WithAddSessionStateToContext(true),
	)
	if err != nil {
		log.Fatalf("Agent error: %v", err)
	}

	fmt.Printf("\n👤 User (Alex): Remember what we discussed? Can you summarize?\n")
	fmt.Printf("🤖 Assistant: %s\n", response5.TextContent)

	fmt.Println("\n=== Demo Complete ===")
	fmt.Println("\n✨ Key Features Demonstrated:")
	fmt.Println("   • WithSessionID - Separate conversation threads")
	fmt.Println("   • WithSessionState - Persist custom data between runs")
	fmt.Println("   • WithAddSessionStateToContext - Include state in prompt")
	fmt.Println("   • WithAddHistoryToContext - Include conversation history")
	fmt.Println("   • Multi-user sessions with isolated contexts")
	fmt.Printf("\n💾 Session data persisted to: %s\n", dbFile)
}
