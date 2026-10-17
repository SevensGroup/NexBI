# NexBI Phase 5 — Elasticsearch MCP Console

Implement the Elasticsearch MCP Console and /tools.
MCP server: Elasticsearch MCP. Tools: search, aggregate, count, get.
Create MCPServerCard, MCPToolCard, MCPQueryViewer, MCPResultViewer, and MCPHealthBadge.
Show the path explicitly: Agent → Semantic Layer → Query Compiler → MCP → Elasticsearch → Result.
Show Semantic Query, generated DSL, MCP request, Elasticsearch result, latency, and status.
Business semantics take priority over Elasticsearch DSL.

When finished, run npm run build and confirm there are no obvious TypeScript errors, unhandled exceptions, or meaningless TODOs.
