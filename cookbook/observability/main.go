package main

import (
	"context"
	"fmt"
	"log"

	"github.com/devalexandre/agno-golang/agno/agent"
	"github.com/devalexandre/agno-golang/agno/models"
	"github.com/devalexandre/agno-golang/agno/models/openai/chat"
	"github.com/devalexandre/agno-golang/agno/utils/telemetry"
)

func main() {
	ctx := context.Background()

	// 1. Initialize OpenTelemetry Tracer
	// By default, it looks for OTEL_EXPORTER_OTLP_ENDPOINT (default: localhost:4318)
	tp, err := telemetry.InitTracer("agno-agent-service")
	if err != nil {
		log.Fatalf("Failed to initialize tracer: %v", err)
	}
	defer tp.Shutdown(ctx)

	fmt.Println("--- Observability Example (OpenTelemetry) ---")
	fmt.Println("Tracer initialized. Run your agent and check your collector (e.g., Jaeger, Honeycomb, Jaeger).")

	// 2. Setup Agent
	openAIModel, _ := chat.NewOpenAIChat(models.WithID("gpt-4o"))
	ag, _ := agent.NewAgent(agent.AgentConfig{
		Model: openAIModel,
		Name:  "ObservedAgent",
	})

	// 3. Run with Tracing
	// The framework automatically creates spans for Run, Model Invoke, and Tool Calls.
	fmt.Println("Running agent... (Check your OTEL dashboard for traces)")
	resp, _ := ag.Run("What's the weather in Tokyo?")

	fmt.Printf("Agent Response: %s\n", resp.TextContent)

	fmt.Println("\nTo see traces, you can run a local Jaeger instance:")
	fmt.Println("docker run -d --name jaeger -e COLLECTOR_OTLP_ENABLED=true -p 16686:16686 -p 4317:4317 -p 4318:4318 jaegertracing/all-in-one:latest")
}
