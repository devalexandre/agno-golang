package tracing

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNoopTracer(t *testing.T) {
	tracer := NoopTracer{}
	ctx, span := tracer.StartSpan(context.Background(), SpanAgentRun, map[string]any{AttrRunID: "run-1"})

	require.NotNil(t, ctx)
	require.NotNil(t, span)

	span.SetAttribute(AttrAgentID, "agent-1")
	span.AddMetric(AttrCost, 1.5)
	span.RecordError(errors.New("boom"))
	span.End()

	assert.NoError(t, tracer.Shutdown(context.Background()))
}

func TestGlobalTracerDefaultsToNoop(t *testing.T) {
	t.Cleanup(func() { SetGlobalTracer(nil) })

	SetGlobalTracer(nil)
	assert.IsType(t, NoopTracer{}, GlobalTracer())
	assert.False(t, Enabled())

	rec := NewRecorder()
	SetGlobalTracer(rec)
	assert.Same(t, rec, GlobalTracer())
	assert.True(t, Enabled())

	_, span := StartSpan(context.Background(), "custom", map[string]any{"k": "v"})
	span.End()
	require.Len(t, rec.Spans(), 1)

	require.NoError(t, Shutdown(context.Background()))
	assert.True(t, rec.IsShutdown())
	assert.IsType(t, NoopTracer{}, GlobalTracer())
}

func TestRecorderCapturesSpanTree(t *testing.T) {
	rec := NewRecorder()

	ctx, runSpan := StartAgentRun(context.Background(), rec, AgentRun{
		AgentID:   "agent-1",
		AgentName: "researcher",
		SessionID: "sess-1",
		RunID:     "run-1",
		UserID:    "user-1",
		Input:     "hello",
	})

	_, modelSpan := StartModelCall(ctx, rec, ModelCall{
		Provider:  "openai",
		ModelID:   "gpt-4o",
		RunID:     "run-1",
		SessionID: "sess-1",
		Streaming: true,
	})
	EndModelCall(modelSpan, Usage{InputTokens: 100, OutputTokens: 20, Cost: 0.002, Duration: 250 * time.Millisecond}, "hi", nil)

	_, toolSpan := StartToolCall(ctx, rec, ToolCall{
		Name:      "search",
		CallID:    "call-1",
		RunID:     "run-1",
		Arguments: `{"q":"agno"}`,
	})
	EndToolCall(toolSpan, "result", 10*time.Millisecond, nil)

	EndAgentRun(runSpan, Metrics{
		RunID:        "run-1",
		SessionID:    "sess-1",
		InputTokens:  100,
		OutputTokens: 20,
		TotalTokens:  120,
		Cost:         0.002,
		Currency:     DefaultCurrency,
		ToolCalls:    1,
		ModelCalls:   1,
		Duration:     300 * time.Millisecond,
	}, "final answer", nil)

	spans := rec.Spans()
	require.Len(t, spans, 3)

	run := rec.SpansByName(SpanAgentRun)
	require.Len(t, run, 1)
	assert.True(t, run[0].Ended)
	assert.Equal(t, KindAgent, run[0].Attributes[AttrSpanKind])
	assert.Equal(t, "agent-1", run[0].Attributes[AttrAgentID])
	assert.Equal(t, "sess-1", run[0].Attributes[AttrSessionID])
	assert.Equal(t, "final answer", run[0].Attributes[AttrOutput])
	assert.Equal(t, DefaultCurrency, run[0].Attributes[AttrCostCurrency])
	assert.Equal(t, float64(120), run[0].Metrics[AttrTokensTotal])
	assert.Equal(t, float64(1), run[0].Metrics[AttrToolCallCount])
	assert.Equal(t, 0.002, run[0].Metrics[AttrCost])
	assert.Equal(t, float64(300), run[0].Metrics[AttrLatencyMS])

	model := rec.SpansByName(SpanModelCall)
	require.Len(t, model, 1)
	assert.Same(t, run[0], model[0].Parent)
	assert.Equal(t, KindLLM, model[0].Attributes[AttrSpanKind])
	assert.Equal(t, "openai", model[0].Attributes[AttrModelProvider])
	assert.Equal(t, "gpt-4o", model[0].Attributes[AttrModelID])
	assert.Equal(t, true, model[0].Attributes[AttrModelStreaming])
	assert.Equal(t, DefaultCurrency, model[0].Attributes[AttrCostCurrency])
	assert.Equal(t, float64(100), model[0].Metrics[AttrTokensInput])
	assert.Equal(t, float64(120), model[0].Metrics[AttrTokensTotal], "total tokens are derived from input+output")
	assert.Equal(t, 0.002, model[0].Metrics[AttrCost])
	assert.Equal(t, float64(250), model[0].Metrics[AttrLatencyMS])

	tool := rec.SpansByName(SpanToolCall)
	require.Len(t, tool, 1)
	assert.Same(t, run[0], tool[0].Parent)
	assert.Equal(t, "search", tool[0].Attributes[AttrToolName])
	assert.Equal(t, "call-1", tool[0].Attributes[AttrToolCallID])
	assert.Equal(t, "result", tool[0].Attributes[AttrToolResult])
	assert.Equal(t, float64(10), tool[0].Metrics[AttrLatencyMS])
	assert.False(t, tool[0].Failed())
}

func TestEndSpansRecordErrors(t *testing.T) {
	rec := NewRecorder()
	failure := errors.New("tool exploded")

	_, span := StartToolCall(context.Background(), rec, ToolCall{Name: "search"})
	EndToolCall(span, "", 0, failure)

	spans := rec.SpansByName(SpanToolCall)
	require.Len(t, spans, 1)
	assert.True(t, spans[0].Failed())
	assert.Equal(t, true, spans[0].Attributes[AttrErrorFlag])
	assert.Equal(t, "tool exploded", spans[0].Attributes[AttrErrorMessage])
	assert.True(t, spans[0].Ended)
}

func TestStartHelpersFallBackToGlobalTracer(t *testing.T) {
	t.Cleanup(func() { SetGlobalTracer(nil) })

	rec := NewRecorder()
	SetGlobalTracer(rec)

	_, span := StartAgentRun(context.Background(), nil, AgentRun{RunID: "run-x"})
	span.End()

	require.Len(t, rec.SpansByName(SpanAgentRun), 1)
}

func TestAgentRunSpanNameByScope(t *testing.T) {
	tests := []struct {
		name string
		run  AgentRun
		span string
		kind string
	}{
		{"agent", AgentRun{AgentID: "a"}, SpanAgentRun, KindAgent},
		{"team", AgentRun{TeamID: "t"}, SpanTeamRun, KindTeam},
		{"workflow", AgentRun{WorkflowID: "w", TeamID: "t"}, SpanWorkflowRun, KindWorkflow},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := NewRecorder()
			_, span := StartAgentRun(context.Background(), rec, tc.run)
			span.End()

			spans := rec.SpansByName(tc.span)
			require.Len(t, spans, 1)
			assert.Equal(t, tc.kind, spans[0].Attributes[AttrSpanKind])
		})
	}
}

func TestSetAttributesIgnoresNilValues(t *testing.T) {
	rec := NewRecorder()
	_, span := rec.StartSpan(context.Background(), "s", nil)

	SetAttributes(span, map[string]any{"a": 1, "b": nil})
	SetAttributes(nil, map[string]any{"a": 1})

	spans := rec.Spans()
	require.Len(t, spans, 1)
	assert.Equal(t, 1, spans[0].Attributes["a"])
	_, ok := spans[0].Attribute("b")
	assert.False(t, ok)
}
