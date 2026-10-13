<script setup lang="ts">
const q = defineModel<string>('q', { default: '' })
const type = defineModel<string>('type', { default: '' })
const agent = defineModel<string>('agent', { default: '' })
</script>

<template>
  <div class="flex flex-wrap gap-2">
    <input v-model="q" class="rounded border border-line bg-canvas px-2 py-1 text-sm" placeholder="Search memory…" />
    <select v-model="type" class="rounded border border-line bg-canvas px-2 py-1 text-sm">
      <option value="">All types</option>
      <option value="SHORT_TERM">Short-term</option>
      <option value="TASK_MEMORY">Task</option>
      <option value="LONG_TERM">Long-term</option>
    </select>
    <select v-model="agent" class="rounded border border-line bg-canvas px-2 py-1 text-sm">
      <option value="">All agents</option>
      <option>Coordinator</option>
      <option>Analyst</option>
      <option>Researcher</option>
      <option>Reviewer</option>
    </select>
  </div>
</template>
