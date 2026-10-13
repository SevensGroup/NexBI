package tools

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"autonomous-agent/internal/logger"
	"autonomous-agent/internal/requestid"
)

func testRegistry(t *testing.T, timeout time.Duration) Registry {
	t.Helper()
	if timeout <= 0 {
		timeout = time.Second
	}
	reg := NewRegistry(logger.New("error", "json", io.Discard), timeout)
	if err := RegisterBuiltins(reg); err != nil {
		t.Fatalf("RegisterBuiltins: %v", err)
	}
	return reg
}

func TestRegistryRegisterGetList(t *testing.T) {
	reg := testRegistry(t, time.Second)

	got, ok := reg.Get("calculator")
	if !ok || got == nil || got.Name() != "calculator" {
		t.Fatalf("Get calculator ok=%v tool=%v", ok, got)
	}
	if _, ok := reg.Get("missing"); ok {
		t.Fatal("Get missing should be false")
	}

	list := reg.List()
	if len(list) != 1 {
		t.Fatalf("List len = %d", len(list))
	}
	if list[0].Name != "calculator" {
		t.Fatalf("name = %q", list[0].Name)
	}
	if list[0].Risk != RiskLow {
		t.Fatalf("risk = %s", list[0].Risk)
	}
	if list[0].InputSchema == nil {
		t.Fatal("input schema is required")
	}
	if list[0].Description == "" {
		t.Fatal("description is required")
	}
}

func TestRegistryRejectsDuplicateAndInvalid(t *testing.T) {
	reg := NewRegistry(logger.New("error", "json", io.Discard), time.Second)

	if err := reg.Register(nil, RiskLow); !errors.Is(err, ErrInvalidTool) {
		t.Fatalf("nil tool err = %v", err)
	}
	if err := RegisterBuiltins(reg); err != nil {
		t.Fatalf("RegisterBuiltins: %v", err)
	}
	if err := RegisterBuiltins(reg); !errors.Is(err, ErrDuplicate) {
		t.Fatalf("duplicate err = %v", err)
	}
	if err := reg.Register(NewCalculator(), "NOPE"); !errors.Is(err, ErrInvalidRisk) {
		t.Fatalf("invalid risk err = %v", err)
	}
}

func TestRegistryExecuteCalculator(t *testing.T) {
	reg := testRegistry(t, time.Second)
	ctx := requestid.With(context.Background(), "trace-calc-1")

	res, err := reg.Execute(ctx, "calculator", map[string]any{"op": "sum", "a": 2, "b": 3})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !res.Success {
		t.Fatalf("success = false error=%s", res.Error)
	}
	data, _ := res.Data.(map[string]any)
	if data["value"] != 5.0 {
		t.Fatalf("value = %v", data["value"])
	}
	if res.Metadata["trace_id"] != "trace-calc-1" {
		t.Fatalf("trace_id = %v", res.Metadata["trace_id"])
	}
	if res.Duration <= 0 {
		t.Fatal("duration must be recorded from real execution")
	}
}

func TestRegistryValidateInput(t *testing.T) {
	reg := testRegistry(t, time.Second)
	_, err := reg.Execute(context.Background(), "calculator", map[string]any{"op": "sum", "a": 1})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("err = %v, want ErrInvalidInput", err)
	}
	var te *ToolError
	if !errors.As(err, &te) || te.Code != CodeInvalidInput {
		t.Fatalf("structured error = %+v", err)
	}
}

func TestRegistryUnknownTool(t *testing.T) {
	reg := testRegistry(t, time.Second)
	res, err := reg.Execute(context.Background(), "nope", map[string]any{})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v", err)
	}
	if res.Success || res.Error == "" {
		t.Fatalf("result = %+v", res)
	}
	if res.Metadata["error_code"] != CodeNotFound {
		t.Fatalf("metadata = %v", res.Metadata)
	}
}

func TestRegistryTimeout(t *testing.T) {
	reg := NewRegistry(logger.New("error", "json", io.Discard), 30*time.Millisecond)
	slow := &slowTool{delay: 400 * time.Millisecond}
	if err := reg.Register(slow, RiskMedium); err != nil {
		t.Fatalf("Register: %v", err)
	}

	res, err := reg.Execute(context.Background(), "slow", map[string]any{})
	if !errors.Is(err, ErrTimeout) {
		t.Fatalf("err = %v, want ErrTimeout", err)
	}
	if res.Success {
		t.Fatal("timeout must not fabricate success")
	}
	var te *ToolError
	if !errors.As(err, &te) || te.Code != CodeTimeout || te.TraceID == "" {
		t.Fatalf("structured timeout = %+v", err)
	}
}

