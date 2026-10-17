<script setup lang="ts">
const q = defineModel<string>('q', { default: '' })
const type = defineModel<string>('type', { default: '' })
const agent = defineModel<string>('agent', { default: '' })
</script>

<template>
  <div class="flex flex-wrap gap-2">
    <input v-model="q" class="rounded-xl border border-line bg-canvas px-3 py-2 text-sm outline-none focus:border-accent" placeholder="Search memory…" />
    <select v-model="type" class="rounded-xl border border-line bg-canvas px-3 py-2 text-sm">
      <option value="">All types</option>
      <option value="SHORT_TERM">Short-term</option>
      <option value="TASK_MEMORY">Task</option>
      <option value="LONG_TERM">Long-term</option>
    </select>
    <select v-model="agent" class="rounded-xl border border-line bg-canvas px-3 py-2 text-sm">
      <option value="">All agents</option>
      <option>Coordinator</option>
      <option>Analyst</option>
      <option>Researcher</option>
      <option>Reviewer</option>
    </select>
  </div>
</template>
