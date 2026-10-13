export type AgentEventType =
  | 'agent.started'
  | 'agent.completed'
  | 'agent.failed'
  | 'goal.understood'
  | 'plan.created'
  | 'task.created'
  | 'task.started'
  | 'task.completed'
  | 'task.failed'
  | 'decision.created'
  | 'tool.started'
  | 'tool.completed'
  | 'tool.failed'
  | 'observation.created'
  | 'reflection.created'
  | 'agent.delegated'
  | 'memory.retrieved'
  | 'memory.saved'
  | 'approval.required'
  | 'approval.approved'
  | 'approval.rejected'
  | 'semantic.query.created'
  | 'semantic.query.compiled'
  | 'mcp.started'
  | 'mcp.completed'
  | 'mcp.failed'
  | 'report.created'

export interface AgentEvent {
  id: string
  runId: string
  step: number
  type: AgentEventType
  timestamp: string
  payload: Record<string, unknown>
}
