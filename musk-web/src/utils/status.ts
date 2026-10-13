import type { AgentStatus } from '@/types'
import type { AgentEventType } from '@/types/events'
import type { RiskLevel } from '@/types/decision'
import type { TaskStatus } from '@/types/task'

export const STATUS_LABEL: Record<AgentStatus, string> = {
  UNDERSTANDING: 'Understanding',
  PLANNING: 'Planning',
  DECIDING: 'Deciding',
  EXECUTING: 'Executing',
  OBSERVING: 'Observing',
  REFLECTING: 'Reflecting',
  WAITING_APPROVAL: 'Needs Approval',
  COMPLETED: 'Completed',
  FAILED: 'Failed',
}

export const STATUS_CLASS: Record<AgentStatus, string> = {
  UNDERSTANDING: 'status-understanding',
  PLANNING: 'status-planning',
  DECIDING: 'status-deciding',
  EXECUTING: 'status-executing',
  OBSERVING: 'status-observing',
  REFLECTING: 'status-reflecting',
  WAITING_APPROVAL: 'status-waiting',
  COMPLETED: 'status-completed',
  FAILED: 'status-failed',
}

export function statusFromEvent(type: AgentEventType): AgentStatus | null {
  switch (type) {
    case 'agent.started':
    case 'goal.understood':
      return 'UNDERSTANDING'
    case 'plan.created':
    case 'task.created':
      return 'PLANNING'
    case 'decision.created':
      return 'DECIDING'
    case 'tool.started':
    case 'mcp.started':
    case 'semantic.query.created':
    case 'semantic.query.compiled':
      return 'EXECUTING'
    case 'observation.created':
    case 'tool.completed':
    case 'mcp.completed':
      return 'OBSERVING'
    case 'reflection.created':
      return 'REFLECTING'
    case 'approval.required':
      return 'WAITING_APPROVAL'
    case 'agent.completed':
    case 'report.created':
      return 'COMPLETED'
    case 'agent.failed':
    case 'task.failed':
    case 'tool.failed':
    case 'mcp.failed':
      return 'FAILED'
    default:
      return null
  }
}

export const RISK_CLASS: Record<RiskLevel, string> = {
  LOW: 'risk-low',
  MEDIUM: 'risk-medium',
  HIGH: 'risk-high',
  CRITICAL: 'risk-critical',
}

export const TASK_STATUS_CLASS: Record<TaskStatus, string> = {
  PENDING: 'task-pending',
  READY: 'task-ready',
  RUNNING: 'task-running',
  COMPLETED: 'task-completed',
  FAILED: 'task-failed',
}

export function isTerminal(status: AgentStatus): boolean {
  return status === 'COMPLETED' || status === 'FAILED'
}
