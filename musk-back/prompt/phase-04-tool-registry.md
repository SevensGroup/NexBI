Continue from Phase 3.

Implement a unified Tool Registry.

Directory:

internal/tools/

Implement:

Tool interface:

type Tool interface {
    Name() string
    Description() string
    InputSchema() any
    Execute(ctx context.Context, input map[string]any) (ToolResult, error)
}

ToolResult:

type ToolResult struct {
    Success bool
    Data any
    Error string
    Metadata map[string]any
    Duration time.Duration
}

Implement:

ToolRegistry:

Register
Get
List
Execute

Requirements:

1. Tools must be registered dynamically.
2. The agent must not depend on a concrete tool.
3. Every tool must have an input schema.
4. Arguments must be validated.
5. Tool execution must have a timeout.
6. Tool execution must be logged.
7. Tool execution must have a trace_id.
8. Tool errors must be structured.
9. A tool must not fabricate a result.
10. Support tool risk levels:

LOW
MEDIUM
HIGH
CRITICAL

Add:

calculator tool

Support:

- sum
- subtract
- multiply
- divide
- percentage

Verify with the calculator tool:

Agent
→ Decision
→ Tool Registry
→ Calculator
→ Observation
→ Decision

Finish with complete tests.
