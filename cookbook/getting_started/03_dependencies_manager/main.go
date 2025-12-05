package main

import (
	"context"
	"fmt"
	"log"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ollama"
)

// UserContext represents user context with dependencies
type UserContext struct {
	UserID   string
	Role     string
	Database string
	APIKey   string
}

func main() {
	ctx := context.Background()

	// Create an Ollama model
	model, err := ollama.NewOllamaChat(
		models.WithID("llama3.2:latest"),
		models.WithBaseURL("http://localhost:11434"),
	)
	if err != nil {
		log.Fatal(err)
	}

	// Example 1: Simple dependencies
	fmt.Println("=== Example 1: Simple Dependencies ===")
	simpleDeps := map[string]interface{}{
		"user_id":      "user_123",
		"api_key":      "secret_key_xyz",
		"database_url": "postgres://localhost:5432/mydb",
	}

	fmt.Printf("✅ Dependencies setup:\n")
	for key, value := range simpleDeps {
		fmt.Printf("   %s: %v\n", key, value)
	}

	// Example 2: Dynamic dependencies (computed at runtime)
	fmt.Println("\n=== Example 2: Dynamic Dependencies ===")
	dynamicDeps := map[string]interface{}{
		"current_timestamp": "2025-12-04T15:30:00Z",
		"user_role":         "admin",
		"environment":       "production",
	}

	fmt.Printf("✅ Dynamic dependencies:\n")
	for key, value := range dynamicDeps {
		fmt.Printf("   %s: %v\n", key, value)
	}

	// Example 3: Merging dependencies
	fmt.Println("\n=== Example 3: Merged Dependencies ===")
	mergedDeps := make(map[string]interface{})
	for k, v := range simpleDeps {
		mergedDeps[k] = v
	}
	for k, v := range dynamicDeps {
		mergedDeps[k] = v
	}

	fmt.Printf("✅ Total merged dependencies: %d\n", len(mergedDeps))

	// Example 4: Use with Agent (dependencies NOT added to context)
	fmt.Println("\n=== Example 4: Agent with Dependencies (not in context) ===")

	ag1, err := agent.NewAgent(agent.AgentConfig{
		Context:                  ctx,
		Model:                    model,
		Name:                     "DependencyAwareAssistant1",
		Dependencies:             simpleDeps,
		AddDependenciesToContext: false, // Dependencies not visible to model
		Instructions: `You are an assistant that works with user context.
Provide helpful responses based on the user's request.
Be friendly and professional.`,
		Debug: false,
	})
	if err != nil {
		log.Fatal(err)
	}

	userContext := UserContext{
		UserID:   "user_123",
		Role:     "admin",
		Database: "production_db",
		APIKey:   "secret_key",
	}

	prompt1 := fmt.Sprintf(
		"I'm working with user %s who has role %s. The database is %s. Give me a brief tip.",
		userContext.UserID,
		userContext.Role,
		userContext.Database,
	)

	fmt.Printf("📝 Agent 1 - Dependencies set but NOT in context\n")
	fmt.Printf("   Prompt: %s\n", prompt1[:80]+"...")
	_ = ag1 // Dependencies are available to tools/hooks, not shown to model

	// Example 5: Use with Agent (dependencies added to context)
	fmt.Println("\n=== Example 5: Agent with Dependencies (in context) ===")

	ag2, err := agent.NewAgent(agent.AgentConfig{
		Context: ctx,
		Model:   model,
		Name:    "DependencyAwareAssistant2",
		Dependencies: map[string]interface{}{
			"company":  "TechCorp",
			"project":  "AI Platform",
			"deadline": "2025-12-31",
		},
		AddDependenciesToContext: true, // Dependencies visible to model
		Instructions: `You are a project assistant.
Use the provided company context and project information to give relevant advice.`,
		Debug: false,
	})
	if err != nil {
		log.Fatal(err)
	}

	prompt2 := "What are the key considerations for managing this project?"
	fmt.Printf("📝 Agent 2 - Dependencies added to system context\n")
	fmt.Printf("   Dependencies visible to model as context\n")
	fmt.Printf("   Prompt: %s\n\n", prompt2)
	_ = ag2

	// Example 6: How dependencies work (like Python)
	fmt.Println("=== Example 6: How Dependencies Work ===")
	fmt.Println("In Go (matching Python agno.Agent):")
	fmt.Println("1. Dependencies field: map[string]interface{}")
	fmt.Println("2. AddDependenciesToContext: bool (default false)")
	fmt.Println("3. If AddDependenciesToContext is true:")
	fmt.Println("   - Dependencies are serialized to JSON")
	fmt.Println("   - Added to system message as context")
	fmt.Println("   - Model can use them in reasoning")
	fmt.Println("4. Tools always have access to dependencies")
	fmt.Println("5. This matches Python's agent.run(dependencies=...)")
}
