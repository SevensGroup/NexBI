<script setup lang="ts">
import type { SemanticMetric } from '@/types'

defineProps<{ metrics: SemanticMetric[] }>()
const emit = defineEmits<{ select: [key: string] }>()
</script>

<template>
  <div class="grid gap-3 md:grid-cols-2">
    <button
      v-for="m in metrics"
      :key="m.key"
      type="button"
      class="rounded-xl border border-line bg-panel p-4 text-left text-sm hover:border-accent/40"
      @click="emit('select', m.key)"
    >
      <div class="font-medium">{{ m.name }}</div>
      <div class="text-xs text-mute">{{ m.description }}</div>
      <div class="mt-1 font-mono text-[11px] text-accent">{{ m.formula || m.field }}</div>
    </button>
  </div>
</template>
