import type { AgentStatus } from './agent'
import type { Decision } from './decision'
import type { Observation, ToolCall } from './tool'
import type { Reflection } from './reflection'
import type { Evidence } from './evidence'

export interface RunSummary {
  id: string
  goal: string
  status: AgentStatus | string
  agentId?: string
  startedAt: string
  endedAt?: string
  durationMs: number
  steps: number
  llmCalls: number
  tokens: number
  costUsd: number
  confidence?: number
}

export interface BackendRun {
  id: string
  goal: string
  status: string
  steps?: Array<{ id: string; index: number; started_at: string }>
  decisions?: Array<Record<string, unknown>>
  tool_calls?: Array<Record<string, unknown>>
  observations?: Array<Record<string, unknown>>
  reflections?: Array<Record<string, unknown>>
  final_result?: string
  answer?: string
  confidence?: number
  evidence?: unknown[]
  token_usage?: { input?: number; output?: number; total?: number }
  cost_usd?: number
  latency_ms?: number
  error?: string
  approval_id?: string
  created_at: string
  updated_at: string
}

export interface TimelineItem {
  id: string
  kind:
    | 'goal'
    | 'understanding'
    | 'plan'
    | 'decision'
    | 'thinking'
    | 'semantic'
    | 'mcp'
    | 'tool'
    | 'observation'
    | 'reflection'
    | 'delegate'
    | 'approval'
    | 'report'
    | 'task'
    | 'memory'
    | 'error'
  title: string
  subtitle?: string
  timestamp: string
  durationMs?: number
  eventId: string
  taskId?: string
  related?: {
    decision?: Decision
    tool?: ToolCall
    observation?: Observation
    reflection?: Reflection
    evidence?: Evidence
  }
}
