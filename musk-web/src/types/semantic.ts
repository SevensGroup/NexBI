export interface SemanticFilter {
  dimension: string
  op: string
  value: unknown
}

export interface SemanticQuery {
  metrics: string[]
  dimensions: string[]
  filters: SemanticFilter[]
  timeRange: { start: string; end: string; label?: string }
  orderBy?: Array<{ field: string; direction?: string }>
  limit?: number
  dataSource?: string
}

export interface SemanticMetric {
  key: string
  name: string
  description: string
  field?: string
  aggregation?: string
  formula?: string
  aliases?: string[]
}

export interface SemanticDimension {
  key: string
  name: string
  description: string
  field: string
  type: string
  entity?: string
  aliases?: string[]
}

export interface SemanticEntity {
  key: string
  name: string
  description: string
  keyField: string
  dimensions: string[]
}

export interface SemanticDataSource {
  key: string
  name: string
  description: string
  index: string
  dateField: string
  default?: boolean
}

export interface CompiledQuery {
  semantic: SemanticQuery
  dsl: Record<string, unknown>
  latencyMs?: number
}
