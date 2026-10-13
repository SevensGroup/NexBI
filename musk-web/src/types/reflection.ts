export type ReflectionNextAction = 'CONTINUE' | 'REPLAN' | 'FINISH' | 'ASK_USER'

export interface Reflection {
  id: string
  runId: string
  step?: number
  goalSatisfied: boolean
  evidenceSufficient: boolean
  resultCorrect: boolean
  missingInformation: string[]
  errors: string[]
  nextAction: ReflectionNextAction | string
  confidence: number
  timestamp: string
  summary?: string
}
