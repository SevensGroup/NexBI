package elasticsearch

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"autonomous-agent/internal/config"
	"autonomous-agent/internal/logger"
	"autonomous-agent/internal/requestid"
	"autonomous-agent/internal/tools"
	"autonomous-agent/internal/tools/mcp"
)

func testESConfig(url string) config.ElasticsearchConfig {
	readOnly := true
	return config.ElasticsearchConfig{
		MCPURL:                url,
		Timeout:               config.Duration(time.Second),
		Retry:                 config.RetryConfig{MaxAttempts: 3, Backoff: config.Duration(time.Millisecond)},
		MaxSize:               20,
		MaxAggregationBuckets: 10,
		AllowedTools:          []string{"search", "get", "count", "aggregate"},
		DeniedTools:           []string{"delete", "update", "index", "bulk"},
		ReadOnly:              &readOnly,
	}
}

func testClient(t *testing.T, url string) mcp.Client {
	t.Helper()
	c, err := mcp.NewHTTPClient(url, time.Second, mcp.RetryPolicy{MaxAttempts: 3, Backoff: time.Millisecond}, logger.New("error", "json", io.Discard))
	if err != nil {
		t.Fatalf("NewHTTPClient: %v", err)
	}
	return c
}

func TestRegisterReadOnlyTools(t *testing.T) {
	srv := mcp.StartMockServer()
	defer srv.Close()

	reg := tools.NewRegistry(logger.New("error", "json", io.Discard), time.Second)
	if err := Register(context.Background(), reg, testClient(t, srv.URL), testESConfig(srv.URL), logger.New("error", "json", io.Discard)); err != nil {
		t.Fatalf("Register: %v", err)
	}

	names := map[string]bool{}
	for _, info := range reg.List() {
		names[info.Name] = true
		if isWriteTool(info.Name) {
			t.Fatalf("write tool registered: %s", info.Name)
		}
	}
	for _, want := range []string{NameSearch, NameGet, NameCount, NameAggregate} {
		if !names[want] {
			t.Fatalf("missing %s, have %v", want, names)
		}
	}
	for _, deny := range []string{"delete", "update", "index", "bulk"} {
		if names[deny] {
			t.Fatalf("denied tool %s was registered", deny)
		}
	}
}

func TestSearchTool(t *testing.T) {
	srv := mcp.StartMockServer()
	defer srv.Close()

	tool := NewSearchTool(testClient(t, srv.URL), MCPSearch, 20, logger.New("error", "json", io.Discard))
	res, err := tool.Execute(requestid.With(context.Background(), "trace-es-search"), map[string]any{
		"index": "logs",
		"query": map[string]any{"match_all": map[string]any{}},
		"size":  5,
	})
	if err != nil || !res.Success {
		t.Fatalf("Execute: res=%+v err=%v", res, err)
	}
	hash, _ := res.Metadata["query_hash"].(string)
	if hash == "" {
		t.Fatal("query_hash is required")
	}
	if _, ok := res.Metadata["query"]; ok {
		t.Fatal("query body must not be stored in metadata")
	}
	raw, _ := json.Marshal(res.Data)
	if !strings.Contains(string(raw), "hits") {
		t.Fatalf("data = %s", raw)
	}
}

func TestSearchRejectsOversizedWindow(t *testing.T) {
	tool := NewSearchTool(nil, MCPSearch, 10, logger.New("error", "json", io.Discard))
	_, err := tool.Execute(context.Background(), map[string]any{
		"index": "logs",
		"query": map[string]any{"match_all": map[string]any{}},
		"size":  50,
	})
	if !errors.Is(err, tools.ErrInvalidInput) {
		t.Fatalf("err = %v", err)
	}
}

func TestSearchRejectsScriptInjection(t *testing.T) {
	tool := NewSearchTool(nil, MCPSearch, 10, logger.New("error", "json", io.Discard))
	_, err := tool.Execute(context.Background(), map[string]any{
		"index": "logs",
		"query": map[string]any{
			"script_score": map[string]any{
				"query":  map[string]any{"match_all": map[string]any{}},
				"script": map[string]any{"source": "payload"},
			},
		},
	})
	if !errors.Is(err, tools.ErrInvalidInput) {
		t.Fatalf("err = %v", err)
	}
}

func TestSearchRejectsLuceneString(t *testing.T) {
	tool := NewSearchTool(nil, MCPSearch, 10, logger.New("error", "json", io.Discard))
	_, err := tool.Execute(context.Background(), map[string]any{
		"index": "logs",
		"query": "status:ok OR *:*",
	})
	if !errors.Is(err, tools.ErrInvalidInput) {
		t.Fatalf("err = %v", err)
	}
}

