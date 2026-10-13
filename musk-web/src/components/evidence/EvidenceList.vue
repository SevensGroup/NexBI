<script setup lang="ts">
import type { Evidence } from '@/types'
import EvidenceCard from './EvidenceCard.vue'

defineProps<{ items: Evidence[] }>()
const emit = defineEmits<{ open: [id: string] }>()
</script>

<template>
  <div class="space-y-1">
    <EvidenceCard v-for="e in items" :key="e.id" :item="e" @open="emit('open', $event)" />
    <div v-if="!items.length" class="text-xs text-mute">No evidence yet.</div>
  </div>
</template>
