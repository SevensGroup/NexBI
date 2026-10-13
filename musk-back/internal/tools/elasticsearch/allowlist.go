package elasticsearch

import (
	"strings"
)

const (
	MCPSearch    = "search"
	MCPGet       = "get"
	MCPCount     = "count"
	MCPAggregate = "aggregate"

	NameSearch    = "elasticsearch_search"
	NameGet       = "elasticsearch_get"
	NameCount     = "elasticsearch_count"
	NameAggregate = "elasticsearch_aggregate"
)

var writeTools = []string{
	"delete",
	"update",
	"index",
	"bulk",
	"bulk_write",
	"bulkwrite",
	"create",
	"put",
	"reindex",
	"delete_by_query",
	"update_by_query",
	"index_document",
	"write",
}

var canonicalReads = map[string]string{
	"search":       MCPSearch,
	"get":          MCPGet,
	"get_document": MCPGet,
	"count":        MCPCount,
	"aggregate":    MCPAggregate,
	NameSearch:     MCPSearch,
	NameGet:        MCPGet,
	NameCount:      MCPCount,
	NameAggregate:  MCPAggregate,
}

var mcpAliases = map[string][]string{
	MCPSearch:    {MCPSearch},
	MCPGet:       {MCPGet, "get_document"},
	MCPCount:     {MCPCount},
	MCPAggregate: {MCPAggregate},
}

func normalizeToolName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func isWriteTool(name string) bool {
	n := normalizeToolName(name)
	n = strings.TrimPrefix(n, "elasticsearch_")
	n = strings.ReplaceAll(n, "-", "_")
	for _, w := range writeTools {
		if n == w || strings.Contains(n, w) {
			// "index" must not match "indices"
			if w == "index" && (n == "indices" || strings.Contains(n, "indices")) {
				continue
			}
			if w == "index" && n != "index" && n != "index_document" && !strings.HasPrefix(n, "index_") {
				continue
			}
			return true
		}
	}
	return false
}

func canonicalRead(name string) (string, bool) {
	n := normalizeToolName(name)
	c, ok := canonicalReads[n]
	return c, ok
}

func allowedMCP(name string, allowed, denied []string, readOnly bool) bool {
	n := normalizeToolName(name)
	if isWriteTool(n) {
		return false
	}
	for _, d := range denied {
		if normalizeToolName(d) == n {
			return false
		}
	}
	if len(allowed) == 0 {
		_, ok := canonicalRead(n)
		return ok
	}
	for _, a := range allowed {
		want := normalizeToolName(a)
		if want == n {
			return true
		}
		if canon, ok := canonicalRead(want); ok {
			if canon == n {
				return true
			}
			for _, alias := range mcpAliases[canon] {
				if alias == n {
					return true
				}
			}
		}
	}
	if readOnly {
		_, ok := canonicalRead(n)
		return ok && containsNormalized(allowed, n)
	}
	return false
}

func containsNormalized(list []string, name string) bool {
	n := normalizeToolName(name)
	for _, item := range list {
		if normalizeToolName(item) == n {
			return true
		}
		if canon, ok := canonicalRead(item); ok && canon == n {
			return true
		}
	}
	return false
}

func resolveMCPName(canonical string, discovered []string) string {
	have := map[string]struct{}{}
	for _, n := range discovered {
		have[normalizeToolName(n)] = struct{}{}
	}
	for _, alias := range mcpAliases[canonical] {
		if _, ok := have[alias]; ok {
			return alias
		}
	}
	return canonical
}

func registryName(canonical string) string {
	switch canonical {
	case MCPSearch:
		return NameSearch
	case MCPGet:
		return NameGet
	case MCPCount:
		return NameCount
	case MCPAggregate:
		return NameAggregate
	default:
		return "elasticsearch_" + canonical
	}
}
