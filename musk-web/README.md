# NexBI

Vue 3 operating console for the NexBI advertising autonomous analyst. This is not a chat UI. The agent lifecycle is:

`Goal Understanding → Planning → Decision → Tool → Observation → Reflection → Re-Decision → Multi-Agent → Evidence → Final Report`

The UI shows **Reasoning Summary / Decision / Evidence / Confidence / Next Action / Observation / Reflection Summary** only. Full chain-of-thought is never rendered.

## Stack

Vue 3 · TypeScript · Vite · Vue Router · Pinia · VueUse · Tailwind CSS · Element Plus · ECharts · Vue Flow · Axios · SSE

## Pages

| Route | Purpose |
| --- | --- |
| `/dashboard` | Active agents, completed today, success rate, runtime, LLM/tokens/cost |
| `/agents` | Multi-agent graph (Coordinator → Analyst / Researcher / Reviewer) |
| `/agents/:id` | Three-pane workspace |
| `/agents/advertising-analyst` | End-to-end Meta ROAS diagnosis workspace |
| `/runs` · `/runs/:id` | Run list and replay |
| `/tasks` | Planner DAG |
| `/tools` | Elasticsearch MCP console |
| `/memory` | Short-term / task / long-term memory |
| `/semantic` | Metrics, dimensions, query builder, compiler |
| `/approvals` | Guardrail HITL |
| `/settings` | API / SSE / mock flags |

## Develop

```bash
cp .env.example .env
npm install
npm run dev
```

```bash
npm run test
npm run lint
npm run build
```

Default talks to musk-back at `http://127.0.0.1:8080` through the Vite `/api` proxy. Set `VITE_USE_MOCK=true` only if you need a local replay without the backend.

```bash
cd musk-back && make run
cd musk-web && npm run dev
```

Open http://localhost:5173/ , submit a goal (default `Analyze why Meta ad ROI declined yesterday`). That issues:

```
POST /api/v1/agent/run
X-Request-ID: <uuid>
{ "message": "<goal>" }
```

then polls `GET /api/v1/runs/:id` until COMPLETED / FAILED / WAITING_APPROVAL. musk-back has **no SSE** stream.

```
VITE_API_BASE_URL=http://127.0.0.1:8080
VITE_USE_MOCK=false
```

## Live self-test (real LLM, spends tokens)

Prerequisites: `musk-back/.env` lines 1–3 (`LLM_BASE_URL` / `LLM_API_KEY` / `LLM_MODEL` = Gemini `gemini-3.5-flash`). **Do not commit `.env`.**

```bash
# terminal 1
cd musk-back && make run

# terminal 2 — API live test (POST /api/v1/agent/run)
cd musk-back && make selftest

# pages (Playwright, http://localhost:5173, live mode)
cd musk-web && npm run selftest
```

From the repo root after the backend is up:

```bash
make selftest
```

`npm test` remains mocked Vitest unit tests. `npm run selftest` / `npm run test:e2e` hit the real console + musk-back.

If `/health` is down, tests fail with: start musk-back with musk-back/.env.

REST used:

- `POST /api/v1/agent/run` — `{ "message" }` → `{ run_id, status, answer, report?, ... }`
- `GET /api/v1/runs/:id` — persisted Goal / Decisions / Tool Calls / Observations / Reflections
- `GET /api/v1/agents` · `/api/v1/tools` · `/api/v1/semantic/metrics` · `/api/v1/semantic/dimensions`
- `POST /api/v1/agent/approvals/:id/approve` · `/reject`
- `POST /api/v1/agent/resume`

Data flow: **POST (async) + poll GET run → mapRun → AgentRunStore reducer → Vue**. Components never mutate core agent state.

Mocks remain behind `VITE_USE_MOCK=true`. They are not used as fake production APIs by default.
