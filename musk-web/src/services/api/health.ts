import type { ConnectionState } from '@/services/sse/SSEClient'
import { api, isMockMode } from './client'

/** musk-back has no SSE. Connection means REST health, never EventSource /stream. */
export function connectionStatusFromHealth(input: {
  backendOk: boolean
  sseAvailable?: boolean
}): ConnectionState {
  void input.sseAvailable
  return input.backendOk ? 'Connected' : 'Disconnected'
}

export async function pingHealth(): Promise<boolean> {
  if (isMockMode()) return true
  try {
    const { data, status } = await api.get('/health', {
      timeout: 4000,
      validateStatus: () => true,
    })
    if (status < 200 || status >= 300) return false
    if (data && typeof data === 'object' && 'status' in data) {
      return String((data as { status: unknown }).status).toLowerCase() === 'ok'
    }
    return true
  } catch {
    return false
  }
}
