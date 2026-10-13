<script setup lang="ts">
import type { ToolCallStatus } from '@/types'

defineProps<{ status: ToolCallStatus }>()
</script>

<template>
  <span class="text-[11px] uppercase text-mute">{{ status }}</span>
</template>
