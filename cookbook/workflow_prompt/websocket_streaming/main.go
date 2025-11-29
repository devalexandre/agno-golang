package main

import (
	"context"
	"fmt"
	"log"

	v2 "github.com/devalexandre/agno-golang/agno/workflow/v2"
)

func main() {
	fmt.Println("=== WebSocket Streaming Example ===\n")

	// Create a WebSocket handler that prints events
	wsHandler := v2.NewDefaultWebSocketHandler(func(event *v2.WorkflowRunResponseEvent) error {
		fmt.Printf("📡 WebSocket Event: %s at %s\n", event.Event, event.Timestamp.Format("15:04:05"))
		if len(event.Metadata) > 0 {
			fmt.Printf("   Metadata: %v\n", event.Metadata)
		}
		fmt.Println()
		return nil
	})
	defer wsHandler.Close()

	// Create workflow with WebSocket streaming
	workflow := v2.NewWorkflow(
		v2.WithWorkflowName("Data Processor"),
		v2.WithWorkflowDescription("Processes data with real-time WebSocket updates"),
		v2.WithWebSocketHandler(wsHandler),
		v2.WithWorkflowSteps(func(input *v2.StepInput) (*v2.StepOutput, error) {
			fmt.Println("🔄 Processing step...")
			return &v2.StepOutput{
				Content:  fmt.Sprintf("Processed: %v", input.Message),
				StepName: "processor",
			}, nil
		}),
	)

	ctx := context.Background()

	fmt.Println("Starting workflow with WebSocket streaming...\n")

	response, err := workflow.Run(ctx, "Process this data")
	if err != nil {
		log.Fatalf("Workflow failed: %v", err)
	}

	fmt.Println("\n=== Workflow Complete ===")
	fmt.Printf("Final Response: %v\n", response.Content)
	fmt.Printf("Total Events Sent: %d\n", len(wsHandler.GetEvents()))
}
