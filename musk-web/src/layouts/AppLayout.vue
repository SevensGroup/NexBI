<script setup lang="ts">
import { computed } from 'vue'
import { useRoute, useRouter, RouterView } from 'vue-router'
import { logout } from '@/auth/session'
import ConnectionStatus from '@/components/common/ConnectionStatus.vue'
import { useAgentRunStore } from '@/stores/agentRun'
import { runFailureBanner } from '@/utils/runDisplay'
import { useBackendHealth } from '@/composables/useBackendHealth'

const route = useRoute()
const router = useRouter()
const runStore = useAgentRunStore()
useBackendHealth()

const nav = [
  { to: '/dashboard', label: 'Dashboard' },
  { to: '/agents', label: 'Agents' },
  { to: '/runs', label: 'Runs' },
  { to: '/tasks', label: 'Tasks' },
  { to: '/tools', label: 'Tools' },
  { to: '/memory', label: 'Memory' },
  { to: '/semantic', label: 'Semantic' },
  { to: '/approvals', label: 'Approvals' },
  { to: '/settings', label: 'Settings' },
]

const active = computed(() => route.path)
const needsApproval = computed(() => runStore.state.status === 'WAITING_APPROVAL')
const failMessage = computed(() => runFailureBanner(runStore.state.status, runStore.state.error))
const onRunDetail = computed(() => route.name === 'run-detail')

function signOut(): void {
  logout()
  void router.push('/login')
}

function isActive(to: string): boolean {
  if (to === '/agents') return active.value.startsWith('/agents')
  if (to === '/runs') return active.value.startsWith('/runs')
  return active.value === to || active.value.startsWith(`${to}/`)
}
</script>

<template>
  <div class="flex h-full min-h-0 flex-col bg-canvas text-ink">
    <header class="flex h-12 shrink-0 items-center gap-3 border-b border-line px-3 sm:px-4">
      <button class="shrink-0 font-semibold tracking-tight" type="button" @click="router.push('/dashboard')">
        MUSK
        <span class="ml-2 hidden text-xs font-normal text-mute sm:inline">Agent Console</span>
      </button>
      <nav class="min-w-0 flex-1 overflow-x-auto text-sm [scrollbar-width:none] [&::-webkit-scrollbar]:hidden">
        <div class="mx-auto flex w-max min-w-full items-center justify-center gap-1">
          <RouterLink
            v-for="item in nav"
            :key="item.to"
            :to="item.to"
            class="shrink-0 rounded px-2.5 py-1 text-mute hover:bg-raised hover:text-ink"
            :class="isActive(item.to) ? 'bg-raised text-ink' : ''"
          >
            {{ item.label }}
          </RouterLink>
        </div>
      </nav>
      <div class="flex shrink-0 items-center gap-3">
        <ConnectionStatus />
        <button class="shrink-0 text-xs text-mute hover:text-ink" type="button" @click="signOut">Sign out</button>
      </div>
    </header>
    <div
      v-if="failMessage && !onRunDetail"
      class="flex shrink-0 items-center justify-between gap-3 border-b border-rose-900/70 bg-rose-950/50 px-4 py-1.5 text-sm text-rose-200"
    >
      <span class="min-w-0 flex-1 whitespace-pre-wrap break-words">
        <span class="font-medium tracking-wide">FAILED</span>
        <span class="ml-2">{{ failMessage }}</span>
      </span>
      <button
        v-if="runStore.state.runId"
        class="shrink-0 text-xs underline"
        type="button"
        @click="router.push(`/runs/${runStore.state.runId}`)"
      >
        Open run
      </button>
    </div>
    <div
      v-if="needsApproval"
      class="flex shrink-0 items-center justify-between border-b border-amber-900/60 bg-amber-950/40 px-4 py-1.5 text-sm text-amber-300"
    >
      <span class="font-medium tracking-wide">NEEDS APPROVAL</span>
      <button class="text-xs underline" type="button" @click="router.push('/approvals')">Open Approvals</button>
    </div>
    <main class="min-h-0 flex-1 overflow-hidden">
      <RouterView />
    </main>
  </div>
</template>
