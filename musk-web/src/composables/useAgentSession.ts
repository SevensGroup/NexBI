import { eventsForGoal, playMockEventStream, type MockStreamHandle } from '@/services/sse/mockStream'
import {
  eventsFromBackendDelta,
  extractApiError,
  fetchRun,
  isMockMode,
  newRunId,
  pollRun,
  startAgentRun,
} from '@/services/api/agent'
import { useAgentRunStore } from '@/stores/agentRun'
import { useConnectionStore } from '@/stores/connection'
import type { BackendRun } from '@/types/run'
import { isLiveSampleCatalog, isSampleCatalogMessage } from '@/utils/runDisplay'
import router from '@/router'

let mockHandle: MockStreamHandle | null = null
let liveAbort: AbortController | null = null

function failLocal(runId: string, message: string): void {
  if (isSampleCatalogMessage(message) || isLiveSampleCatalog(runId, isMockMode())) return
  const runStore = useAgentRunStore()
  const conn = useConnectionStore()
  conn.setError(message)
  if (runStore.state.status === 'FAILED' && runStore.state.error === message) return
  runStore.ingest({
    id: `local-fail-${runId}-${Date.now()}`,
    runId,
    step: 0,
    type: 'agent.failed',
    timestamp: new Date().toISOString(),
    payload: { error: message },
  })
}

export async function startRun(goal: string, _agentId?: string): Promise<string> {
  stopRunStream()
  const runStore = useAgentRunStore()
  const conn = useConnectionStore()
  runStore.reset()
  conn.setError(null)

  const message = goal.trim()
  if (isMockMode()) {
    const runId = newRunId()
    conn.setStatus('Connected')
    mockHandle = playMockEventStream(eventsForGoal(message, runId), (event) => {
      runStore.ingest(event)
      conn.setLastEventId(event.id)
    })
    return runId
  }

  const runId = newRunId()
  const now = new Date().toISOString()
  runStore.ingest({
    id: `local-start-${runId}`,
    runId,
    step: 0,
    type: 'agent.started',
    timestamp: now,
    payload: { goal: message },
  })
  runStore.ingest({
    id: `local-goal-${runId}`,
    runId,
    step: 0,
    type: 'goal.understood',
    timestamp: now,
    payload: { raw: message, summary: message },
  })
  conn.setStatus('Connected')
  conn.setLastEventId(runId)
  if (router.currentRoute.value.params.id !== runId) {
    void router.replace({ name: 'run-detail', params: { id: runId } })
  }

  const ac = new AbortController()
  liveAbort = ac
  const signal = ac.signal

  const applySnapshot = (from: BackendRun | null, next: BackendRun) => {
    const delta = eventsFromBackendDelta(from, next, { skipBootstrap: true })
    if (delta.length) runStore.ingestMany(delta)
    if (next.error && (next.status === 'FAILED' || next.status === 'ABORTED')) {
      conn.setError(next.error)
    }
  }

  const postP = startAgentRun({ message, runId }).then(
    (ok) => ({ ok }),
    (err: unknown) => ({ error: extractApiError(err) }),
  )

  let last: BackendRun | null = null
  try {
    last = await pollRun(runId, {
      signal,
      onSnapshot: (from, next) => {
        last = next
        applySnapshot(from, next)
      },
    })
    const postRes = await postP
    const latest = (await fetchRun(runId)) ?? last
    if (latest) applySnapshot(last, latest)
    const errText =
      ('error' in postRes && postRes.error) ||
      latest?.error ||
      (latest?.status === 'FAILED' || latest?.status === 'ABORTED' ? latest.status : '')
    if (errText && runStore.state.status !== 'FAILED') {
      failLocal(runId, errText)
    } else if (errText) {
      conn.setError(errText)
    }
    runStore.rememberCurrent()
  } catch (err) {
    failLocal(runId, extractApiError(err))
    runStore.rememberCurrent()
  }

  return runId
}

export async function replayRun(runId: string): Promise<void> {
  const runStore = useAgentRunStore()
  const conn = useConnectionStore()

  if (isLiveSampleCatalog(runId, isMockMode())) {
    stopRunStream()
    runStore.reset()
    conn.setError(null)
    return
  }

  if (runStore.state.runId === runId && runStore.state.events.length > 0) {
    if (!isMockMode()) {
      try {
        const persisted = await fetchRun(runId)
        if (persisted && (persisted.status === 'FAILED' || persisted.status === 'ABORTED') && runStore.state.status !== 'FAILED') {
          runStore.ingestMany(eventsFromBackendDelta({ ...persisted, status: 'RUNNING' }, persisted, { skipBootstrap: true }))
          if (persisted.error) conn.setError(persisted.error)
        }
      } catch (err) {
        failLocal(runId, extractApiError(err))
      }
    }
    return
  }

  stopRunStream()
  runStore.reset()
  conn.setError(null)
  if (isMockMode()) {
    const { replayEvents } = await import('@/services/api/agent')
    runStore.ingestMany(replayEvents(runId))
    conn.setStatus('Connected')
    return
  }
  try {
    const persisted = await fetchRun(runId)
    if (!persisted) {
      failLocal(runId, `agent run not found: ${runId}`)
      return
    }
    runStore.ingestMany(eventsFromBackendDelta(null, persisted))
    if (persisted.error) conn.setError(persisted.error)
    runStore.rememberCurrent()
  } catch (err) {
    failLocal(runId, extractApiError(err))
  }
}

export function stopRunStream(): void {
  mockHandle?.stop()
  mockHandle = null
  liveAbort?.abort()
  liveAbort = null
}
