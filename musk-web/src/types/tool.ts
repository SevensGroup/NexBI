import type { RiskLevel } from './decision'

export type ToolCallStatus = 'PENDING' | 'RUNNING' | 'COMPLETED' | 'FAILED'

export interface ToolCall {
  id: string
  runId: string
  toolName: string
  description: string
  arguments: Record<string, unknown>
  status: ToolCallStatus
  duration: number
  result?: unknown
  error?: string
  risk: RiskLevel
  timestamp: string
}

export interface Observation {
  id: string
  runId: string
  step: number
  summary: string
  reason: string
  evidence: string
  confidence: number
  toolCallId?: string
  timestamp: string
}
