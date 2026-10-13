import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
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
import type { TimelineItem, RunSummary } from '@/types/run'
import { applyAgentEvent, createEmptyRunState, type AgentRunState } from './reducer'

export const useAgentRunStore = defineStore('agentRun', () => {
  const state = ref<AgentRunState>(createEmptyRunState())
  const history = ref<RunSummary[]>([])

  function reset(): void {
    state.value = createEmptyRunState()
  }

  function rememberCurrent(): void {
    const s = state.value
    if (!s.runId) return
    const row: RunSummary = {
      id: s.runId,
      goal: s.goal,
      status: s.status,
      startedAt: s.events[0]?.timestamp ?? new Date().toISOString(),
      durationMs: 0,
      steps: s.decisions.length,
      llmCalls: 0,
      tokens: 0,
      costUsd: 0,
      confidence: s.report?.confidence ?? s.decisions.at(-1)?.confidence,
    }
    history.value = [row, ...history.value.filter((h) => h.id !== row.id)].slice(0, 30)
  }

  function ingest(event: AgentEvent): void {
    state.value = applyAgentEvent(state.value, event)
  }

  function ingestMany(events: AgentEvent[]): void {
    let next = state.value
    for (const event of events) next = applyAgentEvent(next, event)
    state.value = next
  }

  function selectEvent(id: string | null): void {
    state.value = { ...state.value, selectedEventId: id }
  }

  function selectTask(id: string | null): void {
    state.value = { ...state.value, selectedTaskId: id }
  }

  function selectEvidence(id: string | null): void {
    state.value = { ...state.value, selectedEvidenceId: id }
  }

  function retryTool(toolCallId: string): void {
    const tc = state.value.toolCalls.find((t) => t.id === toolCallId)
    if (!tc || !state.value.runId) return
    const now = new Date().toISOString()
    ingest({
      id: `retry-${Date.now()}`,
      runId: state.value.runId,
      step: state.value.events.length,
      type: 'tool.started',
      timestamp: now,
      payload: {
        toolCall: {
          ...tc,
          id: `${tc.id}-retry`,
          status: 'RUNNING',
          duration: 0,
          error: undefined,
          timestamp: now,
        },
      },
    })
  }

  const selectedEvent = computed(() => state.value.events.find((e) => e.id === state.value.selectedEventId) ?? null)
  const selectedTask = computed(() => state.value.plan?.tasks.find((t) => t.id === state.value.selectedTaskId) ?? null)
  const selectedEvidence = computed(() => state.value.evidence.find((e) => e.id === state.value.selectedEvidenceId) ?? null)
  const pendingApprovals = computed(() => state.value.approvals.filter((a) => a.status === 'PENDING'))
  const usedMemories = computed(() => state.value.memories.filter((m) => m.used))
  const latestReflection = computed(() => state.value.reflections.at(-1) ?? null)
  const latestDecision = computed(() => state.value.decisions.at(-1) ?? null)

  return {
    state,
    history,
    reset,
    rememberCurrent,
    ingest,
    ingestMany,
    selectEvent,
    selectTask,
    selectEvidence,
    retryTool,
    selectedEvent,
    selectedTask,
    selectedEvidence,
    pendingApprovals,
    usedMemories,
    latestReflection,
    latestDecision,
  }
})

export type { AgentRunState }
export { applyAgentEvent, createEmptyRunState }

export interface RunSlice {
  runId: string | null
  goal: string
  status: AgentStatus
  understanding: GoalUnderstanding | null
  plan: Plan | null
  tasks: Task[]
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
}
