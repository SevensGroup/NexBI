import type { AgentEvent } from '@/types/events'
import { buildAdvertisingEvents, buildHitlEvents, DEFAULT_GOAL } from '@/mocks'

export interface MockStreamHandle {
  stop: () => void
}

export function playMockEventStream(
  events: AgentEvent[],
  onEvent: (event: AgentEvent) => void,
  intervalMs = 220,
): MockStreamHandle {
  let i = 0
  let stopped = false
  const tick = () => {
    if (stopped) return
    if (i >= events.length) return
    onEvent(events[i])
    i += 1
    if (i < events.length) timer = setTimeout(tick, intervalMs)
  }
  let timer: ReturnType<typeof setTimeout> = setTimeout(tick, 40)
  return {
    stop() {
      stopped = true
      clearTimeout(timer)
    },
  }
}

export function eventsForGoal(goal: string, runId: string): AgentEvent[] {
  const g = goal.trim()
  if (g.toLowerCase().includes('reallocate') || g.toLowerCase().includes('budget') || g.toLowerCase().includes('approval')) {
    return buildHitlEvents(runId)
  }
  return buildAdvertisingEvents(runId, g || DEFAULT_GOAL)
}
