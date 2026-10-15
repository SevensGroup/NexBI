import type { AgentEvent } from '@/types/events'
import type { AgentStatus, GoalUnderstanding } from '@/types/agent'
import type { Decision } from '@/types/decision'
import type { Plan, Task } from '@/types/task'
import type { Observation, ToolCall } from '@/types/tool'
import type { Reflection } from '@/types/reflection'
import type { Evidence } from '@/types/evidence'
import type { MemoryItem } from '@/types/memory'
import type { Approval } from '@/types/approval'
import type { Report } from '@/types/report'
import type { SemanticQuery } from '@/types/semantic'
import type { MCPExchange } from '@/types/mcp'
import type { TimelineItem } from '@/types/run'
import { statusFromEvent } from '@/utils/status'
import { clonePlain } from '@/utils/clone'
import { MOCK_MEMORIES } from '@/mocks/catalog'

export interface AgentRunState {
  runId: string | null
  goal: string
  status: AgentStatus
  understanding: GoalUnderstanding | null
  plan: Plan | null
  decisions: Decision[]
  toolCalls: ToolCall[]
  observations: Observation[]
  reflections: Reflection[]
  evidence: Evidence[]
  memories: MemoryItem[]
  approvals: Approval[]
  report: Report | null
  queries: SemanticQuery[]
  mcp: MCPExchange[]
  events: AgentEvent[]
  timeline: TimelineItem[]
  collaborations: Array<{ from: string; to: string; task: string; status: string; timestamp: string }>
  selectedEventId: string | null
  selectedTaskId: string | null
  selectedEvidenceId: string | null
  error: string | null
  compiledDsl: Record<string, unknown> | null
  boardUrl: string
}

export function createEmptyRunState(): AgentRunState {
  return {
    runId: null,
    goal: '',
    status: 'UNDERSTANDING',
    understanding: null,
    plan: null,
    decisions: [],
    toolCalls: [],
    observations: [],
    reflections: [],
    evidence: [],
    memories: [],
    approvals: [],
    report: null,
    queries: [],
    mcp: [],
    events: [],
    timeline: [],
    collaborations: [],
    selectedEventId: null,
    selectedTaskId: null,
    selectedEvidenceId: null,
    error: null,
    compiledDsl: null,
    boardUrl: '',
  }
}

