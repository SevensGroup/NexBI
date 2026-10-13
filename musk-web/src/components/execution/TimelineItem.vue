<script setup lang="ts">
import { computed } from 'vue'
import type { TimelineItem } from '@/types'
import { formatTime } from '@/utils/format'
import { useAgentRunStore } from '@/stores/agentRun'
import { useUiStore } from '@/stores/ui'

const props = defineProps<{ item: TimelineItem }>()
const run = useAgentRunStore()
const ui = useUiStore()
const active = computed(() => run.state.selectedEventId === props.item.eventId)
const failed = computed(() => props.item.kind === 'error' || props.item.title === 'Failed')

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
  <button
    type="button"
    class="flex w-full gap-3 border-l-2 px-3 py-2 text-left text-xs"
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
