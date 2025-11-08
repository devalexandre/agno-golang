package main

import (
	"context"
	"fmt"
	"log"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/memory"
	memorysqlite "github.com/devalexandre/agno-golang/agno/memory/sqlite"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ollama"
)

func main() {
	ctx := context.Background()

	fmt.Println("🧠 Memory-enabled Agent Example")
	fmt.Println("================================")
	fmt.Println("")

	// 1. Setup SQLite database for memory persistence
	fmt.Println("💾 Setting up SQLite memory database...")
	dbFile := "agent_memory.db"
	memoryDB, err := memorysqlite.NewSqliteMemoryDb("user_memories", dbFile)
	if err != nil {
		log.Fatalf("Failed to create memory database: %v", err)
	}

	// 2. Create cloud LLM model for the agent
	fmt.Println("🤖 Setting up cloud LLM...")
	cloudModel, err := ollama.NewOllamaChat(
		models.WithID("kimi-k2:1t-cloud"),
		models.WithBaseURL("https://ollama.com"),
	)
	if err != nil {
		log.Fatalf("Failed to create model: %v", err)
	}

	// 3. Create Basic Memory (compatible with agent)
	fmt.Println("🧠 Creating memory manager...")
	memoryManager := memory.NewMemory(cloudModel, memoryDB)

	// 4. Create agent with memory
	fmt.Println("🎯 Creating memory-enabled agent...")
	agt, err := agent.NewAgent(agent.AgentConfig{
		Name:        "Memory Assistant",
		Model:       cloudModel,
		Description: "An AI assistant with persistent memory that remembers conversations",
		Instructions: "You are a helpful assistant with memory. " +
			"You can remember previous conversations and user preferences. " +
			"When users mention something personal, remember it for future conversations. " +
			"Reference past conversations when relevant.",
		Memory:        memoryManager,
		Markdown:      true,
		ShowToolsCall: false,
	})
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	fmt.Println("\n✅ Agent created with memory enabled!")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("")

	// User ID for this session
	userID := "user_alexandre"

	// 5. Conversation 1: Share personal information
	fmt.Println("💬 Conversation 1: Introducing myself")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	messages := []struct {
		text   string
		should bool // should create memory
	}{
		{"Hi! My name is Alexandre and I'm a software developer from Brazil.", true},
		{"I love working with Go and building AI applications.", true},
		{"My favorite programming paradigm is functional programming.", true},
		{"What's 2+2?", false}, // Just a question, no memory needed
	}

	for _, msg := range messages {
		fmt.Printf("\n👤 User: %s\n", msg.text)

		// Run the agent
		response, err := agt.Run(msg.text)
		if err != nil {
			log.Printf("Error: %v", err)
			continue
		}
		fmt.Printf("🤖 Assistant: %s\n", response.TextContent)

		// Create and store memory for important messages
		if msg.should {
			memory, err := memoryManager.CreateMemory(ctx, userID, msg.text, response.TextContent)
			if err != nil {
				log.Printf("Error creating memory: %v", err)
			} else if memory != nil {
				fmt.Printf("   💾 Memory stored: %s\n", memory.Memory)
			}
		}
	}

	// 6. Display stored memories
	fmt.Println("\n\n📊 Stored Memories")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	storedMemories, err := memoryManager.GetUserMemories(ctx, userID)
	if err != nil {
		log.Printf("Failed to get memories: %v", err)
	} else {
		fmt.Printf("\n📝 Total memories for %s: %d\n", userID, len(storedMemories))
		for i, mem := range storedMemories {
			fmt.Printf("\n%d. %s\n", i+1, mem.Memory)
			fmt.Printf("   From input: %s\n", mem.Input)
			fmt.Printf("   Created: %s\n", mem.CreatedAt.Format("2006-01-02 15:04:05"))
		}
	}

	// 7. Conversation 2: Test memory recall
	fmt.Println("\n\n� Conversation 2: Testing memory recall")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	// Get existing memories to add context
	existingMemories, err := memoryManager.GetUserMemories(ctx, userID)
	if err != nil {
		log.Printf("Failed to get existing memories: %v", err)
	}

	recallQueries := []string{
		"What's my name?",
		"What do you know about my programming interests?",
		"What country am I from?",
	}

	for _, query := range recallQueries {
		fmt.Printf("\n👤 User: %s\n", query)

		// Build context from memories
		contextPrompt := query
		if len(existingMemories) > 0 {
			memoryContext := "\n\nWhat I remember about you:\n"
			for _, mem := range existingMemories {
				memoryContext += fmt.Sprintf("- %s\n", mem.Memory)
			}
			contextPrompt = memoryContext + "\nQuestion: " + query
		}

		response, err := agt.Run(contextPrompt)
		if err != nil {
			log.Printf("Error: %v", err)
			continue
		}
		fmt.Printf("🤖 Assistant: %s\n", response.TextContent)
	}

	// 8. Test memory summarization
	fmt.Println("\n\n📄 Memory Summarization")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	if len(storedMemories) > 0 {
		fmt.Println("\nAll memories combined:")
		for i, mem := range storedMemories {
			fmt.Printf("%d. %s\n", i+1, mem.Memory)
		}
		fmt.Println("\n💡 These memories will be used in future conversations to provide personalized responses")
	}

	fmt.Println("\n\n✨ Memory example completed!")
	fmt.Printf("💾 Memory persisted to: %s\n", dbFile)
	fmt.Println("\n💡 Tip: Run this example again to see how the agent remembers previous conversations!")
}
