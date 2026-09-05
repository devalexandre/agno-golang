package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/eval"
	"github.com/devalexandre/agno-golang/agno/models"
	openai "github.com/devalexandre/agno-golang/agno/models/openai/chat"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// MathToolkit exposes basic arithmetic operations to the agent.
type MathToolkit struct {
	toolkit.Toolkit
}

// MathParams are the parameters of the arithmetic operations.
type MathParams struct {
	A float64 `json:"a" description:"First number"`
	B float64 `json:"b" description:"Second number"`
}

// Multiply returns the product of two numbers.
func (mt *MathToolkit) Multiply(params MathParams) (float64, error) {
	return params.A * params.B, nil
}

// Add returns the sum of two numbers.
func (mt *MathToolkit) Add(params MathParams) (float64, error) {
	return params.A + params.B, nil
}

// ReliabilityEval asserts that the tool calls expected from a run actually happened,
// optionally checking the arguments the tools were called with.
func main() {
	ctx := context.Background()

	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		log.Fatal("OPENAI_API_KEY is required to run this example")
	}

	model, err := openai.NewOpenAIChat(
		models.WithID("gpt-4o-mini"),
		models.WithAPIKey(apiKey),
	)
	if err != nil {
		log.Fatalf("Failed to create the model: %v", err)
	}

	// 1. Toolkit exposed to the agent
	mathToolkit := &MathToolkit{Toolkit: toolkit.NewToolkit()}
	mathToolkit.Name = "math"
	mathToolkit.Description = "Basic arithmetic operations"
	mathToolkit.Register("Multiply", "Multiply two numbers", mathToolkit, mathToolkit.Multiply, MathParams{})
	mathToolkit.Register("Add", "Add two numbers", mathToolkit, mathToolkit.Add, MathParams{})

	// 2. Agent under evaluation
	calculator, err := agent.NewAgent(agent.AgentConfig{
		Context:      ctx,
		Name:         "Calculator",
		Model:        model,
		Instructions: "You are a calculator. Always use the provided tools to compute the result.",
		Tools:        []toolkit.Tool{mathToolkit},
	})
	if err != nil {
		log.Fatalf("Failed to create the agent: %v", err)
	}

	// 3. Run the agent
	response, err := calculator.Run("What is 10 * 5?")
	if err != nil {
		log.Fatalf("Agent run failed: %v", err)
	}

	// 4. Assert the expected tool calls happened with the expected arguments
	evaluation := &eval.ReliabilityEval{
		Name:              "calculator-reliability",
		AgentResponse:     &response,
		ExpectedToolCalls: []string{"Multiply"},
		ExpectedToolCallArguments: map[string][]map[string]interface{}{
			"Multiply": {{"a": 10, "b": 5}},
		},
		AllowAdditionalToolCalls: false,
		PrintResults:             true,
	}

	result, err := evaluation.Run()
	if err != nil {
		log.Fatalf("Evaluation failed: %v", err)
	}

	if err := result.AssertPassed(); err != nil {
		log.Fatalf("Reliability evaluation did not pass: %v", err)
	}

	fmt.Println("\nReliability evaluation passed")
}
