Continue the existing project.

Integrate Elasticsearch MCP.

Goal:

Let the agent use Elasticsearch through the unified tool layer.

Requirements:

Create:

internal/tools/mcp/
internal/tools/elasticsearch/

Do not let the agent depend on the Elasticsearch SDK directly.

Architecture:

Agent
→ Tool Registry
→ MCP Tool
→ Elasticsearch MCP Server
→ Elasticsearch

Implement an MCP client adapter.

Support:

- tool discovery
- tool execution
- timeout
- retry
- error normalization

Adapt MCP tools to:

the Tool interface.

Support at least:

search
get
count
aggregate

Add configuration:

ELASTICSEARCH_MCP_URL

Add:

configs/tools/elasticsearch.yaml

Define the allowed tools.

Security requirements:

Phase 1 allows READ only.

Forbid:

- delete
- update
- index
- bulk write

Implement:

ElasticsearchSearchTool
ElasticsearchAggregateTool

Requirements:

1. Queries must be validated.
2. Cap the maximum size.
3. Cap the maximum aggregation buckets.
4. Enforce a timeout.
5. Prevent query injection.
6. Record a query hash.
7. Do not print sensitive data in logs.

Tests:

Use a mock MCP server.

Test:

Agent
→ Decision
→ Elasticsearch Tool
→ MCP
→ Mock ES
→ Observation
→ Reflection

Do not implement the Semantic Layer.
