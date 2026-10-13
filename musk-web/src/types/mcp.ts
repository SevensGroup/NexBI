export type MCPHealth = 'healthy' | 'degraded' | 'down'

export interface MCPServer {
  id: string
  name: string
  description: string
  health: MCPHealth
  latencyMs: number
  tools: MCPTool[]
}

export interface MCPTool {
  name: string
  description: string
  status: 'idle' | 'running' | 'ok' | 'error'
}

export interface MCPExchange {
  id: string
  semanticQuery: Record<string, unknown>
  generatedDsl: Record<string, unknown>
  mcpRequest: Record<string, unknown>
  elasticsearchResult: unknown
  latencyMs: number
  status: 'ok' | 'error'
  tool: string
}
