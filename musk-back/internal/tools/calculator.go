package tools

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

const (
	calculatorName = "calculator"

	OpSum        = "sum"
	OpSubtract   = "subtract"
	OpMultiply   = "multiply"
	OpDivide     = "divide"
	OpPercentage = "percentage"
)

// Calculator performs arithmetic. Results are computed here, never invented by the LLM.
type Calculator struct{}

// NewCalculator constructs the built-in calculator tool.
func NewCalculator() *Calculator {
	return &Calculator{}
}

func (c *Calculator) Name() string { return calculatorName }

func (c *Calculator) Description() string {
	return "Perform arithmetic on two numbers. op is one of sum, subtract, multiply, divide, percentage. percentage(a, b) returns a percent of b (a * b / 100)."
}

func (c *Calculator) InputSchema() any {
	return map[string]any{
		"type":     "object",
		"required": []string{"op", "a", "b"},
		"properties": map[string]any{
			"op": map[string]any{
				"type":        "string",
				"description": "Arithmetic operation",
				"enum":        []string{OpSum, OpSubtract, OpMultiply, OpDivide, OpPercentage},
			},
			"a": map[string]any{
				"type":        "number",
				"description": "Left operand",
			},
			"b": map[string]any{
				"type":        "number",
				"description": "Right operand",
			},
		},
	}
}

func (c *Calculator) Execute(ctx context.Context, input map[string]any) (ToolResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		code := CodeCanceled
		sentinel := ErrCanceled
		if errors.Is(err, context.DeadlineExceeded) {
			code = CodeTimeout
			sentinel = ErrTimeout
		}
		te := newToolError(code, calculatorName, traceIDFrom(ctx), err.Error(), sentinel)
		return failedResult(te, nil), te
	}
	if input == nil {
		input = map[string]any{}
	}

	op := strings.ToLower(strings.TrimSpace(fmt.Sprint(input["op"])))
	a, okA := asFloat(input["a"])
	b, okB := asFloat(input["b"])
	if !okA || !okB {
		te := newToolError(CodeInvalidInput, calculatorName, traceIDFrom(ctx), "a and b must be numbers", ErrInvalidInput)
		return failedResult(te, nil), te
	}

	var value float64
	switch op {
	case OpSum:
		value = a + b
	case OpSubtract:
		value = a - b
	case OpMultiply:
		value = a * b
	case OpDivide:
		if b == 0 {
			te := newToolError(CodeDivisionByZero, calculatorName, traceIDFrom(ctx), "division by zero", ErrInvalidInput)
			return failedResult(te, map[string]any{"op": op, "a": a, "b": b}), te
		}
		value = a / b
	case OpPercentage:
		value = a * b / 100
	default:
		te := newToolError(CodeUnknownOperation, calculatorName, traceIDFrom(ctx), fmt.Sprintf("unsupported op %q", op), ErrInvalidInput)
		return failedResult(te, map[string]any{"op": op}), te
	}

	return ToolResult{
		Success: true,
		Data: map[string]any{
			"op":    op,
			"a":     a,
			"b":     b,
			"value": value,
		},
		Metadata: map[string]any{
			"tool": calculatorName,
			"op":   op,
		},
	}, nil
}
