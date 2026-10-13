<script setup lang="ts">
import { computed } from 'vue'
import { copyJson, safeJson } from '@/utils/copy'
import { ElMessage } from 'element-plus'

const props = defineProps<{ value: unknown; maxHeight?: string }>()
const text = computed(() => safeJson(props.value))

async function copy(): Promise<void> {
  const ok = await copyJson(props.value)
  ElMessage({ message: ok ? 'Copied JSON' : 'Copy failed', type: ok ? 'success' : 'error' })
}
</script>

<template>
  <div class="relative rounded border border-line bg-canvas">
    <button class="absolute right-2 top-2 text-[11px] text-mute hover:text-ink" type="button" @click="copy">
      Copy JSON
    </button>
    <pre class="overflow-auto p-3 pr-20 font-mono text-[11px] leading-relaxed text-mute" :style="{ maxHeight: maxHeight || '240px' }">{{ text }}</pre>
  </div>
</template>
