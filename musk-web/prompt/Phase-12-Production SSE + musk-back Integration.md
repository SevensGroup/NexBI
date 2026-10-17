# NexBI Phase 12 — Production SSE + musk-back Integration

Connect the real musk-back service.
REST: POST /api/v1/agent/run. Request {goal, agentId?, context?}. Response {runId}.
SSE: GET /api/v1/agent/runs/:runId/stream.
Define AgentEvent: id, runId, step, type, timestamp, payload.
Events: agent.started/completed/failed, goal.understood, plan.created, task.created/started/completed/failed, decision.created, tool.started/completed/failed, observation.created, reflection.created, agent.delegated, memory.retrieved/saved, approval.required/approved/rejected, semantic.query.created/compiled, mcp.started/completed/failed, report.created.
Create services/sse/SSEClient.ts, stores/agentRun.ts, and types/events.ts.
Data flow: SSE → Parser → AgentRunStore/Reducer → Vue UI.
Components must not mutate core Agent state directly.
Event mapping: decision.created = DECIDING, tool.started = EXECUTING, reflection.created = REFLECTING, approval.required = WAITING_APPROVAL, agent.completed = COMPLETED.
SSE supports automatic reconnect, heartbeat, timeout, Last-Event-ID, and connection recovery.
Create ConnectionStatus: Connected, Reconnecting, Disconnected.
Implement run replay on /runs and /runs/:id.
Add Active Agents, Completed Today, Success Rate, Average Runtime, LLM Calls, Tokens, and Cost to the dashboard.
Configure .env.example with VITE_API_BASE_URL and VITE_SSE_BASE_URL.
Finish the README, lint, test, and build. Remove unnecessary fake production APIs. Keep mocks for local development.

When finished, run npm run build and confirm there are no obvious TypeScript errors, unhandled exceptions, or meaningless TODOs.