export function applyAgentEvent(prev: AgentRunState, rawEvent: AgentEvent): AgentRunState {
  const state = clonePlain(prev)
  const event = clonePlain(rawEvent)
  state.events = [...state.events, event]
  if (!state.runId) state.runId = event.runId
  const mapped = statusFromEvent(event.type)
  if (mapped && state.status !== 'WAITING_APPROVAL') {
    state.status = mapped
  }
  if (event.type === 'approval.required') state.status = 'WAITING_APPROVAL'
  if (event.type === 'approval.approved' || event.type === 'approval.rejected') {
    if (state.status === 'WAITING_APPROVAL') state.status = 'EXECUTING'
  }

  switch (event.type) {
    case 'agent.started':
      state.goal = String(event.payload.goal ?? state.goal)
      state.status = 'UNDERSTANDING'
      pushTimeline(state, event, 'goal', 'Goal', state.goal)
      break
    case 'goal.understood':
      state.understanding = {
        raw: String(event.payload.raw ?? state.goal),
        channel: event.payload.channel as string | undefined,
        metric: event.payload.metric as string | undefined,
        timeRange: event.payload.timeRange as string | undefined,
        summary: String(event.payload.summary ?? ''),
      }
      if (!state.goal) state.goal = state.understanding.raw
      pushTimeline(state, event, 'understanding', 'Understanding', state.understanding.summary)
      break
    case 'plan.created': {
      const plan = event.payload.plan as Plan
      state.plan = plan
      pushTimeline(state, event, 'plan', 'Plan', `${plan.tasks.length} tasks`)
      break
    }
    case 'task.created':
      upsertTask(state, event.payload.task as Task)
      break
    case 'task.started':
      patchTask(state, String(event.payload.taskId), 'RUNNING')
      break
    case 'task.completed':
      patchTask(state, String(event.payload.taskId), 'COMPLETED')
      break
    case 'task.failed':
      patchTask(state, String(event.payload.taskId), 'FAILED')
      break
    case 'decision.created': {
      const decision = event.payload.decision as Decision
      state.decisions = [...state.decisions, decision]
      const thought = (decision.thought || decision.reason || decision.summary || '').trim()
      if (thought) pushTimeline(state, event, 'thinking', 'Thought', thought)
      break
    }
    case 'tool.started': {
      const toolCall = event.payload.toolCall as ToolCall
      state.toolCalls = [...state.toolCalls.filter((t) => t.id !== toolCall.id), { ...toolCall, status: 'RUNNING' }]
      pushTimeline(state, event, 'tool', 'Tool Started', toolCall.toolName)
      break
    }
    case 'tool.completed': {
      const id = String(event.payload.toolCallId)
      state.toolCalls = state.toolCalls.map((t) =>
        t.id === id
          ? { ...t, status: 'COMPLETED', duration: Number(event.payload.duration ?? t.duration), result: event.payload.result }
          : t,
      )
      pushTimeline(state, event, 'tool', 'Tool Completed', id)
      break
    }
    case 'tool.failed': {
      const id = String(event.payload.toolCallId ?? '')
      state.toolCalls = state.toolCalls.map((t) =>
        t.id === id ? { ...t, status: 'FAILED', error: String(event.payload.error ?? 'tool failed') } : t,
      )
      state.error = String(event.payload.error ?? 'tool failed')
      pushTimeline(state, event, 'tool', 'Tool Failed', String(event.payload.error ?? ''))
      break
    }
    case 'observation.created': {
      const observation = event.payload.observation as Observation
      state.observations = [...state.observations, observation]
      if (observation.evidence) {
        maybeEvidence(state, observation.evidence, observation.summary)
      }
      pushTimeline(state, event, 'observation', 'Observation', observation.summary)
      break
    }
    case 'reflection.created': {
      const reflection = event.payload.reflection as Reflection
      state.reflections = [...state.reflections, reflection]
      pushTimeline(state, event, 'thinking', 'Thought', reflectionThought(reflection))
      break
    }
    case 'agent.delegated':
      state.collaborations = [
        ...state.collaborations,
        {
          from: String(event.payload.from ?? 'Coordinator'),
          to: String(event.payload.to ?? ''),
          task: String(event.payload.task ?? ''),
          status: String(event.payload.status ?? 'delegated'),
          timestamp: event.timestamp,
        },
      ]
      pushTimeline(state, event, 'delegate', `${event.payload.from} → ${event.payload.to}`, String(event.payload.status ?? 'delegated'))
      break
    case 'memory.retrieved': {
      const ids = (event.payload.ids as string[]) ?? []
      const found = MOCK_MEMORIES.filter((m) => ids.includes(m.id)).map((m) => ({ ...m, used: true }))
      mergeMemories(state, found)
      pushTimeline(state, event, 'memory', 'Memory Retrieved', ids.join(', '))
      break
    }
    case 'memory.saved':
      mergeMemories(state, [
        {
          id: String(event.payload.id ?? `mem-${event.id}`),
          type: (event.payload.type as MemoryItem['type']) ?? 'TASK_MEMORY',
          agent: 'Coordinator',
          title: 'Saved memory',
          content: String(event.payload.content ?? 'Persisted task conclusion'),
          tags: [],
          timestamp: event.timestamp,
        },
      ])
      break
    case 'approval.required': {
      const approval = event.payload.approval as Approval
      state.approvals = [...state.approvals.filter((a) => a.id !== approval.id), approval]
      pushTimeline(state, event, 'approval', 'Risk Check · Needs Approval', approval.action)
      break
    }
    case 'approval.approved':
    case 'approval.rejected': {
      const id = String(event.payload.approvalId ?? event.payload.id ?? '')
      const status = event.type === 'approval.approved' ? 'APPROVED' : 'REJECTED'
      state.approvals = state.approvals.map((a) => (a.id === id ? { ...a, status } : a))
      pushTimeline(state, event, 'approval', status === 'APPROVED' ? 'Approved · Resume' : 'Rejected', id)
      break
    }
    case 'semantic.query.created': {
      const query = event.payload.query as SemanticQuery
      state.queries = [...state.queries, query]
      pushTimeline(state, event, 'semantic', 'Semantic Query', query.metrics?.join(', '))
      break
    }
    case 'semantic.query.compiled':
      state.compiledDsl = (event.payload.dsl as Record<string, unknown>) ?? null
      pushTimeline(state, event, 'semantic', 'Query Compiled', 'Elasticsearch DSL')
      break
    case 'mcp.started':
      pushTimeline(state, event, 'mcp', 'MCP', String(event.payload.tool ?? 'elasticsearch'))
      break
    case 'mcp.completed': {
      const exchange: MCPExchange = {
        id: event.id,
        semanticQuery: (state.queries.at(-1) as unknown as Record<string, unknown>) ?? {},
        generatedDsl: state.compiledDsl ?? {},
        mcpRequest: (event.payload.request as Record<string, unknown>) ?? { tool: event.payload.tool },
        elasticsearchResult: event.payload.result,
        latencyMs: Number(event.payload.latencyMs ?? 0),
        status: 'ok',
        tool: String(event.payload.tool ?? 'aggregate'),
      }
      state.mcp = [...state.mcp, exchange]
      pushTimeline(state, event, 'mcp', 'Elasticsearch Result', `${exchange.latencyMs}ms`)
      break
    }
    case 'mcp.failed':
      state.error = String(event.payload.error ?? 'MCP failed')
      pushTimeline(state, event, 'mcp', 'MCP Failed', state.error)
      break
    case 'report.created': {
      state.report = event.payload.report as Report
      for (const item of state.report.evidence ?? []) {
        maybeEvidence(state, item.id, item.description, item)
      }
      pushTimeline(state, event, 'report', 'Final Report', state.report.summary.slice(0, 80))
      break
    }
    case 'agent.completed':
      state.status = 'COMPLETED'
      if (typeof event.payload.boardUrl === 'string' && event.payload.boardUrl) {
        state.boardUrl = event.payload.boardUrl
      }
      pushTimeline(state, event, 'report', 'Completed', state.boardUrl || String(event.payload.status ?? 'COMPLETED'))
      break
    case 'agent.failed':
      state.status = 'FAILED'
      state.error = String(event.payload.error ?? 'agent failed')
      pushTimeline(state, event, 'error', 'Failed', state.error)
      break
    default:
      break
  }

  return state
}

