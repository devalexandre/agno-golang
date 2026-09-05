package tracing

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// newTestTracer builds an OTelTracer that exports to memory: no network is
// touched by the test suite.
func newTestTracer(t *testing.T) (*OTelTracer, *tracetest.InMemoryExporter) {
	t.Helper()

	exporter := tracetest.NewInMemoryExporter()
	tracer, err := NewOTelTracer(context.Background(), OTelConfig{
		ServiceName: "agno-test",
		Environment: "test",
		Exporter:    exporter,
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = tracer.Shutdown(context.Background()) })

	return tracer, exporter
}

func attrOf(t *testing.T, span tracetest.SpanStub, key string) attribute.Value {
	t.Helper()
	for _, kv := range span.Attributes {
		if string(kv.Key) == key {
			return kv.Value
		}
	}
	t.Fatalf("attribute %q not found on span %q", key, span.Name)
	return attribute.Value{}
}

func TestOTelTracerExportsAgentRunTree(t *testing.T) {
	tracer, exporter := newTestTracer(t)

	ctx, runSpan := StartAgentRun(context.Background(), tracer, AgentRun{
		AgentID:   "agent-1",
		AgentName: "researcher",
		SessionID: "sess-1",
		RunID:     "run-1",
		UserID:    "user-1",
	})

	_, modelSpan := StartModelCall(ctx, tracer, ModelCall{
		Provider: "openai",
		ModelID:  "gpt-4o",
		RunID:    "run-1",
	})
	EndModelCall(modelSpan, Usage{InputTokens: 80, OutputTokens: 20, Cost: 0.004, Duration: 120 * time.Millisecond}, "", nil)

	_, toolSpan := StartToolCall(ctx, tracer, ToolCall{Name: "search", CallID: "call-1", RunID: "run-1"})
	EndToolCall(toolSpan, "ok", 7*time.Millisecond, nil)

	EndAgentRun(runSpan, Metrics{
		RunID:       "run-1",
		SessionID:   "sess-1",
		TotalTokens: 100,
		Cost:        0.004,
		Currency:    DefaultCurrency,
		ToolCalls:   1,
		ModelCalls:  1,
	}, "done", nil)

	require.NoError(t, tracer.ForceFlush(context.Background()))

	spans := exporter.GetSpans()
	require.Len(t, spans, 3)

	byName := map[string]tracetest.SpanStub{}
	for _, s := range spans {
		byName[s.Name] = s
	}

	run, ok := byName[SpanAgentRun]
	require.True(t, ok)
	assert.Equal(t, "agent-1", attrOf(t, run, AttrAgentID).AsString())
	assert.Equal(t, "sess-1", attrOf(t, run, AttrSessionID).AsString())
	assert.Equal(t, float64(100), attrOf(t, run, AttrTokensTotal).AsFloat64())
	assert.Equal(t, float64(1), attrOf(t, run, AttrToolCallCount).AsFloat64())

	model, ok := byName[SpanModelCall]
	require.True(t, ok)
	assert.Equal(t, "openai", attrOf(t, model, AttrModelProvider).AsString())
	assert.Equal(t, float64(100), attrOf(t, model, AttrTokensTotal).AsFloat64())
	assert.InDelta(t, 0.004, attrOf(t, model, AttrCost).AsFloat64(), 1e-9)
	assert.Equal(t, float64(120), attrOf(t, model, AttrLatencyMS).AsFloat64())

	tool, ok := byName[SpanToolCall]
	require.True(t, ok)
	assert.Equal(t, "search", attrOf(t, tool, AttrToolName).AsString())
	assert.Equal(t, "ok", attrOf(t, tool, AttrToolResult).AsString())

	// Model and tool spans are children of the agent run span, in the same trace.
	assert.Equal(t, run.SpanContext.TraceID(), model.SpanContext.TraceID())
	assert.Equal(t, run.SpanContext.SpanID(), model.Parent.SpanID())
	assert.Equal(t, run.SpanContext.SpanID(), tool.Parent.SpanID())
}

