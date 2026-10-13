<script setup lang="ts">
import { computed } from 'vue'
import type { TaskStatus } from '@/types'
import { TASK_STATUS_CLASS } from '@/utils/status'

const props = defineProps<{ status: TaskStatus }>()
const cls = computed(() => TASK_STATUS_CLASS[props.status])
</script>

<template>
  <span class="text-[11px] font-medium uppercase" :class="cls">{{ status }}</span>
</template>
