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
    <div class="col-span-8 overflow-auto p-6">
      <h1 class="text-lg font-medium">Memory Center</h1>
      <p class="text-sm text-mute">Short-term · Task memory · Long-term knowledge</p>
      <div class="mt-4">
        <MemorySearch v-model:q="q" v-model:type="type" v-model:agent="agent" />
      </div>
      <PageState class="mt-4" :loading="loading" :empty="!filtered.length" empty-text="No matching memories.">
        <div class="grid gap-2 md:grid-cols-2">
          <MemoryCard v-for="m in filtered" :key="m.id" :item="m" @open="openMem" />
        </div>
      </PageState>
    </div>
    <aside class="col-span-4 overflow-auto border-l border-line p-4">
      <h2 class="mb-3 text-xs uppercase text-mute">Timeline</h2>
      <MemoryTimeline :items="filtered" />
    </aside>
    <MemoryDetailDrawer v-model="open" :item="current" />
  </div>
</template>
