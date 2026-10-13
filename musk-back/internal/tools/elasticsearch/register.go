package elasticsearch

import (
	"context"
	"fmt"
	"strings"
	"time"

	"autonomous-agent/internal/config"
	"autonomous-agent/internal/logger"
	"autonomous-agent/internal/tools"
	"autonomous-agent/internal/tools/mcp"
)

const defaultRisk = tools.RiskMedium

// RegisterFromConfig discovers MCP tools and registers allowed read-only Elasticsearch tools.
// Empty MCP URL is a no-op so Phases 1–4 keep working without Elasticsearch.
func RegisterFromConfig(ctx context.Context, reg tools.Registry, cfg config.ElasticsearchConfig, log logger.Logger) error {
	if !cfg.Enabled() {
		return nil
	}
	if log == nil {
		return fmt.Errorf("logger is required")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	timeout := cfg.Timeout.Duration()
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	client, err := mcp.NewHTTPClient(cfg.MCPURL, timeout, mcp.RetryPolicy{
		MaxAttempts: cfg.Retry.MaxAttempts,
		Backoff:     cfg.Retry.Backoff.Duration(),
	}, log)
	if err != nil {
		return err
	}
	return Register(ctx, reg, client, cfg, log)
}

// Register converts allowed MCP tools into Tool implementations and adds them to the registry.
func Register(ctx context.Context, reg tools.Registry, client mcp.Client, cfg config.ElasticsearchConfig, log logger.Logger) error {
	if reg == nil {
		return fmt.Errorf("tool registry is required")
	}
	if client == nil {
		return fmt.Errorf("mcp client is required")
	}
	if log == nil {
		return fmt.Errorf("logger is required")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	allowed := cfg.AllowedTools
	if len(allowed) == 0 {
		allowed = []string{MCPSearch, MCPGet, MCPCount, MCPAggregate}
	}
	denied := cfg.DeniedTools
	if len(denied) == 0 {
		denied = []string{"delete", "update", "index", "bulk"}
	}
	readOnly := true
	if cfg.ReadOnly != nil {
		readOnly = *cfg.ReadOnly
	}

	var discovered []mcp.ToolInfo
	if listed, err := client.ListTools(ctx); err != nil {
		log.WarnContext(ctx, "elasticsearch mcp discovery failed; registering allowlisted read tools",
			"error", err,
		)
	} else {
		discovered = listed
		for _, info := range discovered {
			name := normalizeToolName(info.Name)
			if isWriteTool(name) || !allowedMCP(name, allowed, denied, readOnly) {
				log.InfoContext(ctx, "elasticsearch mcp tool skipped",
					"mcp_tool", name,
					"reason", "not an allowed read tool",
				)
			}
		}
	}

	discoveredNames := make([]string, 0, len(discovered))
	for _, info := range discovered {
		discoveredNames = append(discoveredNames, normalizeToolName(info.Name))
	}

	maxSize := cfg.MaxSize
	if maxSize < 1 {
		maxSize = 100
	}
	maxBuckets := cfg.MaxAggregationBuckets
	if maxBuckets < 1 {
		maxBuckets = 100
	}

	type spec struct {
		canonical string
		build     func(mcpName string) tools.Tool
	}
	wanted := []spec{
		{MCPSearch, func(mcpName string) tools.Tool {
			return NewSearchTool(client, mcpName, maxSize, log)
		}},
		{MCPGet, func(mcpName string) tools.Tool {
			return NewGetTool(client, mcpName, log)
		}},
		{MCPCount, func(mcpName string) tools.Tool {
			return NewCountTool(client, mcpName, log)
		}},
		{MCPAggregate, func(mcpName string) tools.Tool {
			return NewAggregateTool(client, mcpName, maxSize, maxBuckets, log)
		}},
	}

	registered := map[string]struct{}{}
	for _, item := range wanted {
		if !containsNormalized(allowed, item.canonical) && !containsNormalized(allowed, registryName(item.canonical)) {
			continue
		}
		if isWriteTool(item.canonical) {
			continue
		}
		mcpName := resolveMCPName(item.canonical, discoveredNames)
		if err := reg.Register(item.build(mcpName), defaultRisk); err != nil {
			return fmt.Errorf("register %s: %w", registryName(item.canonical), err)
		}
		registered[item.canonical] = struct{}{}
		registered[mcpName] = struct{}{}
	}

	// Extra discovered read tools that are explicitly allowlisted become MCP adapters.
	for _, info := range discovered {
		name := normalizeToolName(info.Name)
		if _, ok := registered[name]; ok {
			continue
		}
		if canon, ok := canonicalRead(name); ok {
			if _, exists := registered[canon]; exists {
				continue
			}
		}
		if !allowedMCP(name, allowed, denied, readOnly) {
			continue
		}
		if isWriteTool(name) {
			continue
		}
		adapter := mcp.NewAdapter(client, info)
		if err := reg.Register(adapter, defaultRisk); err != nil {
			return fmt.Errorf("register mcp tool %s: %w", name, err)
		}
	}

	log.InfoContext(ctx, "elasticsearch mcp tools registered",
		"mcp_url_set", strings.TrimSpace(cfg.MCPURL) != "",
		"discovered", len(discovered),
	)
	return nil
}
