package telemetry

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const tracerName = "clawgo/session"

// tracer returns the session tracer from the global TracerProvider.
func tracer() trace.Tracer {
	return otel.Tracer(tracerName)
}

// StartInteractionSpan starts a span representing a user interaction (one
// prompt-response cycle). The caller must call span.End() when the
// interaction completes.
func StartInteractionSpan(ctx context.Context, sessionID string) (context.Context, trace.Span) {
	return tracer().Start(ctx, "interaction",
		trace.WithAttributes(
			attribute.String("session.id", sessionID),
		),
	)
}

// StartLLMRequestSpan starts a span representing an LLM API request. The
// caller must call span.End() when the request completes. Use RecordLLMResponse
// to attach response attributes before ending the span.
func StartLLMRequestSpan(ctx context.Context, model string, inputTokens int) (context.Context, trace.Span) {
	return tracer().Start(ctx, "llm.request",
		trace.WithAttributes(
			attribute.String("model.name", model),
			attribute.Int("llm.input_tokens", inputTokens),
		),
	)
}

// StartToolCallSpan starts a span representing a tool call execution. The
// caller must call span.End() when the tool call completes. Use RecordToolResult
// to attach result attributes before ending the span.
func StartToolCallSpan(ctx context.Context, toolName string) (context.Context, trace.Span) {
	return tracer().Start(ctx, "tool.call",
		trace.WithAttributes(
			attribute.String("tool.name", toolName),
		),
	)
}

// RecordLLMResponse attaches response attributes to an LLM request span.
// Call this before span.End().
func RecordLLMResponse(span trace.Span, outputTokens int, durationMs int64) {
	span.SetAttributes(
		attribute.Int("llm.output_tokens", outputTokens),
		attribute.Int64("llm.duration_ms", durationMs),
	)
}

// RecordToolResult attaches result attributes to a tool call span.
// If isError is true, the span status is set to Error. Call this before
// span.End().
func RecordToolResult(span trace.Span, isError bool, durationMs int64) {
	span.SetAttributes(
		attribute.Bool("tool.is_error", isError),
		attribute.Int64("tool.duration_ms", durationMs),
	)
	if isError {
		span.SetStatus(codes.Error, "tool execution failed")
	}
}
