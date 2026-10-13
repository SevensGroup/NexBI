package mcp

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"time"
)

// MockServer is an in-process Streamable-HTTP MCP server for tests.
type MockServer struct {
	URL    string
	server *httptest.Server

	mu          sync.Mutex
	Tools       []ToolInfo
	Handler     func(name string, arguments map[string]any) (CallResult, error)
	FailTimes   int
	Delay       time.Duration
	Calls       map[string]int
	LastArgs    map[string]any
	LastTool    string
	SSE         bool
	RejectWrite bool
}

// ServeHTTP implements http.Handler for Streamable-HTTP MCP.
func (m *MockServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	m.serve(w, r)
}

// NewMockServer constructs an MCP mock that is not yet listening.
func NewMockServer() *MockServer {
	return &MockServer{
		Tools: []ToolInfo{
			{Name: "search", Description: "Search documents", InputSchema: map[string]any{"type": "object"}},
			{Name: "get", Description: "Get a document", InputSchema: map[string]any{"type": "object"}},
			{Name: "count", Description: "Count documents", InputSchema: map[string]any{"type": "object"}},
			{Name: "aggregate", Description: "Aggregate documents", InputSchema: map[string]any{"type": "object"}},
			{Name: "delete", Description: "Delete a document", InputSchema: map[string]any{"type": "object"}},
			{Name: "update", Description: "Update a document", InputSchema: map[string]any{"type": "object"}},
			{Name: "index", Description: "Index a document", InputSchema: map[string]any{"type": "object"}},
			{Name: "bulk", Description: "Bulk write", InputSchema: map[string]any{"type": "object"}},
		},
		Calls:       map[string]int{},
		RejectWrite: true,
	}
}

// StartMockServer starts a JSON-RPC MCP test server.
func StartMockServer() *MockServer {
	m := NewMockServer()
	m.server = httptest.NewServer(m)
	m.URL = m.server.URL
	return m
}

// CallCount returns how many times a tool was invoked.
func (m *MockServer) CallCount(name string) int {
	if m == nil {
		return 0
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.Calls[normalizeCallName(name)]
}

func normalizeCallName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

// Close shuts down the mock server.
func (m *MockServer) Close() {
	if m != nil && m.server != nil {
		m.server.Close()
	}
}

func (m *MockServer) serve(w http.ResponseWriter, r *http.Request) {
	if m.Delay > 0 {
		time.Sleep(m.Delay)
	}

	m.mu.Lock()
	if m.FailTimes > 0 {
		m.FailTimes--
		m.mu.Unlock()
		http.Error(w, "temporarily unavailable", http.StatusServiceUnavailable)
		return
	}
	m.mu.Unlock()

	raw, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	var req rpcRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if req.Method == methodInitialized || req.ID == nil {
		w.WriteHeader(http.StatusAccepted)
		return
	}

	var result any
	var rpcErr *rpcError
	switch req.Method {
	case methodInitialize:
		w.Header().Set("Mcp-Session-Id", "mock-session")
		result = map[string]any{
			"protocolVersion": protocolVersion,
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo":      map[string]any{"name": "mock-elasticsearch-mcp", "version": "test"},
		}
	case methodListTools:
		m.mu.Lock()
		result = listToolsResult{Tools: append([]ToolInfo(nil), m.Tools...)}
		m.mu.Unlock()
	case methodCallTool:
		result, rpcErr = m.handleCall(req.Params)
	default:
		rpcErr = &rpcError{Code: -32601, Message: "method not found"}
	}

	resp := rpcResponse{JSONRPC: jsonRPCVersion, ID: req.ID}
	if rpcErr != nil {
		resp.Error = rpcErr
	} else {
		encoded, _ := json.Marshal(result)
		resp.Result = encoded
	}

	payload, _ := json.Marshal(resp)
	if m.SSE {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("event: message\ndata: " + string(payload) + "\n\n"))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(payload)
}

func (m *MockServer) handleCall(params any) (any, *rpcError) {
	raw, _ := json.Marshal(params)
	var call callToolParams
	if err := json.Unmarshal(raw, &call); err != nil {
		return nil, &rpcError{Code: -32602, Message: "invalid params"}
	}
	name := strings.ToLower(strings.TrimSpace(call.Name))
	args := call.Arguments
	if args == nil {
		args = map[string]any{}
	}

	m.mu.Lock()
	m.Calls[name]++
	m.LastTool = name
	m.LastArgs = args
	handler := m.Handler
	rejectWrite := m.RejectWrite
	m.mu.Unlock()

	if rejectWrite && isWriteName(name) {
		return callToolResult{
			Content: []Content{{Type: "text", Text: "write operation is disabled"}},
			IsError: true,
		}, nil
	}

	if handler != nil {
		out, err := handler(name, args)
		if err != nil {
			return nil, &rpcError{Code: -32000, Message: err.Error()}
		}
		return callToolResult{
			Content:           out.Content,
			StructuredContent: out.StructuredContent,
			IsError:           out.IsError,
		}, nil
	}

	return defaultMockResult(name, args), nil
}

func isWriteName(name string) bool {
	n := strings.ToLower(strings.TrimSpace(name))
	switch {
	case n == "delete", n == "update", n == "index", n == "bulk", n == "bulk_write":
		return true
	case strings.Contains(n, "delete"), strings.Contains(n, "update"), strings.Contains(n, "bulk"):
		return true
	case strings.HasPrefix(n, "index") && n != "indices":
		return n == "index" || strings.Contains(n, "index_document")
	default:
		return false
	}
}

func defaultMockResult(name string, args map[string]any) callToolResult {
	var payload any
	switch name {
	case "search":
		payload = map[string]any{
			"hits": map[string]any{
				"total": map[string]any{"value": 1, "relation": "eq"},
				"hits": []any{
					map[string]any{"_id": "1", "_source": map[string]any{"message": "hello from mock es"}},
				},
			},
		}
	case "get", "get_document":
		id := strings.TrimSpace(fmtString(args["id"]))
		payload = map[string]any{"_id": id, "found": true, "_source": map[string]any{"message": "hello from mock es"}}
	case "count":
		payload = map[string]any{"count": 1}
	case "aggregate":
		payload = map[string]any{
			"aggregations": map[string]any{
				"by_status": map[string]any{
					"buckets": []any{map[string]any{"key": "ok", "doc_count": 1}},
				},
			},
		}
	default:
		payload = map[string]any{"ok": true, "tool": name}
	}
	raw, _ := json.Marshal(payload)
	return callToolResult{
		Content:           []Content{{Type: "text", Text: string(raw)}},
		StructuredContent: payload,
	}
}

func fmtString(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	b, _ := json.Marshal(v)
	return string(b)
}
