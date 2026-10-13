<script setup lang="ts">
import { computed } from 'vue'
import { clamp01 } from '@/utils/format'

const props = defineProps<{ value: number }>()
const pct = computed(() => Math.round(clamp01(props.value) * 100))
</script>

<template>
  <div class="flex items-center gap-2">
    <div class="h-1.5 flex-1 overflow-hidden rounded bg-raised">
      <div class="h-full bg-accent" :style="{ width: `${pct}%` }" />
    </div>
    <span class="w-10 text-right font-mono text-[11px] text-mute">{{ pct }}%</span>
  </div>
</template>
