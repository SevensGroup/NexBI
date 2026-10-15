export type DecisionType =
  | 'CALL_TOOL'
  | 'DELEGATE_AGENT'
  | 'REPLAN'
  | 'REFLECT'
  | 'ASK_USER'
  | 'FINISH'
  | 'ABORT'

export type RiskLevel = 'LOW' | 'MEDIUM' | 'HIGH' | 'CRITICAL'

export interface Decision {
  id: string
  runId: string
  step: number
  type: DecisionType
  thought?: string
  objective: string
  summary: string
  reason: string
  confidence: number
  risk: RiskLevel
  nextAction: string
  timestamp: string
  tool?: string
  arguments?: Record<string, unknown>
  agent?: string
}
