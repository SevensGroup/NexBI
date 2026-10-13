import { api, isMockMode } from './client'

export { isMockMode }
import { MOCK_AGENTS, MOCK_APPROVALS, MOCK_DASHBOARD, MOCK_DATASOURCES, MOCK_DIMENSIONS, MOCK_ENTITIES, MOCK_MCP, MOCK_MEMORIES, MOCK_METRICS, MOCK_RUNS } from '@/mocks'
import { buildAdvertisingEvents, buildHitlEvents } from '@/mocks/events'
import type { AgentEvent } from '@/types/events'
import type { Approval } from '@/types/approval'
import type { BackendRun, RunSummary } from '@/types/run'
import type { AgentInfo } from '@/types/agent'
import type { SemanticDimension, SemanticMetric } from '@/types/semantic'
import { eventsFromBackendDelta, eventsFromBackendRun } from './mapRun'
import { isAxiosError } from 'axios'

export const RUN_POST_TIMEOUT_MS = 600_000

export interface StartRunRequest {
  message: string
  runId: string
}

export interface StartRunResponse {
  runId: string
  status?: string
  approvalId?: string
  answer?: string
  report?: unknown
  requestId?: string
}

export function newRunId(): string {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
    return crypto.randomUUID()
  }
  return `run-${Date.now()}-${Math.random().toString(16).slice(2)}`
}

export function extractApiError(err: unknown): string {
  if (isAxiosError(err)) {
    const data = err.response?.data as { error?: string } | undefined
    if (typeof data?.error === 'string' && data.error.trim()) return data.error.trim()
    if (err.message) return err.message
  }
  return err instanceof Error ? err.message : String(err)
}

export function runRequestBody(message: string): { message: string } {
  return { message }
}

export async function startAgentRun(req: StartRunRequest): Promise<StartRunResponse> {
  if (isMockMode()) {
    return { runId: req.runId, status: 'RUNNING' }
  }
  const { data, headers } = await api.post('/api/v1/agent/run', runRequestBody(req.message), {
    headers: { 'X-Request-ID': req.runId },
    timeout: RUN_POST_TIMEOUT_MS,
    validateStatus: (s) => s === 200 || s === 202,
  })
  return {
    runId: data.run_id || data.runId || req.runId,
    status: data.status,
    approvalId: data.approval_id,
    answer: data.answer,
    report: data.report,
    requestId: data.request_id || headers['x-request-id'],
  }
}

export async function fetchRun(runId: string): Promise<BackendRun | null> {
  if (isMockMode()) return null
  try {
    const { data } = await api.get(`/api/v1/runs/${encodeURIComponent(runId)}`)
    return data as BackendRun
  } catch (err) {
    if (isAxiosError(err) && err.response?.status === 404) return null
    throw err
  }
}

export async function pollRun(
  runId: string,
  opts: {
    intervalMs?: number
    timeoutMs?: number
    signal?: AbortSignal
    onSnapshot: (prev: BackendRun | null, next: BackendRun) => void
  },
): Promise<BackendRun | null> {
  const interval = opts.intervalMs ?? 1000
  const timeout = opts.timeoutMs ?? RUN_POST_TIMEOUT_MS
  const started = Date.now()
  let prev: BackendRun | null = null
  let lastErr: string | null = null
  while (!opts.signal?.aborted) {
    if (Date.now() - started > timeout) {
      if (!prev && lastErr) throw new Error(lastErr)
      return prev
    }
    try {
      const next = await fetchRun(runId)
      lastErr = null
      if (next) {
        opts.onSnapshot(prev, next)
        prev = next
        if (next.status === 'COMPLETED' || next.status === 'FAILED' || next.status === 'ABORTED' || next.status === 'WAITING_APPROVAL') {
          return next
        }
      }
    } catch (err) {
      lastErr = extractApiError(err)
    }
    await delay(interval, opts.signal)
  }
  if (!prev && lastErr && !opts.signal?.aborted) throw new Error(lastErr)
  return prev
}

