package tools

import (
	"context"
	"strings"
	"time"
)

// RiskLevel classifies the blast radius of executing a tool.
type RiskLevel string

const (
	RiskLow      RiskLevel = "LOW"
	RiskMedium   RiskLevel = "MEDIUM"
	RiskHigh     RiskLevel = "HIGH"
	RiskCritical RiskLevel = "CRITICAL"
)

// Tool is a dynamically registered capability. Agents depend on the registry,
// never on a concrete implementation.
type Tool interface {
	Name() string
	Description() string
	InputSchema() any
	Execute(ctx context.Context, input map[string]any) (ToolResult, error)
}

// ToolResult is the structured outcome of a real tool execution.
// Data must come from the tool itself — never from the LLM.
type ToolResult struct {
	Success  bool
	Data     any
	Error    string
	Metadata map[string]any
	Duration time.Duration
}

// Info is the public description of a registered tool.
type Info struct {
	Name        string
	Description string
	InputSchema any
	Risk        RiskLevel
}

// ParseRiskLevel accepts LOW / MEDIUM / HIGH / CRITICAL (any case).
func ParseRiskLevel(s string) (RiskLevel, error) {
	switch RiskLevel(strings.ToUpper(strings.TrimSpace(s))) {
	case RiskLow:
		return RiskLow, nil
	case RiskMedium:
		return RiskMedium, nil
	case RiskHigh:
		return RiskHigh, nil
	case RiskCritical:
		return RiskCritical, nil
	default:
		return "", ErrInvalidRisk
	}
}

func validRisk(r RiskLevel) bool {
	_, err := ParseRiskLevel(string(r))
	return err == nil
}

func cloneMetadata(in map[string]any) map[string]any {
	if in == nil {
		return map[string]any{}
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
