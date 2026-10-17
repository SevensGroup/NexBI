<script setup lang="ts">
import { computed } from 'vue'
import { useAgentRunStore } from '@/stores/agentRun'
import { formatTime } from '@/utils/format'
import type { Decision } from '@/types/decision'
import type { Reflection } from '@/types/reflection'

interface Step {
  id: string
  at: string
  kind: 'decision' | 'thought'
  label: string
  body: string
}

const run = useAgentRunStore()

const working = computed(() => {
  if (!run.state.runId) return false
  const status = run.state.status
  const stopped = new Set<string>(['COMPLETED', 'FAILED', 'ABORTED', 'WAITING_APPROVAL'])
  return !stopped.has(status)
})

const steps = computed(() => {
  const items: Step[] = []
  for (const decision of run.state.decisions) {
    const decisionText = (decision.objective || decision.summary || '').trim()
    const thought = (decision.thought || decision.reason || '').trim()
    if (decisionText && decisionText !== thought) {
      items.push({
        id: `${decision.id}-decision`,
        at: decision.timestamp,
        kind: 'decision',
        label: decisionLabel(decision),
        body: decisionText,
      })
    }
    if (thought) {
      items.push({
        id: `${decision.id}-thought`,
        at: decision.timestamp,
        kind: 'thought',
        label: 'Thought',
        body: thought,
      })
    }
  }
  for (const reflection of run.state.reflections) {
    const body = reflectionText(reflection)
    if (!body || items.some((item) => item.body === body)) continue
    items.push({
      id: reflection.id,
      at: reflection.timestamp,
      kind: 'thought',
      label: 'Thought',
      body,
    })
  }
  return items.sort((a, b) => Date.parse(a.at) - Date.parse(b.at))
})

function decisionLabel(decision: Decision): string {
  const tool = decision.tool ? ` · ${decision.tool}` : ''
  return `${decision.type}${tool}`
}

function reflectionText(reflection: Reflection): string {
  const summary = reflection.summary?.trim()
  if (summary && summary !== reflection.nextAction) return summary
  switch (reflection.nextAction) {
    case 'FINISH':
      return 'The goal is done and the evidence is enough, so I can stop.'
    case 'REPLAN':
      return 'This plan is stuck. I should replan before the next action.'
    case 'ASK_USER':
      return 'I am missing something only the user can answer, so I should ask.'
    default:
      return 'This step is not the whole goal. I should read what just came back and decide the next move.'
  }
}
</script>

<template>
  <section class="flex h-full min-h-0 flex-col">
    <div class="border-b border-line px-4 py-3">
      <h2 class="text-sm font-medium">Agent</h2>
      <p class="mt-0.5 text-[11px] text-mute">Decisions and thinking</p>
    </div>
    <div class="min-h-0 flex-1 overflow-auto px-4 py-3">
      <p v-if="!steps.length && !working" class="text-sm text-mute">No decision yet.</p>
      <ol class="space-y-3">
        <li v-for="step in steps" :key="step.id" class="rounded-xl border border-line bg-panel px-3 py-3">
          <div class="flex items-center justify-between gap-3 text-[11px]">
            <span class="font-medium" :class="step.kind === 'decision' ? 'text-accent' : 'text-mute'">{{ step.label }}</span>
            <span class="font-mono text-mute">{{ formatTime(step.at) }}</span>
          </div>
          <p class="mt-1.5 whitespace-pre-wrap break-words text-sm leading-6">{{ step.body }}</p>
        </li>
        <li v-if="working" class="rounded-xl border border-accent/40 bg-panel px-3 py-3 text-sm text-mute">
          Thinking…
        </li>
      </ol>
    </div>
  </section>
</template>
