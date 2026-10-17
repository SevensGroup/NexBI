<script setup lang="ts">
import type { MemoryItem } from '@/types'
import MemoryTypeBadge from './MemoryTypeBadge.vue'
import { formatDateTime } from '@/utils/format'

defineProps<{ item: MemoryItem }>()
const emit = defineEmits<{ open: [id: string] }>()
</script>

<template>
  <button type="button" class="w-full rounded-xl border border-line bg-panel p-4 text-left text-sm hover:border-accent/40" @click="emit('open', item.id)">
    <div class="flex items-center justify-between gap-2">
      <span class="font-medium">{{ item.title }}</span>
      <MemoryTypeBadge :type="item.type" />
    </div>
    <p class="mt-1 line-clamp-2 text-xs text-mute">{{ item.content }}</p>
    <div class="mt-2 text-[11px] text-mute">{{ item.agent }} · {{ formatDateTime(item.timestamp) }}</div>
  </button>
</template>
