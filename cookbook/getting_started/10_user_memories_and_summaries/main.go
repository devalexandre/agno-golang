package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/memory"
	memorysqlite "github.com/devalexandre/agno-golang/agno/memory/sqlite"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ollama"
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

	// Create SQLite database for memory persistence
	dbFile := "agent_memory.db"
	memoryDB, err := memorysqlite.NewSqliteMemoryDb("user_memories", dbFile)
	if err != nil {
		log.Fatal(err)
	}

	// Create memory manager
	memoryManager := memory.NewMemory(model, memoryDB)

	// Create agent with memory capabilities
	ag, err := agent.NewAgent(agent.AgentConfig{
		Context:   ctx,
		Model:     model,
		Name:      "PersonalCoach",
		UserID:    "user_alex_123",
		SessionID: "coaching_session_1",
		Instructions: `You are a personal fitness and wellness coach! 💪

You remember important details about your clients including:
- Their fitness goals and progress
- Health conditions and limitations
- Dietary preferences and restrictions
- Personal preferences and motivations
- Past achievements and challenges

Use this information to provide personalized, encouraging advice and track their journey.`,
		Memory:   memoryManager,
		Markdown: true,
		Debug:    false,
	})
	if err != nil {
		log.Fatal(err)
	}

	userID := "user_alex_123"

	// First interaction - agent learns about the user
	utils.InfoPanel("=== First Session: Getting to Know You ===")
	prompt1 := "Hi! I'm Alex. I want to start working out. I'm 30 years old, work in tech, and I'm a vegetarian. I have a knee injury from 2 years ago that sometimes bothers me. My goal is to lose 10kg and build strength."

	response1, err := ag.Run(prompt1)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("👤 User: %s\n\n", prompt1)
	fmt.Printf("🤖 Coach: %s\n\n", response1.TextContent)

	// Create memory from this interaction
	mem1, err := memoryManager.CreateMemory(ctx, userID, prompt1, response1.TextContent)
	if err == nil && mem1 != nil {
		utils.InfoPanel(fmt.Sprintf("💾 Memory stored: %s", mem1.Memory))
	}
	time.Sleep(2 * time.Second)

	// Second interaction - agent should remember
	utils.InfoPanel("\n=== Second Session: Workout Plan ===")
	prompt2 := "Can you create a workout plan for me considering my knee issue?"

	// Get existing memories to add context
	existingMemories, _ := memoryManager.GetUserMemories(ctx, userID)
	contextPrompt := prompt2
	if len(existingMemories) > 0 {
		memoryContext := "\n\nWhat I remember about you:\n"
		for _, mem := range existingMemories {
			memoryContext += fmt.Sprintf("- %s\n", mem.Memory)
		}
		contextPrompt = memoryContext + "\nQuestion: " + prompt2
	}

	response2, err := ag.Run(contextPrompt)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("👤 User: %s\n\n", prompt2)
	fmt.Printf("🤖 Coach: %s\n\n", response2.TextContent)

	mem2, err := memoryManager.CreateMemory(ctx, userID, prompt2, response2.TextContent)
	if err == nil && mem2 != nil {
		utils.InfoPanel(fmt.Sprintf("💾 Memory stored: %s", mem2.Memory))
	}
	time.Sleep(2 * time.Second)

	// Third interaction - dietary advice
	utils.InfoPanel("\n=== Third Session: Nutrition ===")
	prompt3 := "What should I eat to support my fitness goals?"

	existingMemories, _ = memoryManager.GetUserMemories(ctx, userID)
	contextPrompt = prompt3
	if len(existingMemories) > 0 {
		memoryContext := "\n\nWhat I remember about you:\n"
		for _, mem := range existingMemories {
			memoryContext += fmt.Sprintf("- %s\n", mem.Memory)
		}
		contextPrompt = memoryContext + "\nQuestion: " + prompt3
	}

	response3, err := ag.Run(contextPrompt)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("👤 User: %s\n\n", prompt3)
	fmt.Printf("🤖 Coach: %s\n\n", response3.TextContent)

	// Show stored memories
	memories, err := memoryManager.GetUserMemories(ctx, userID)
	if err == nil && len(memories) > 0 {
		utils.SuccessPanel(fmt.Sprintf("Stored %d memories about the user:", len(memories)))
		for i, mem := range memories {
			utils.InfoPanel(fmt.Sprintf("Memory %d: %s\nCreated: %s", i+1, mem.Memory, mem.CreatedAt.Format("2006-01-02 15:04:05")))
		}
	}

	utils.SuccessPanel(fmt.Sprintf("Memory persisted to: %s", dbFile))
}
