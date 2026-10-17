<script setup lang="ts">
import { computed } from 'vue'
import { formatBoardValue, type BoardView } from './board'

const props = defineProps<{ board: BoardView; compact?: boolean }>()

const columns = computed(() => {
  if (props.board.columns?.length) return props.board.columns
  return Object.keys(props.board.rows?.[0] ?? {})
})
</script>

<template>
  <article :class="compact ? '' : 'rounded-xl border border-line bg-panel p-6'">
    <h2 class="text-lg font-semibold">{{ board.name }}</h2>
    <p v-if="board.summary" class="mt-2 text-sm leading-6 text-mute">{{ board.summary }}</p>
    <div v-if="board.metrics?.length" class="mt-5 grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
      <div v-for="item in board.metrics" :key="item.label" class="rounded-xl bg-raised px-4 py-3">
        <p class="text-xs text-mute">{{ item.label }}</p>
        <p class="mt-1 text-xl font-semibold tabular-nums">{{ formatBoardValue(item.value) }}</p>
      </div>
    </div>
    <div v-if="board.rows?.length" class="mt-5 overflow-auto">
      <table class="w-full text-left text-sm">
        <thead class="text-xs text-mute">
          <tr>
            <th v-for="col in columns" :key="col" class="px-3 py-2 font-medium">{{ col }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="(row, index) in board.rows" :key="index" class="border-t border-line">
            <td v-for="col in columns" :key="col" class="whitespace-nowrap px-3 py-2.5 tabular-nums">{{ formatBoardValue(String(row[col] ?? '')) }}</td>
          </tr>
        </tbody>
      </table>
    </div>
  </article>
</template>
