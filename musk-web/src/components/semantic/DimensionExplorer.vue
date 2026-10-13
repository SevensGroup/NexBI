<script setup lang="ts">
import type { SemanticDimension } from '@/types'

defineProps<{ dimensions: SemanticDimension[] }>()
const emit = defineEmits<{ select: [key: string] }>()
</script>

<template>
  <div class="grid gap-2 md:grid-cols-2">
    <button
      v-for="d in dimensions"
      :key="d.key"
      type="button"
      class="rounded border border-line bg-panel p-3 text-left text-sm hover:border-accent/40"
      @click="emit('select', d.key)"
    >
      <div class="font-medium">{{ d.name }}</div>
      <div class="text-xs text-mute">{{ d.description }}</div>
      <div class="mt-1 text-[11px] text-mute">field {{ d.field }} · {{ d.type }}</div>
    </button>
  </div>
</template>
