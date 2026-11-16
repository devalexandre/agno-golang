package tracing

import (
	"context"
	"sync/atomic"
)

// globalTracer holds the process-wide Tracer. It defaults to NoopTracer so
// instrumented code can always call GlobalTracer() without a nil check.
var globalTracer atomic.Value // of tracerHolder

type tracerHolder struct{ tracer Tracer }

func init() {
	globalTracer.Store(tracerHolder{tracer: NoopTracer{}})
}

// SetGlobalTracer installs t as the process-wide Tracer. Passing nil resets the
// registry back to NoopTracer.
func SetGlobalTracer(t Tracer) {
	if t == nil {
		t = NoopTracer{}
	}
	globalTracer.Store(tracerHolder{tracer: t})
}

// GlobalTracer returns the process-wide Tracer. It never returns nil.
func GlobalTracer() Tracer {
	h, ok := globalTracer.Load().(tracerHolder)
	if !ok || h.tracer == nil {
		return NoopTracer{}
	}
	return h.tracer
}

// Enabled reports whether a real (non no-op) tracer is installed globally.
func Enabled() bool {
	_, isNoop := GlobalTracer().(NoopTracer)
	return !isNoop
}

// Shutdown flushes and shuts down the global tracer, then resets the registry
// back to NoopTracer.
func Shutdown(ctx context.Context) error {
	t := GlobalTracer()
	SetGlobalTracer(nil)
	return t.Shutdown(ctx)
}

// StartSpan starts a span on the global tracer.
func StartSpan(ctx context.Context, name string, attrs map[string]any) (context.Context, Span) {
	return GlobalTracer().StartSpan(ctx, name, attrs)
}

// tracerOrGlobal returns t, or the global tracer when t is nil.
func tracerOrGlobal(t Tracer) Tracer {
	if t == nil {
		return GlobalTracer()
	}
	return t
}
