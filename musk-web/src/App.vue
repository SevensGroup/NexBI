<script setup lang="ts">
import { ref } from 'vue'
import { useRoute, RouterView } from 'vue-router'
import AppLayout from '@/layouts/AppLayout.vue'
import router from '@/router'

const route = useRoute()
const ready = ref(false)
void router.isReady().then(() => {
  ready.value = true
})
</script>

<template>
  <template v-if="ready">
    <RouterView v-if="route.meta.public" />
    <AppLayout v-else />
  </template>
</template>