func TestOTelTracerRecordsErrorStatus(t *testing.T) {
	tracer, exporter := newTestTracer(t)

	_, span := StartToolCall(context.Background(), tracer, ToolCall{Name: "search"})
	EndToolCall(span, "", 0, errors.New("upstream timeout"))

	require.NoError(t, tracer.ForceFlush(context.Background()))

	spans := exporter.GetSpans()
	require.Len(t, spans, 1)
	assert.Equal(t, codes.Error, spans[0].Status.Code)
	assert.Equal(t, "upstream timeout", spans[0].Status.Description)
	require.Len(t, spans[0].Events, 1)
	assert.Equal(t, "exception", spans[0].Events[0].Name)
}

func TestOTelTracerResourceAttributes(t *testing.T) {
	tracer, exporter := newTestTracer(t)

	_, span := tracer.StartSpan(context.Background(), "custom", nil)
	span.End()

	require.NoError(t, tracer.ForceFlush(context.Background()))

	spans := exporter.GetSpans()
	require.Len(t, spans, 1)

	resourceAttrs := map[string]string{}
	for _, kv := range spans[0].Resource.Attributes() {
		resourceAttrs[string(kv.Key)] = kv.Value.Emit()
	}
	assert.Equal(t, "agno-test", resourceAttrs[AttrServiceName])
	assert.Equal(t, "test", resourceAttrs[AttrDeploymentEnv])
	assert.Equal(t, DefaultServiceName, resourceAttrs[AttrTelemetryFramework])
}

func TestNewOTelTracerFromProvider(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	t.Cleanup(func() { _ = provider.Shutdown(context.Background()) })

	tracer := NewOTelTracerFromProvider(provider)
	assert.Nil(t, tracer.Provider(), "external providers are not owned by the tracer")

	_, span := tracer.StartSpan(context.Background(), "external", map[string]any{AttrRunID: "run-1"})
	span.End()

	// Shutdown must not tear down a provider the tracer does not own.
	require.NoError(t, tracer.Shutdown(context.Background()))

	spans := exporter.GetSpans()
	require.Len(t, spans, 1)
	assert.Equal(t, "run-1", attrOf(t, spans[0], AttrRunID).AsString())
}

func TestToOTelAttributes(t *testing.T) {
	attrs := ToOTelAttributes(map[string]any{
		"string":   "value",
		"bool":     true,
		"int":      7,
		"int64":    int64(8),
		"float":    1.5,
		"slice":    []string{"a", "b"},
		"duration": 2 * time.Second,
		"error":    errors.New("boom"),
		"other":    struct{ A int }{A: 1},
		"nil":      nil,
	})

	got := map[string]attribute.Value{}
	for _, kv := range attrs {
		got[string(kv.Key)] = kv.Value
	}

	require.Len(t, got, 9, "nil values must be dropped")
	assert.Equal(t, "value", got["string"].AsString())
	assert.True(t, got["bool"].AsBool())
	assert.Equal(t, int64(7), got["int"].AsInt64())
	assert.Equal(t, int64(8), got["int64"].AsInt64())
	assert.Equal(t, 1.5, got["float"].AsFloat64())
	assert.Equal(t, []string{"a", "b"}, got["slice"].AsStringSlice())
	assert.Equal(t, float64(2000), got["duration"].AsFloat64())
	assert.Equal(t, "boom", got["error"].AsString())
	assert.Equal(t, "{1}", got["other"].AsString())

	assert.Nil(t, ToOTelAttributes(nil))
}

func TestNewSampler(t *testing.T) {
	assert.Equal(t, sdktrace.AlwaysSample().Description(), newSampler(0).Description())
	assert.Equal(t, sdktrace.AlwaysSample().Description(), newSampler(1).Description())
	assert.Contains(t, newSampler(0.25).Description(), "TraceIDRatioBased")
}
