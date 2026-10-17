# NexBI

SEVENS · AI Growth Data Hub

An operator types one natural-language sentence and gets a cross-channel, metric-consistent, interactive dashboard in seconds.

NexBI is the AI data analyst for Chinese brands selling overseas. It follows the product shape of Polar Analytics and fits the channel mix those brands actually run: Meta, Google Ads, TikTok, GA4, and, once connected, Shopify and Amazon.

## The problem

Ad data sits in separate platforms. Traditional BI (Quick BI, Tencent Cloud BI) takes a long time to set up and locks people into fixed dimensions. Non-technical operators cannot answer a full-channel ROI question on their own. Overseas, Polar Analytics has already turned an ecommerce data stack plus an AI analyst into a product; its website says it serves more than 4,000 brands and agencies. The Chinese outbound-brand market still lacks an AI-native analytics tool.

## What it does

Someone asks in plain language — for example, “Build me an advertising data board” or “Why did Meta ROAS drop yesterday?” The agent understands the metric, the period, the channel, and the comparison, reads the numbers through a controlled query channel, and publishes a dashboard.

A finished board has a title, a short conclusion, KPI cards, and a table. It lives at a public link, `/bi/<code>`, so it can be opened on its own. On a run, the same board is visible beside the agent’s decisions, and it can also be opened in a new page.

Every number is meant to be checkable: one shared definition, the SQL behind it, and the time the data was refreshed.

## Product scope

In this product:

- Natural-language questions and follow-ups
- Cross-channel queries
- Chart and layout generation
- Saving and sharing dashboards
- A metric semantic layer
- An evaluation set and regression
- Multi-tenancy and permissions
- An MCP / API so a customer’s own agents can call in

Held for a separate track: a general-purpose warehouse, attribution modeling (MMM), mobile, and a full manual dashboard editor.

## Architecture

The agent never connects to ad platforms directly. It only reads through DataHub’s controlled query channel. That is the interface-isolation rule of the Growth Data Hub.

```text
Entry
  Web chat            Ask, follow up, edit charts
  Feishu webhook      Scheduled push and questions
  MCP / API           A customer's own agents call in
        │ natural language
        ▼
Agent orchestration
  Intent              Metric, period, channel, comparison
  Task planner        Split the ask into steps
  Schema mapping      Align fields across platforms
  SQL                 Read-only, allowlist, cost limits
  Charts and layout   Pick a chart from the shape of the data
        │ query plan / result sets
        ▼
Execution and rendering
  Multi-source SQL    Through the DataHub query channel
  Render protocol     JSON spec, drawn by front-end components
  Dashboard service   Save, share, versions
  Evaluation          Question set, query logs, cost
        │
        ▼
DataHub
  Semantic layer      One definition each for ROAS, CTR, CPC, CPM, AOV, GMV
  DWS / DWD / ODS     Layered warehouse and wide ad tables
  Qdrant              Vector search over metrics and schemas
  Data quality        Great Expectations
        ▲ about a million rows ingested daily
Sources
  Meta, TikTok, GA4         Ingested
  Google Ads                ATC fields connected
  Shopify, Amazon           Integration still to confirm
```

## Design

| Topic | Approach | Why |
| --- | --- | --- |
| Metric semantic layer | Define ROAS, CTR, CPC, CPM, AOV, GMV and the rest once. Map each definition onto every platform’s fields. The model may cite only defined metrics. | Numbers that do not reconcile are the main reason people stop trusting this kind of product. |
| Schema mapping | Retrieve the relevant metrics and tables from Qdrant, then let the model write the mapping. Turn frequent mappings into rules. | Lower latency, fewer wrong joins. |
| SQL guardrails | Read-only account. Syntax and table allowlists. Caps on rows, scan volume, and runtime. Tenant row-level filters. | Security and cost. |
| Charts and layout | Choose the chart from the shape of the data: time series, comparison, share, or funnel. Emit the layout as a JSON spec. | The front end stays independent of the agent, and new components plug in by protocol. |
| Traceability | Any figure can be opened to its definition, its SQL, and the data refresh time. | Operators can check a number themselves. |
| Evaluation set | Analysts curate real questions with reference answers. Every change runs the regression. | Accuracy is a measured number. |
| Model selection | A stronger model for complex planning. A faster model for simple lookups and chart edits. Cache frequent questions. | Latency and cost stay bounded. |

## Repository

| Piece | Path | Role |
| --- | --- | --- |
| Agent | [`musk-back`](musk-back/README.md) | Go service on CloudWeGo Eino. Understands the goal, plans, calls tools, reflects, and returns a report. |
| Console | [`musk-web`](musk-web/README.md) | Vue 3. A run shows the agent’s decisions on the left and the board on the right. |
| Boards | `/bi/<code>` | Public dashboard links. Each code is 11 lowercase letters and is stored in MySQL `bi_public_board`. |
| Semantic catalog | `musk-back/configs/semantic/` | Shared metric and dimension definitions, including ROAS, CTR, CPC, CPM, CVR, CPA, spend, and revenue. |

The agent publishes a board with the `bi_dashboard` tool: first query the advertising rows, then create the board from that result. The model endpoint is OpenAI-compatible. Gemini is primary; DeepSeek is the fallback when Gemini fails.

## Run locally

Secrets live in `musk-back/.env`. That file is gitignored. Do not commit it.

```bash
# terminal 1
cd musk-back && make run

# terminal 2
cd musk-web && npm install && npm run dev
```

- API health: <http://127.0.0.1:8080/health>
- Console: <http://localhost:5173/>
- A board: <http://localhost:5173/bi/&lt;code&gt;

Demo login is `demo` / `musk888`. A useful first goal is `Build me an advertising data board`. Configuration, APIs, and live self-test commands are in [musk-back/README.md](musk-back/README.md) and [musk-web/README.md](musk-web/README.md).
