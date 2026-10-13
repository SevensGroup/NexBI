<script setup lang="ts">
import { useVueFlow } from '@vue-flow/core'

const { zoomIn, zoomOut, fitView } = useVueFlow()
</script>

<template>
  <div class="absolute right-2 top-2 z-10 flex gap-1">
    <button class="rounded border border-line bg-panel px-2 py-0.5 text-[11px] text-mute hover:text-ink" type="button" @click="zoomIn()">+</button>
    <button class="rounded border border-line bg-panel px-2 py-0.5 text-[11px] text-mute hover:text-ink" type="button" @click="zoomOut()">−</button>
    <button class="rounded border border-line bg-panel px-2 py-0.5 text-[11px] text-mute hover:text-ink" type="button" @click="fitView()">Fit</button>
  </div>
</template>
