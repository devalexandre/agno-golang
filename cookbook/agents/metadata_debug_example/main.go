package main

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ollama"
)

func main() {
	fmt.Println("=== Metadata and Debug Mode Example ===\n")
	fmt.Println("This example demonstrates:")
	fmt.Println("  • WithMetadata - Track request metadata (user, source, tracking IDs)")
	fmt.Println("  • WithDebugMode - Enable detailed debugging output")
	fmt.Println("  • Use cases: analytics, monitoring, troubleshooting\n")

	// 1. Create cloud model
	fmt.Println("🤖 Setting up cloud LLM...")
	model, err := ollama.NewOllamaChat(
		models.WithID("kimi-k2:1t-cloud"),
		models.WithBaseURL("https://ollama.com"),
	)
	if err != nil {
		log.Fatalf("Failed to create model: %v", err)
	}

	// 2. Create agent
	fmt.Println("🎯 Creating agent...")
	ag, err := agent.NewAgent(agent.AgentConfig{
		Name:          "Metadata Assistant",
		Model:         model,
		Description:   "AI assistant with metadata tracking",
		Instructions:  "You are a helpful assistant. Answer questions concisely.",
		Markdown:      true,
		ShowToolsCall: false,
	})
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	fmt.Println("\n✅ Agent created!")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")

	// Scenario 1: Basic metadata tracking
	fmt.Println("--- Scenario 1: Basic Metadata Tracking ---")

	metadata1 := map[string]interface{}{
		"user_id":    "user_123",
		"request_id": "req_abc_001",
		"timestamp":  time.Now().Unix(),
		"source":     "web_app",
		"ip_address": "192.168.1.100",
		"user_agent": "Mozilla/5.0",
	}

	response1, err := ag.Run(
		"What is 2 + 2?",
		agent.WithMetadata(metadata1),
	)
	if err != nil {
		log.Printf("Error: %v", err)
	} else {
		fmt.Printf("\n👤 User: What is 2 + 2?\n")
		fmt.Printf("📊 Metadata: %s\n", formatMetadata(metadata1))
		fmt.Printf("🤖 Assistant: %s\n", response1.TextContent)
		fmt.Printf("\n💡 Metadata can be used for analytics and tracking\n")
	}

	// Scenario 2: Metadata for A/B testing
	fmt.Println("\n--- Scenario 2: A/B Testing Metadata ---")

	metadata2 := map[string]interface{}{
		"user_id":       "user_456",
		"experiment_id": "exp_2024_11",
		"variant":       "variant_B",
		"feature_flags": []string{"new_ui", "enhanced_search"},
		"session_start": time.Now().Add(-5 * time.Minute).Unix(),
		"page":          "/dashboard",
	}

	response2, err := ag.Run(
		"Explain quantum computing briefly",
		agent.WithMetadata(metadata2),
	)
	if err != nil {
		log.Printf("Error: %v", err)
	} else {
		fmt.Printf("\n👤 User: Explain quantum computing briefly\n")
		fmt.Printf("📊 Metadata: %s\n", formatMetadata(metadata2))
		fmt.Printf("🤖 Assistant: %s\n", response2.TextContent)
		fmt.Printf("\n💡 Track which variant performs better with metadata\n")
	}

	// Scenario 3: Debug mode disabled (default)
	fmt.Println("\n--- Scenario 3: Normal Mode (Debug OFF) ---")

	metadata3 := map[string]interface{}{
		"user_id":    "user_789",
		"request_id": "req_xyz_003",
		"debug":      false,
	}

	response3, err := ag.Run(
		"What are Go channels?",
		agent.WithMetadata(metadata3),
		agent.WithDebugMode(false), // Explicitly disable debug
	)
	if err != nil {
		log.Printf("Error: %v", err)
	} else {
		fmt.Printf("\n👤 User: What are Go channels?\n")
		fmt.Printf("🔧 Debug Mode: OFF\n")
		fmt.Printf("🤖 Assistant: %s\n", response3.TextContent)
		fmt.Printf("\n💡 Normal mode - no debug output\n")
	}

	// Scenario 4: Debug mode enabled
	fmt.Println("\n--- Scenario 4: Debug Mode ON ---")
	fmt.Println("⚠️  Enabling debug mode for detailed output...\n")

	metadata4 := map[string]interface{}{
		"user_id":     "developer_001",
		"request_id":  "debug_req_004",
		"environment": "development",
		"debug":       true,
	}

	response4, err := ag.Run(
		"What is the capital of France?",
		agent.WithMetadata(metadata4),
		agent.WithDebugMode(true), // Enable debug mode
	)
	if err != nil {
		log.Printf("Error: %v", err)
	} else {
		fmt.Printf("\n👤 User: What is the capital of France?\n")
		fmt.Printf("🔧 Debug Mode: ON\n")
		fmt.Printf("🤖 Assistant: %s\n", response4.TextContent)
		fmt.Printf("\n💡 Debug mode shows internal processing details\n")
	}

	// Scenario 5: Complex metadata for production monitoring
	fmt.Println("\n--- Scenario 5: Production Monitoring Metadata ---")

	metadata5 := map[string]interface{}{
		"request_id":   "prod_req_005",
		"user_id":      "premium_user_999",
		"organization": "acme_corp",
		"plan":         "enterprise",
		"api_version":  "v2",
		"client_sdk":   "agno-golang-sdk/1.0.0",
		"region":       "us-east-1",
		"cost_center":  "engineering",
		"priority":     "high",
		"sla_tier":     "gold",
		"tags": map[string]string{
			"department": "R&D",
			"project":    "ai_assistant",
			"env":        "production",
		},
	}

	response5, err := ag.Run(
		"Summarize the benefits of microservices architecture",
		agent.WithMetadata(metadata5),
	)
	if err != nil {
		log.Printf("Error: %v", err)
	} else {
		fmt.Printf("\n👤 User: Summarize the benefits of microservices architecture\n")
		fmt.Printf("📊 Metadata: %s\n", formatMetadata(metadata5))
		fmt.Printf("🤖 Assistant: %s\n", response5.TextContent)
		fmt.Printf("\n💡 Rich metadata for enterprise monitoring and billing\n")
	}

	// Scenario 6: Combining metadata with other options
	fmt.Println("\n--- Scenario 6: Metadata + SessionID + UserID ---")

	sessionID := "session_combined_006"
	userID := "user_combo_123"

	metadata6 := map[string]interface{}{
		"request_id":      "combo_req_006",
		"conversation_id": sessionID,
		"user_identifier": userID,
		"channel":         "slack",
		"bot_name":        "agno-bot",
		"workspace_id":    "ws_12345",
	}

	response6, err := ag.Run(
		"Hello!",
		agent.WithSessionID(sessionID),
		agent.WithUserID(userID),
		agent.WithMetadata(metadata6),
	)
	if err != nil {
		log.Printf("Error: %v", err)
	} else {
		fmt.Printf("\n👤 User: Hello!\n")
		fmt.Printf("🆔 Session ID: %s\n", sessionID)
		fmt.Printf("👤 User ID: %s\n", userID)
		fmt.Printf("📊 Metadata: %s\n", formatMetadata(metadata6))
		fmt.Printf("🤖 Assistant: %s\n", response6.TextContent)
		fmt.Printf("\n💡 Combine metadata with SessionID and UserID for complete tracking\n")
	}

	fmt.Println("\n=== Demo Complete ===")
	fmt.Println("\n✨ Key Features Demonstrated:")
	fmt.Println("   • WithMetadata - Attach custom tracking data to requests")
	fmt.Println("   • WithDebugMode - Enable/disable detailed debugging")
	fmt.Println("   • Metadata for analytics (user tracking, A/B testing)")
	fmt.Println("   • Metadata for monitoring (SLA, cost centers, regions)")
	fmt.Println("   • Combined with SessionID and UserID")
	fmt.Println("\n💡 Use Cases:")
	fmt.Println("   • Request tracing across microservices")
	fmt.Println("   • A/B testing and feature flag tracking")
	fmt.Println("   • Cost allocation and billing")
	fmt.Println("   • Performance monitoring and SLA compliance")
	fmt.Println("   • Debugging production issues")
	fmt.Println("   • Analytics and user behavior tracking")
}

// formatMetadata converts metadata to a readable JSON string
func formatMetadata(metadata map[string]interface{}) string {
	bytes, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return fmt.Sprintf("%v", metadata)
	}
	return string(bytes)
}
