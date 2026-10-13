<script setup lang="ts">
import { computed } from 'vue'
import { useAgentRunStore } from '@/stores/agentRun'
import TimelineItemRow from './TimelineItem.vue'
import TimelineDetailDrawer from './TimelineDetailDrawer.vue'
import PageState from '@/components/common/PageState.vue'
import SectionHeader from '@/components/common/SectionHeader.vue'

const run = useAgentRunStore()
const items = computed(() => run.state.timeline)
</script>

<template>
  <div class="flex h-full min-h-0 flex-col">
    <div class="border-b border-line px-3 py-2">
      <SectionHeader title="Execution Timeline" :hint="`${items.length} events`" />
    </div>
    <div class="min-h-0 flex-1 overflow-auto">
      <PageState :empty="items.length === 0" empty-text="No execution yet. Submit a goal from Dashboard or the input below.">
        <TimelineItemRow v-for="item in items" :key="item.id" :item="item" />
      </PageState>
    </div>
    <TimelineDetailDrawer />
  </div>
</template>
