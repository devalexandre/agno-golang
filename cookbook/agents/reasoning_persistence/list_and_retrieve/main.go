package main

import (
	"context"
	"fmt"
	"log"

	"github.com/devalexandre/agno-golang/agno/reasoning"
)

func main() {
	config := &reasoning.DatabaseConfig{
		Type:     reasoning.DatabaseTypeSQLite,
		Database: ":memory:",
	}

	persistence, err := reasoning.NewReasoningPersistence(config)
	if err != nil {
		log.Fatalf("Failed to create persistence: %v", err)
	}

	ctx := context.Background()
	runID := "run-001"
	agentID := "agent-reasoning-001"

	// Save some steps first
	for i := 1; i <= 5; i++ {
		step := reasoning.ReasoningStepRecord{
			RunID:           runID,
			AgentID:         agentID,
			StepNumber:      i,
			Title:           fmt.Sprintf("Analysis Step %d", i),
			Reasoning:       fmt.Sprintf("Analyzing problem from angle %d", i),
			Action:          "analyze",
			Result:          fmt.Sprintf("Found insight %d", i),
			Confidence:      0.75 + float64(i)*0.05,
			ReasoningTokens: 100 * i,
			InputTokens:     30 * i,
			OutputTokens:    70 * i,
			Duration:        int64(1000 * i),
		}
		persistence.SaveReasoningStep(ctx, step)
	}

	fmt.Println("=== List and Retrieve Reasoning Steps ===\n")

	// List all reasoning steps
	steps, err := persistence.ListReasoningSteps(ctx, runID)
	if err != nil {
		log.Fatalf("Error listing steps: %v", err)
	}

	fmt.Printf("Total of steps: %d\n\n", len(steps))
	for _, step := range steps {
		fmt.Printf("Step %d: %s\n", step.StepNumber, step.Title)
		fmt.Printf("  Reasoning: %s\n", step.Reasoning)
		fmt.Printf("  Confidence: %.2f\n", step.Confidence)
		fmt.Printf("  Reasoning Tokens: %d\n", step.ReasoningTokens)
		fmt.Printf("  Duration: %dms\n\n", step.Duration)
	}

	fmt.Println("✓ All reasoning steps retrieved successfully")
}
