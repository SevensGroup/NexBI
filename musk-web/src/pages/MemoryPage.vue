<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import MemoryCard from '@/components/memory/MemoryCard.vue'
import MemorySearch from '@/components/memory/MemorySearch.vue'
import MemoryTimeline from '@/components/memory/MemoryTimeline.vue'
import MemoryDetailDrawer from '@/components/memory/MemoryDetailDrawer.vue'
import PageState from '@/components/common/PageState.vue'
import { listMemories } from '@/services/api/agent'
import type { MemoryItem } from '@/types'

const items = ref<MemoryItem[]>([])
const loading = ref(true)
const q = ref('')
const type = ref('')
const agent = ref('')
const open = ref(false)
const current = ref<MemoryItem | null>(null)

const filtered = computed(() =>
  items.value.filter((m) => {
    if (type.value && m.type !== type.value) return false
    if (agent.value && m.agent !== agent.value) return false
    if (q.value && !`${m.title} ${m.content}`.toLowerCase().includes(q.value.toLowerCase())) return false
    return true
  }),
)

onMounted(async () => {
  items.value = await listMemories()
  loading.value = false
})

function openMem(id: string): void {
  current.value = items.value.find((m) => m.id === id) ?? null
  open.value = true
}
</script>

<template>
  <div class="grid h-full min-h-0 grid-cols-12">
    <div class="col-span-8 overflow-auto px-6 py-8">
      <header>
        <h1 class="text-xl font-semibold tracking-tight">Memory</h1>
        <p class="mt-1 text-sm text-mute">Short-term notes, task memory, and long-term knowledge.</p>
      </header>
      <section class="mt-8">
        <div class="mb-3">
          <h2 class="text-sm font-medium">Search</h2>
          <p class="mt-0.5 text-xs leading-5 text-mute">Filter by text, type, or agent.</p>
        </div>
        <div class="rounded-xl border border-line bg-panel p-4">
          <MemorySearch v-model:q="q" v-model:type="type" v-model:agent="agent" />
        </div>
      </section>
      <section class="mt-8">
        <div class="mb-3">
          <h2 class="text-sm font-medium">Items</h2>
          <p class="mt-0.5 text-xs leading-5 text-mute">Open a card for the full note.</p>
        </div>
        <PageState :loading="loading" :empty="!filtered.length" empty-text="No matching memories.">
          <div class="grid gap-3 md:grid-cols-2">
            <MemoryCard v-for="m in filtered" :key="m.id" :item="m" @open="openMem" />
          </div>
        </PageState>
      </section>
    </div>
    <aside class="col-span-4 overflow-auto border-l border-line px-5 py-8">
      <h2 class="text-sm font-medium">Timeline</h2>
      <p class="mb-4 mt-0.5 text-xs text-mute">Same items, in time order.</p>
      <MemoryTimeline :items="filtered" />
    </aside>
    <MemoryDetailDrawer v-model="open" :item="current" />
  </div>
</template>
