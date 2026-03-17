package main

import (
	"fmt"
	"strings"

	"github.com/devalexandre/agno-golang/agno/flow"
	v2 "github.com/devalexandre/agno-golang/agno/workflow/v2"
)

func main() {
	// Create a simple workflow using the fluent API
	workflow := flow.New("Basic Workflow").
		Description("A simple workflow example").
		Step("uppercase", func(input *v2.StepInput) (*v2.StepOutput, error) {
			msg := input.GetMessageAsString()
			return &v2.StepOutput{
				Content: strings.ToUpper(msg),
			}, nil
		}).
		If(v2.IfContentContains("HELLO"),
			func(input *v2.StepInput) (*v2.StepOutput, error) {
				return &v2.StepOutput{
					Content: "Greeting detected: " + input.GetLastStepContent().(string),
				}, nil
			},
		).
		Else(
			func(input *v2.StepInput) (*v2.StepOutput, error) {
				return &v2.StepOutput{
					Content: "Normal message: " + input.GetLastStepContent().(string),
				}, nil
			},
		).
		Build()

	// Run with a greeting
	fmt.Println("--- Test 1: Greeting ---")
	workflow.PrintResponse("hello world", false)

	// Run without a greeting
	fmt.Println("\n--- Test 2: Non-greeting ---")
	workflow.PrintResponse("how are you?", false)
}
