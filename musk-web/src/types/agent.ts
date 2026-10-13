export type AgentStatus =
  | 'UNDERSTANDING'
  | 'PLANNING'
  | 'DECIDING'
  | 'EXECUTING'
  | 'OBSERVING'
  | 'REFLECTING'
  | 'WAITING_APPROVAL'
  | 'COMPLETED'
  | 'FAILED'

export type AgentRole = 'Coordinator' | 'Analyst' | 'Researcher' | 'Reviewer'

export interface AgentInfo {
  id: string
  name: string
  role: AgentRole | string
  description: string
  status: AgentStatus | 'IDLE'
  currentTask?: string
  progress: number
  confidence: number
  tools: string[]
  steps?: number
}

export interface GoalUnderstanding {
  raw: string
  channel?: string
  metric?: string
  timeRange?: string
  summary: string
}
