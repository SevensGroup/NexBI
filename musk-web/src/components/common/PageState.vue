<script setup lang="ts">
defineProps<{
  loading?: boolean
  error?: string | null
  empty?: boolean
  emptyText?: string
}>()
</script>

<template>
  <div v-if="loading" class="p-6 text-sm text-mute">Loading…</div>
  <div v-else-if="error" class="p-6 text-sm text-rose-400">{{ error }}</div>
  <div v-else-if="empty" class="p-6 text-sm text-mute">{{ emptyText || 'No data' }}</div>
  <slot v-else />
</template>
