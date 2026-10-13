<script setup lang="ts">
import { computed } from 'vue'
import type { MemoryItem } from '@/types'
import MemoryTypeBadge from './MemoryTypeBadge.vue'

const props = defineProps<{ item: MemoryItem | null; modelValue: boolean }>()
const emit = defineEmits<{ 'update:modelValue': [v: boolean] }>()
const open = computed({
  get: () => props.modelValue,
  set: (v) => emit('update:modelValue', v),
})
</script>

<template>
  <el-drawer v-model="open" :title="item?.title || 'Memory'" size="420px" append-to-body>
    <div v-if="item" class="space-y-3 text-sm">
      <MemoryTypeBadge :type="item.type" />
      <p>{{ item.content }}</p>
      <div class="text-xs text-mute">{{ item.agent }} · {{ item.timestamp }}</div>
      <div class="flex flex-wrap gap-1">
        <span v-for="t in item.tags" :key="t" class="rounded bg-raised px-2 py-0.5 text-[11px]">{{ t }}</span>
      </div>
    </div>
  </el-drawer>
</template>
