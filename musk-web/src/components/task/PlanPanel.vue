<script setup lang="ts">
import type { Plan } from '@/types'
import TaskStatusBadge from './TaskStatusBadge.vue'
import { useAgentRunStore } from '@/stores/agentRun'
import { useUiStore } from '@/stores/ui'

defineProps<{ plan: Plan | null }>()
const run = useAgentRunStore()
const ui = useUiStore()

function open(id: string): void {
  run.selectTask(id)
  ui.taskDrawerId = id
}
</script>

<template>
  <div v-if="plan" class="space-y-1">
    <div class="text-xs text-mute">{{ plan.goal }}</div>
    <button
      v-for="task in plan.tasks"
      :key="task.id"
      type="button"
      class="flex w-full items-center justify-between rounded px-2 py-1.5 text-left text-xs hover:bg-raised"
      :class="run.state.selectedTaskId === task.id ? 'bg-raised' : ''"
      @click="open(task.id)"
    >
      <span>{{ task.title }}</span>
      <TaskStatusBadge :status="task.status" />
    </button>
  </div>
  <div v-else class="text-xs text-mute">No plan yet.</div>
</template>
