package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"autonomous-agent/internal/logger"
	"autonomous-agent/internal/requestid"
)

const defaultHTTPTimeout = 10 * time.Second

type rpcRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      any    `json:"id,omitempty"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type initializeResult struct {
	ProtocolVersion string `json:"protocolVersion"`
}

type listToolsResult struct {
	Tools []ToolInfo `json:"tools"`
}

type callToolParams struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments,omitempty"`
}

type callToolResult struct {
	Content           []Content `json:"content"`
	StructuredContent any       `json:"structuredContent,omitempty"`
	IsError           bool      `json:"isError,omitempty"`
}

// HTTPClient is a Streamable-HTTP MCP client with timeout, retry, and error normalization.
type HTTPClient struct {
	endpoint string
	http     *http.Client
	timeout  time.Duration
	retry    RetryPolicy
	logger   logger.Logger

	mu      sync.Mutex
	initErr error
	inited  bool
	session string
	nextID  atomic.Int64
}

var _ Client = (*HTTPClient)(nil)

// NewHTTPClient constructs an MCP client for an http(s) MCP endpoint.
func NewHTTPClient(rawURL string, timeout time.Duration, retry RetryPolicy, log logger.Logger) (*HTTPClient, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return nil, fmt.Errorf("%w: url is required", ErrInvalidURL)
	}
	u, err := url.Parse(rawURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, fmt.Errorf("%w: %q", ErrInvalidURL, rawURL)
	}
	if timeout <= 0 {
		timeout = defaultHTTPTimeout
	}
	if log == nil {
		log = logger.New("info", "json", io.Discard)
	}
	return &HTTPClient{
		endpoint: u.String(),
		http:     &http.Client{Timeout: timeout},
		timeout:  timeout,
		retry:    retry,
		logger:   log,
	}, nil
}

// ListTools discovers tools from the MCP server.
func (c *HTTPClient) ListTools(ctx context.Context) ([]ToolInfo, error) {
	if err := c.ensureInit(ctx); err != nil {
		return nil, err
	}
	raw, err := c.callRPC(ctx, methodListTools, map[string]any{})
	if err != nil {
		return nil, err
	}
	var out listToolsResult
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &out); err != nil {
			return nil, fmt.Errorf("%w: decode tools/list: %v", ErrRPC, err)
		}
	}
	return out.Tools, nil
}

// Call executes a named MCP tool. It does not invent a successful payload.
func (c *HTTPClient) Call(ctx context.Context, name string, arguments map[string]any) (CallResult, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return CallResult{}, fmt.Errorf("%w: tool name is required", ErrRPC)
	}
	if err := c.ensureInit(ctx); err != nil {
		return CallResult{}, err
	}
	if arguments == nil {
		arguments = map[string]any{}
	}
	raw, err := c.callRPC(ctx, methodCallTool, callToolParams{Name: name, Arguments: arguments})
	if err != nil {
		return CallResult{}, err
	}
	var out callToolResult
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &out); err != nil {
			return CallResult{}, fmt.Errorf("%w: decode tools/call: %v", ErrRPC, err)
		}
	}
	res := CallResult{
		Content:           out.Content,
		StructuredContent: out.StructuredContent,
		IsError:           out.IsError,
		Raw:               raw,
	}
	if out.IsError {
		return res, fmt.Errorf("%w: %s", ErrToolFailed, contentText(out.Content))
	}
	return res, nil
}

func (c *HTTPClient) ensureInit(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.inited {
		return c.initErr
	}
	params := map[string]any{
		"protocolVersion": protocolVersion,
		"capabilities":    map[string]any{},
		"clientInfo": map[string]any{
			"name":    "autonomous-agent",
			"version": "phase-5",
		},
	}
	raw, err := c.callRPCLocked(ctx, methodInitialize, params, true)
	if err != nil {
		// Some MCP endpoints only expose tools/list + tools/call.
		c.logger.WarnContext(ctx, "mcp initialize failed; continuing without session",
			"trace_id", requestid.From(ctx),
			"error", err,
		)
		c.inited = true
		c.initErr = nil
		return nil
	}
	var init initializeResult
	_ = json.Unmarshal(raw, &init)
	if nerr := c.postNotificationLocked(ctx, methodInitialized, map[string]any{}); nerr != nil {
		c.logger.WarnContext(ctx, "mcp initialized notification failed",
			"trace_id", requestid.From(ctx),
			"error", nerr,
		)
	}
	c.inited = true
	return nil
}

func (c *HTTPClient) callRPC(ctx context.Context, method string, params any) (json.RawMessage, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.callRPCLocked(ctx, method, params, false)
}

