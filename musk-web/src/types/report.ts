export interface Anomaly {
  entity: string
  name: string
  metric: string
  current: number
  baseline: number
  changePct: number
  direction?: string
  evidenceId: string
}

export interface RootCause {
  statement: string
  evidenceIds: string[]
}

export interface Recommendation {
  action: string
  evidenceIds: string[]
}

export interface Report {
  summary: string
  metrics: Record<string, unknown>
  anomalies: Anomaly[]
  rootCauses: RootCause[]
  evidence: Array<{
    id: string
    description: string
    period?: string
    dimension?: string
    metrics?: Record<string, unknown>
  }>
  recommendations: Recommendation[]
  confidence: number
}
