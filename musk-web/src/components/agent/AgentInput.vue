<script setup lang="ts">
import { useStorage } from '@vueuse/core'
import { DEFAULT_GOAL } from '@/mocks/catalog'

const props = defineProps<{ disabled?: boolean; placeholder?: string }>()
const emit = defineEmits<{ submit: [goal: string] }>()
const text = useStorage('musk-agent-goal', DEFAULT_GOAL)
if (text.value === 'Analyze why Meta ad ROI declined yesterday') text.value = DEFAULT_GOAL

function send(): void {
  const g = text.value.trim()
  if (!g || props.disabled) return
  emit('submit', g)
}
</script>

<template>
  <form class="flex items-end gap-3 border-t border-line bg-panel px-4 py-3" @submit.prevent="send">
    <textarea
      v-model="text"
      rows="2"
      class="min-h-[44px] flex-1 resize-none rounded-xl border border-line bg-canvas px-3 py-2 text-sm outline-none focus:border-accent"
      :placeholder="placeholder || 'Goal for the agent…'"
      :disabled="disabled"
      @keydown.meta.enter="send"
      @keydown.ctrl.enter="send"
    />
    <button
      class="h-10 rounded-xl bg-accent px-4 text-sm font-medium text-white disabled:opacity-40"
      type="submit"
      :disabled="disabled || !text.trim()"
    >
      Run
    </button>
  </form>
</template>
