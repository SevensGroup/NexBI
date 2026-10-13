package observability

import (
	"context"

	"github.com/google/uuid"
)

type ctxKey int

const (
	runIDKey ctxKey = iota
	stepIDKey
	toolCallIDKey
)

// NewID generates a correlation identifier (run / step / tool call).
func NewID() string {
	return uuid.NewString()
}

// WithRunID stores the Agent Run ID on ctx.
func WithRunID(ctx context.Context, id string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, runIDKey, id)
}

// RunID returns the Agent Run ID, or empty.
func RunID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	id, _ := ctx.Value(runIDKey).(string)
	return id
}

// WithStepID stores the Step ID on ctx.
func WithStepID(ctx context.Context, id string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, stepIDKey, id)
}

// StepID returns the Step ID, or empty.
func StepID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	id, _ := ctx.Value(stepIDKey).(string)
	return id
}

// WithToolCallID stores the Tool Call ID on ctx.
func WithToolCallID(ctx context.Context, id string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, toolCallIDKey, id)
}

// ToolCallID returns the Tool Call ID, or empty.
func ToolCallID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	id, _ := ctx.Value(toolCallIDKey).(string)
	return id
}

// Attrs returns structured log key/values for the correlation IDs on ctx.
func Attrs(ctx context.Context) []any {
	if ctx == nil {
		return nil
	}
	out := make([]any, 0, 6)
	if id := RunID(ctx); id != "" {
		out = append(out, "run_id", id)
	}
	if id := StepID(ctx); id != "" {
		out = append(out, "step_id", id)
	}
	if id := ToolCallID(ctx); id != "" {
		out = append(out, "tool_call_id", id)
	}
	return out
}
