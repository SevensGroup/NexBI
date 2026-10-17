<script setup lang="ts">
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import MemoryPage from '@/pages/MemoryPage.vue'
import SemanticPage from '@/pages/SemanticPage.vue'
import ToolsPage from '@/pages/ToolsPage.vue'
import { useConnectionStore } from '@/stores/connection'
import { isMockMode } from '@/services/api/client'

const route = useRoute()
const router = useRouter()
const conn = useConnectionStore()
const apiBase = import.meta.env.VITE_API_BASE_URL || 'http://127.0.0.1:8080'
const mock = isMockMode()
const connected = computed(() => conn.status === 'Connected')
const sections = [
  { id: 'connection', label: 'Connection' },
  { id: 'tools', label: 'Tools' },
  { id: 'semantic', label: 'Semantic' },
  { id: 'memory', label: 'Memory' },
] as const
const section = computed(() => {
  const value = route.query.section
  return sections.some((item) => item.id === value) ? String(value) : 'connection'
})

function openSection(id: string): void {
  const query = id === 'connection' ? {} : { section: id }
  void router.replace({ path: '/settings', query })
}
</script>

<template>
  <div class="flex h-full min-h-0 flex-col">
    <div class="mx-auto flex w-full max-w-5xl shrink-0 flex-col gap-4 px-6 pb-4 pt-8">
      <header>
        <h1 class="text-xl font-semibold tracking-tight">Settings</h1>
        <p class="mt-1 text-sm text-mute">Connection, tool catalog, metric definitions, and memory.</p>
      </header>
      <div class="flex flex-wrap gap-2">
        <button
          v-for="item in sections"
          :key="item.id"
          class="rounded-full border px-3 py-1 text-xs"
          :class="section === item.id ? 'border-line bg-raised text-ink' : 'border-transparent text-mute hover:text-ink'"
          type="button"
          @click="openSection(item.id)"
        >
          {{ item.label }}
        </button>
      </div>
    </div>
    <ToolsPage v-if="section === 'tools'" embedded class="min-h-0 flex-1" />
    <SemanticPage v-else-if="section === 'semantic'" embedded class="min-h-0 flex-1" />
    <MemoryPage v-else-if="section === 'memory'" embedded class="min-h-0 flex-1" />
    <div v-else class="min-h-0 flex-1 overflow-auto">
    <div class="mx-auto flex max-w-3xl flex-col gap-8 px-6 pb-8">

      <section>
        <div class="mb-3">
          <h2 class="text-sm font-medium">Connection</h2>
          <p class="mt-0.5 text-xs leading-5 text-mute">Where the console sends requests, and whether the agent is reachable.</p>
        </div>
        <div class="overflow-hidden rounded-xl border border-line bg-panel">
          <div class="flex items-start justify-between gap-6 border-b border-line px-4 py-3.5">
            <div class="min-w-0">
              <p class="text-sm">API base</p>
              <p class="mt-0.5 text-xs text-mute">Agent server origin.</p>
            </div>
            <p class="max-w-[55%] truncate text-right font-mono text-xs">{{ apiBase }}</p>
          </div>
          <div class="flex items-start justify-between gap-6 border-b border-line px-4 py-3.5">
            <div class="min-w-0">
              <p class="text-sm">Dev proxy</p>
              <p class="mt-0.5 text-xs text-mute">Browser path forwarded to the API base.</p>
            </div>
            <p class="max-w-[55%] truncate text-right font-mono text-xs">/api → {{ apiBase }}</p>
          </div>
          <div class="flex items-start justify-between gap-6 px-4 py-3.5">
            <div class="min-w-0">
              <p class="text-sm">Status</p>
              <p class="mt-0.5 text-xs text-mute">GET /health. This console does not use an SSE stream.</p>
            </div>
            <span
              class="inline-flex items-center gap-1.5 rounded-full border border-line bg-raised px-2.5 py-1 text-xs"
              :class="connected ? 'text-emerald-300' : 'text-mute'"
            >
              <span class="status-dot" :class="connected ? 'bg-emerald-400' : 'bg-zinc-500'" />
              {{ conn.status }}
            </span>
          </div>
        </div>
      </section>

      <section>
        <div class="mb-3">
          <h2 class="text-sm font-medium">Runtime</h2>
          <p class="mt-0.5 text-xs leading-5 text-mute">How a run is started from this page.</p>
        </div>
        <div class="overflow-hidden rounded-xl border border-line bg-panel">
          <div class="flex items-start justify-between gap-6 border-b border-line px-4 py-3.5">
            <div class="min-w-0">
              <p class="text-sm">Mock mode</p>
              <p class="mt-0.5 max-w-md text-xs leading-5 text-mute">Off unless VITE_USE_MOCK=true. Use that only to replay a sample without a backend.</p>
            </div>
            <span class="rounded-full border border-line bg-raised px-2.5 py-1 font-mono text-xs text-mute">
              {{ mock ? 'on' : 'off' }}
            </span>
          </div>
          <div class="flex items-start justify-between gap-6 border-b border-line px-4 py-3.5">
            <div class="min-w-0">
              <p class="text-sm">Run</p>
              <p class="mt-0.5 text-xs text-mute">Create a run, then poll its id.</p>
            </div>
            <p class="max-w-[55%] text-right font-mono text-[11px] leading-5 text-mute">
              POST /api/v1/agent/run
              <br />
              GET /api/v1/runs/:id
            </p>
          </div>
          <div class="flex items-start justify-between gap-6 px-4 py-3.5">
            <div class="min-w-0">
              <p class="text-sm">Request id</p>
              <p class="mt-0.5 text-xs text-mute">Last id seen by the connection check.</p>
            </div>
            <p class="max-w-[55%] truncate text-right font-mono text-xs">{{ conn.lastEventId || '—' }}</p>
          </div>
        </div>
      </section>
    </div>
    </div>
  </div>
</template>
