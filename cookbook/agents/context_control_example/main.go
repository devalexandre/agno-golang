package main

import (
	"fmt"
	"log"
	"time"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ollama"
)

func main() {
	fmt.Println("=== Dependencies and Context Control Example ===\n")
	fmt.Println("This example demonstrates:")
	fmt.Println("  • WithDependencies - Pass external resources")
	fmt.Println("  • WithAddHistoryToContext - Control conversation history")
	fmt.Println("  • WithAddSessionStateToContext - Include session state\n")

	// 1. Setup dependencies (simulated external resources)
	fmt.Println("🔧 Setting up dependencies...")

	dependencies := map[string]interface{}{
		"api_key":      "sk-test-123",
		"environment":  "production",
		"service_name": "AgnoBot",
		"version":      "1.0.0",
		"max_retries":  3,
	}

	// 2. Create cloud model
	fmt.Println("🤖 Setting up cloud LLM...")
	model, err := ollama.NewOllamaChat(
		models.WithID("kimi-k2:1t-cloud"),
		models.WithBaseURL("https://ollama.com"),
	)
	if err != nil {
		log.Fatalf("Failed to create model: %v", err)
	}

	// 3. Create agent
	fmt.Println("🎯 Creating agent...")
	ag, err := agent.NewAgent(agent.AgentConfig{
		Name:        "Context-Aware Assistant",
		Model:       model,
		Description: "AI assistant demonstrating context control",
		Instructions: `You are a helpful assistant.
		You demonstrate different context control options.
		When appropriate, reference information from the conversation history and session state.`,
		Markdown:      true,
		ShowToolsCall: false,
	})
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	fmt.Println("\n✅ Agent created!")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")

	// Scenario 1: With conversation history (default)
	fmt.Println("--- Scenario 1: With History Context (Default) ---")

	response1a, err := ag.Run(
		"Remember this: My favorite programming language is Go.",
		agent.WithDependencies(dependencies),
		agent.WithAddHistoryToContext(true), // Include history
	)
	if err != nil {
		log.Printf("Error: %v", err)
	} else {
		fmt.Printf("\n👤 User: Remember this: My favorite programming language is Go.\n")
		fmt.Printf("🔧 Config: AddHistoryToContext=true\n")
		fmt.Printf("🤖 Assistant: %s\n", response1a.TextContent)
	}

	time.Sleep(500 * time.Millisecond)

	// Follow-up that requires history
	response1b, err := ag.Run(
		"What did I just tell you about my preferences?",
		agent.WithDependencies(dependencies),
		agent.WithAddHistoryToContext(true),
	)
	if err != nil {
		log.Printf("Error: %v", err)
	} else {
		fmt.Printf("\n👤 User: What did I just tell you about my preferences?\n")
		fmt.Printf("🔧 Config: AddHistoryToContext=true\n")
		fmt.Printf("🤖 Assistant: %s\n", response1b.TextContent)
		fmt.Printf("✅ Agent should remember: favorite language is Go\n")
	}

	// Scenario 2: Without history context
	fmt.Println("\n--- Scenario 2: Without History Context ---")

	response2, err := ag.Run(
		"What's my favorite programming language?",
		agent.WithDependencies(dependencies),
		agent.WithAddHistoryToContext(false), // NO history
	)
	if err != nil {
		log.Printf("Error: %v", err)
	} else {
		fmt.Printf("\n👤 User: What's my favorite programming language?\n")
		fmt.Printf("🔧 Config: AddHistoryToContext=false\n")
		fmt.Printf("🤖 Assistant: %s\n", response2.TextContent)
		fmt.Printf("💡 Agent should NOT know (no history access)\n")
	}

	// Scenario 3: With session state in context
	fmt.Println("\n--- Scenario 3: Session State in Context ---")

	sessionState := map[string]interface{}{
		"user_id":       "user_123",
		"current_page":  "/dashboard",
		"last_activity": time.Now().Unix(),
		"permissions":   []string{"read", "write", "delete"},
		"theme":         "dark",
	}

	response3, err := ag.Run(
		"What page am I currently on and what are my permissions?",
		agent.WithDependencies(dependencies),
		agent.WithSessionState(sessionState),
		agent.WithAddSessionStateToContext(true), // Include session state
	)
	if err != nil {
		log.Printf("Error: %v", err)
	} else {
		fmt.Printf("\n👤 User: What page am I currently on and what are my permissions?\n")
		fmt.Printf("🔧 Config: AddSessionStateToContext=true\n")
		fmt.Printf("📊 Session State: current_page=/dashboard, permissions=[read,write,delete]\n")
		fmt.Printf("🤖 Assistant: %s\n", response3.TextContent)
	}

	// Scenario 4: Dependencies in prompt context
	fmt.Println("\n--- Scenario 4: Dependencies in Context ---")

	response4, err := ag.Run(
		"What service am I using and in what environment?",
		agent.WithDependencies(dependencies),
		agent.WithAddDependenciesToContext(true), // Expose deps to prompt
	)
	if err != nil {
		log.Printf("Error: %v", err)
	} else {
		fmt.Printf("\n👤 User: What service am I using and in what environment?\n")
		fmt.Printf("🔧 Config: AddDependenciesToContext=true\n")
		fmt.Printf("📦 Dependencies: service_name=AgnoBot, environment=production, version=1.0.0\n")
		fmt.Printf("🤖 Assistant: %s\n", response4.TextContent)
	}

	// Scenario 5: Combining all context options
	fmt.Println("\n--- Scenario 5: All Context Options Combined ---")

	response5, err := ag.Run(
		"Give me a complete summary of our session: what I told you, where I am, and what service I'm using.",
		agent.WithDependencies(dependencies),
		agent.WithSessionState(sessionState),
		agent.WithAddHistoryToContext(true),
		agent.WithAddDependenciesToContext(true),
		agent.WithAddSessionStateToContext(true),
	)
	if err != nil {
		log.Printf("Error: %v", err)
	} else {
		fmt.Printf("\n👤 User: Give me a complete summary of our session\n")
		fmt.Printf("🔧 Config: ALL context options enabled\n")
		fmt.Printf("   • AddHistoryToContext = true\n")
		fmt.Printf("   • AddDependenciesToContext = true\n")
		fmt.Printf("   • AddSessionStateToContext = true\n")
		fmt.Printf("🤖 Assistant: %s\n", response5.TextContent)
		fmt.Printf("✅ Should include: Go preference, /dashboard page, AgnoBot service, permissions\n")
	}

	// Scenario 6: Stateless query (no context)
	fmt.Println("\n--- Scenario 6: Completely Stateless ---")

	response6, err := ag.Run(
		"Hello! Tell me about Go programming language.",
		agent.WithAddHistoryToContext(false),
		agent.WithAddDependenciesToContext(false),
		agent.WithAddSessionStateToContext(false),
	)
	if err != nil {
		log.Printf("Error: %v", err)
	} else {
		fmt.Printf("\n👤 User: Hello! Tell me about Go programming language.\n")
		fmt.Printf("🔧 Config: ALL context options disabled\n")
		fmt.Printf("🤖 Assistant: %s\n", response6.TextContent)
		fmt.Printf("💡 Fresh conversation - no prior context\n")
	}

	fmt.Println("\n=== Demo Complete ===")
	fmt.Println("\n✨ Key Features Demonstrated:")
	fmt.Println("   • WithDependencies - External resources (API keys, config)")
	fmt.Println("   • WithAddHistoryToContext - Control conversation memory")
	fmt.Println("   • WithAddDependenciesToContext - Expose deps to LLM")
	fmt.Println("   • WithAddSessionStateToContext - Include session data")
	fmt.Println("\n💡 Use Cases:")
	fmt.Println("   • Stateful vs stateless conversations")
	fmt.Println("   • Session-aware responses (current page, permissions)")
	fmt.Println("   • Environment-specific behavior (dev/prod)")
	fmt.Println("   • Privacy-conscious contexts (disable history)")
}
