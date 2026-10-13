<script setup lang="ts">
import { computed } from 'vue'
import type { AgentInfo } from '@/types'
import AgentStateBadge from './AgentStateBadge.vue'
import { MOCK_MEMORIES } from '@/mocks/catalog'

const props = defineProps<{ agent: AgentInfo | null; modelValue: boolean }>()
const emit = defineEmits<{ 'update:modelValue': [v: boolean] }>()
const open = computed({
  get: () => props.modelValue,
  set: (v) => emit('update:modelValue', v),
})
const memories = computed(() => MOCK_MEMORIES.filter((m) => !props.agent || m.agent === props.agent.name).slice(0, 5))
</script>

<template>
  <el-drawer v-model="open" :title="agent?.name || 'Agent'" size="420px" append-to-body>
    <div v-if="agent" class="space-y-4 text-sm">
      <AgentStateBadge :status="agent.status" live />
      <p class="text-mute">{{ agent.description }}</p>
      <div>
        <div class="mb-1 text-xs uppercase text-mute">Tools</div>
        <div class="flex flex-wrap gap-1">
          <span v-for="t in agent.tools" :key="t" class="rounded bg-raised px-2 py-0.5 font-mono text-[11px]">{{ t }}</span>
        </div>
      </div>
      <div>
        <div class="mb-1 text-xs uppercase text-mute">Current task</div>
        <div>{{ agent.currentTask || '—' }}</div>
      </div>
      <div>
        <div class="mb-1 text-xs uppercase text-mute">Progress / Confidence</div>
        <div>{{ Math.round(agent.progress * 100) }}% · {{ Math.round(agent.confidence * 100) }}%</div>
      </div>
      <div>
        <div class="mb-1 text-xs uppercase text-mute">Memory</div>
        <ul class="space-y-1 text-mute">
          <li v-for="m in memories" :key="m.id">{{ m.title }}</li>
        </ul>
      </div>
    </div>
  </el-drawer>
</template>
