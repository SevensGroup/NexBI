<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import PageState from '@/components/common/PageState.vue'
import { RUN_PAGE_SIZE, listRuns } from '@/services/api/agent'
import type { RunSummary } from '@/types/run'
import { formatDateTime, shortId } from '@/utils/format'

const route = useRoute()
const router = useRouter()
const rows = ref<RunSummary[]>([])
const total = ref(0)
const loading = ref(true)
const error = ref('')

const page = computed(() => {
  const n = Number(route.query.page)
  return Number.isInteger(n) && n > 0 ? n : 1
})
const pageCount = computed(() => Math.max(1, Math.ceil(total.value / RUN_PAGE_SIZE)))
const rangeLabel = computed(() => {
  if (total.value === 0 || rows.value.length === 0) return `0 of ${total.value}`
  const start = (page.value - 1) * RUN_PAGE_SIZE + 1
  const end = start + rows.value.length - 1
  return `${start}–${end} of ${total.value}`
})

watch(page, load, { immediate: true })

async function load(next: number): Promise<void> {
  loading.value = true
  error.value = ''
  try {
    const result = await listRuns(next, RUN_PAGE_SIZE)
    const pages = Math.max(1, Math.ceil(result.total / RUN_PAGE_SIZE))
    if (result.total > 0 && next > pages) {
      go(pages)
      return
    }
    rows.value = result.runs
    total.value = result.total
  } catch (err) {
    rows.value = []
    error.value = err instanceof Error ? err.message : 'Could not load runs'
  } finally {
    loading.value = false
  }
}

function go(next: number): void {
  const query = { ...route.query }
  if (next <= 1) delete query.page
  else query.page = String(next)
  void router.push({ query })
}
</script>

<template>
  <div class="h-full overflow-auto">
    <div class="mx-auto flex max-w-5xl flex-col gap-8 px-6 py-8">
      <header class="flex items-end justify-between gap-4">
        <div>
          <h1 class="text-xl font-semibold tracking-tight">Runs</h1>
          <p class="mt-1 text-sm text-mute">Past runs on this server. Open a row for the board and the task plan.</p>
        </div>
        <button
          class="h-10 shrink-0 rounded-xl bg-accent px-5 text-sm font-medium text-white"
          type="button"
          @click="router.push('/dashboard')"
        >
          New run
        </button>
      </header>
      <PageState :loading="loading" :error="error" :empty="!loading && !error && total === 0" empty-text="No runs yet. Submit a goal from Dashboard.">
        <div class="overflow-hidden rounded-xl border border-line bg-panel">
          <table class="w-full text-left text-sm">
            <thead class="text-xs text-mute">
              <tr>
                <th class="px-4 py-3 font-medium">Run</th>
                <th class="px-4 py-3 font-medium">Goal</th>
                <th class="px-4 py-3 font-medium">Status</th>
                <th class="px-4 py-3 font-medium">Started</th>
                <th class="px-4 py-3 font-medium">Steps</th>
              </tr>
            </thead>
            <tbody>
              <tr v-if="!rows.length">
                <td class="px-4 py-6 text-sm text-mute" colspan="5">No runs on this page.</td>
              </tr>
              <tr
                v-for="r in rows"
                :key="r.id"
                class="cursor-pointer border-t border-line hover:bg-raised"
                @click="router.push(`/runs/${r.id}`)"
              >
                <td class="px-4 py-3.5 font-mono text-xs">{{ shortId(r.id) }}</td>
                <td class="max-w-lg truncate px-4 py-3.5">{{ r.goal }}</td>
                <td class="px-4 py-3.5">{{ r.status }}</td>
                <td class="px-4 py-3.5 text-mute">{{ formatDateTime(r.startedAt) }}</td>
                <td class="px-4 py-3.5">{{ r.steps }}</td>
              </tr>
            </tbody>
          </table>
          <div class="flex items-center justify-between gap-3 border-t border-line px-4 py-3 text-sm">
            <p class="text-mute">{{ rangeLabel }}</p>
            <div class="flex items-center gap-2">
              <button
                class="h-8 rounded-lg border border-line px-3 text-xs disabled:opacity-40"
                type="button"
                :disabled="page <= 1"
                @click="go(page - 1)"
              >
                Previous
              </button>
              <span class="min-w-16 text-center text-xs text-mute">{{ page }} / {{ pageCount }}</span>
              <button
                class="h-8 rounded-lg border border-line px-3 text-xs disabled:opacity-40"
                type="button"
                :disabled="page >= pageCount"
                @click="go(page + 1)"
              >
                Next
              </button>
            </div>
          </div>
        </div>
      </PageState>
    </div>
  </div>
</template>
