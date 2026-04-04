package telemetry

import (
	"context"
	"os"
	"sync"
	"time"

	"github.com/khaledmoayad/clawgo/internal/app"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
)

const (
	serviceName     = "claude-code"
	shutdownTimeout = 5 * time.Second
)

// Config controls telemetry initialization.
type Config struct {
	// ServiceVersion is the version string reported in OTel resource attributes.
	ServiceVersion string
	// Enabled controls whether OTel providers are initialized.
	// When false, InitTelemetry returns a no-op shutdown function.
	Enabled bool
}

var (
	shutdownOnce sync.Once
	shutdownFn   func()
)

// InitTelemetry initializes OTel TracerProvider and MeterProvider based on
// environment variables. It returns a shutdown function that flushes pending
// spans and metrics within a 5-second timeout.
//
// Environment variables:
//   - OTEL_TRACES_EXPORTER: "otlp" for OTLP HTTP exporter, "console" for stdout, else no-op
//   - OTEL_METRICS_EXPORTER: "otlp" for OTLP HTTP exporter, else no-op
func InitTelemetry(ctx context.Context, cfg Config) (func(), error) {
	if !cfg.Enabled {
		noop := func() {}
		return noop, nil
	}

	res := resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceNameKey.String(serviceName),
		semconv.ServiceVersionKey.String(cfg.ServiceVersion),
	)

	// Set up trace exporter based on OTEL_TRACES_EXPORTER env var.
	var tp *sdktrace.TracerProvider
	tracesExporter := os.Getenv("OTEL_TRACES_EXPORTER")
	switch tracesExporter {
	case "otlp":
		exporter, err := otlptracehttp.New(ctx)
		if err != nil {
			return nil, err
		}
		tp = sdktrace.NewTracerProvider(
			sdktrace.WithResource(res),
			sdktrace.WithBatcher(exporter),
		)
	case "console":
		exporter, err := stdouttrace.New()
		if err != nil {
			return nil, err
		}
		tp = sdktrace.NewTracerProvider(
			sdktrace.WithResource(res),
			sdktrace.WithBatcher(exporter),
		)
	default:
		// No trace exporter configured; create provider with resource only.
		tp = sdktrace.NewTracerProvider(
			sdktrace.WithResource(res),
		)
	}
	otel.SetTracerProvider(tp)

	// Set up metrics exporter based on OTEL_METRICS_EXPORTER env var.
	var mp *metric.MeterProvider
	metricsExporter := os.Getenv("OTEL_METRICS_EXPORTER")
	switch metricsExporter {
	case "otlp":
		exporter, err := otlpmetrichttp.New(ctx)
		if err != nil {
			return nil, err
		}
		reader := metric.NewPeriodicReader(exporter)
		mp = metric.NewMeterProvider(
			metric.WithResource(res),
			metric.WithReader(reader),
		)
	default:
		// No metrics exporter configured; create provider with resource only.
		mp = metric.NewMeterProvider(
			metric.WithResource(res),
		)
	}
	otel.SetMeterProvider(mp)

	// Set global propagator for distributed tracing context propagation.
	otel.SetTextMapPropagator(propagation.TraceContext{})

	// Build shutdown function that flushes providers with a timeout.
	shutdown := func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()

		// Flush and shut down trace provider.
		_ = tp.Shutdown(shutdownCtx)
		// Flush and shut down meter provider.
		_ = mp.Shutdown(shutdownCtx)
	}

	// Store for package-level Shutdown().
	shutdownFn = shutdown

	// Register with graceful shutdown so telemetry is flushed on process exit.
	app.RegisterCleanup(func() {
		Shutdown()
	})

	return shutdown, nil
}

// Shutdown flushes pending telemetry data. Safe to call even if InitTelemetry
// was never called or telemetry was disabled. Calling multiple times is a no-op
// after the first invocation.
func Shutdown() {
	shutdownOnce.Do(func() {
		if shutdownFn != nil {
			shutdownFn()
		}
	})
}

// resetForTesting resets the package-level state so tests can call
// InitTelemetry multiple times without interference.
func resetForTesting() {
	shutdownOnce = sync.Once{}
	shutdownFn = nil
}
