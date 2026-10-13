import { defineStore } from 'pinia'
import { ref } from 'vue'
import type { ConnectionState } from '@/services/sse/SSEClient'

export const useConnectionStore = defineStore('connection', () => {
  const status = ref<ConnectionState>('Reconnecting')
  const lastEventId = ref<string | undefined>()
  const error = ref<string | null>(null)

  function setStatus(next: ConnectionState): void {
    status.value = next
  }

  function setLastEventId(id: string | undefined): void {
    lastEventId.value = id
  }

  function setError(msg: string | null): void {
    error.value = msg
  }

  return { status, lastEventId, error, setStatus, setLastEventId, setError }
})
