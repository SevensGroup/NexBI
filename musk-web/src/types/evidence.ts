export interface Evidence {
  id: string
  runId: string
  description: string
  period?: string
  dimension?: string
  metrics?: Record<string, unknown>
  queryId?: string
  toolCallId?: string
  resultRef?: string
  metricKey?: string
}