func TestRegistryDoesNotFabricateResult(t *testing.T) {
	reg := testRegistry(t, time.Second)
	res, err := reg.Execute(context.Background(), "calculator", map[string]any{
		"op": "divide", "a": 1, "b": 0,
	})
	if err == nil || res.Success {
		t.Fatalf("divide by zero must fail, res=%+v err=%v", res, err)
	}
	if res.Data != nil {
		t.Fatalf("failed execution must not invent data, got %v", res.Data)
	}
}

func TestRiskLevels(t *testing.T) {
	for _, level := range []RiskLevel{RiskLow, RiskMedium, RiskHigh, RiskCritical} {
		if _, err := ParseRiskLevel(string(level)); err != nil {
			t.Fatalf("ParseRiskLevel(%s): %v", level, err)
		}
	}
	if _, err := ParseRiskLevel("extreme"); !errors.Is(err, ErrInvalidRisk) {
		t.Fatalf("err = %v", err)
	}

	reg := NewRegistry(logger.New("error", "json", io.Discard), time.Second)
	if err := reg.Register(&namedSchemaTool{name: "audit", desc: "n", schema: map[string]any{"type": "object"}}, RiskCritical); err != nil {
		t.Fatalf("Register: %v", err)
	}
	list := reg.List()
	if len(list) != 1 || list[0].Risk != RiskCritical {
		t.Fatalf("list = %+v", list)
	}
}

type slowTool struct {
	delay time.Duration
}

func (s *slowTool) Name() string        { return "slow" }
func (s *slowTool) Description() string { return "sleeps" }
func (s *slowTool) InputSchema() any {
	return map[string]any{"type": "object"}
}
func (s *slowTool) Execute(ctx context.Context, _ map[string]any) (ToolResult, error) {
	time.Sleep(s.delay)
	if err := ctx.Err(); err != nil {
		return ToolResult{Success: false, Error: err.Error()}, err
	}
	return ToolResult{Success: true, Data: "done"}, nil
}

type namedSchemaTool struct {
	name   string
	desc   string
	schema any
}

func (t *namedSchemaTool) Name() string        { return t.name }
func (t *namedSchemaTool) Description() string { return t.desc }
func (t *namedSchemaTool) InputSchema() any    { return t.schema }
func (t *namedSchemaTool) Execute(context.Context, map[string]any) (ToolResult, error) {
	return ToolResult{Success: true, Data: true}, nil
}

func TestValidateInputSchema(t *testing.T) {
	schema := NewCalculator().InputSchema()
	if err := ValidateInput(schema, map[string]any{"op": "sum", "a": 1, "b": 2}); err != nil {
		t.Fatalf("valid input: %v", err)
	}
	if err := ValidateInput(schema, map[string]any{"op": "power", "a": 1, "b": 2}); err == nil {
		t.Fatal("expected enum rejection")
	}
	if err := ValidateInput(nil, map[string]any{}); err == nil {
		t.Fatal("nil schema must fail")
	}
}

func TestCalculatorOperations(t *testing.T) {
	c := NewCalculator()
	ctx := context.Background()
	cases := []struct {
		op      string
		a, b    float64
		want    float64
		wantErr string
	}{
		{OpSum, 2, 3, 5, ""},
		{OpSubtract, 9, 4, 5, ""},
		{OpMultiply, 3, 4, 12, ""},
		{OpDivide, 8, 2, 4, ""},
		{OpPercentage, 10, 50, 5, ""},
		{OpDivide, 1, 0, 0, CodeDivisionByZero},
	}
	for _, tc := range cases {
		res, err := c.Execute(ctx, map[string]any{"op": tc.op, "a": tc.a, "b": tc.b})
		if tc.wantErr != "" {
			if err == nil || res.Success {
				t.Fatalf("%s: expected error", tc.op)
			}
			var te *ToolError
			if !errors.As(err, &te) || te.Code != tc.wantErr {
				t.Fatalf("%s: err = %v", tc.op, err)
			}
			continue
		}
		if err != nil {
			t.Fatalf("%s: %v", tc.op, err)
		}
		data, _ := res.Data.(map[string]any)
		if data["value"] != tc.want {
			t.Fatalf("%s: value = %v want %v", tc.op, data["value"], tc.want)
		}
	}
}

func TestCalculatorRejectsUnknownOp(t *testing.T) {
	_, err := NewCalculator().Execute(context.Background(), map[string]any{"op": "mod", "a": 1, "b": 1})
	var te *ToolError
	if !errors.As(err, &te) || te.Code != CodeUnknownOperation {
		t.Fatalf("err = %v", err)
	}
}

func TestToolErrorStructured(t *testing.T) {
	err := newToolError(CodeNotFound, "x", "tid", "missing", ErrNotFound)
	if !strings.Contains(err.Error(), "trace_id=tid") {
		t.Fatalf("error = %q", err.Error())
	}
	if !errors.Is(err, ErrNotFound) {
		t.Fatal("must unwrap ErrNotFound")
	}
}
