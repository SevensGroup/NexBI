<script setup lang="ts">
import { Handle, Position } from '@vue-flow/core'
import type { Task } from '@/types'
import TaskStatusBadge from './TaskStatusBadge.vue'

defineProps<{ data: { task: Task; active?: boolean } }>()
</script>

<template>
  <div
    class="w-[200px] rounded border bg-panel px-2.5 py-2"
    :class="data.active ? 'border-accent' : 'border-line'"
  >
    <Handle type="target" :position="Position.Left" />
    <div class="flex items-center justify-between gap-2">
      <span class="text-xs font-medium">{{ data.task.title }}</span>
      <TaskStatusBadge :status="data.task.status" />
    </div>
    <div class="mt-1 truncate text-[11px] text-mute">{{ data.task.description }}</div>
    <Handle type="source" :position="Position.Right" />
  </div>
</template>
