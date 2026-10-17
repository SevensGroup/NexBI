<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import BoardView from '@/components/board/BoardView.vue'
import type { BoardView as Board } from '@/components/board/board'

const route = useRoute()
const router = useRouter()
const board = ref<Board | null>(null)
const error = ref('')
const loading = ref(true)

onMounted(load)

async function load() {
  const code = String(route.params.code || '')
  loading.value = true
  error.value = ''
  try {
    const res = await fetch(`/api/v1/boards/${encodeURIComponent(code)}`)
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
  <div class="min-h-full bg-canvas text-ink">
    <header class="flex h-14 items-center gap-4 border-b border-line px-4 sm:px-6">
      <button
        class="shrink-0 rounded-lg border border-line px-3 py-1.5 text-xs text-mute hover:bg-raised hover:text-ink"
        type="button"
        @click="router.push('/dashboard')"
      >
        return home
      </button>
      <p class="min-w-0 flex-1 truncate text-sm font-semibold">{{ board?.name || 'Board' }}</p>
      <p v-if="board" class="shrink-0 font-mono text-xs text-mute">/bi/{{ board.code }}</p>
    </header>
    <main class="mx-auto max-w-6xl px-6 py-8">
      <p v-if="loading" class="text-sm text-mute">Opening the board…</p>
      <p v-else-if="error" class="rounded-xl border border-line bg-panel px-4 py-8 text-center text-sm text-rose-200">{{ error }}</p>
      <BoardView v-else-if="board" :board="board" />
    </main>
  </div>
</template>
