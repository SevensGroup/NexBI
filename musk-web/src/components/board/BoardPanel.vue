<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useAgentRunStore } from '@/stores/agentRun'
import BoardView from './BoardView.vue'
import { boardCodeFromUrl, type BoardView as Board } from './board'

const props = defineProps<{ url: string }>()

const run = useAgentRunStore()
const board = ref<Board | null>(null)
const error = ref('')
const loading = ref(false)
const code = computed(() => boardCodeFromUrl(props.url))
const pageHref = computed(() => (code.value ? `/bi/${code.value}` : props.url))
const waiting = computed(() => {
  if (code.value) return false
  if (!run.state.runId) return false
  const stopped = new Set<string>(['COMPLETED', 'FAILED', 'ABORTED', 'WAITING_APPROVAL'])
  return !stopped.has(run.state.status)
})

watch(
  () => props.url,
  (next) => {
    void load(next)
  },
  { immediate: true },
)

async function load(url: string) {
  const nextCode = boardCodeFromUrl(url)
  board.value = null
  error.value = ''
  if (!nextCode) {
    loading.value = false
    return
  }
  loading.value = true
  try {
    const res = await fetch(`/api/v1/boards/${encodeURIComponent(nextCode)}`)
    const payload = (await res.json()) as Board & { error?: string }
    if (!res.ok) {
      error.value = payload.error || 'Could not open this board'
      return
    }
    board.value = payload
  } catch {
    error.value = 'Could not open this board'
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <section class="flex h-full min-h-0 flex-col">
    <div class="flex items-center justify-between gap-3 border-b border-line px-4 py-3">
      <div>
        <h2 class="text-sm font-medium">Board</h2>
        <p class="mt-0.5 font-mono text-[11px] text-mute">{{ code ? `/bi/${code}` : 'Waiting for a published board' }}</p>
      </div>
      <a
        v-if="code"
        class="shrink-0 rounded-lg border border-line px-3 py-1.5 text-xs text-accent hover:bg-raised"
        :href="pageHref"
        target="_blank"
        rel="noreferrer"
      >
        Open in new page
      </a>
    </div>
    <div class="min-h-0 flex-1 overflow-auto p-4">
      <div v-if="waiting" class="space-y-3">
        <div class="flex items-center gap-3 text-sm text-mute">
          <span class="spin-ring shrink-0" />
          <span>Building the board</span>
        </div>
        <div class="grid grid-cols-3 gap-3">
          <div v-for="n in 3" :key="n" class="rounded-xl border border-line bg-panel p-4">
            <div class="loading-bar w-16" />
            <div class="loading-bar mt-3 w-24" />
          </div>
        </div>
        <div class="space-y-2 rounded-xl border border-line bg-panel p-4">
          <div v-for="n in 4" :key="n" class="loading-bar" :class="n % 2 ? 'w-full' : 'w-4/5'" />
        </div>
      </div>
      <p v-else-if="!code" class="rounded-xl border border-dashed border-line px-4 py-10 text-center text-sm text-mute">
        The board shows up here after the agent publishes it. You can also open that page on its own.
      </p>
      <p v-else-if="loading" class="flex items-center gap-3 text-sm text-mute">
        <span class="spin-ring shrink-0" />
        Opening the board
      </p>
      <p v-else-if="error" class="rounded-xl border border-line bg-panel px-4 py-8 text-center text-sm text-rose-200">{{ error }}</p>
      <BoardView v-else-if="board" :board="board" compact />
    </div>
  </section>
</template>
