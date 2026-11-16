// Example: instrumenting an agent run with the agno tracing package.
//
// It builds a tracer, emits the canonical spans (agent run -> model call ->
// tool call), consolidates tokens/cost/latency per run and session, and prints
// the result.
//
// By default it uses the in-memory Recorder, so it runs with no external
// dependency. Export to a real OpenTelemetry collector by setting:
//
//	OTEL_EXPORTER_OTLP_ENDPOINT=localhost:4318 go run cookbook/tracing/main.go
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/devalexandre/agno-golang/agno/tracing"
)

func main() {
	ctx := context.Background()

	tracer, err := buildTracer(ctx)
	if err != nil {
		log.Fatalf("failed to build tracer: %v", err)
	}

	// Register the tracer globally: instrumentation helpers with a nil tracer
	// fall back to it.
	tracing.SetGlobalTracer(tracer)
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := tracing.Shutdown(shutdownCtx); err != nil {
			log.Printf("failed to shut tracing down: %v", err)
		}
	}()

	collector := tracing.NewMetricsCollector()

	const (
		agentID   = "agent-research"
		sessionID = "session-1"
		runID     = "run-1"
	)

	collector.StartRun(runID, sessionID, agentID)

	// 1. Agent run span (root).
	runCtx, runSpan := tracing.StartAgentRun(ctx, nil, tracing.AgentRun{
		AgentID:   agentID,
		AgentName: "Research Agent",
		SessionID: sessionID,
		RunID:     runID,
		UserID:    "user-1",
		Input:     "What is the agno framework?",
	})

	// 2. Model call span (child of the run).
	_, modelSpan := tracing.StartModelCall(runCtx, nil, tracing.ModelCall{
		Provider:    "openai",
		ModelID:     "gpt-4o-mini",
		AgentID:     agentID,
		SessionID:   sessionID,
		RunID:       runID,
		Temperature: 0.2,
		MaxTokens:   1024,
	})

	usage := tracing.Usage{
		InputTokens:  842,
		OutputTokens: 173,
		Cost:         0.00042,
		Duration:     640 * time.Millisecond,
	}
	tracing.EndModelCall(modelSpan, usage, "agno is an agent framework", nil)
	collector.RecordUsage(runID, usage)

	// 3. Tool call span (child of the run).
	toolStart := time.Now()
	_, toolSpan := tracing.StartToolCall(runCtx, nil, tracing.ToolCall{
		Name:      "web_search",
		CallID:    "call-1",
		AgentID:   agentID,
		SessionID: sessionID,
		RunID:     runID,
		Arguments: `{"query":"agno framework"}`,
	})
	toolResult, toolErr := runTool()
	toolDuration := time.Since(toolStart)
	tracing.EndToolCall(toolSpan, toolResult, toolDuration, toolErr)
	collector.RecordToolCall(runID, toolDuration, toolErr)

	// 4. Close the run with its consolidated metrics.
	metrics := collector.EndRun(runID)
	tracing.EndAgentRun(runSpan, metrics, "agno is an agent framework", nil)

	printMetrics("run", metrics)
	if session, ok := collector.SessionMetrics(sessionID); ok {
		printMetrics("session", session)
	}

	if recorder, ok := tracer.(*tracing.Recorder); ok {
		printRecordedSpans(recorder)
	}
}

// buildTracer returns an OTLP tracer when a collector endpoint is configured,
// and the in-memory recorder otherwise.
func buildTracer(ctx context.Context) (tracing.Tracer, error) {
	endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if endpoint == "" {
		fmt.Println("OTEL_EXPORTER_OTLP_ENDPOINT is not set: using the in-memory recorder")
		return tracing.NewRecorder(), nil
	}

	fmt.Printf("exporting traces to the OTLP collector at %s\n", endpoint)
	return tracing.NewOTelTracer(ctx, tracing.OTelConfig{
		ServiceName:    "agno-cookbook",
		ServiceVersion: "1.0.0",
		Environment:    "local",
		Endpoint:       endpoint,
		Insecure:       true,
		Batch:          true,
	})
}

// runTool simulates a tool execution.
func runTool() (string, error) {
	time.Sleep(25 * time.Millisecond)
	return "3 results found", nil
}

func printMetrics(scope string, m tracing.Metrics) {
	fmt.Printf("\n=== %s metrics ===\n", scope)
	fmt.Printf("  input tokens : %d\n", m.InputTokens)
	fmt.Printf("  output tokens: %d\n", m.OutputTokens)
	fmt.Printf("  total tokens : %d\n", m.TotalTokens)
	fmt.Printf("  cost         : %.5f %s\n", m.Cost, m.Currency)
	fmt.Printf("  model calls  : %d\n", m.ModelCalls)
	fmt.Printf("  tool calls   : %d\n", m.ToolCalls)
	fmt.Printf("  errors       : %d\n", m.Errors)
	fmt.Printf("  duration     : %s\n", m.Duration)
}

func printRecordedSpans(recorder *tracing.Recorder) {
	fmt.Println("\n=== recorded spans ===")
	for _, span := range recorder.Spans() {
		parent := "-"
		if span.Parent != nil {
			parent = span.Parent.Name
		}
		fmt.Printf("  %-16s parent=%-12s duration=%-12s attrs=%d\n",
			span.Name, parent, span.Duration(), len(span.Attributes))
	}
}
