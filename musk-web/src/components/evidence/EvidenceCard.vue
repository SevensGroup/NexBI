<script setup lang="ts">
import type { Evidence } from '@/types'

defineProps<{ item: Evidence }>()
const emit = defineEmits<{ open: [id: string] }>()
</script>

<template>
  <button type="button" class="w-full rounded border border-line bg-raised px-2 py-1.5 text-left text-xs hover:border-accent/50" @click="emit('open', item.id)">
    <div class="font-mono text-accent">{{ item.id }}</div>
    <div class="text-mute">{{ item.description }}</div>
  </button>
</template>
