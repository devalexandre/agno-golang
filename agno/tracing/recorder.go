package tracing

import (
	"context"
	"sync"
	"time"
)

// RecordedSpan is a span captured in memory by a Recorder.
type RecordedSpan struct {
	Name       string
	Attributes map[string]any
	Metrics    map[string]float64
	Errors     []error
	Parent     *RecordedSpan
	StartTime  time.Time
	EndTime    time.Time
	Ended      bool

	mu sync.Mutex
}

// SetAttribute implements Span.
func (s *RecordedSpan) SetAttribute(key string, value any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Attributes[key] = value
}

// RecordError implements Span.
func (s *RecordedSpan) RecordError(err error) {
	if err == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Errors = append(s.Errors, err)
}

// AddMetric implements Span.
func (s *RecordedSpan) AddMetric(name string, value float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Metrics[name] = value
}

// End implements Span.
func (s *RecordedSpan) End() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Ended {
		return
	}
	s.Ended = true
	s.EndTime = time.Now()
}

// Attribute returns the value of an attribute.
func (s *RecordedSpan) Attribute(key string) (any, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.Attributes[key]
	return v, ok
}

// Duration returns the span duration (zero while the span is open).
func (s *RecordedSpan) Duration() time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.Ended {
		return 0
	}
	return s.EndTime.Sub(s.StartTime)
}

// Failed reports whether at least one error was recorded on the span.
func (s *RecordedSpan) Failed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.Errors) > 0
}

var _ Span = (*RecordedSpan)(nil)

// Recorder is an in-memory Tracer that keeps every span it starts. It performs
// no I/O, which makes it the tracer of choice for tests and local debugging.
type Recorder struct {
	mu       sync.Mutex
	spans    []*RecordedSpan
	shutdown bool
}

// NewRecorder creates an empty Recorder.
func NewRecorder() *Recorder { return &Recorder{} }

type recorderCtxKey struct{}

// StartSpan implements Tracer. The span is linked to the span already present
// in ctx, if any, so parent/child relationships are preserved.
func (r *Recorder) StartSpan(ctx context.Context, name string, attrs map[string]any) (context.Context, Span) {
	span := &RecordedSpan{
		Name:       name,
		Attributes: make(map[string]any, len(attrs)),
		Metrics:    make(map[string]float64),
		StartTime:  time.Now(),
	}
	for k, v := range attrs {
		span.Attributes[k] = v
	}
	if parent, ok := ctx.Value(recorderCtxKey{}).(*RecordedSpan); ok {
		span.Parent = parent
	}

	r.mu.Lock()
	r.spans = append(r.spans, span)
	r.mu.Unlock()

	return context.WithValue(ctx, recorderCtxKey{}, span), span
}

// Shutdown implements Tracer.
func (r *Recorder) Shutdown(context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.shutdown = true
	return nil
}

// Spans returns every span recorded so far, in start order.
func (r *Recorder) Spans() []*RecordedSpan {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]*RecordedSpan, len(r.spans))
	copy(out, r.spans)
	return out
}

// SpansByName returns every recorded span with the given name.
func (r *Recorder) SpansByName(name string) []*RecordedSpan {
	var out []*RecordedSpan
	for _, s := range r.Spans() {
		if s.Name == name {
			out = append(out, s)
		}
	}
	return out
}

// Reset drops every recorded span.
func (r *Recorder) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.spans = nil
}

// IsShutdown reports whether Shutdown was called.
func (r *Recorder) IsShutdown() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.shutdown
}

var _ Tracer = (*Recorder)(nil)
