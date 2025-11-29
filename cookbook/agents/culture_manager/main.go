package main

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/culture"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ollama"
)

func main() {
	ctx := context.Background()

	fmt.Println("=== Culture Manager Example ===\n")

	// Create AI model
	model, err := ollama.NewOllamaChat(
		models.WithID("llama3.2:latest"),
		models.WithBaseURL("http://localhost:11434"),
	)
	if err != nil {
		log.Fatal(err)
	}

	// Create Culture Manager
	cultureManager := culture.NewCultureManager(culture.CultureManagerConfig{
		Enabled: true,
	})

	// Simulate storing cultural knowledge for a user
	userID := "user123"

	fmt.Println("📚 Storing cultural knowledge...")
	err = cultureManager.UpdateCulturalKnowledge(ctx, userID, map[string]interface{}{
		"preferred_language":  "Portuguese (Brazil)",
		"timezone":            "America/Sao_Paulo",
		"communication_style": "friendly and informal",
		"interests":           []string{"technology", "AI", "Go programming"},
		"previous_topics":     []string{"microservices", "database design"},
	})
	if err != nil {
		log.Fatalf("Failed to update cultural knowledge: %v", err)
	}
	fmt.Println("✅ Cultural knowledge stored\n")

	// Retrieve cultural knowledge
	fmt.Println("🔍 Retrieving cultural knowledge...")
	knowledge, err := cultureManager.GetCulturalKnowledge(ctx, userID)
	if err != nil {
		log.Fatalf("Failed to get cultural knowledge: %v", err)
	}
	fmt.Printf("✅ Retrieved knowledge for user: %s\n", knowledge.UserID)
	fmt.Printf("   Knowledge items: %d\n\n", len(knowledge.Knowledge))

	// Create agent WITH culture manager
	fmt.Println("🤖 Creating agent with Culture Manager...")
	culturalAgent, err := agent.NewAgent(agent.AgentConfig{
		Context:              ctx,
		Name:                 "Cultural Assistant",
		Model:                model,
		UserID:               userID, // Important: set the UserID!
		Instructions:         "You are a helpful assistant that adapts to user preferences and cultural context.",
		CultureManager:       cultureManager,
		EnableAgenticCulture: true,
		AddCultureToContext:  true,
		Debug:                true, // Enable debug to see the cultural context
	})
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}
	fmt.Println("✅ Agent created with cultural awareness\n")

	// Create agent WITHOUT culture manager for comparison
	fmt.Println("🤖 Creating standard agent (no culture)...")
	standardAgent, err := agent.NewAgent(agent.AgentConfig{
		Context:      ctx,
		Name:         "Standard Assistant",
		Model:        model,
		Instructions: "You are a helpful assistant.",
		Debug:        false,
	})
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}
	fmt.Println("✅ Standard agent created\n")

	// Test both agents
	testQuery := "What topics should we discuss today?"

	fmt.Println(strings.Repeat("=", 60))
	fmt.Println("\n📝 Testing STANDARD agent (no cultural context)...")
	fmt.Println("Query:", testQuery)
	response1, err := standardAgent.Run(testQuery)
	if err != nil {
		log.Printf("Error: %v", err)
	} else {
		fmt.Println("\nResponse:", response1.TextContent)
	}

	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("\n📝 Testing CULTURAL agent (with cultural context)...")
	fmt.Println("Query:", testQuery)
	fmt.Println("\nNote: The agent has access to:")
	fmt.Println("  - Preferred language: Portuguese (Brazil)")
	fmt.Println("  - Interests: technology, AI, Go programming")
	fmt.Println("  - Previous topics: microservices, database design")
	fmt.Println("  - Communication style: friendly and informal")

	response2, err := culturalAgent.Run(testQuery)
	if err != nil {
		log.Printf("Error: %v", err)
	} else {
		fmt.Println("\nResponse:", response2.TextContent)
	}

	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("\n💡 Notice how the cultural agent:")
	fmt.Println("  ✓ Adapts to user preferences")
	fmt.Println("  ✓ References previous topics")
	fmt.Println("  ✓ Uses appropriate communication style")
	fmt.Println("  ✓ Provides more personalized responses")
}
