<script setup lang="ts">
defineProps<{ summary: string }>()
</script>

<template>
  <p class="text-sm leading-relaxed text-ink/90">{{ summary }}</p>
</template>
