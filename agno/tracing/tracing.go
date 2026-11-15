// Package tracing provides the observability layer of agno-golang.
//
// It mirrors the contract of agno-python/tracing (setup, exporter, schemas):
// agent/team/workflow runs, model calls and tool executions are instrumented
// with spans, standardized attributes and metrics (tokens, cost, latency),
// exportable to any OpenTelemetry-compatible backend.
//
// The package ships three Tracer implementations:
//
//   - NoopTracer: zero-cost default, used when tracing is disabled.
//   - OTelTracer: real OpenTelemetry integration (OTLP/HTTP exporter or any
//     custom SpanExporter / TracerProvider).
//   - Recorder: in-memory tracer used by tests and local debugging.
package tracing

import "context"

// Tracer starts spans and manages the tracing lifecycle.
type Tracer interface {
	// StartSpan starts a new span with the given name and attributes, returning
	// the derived context and the Span. The caller must call Span.End().
	StartSpan(ctx context.Context, name string, attrs map[string]any) (context.Context, Span)

	// Shutdown flushes and shuts down the tracer/exporter.
	Shutdown(ctx context.Context) error
}

// Span represents an instrumented unit of work.
type Span interface {
	// SetAttribute adds/updates an attribute on the span.
	SetAttribute(key string, value any)

	// RecordError records an error associated with the span.
	RecordError(err error)

	// AddMetric records a numeric metric (e.g. "tokens_in", "cost_usd").
	AddMetric(name string, value float64)

	// End ends the span.
	End()
}

// NoopTracer is the default (no-op) implementation used when tracing is
// disabled. It guarantees instrumented code never has to deal with nil.
type NoopTracer struct{}

// StartSpan implements Tracer.
func (NoopTracer) StartSpan(ctx context.Context, _ string, _ map[string]any) (context.Context, Span) {
	return ctx, noopSpan{}
}

// Shutdown implements Tracer.
func (NoopTracer) Shutdown(context.Context) error { return nil }

type noopSpan struct{}

func (noopSpan) SetAttribute(string, any)  {}
func (noopSpan) RecordError(error)         {}
func (noopSpan) AddMetric(string, float64) {}
func (noopSpan) End()                      {}

// Ensure interface compliance.
var _ Tracer = NoopTracer{}
var _ Span = noopSpan{}

// SetAttributes applies every key/value of attrs to the span.
// Nil-safe: a nil span or nil map is a no-op.
func SetAttributes(span Span, attrs map[string]any) {
	if span == nil || len(attrs) == 0 {
		return
	}
	for k, v := range attrs {
		if v == nil {
			continue
		}
		span.SetAttribute(k, v)
	}
}
