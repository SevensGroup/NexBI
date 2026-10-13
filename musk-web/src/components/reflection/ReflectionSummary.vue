<script setup lang="ts">
import type { Reflection } from '@/types'
import ConfidenceBar from '@/components/decision/ConfidenceBar.vue'

defineProps<{ reflection: Reflection }>()
</script>

<template>
  <p class="text-sm">
    {{ reflection.summary || (reflection.goalSatisfied ? 'Ready to finish.' : 'Continue — goal not yet satisfied.') }}
  </p>
  <div class="mt-2">
    <ConfidenceBar :value="reflection.confidence" />
  </div>
</template>