export async function listAgents(): Promise<AgentInfo[]> {
  if (isMockMode()) return MOCK_AGENTS
  const { data } = await api.get('/api/v1/agents')
  const agents = (data.agents ?? data) as Array<Record<string, unknown>>
  return agents.map((a, i) => ({
    id: String(a.name ?? a.id ?? i),
    name: String(a.name ?? 'agent'),
    role: String(a.role ?? a.name ?? ''),
    description: String(a.description ?? ''),
    status: 'IDLE',
    progress: 0,
    confidence: 0,
    tools: Array.isArray(a.tools) ? (a.tools as string[]) : [],
  }))
}

export async function listTools(): Promise<Array<{ name: string; description: string; risk?: string; input_schema?: unknown }>> {
  if (isMockMode()) {
    return MOCK_MCP.tools.map((t) => ({ name: t.name, description: t.description, risk: 'LOW' }))
  }
  const { data } = await api.get('/api/v1/tools')
  return data.tools ?? []
}

export async function listMetrics(): Promise<SemanticMetric[]> {
  if (isMockMode()) return MOCK_METRICS
  const { data } = await api.get('/api/v1/semantic/metrics')
  return (data.metrics ?? []) as SemanticMetric[]
}

export async function listDimensions(): Promise<SemanticDimension[]> {
  if (isMockMode()) return MOCK_DIMENSIONS
  const { data } = await api.get('/api/v1/semantic/dimensions')
  return (data.dimensions ?? []) as SemanticDimension[]
}

export async function approveAction(id: string): Promise<void> {
  if (isMockMode()) return
  await api.post(`/api/v1/agent/approvals/${encodeURIComponent(id)}/approve`, {})
}

export async function rejectAction(id: string, reason: string): Promise<void> {
  if (isMockMode()) return
  await api.post(`/api/v1/agent/approvals/${encodeURIComponent(id)}/reject`, { reason })
}

export async function resumeRun(runId: string): Promise<StartRunResponse> {
  if (isMockMode()) return { runId, status: 'RUNNING' }
  const { data } = await api.post('/api/v1/agent/resume', { run_id: runId })
  return {
    runId: data.run_id || runId,
    status: data.status,
    answer: data.answer,
  }
}

export async function listRuns(): Promise<RunSummary[]> {
  if (isMockMode()) return MOCK_RUNS
  return []
}

export async function listMemories() {
  if (isMockMode()) return MOCK_MEMORIES
  return []
}

export async function listApprovals(): Promise<Approval[]> {
  if (isMockMode()) return MOCK_APPROVALS
  return []
}

export function mockCatalog() {
  return {
    agents: MOCK_AGENTS,
    metrics: MOCK_METRICS,
    dimensions: MOCK_DIMENSIONS,
    entities: MOCK_ENTITIES,
    dataSources: MOCK_DATASOURCES,
    mcp: MOCK_MCP,
    dashboard: MOCK_DASHBOARD,
  }
}

export function replayEvents(runId: string): AgentEvent[] {
  if (runId.startsWith('run-hitl')) return buildHitlEvents(runId)
  if (runId === 'run-fail-01') {
    return [
      {
        id: 'f1',
        runId,
        step: 0,
        type: 'agent.started',
        timestamp: '2026-08-25T16:02:00Z',
        payload: { goal: 'Pull the TikTok creative fatigue report for last week' },
      },
      {
        id: 'f2',
        runId,
        step: 1,
        type: 'tool.failed',
        timestamp: '2026-08-25T16:02:12Z',
        payload: { toolCallId: 'tc-x', error: 'MCP timeout talking to Elasticsearch' },
      },
      {
        id: 'f3',
        runId,
        step: 1,
        type: 'agent.failed',
        timestamp: '2026-08-25T16:02:18Z',
        payload: { error: 'MCP timeout talking to Elasticsearch' },
      },
    ]
  }
  return buildAdvertisingEvents(runId)
}

export { eventsFromBackendRun, eventsFromBackendDelta }

function delay(ms: number, signal?: AbortSignal): Promise<void> {
  return new Promise((resolve) => {
    if (signal?.aborted) {
      resolve()
      return
    }
    const t = setTimeout(resolve, ms)
    signal?.addEventListener('abort', () => {
      clearTimeout(t)
      resolve()
    })
  })
}
