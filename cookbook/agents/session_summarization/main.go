package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/memory"
	memorysqlite "github.com/devalexandre/agno-golang/agno/memory/sqlite"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ollama"
)

func main() {
	ctx := context.Background()

	fmt.Println("📝 Session Summarization Example")
	fmt.Println("=================================")
	fmt.Println("")

	// Get API key from environment
	apiKey := os.Getenv("OLLAMA_API_KEY")
	if apiKey == "" {
		log.Fatal("OLLAMA_API_KEY environment variable is required")
	}

	// 1. Setup SQLite database for memory persistence
	fmt.Println("💾 Setting up SQLite memory database...")
	dbFile := "session_summary.db"
	memoryDB, err := memorysqlite.NewSqliteMemoryDb("user_memories", dbFile)
	if err != nil {
		log.Fatalf("Failed to create memory database: %v", err)
	}

	// 2. Create Ollama Cloud model
	fmt.Println("🤖 Setting up Ollama Cloud model...")
	ollamaModel, err := ollama.NewOllamaChat(
		models.WithID("kimi-k2:1t-cloud"),
		models.WithBaseURL("https://ollama.com"),
		models.WithAPIKey(apiKey),
	)
	if err != nil {
		log.Fatalf("Failed to create Ollama Cloud model: %v", err)
	}

	// 3. Create Memory Manager
	fmt.Println("🧠 Creating memory manager...")
	memoryManager := memory.NewMemory(ollamaModel, memoryDB)

	// 4. Create agent with memory
	fmt.Println("🎯 Creating agent...")
	agt, err := agent.NewAgent(agent.AgentConfig{
		Name:         "Session Assistant",
		Model:        ollamaModel,
		Description:  "An AI assistant that summarizes conversation sessions",
		Instructions: "You are a helpful assistant. Engage in natural conversations with users.",
		Memory:       memoryManager,
		Markdown:     false,
		Debug:        false,
	})
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	fmt.Println("\n✅ Agent created successfully!")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("")

	// User and session IDs
	userID := "user_123"
	sessionID := "session_001"

	// 5. Simulate a conversation session
	fmt.Println("💬 Starting conversation session...")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	conversationMessages := []map[string]interface{}{}

	conversations := []struct {
		user      string
		assistant string
	}{
		{
			user:      "Hi! I need help planning a trip to Japan.",
			assistant: "I'd be happy to help you plan your trip to Japan! When are you planning to go?",
		},
		{
			user:      "I'm thinking about going in April for the cherry blossom season.",
			assistant: "April is a wonderful time to visit Japan for cherry blossoms! The peak bloom is usually mid to late April. Which cities are you interested in visiting?",
		},
		{
			user:      "I want to visit Tokyo, Kyoto, and maybe Osaka.",
			assistant: "Excellent choices! Tokyo offers modern attractions, Kyoto has beautiful temples and traditional culture, and Osaka is known for its food scene. I recommend spending 3-4 days in Tokyo, 2-3 days in Kyoto, and 1-2 days in Osaka.",
		},
		{
			user:      "What about accommodation? Any recommendations?",
			assistant: "For accommodation, I recommend staying in areas like Shinjuku or Shibuya in Tokyo for convenience. In Kyoto, the Gion district offers a traditional experience. For Osaka, Namba or Umeda are great central locations. Consider booking early as April is peak season.",
		},
		{
			user:      "Thanks! What should I budget for this trip?",
			assistant: "For a 7-10 day trip to Japan, budget approximately $2,000-3,000 per person including flights, accommodation, food, and activities. This can vary based on your travel style. The JR Pass is highly recommended for intercity travel.",
		},
	}

	for i, conv := range conversations {
		fmt.Printf("\n[Turn %d]\n", i+1)
		fmt.Printf("👤 User: %s\n", conv.user)
		fmt.Printf("🤖 Assistant: %s\n", conv.assistant)

		// Add to conversation history
		conversationMessages = append(conversationMessages,
			map[string]interface{}{
				"role":    "user",
				"content": conv.user,
			},
			map[string]interface{}{
				"role":    "assistant",
				"content": conv.assistant,
			},
		)
	}

	// 6. Create session summary
	fmt.Println("\n\n📊 Creating Session Summary")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	summary, err := memoryManager.CreateSessionSummary(ctx, userID, sessionID, conversationMessages)
	if err != nil {
		log.Fatalf("Failed to create session summary: %v", err)
	}

	fmt.Printf("\n✅ Session Summary Created:\n")
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	fmt.Printf("User ID: %s\n", summary.UserID)
	fmt.Printf("Session ID: %s\n", summary.SessionID)
	fmt.Printf("Created: %s\n\n", summary.CreatedAt.Format("2006-01-02 15:04:05"))
	fmt.Printf("Summary:\n%s\n", summary.Summary)

	// 7. Retrieve session summary
	fmt.Println("\n\n🔍 Retrieving Session Summary")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	retrievedSummary, err := memoryManager.GetSessionSummary(ctx, userID, sessionID)
	if err != nil {
		log.Fatalf("Failed to retrieve session summary: %v", err)
	}

	if retrievedSummary != nil {
		fmt.Printf("\n📝 Retrieved Summary:\n")
		fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
		fmt.Printf("%s\n", retrievedSummary.Summary)
	}

	// 8. Demonstrate using summary in new conversation
	fmt.Println("\n\n💡 Using Summary in New Conversation")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	newQuery := "What did we discuss in our last conversation?"
	fmt.Printf("\n👤 User: %s\n", newQuery)

	// Build context with summary
	contextPrompt := fmt.Sprintf(`Previous conversation summary:
%s

Current question: %s`, retrievedSummary.Summary, newQuery)

	response, err := agt.Run(contextPrompt)
	if err != nil {
		log.Fatalf("Error: %v", err)
	}
	fmt.Printf("🤖 Assistant: %s\n", response.TextContent)

	fmt.Println("\n\n✨ Session Summarization example completed!")
	fmt.Printf("💾 Data persisted to: %s\n", dbFile)
	fmt.Println("\n💡 Benefits of Session Summarization:")
	fmt.Println("   • Reduces context window usage")
	fmt.Println("   • Maintains conversation continuity")
	fmt.Println("   • Enables long-term memory")
	fmt.Println("   • Improves response relevance")
}