function reflectionThought(reflection: Reflection): string {
  const summary = reflection.summary?.trim()
  if (summary && summary !== reflection.nextAction) return summary
  switch (reflection.nextAction) {
    case 'FINISH':
      return 'The goal is done and the evidence is enough, so I can stop.'
    case 'REPLAN':
      return 'This plan is stuck. I should replan before the next action.'
    case 'ASK_USER':
      return 'I am missing something only the user can answer, so I should ask.'
    default:
      return 'This step is not the whole goal. I should read what just came back and decide the next move.'
  }
}

function pushTimeline(
  state: AgentRunState,
  event: AgentEvent,
  kind: TimelineItem['kind'],
  title: string,
  subtitle?: string,
): void {
  const previous = state.timeline.at(-1)
  const started = previous ? Date.parse(previous.timestamp) : NaN
  const ended = Date.parse(event.timestamp)
  const durationMs = Number.isFinite(started) && Number.isFinite(ended) && ended >= started ? ended - started : undefined
  state.timeline = [
    ...state.timeline,
    { id: `tl-${event.id}`, kind, title, subtitle, timestamp: event.timestamp, eventId: event.id, durationMs },
  ]
}

function upsertTask(state: AgentRunState, task: Task): void {
  if (!state.plan) {
    state.plan = { id: 'plan', goal: state.goal, tasks: [task] }
    return
  }
  const exists = state.plan.tasks.some((t) => t.id === task.id)
  state.plan = {
    ...state.plan,
    tasks: exists ? state.plan.tasks.map((t) => (t.id === task.id ? { ...t, ...task } : t)) : [...state.plan.tasks, task],
  }
}

function patchTask(state: AgentRunState, id: string, status: Task['status']): void {
  if (!state.plan) return
  state.plan = {
    ...state.plan,
    tasks: state.plan.tasks.map((t) => (t.id === id ? { ...t, status } : t)),
  }
  state.selectedTaskId = id
}

function maybeEvidence(
  state: AgentRunState,
  id: string,
  description: string,
  extra?: Partial<Evidence>,
): void {
  if (!id || state.evidence.some((e) => e.id === id)) {
    if (id) {
      state.evidence = state.evidence.map((e) => (e.id === id ? { ...e, description, ...extra } : e))
    }
    return
  }
  state.evidence = [
    ...state.evidence,
    {
      id,
      runId: state.runId ?? '',
      description,
      period: extra?.period,
      dimension: extra?.dimension,
      metrics: extra?.metrics,
      queryId: extra?.queryId,
      toolCallId: extra?.toolCallId,
      metricKey: extra?.metricKey,
    },
  ]
}

function mergeMemories(state: AgentRunState, items: MemoryItem[]): void {
  const map = new Map(state.memories.map((m) => [m.id, m]))
  for (const item of items) map.set(item.id, { ...map.get(item.id), ...item })
  state.memories = [...map.values()]
}
