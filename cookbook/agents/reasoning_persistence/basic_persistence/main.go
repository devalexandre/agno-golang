package main

import (
	"context"
	"fmt"
	"log"

	"github.com/devalexandre/agno-golang/agno/reasoning"
)

func main() {
	// Create persistence using factory pattern
	config := &reasoning.DatabaseConfig{
		Type:     reasoning.DatabaseTypeSQLite,
		Database: ":memory:", // In-memory database
	}

	persistence, err := reasoning.NewReasoningPersistence(config)
	if err != nil {
		log.Fatalf("Failed to create persistence: %v", err)
	}

	ctx := context.Background()

	fmt.Println("=== Basic Persistence: Saving Reasoning Steps ===\n")

	runID := "run-001"
	agentID := "agent-reasoning-001"

	// Save 5 reasoning steps
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
			NextAction:      "continue",
			ReasoningTokens: 100 * i,
			InputTokens:     30 * i,
			OutputTokens:    70 * i,
			Duration:        int64(1000 * i),
			Metadata: map[string]interface{}{
				"model":       "o1",
				"temperature": 0.7,
				"step_type":   "analysis",
			},
		}

		err := persistence.SaveReasoningStep(ctx, step)
		if err != nil {
			log.Printf("Error saving step %d: %v", i, err)
			continue
		}

		fmt.Printf("✓ Step %d saved successfully (ID: %d)\n", i, step.ID)
	}

	fmt.Println("\n✓ All reasoning steps saved successfully")
	fmt.Printf("  Run ID: %s\n", runID)
	fmt.Printf("  Agent ID: %s\n", agentID)
	fmt.Printf("  Total Steps: 5\n")
}
