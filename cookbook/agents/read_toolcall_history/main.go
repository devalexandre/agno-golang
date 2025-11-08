package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ollama"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// CalculatorToolkit provides basic math operations for demonstration
type CalculatorToolkit struct {
	Name        string
	Description string
}

type AddParams struct {
	A float64 `json:"a" jsonschema:"required,description=First number"`
	B float64 `json:"b" jsonschema:"required,description=Second number"`
}

func (ct *CalculatorToolkit) Add(params AddParams) (float64, error) {
	return params.A + params.B, nil
}

type MultiplyParams struct {
	A float64 `json:"a" jsonschema:"required,description=First number"`
	B float64 `json:"b" jsonschema:"required,description=Second number"`
}

func (ct *CalculatorToolkit) Multiply(params MultiplyParams) (float64, error) {
	return params.A * params.B, nil
}

type DivideParams struct {
	A float64 `json:"a" jsonschema:"required,description=Numerator"`
	B float64 `json:"b" jsonschema:"required,description=Denominator"`
}

func (ct *CalculatorToolkit) Divide(params DivideParams) (float64, error) {
	if params.B == 0 {
		return 0, fmt.Errorf("division by zero")
	}
	return params.A / params.B, nil
}

func main() {
	ctx := context.Background()

	// Get API key
	apiKey := os.Getenv("OLLAMA_API_KEY")
	if apiKey == "" {
		log.Fatal("OLLAMA_API_KEY environment variable is required")
	}

	// Create calculator toolkit
	calc := &CalculatorToolkit{}
	calc.Name = "calculator"
	calc.Description = "A simple calculator that can add, multiply, and divide numbers"

	tk := toolkit.NewToolkit()
	tk.Name = calc.Name
	tk.Description = calc.Description
	tk.Register("add", calc, calc.Add, AddParams{})
	tk.Register("multiply", calc, calc.Multiply, MultiplyParams{})
	tk.Register("divide", calc, calc.Divide, DivideParams{})

	// Create Ollama Cloud model
	model, err := ollama.NewOllamaChat(
		models.WithID("kimi-k2:1t-cloud"),
		models.WithAPIKey(apiKey),
		models.WithBaseURL("https://ollama.com"),
	)
	if err != nil {
		log.Fatalf("Failed to create Ollama model: %v", err)
	}

	// Create agent with read_toolcall_history tool enabled
	ag, err := agent.NewAgent(agent.AgentConfig{
		Context:                       ctx,
		Model:                         model,
		Name:                          "CalculatorAssistant",
		Description:                   "You are a calculator assistant. Use the calculator tools to perform calculations. You can also check your tool usage history with tool_history.read() and tool_history.stats().",
		Tools:                         []toolkit.Tool{&tk},
		EnableReadToolCallHistoryTool: true, // Enable the default tool
		AddHistoryToMessages:          true, // Keep conversation history
		NumHistoryRuns:                10,
		Debug:                         false,
	})
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	fmt.Println("=== Read Tool Call History Tool Demo ===\n")
	fmt.Println("This example demonstrates the ReadToolCallHistory default tool.")
	fmt.Println("The agent can use tool_history.read(limit) and tool_history.stats() to track tool usage.\n")

	// Conversation 1: Use add
	fmt.Println("\n--- Conversation 1: Addition ---")
	runConversation(ctx, ag, "Calculate 15 + 27")

	// Conversation 2: Use multiply
	fmt.Println("\n--- Conversation 2: Multiplication ---")
	runConversation(ctx, ag, "Multiply 8 by 12")

	// Conversation 3: Use divide
	fmt.Println("\n--- Conversation 3: Division ---")
	runConversation(ctx, ag, "What is 144 divided by 12?")

	// Conversation 4: Multiple operations
	fmt.Println("\n--- Conversation 4: Complex Calculation ---")
	runConversation(ctx, ag, "Calculate (25 + 15) * 3")

	// Conversation 5: Check tool history
	fmt.Println("\n--- Conversation 5: Read Tool History ---")
	runConversation(ctx, ag, "Can you check your tool usage history? Use tool_history.read() to see the last few tool calls you made.")

	// Conversation 6: Get statistics
	fmt.Println("\n--- Conversation 6: Tool Statistics ---")
	runConversation(ctx, ag, "Show me statistics about your tool usage with tool_history.stats().")

	fmt.Println("\n=== Demo Complete ===")
	fmt.Println("\nThe agent successfully used tool_history.read() and tool_history.stats() to track calculator tool usage.")
}

func runConversation(ctx context.Context, ag *agent.Agent, userMessage string) {
	fmt.Printf("User: %s\n", userMessage)

	// Run agent
	run, err := ag.Run(userMessage)
	if err != nil {
		log.Printf("Agent error: %v", err)
		return
	}

	// Print response
	fmt.Printf("Assistant: %s\n", run.TextContent)
	fmt.Println()
}
