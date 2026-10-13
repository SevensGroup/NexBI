<script setup lang="ts">
import { computed } from 'vue'
import { useAgentRunStore } from '@/stores/agentRun'
import { useUiStore } from '@/stores/ui'
import TaskStatusBadge from './TaskStatusBadge.vue'

const run = useAgentRunStore()
const ui = useUiStore()
const open = computed({
  get: () => Boolean(ui.taskDrawerId),
  set: (v) => {
    if (!v) ui.taskDrawerId = null
  },
})
const task = computed(() => run.state.plan?.tasks.find((t) => t.id === ui.taskDrawerId) ?? null)
</script>

<template>
  <el-drawer v-model="open" :title="task?.title || 'Task'" size="400px" append-to-body>
    <div v-if="task" class="space-y-3 text-sm">
      <TaskStatusBadge :status="task.status" />
      <p>{{ task.description }}</p>
      <div class="text-xs text-mute">Priority {{ task.priority }} · Agent {{ task.agent || '—' }}</div>
      <div class="text-xs">Dependencies: {{ task.dependencies.join(', ') || 'none' }}</div>
    </div>
  </el-drawer>
</template>
