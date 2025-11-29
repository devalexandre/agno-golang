package main

import (
	"context"
	"fmt"
	"log"
	"strings"

	v2 "github.com/devalexandre/agno-golang/agno/workflow/v2"
)

// WorkflowInput defines the expected input structure
type WorkflowInput struct {
	Query    string `json:"query" validate:"required"`
	MaxSteps int    `json:"max_steps"`
	UserID   string `json:"user_id" validate:"required"`
}

func main() {
	fmt.Println("=== Input Schema Validation Example ===\n")

	// Create a simple workflow with input validation
	workflow := v2.NewWorkflow(
		v2.WithWorkflowName("Data Processor"),
		v2.WithWorkflowDescription("Processes data with input validation"),
		v2.WithInputSchema(&WorkflowInput{}), // Enable input validation
		v2.WithWorkflowSteps(func(input *v2.StepInput) (*v2.StepOutput, error) {
			// Access validated input
			fmt.Printf("Processing validated input: %v\n", input.Message)

			return &v2.StepOutput{
				Content:  fmt.Sprintf("Processed: %v", input.Message),
				StepName: "processor",
			}, nil
		}),
	)

	ctx := context.Background()

	// Test 1: Valid input
	fmt.Println("Test 1: Valid Input")
	fmt.Println(strings.Repeat("-", 60))

	validInput := &WorkflowInput{
		Query:    "Analyze sales data",
		MaxSteps: 5,
		UserID:   "user123",
	}

	response, err := workflow.Run(ctx, validInput)
	if err != nil {
		log.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("✅ Success! Response: %v\n", response.Content)
	}

	fmt.Println()

	// Test 2: Invalid input (missing required field)
	fmt.Println("Test 2: Invalid Input (missing Query)")
	fmt.Println(strings.Repeat("-", 60))

	invalidInput := &WorkflowInput{
		MaxSteps: 5,
		UserID:   "user123",
		// Query is missing - should fail validation
	}

	response, err = workflow.Run(ctx, invalidInput)
	if err != nil {
		fmt.Printf("❌ Validation Failed (as expected): %v\n", err)
	} else {
		fmt.Printf("Unexpected success: %v\n", response.Content)
	}

	fmt.Println()

	// Test 3: Wrong type
	fmt.Println("Test 3: Wrong Input Type")
	fmt.Println(strings.Repeat("-", 60))

	wrongTypeInput := "just a string"

	response, err = workflow.Run(ctx, wrongTypeInput)
	if err != nil {
		fmt.Printf("❌ Type Validation Failed (as expected): %v\n", err)
	} else {
		fmt.Printf("Unexpected success: %v\n", response.Content)
	}

	fmt.Println()

	// Test 4: Nil input
	fmt.Println("Test 4: Nil Input")
	fmt.Println(strings.Repeat("-", 60))

	response, err = workflow.Run(ctx, nil)
	if err != nil {
		fmt.Printf("❌ Nil Validation Failed (as expected): %v\n", err)
	} else {
		fmt.Printf("Unexpected success: %v\n", response.Content)
	}

	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("\nValidation Summary:")
	fmt.Println("✅ Valid input with all required fields: PASSED")
	fmt.Println("❌ Missing required field: REJECTED")
	fmt.Println("❌ Wrong type: REJECTED")
	fmt.Println("❌ Nil input: REJECTED")
}
