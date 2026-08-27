// Package otelx provides optional OpenTelemetry bootstrap helpers.
//
// Nested module: go get github.com/boracomet/go-irmik/irmik/observe/otelx
//
//	shutdown, err := otelx.Setup(ctx, otelx.Options{Service: "myapp"})
//	defer shutdown(context.Background())
//
// Experimental: uses the OTLP HTTP exporter when OTEL_EXPORTER_OTLP_ENDPOINT is set;
// otherwise installs a TracerProvider that never samples (local noop-ish).
package otelx

import (
	"context"
	"fmt"
	"os"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"go.opentelemetry.io/otel/trace"
)

// Options configures tracing.
type Options struct {
	Service string
	// SampleRatio is the parent-based trace ratio (default 1.0).
	SampleRatio float64
}

// Setup installs a global TracerProvider. Returns a shutdown func.
func Setup(ctx context.Context, opts Options) (func(context.Context) error, error) {
	if opts.Service == "" {
		opts.Service = "irmik"
	}
	if opts.SampleRatio <= 0 {
		opts.SampleRatio = 1
	}
	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(opts.Service),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("otelx: resource: %w", err)
	}

	var tp *sdktrace.TracerProvider
	if ep := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"); ep != "" {
		exp, err := otlptracehttp.New(ctx)
		if err != nil {
			return nil, fmt.Errorf("otelx: otlp exporter: %w", err)
		}
		tp = sdktrace.NewTracerProvider(
			sdktrace.WithBatcher(exp),
			sdktrace.WithResource(res),
			sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(opts.SampleRatio))),
		)
	} else {
		tp = sdktrace.NewTracerProvider(
			sdktrace.WithResource(res),
			sdktrace.WithSampler(sdktrace.NeverSample()),
		)
	}

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})
	return func(ctx context.Context) error {
		cctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		return tp.Shutdown(cctx)
	}, nil
}

// Tracer returns a named tracer from the global provider.
func Tracer(name string) trace.Tracer {
	return otel.Tracer(name)
}