func (c *HTTPClient) callRPCLocked(ctx context.Context, method string, params any, initializing bool) (json.RawMessage, error) {
	id := c.nextID.Add(1)
	req := rpcRequest{
		JSONRPC: jsonRPCVersion,
		ID:      id,
		Method:  method,
		Params:  params,
	}
	var result json.RawMessage
	err := withRetry(ctx, c.retry, func(attempt int) error {
		resp, session, err := c.post(ctx, req, initializing)
		if err != nil {
			return err
		}
		if session != "" {
			c.session = session
		}
		if resp.Error != nil {
			return resp.Error
		}
		result = resp.Result
		return nil
	})
	return result, err
}

func (c *HTTPClient) postNotificationLocked(ctx context.Context, method string, params any) error {
	req := rpcRequest{
		JSONRPC: jsonRPCVersion,
		Method:  method,
		Params:  params,
	}
	return withRetry(ctx, c.retry, func(int) error {
		_, session, err := c.post(ctx, req, false)
		if session != "" {
			c.session = session
		}
		return err
	})
}

func (c *HTTPClient) post(ctx context.Context, payload rpcRequest, initializing bool) (*rpcResponse, string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, "", fmt.Errorf("%w: marshal: %v", ErrRPC, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, "", fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("MCP-Protocol-Version", protocolVersion)
	if !initializing && c.session != "" {
		req.Header.Set("Mcp-Session-Id", c.session)
	}

	httpResp, err := c.http.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	defer httpResp.Body.Close()

	session := strings.TrimSpace(httpResp.Header.Get("Mcp-Session-Id"))

	if payload.ID == nil && (httpResp.StatusCode == http.StatusAccepted || httpResp.StatusCode == http.StatusOK || httpResp.StatusCode == http.StatusNoContent) {
		_, _ = io.Copy(io.Discard, httpResp.Body)
		return &rpcResponse{}, session, nil
	}

	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		snippet, _ := io.ReadAll(io.LimitReader(httpResp.Body, 2048))
		return nil, session, &httpStatusError{Status: httpResp.StatusCode, Body: string(snippet)}
	}

	decoded, err := decodeHTTPBody(httpResp, payload.ID)
	if err != nil {
		return nil, session, err
	}
	return decoded, session, nil
}

func decodeHTTPBody(resp *http.Response, id any) (*rpcResponse, error) {
	ct := strings.ToLower(resp.Header.Get("Content-Type"))
	if strings.Contains(ct, "text/event-stream") {
		return decodeSSE(resp.Body, id)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		return nil, fmt.Errorf("%w: read body: %v", ErrUnavailable, err)
	}
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 {
		return &rpcResponse{ID: id}, nil
	}
	var out rpcResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("%w: decode json: %v", ErrRPC, err)
	}
	return &out, nil
}

func decodeSSE(r io.Reader, id any) (*rpcResponse, error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 4<<20)
	var block strings.Builder
	flush := func() (*rpcResponse, bool) {
		data := strings.TrimSpace(block.String())
		block.Reset()
		if data == "" {
			return nil, false
		}
		var out rpcResponse
		if err := json.Unmarshal([]byte(data), &out); err != nil {
			return nil, false
		}
		if id != nil && out.ID != nil && fmt.Sprint(out.ID) != fmt.Sprint(id) {
			return nil, false
		}
		return &out, true
	}
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			if out, ok := flush(); ok {
				return out, nil
			}
			continue
		}
		if strings.HasPrefix(line, "data:") {
			if block.Len() > 0 {
				block.WriteByte('\n')
			}
			block.WriteString(strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		}
	}
	if out, ok := flush(); ok {
		return out, nil
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("%w: sse: %v", ErrUnavailable, err)
	}
	return nil, fmt.Errorf("%w: empty sse response", ErrRPC)
}

func contentText(blocks []Content) string {
	parts := make([]string, 0, len(blocks))
	for _, b := range blocks {
		if s := strings.TrimSpace(b.Text); s != "" {
			parts = append(parts, s)
		}
	}
	return strings.Join(parts, "\n")
}

// DataFromCall extracts structured data from an MCP tool result without fabricating it.
func DataFromCall(res CallResult) any {
	if res.StructuredContent != nil {
		return res.StructuredContent
	}
	text := strings.TrimSpace(contentText(res.Content))
	if text == "" {
		if len(res.Raw) == 0 {
			return nil
		}
		var generic any
		if err := json.Unmarshal(res.Raw, &generic); err == nil {
			return generic
		}
		return nil
	}
	var parsed any
	if err := json.Unmarshal([]byte(text), &parsed); err == nil {
		return parsed
	}
	return text
}
