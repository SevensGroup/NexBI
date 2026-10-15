<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import { useAgentRunStore } from '@/stores/agentRun'
import TimelineItemRow from './TimelineItem.vue'
import TimelineDetailDrawer from './TimelineDetailDrawer.vue'
import PageState from '@/components/common/PageState.vue'
import SectionHeader from '@/components/common/SectionHeader.vue'

const run = useAgentRunStore()
const items = computed(() => run.state.timeline)
const scroller = ref<HTMLElement | null>(null)
const working = computed(() => {
  if (!run.state.runId) return false
  const status = run.state.status
  return status !== 'COMPLETED' && status !== 'FAILED' && status !== 'WAITING_APPROVAL'
})
const showWaitingRow = computed(() => working.value && items.value.at(-1)?.kind !== 'thinking')

watch(
  () => items.value.length,
  async () => {
    await nextTick()
    const el = scroller.value
    if (!el || !working.value) return
    el.scrollTo({ top: el.scrollHeight, behavior: 'smooth' })
  },
)
</script>

<template>
  <div class="flex h-full min-h-0 flex-col">
    <div class="border-b border-line px-3 py-2">
      <SectionHeader title="Execution Timeline" :hint="`${items.length} events`" />
    </div>
    <div ref="scroller" class="min-h-0 flex-1 overflow-auto">
      <PageState :empty="items.length === 0 && !working" empty-text="No execution yet. Submit a goal from Dashboard or the input below.">
        <TimelineItemRow v-for="item in items" :key="item.id" :item="item" />
        <div v-if="showWaitingRow" class="timeline-in border-l-2 border-accent/50 px-3 py-2" data-testid="timeline-waiting">
          <div class="flex items-center gap-2 text-xs text-ink">
            <span class="w-14 shrink-0 font-mono text-mute">now</span>
            <span>Thinking</span>
            <span class="inline-flex items-center gap-1">
              <span class="think-dot" />
              <span class="think-dot" />
              <span class="think-dot" />
            </span>
          </div>
          <div class="ml-[4.75rem] mt-2 space-y-1.5">
            <div class="skeleton-line w-4/5" />
            <div class="skeleton-line w-2/5" />
          </div>
        </div>
      </PageState>
    </div>
    <TimelineDetailDrawer />
  </div>
</template>
