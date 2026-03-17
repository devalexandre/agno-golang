package telemetry

import (
	"context"
	"fmt"
	"os"
	"sync"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
	"go.opentelemetry.io/otel/trace"
)

var (
	tp   *sdktrace.TracerProvider
	once sync.Once
)

// InitTracer initializes an OTLP exporter, and configures the corresponding trace provider.
func InitTracer(serviceName string) (*sdktrace.TracerProvider, error) {
	var err error
	once.Do(func() {
		ctx := context.Background()

		endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
		if endpoint == "" {
			endpoint = "localhost:4318"
		}

		exporter, exporterErr := otlptracehttp.New(ctx,
			otlptracehttp.WithEndpoint(endpoint),
			otlptracehttp.WithInsecure(),
		)
		if exporterErr != nil {
			err = fmt.Errorf("failed to create OTLP trace exporter: %w", exporterErr)
			return
		}

		res, resErr := resource.New(ctx,
			resource.WithAttributes(
				semconv.ServiceNameKey.String(serviceName),
			),
		)
		if resErr != nil {
			err = fmt.Errorf("failed to create resource: %w", resErr)
			return
		}

		tp = sdktrace.NewTracerProvider(
			sdktrace.WithBatcher(exporter),
			sdktrace.WithResource(res),
		)
		otel.SetTracerProvider(tp)
	})

	return tp, err
}

// GetTracer returns a tracer from the global provider.
func GetTracer(name string) trace.Tracer {
	return otel.GetTracerProvider().Tracer(name)
}

// StartSpan starts a new span from the given context.
func StartSpan(ctx context.Context, tracerName string, spanName string) (context.Context, trace.Span) {
	return GetTracer(tracerName).Start(ctx, spanName)
}

// Shutdown shuts down the tracer provider.
func Shutdown(ctx context.Context) error {
	if tp != nil {
		return tp.Shutdown(ctx)
	}
	return nil
}
