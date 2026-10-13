import { onMounted, onUnmounted } from 'vue'
import { isMockMode } from '@/services/api/client'
import { connectionStatusFromHealth, pingHealth } from '@/services/api/health'
import { useConnectionStore } from '@/stores/connection'

/** Poll GET /health. musk-back has no SSE; missing /stream must not mark Disconnected. */
export function useBackendHealth(intervalMs = 15_000): void {
  const conn = useConnectionStore()
  let timer: ReturnType<typeof setInterval> | undefined

  async function tick(): Promise<void> {
    if (isMockMode()) {
      conn.setStatus('Connected')
      return
    }
    const ok = await pingHealth()
    conn.setStatus(connectionStatusFromHealth({ backendOk: ok, sseAvailable: false }))
  }

  onMounted(() => {
    conn.setStatus('Reconnecting')
    void tick()
    timer = setInterval(() => void tick(), intervalMs)
  })

  onUnmounted(() => {
    if (timer) clearInterval(timer)
  })
}
