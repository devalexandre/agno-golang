package main

import (
	"fmt"
	"log"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ollama"
)

func main() {
	fmt.Println("=== Retry Example ===\n")
	fmt.Println("This example demonstrates:")
	fmt.Println("  • WithRetries - Automatic retry on failures")
	fmt.Println("  • Resilience against transient errors")
	fmt.Println("  • Use cases: network issues, rate limits, temporary outages\n")

	// Create cloud model
	fmt.Println("🤖 Setting up cloud LLM...")
	model, err := ollama.NewOllamaChat(
		models.WithID("kimi-k2:1t-cloud"),
		models.WithBaseURL("https://ollama.com"),
	)
	if err != nil {
		log.Fatalf("Failed to create model: %v", err)
	}

	// Create agent
	fmt.Println("🎯 Creating agent...")
	ag, err := agent.NewAgent(agent.AgentConfig{
		Name:          "Resilient Assistant",
		Model:         model,
		Description:   "AI assistant with retry capability",
		Instructions:  "You are a helpful assistant. Answer questions concisely.",
		Markdown:      true,
		ShowToolsCall: false,
	})
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	fmt.Println("\n✅ Agent created!")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")

	// Scenario 1: Normal request without retries (default)
	fmt.Println("--- Scenario 1: No Retries (Default) ---")

	response1, err := ag.Run("What is Go?")
	if err != nil {
		fmt.Printf("❌ Error: %v\n", err)
	} else {
		fmt.Printf("\n👤 User: What is Go?\n")
		fmt.Printf("🔧 Retries: default (0)\n")
		fmt.Printf("🤖 Assistant: %s\n", response1.TextContent)
	}

	// Scenario 2: Request with 3 retries
	fmt.Println("\n--- Scenario 2: With 3 Retries ---")
	fmt.Println("If the request fails due to network issues, it will retry up to 3 times")

	response2, err := ag.Run(
		"Explain concurrency in one sentence",
		agent.WithRetries(3),
	)
	if err != nil {
		fmt.Printf("❌ Error after 3 retries: %v\n", err)
	} else {
		fmt.Printf("\n👤 User: Explain concurrency in one sentence\n")
		fmt.Printf("🔧 Retries: 3\n")
		fmt.Printf("🤖 Assistant: %s\n", response2.TextContent)
	}

	// Scenario 3: High retry count for critical operations
	fmt.Println("\n--- Scenario 3: High Retry Count (10) ---")
	fmt.Println("For critical operations, use higher retry counts")

	response3, err := ag.Run(
		"What are the benefits of microservices?",
		agent.WithRetries(10),
	)
	if err != nil {
		fmt.Printf("❌ Error after 10 retries: %v\n", err)
	} else {
		fmt.Printf("\n👤 User: What are the benefits of microservices?\n")
		fmt.Printf("🔧 Retries: 10 (for critical operations)\n")
		fmt.Printf("🤖 Assistant: %s\n", response3.TextContent)
	}

	// Scenario 4: Combined with other options
	fmt.Println("\n--- Scenario 4: Retries + Metadata + SessionID ---")

	response4, err := ag.Run(
		"Summarize REST API best practices",
		agent.WithRetries(5),
		agent.WithMetadata(map[string]interface{}{
			"request_id": "critical_req_001",
			"priority":   "high",
		}),
		agent.WithSessionID("session_retry_demo"),
	)
	if err != nil {
		fmt.Printf("❌ Error: %v\n", err)
	} else {
		fmt.Printf("\n👤 User: Summarize REST API best practices\n")
		fmt.Printf("🔧 Retries: 5\n")
		fmt.Printf("📊 Metadata: request_id=critical_req_001, priority=high\n")
		fmt.Printf("🆔 Session ID: session_retry_demo\n")
		fmt.Printf("🤖 Assistant: %s\n", response4.TextContent)
	}

	fmt.Println("\n=== Demo Complete ===")
	fmt.Println("\n✨ Key Features Demonstrated:")
	fmt.Println("   • WithRetries(n) - Retry up to n times on failure")
	fmt.Println("   • Default behavior (no retries)")
	fmt.Println("   • High retry counts for critical operations")
	fmt.Println("   • Combining retries with other options")
	fmt.Println("\n💡 Use Cases:")
	fmt.Println("   • Network instability (temporary connection issues)")
	fmt.Println("   • API rate limiting (429 errors)")
	fmt.Println("   • Transient service outages")
	fmt.Println("   • Load balancer failovers")
	fmt.Println("   • Database connection pool exhaustion")
	fmt.Println("\n⚙️  Retry Guidelines:")
	fmt.Println("   • 0 retries: Interactive user-facing operations (fast fail)")
	fmt.Println("   • 3-5 retries: Most production scenarios")
	fmt.Println("   • 10+ retries: Critical operations that must succeed")
	fmt.Println("   • Consider exponential backoff for future enhancement")
	fmt.Println("\n⚠️  Note:")
	fmt.Println("   This example demonstrates retry configuration.")
	fmt.Println("   In practice, retries kick in automatically on:")
	fmt.Println("   - Network errors (connection timeout, refused)")
	fmt.Println("   - HTTP 429 (rate limit)")
	fmt.Println("   - HTTP 5xx (server errors)")
	fmt.Println("   - Temporary model unavailability")
}
