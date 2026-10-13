import { defineStore } from 'pinia'
import { ref } from 'vue'
import { isMockMode } from '@/services/api/client'

export type ContextTab = 'plan' | 'evidence' | 'memory' | 'decision' | 'report'

export const useUiStore = defineStore('ui', () => {
  const contextTab = ref<ContextTab>('plan')
  const taskDrawerId = ref<string | null>(null)
  const agentDrawerId = ref<string | null>(null)
  const approvalDrawerId = ref<string | null>(null)
  const memoryDrawerId = ref<string | null>(null)
  const timelineDrawerOpen = ref(false)
  const useMock = ref(isMockMode())

  return {
    contextTab,
    taskDrawerId,
    agentDrawerId,
    approvalDrawerId,
    memoryDrawerId,
    timelineDrawerOpen,
    useMock,
  }
})
