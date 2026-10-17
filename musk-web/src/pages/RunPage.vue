<script setup lang="ts">
import { useRouter } from 'vue-router'

const router = useRouter()
</script>

<template>
  <div class="flex h-full items-center justify-center px-6">
    <div class="max-w-md text-center">
      <h1 class="text-xl font-semibold tracking-tight">Run</h1>
      <p class="mt-2 text-sm leading-6 text-mute">
        No run is open. Start one from Dashboard, then the decisions, agents, tasks, and board show up here.
      </p>
      <button
        class="mt-6 h-10 rounded-xl bg-accent px-5 text-sm font-medium text-white"
        type="button"
        @click="router.push('/dashboard')"
      >
        Back to Dashboard
      </button>
    </div>
  </div>
</template>
