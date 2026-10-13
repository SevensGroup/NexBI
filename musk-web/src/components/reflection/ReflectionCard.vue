<script setup lang="ts">
import type { Reflection } from '@/types'
import ReflectionSummary from './ReflectionSummary.vue'
import { formatTime } from '@/utils/format'

defineProps<{ reflection: Reflection }>()

function yn(v: boolean): string {
  return v ? 'Yes' : 'No'
}
</script>

<template>
  <article class="rounded border border-line bg-panel p-3 text-sm">
    <div class="flex justify-between text-[11px] text-mute">
      <span>Reflection</span>
      <span>{{ formatTime(reflection.timestamp) }}</span>
    </div>
    <dl class="mt-2 grid grid-cols-2 gap-2 text-xs">
      <div>Goal satisfied <strong class="ml-1">{{ yn(reflection.goalSatisfied) }}</strong></div>
      <div>Evidence sufficient <strong class="ml-1">{{ yn(reflection.evidenceSufficient) }}</strong></div>
      <div>Result validated <strong class="ml-1">{{ yn(reflection.resultCorrect) }}</strong></div>
      <div>Next action <strong class="ml-1">{{ reflection.nextAction }}</strong></div>
    </dl>
    <div class="mt-2 rounded bg-raised px-2 py-1 text-xs">
      {{ reflection.goalSatisfied ? 'Ready to finish' : 'Continue' }}
    </div>
    <div v-if="reflection.missingInformation.length" class="mt-2 text-xs text-mute">
      Missing: {{ reflection.missingInformation.join('; ') }}
    </div>
    <div v-if="reflection.errors.length" class="mt-2 text-xs text-rose-300">
      Errors: {{ reflection.errors.join('; ') }}
    </div>
    <div class="mt-2">
      <ReflectionSummary :reflection="reflection" />
    </div>
  </article>
</template>
