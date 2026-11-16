package tracing

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	oteltrace "go.opentelemetry.io/otel/trace"
)

// OTelConfig configures an OTelTracer.
type OTelConfig struct {
	// ServiceName reported as the OpenTelemetry resource service name.
	// Defaults to DefaultServiceName.
	ServiceName string
	// ServiceVersion reported as the resource service version (optional).
	ServiceVersion string
	// Environment reported as deployment.environment (optional).
	Environment string
	// ResourceAttributes are extra resource attributes (optional).
	ResourceAttributes map[string]string

	// Endpoint of the OTLP/HTTP collector, e.g. "localhost:4318". When empty,
	// the standard OTEL_EXPORTER_OTLP_ENDPOINT environment variable is used.
	Endpoint string
	// URLPath overrides the OTLP/HTTP traces path (default "/v1/traces").
	URLPath string
	// Insecure sends traces over plain HTTP instead of HTTPS.
	Insecure bool
	// Headers are sent with every OTLP request (e.g. auth tokens).
	Headers map[string]string
	// ExportTimeout bounds a single export attempt (default 10s).
	ExportTimeout time.Duration

	// SampleRatio is the head-sampling ratio in [0,1]. Zero means "always
	// sample" (ratios must be explicit to disable spans).
	SampleRatio float64

	// Batch enables the batching span processor (recommended in production).
	// When false, spans are exported synchronously as they end.
	Batch bool
	// BatchTimeout is the delay between batch exports (default 5s).
	BatchTimeout time.Duration
	// MaxQueueSize / MaxExportBatchSize tune the batch processor.
	MaxQueueSize       int
	MaxExportBatchSize int

	// Exporter overrides the OTLP exporter entirely. Useful for tests
	// (in-memory exporters) or custom backends.
	Exporter sdktrace.SpanExporter

	// TracerProvider, when set, is used as-is: no exporter/provider is created
	// and Shutdown() will not shut it down.
	TracerProvider oteltrace.TracerProvider

	// SetGlobalProvider also installs the created provider as the OpenTelemetry
	// global provider (and a W3C trace-context propagator).
	SetGlobalProvider bool
}

func (c OTelConfig) withDefaults() OTelConfig {
	if c.ServiceName == "" {
		c.ServiceName = DefaultServiceName
	}
	if c.ExportTimeout <= 0 {
		c.ExportTimeout = 10 * time.Second
	}
	if c.BatchTimeout <= 0 {
		c.BatchTimeout = 5 * time.Second
	}
	if c.MaxQueueSize <= 0 {
		c.MaxQueueSize = 2048
	}
	if c.MaxExportBatchSize <= 0 {
		c.MaxExportBatchSize = 512
	}
	return c
}

// OTelTracer is a Tracer backed by the OpenTelemetry SDK. Spans are exported
// through OTLP/HTTP by default, and any OpenTelemetry-compatible backend
// (Jaeger, Tempo, Langfuse, Phoenix, ...) can consume them.
type OTelTracer struct {
	tracer   oteltrace.Tracer
	provider *sdktrace.TracerProvider // nil when an external provider is used
}

// NewOTelTracer builds an OTelTracer from cfg, creating the exporter, the
// resource and the tracer provider as needed.
func NewOTelTracer(ctx context.Context, cfg OTelConfig) (*OTelTracer, error) {
	cfg = cfg.withDefaults()

	// External provider: use it as-is.
	if cfg.TracerProvider != nil {
		return &OTelTracer{tracer: cfg.TracerProvider.Tracer(TracerName)}, nil
	}

	exporter := cfg.Exporter
	if exporter == nil {
		var err error
		exporter, err = newOTLPExporter(ctx, cfg)
		if err != nil {
			return nil, fmt.Errorf("tracing: failed to create OTLP exporter: %w", err)
		}
	}

	res, err := newResource(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("tracing: failed to create resource: %w", err)
	}

	opts := []sdktrace.TracerProviderOption{
		sdktrace.WithResource(res),
		sdktrace.WithSampler(newSampler(cfg.SampleRatio)),
	}
	if cfg.Batch {
		opts = append(opts, sdktrace.WithBatcher(exporter,
			sdktrace.WithBatchTimeout(cfg.BatchTimeout),
			sdktrace.WithMaxQueueSize(cfg.MaxQueueSize),
			sdktrace.WithMaxExportBatchSize(cfg.MaxExportBatchSize),
		))
	} else {
		opts = append(opts, sdktrace.WithSyncer(exporter))
	}

	provider := sdktrace.NewTracerProvider(opts...)
	if cfg.SetGlobalProvider {
		otel.SetTracerProvider(provider)
		otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{}, propagation.Baggage{},
		))
	}

	return &OTelTracer{tracer: provider.Tracer(TracerName), provider: provider}, nil
}

// NewOTelTracerFromProvider wraps an existing OpenTelemetry TracerProvider.
// Shutdown() does not shut the provider down: its owner remains responsible.
func NewOTelTracerFromProvider(tp oteltrace.TracerProvider) *OTelTracer {
	if tp == nil {
		tp = otel.GetTracerProvider()
	}
	return &OTelTracer{tracer: tp.Tracer(TracerName)}
}

// StartSpan implements Tracer.
func (t *OTelTracer) StartSpan(ctx context.Context, name string, attrs map[string]any) (context.Context, Span) {
	ctx, span := t.tracer.Start(ctx, name,
		oteltrace.WithSpanKind(oteltrace.SpanKindInternal),
		oteltrace.WithAttributes(ToOTelAttributes(attrs)...),
	)
	return ctx, &otelSpan{span: span}
}

