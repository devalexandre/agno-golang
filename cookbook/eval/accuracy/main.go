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
)

// AccuracyEval runs an agent several times over the same input and asks a model,
// acting as judge, to score the output against the expected output (1 to 10).
func main() {
	ctx := context.Background()

	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		log.Fatal("OPENAI_API_KEY is required to run this example")
	}

	// 1. Model used by the agent under evaluation
	model, err := openai.NewOpenAIChat(
		models.WithID("gpt-4o-mini"),
		models.WithAPIKey(apiKey),
	)
	if err != nil {
		log.Fatalf("Failed to create the model: %v", err)
	}

	// 2. Model used as judge
	judge, err := openai.NewOpenAIChat(
		models.WithID("gpt-4o-mini"),
		models.WithAPIKey(apiKey),
	)
	if err != nil {
		log.Fatalf("Failed to create the judge model: %v", err)
	}

	// 3. Agent under evaluation
	calculator, err := agent.NewAgent(agent.AgentConfig{
		Context:      ctx,
		Name:         "Calculator",
		Model:        model,
		Instructions: "You are a calculator. Answer with the final number and nothing else.",
	})
	if err != nil {
		log.Fatalf("Failed to create the agent: %v", err)
	}

	// 4. Accuracy evaluation
	evaluation := &eval.AccuracyEval{
		Name:           "calculator-accuracy",
		Agent:          calculator,
		Judge:          judge,
		Input:          "What is 10 * 5 then to the power of 2? Do it step by step.",
		ExpectedOutput: "2500",
		NumIterations:  3,
		AdditionalGuidelines: []string{
			"Only the final numeric answer matters, the reasoning steps are irrelevant.",
		},
		PrintResults: true,
		PrintSummary: true,
	}

	result, err := evaluation.Run(ctx)
	if err != nil {
		log.Fatalf("Evaluation failed: %v", err)
	}

	fmt.Printf("\nAverage score across %d runs: %.2f/10\n", len(result.Results), result.AvgScore)
}
