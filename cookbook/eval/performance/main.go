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

// PerformanceEval measures the latency and the memory allocated by a function,
// running it a few times to warm up and then collecting statistics.
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

	// 1. Measure how long it takes to instantiate an agent
	instantiation := &eval.PerformanceEval{
		Name: "agent-instantiation",
		Func: func() error {
			_, err := agent.NewAgent(agent.AgentConfig{
				Context:      ctx,
				Name:         "Assistant",
				Model:        model,
				Instructions: "You are a helpful assistant.",
			})
			return err
		},
		MeasureRuntime: true,
		MeasureMemory:  true,
		WarmupRuns:     3,
		NumIterations:  20,
		PrintSummary:   true,
	}

	instantiationResult, err := instantiation.Run()
	if err != nil {
		log.Fatalf("Evaluation failed: %v", err)
	}

	fmt.Printf("\nAverage instantiation time: %.6fs\n", instantiationResult.RunTimeStats.Avg)
	fmt.Printf("Average memory allocated  : %.6f MiB\n", instantiationResult.MemoryStats.Avg)

	// 2. Measure the latency of a full agent run
	assistant, err := agent.NewAgent(agent.AgentConfig{
		Context:      ctx,
		Name:         "Assistant",
		Model:        model,
		Instructions: "You are a helpful assistant. Answer in a single short sentence.",
	})
	if err != nil {
		log.Fatalf("Failed to create the agent: %v", err)
	}

	runLatency := &eval.PerformanceEval{
		Name: "agent-run",
		Func: func() error {
			_, err := assistant.Run("What is the capital of France?")
			return err
		},
		MeasureRuntime: true,
		WarmupRuns:     1,
		NumIterations:  3,
		PrintResults:   true,
		PrintSummary:   true,
	}

	if _, err := runLatency.Run(); err != nil {
		log.Fatalf("Evaluation failed: %v", err)
	}
}