// Shutdown implements Tracer: it flushes and shuts down the owned provider.
func (t *OTelTracer) Shutdown(ctx context.Context) error {
	if t.provider == nil {
		return nil
	}
	if err := t.provider.ForceFlush(ctx); err != nil {
		return err
	}
	return t.provider.Shutdown(ctx)
}

// ForceFlush exports every span still pending in the batch processor.
func (t *OTelTracer) ForceFlush(ctx context.Context) error {
	if t.provider == nil {
		return nil
	}
	return t.provider.ForceFlush(ctx)
}

// Provider returns the owned TracerProvider, or nil when the tracer was built
// on top of an external provider.
func (t *OTelTracer) Provider() *sdktrace.TracerProvider { return t.provider }

var _ Tracer = (*OTelTracer)(nil)

// otelSpan adapts an OpenTelemetry span to the Span interface.
type otelSpan struct {
	span oteltrace.Span
}

func (s *otelSpan) SetAttribute(key string, value any) {
	s.span.SetAttributes(toOTelAttribute(key, value))
}

func (s *otelSpan) RecordError(err error) {
	if err == nil {
		return
	}
	s.span.RecordError(err)
	s.span.SetStatus(codes.Error, err.Error())
}

// AddMetric records a numeric metric as a span attribute, which is how OTLP
// backends surface token/cost/latency values on the span itself.
func (s *otelSpan) AddMetric(name string, value float64) {
	s.span.SetAttributes(attribute.Float64(name, value))
}

func (s *otelSpan) End() { s.span.End() }

// OTelSpan exposes the underlying OpenTelemetry span, when the Span was
// produced by an OTelTracer.
func (s *otelSpan) OTelSpan() oteltrace.Span { return s.span }

var _ Span = (*otelSpan)(nil)

// ToOTelAttributes converts a generic attribute map into OpenTelemetry
// key/value pairs.
func ToOTelAttributes(attrs map[string]any) []attribute.KeyValue {
	if len(attrs) == 0 {
		return nil
	}
	out := make([]attribute.KeyValue, 0, len(attrs))
	for k, v := range attrs {
		if v == nil {
			continue
		}
		out = append(out, toOTelAttribute(k, v))
	}
	return out
}

func toOTelAttribute(key string, value any) attribute.KeyValue {
	switch v := value.(type) {
	case string:
		return attribute.String(key, v)
	case bool:
		return attribute.Bool(key, v)
	case int:
		return attribute.Int64(key, int64(v))
	case int32:
		return attribute.Int64(key, int64(v))
	case int64:
		return attribute.Int64(key, v)
	case uint:
		return attribute.Int64(key, int64(v))
	case uint32:
		return attribute.Int64(key, int64(v))
	case uint64:
		return attribute.Int64(key, int64(v))
	case float32:
		return attribute.Float64(key, float64(v))
	case float64:
		return attribute.Float64(key, v)
	case []string:
		return attribute.StringSlice(key, v)
	case []int:
		return attribute.IntSlice(key, v)
	case []float64:
		return attribute.Float64Slice(key, v)
	case time.Duration:
		return attribute.Float64(key, float64(v.Milliseconds()))
	case error:
		return attribute.String(key, v.Error())
	default:
		return attribute.String(key, fmt.Sprintf("%v", v))
	}
}

// newOTLPExporter builds the OTLP/HTTP span exporter described by cfg.
func newOTLPExporter(ctx context.Context, cfg OTelConfig) (sdktrace.SpanExporter, error) {
	opts := []otlptracehttp.Option{otlptracehttp.WithTimeout(cfg.ExportTimeout)}
	if cfg.Endpoint != "" {
		opts = append(opts, otlptracehttp.WithEndpoint(cfg.Endpoint))
	}
	if cfg.URLPath != "" {
		opts = append(opts, otlptracehttp.WithURLPath(cfg.URLPath))
	}
	if cfg.Insecure {
		opts = append(opts, otlptracehttp.WithInsecure())
	}
	if len(cfg.Headers) > 0 {
		opts = append(opts, otlptracehttp.WithHeaders(cfg.Headers))
	}
	return otlptracehttp.New(ctx, opts...)
}

// newResource builds the OpenTelemetry resource describing this service.
func newResource(ctx context.Context, cfg OTelConfig) (*resource.Resource, error) {
	attrs := []attribute.KeyValue{
		attribute.String(AttrServiceName, cfg.ServiceName),
		attribute.String(AttrTelemetryFramework, DefaultServiceName),
	}
	if cfg.ServiceVersion != "" {
		attrs = append(attrs, attribute.String(AttrServiceVersion, cfg.ServiceVersion))
	}
	if cfg.Environment != "" {
		attrs = append(attrs, attribute.String(AttrDeploymentEnv, cfg.Environment))
	}
	for k, v := range cfg.ResourceAttributes {
		attrs = append(attrs, attribute.String(k, v))
	}
	return resource.New(ctx, resource.WithAttributes(attrs...))
}

// newSampler maps a sampling ratio to an OpenTelemetry sampler.
func newSampler(ratio float64) sdktrace.Sampler {
	switch {
	case ratio <= 0:
		return sdktrace.AlwaysSample()
	case ratio >= 1:
		return sdktrace.AlwaysSample()
	default:
		return sdktrace.ParentBased(sdktrace.TraceIDRatioBased(ratio))
	}
}
