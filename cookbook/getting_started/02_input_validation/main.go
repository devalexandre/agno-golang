package main

import (
	"context"
	"fmt"
	"log"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ollama"
)

// UserRequest represents a request that will be validated by the agent
// The Agent will automatically validate input against this schema using JSON marshaling
type UserRequest struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	Age   int    `json:"age"`
	Topic string `json:"topic"`
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

	// Example 1: Agent with InputSchema validation
	fmt.Println("=== Example 1: Agent with InputSchema Validation ===")

	ag, err := agent.NewAgent(agent.AgentConfig{
		Context: ctx,
		Model:   model,
		Name:    "InputValidationAssistant",
		// Set InputSchema to validate inputs - Agent will validate against this type
		InputSchema: &UserRequest{},
		Instructions: `You are a helpful assistant that processes user requests.
When a user submits their information (name, email, age, topic), acknowledge it and provide a brief, friendly response.
Keep responses concise and relevant to their topic of interest.`,
		Debug: false,
	})
	if err != nil {
		log.Fatal(err)
	}

	// Example 1A: Valid input as a map - will be automatically converted to UserRequest
	fmt.Println("\n📋 Example 1A: Input as Map (auto-converted to UserRequest)")
	mapInput := map[string]interface{}{
		"name":  "John Doe",
		"email": "john@example.com",
		"age":   28,
		"topic": "AI and Machine Learning",
	}
	fmt.Printf("   Input: %+v\n", mapInput)

	// Agent internally calls validateInput() which converts map → UserRequest struct
	fmt.Println("   [Agent will validate and convert map to UserRequest struct]")

	// Example 1B: JSON string input
	fmt.Println("\n📋 Example 1B: Input as JSON String (parsed and validated)")
	jsonInput := `{
		"name": "Jane Smith",
		"email": "jane@example.com",
		"age": 35,
		"topic": "Go Programming"
	}`
	fmt.Printf("   Input: %s\n", jsonInput)
	fmt.Println("   [Agent will parse JSON string, validate it matches UserRequest, then convert to struct]")

	// Example 1C: Direct struct input
	fmt.Println("\n📋 Example 1C: Direct Struct Input (validated as-is)")
	directInput := UserRequest{
		Name:  "Bob Johnson",
		Email: "bob@example.com",
		Age:   42,
		Topic: "Distributed Systems",
	}
	fmt.Printf("   Input: %+v\n", directInput)
	fmt.Println("   [Agent validates struct matches UserRequest type]")

	// Example 2: Show how validation works with actual Agent execution
	fmt.Println("\n\n=== Example 2: Actual Agent Execution with Validation ===")

	// Prepare a validated input
	userReq := UserRequest{
		Name:  "Alice Chen",
		Email: "alice@example.com",
		Age:   31,
		Topic: "Kubernetes and Cloud Native Development",
	}

	// Create a message incorporating the user request
	message := fmt.Sprintf(
		"Process this user request: Name=%s, Email=%s, Age=%d, Topic=%s. Provide a 1-2 sentence response.",
		userReq.Name,
		userReq.Email,
		userReq.Age,
		userReq.Topic,
	)

	fmt.Printf("Sending to agent with validated input:\n")
	fmt.Printf("  Message: %s\n\n", message)

	// Run the agent - internally it will:
	// 1. Call validateInput(message)
	// 2. Since message is a string, it stays as string (InputSchema validation is for user data, not prompts)
	// 3. Process and generate response
	response, err := ag.Run(ctx, message)
	if err != nil {
		log.Printf("Agent error: %v", err)
	} else {
		fmt.Printf("Agent response:\n%v\n", response.Messages)
	}

	// Example 3: Demonstrate InputSchema behavior
	fmt.Println("\n\n=== Example 3: Understanding InputSchema Validation ===")
	fmt.Println("InputSchema defines how User Input Data should be structured")
	fmt.Println("")
	fmt.Println("The Agent's validateInput() method:")
	fmt.Println("  • If InputSchema is nil: returns input unchanged")
	fmt.Println("  • If input is a string:")
	fmt.Println("    - Tries to parse as JSON")
	fmt.Println("    - Converts to InputSchema type via JSON marshaling")
	fmt.Println("  • If input is a map:")
	fmt.Println("    - Converts to InputSchema type via JSON marshaling")
	fmt.Println("  • If input is a struct:")
	fmt.Println("    - Validates it matches InputSchema type")
	fmt.Println("    - Returns as-is if valid")
	fmt.Println("")
	fmt.Println("This pattern matches Python's Pydantic model validation!")

	// Example 4: Show interaction between InputSchema and AddDependenciesToContext
	fmt.Println("\n\n=== Example 4: InputSchema with Dependencies ===")

	ag2, err := agent.NewAgent(agent.AgentConfig{
		Context:     ctx,
		Model:       model,
		Name:        "ValidationWithDeps",
		InputSchema: &UserRequest{},
		Dependencies: map[string]interface{}{
			"validation_rules": "All users must be 18+",
			"max_topic_length": 100,
		},
		AddDependenciesToContext: true,
		Instructions: `You are a form processor.
The dependencies provide validation rules - follow them strictly.
Acknowledge receipt of form data and confirm it meets requirements.`,
		Debug: false,
	})
	if err != nil {
		log.Printf("Error creating agent: %v", err)
	}

	formData := UserRequest{
		Name:  "Charlie Davis",
		Email: "charlie@example.com",
		Age:   25,
		Topic: "DevOps and Infrastructure as Code",
	}

	msg := fmt.Sprintf("Validate this form data: %+v", formData)
	fmt.Printf("Agent with InputSchema + Dependencies:\n")
	fmt.Printf("  InputSchema: UserRequest struct\n")
	fmt.Printf("  Dependencies: {validation_rules, max_topic_length}\n")
	fmt.Printf("  Message: %s\n\n", msg)

	resp, err := ag2.Run(ctx, msg)
	if err != nil {
		log.Printf("Agent error: %v", err)
	} else {
		fmt.Printf("Response:\n%v\n", resp.Messages)
	}

	fmt.Println("\n✅ InputSchema validation is working! Check the responses above.")
}
