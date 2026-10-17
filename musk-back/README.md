# NexBI

Production Agent Platform (Phase 12) for a Go AI Agent built on [CloudWeGo Eino](https://github.com/cloudwego/eino).

The product is an **Advertising Autonomous Analyst**. A goal such as `Why did Meta ad ROI decline yesterday` is understood (`channel=Meta`, `metric=ROAS`, `time=yesterday`), investigated through the Semantic Layer until `EvidenceSufficient`, and returned as a structured report. The process is now a production-grade platform: OpenTelemetry traces and metrics, persisted Agent Runs, retry / circuit breaker / rate limiter / concurrency limiter, LLM fallback, tool timeouts, and an explicit Agent budget.

```
Eino
+ Autonomous Decision Engine
+ Planner
+ Tool Registry
+ Elasticsearch MCP
+ Semantic Layer
+ Memory
+ Multi-Agent
+ Reflection
+ Guardrail
+ Observability
```

POST `/api/v1/agent/run` is the product surface:

```json
{
  "run_id": "...",
  "status": "COMPLETED",
  "answer": "...",
  "confidence": 0.93,
  "evidence": [],
  "steps": 12
}
```

## Architecture

```
Goal → Plan → Task Graph → Understand (recall memory)
     → Decide → Guardrail → Act (CALL_TOOL or DELEGATE_AGENT)
     → Observe → Reflection → Decide / Finish
```

HIGH and CRITICAL actions interrupt (`WAITING_APPROVAL`), wait for Human-in-the-Loop, then `RESUME` from a checkpoint.

The Coordinator plans and delegates. Specialists run the same loop with independent prompts, tool allowlists, memory scopes, and I/O schemas:

```
Coordinator
├── Analyst      # data analysis / metric analysis / anomaly detection
├── Researcher   # external search / industry context / background material
└── Reviewer     # verify conclusions / verify evidence / find logical errors
```

`CALL_TOOL` is executed by the unified Tool Registry. Elasticsearch is reached only through MCP. Advertising analysis goes through the Semantic Layer (`semantic_query` → Compiler → `elasticsearch_aggregate`). Every run is persisted (`CREATED` → `RUNNING` → `COMPLETED` / `FAILED` / `ABORTED` / `WAITING_APPROVAL`) and traced with `run_id`, `step_id`, and `tool_call_id`.

```
.
├── cmd/server/main.go
├── cmd/mockmcp/main.go
├── configs/
│   ├── agent.yaml
│   ├── dev/agent.yaml
│   ├── test/agent.yaml
│   ├── prod/agent.yaml
│   ├── semantic/
│   └── tools/elasticsearch.yaml
├── internal/
│   ├── agent/decision/
│   ├── agent/multi/
│   ├── agent/planner/
│   ├── agent/reflection/
│   ├── agent/runtime/
│   ├── analyst/
│   ├── api/
│   ├── config/
│   ├── guardrail/
│   ├── logger/
│   ├── memory/
│   ├── model/              # OpenAI-compatible + Primary → Secondary → Fail
│   ├── observability/      # OpenTelemetry traces, metrics, cost, IDs
│   ├── resilience/         # retry, circuit breaker, rate limiter, concurrency
│   ├── runstore/           # Agent Run persistence
│   ├── semantic/
│   └── tools/              # Registry + MCP + Elasticsearch
├── Dockerfile
├── docker-compose.yml
├── Makefile
└── README.md
```

## Quick Start

Requirements: Go 1.24+, an OpenAI-compatible Chat Completions endpoint. Optional: PostgreSQL, Elasticsearch MCP, OTLP collector.

```bash
cp .env.example .env
# edit LLM_BASE_URL, LLM_API_KEY, LLM_MODEL

export LLM_BASE_URL=https://generativelanguage.googleapis.com/v1beta/openai
export LLM_API_KEY=sk-your-key
export LLM_MODEL=gemini-3.5-flash
export AGENT_MAX_STEPS=30
export AGENT_MAX_TOOL_CALLS=50
export AGENT_MAX_RUNTIME=10m
export AGENT_MAX_LLM_COST=1.0

make run
```

Or with Docker Compose (agent + postgres + elasticsearch + mock-mcp-server):

```bash
cp .env.example .env
docker compose up --build
```

Default listen address is `:8080`. `SIGINT` / `SIGTERM` trigger graceful shutdown.

```bash
curl -s http://127.0.0.1:8080/health

curl -s -X POST http://127.0.0.1:8080/api/v1/agent/run \
  -H 'Content-Type: application/json' \
  -d '{"message":"Analyze why Meta ad ROI declined yesterday"}'
```

## Live self-test (real LLM)

This **spends Gemini tokens**. It reads `LLM_BASE_URL` / `LLM_API_KEY` / `LLM_MODEL` from `musk-back/.env` at runtime. **Do not commit `.env` or hardcode the key.**

```bash
# start the server (Makefile sources .env)
make run

# another terminal
make selftest
# alias: make test-live
```

`make test` stays the mocked Go unit suite (`go test ./...` does **not** hit Gemini). `make selftest` is `-tags live` plus `/health` and `POST /api/v1/agent/run`.

If the API is down the command exits with: start musk-back with musk-back/.env.

Page-level checks: `cd ../musk-web && npm run selftest` (Playwright, live Vite, no mock). Or from the repo root: `make selftest` (API then pages).

## Configuration

YAML is loaded from `-config` (default `configs/agent.yaml`). `CONFIG_ENV=dev|test|prod` selects `configs/{dev,test,prod}/agent.yaml` when the default path is used.

Environment variables overlay YAML. Do not hardcode secrets.

| Variable | Meaning |
| --- | --- |
| `LLM_BASE_URL` / `LLM_API_KEY` / `LLM_MODEL` | Primary OpenAI-compatible model |
| `LLM_FALLBACK_BASE_URL` / `LLM_FALLBACK_API_KEY` / `LLM_FALLBACK_MODEL` | Secondary model (Primary → Secondary → Fail) |
| `AGENT_MAX_STEPS` / `MAX_STEPS` | Decision-loop cap (prod default 30) |
| `AGENT_MAX_TASKS` | Planner DAG size |
| `AGENT_MAX_TOOL_CALLS` / `MAX_TOOL_CALLS` | Tool-call budget (default 50) |
| `AGENT_MAX_RUNTIME` / `MAX_RUNTIME` | Wall-clock budget (default 10m) |
| `AGENT_MAX_LLM_COST` / `MAX_LLM_COST` | USD spend budget (default 1.0) |
| `TOOL_TIMEOUT` | Per-tool timeout (default 10s) |
| `ELASTICSEARCH_MCP_URL` | Elasticsearch MCP endpoint |
| `SEMANTIC_DIR` | Semantic catalog directory |
| `MEMORY_POSTGRES_URL` | Postgres for memory **and** Agent Run persistence |
| `GUARDRAIL_MEDIUM_AUTO` | Auto-execute MEDIUM actions |
| `GUARDRAIL_APPROVAL_TTL` | Pending approval lifetime (default 15m) |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | OTLP HTTP endpoint (empty = in-process traces/metrics) |
| `OTEL_SERVICE_NAME` | OpenTelemetry service name |
| `RATE_LIMIT_RPS` / `CONCURRENCY_MAX` | HTTP rate limiter and concurrency limiter |
| `CONFIG_ENV` | `dev` / `test` / `prod` |

LLM fallback is optional. When the primary model fails (after retry + circuit breaker), the secondary is tried. If both fail, the run fails.

Agent budget is enforced every loop:

- `MAX_STEPS=30`
- `MAX_TOOL_CALLS=50`
- `MAX_RUNTIME=10m`
- `MAX_LLM_COST=1.0`

Exceeding budget marks the run `ABORTED`.

## Semantic Layer

Advertising analysis uses business concepts, not Elasticsearch fields:

`Agent → Semantic Query → Compiler → ES DSL → Tool Registry → MCP → Elasticsearch`

Catalog YAML lives in `configs/semantic/` (`metrics.yaml`, `dimensions.yaml`, `entities.yaml`, `datasources.yaml`). The Analyst must not generate Query DSL. Raw `elasticsearch_*` tools are hidden from the agent when `semantic_query` is registered.

List the catalog:

```bash
curl -s http://127.0.0.1:8080/api/v1/semantic/metrics
curl -s http://127.0.0.1:8080/api/v1/semantic/dimensions
```

## Tool Development

Tools implement `internal/tools.Tool` (`Name`, `Description`, `InputSchema`, `Execute`) and are dynamically `Register`'d with a risk level (`LOW` / `MEDIUM` / `HIGH` / `CRITICAL`). The agent only calls `Registry.Execute` by name.

`Execute` validates input against the JSON Schema, applies `TOOL_TIMEOUT`, logs `trace_id` / `tool_call_id`, and never fabricates a successful result.

Built-in tools:

- `calculator`
- `search_memory` / `save_memory`
- `semantic_query`
- `analyze_advertising`
- Elasticsearch MCP read tools when `ELASTICSEARCH_MCP_URL` is set

```bash
curl -s http://127.0.0.1:8080/api/v1/tools
```

## MCP Integration

Elasticsearch is reached only through the MCP Client Adapter (`internal/tools/mcp`). The agent never imports an Elasticsearch SDK. Phase 5 remains read-only: `delete` / `update` / `index` / `bulk` are denied. Search and aggregate queries are validated (max `size`, max aggregation buckets, injection rejection). Logs record a `query_hash`, not the query body.

`cmd/mockmcp` is a Streamable-HTTP MCP server for local and Compose use. When `ELASTICSEARCH_URL` is set it proxies `search` / `get` / `count` / `aggregate` to Elasticsearch; otherwise it returns mock read results.

Compose wiring:

- `elasticsearch` — single-node ES
- `mock-mcp-server` — MCP in front of ES (`ELASTICSEARCH_MCP_URL=http://mock-mcp-server:8081/mcp`)

## Agent Development

The runtime loop is `internal/agent/runtime`. Decisions are schema-constrained (`CALL_TOOL`, `DELEGATE_AGENT`, `REPLAN`, `REFLECT`, `ASK_USER`, `FINISH`, `ABORT`). The planner emits a DAG capped by `AGENT_MAX_TASKS`. Reflection judges `CONTINUE` / `REPLAN` / `FINISH` / `ASK_USER`. Three identical decision+tool+args force `REPLAN`.

Each decision gets a `step_id`. Each tool execution gets a `tool_call_id`. Token usage, latency, and estimated USD cost are recorded on the run.

```bash
curl -s http://127.0.0.1:8080/api/v1/agents
```

## Multi-Agent

`DELEGATE_AGENT` is Agent-as-Tool. Nested delegate is rejected. Reviewer `{"approved":false,"reason":"insufficient evidence"}` forces Coordinator `REPLAN`. Specialists have independent system prompts, tool allowlists, and memory scopes (`run_id:agent`).

## Memory

Three layers, Postgres when `MEMORY_POSTGRES_URL` is set, otherwise in-process:

- Short-term: Goal, Plan, Decision, ToolCall, Observation
- Task: Task, Conclusion, Evidence, Failures, SuccessfulActions
- Long-term: UserPreference, BusinessKnowledge, HistoricalInsight, QueryPattern

API keys and credentials are stripped. A memory write failure is logged and never aborts the loop.

## Guardrail

- `LOW` — auto execute
- `MEDIUM` — auto when `GUARDRAIL_MEDIUM_AUTO=true` (default)
- `HIGH` / `CRITICAL` — always approval

HTTP `202 WAITING_APPROVAL`. Approve then resume:

```bash
curl -s -X POST http://127.0.0.1:8080/api/v1/agent/approvals/<approval_id>/approve
curl -s -X POST http://127.0.0.1:8080/api/v1/agent/resume \
  -H 'Content-Type: application/json' \
  -d '{"run_id":"<run_id>"}'
```

## Observability

OpenTelemetry traces and metrics (`internal/observability`):

- Trace spans for `agent.run`, `llm.generate`, HTTP
- Metrics: run latency, LLM tokens, LLM cost USD, tool calls, errors
- Structured JSON logs with `run_id` / `step_id` / `tool_call_id`
- Token usage and cost tracking (configurable USD per 1K input/output tokens)
- Error tracking on spans, metrics, and the persisted run

When `OTEL_EXPORTER_OTLP_ENDPOINT` is empty, the SDK stays in-process (runs still record locally). Point it at an OTLP HTTP collector for production export.

## API

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/health` | Liveness |
| `POST` | `/api/v1/agent/run` | Run the agent |
| `GET` | `/api/v1/runs/:id` | Persisted Agent Run |
| `GET` | `/api/v1/agents` | Coordinator + specialists |
| `GET` | `/api/v1/tools` | Tool Registry |
| `GET` | `/api/v1/semantic/metrics` | Semantic metrics |
| `GET` | `/api/v1/semantic/dimensions` | Semantic dimensions |
| `POST` | `/api/v1/agent/approvals/:id/approve` | HITL approve |
| `POST` | `/api/v1/agent/approvals/:id/reject` | HITL reject |
| `POST` | `/api/v1/agent/resume` | Resume after approval |

`POST /api/v1/agent/run` input:

```json
{"message": "Analyze why Meta ad ROI declined yesterday"}
```

Success:

```json
{
  "run_id": "...",
  "status": "COMPLETED",
  "answer": "...",
  "confidence": 0.93,
  "evidence": [],
  "steps": 12
}
```

`GET /api/v1/runs/:id` returns Goal, Status, Steps, Decisions, Tool Calls, Observations, Reflection, and Final Result, plus token usage, cost, and latency.

Run statuses: `CREATED`, `RUNNING`, `WAITING_APPROVAL`, `COMPLETED`, `FAILED`, `ABORTED`.

Inbound HTTP is rate-limited and concurrency-limited (`RATE_LIMIT_RPS`, `CONCURRENCY_MAX`).

## Testing

```bash
make test
make vet
make build
```

Equivalent:

```bash
go test ./...
go vet ./...
go build ./...
```

Coverage includes decision/planner/tools/MCP/semantic/memory/reflection/multi-agent/guardrail/analyst, plus production-platform tests for retry, circuit breaker, rate limiter, concurrency limiter, LLM fallback, run persistence, observability IDs/cost, and the new HTTP catalog/run APIs.

## Production Deployment

1. Use `CONFIG_ENV=prod` (or `-config configs/prod/agent.yaml`).
2. Set `LLM_*` in the environment. Optionally set `LLM_FALLBACK_*`.
3. Set `MEMORY_POSTGRES_URL` so memory **and** Agent Runs persist.
4. Set `ELASTICSEARCH_MCP_URL` at a real MCP server (Compose uses `mock-mcp-server` in front of Elasticsearch).
5. Set `OTEL_EXPORTER_OTLP_ENDPOINT` at your collector.
6. Tune budget: `AGENT_MAX_STEPS=30`, `AGENT_MAX_TOOL_CALLS=50`, `AGENT_MAX_RUNTIME=10m`, `AGENT_MAX_LLM_COST=1.0`.
7. Tune `RATE_LIMIT_RPS` and `CONCURRENCY_MAX`.
8. Deploy with Docker Compose or build `./cmd/server` into your orchestrator.

```bash
CONFIG_ENV=prod docker compose up --build
```

The Compose stack is:

- `autonomous-agent`
- `postgres`
- `elasticsearch`
- `mock-mcp-server`
