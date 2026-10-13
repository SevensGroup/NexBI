package tools

import (
	"errors"
	"fmt"
	"strings"
)

const (
	CodeNotFound         = "TOOL_NOT_FOUND"
	CodeDuplicate        = "TOOL_DUPLICATE"
	CodeInvalidTool      = "TOOL_INVALID"
	CodeInvalidRisk      = "TOOL_INVALID_RISK"
	CodeInvalidInput     = "TOOL_INVALID_INPUT"
	CodeTimeout          = "TOOL_TIMEOUT"
	CodeCanceled         = "TOOL_CANCELED"
	CodeExecutionFailed  = "TOOL_EXECUTION_FAILED"
	CodeUnknownOperation = "TOOL_UNKNOWN_OPERATION"
	CodeDivisionByZero   = "TOOL_DIVISION_BY_ZERO"
)

var (
	// ErrNotFound is returned when Execute/Get cannot locate a tool.
	ErrNotFound = errors.New("tool not found")
	// ErrDuplicate is returned when Register is called twice for the same name.
	ErrDuplicate = errors.New("tool already registered")
	// ErrInvalidTool is returned when a tool is missing required metadata.
	ErrInvalidTool = errors.New("invalid tool")
	// ErrInvalidRisk is returned when a risk level is not LOW/MEDIUM/HIGH/CRITICAL.
	ErrInvalidRisk = errors.New("invalid tool risk level")
	// ErrInvalidInput is returned when arguments fail InputSchema validation.
	ErrInvalidInput = errors.New("invalid tool input")
	// ErrTimeout is returned when execution exceeds the configured timeout.
	ErrTimeout = errors.New("tool execution timeout")
	// ErrCanceled is returned when the caller context is canceled.
	ErrCanceled = errors.New("tool execution canceled")
	// ErrExecutionFailed is returned when a tool panics or returns an execution error.
	ErrExecutionFailed = errors.New("tool execution failed")
)

// ToolError is a structured tool failure. It is safe to log and to store as an observation.
type ToolError struct {
	Code    string
	Tool    string
	Message string
	TraceID string
	Err     error
}

func (e *ToolError) Error() string {
	if e == nil {
		return ""
	}
	tool := strings.TrimSpace(e.Tool)
	msg := strings.TrimSpace(e.Message)
	if msg == "" && e.Err != nil {
		msg = e.Err.Error()
	}
	code := strings.TrimSpace(e.Code)
	if code == "" {
		code = CodeExecutionFailed
	}
	switch {
	case tool != "" && e.TraceID != "":
		return fmt.Sprintf("%s: tool %q: %s (trace_id=%s)", code, tool, msg, e.TraceID)
	case tool != "":
		return fmt.Sprintf("%s: tool %q: %s", code, tool, msg)
	case e.TraceID != "":
		return fmt.Sprintf("%s: %s (trace_id=%s)", code, msg, e.TraceID)
	default:
		return fmt.Sprintf("%s: %s", code, msg)
	}
}

func (e *ToolError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func newToolError(code, tool, traceID, message string, err error) *ToolError {
	if err == nil {
		switch code {
		case CodeNotFound:
			err = ErrNotFound
		case CodeDuplicate:
			err = ErrDuplicate
		case CodeInvalidTool:
			err = ErrInvalidTool
		case CodeInvalidRisk:
			err = ErrInvalidRisk
		case CodeInvalidInput:
			err = ErrInvalidInput
		case CodeTimeout:
			err = ErrTimeout
		case CodeCanceled:
			err = ErrCanceled
		default:
			err = ErrExecutionFailed
		}
	}
	return &ToolError{
		Code:    code,
		Tool:    tool,
		Message: message,
		TraceID: traceID,
		Err:     err,
	}
}

func failedResult(err *ToolError, extra map[string]any) ToolResult {
	meta := map[string]any{}
	for k, v := range extra {
		meta[k] = v
	}
	if err != nil {
		if err.TraceID != "" {
			meta["trace_id"] = err.TraceID
		}
		if err.Tool != "" {
			meta["tool"] = err.Tool
		}
		if err.Code != "" {
			meta["error_code"] = err.Code
		}
	}
	msg := ""
	if err != nil {
		msg = err.Error()
	}
	return ToolResult{
		Success:  false,
		Error:    msg,
		Metadata: meta,
	}
}
