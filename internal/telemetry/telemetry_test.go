package telemetry

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// setupTestTracer installs an in-memory span exporter as the global
// TracerProvider so test assertions can inspect recorded spans.
func setupTestTracer(t *testing.T) *tracetest.InMemoryExporter {
	t.Helper()
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	otel.SetTracerProvider(tp)
	t.Cleanup(func() {
		_ = tp.Shutdown(context.Background())
	})
	return exporter
}

// findAttribute searches a span stub's attributes for a given key.
func findAttribute(attrs []attribute.KeyValue, key string) (attribute.KeyValue, bool) {
	for _, a := range attrs {
		if string(a.Key) == key {
			return a, true
		}
	}
	return attribute.KeyValue{}, false
}

// --- InitTelemetry tests ---

func TestInitTelemetry_Disabled(t *testing.T) {
	resetForTesting()

	cfg := Config{
		ServiceVersion: "1.0.0-test",
		Enabled:        false,
	}

	shutdown, err := InitTelemetry(context.Background(), cfg)
	require.NoError(t, err)
	assert.NotNil(t, shutdown, "shutdown function should be non-nil even when disabled")

	// Should be safe to call the no-op shutdown.
	shutdown()
}

func TestInitTelemetry_ConsoleExporter(t *testing.T) {
	resetForTesting()

	t.Setenv("OTEL_TRACES_EXPORTER", "console")
	t.Setenv("OTEL_METRICS_EXPORTER", "")

	cfg := Config{
		ServiceVersion: "1.0.0-test",
		Enabled:        true,
	}

	shutdown, err := InitTelemetry(context.Background(), cfg)
	require.NoError(t, err)
	assert.NotNil(t, shutdown, "shutdown function should be non-nil with console exporter")

	// Clean up.
	shutdown()
}

func TestInitTelemetry_ShutdownIdempotent(t *testing.T) {
	resetForTesting()

	t.Setenv("OTEL_TRACES_EXPORTER", "")
	t.Setenv("OTEL_METRICS_EXPORTER", "")

	cfg := Config{
		ServiceVersion: "1.0.0-test",
		Enabled:        true,
	}

	shutdown, err := InitTelemetry(context.Background(), cfg)
	require.NoError(t, err)

	// Call Shutdown multiple times -- should not panic.
	Shutdown()
	Shutdown()

	// Also safe to call the returned shutdown directly after package Shutdown.
	shutdown()
}

// --- Session tracing tests ---

func TestStartInteractionSpan(t *testing.T) {
	exporter := setupTestTracer(t)

	ctx, span := StartInteractionSpan(context.Background(), "sess-abc123")
	assert.NotNil(t, ctx)
	span.End()

	spans := exporter.GetSpans()
	require.Len(t, spans, 1)
	assert.Equal(t, "interaction", spans[0].Name)

	attr, ok := findAttribute(spans[0].Attributes, "session.id")
	require.True(t, ok, "session.id attribute must be present")
	assert.Equal(t, "sess-abc123", attr.Value.AsString())
}

func TestStartLLMRequestSpan(t *testing.T) {
	exporter := setupTestTracer(t)

	ctx, span := StartLLMRequestSpan(context.Background(), "claude-3-opus", 1500)
	assert.NotNil(t, ctx)
	span.End()

	spans := exporter.GetSpans()
	require.Len(t, spans, 1)
	assert.Equal(t, "llm.request", spans[0].Name)

	modelAttr, ok := findAttribute(spans[0].Attributes, "model.name")
	require.True(t, ok, "model.name attribute must be present")
	assert.Equal(t, "claude-3-opus", modelAttr.Value.AsString())

	tokensAttr, ok := findAttribute(spans[0].Attributes, "llm.input_tokens")
	require.True(t, ok, "llm.input_tokens attribute must be present")
	assert.Equal(t, int64(1500), tokensAttr.Value.AsInt64())
}

func TestStartToolCallSpan(t *testing.T) {
	exporter := setupTestTracer(t)

	ctx, span := StartToolCallSpan(context.Background(), "BashTool")
	assert.NotNil(t, ctx)
	span.End()

	spans := exporter.GetSpans()
	require.Len(t, spans, 1)
	assert.Equal(t, "tool.call", spans[0].Name)

	attr, ok := findAttribute(spans[0].Attributes, "tool.name")
	require.True(t, ok, "tool.name attribute must be present")
	assert.Equal(t, "BashTool", attr.Value.AsString())
}

func TestRecordLLMResponse(t *testing.T) {
	exporter := setupTestTracer(t)

	_, span := StartLLMRequestSpan(context.Background(), "claude-3-sonnet", 500)
	RecordLLMResponse(span, 1200, 3500)
	span.End()

	spans := exporter.GetSpans()
	require.Len(t, spans, 1)

	outputAttr, ok := findAttribute(spans[0].Attributes, "llm.output_tokens")
	require.True(t, ok, "llm.output_tokens attribute must be present")
	assert.Equal(t, int64(1200), outputAttr.Value.AsInt64())

	durationAttr, ok := findAttribute(spans[0].Attributes, "llm.duration_ms")
	require.True(t, ok, "llm.duration_ms attribute must be present")
	assert.Equal(t, int64(3500), durationAttr.Value.AsInt64())
}

func TestRecordToolResult_Success(t *testing.T) {
	exporter := setupTestTracer(t)

	_, span := StartToolCallSpan(context.Background(), "FileReadTool")
	RecordToolResult(span, false, 150)
	span.End()

	spans := exporter.GetSpans()
	require.Len(t, spans, 1)

	isErrAttr, ok := findAttribute(spans[0].Attributes, "tool.is_error")
	require.True(t, ok, "tool.is_error attribute must be present")
	assert.False(t, isErrAttr.Value.AsBool())

	durationAttr, ok := findAttribute(spans[0].Attributes, "tool.duration_ms")
	require.True(t, ok, "tool.duration_ms attribute must be present")
	assert.Equal(t, int64(150), durationAttr.Value.AsInt64())

	// Status should not be Error on success.
	assert.NotEqual(t, codes.Error, spans[0].Status.Code)
}

func TestRecordToolResult_Error(t *testing.T) {
	exporter := setupTestTracer(t)

	_, span := StartToolCallSpan(context.Background(), "BashTool")
	RecordToolResult(span, true, 2000)
	span.End()

	spans := exporter.GetSpans()
	require.Len(t, spans, 1)

	isErrAttr, ok := findAttribute(spans[0].Attributes, "tool.is_error")
	require.True(t, ok, "tool.is_error attribute must be present")
	assert.True(t, isErrAttr.Value.AsBool())

	// Status should be Error.
	assert.Equal(t, codes.Error, spans[0].Status.Code)
	assert.Equal(t, "tool execution failed", spans[0].Status.Description)
}