func TestSearchRejectsBadIndex(t *testing.T) {
	tool := NewSearchTool(nil, MCPSearch, 10, logger.New("error", "json", io.Discard))
	_, err := tool.Execute(context.Background(), map[string]any{
		"index": "../secret",
		"query": map[string]any{"match_all": map[string]any{}},
	})
	if !errors.Is(err, tools.ErrInvalidInput) {
		t.Fatalf("err = %v", err)
	}
}

func TestAggregateRejectsHugeBuckets(t *testing.T) {
	tool := NewAggregateTool(nil, MCPAggregate, 10, 5, logger.New("error", "json", io.Discard))
	_, err := tool.Execute(context.Background(), map[string]any{
		"index": "logs",
		"aggregations": map[string]any{
			"by_status": map[string]any{
				"terms": map[string]any{"field": "status", "size": 500},
			},
		},
	})
	if !errors.Is(err, tools.ErrInvalidInput) {
		t.Fatalf("err = %v", err)
	}
}

func TestAggregateTool(t *testing.T) {
	srv := mcp.StartMockServer()
	defer srv.Close()

	tool := NewAggregateTool(testClient(t, srv.URL), MCPAggregate, 20, 10, logger.New("error", "json", io.Discard))
	res, err := tool.Execute(context.Background(), map[string]any{
		"index": "logs",
		"query": map[string]any{"match_all": map[string]any{}},
		"aggregations": map[string]any{
			"by_status": map[string]any{"terms": map[string]any{"field": "status", "size": 5}},
		},
	})
	if err != nil || !res.Success {
		t.Fatalf("Execute: res=%+v err=%v", res, err)
	}
	if res.Metadata["query_hash"] == "" {
		t.Fatal("query_hash is required")
	}
}

func TestGetAndCount(t *testing.T) {
	srv := mcp.StartMockServer()
	defer srv.Close()
	client := testClient(t, srv.URL)

	got, err := NewGetTool(client, MCPGet, logger.New("error", "json", io.Discard)).Execute(context.Background(), map[string]any{
		"index": "logs",
		"id":    "doc-1",
	})
	if err != nil || !got.Success {
		t.Fatalf("get: %+v %v", got, err)
	}

	count, err := NewCountTool(client, MCPCount, logger.New("error", "json", io.Discard)).Execute(context.Background(), map[string]any{
		"index": "logs",
		"query": map[string]any{"match_all": map[string]any{}},
	})
	if err != nil || !count.Success {
		t.Fatalf("count: %+v %v", count, err)
	}
}

func TestSearchDoesNotLogQuery(t *testing.T) {
	srv := mcp.StartMockServer()
	defer srv.Close()

	var buf bytes.Buffer
	log := logger.New("info", "json", &buf)
	secret := "super-secret-token-xyz"
	tool := NewSearchTool(testClient(t, srv.URL), MCPSearch, 20, log)
	res, err := tool.Execute(context.Background(), map[string]any{
		"index": "logs",
		"query": map[string]any{"term": map[string]any{"token": secret}},
		"size":  1,
	})
	if err != nil || !res.Success {
		t.Fatalf("Execute: %+v %v", res, err)
	}
	logged := buf.String()
	if strings.Contains(logged, secret) {
		t.Fatalf("logs leaked query data: %s", logged)
	}
	if !strings.Contains(logged, "query_hash") {
		t.Fatalf("logs missing query_hash: %s", logged)
	}
}

func TestWriteToolsNeverAllowed(t *testing.T) {
	for _, name := range []string{"delete", "update", "index", "bulk", "elasticsearch_delete"} {
		if !isWriteTool(name) {
			t.Fatalf("%s should be treated as write", name)
		}
		if allowedMCP(name, []string{"delete", "search"}, []string{}, true) {
			t.Fatalf("%s must not be allowlisted", name)
		}
	}
}

func TestQueryHashStable(t *testing.T) {
	q := map[string]any{"match": map[string]any{"field": "x"}}
	a := queryHash(q)
	b := queryHash(q)
	if a == "" || a != b {
		t.Fatalf("hash = %q %q", a, b)
	}
}

func TestRegistryExecuteSearch(t *testing.T) {
	srv := mcp.StartMockServer()
	defer srv.Close()

	reg := tools.NewRegistry(logger.New("error", "json", io.Discard), time.Second)
	if err := Register(context.Background(), reg, testClient(t, srv.URL), testESConfig(srv.URL), logger.New("error", "json", io.Discard)); err != nil {
		t.Fatalf("Register: %v", err)
	}
	res, err := reg.Execute(requestid.With(context.Background(), "trace-reg-es"), NameSearch, map[string]any{
		"index": "logs",
		"query": map[string]any{"match_all": map[string]any{}},
		"size":  5,
	})
	if err != nil || !res.Success {
		t.Fatalf("Execute: %+v %v", res, err)
	}
	if res.Metadata["query_hash"] == nil || res.Metadata["query_hash"] == "" {
		t.Fatal("query_hash is required")
	}
}
