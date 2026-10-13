package mcp

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"

	"autonomous-agent/internal/tools"
)

var (
	// ErrUnavailable is returned when the MCP server cannot be reached.
	ErrUnavailable = errors.New("mcp server unavailable")
	// ErrRPC is returned for JSON-RPC protocol errors.
	ErrRPC = errors.New("mcp rpc error")
	// ErrToolFailed is returned when the MCP tool reports isError.
	ErrToolFailed = errors.New("mcp tool failed")
	// ErrInvalidURL is returned when the MCP endpoint URL is empty or not http(s).
	ErrInvalidURL = errors.New("invalid mcp url")
)

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

func (e *rpcError) Error() string {
	if e == nil {
		return ""
	}
	msg := strings.TrimSpace(e.Message)
	if msg == "" {
		msg = "rpc error"
	}
	return fmt.Sprintf("%s: code=%d %s", ErrRPC, e.Code, msg)
}

func (e *rpcError) Unwrap() error { return ErrRPC }

type httpStatusError struct {
	Status int
	Body   string
}

func (e *httpStatusError) Error() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("%s: http %d", ErrUnavailable, e.Status)
}

func (e *httpStatusError) Unwrap() error { return ErrUnavailable }

func retryable(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	var httpErr *httpStatusError
	if errors.As(err, &httpErr) {
		if httpErr.Status == 429 {
			return true
		}
		return httpErr.Status >= 500
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}
	return errors.Is(err, ErrUnavailable)
}

// Normalize maps MCP/transport errors onto structured ToolError values.
func Normalize(tool, traceID string, err error) *tools.ToolError {
	if err == nil {
		return nil
	}
	var te *tools.ToolError
	if errors.As(err, &te) {
		if te.Tool == "" {
			te.Tool = tool
		}
		if te.TraceID == "" {
			te.TraceID = traceID
		}
		return te
	}

	code := tools.CodeExecutionFailed
	sentinel := tools.ErrExecutionFailed
	msg := err.Error()
	switch {
	case errors.Is(err, context.DeadlineExceeded), errors.Is(err, tools.ErrTimeout):
		code = tools.CodeTimeout
		sentinel = tools.ErrTimeout
	case errors.Is(err, context.Canceled), errors.Is(err, tools.ErrCanceled):
		code = tools.CodeCanceled
		sentinel = tools.ErrCanceled
	case errors.Is(err, ErrUnavailable):
		code = tools.CodeExecutionFailed
		sentinel = ErrUnavailable
	case errors.Is(err, ErrRPC), errors.Is(err, ErrToolFailed):
		code = tools.CodeExecutionFailed
		sentinel = err
	case errors.Is(err, tools.ErrInvalidInput):
		code = tools.CodeInvalidInput
		sentinel = tools.ErrInvalidInput
	}
	return &tools.ToolError{
		Code:    code,
		Tool:    tool,
		Message: msg,
		TraceID: traceID,
		Err:     sentinel,
	}
}
