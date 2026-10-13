export type MemoryType = 'SHORT_TERM' | 'TASK_MEMORY' | 'LONG_TERM'

export interface MemoryItem {
  id: string
  type: MemoryType
  agent: string
  title: string
  content: string
  tags: string[]
  timestamp: string
  used?: boolean
}
