<script setup lang="ts">
import { computed, ref } from 'vue'
import type { TimelineItem } from '@/types'
import { formatTime } from '@/utils/format'
import { useAgentRunStore } from '@/stores/agentRun'
import { useUiStore } from '@/stores/ui'

const props = defineProps<{ item: TimelineItem }>()
const run = useAgentRunStore()
const ui = useUiStore()
const active = computed(() => run.state.selectedEventId === props.item.eventId)
const failed = computed(() => props.item.kind === 'error' || props.item.title === 'Failed')
const open = ref(true)
const thinking = computed(() => props.item.kind === 'thinking')
const live = computed(() => {
  if (!thinking.value) return false
  const status = run.state.status
  if (status === 'COMPLETED' || status === 'FAILED') return false
  return run.state.timeline.at(-1)?.id === props.item.id
})
const seconds = computed(() => {
  const ms = props.item.durationMs ?? 0
  if (ms < 800) return 0
  return Math.max(1, Math.round(ms / 1000))
})

function select(): void {
  run.selectEvent(props.item.eventId)
  if (props.item.taskId) run.selectTask(props.item.taskId)
  ui.timelineDrawerOpen = true
  if (props.item.kind === 'decision') ui.contextTab = 'decision'
  if (props.item.kind === 'report') ui.contextTab = 'report'
  if (props.item.kind === 'error') ui.contextTab = 'decision'
}
</script>

<template>
  <div
    v-if="thinking"
    class="timeline-in border-l-2 border-accent/50 px-3 py-2"
    data-testid="timeline-row"
  >
    <button type="button" class="flex w-full items-center gap-2 text-left text-xs text-mute" @click="open = !open">
      <span class="w-14 shrink-0 font-mono">{{ formatTime(item.timestamp) }}</span>
      <span class="w-3 shrink-0 text-[10px] text-accent" :class="open ? 'rotate-90' : ''">▸</span>
      <span v-if="live" class="inline-flex items-center gap-2 text-ink">
        Thinking
        <span class="inline-flex items-center gap-1">
          <span class="think-dot" />
          <span class="think-dot" />
          <span class="think-dot" />
        </span>
      </span>
      <span v-else>Thought<span v-if="seconds"> for {{ seconds }}s</span></span>
    </button>
    <p
      class="ml-[4.75rem] mt-1 whitespace-pre-wrap break-words text-xs leading-5 text-mute"
      :class="[open ? '' : 'line-clamp-1', live ? 'thinking-caret' : '']"
    >
      {{ item.subtitle }}
    </p>
    <div v-if="live && open" class="ml-[4.75rem] mt-2 space-y-1.5">
      <div class="skeleton-line w-4/5" />
      <div class="skeleton-line w-3/5" />
    </div>
  </div>
  <button
    v-else
    type="button"
    class="timeline-in flex w-full gap-3 border-l-2 px-3 py-2 text-left text-xs"
    data-testid="timeline-row"
    :class="[
      active ? 'border-accent bg-raised' : 'border-line hover:bg-raised/60',
      failed ? 'border-rose-500/80' : '',
    ]"
    @click="select"
  >
    <span class="w-14 shrink-0 font-mono text-mute">{{ formatTime(item.timestamp) }}</span>
    <span class="min-w-0 flex-1">
      <div class="flex items-center justify-between gap-2">
        <span class="font-medium" :class="failed ? 'text-rose-300' : ''">{{ item.title }}</span>
        <span class="shrink-0 text-[10px] uppercase tracking-wide text-accent">Detail</span>
      </div>
      <div v-if="item.subtitle" class="mt-0.5 line-clamp-3 whitespace-pre-wrap break-words text-mute">{{ item.subtitle }}</div>
    </span>
  </button>
</template>
