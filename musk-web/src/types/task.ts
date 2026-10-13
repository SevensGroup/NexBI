export type TaskStatus = 'PENDING' | 'READY' | 'RUNNING' | 'COMPLETED' | 'FAILED'

export interface Task {
  id: string
  title: string
  description: string
  dependencies: string[]
  priority: number
  status: TaskStatus
  agent?: string
  evidenceIds?: string[]
}

export interface Plan {
  id: string
  goal: string
  tasks: Task[]
}
