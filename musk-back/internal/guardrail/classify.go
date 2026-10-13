package guardrail

import (
	"fmt"
	"strconv"
	"strings"

	"autonomous-agent/internal/tools"
)

const largeQuerySize = 1000

var criticalTokens = []string{
	"delete", "drop", "purge", "destroy", "remove",
	"fund", "payment", "transfer", "withdraw", "irreversible",
}

var highTokens = []string{
	"update", "modify", "mutate", "write", "patch", "publish",
	"config", "webhook", "external",
}

var lowTokens = []string{
	"search", "query", "get", "count", "calculate", "calculator",
	"lookup", "read", "list", "aggregate", "recall",
}

var mediumTokens = []string{
	"scroll", "bulk_search", "export", "long", "batch",
}

// Classify returns the effective risk: max(registered tool risk, inferred from name/args).
//
// Rules:
//   - LOW: query data, calculate, search
//   - MEDIUM: large queries, long-running tasks
//   - HIGH: modify business data, modify advertising config, call external APIs
//   - CRITICAL: delete, funds, irreversible production operations
func Classify(action Action) RiskLevel {
	inferred := inferRisk(action.Tool, action.Arguments)
	registered, err := tools.ParseRiskLevel(string(action.RegisteredRisk))
	if err != nil {
		registered = ""
	}
	return maxRisk(registered, inferred)
}

func inferRisk(tool string, args map[string]any) RiskLevel {
	blob := strings.ToLower(strings.TrimSpace(tool) + " " + flattenArgs(args))
	if blob == "" {
		return RiskLow
	}

	level := RiskLow
	if containsAny(blob, lowTokens) {
		level = RiskLow
	}
	if containsAny(blob, mediumTokens) || largeQuery(args) {
		level = maxRisk(level, RiskMedium)
	}
	if containsAny(blob, highTokens) || hasExternalURL(blob) {
		level = maxRisk(level, RiskHigh)
	}
	if containsAny(blob, criticalTokens) {
		level = maxRisk(level, RiskCritical)
	}
	return level
}

func largeQuery(args map[string]any) bool {
	if args == nil {
		return false
	}
	for _, key := range []string{"size", "limit", "count", "n"} {
		if n, ok := asInt(args[key]); ok && n >= largeQuerySize {
			return true
		}
	}
	return false
}

func asInt(v any) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case int64:
		return int(n), true
	case float64:
		return int(n), true
	case string:
		parsed, err := strconv.Atoi(strings.TrimSpace(n))
		return parsed, err == nil
	default:
		return 0, false
	}
}

func hasExternalURL(blob string) bool {
	return strings.Contains(blob, "http://") || strings.Contains(blob, "https://")
}

func containsAny(blob string, tokens []string) bool {
	for _, tok := range tokens {
		if tok == "" {
			continue
		}
		if strings.Contains(blob, tok) {
			return true
		}
	}
	return false
}

func flattenArgs(args map[string]any) string {
	if args == nil {
		return ""
	}
	parts := make([]string, 0, len(args)*2)
	for k, v := range args {
		parts = append(parts, strings.ToLower(k), strings.ToLower(fmt.Sprint(v)))
	}
	return strings.Join(parts, " ")
}

func maxRisk(a, b RiskLevel) RiskLevel {
	if rank(a) >= rank(b) {
		if a == "" {
			return b
		}
		return a
	}
	if b == "" {
		return a
	}
	return b
}

func rank(r RiskLevel) int {
	switch r {
	case RiskLow:
		return 1
	case RiskMedium:
		return 2
	case RiskHigh:
		return 3
	case RiskCritical:
		return 4
	default:
		return 0
	}
}
