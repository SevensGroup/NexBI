package mcp

import (
	"context"
	"encoding/json"
	"time"
)

const (
	jsonRPCVersion    = "2.0"
	protocolVersion   = "2025-03-26"
	methodInitialize  = "initialize"
	methodInitialized = "notifications/initialized"
	methodListTools   = "tools/list"
	methodCallTool    = "tools/call"
)

// ToolInfo is a discovered MCP tool.
type ToolInfo struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	InputSchema any    `json:"inputSchema,omitempty"`
}

// Content is one MCP content block from a tool result.
type Content struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// CallResult is a normalized tools/call outcome. Data is only what the MCP server returned.
type CallResult struct {
	Content           []Content
	StructuredContent any
	IsError           bool
	Raw               json.RawMessage
}

// Client is the MCP Client Adapter: discovery + execution.
// Agent tools depend on this interface, never on an Elasticsearch SDK.
type Client interface {
	ListTools(ctx context.Context) ([]ToolInfo, error)
	Call(ctx context.Context, name string, arguments map[string]any) (CallResult, error)
}

// RetryPolicy controls MCP request retries.
type RetryPolicy struct {
	MaxAttempts int
	Backoff     time.Duration
}

func (p RetryPolicy) attempts() int {
	if p.MaxAttempts < 1 {
		return 1
	}
	return p.MaxAttempts
}

func (p RetryPolicy) backoff() time.Duration {
	if p.Backoff <= 0 {
		return 200 * time.Millisecond
	}
	return p.Backoff
}
