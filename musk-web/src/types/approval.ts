import type { RiskLevel } from './decision'

export type ApprovalStatus = 'PENDING' | 'APPROVED' | 'REJECTED'

export interface Approval {
  id: string
  runId: string
  action: string
  description: string
  risk: RiskLevel
  reason: string
  confidence: number
  status: ApprovalStatus
  currentValue?: string
  proposedValue?: string
  tool?: string
  timestamp: string
}
