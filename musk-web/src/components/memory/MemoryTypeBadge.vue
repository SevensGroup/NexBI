<script setup lang="ts">
import type { MemoryType } from '@/types'

const color: Record<MemoryType, string> = {
  SHORT_TERM: 'text-sky-300',
  TASK_MEMORY: 'text-violet-300',
  LONG_TERM: 'text-emerald-300',
}

defineProps<{ type: MemoryType }>()
</script>

<template>
  <span class="text-[11px] uppercase" :class="color[type]">{{ type.replace('_', ' ') }}</span>
</template>
