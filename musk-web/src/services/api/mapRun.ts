import type { AgentEvent } from '@/types/events'
import type { BackendRun } from '@/types/run'
import type { Decision, RiskLevel } from '@/types/decision'
import type { Reflection } from '@/types/reflection'
import type { Observation, ToolCall } from '@/types/tool'
import type { Anomaly, Recommendation, Report, RootCause } from '@/types/report'
import { presentRunError } from '@/utils/runDisplay'

const TERMINAL = new Set(['COMPLETED', 'FAILED', 'ABORTED', 'WAITING_APPROVAL'])

export function isTerminalStatus(status: string | undefined): boolean {
  return Boolean(status && TERMINAL.has(status))
}

export function eventsFromBackendRun(run: BackendRun): AgentEvent[] {
  return eventsFromBackendDelta(null, run)
}

export function eventsFromBackendDelta(
  prev: BackendRun | null,
  next: BackendRun,
  opts?: { skipBootstrap?: boolean },
): AgentEvent[] {
  const events: AgentEvent[] = []
  const runId = next.id
  let n = prev ? 1000 + (prev.decisions?.length ?? 0) + (prev.tool_calls?.length ?? 0) : 0
  const push = (type: AgentEvent['type'], step: number, timestamp: string, payload: Record<string, unknown>) => {
    events.push({ id: `be-${runId}-${++n}`, runId, step, type, timestamp, payload })
  }

  if (!prev && !opts?.skipBootstrap) {
    push('agent.started', 0, next.created_at, { goal: next.goal })
    if (next.goal) {
      push('goal.understood', 0, next.created_at, { raw: next.goal, summary: next.goal })
    }
  }

  sliceNew(prev?.decisions, next.decisions).forEach((raw, i) => {
    const step = (prev?.decisions?.length ?? 0) + i + 1
    const d = decisionFromRecord(runId, step, raw)
    push('decision.created', step, d.timestamp, { decision: d })
  })

  sliceNew(prev?.tool_calls, next.tool_calls).forEach((raw, i) => {
    const step = (prev?.tool_calls?.length ?? 0) + i + 1
    const tc = toolFromRecord(runId, raw)
    push('tool.started', step, tc.timestamp, { toolCall: { ...tc, status: 'RUNNING', duration: 0 } })
    if (tc.status === 'FAILED') {
      push('tool.failed', step, tc.timestamp, { toolCallId: tc.id, error: tc.error })
    } else {
      push('tool.completed', step, tc.timestamp, { toolCallId: tc.id, duration: tc.duration, result: tc.result })
    }
  })

  sliceNew(prev?.observations, next.observations).forEach((raw, i) => {
    const step = (prev?.observations?.length ?? 0) + i + 1
    const o = observationFromRecord(runId, step, raw)
    push('observation.created', step, o.timestamp, { observation: o })
  })

  sliceNew(prev?.reflections, next.reflections).forEach((raw, i) => {
    const step = (prev?.reflections?.length ?? 0) + i + 1
    const r = reflectionFromRecord(runId, step, raw)
    push('reflection.created', step, r.timestamp, { reflection: r })
  })

  const becameWaiting = next.status === 'WAITING_APPROVAL' && prev?.status !== 'WAITING_APPROVAL'
  if (becameWaiting && next.approval_id) {
    push('approval.required', next.steps?.length ?? 1, next.updated_at, {
      approval: {
        id: next.approval_id,
        runId,
        action: 'High-risk tool',
        description: 'Waiting for human approval',
        risk: 'HIGH',
        reason: 'Guardrail interrupt',
        confidence: next.confidence ?? 0,
        status: 'PENDING',
        timestamp: next.updated_at,
      },
    })
  }

  const becameTerminal = isTerminalStatus(next.status) && (!prev || prev.status !== next.status)
  if (becameTerminal && next.status === 'COMPLETED') {
    const report = reportFromRun(next)
    if (report) {
      push('report.created', next.steps?.length ?? 1, next.updated_at, { report })
    }
    push('agent.completed', next.steps?.length ?? 1, next.updated_at, {
      status: 'COMPLETED',
      confidence: next.confidence,
      boardUrl: boardLink(next),
    })
  }
  if (becameTerminal && (next.status === 'FAILED' || next.status === 'ABORTED')) {
    push('agent.failed', next.steps?.length ?? 1, next.updated_at, {
      error: presentRunError(next.error) || next.status,
    })
  }

  events.sort((a, b) => {
    const left = Date.parse(a.timestamp)
    const right = Date.parse(b.timestamp)
    if (!Number.isFinite(left) || !Number.isFinite(right) || left === right) return 0
    return left - right
  })
  return events
}

