package tools

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"autonomous-agent/internal/logger"
	"autonomous-agent/internal/requestid"
)

const defaultTimeout = 10 * time.Second

// Registry is the unified tool system: dynamic Register / Get / List / Execute.
type Registry interface {
	Register(tool Tool, risk RiskLevel) error
	Get(name string) (Tool, bool)
	List() []Info
	Execute(ctx context.Context, name string, input map[string]any) (ToolResult, error)
}

type registeredTool struct {
	tool Tool
	info Info
}

type registry struct {
	mu      sync.RWMutex
	tools   map[string]registeredTool
	timeout time.Duration
	logger  logger.Logger
}

// NewRegistry constructs an empty registry. Tools must be Register()'d dynamically.
func NewRegistry(log logger.Logger, timeout time.Duration) Registry {
	if log == nil {
		log = logger.New("info", "json", nil)
	}
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	return &registry{
		tools:   map[string]registeredTool{},
		timeout: timeout,
		logger:  log,
	}
}

// Register adds a tool. Name must be unique. Schema and risk are required.
func (r *registry) Register(tool Tool, risk RiskLevel) error {
	if r == nil {
		return newToolError(CodeInvalidTool, "", "", "registry is required", ErrInvalidTool)
	}
	if tool == nil {
		return newToolError(CodeInvalidTool, "", "", "tool is required", ErrInvalidTool)
	}

	name := normalizeName(tool.Name())
	if name == "" {
		return newToolError(CodeInvalidTool, "", "", "tool name is required", ErrInvalidTool)
	}
	if strings.TrimSpace(tool.Description()) == "" {
		return newToolError(CodeInvalidTool, name, "", "tool description is required", ErrInvalidTool)
	}
	if tool.InputSchema() == nil {
		return newToolError(CodeInvalidTool, name, "", "tool input schema is required", ErrInvalidTool)
	}
	if !validRisk(risk) {
		return newToolError(CodeInvalidRisk, name, "", fmt.Sprintf("unsupported risk level %q", risk), ErrInvalidRisk)
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.tools[name]; exists {
		return newToolError(CodeDuplicate, name, "", "already registered", ErrDuplicate)
	}
	r.tools[name] = registeredTool{
		tool: tool,
		info: Info{
			Name:        name,
			Description: strings.TrimSpace(tool.Description()),
			InputSchema: tool.InputSchema(),
			Risk:        risk,
		},
	}
	return nil
}

// Get returns a registered tool by name.
func (r *registry) Get(name string) (Tool, bool) {
	if r == nil {
		return nil, false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	item, ok := r.tools[normalizeName(name)]
	if !ok {
		return nil, false
	}
	return item.tool, true
}

// List returns registered tools sorted by name.
func (r *registry) List() []Info {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Info, 0, len(r.tools))
	for _, item := range r.tools {
		out = append(out, item.info)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Name < out[j].Name
	})
	return out
}

