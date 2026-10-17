<script setup lang="ts">
import { useRoute, RouterView } from 'vue-router'
import AppLayout from '@/layouts/AppLayout.vue'

const route = useRoute()
</script>

<template>
  <RouterView v-if="route.meta.public" />
  <AppLayout v-else />
</template>
