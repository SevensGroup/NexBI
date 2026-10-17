<script setup lang="ts">
import { computed } from 'vue'
import { use } from 'echarts/core'
import { CanvasRenderer } from 'echarts/renderers'
import { BarChart, PieChart } from 'echarts/charts'
import { GridComponent, LegendComponent, TooltipComponent } from 'echarts/components'
import VChart from 'vue-echarts'
import { channelSlices, formatBoardValue, type BoardView } from './board'

use([CanvasRenderer, PieChart, BarChart, GridComponent, TooltipComponent, LegendComponent])

const props = defineProps<{ board: BoardView; compact?: boolean }>()

const palette = ['#5b8def', '#3dbe8c', '#e2b15a', '#d16b6b', '#9b8afb', '#5ec8d8']
const axis = { color: '#8b929e' }
const split = { lineStyle: { color: '#2a303b' } }

const columns = computed(() => {
  if (props.board.columns?.length) return props.board.columns
  return Object.keys(props.board.rows?.[0] ?? {})
})

const slices = computed(() => channelSlices(columns.value, props.board.rows ?? []))
const hasRevenue = computed(() => slices.value.some((slice) => slice.revenue > 0))

const pie = computed(() => ({
  backgroundColor: 'transparent',
  color: palette,
  tooltip: { trigger: 'item', valueFormatter: (value: number) => formatBoardValue(String(value)) },
  legend: { bottom: 0, textStyle: axis },
  series: [
    {
      type: 'pie',
      radius: ['46%', '70%'],
      center: ['50%', '44%'],
      label: { color: '#c5cad3', formatter: '{b}\n{d}%' },
      data: slices.value.map((slice) => ({ name: slice.name, value: slice.spend })),
    },
  ],
}))

const bar = computed(() => ({
  backgroundColor: 'transparent',
  color: palette,
  tooltip: { trigger: 'axis', valueFormatter: (value: number) => formatBoardValue(String(value)) },
  legend: { top: 0, textStyle: axis },
  grid: { left: 64, right: 12, top: 32, bottom: 28 },
  xAxis: {
    type: 'category',
    data: slices.value.map((slice) => slice.name),
    axisLine: split,
    axisLabel: axis,
  },
  yAxis: { type: 'value', splitLine: split, axisLabel: axis },
  series: [
    {
      name: 'Spend',
      type: 'bar',
      data: slices.value.map((slice) => slice.spend),
      itemStyle: { borderRadius: [4, 4, 0, 0] },
    },
    ...(hasRevenue.value
      ? [
          {
            name: 'Revenue',
            type: 'bar',
            data: slices.value.map((slice) => slice.revenue),
            itemStyle: { borderRadius: [4, 4, 0, 0] },
          },
        ]
      : []),
  ],
}))
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
    <div v-if="slices.length" class="mt-5 grid gap-3" :class="compact ? '' : 'lg:grid-cols-2'">
      <div class="rounded-xl bg-raised px-3 pb-2 pt-3">
        <p class="text-xs text-mute">Spend share by channel</p>
        <VChart class="h-64" :option="pie" autoresize />
      </div>
      <div class="rounded-xl bg-raised px-3 pb-2 pt-3">
        <p class="text-xs text-mute">Spend and revenue by channel</p>
        <VChart class="h-64" :option="bar" autoresize />
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