// Execute validates arguments, applies timeout, logs with trace_id, and runs the tool.
// It never fabricates a successful result: Data is only what the tool returned.
func (r *registry) Execute(ctx context.Context, name string, input map[string]any) (ToolResult, error) {
	started := time.Now()
	name = normalizeName(name)
	traceID := traceIDFrom(ctx)
	if input == nil {
		input = map[string]any{}
	}

	baseMeta := map[string]any{
		"trace_id": traceID,
		"tool":     name,
	}

	if r == nil {
		err := newToolError(CodeInvalidTool, name, traceID, "registry is required", ErrInvalidTool)
		return finish(failedResult(err, baseMeta), started), err
	}

	r.mu.RLock()
	item, ok := r.tools[name]
	timeout := r.timeout
	r.mu.RUnlock()
	if !ok {
		err := newToolError(CodeNotFound, name, traceID, "not registered", ErrNotFound)
		res := finish(failedResult(err, baseMeta), started)
		r.logExec(ctx, name, "", traceID, started, res, err)
		return res, err
	}

	baseMeta["risk"] = string(item.info.Risk)

	if err := ValidateInput(item.info.InputSchema, input); err != nil {
		toolErr := newToolError(CodeInvalidInput, name, traceID, err.Error(), ErrInvalidInput)
		res := finish(failedResult(toolErr, baseMeta), started)
		r.logExec(ctx, name, item.info.Risk, traceID, started, res, toolErr)
		return res, toolErr
	}

	if ctx == nil {
		ctx = context.Background()
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	r.logger.InfoContext(ctx, "tool execute started",
		"trace_id", traceID,
		"tool", name,
		"risk", item.info.Risk,
		"timeout", timeout.String(),
	)

	type outcome struct {
		result ToolResult
		err    error
	}
	done := make(chan outcome, 1)
	go func() {
		defer func() {
			if rec := recover(); rec != nil {
				te := newToolError(CodeExecutionFailed, name, traceID, fmt.Sprintf("panic: %v", rec), ErrExecutionFailed)
				done <- outcome{result: failedResult(te, baseMeta), err: te}
			}
		}()
		res, err := item.tool.Execute(runCtx, input)
		if err != nil && res.Error == "" {
			res.Success = false
			res.Error = err.Error()
		}
		if res.Metadata == nil {
			res.Metadata = map[string]any{}
		}
		for k, v := range baseMeta {
			if _, exists := res.Metadata[k]; !exists {
				res.Metadata[k] = v
			}
		}
		if err != nil {
			var te *ToolError
			switch {
			case errors.As(err, &te):
			case errors.Is(err, context.DeadlineExceeded):
				te = newToolError(CodeTimeout, name, traceID, err.Error(), ErrTimeout)
			case errors.Is(err, context.Canceled):
				te = newToolError(CodeCanceled, name, traceID, err.Error(), ErrCanceled)
			default:
				te = newToolError(CodeExecutionFailed, name, traceID, err.Error(), err)
			}
			if te.TraceID == "" {
				te.TraceID = traceID
			}
			if te.Tool == "" {
				te.Tool = name
			}
			done <- outcome{result: res, err: te}
			return
		}
		done <- outcome{result: res, err: nil}
	}()

	var out outcome
	select {
	case out = <-done:
	case <-runCtx.Done():
		select {
		case out = <-done:
		default:
			code := CodeCanceled
			sentinel := ErrCanceled
			msg := "canceled"
			if errors.Is(runCtx.Err(), context.DeadlineExceeded) {
				code = CodeTimeout
				sentinel = ErrTimeout
				msg = fmt.Sprintf("exceeded timeout %s", timeout)
			}
			te := newToolError(code, name, traceID, msg, sentinel)
			res := finish(failedResult(te, baseMeta), started)
			r.logExec(ctx, name, item.info.Risk, traceID, started, res, te)
			return res, te
		}
	}

	out.result = finish(out.result, started)
	if out.result.Metadata == nil {
		out.result.Metadata = cloneMetadata(baseMeta)
	} else {
		out.result.Metadata["trace_id"] = traceID
		out.result.Metadata["tool"] = name
		out.result.Metadata["risk"] = string(item.info.Risk)
	}
	r.logExec(ctx, name, item.info.Risk, traceID, started, out.result, out.err)
	return out.result, out.err
}

func (r *registry) logExec(ctx context.Context, name string, risk RiskLevel, traceID string, started time.Time, res ToolResult, err error) {
	if r == nil || r.logger == nil {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	args := []any{
		"trace_id", traceID,
		"request_id", requestid.From(ctx),
		"tool", name,
		"risk", risk,
		"success", res.Success,
		"duration_ms", time.Since(started).Milliseconds(),
	}
	if err != nil {
		args = append(args, "error", err)
		var te *ToolError
		if errors.As(err, &te) && te.Code != "" {
			args = append(args, "error_code", te.Code)
		}
		r.logger.ErrorContext(ctx, "tool execute finished", args...)
		return
	}
	r.logger.InfoContext(ctx, "tool execute finished", args...)
}

func finish(res ToolResult, started time.Time) ToolResult {
	if res.Duration == 0 {
		res.Duration = time.Since(started)
	}
	if res.Metadata == nil {
		res.Metadata = map[string]any{}
	}
	return res
}

func traceIDFrom(ctx context.Context) string {
	if ctx == nil {
		return requestid.New()
	}
	if id := strings.TrimSpace(requestid.From(ctx)); id != "" {
		return id
	}
	return requestid.New()
}

func normalizeName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

// RegisterBuiltins dynamically registers the built-in calculator tool.
func RegisterBuiltins(reg Registry) error {
	if reg == nil {
		return newToolError(CodeInvalidTool, "", "", "registry is required", ErrInvalidTool)
	}
	return reg.Register(NewCalculator(), RiskLow)
}
