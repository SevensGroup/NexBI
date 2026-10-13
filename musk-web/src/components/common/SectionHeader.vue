<script setup lang="ts">
defineProps<{ title: string; hint?: string }>()
</script>

<template>
  <div class="mb-2 flex items-baseline justify-between">
    <h3 class="text-xs font-medium uppercase tracking-wide text-mute">{{ title }}</h3>
    <span v-if="hint" class="text-[11px] text-mute">{{ hint }}</span>
  </div>
</template>
