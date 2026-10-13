<script setup lang="ts">
import type { MemoryItem } from '@/types'
import { formatTime } from '@/utils/format'

defineProps<{ items: MemoryItem[] }>()
</script>

<template>
  <ol class="space-y-2 text-xs">
    <li v-for="m in items" :key="m.id" class="flex gap-2">
      <span class="w-14 font-mono text-mute">{{ formatTime(m.timestamp) }}</span>
      <span>{{ m.title }}</span>
    </li>
  </ol>
</template>