export function boardLink(run: BackendRun): string {
  const blobs: string[] = []
  if (run.answer) blobs.push(run.answer)
  if (run.final_result) blobs.push(run.final_result)
  for (const call of run.tool_calls ?? []) {
    const result = call.result
    if (typeof result === 'string') blobs.push(result)
    else if (result != null) blobs.push(JSON.stringify(result))
  }
  for (const blob of blobs) {
    const hit = linkFromText(blob)
    if (hit) return hit
  }
  return ''
}

function linkFromText(raw: string): string {
  const matched = raw.match(/https?:\/\/[^\s"'<>]+\/bi\/[a-z0-9]+/i)
  if (matched) return matched[0]
  const parsed = tryParseJson(raw)
  if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) return ''
  const row = parsed as Record<string, unknown>
  const code = typeof row.code === 'string' ? row.code.trim() : ''
  if (!/^[a-z0-9]{6,32}$/i.test(code)) return ''
  const origin = typeof window !== 'undefined' ? window.location.origin : 'http://localhost:5173'
  return `${origin.replace(/\/$/, '')}/bi/${code}`
}

export function reportFromRun(run: BackendRun): Report | null {
  const raw = run.answer || run.final_result || ''
  if (raw) {
    const parsed = tryParseJson(raw)
    if (parsed) return normalizeReport(parsed, run.confidence)
    if (raw.trim()) {
      return normalizeReport({ summary: raw, metrics: {}, confidence: run.confidence }, run.confidence)
    }
  }
  return null
}

export function normalizeReport(raw: unknown, fallbackConfidence = 0): Report {
  const o = (raw && typeof raw === 'object' ? raw : {}) as Record<string, unknown>
  const anomalies = Array.isArray(o.anomalies) ? o.anomalies.map(normalizeAnomaly) : []
  const rootRaw = (o.rootCauses ?? o.root_causes) as unknown
  const recRaw = o.recommendations
  return {
    summary: str(o.summary),
    metrics: (o.metrics as Record<string, unknown>) ?? {},
    anomalies,
    rootCauses: Array.isArray(rootRaw) ? rootRaw.map(normalizeCause) : [],
    evidence: Array.isArray(o.evidence) ? (o.evidence as Report['evidence']) : [],
    recommendations: Array.isArray(recRaw) ? recRaw.map(normalizeRec) : [],
    confidence: num(o.confidence, fallbackConfidence),
  }
}

function normalizeAnomaly(raw: unknown): Anomaly {
  const o = (raw && typeof raw === 'object' ? raw : {}) as Record<string, unknown>
  return {
    entity: str(o.entity),
    name: str(o.name),
    metric: str(o.metric),
    current: num(o.current),
    baseline: num(o.baseline),
    changePct: num(o.changePct ?? o.change_pct),
    direction: str(o.direction) || undefined,
    evidenceId: str(o.evidenceId ?? o.evidence_id),
  }
}

function normalizeCause(raw: unknown): RootCause {
  const o = (raw && typeof raw === 'object' ? raw : {}) as Record<string, unknown>
  const ids = o.evidenceIds ?? o.evidence_ids
  return {
    statement: str(o.statement),
    evidenceIds: Array.isArray(ids) ? ids.map((x) => String(x)) : [],
  }
}

function normalizeRec(raw: unknown): Recommendation {
  const o = (raw && typeof raw === 'object' ? raw : {}) as Record<string, unknown>
  const ids = o.evidenceIds ?? o.evidence_ids
  return {
    action: str(o.action),
    evidenceIds: Array.isArray(ids) ? ids.map((x) => String(x)) : [],
  }
}

function sliceNew<T>(prev: T[] | undefined, next: T[] | undefined): T[] {
  const a = prev ?? []
  const b = next ?? []
  if (b.length <= a.length) return []
  return b.slice(a.length)
}

function tryParseJson(raw: string): unknown | null {
  const s = raw.trim()
  if (!s.startsWith('{') && !s.includes('{')) return null
  try {
    return JSON.parse(s)
  } catch {
    const start = s.indexOf('{')
    const end = s.lastIndexOf('}')
    if (start >= 0 && end > start) {
      try {
        return JSON.parse(s.slice(start, end + 1))
      } catch {
        return null
      }
    }
    return null
  }
}

function str(v: unknown, fallback = ''): string {
  return typeof v === 'string' ? v : fallback
}

function num(v: unknown, fallback = 0): number {
  return typeof v === 'number' ? v : fallback
}

function decisionFromRecord(runId: string, step: number, raw: Record<string, unknown>): Decision {
  const risk = (str(raw.risk, 'LOW') || 'LOW').toUpperCase() as RiskLevel
  const args = (raw.arguments as Record<string, unknown>) || undefined
  const agent = str(raw.agent) || str(args?.agent)
  return {
    id: str(raw.step_id, `d-${step}`),
    runId,
    step,
    type: str(raw.type, 'CALL_TOOL') as Decision['type'],
    thought: str(raw.thought) || undefined,
    objective: str(raw.objective),
    summary: str(raw.reason),
    reason: str(raw.reason),
    confidence: num(raw.confidence),
    risk: ['LOW', 'MEDIUM', 'HIGH', 'CRITICAL'].includes(risk) ? risk : 'LOW',
    nextAction: str(raw.tool, str(raw.type)),
    timestamp: str(raw.at, new Date().toISOString()),
    tool: str(raw.tool) || undefined,
    arguments: args,
    agent: agent || undefined,
  }
}

function toolFromRecord(runId: string, raw: Record<string, unknown>): ToolCall {
  const ok = raw.success !== false
  return {
    id: str(raw.id, `tc-${Math.random().toString(36).slice(2, 8)}`),
    runId,
    toolName: str(raw.tool, 'tool'),
    description: str(raw.tool, 'tool'),
    arguments: (raw.arguments as Record<string, unknown>) || {},
    status: ok ? 'COMPLETED' : 'FAILED',
    duration: num(raw.latency_ms),
    result: raw.result,
    error: ok ? undefined : str(raw.result),
    risk: 'LOW',
    timestamp: str(raw.at, new Date().toISOString()),
  }
}

function observationFromRecord(runId: string, step: number, raw: Record<string, unknown>): Observation {
  return {
    id: `obs-${step}`,
    runId,
    step,
    summary: str(raw.summary),
    reason: str(raw.reason),
    evidence: str(raw.evidence),
    confidence: num(raw.confidence),
    timestamp: str(raw.at, new Date().toISOString()),
  }
}

function reflectionFromRecord(runId: string, step: number, raw: Record<string, unknown>): Reflection {
  return {
    id: `ref-${step}`,
    runId,
    step,
    goalSatisfied: Boolean(raw.goal_satisfied),
    evidenceSufficient: Boolean(raw.evidence_sufficient),
    resultCorrect: Boolean(raw.result_correct),
    missingInformation: Array.isArray(raw.missing_information) ? (raw.missing_information as string[]) : [],
    errors: Array.isArray(raw.errors) ? (raw.errors as string[]) : [],
    nextAction: str(raw.next_action, 'CONTINUE'),
    confidence: num(raw.confidence),
    timestamp: str(raw.at, new Date().toISOString()),
  }
}
