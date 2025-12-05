package main

import (
	"context"
	"fmt"
	"math/rand"
	"strings"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/ollama"
	"github.com/devalexandre/agno-golang/agno/tools"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
	"github.com/devalexandre/agno-golang/agno/utils"
)

func main() {
	ctx := context.Background()

	model, err := ollama.NewOllamaChat(
		models.WithID("llama3.2:latest"),
		models.WithBaseURL("http://localhost:11434"),
	)
	if err != nil {
		utils.ErrorPanel(err)
		return
	}

	validateTool := tools.NewToolFromFunction(
		func(ctx context.Context, data string) (string, error) {
			if strings.TrimSpace(data) == "" {
				return "", fmt.Errorf("validation failed: empty data")
			}
			if len(data) < 3 {
				return "", fmt.Errorf("validation failed: data too short (minimum 3 characters)")
			}
			return fmt.Sprintf("VALIDATED_%s", strings.ToUpper(data)), nil
		},
		"Validates input data format",
	)

	transformTool := tools.NewToolFromFunction(
		func(ctx context.Context, data string) (string, error) {
			if rand.Float32() < 0.3 {
				return "", fmt.Errorf("transform failed: temporary processing error")
			}
			return fmt.Sprintf("TRANSFORMED[%s]", data), nil
		},
		"Transforms validated data",
	)

	enrichTool := tools.NewToolFromFunction(
		func(ctx context.Context, data string) (string, error) {
			if !strings.Contains(data, "TRANSFORMED") {
				return "", fmt.Errorf("enrichment failed: invalid input format")
			}
			return fmt.Sprintf("ENRICHED{%s}", data), nil
		},
		"Enriches transformed data",
	)

	storeTool := tools.NewToolFromFunction(
		func(ctx context.Context, data string) (string, error) {
			if !strings.Contains(data, "ENRICHED") {
				return "", fmt.Errorf("storage failed: data not enriched")
			}
			return fmt.Sprintf("STORED{%s}", data), nil
		},
		"Stores enriched data",
	)

	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Println("ChainTool with Error Handling & Rollback")
	fmt.Println(strings.Repeat("=", 80) + "\n")

	testCases := []struct {
		name     string
		input    string
		strategy agent.RollbackStrategy
		maxRetry int
	}{
		{
			name:     "Success Case - 4 tools execute successfully",
			input:    "mydata",
			strategy: agent.RollbackNone,
			maxRetry: 1,
		},
		{
			name:     "Error with Rollback to Start - Reverts to initial input",
			input:    "x",
			strategy: agent.RollbackToStart,
			maxRetry: 1,
		},
		{
			name:     "Error with Rollback to Previous - Uses last successful result",
			input:    "data123",
			strategy: agent.RollbackToPrevious,
			maxRetry: 1,
		},
		{
			name:     "Error with Skip - Continues with next tool",
			input:    "testdata",
			strategy: agent.RollbackSkip,
			maxRetry: 2,
		},
	}

	for _, tc := range testCases {
		fmt.Printf("\n%s\n", tc.name)
		fmt.Println(strings.Repeat("-", 80))

		ag, err := agent.NewAgent(agent.AgentConfig{
			Context:         ctx,
			Model:           model,
			Name:            "DataProcessor",
			Description:     "Data processing pipeline with error handling",
			Tools:           []toolkit.Tool{validateTool, transformTool, enrichTool, storeTool},
			EnableChainTool: true,
			ChainToolErrorConfig: &agent.ChainToolErrorConfig{
				Strategy:   tc.strategy,
				MaxRetries: tc.maxRetry,
			},
			ChainToolErrorHandler: agent.NewDefaultErrorHandler(tc.strategy, tc.maxRetry),
			Debug:                 false,
		})
		if err != nil {
			utils.ErrorPanel(err)
			continue
		}

		input := fmt.Sprintf("Process this data: %s", tc.input)
		response, err := ag.Run(input)
		if err != nil {
			utils.ErrorPanel(err)
			continue
		}

		fmt.Printf("✓ Response: %s\n", response.TextContent)
	}

	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Println("Rollback Strategy Comparison:")
	fmt.Println(strings.Repeat("=", 80))
	fmt.Printf(`
Strategies Available:

1. RollbackNone
   - Stops execution immediately on first error
   - Does not attempt recovery
   - Best for: Critical data pipelines where failure is unacceptable

2. RollbackToStart
   - Reverts to the initial input value
   - Restarts chain from beginning
   - Best for: Retrying entire pipeline with fresh state

3. RollbackToPrevious
   - Uses the result from the last successful tool
   - Skips the failed tool and continues
   - Best for: Tolerating individual tool failures

4. RollbackSkip
   - Continues with next tool using current state
   - Skips only the failed tool
   - Best for: Graceful degradation and optional processing
`)

	fmt.Println(strings.Repeat("=", 80) + "\n")
}
