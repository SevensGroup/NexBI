<script setup lang="ts">
import { computed } from 'vue'
import { useConnectionStore } from '@/stores/connection'
import { isMockMode } from '@/services/api/client'

const conn = useConnectionStore()
const apiBase = import.meta.env.VITE_API_BASE_URL || 'http://127.0.0.1:8080'
const mock = isMockMode()
const connected = computed(() => conn.status === 'Connected')
</script>

<template>
  <div class="h-full overflow-auto">
    <div class="mx-auto flex max-w-3xl flex-col gap-8 px-6 py-8">
      <header>
        <h1 class="text-xl font-semibold tracking-tight">Settings</h1>
        <p class="mt-1 text-sm text-mute">Stored in this build. Restart Vite after changing .env.</p>
      </header>

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
</template>
