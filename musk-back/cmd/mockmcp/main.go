package main

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"autonomous-agent/internal/tools/mcp"
)

func main() {
	addr := strings.TrimSpace(os.Getenv("MOCK_MCP_ADDR"))
	if addr == "" {
		addr = ":8081"
	}
	esURL := strings.TrimSpace(os.Getenv("ELASTICSEARCH_URL"))

	mock := mcp.NewMockServer()
	if esURL != "" {
		mock.Handler = esProxy(esURL)
		log.Printf("mock-mcp-server proxying Elasticsearch at %s", esURL)
	}

	mux := http.NewServeMux()
	mux.Handle("/mcp", mock)
	mux.Handle("/", mock)
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	log.Printf("mock-mcp-server listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}

func esProxy(base string) func(name string, arguments map[string]any) (mcp.CallResult, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	base = strings.TrimRight(base, "/")
	return func(name string, arguments map[string]any) (mcp.CallResult, error) {
		index, _ := arguments["index"].(string)
		if strings.TrimSpace(index) == "" {
			index = "_all"
		}
		var (
			method string
			path   string
			body   any
		)
		switch strings.ToLower(strings.TrimSpace(name)) {
		case "search":
			method, path = http.MethodPost, "/"+index+"/_search"
			body = arguments["query"]
			if body == nil {
				body = arguments
			}
		case "aggregate":
			method, path = http.MethodPost, "/"+index+"/_search"
			body = map[string]any{
				"size":         arguments["size"],
				"query":        arguments["query"],
				"aggregations": firstNonNil(arguments["aggregations"], arguments["aggs"]),
			}
		case "count":
			method, path = http.MethodPost, "/"+index+"/_count"
			body = arguments["query"]
			if body == nil {
				body = arguments
			}
		case "get", "get_document":
			id, _ := arguments["id"].(string)
			method, path = http.MethodGet, "/"+index+"/_doc/"+id
		default:
			return mcp.CallResult{IsError: true, Content: []mcp.Content{{Type: "text", Text: "unsupported tool"}}}, nil
		}

		var reader io.Reader
		if body != nil && method != http.MethodGet {
			raw, err := json.Marshal(body)
			if err != nil {
				return mcp.CallResult{}, err
			}
			reader = bytes.NewReader(raw)
		}
		req, err := http.NewRequest(method, base+path, reader)
		if err != nil {
			return mcp.CallResult{}, err
		}
		if reader != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		resp, err := client.Do(req)
		if err != nil {
			return mcp.CallResult{}, err
		}
		defer resp.Body.Close()
		payload, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		var parsed any
		_ = json.Unmarshal(payload, &parsed)
		return mcp.CallResult{
			Content:           []mcp.Content{{Type: "text", Text: string(payload)}},
			StructuredContent: parsed,
			IsError:           resp.StatusCode >= 400,
		}, nil
	}
}

func firstNonNil(vals ...any) any {
	for _, v := range vals {
		if v != nil {
			return v
		}
	}
	return nil
}
