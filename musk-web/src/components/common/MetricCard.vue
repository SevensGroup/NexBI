<script setup lang="ts">
defineProps<{ label: string; value: string | number; hint?: string }>()
</script>

<template>
  <div class="rounded border border-line bg-panel p-3">
    <div class="text-[11px] uppercase tracking-wide text-mute">{{ label }}</div>
    <div class="mt-1 text-xl font-medium tabular-nums">{{ value }}</div>
    <div v-if="hint" class="mt-1 text-[11px] text-mute">{{ hint }}</div>
  </div>
</template>
