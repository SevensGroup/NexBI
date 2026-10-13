<script setup lang="ts">
import { useUiStore } from '@/stores/ui'
import { useConnectionStore } from '@/stores/connection'
import { isMockMode } from '@/services/api/client'

const ui = useUiStore()
const conn = useConnectionStore()
const apiBase = import.meta.env.VITE_API_BASE_URL || 'http://127.0.0.1:8080'
const runHint = 'POST /api/v1/agent/run { "message" } then poll GET /api/v1/runs/:id'
</script>

<template>
  <div class="h-full overflow-auto p-6">
    <h1 class="text-lg font-medium">Settings</h1>
    <dl class="mt-6 max-w-xl space-y-3 text-sm">
      <div class="flex justify-between border-b border-line py-2">
        <dt class="text-mute">API base</dt>
        <dd class="font-mono text-xs">{{ apiBase }}</dd>
      </div>
      <div class="flex justify-between border-b border-line py-2">
        <dt class="text-mute">Dev proxy</dt>
        <dd class="font-mono text-xs">/api → {{ apiBase }}</dd>
      </div>
      <div class="flex justify-between border-b border-line py-2">
        <dt class="text-mute">Mock mode</dt>
        <dd>{{ isMockMode() ? 'on' : 'off' }} (VITE_USE_MOCK)</dd>
      </div>
      <div class="flex justify-between border-b border-line py-2">
        <dt class="text-mute">Connection</dt>
        <dd>{{ conn.status }}</dd>
      </div>
      <div class="flex justify-between py-2">
        <dt class="text-mute">Run / request id</dt>
        <dd class="font-mono text-xs">{{ conn.lastEventId || '—' }}</dd>
      </div>
    </dl>
    <p class="mt-6 max-w-xl text-xs text-mute">
      Live mode (default): {{ runHint }}. musk-back has no SSE stream. Set VITE_USE_MOCK=true only for local replay without a backend. Restart Vite after changing .env.
    </p>
    <label class="mt-4 flex items-center gap-2 text-sm">
      <input v-model="ui.useMock" type="checkbox" disabled />
      Mock is off unless VITE_USE_MOCK=true
    </label>
  </div>
</template>
