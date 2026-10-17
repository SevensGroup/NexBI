<script setup lang="ts">
import { ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { DEMO_ACCOUNT, login } from '@/auth/session'

const route = useRoute()
const router = useRouter()
const account = ref(DEMO_ACCOUNT)
const password = ref('')
const error = ref('')
const submitting = ref(false)

function redirectTarget(): string {
  const redirect = route.query.redirect
  if (typeof redirect === 'string' && redirect.startsWith('/') && !redirect.startsWith('//') && !redirect.startsWith('/login')) {
    return redirect
  }
  return '/dashboard'
}

function submit(): void {
  error.value = ''
  submitting.value = true
  if (!login(account.value, password.value)) {
    error.value = 'Incorrect account or password'
    submitting.value = false
    return
  }
  void router.replace(redirectTarget())
}
</script>

<template>
  <div class="flex min-h-full items-center justify-center bg-canvas px-6 py-12">
    <div class="grid w-full max-w-4xl overflow-hidden rounded-3xl border border-line bg-panel md:grid-cols-[1.1fr_0.9fr]">
      <section class="hidden flex-col justify-between border-r border-line px-10 py-12 md:flex">
        <p class="text-sm font-semibold tracking-tight">MUSK</p>
        <div>
          <h1 class="text-3xl font-semibold leading-snug">Build a board in one sentence</h1>
          <p class="mt-4 max-w-sm text-sm leading-6 text-mute">
            Describe the channels, spend, and revenue. The agent queries the data, then writes the board.
          </p>
        </div>
        <p class="text-xs text-mute">Use the demo account to enter</p>
      </section>

      <form class="flex flex-col px-8 py-10" @submit.prevent="submit">
        <p class="text-sm font-semibold md:hidden">MUSK</p>
        <h2 class="mt-6 text-xl font-semibold md:mt-2">Sign in</h2>
        <p class="mt-2 text-sm text-mute">Use the demo account to enter the console</p>

        <label class="mt-8 text-xs text-mute">
          Account
          <input
            v-model="account"
            autocomplete="username"
            class="mt-1.5 w-full rounded-xl border border-line bg-canvas px-3 py-2.5 text-sm text-ink outline-none focus:border-accent"
            data-testid="login-account"
          />
        </label>
        <label class="mt-4 text-xs text-mute">
          Password
          <input
            v-model="password"
            type="password"
            autocomplete="current-password"
            class="mt-1.5 w-full rounded-xl border border-line bg-canvas px-3 py-2.5 text-sm text-ink outline-none focus:border-accent"
            data-testid="login-password"
          />
        </label>

        <p v-if="error" class="mt-3 text-sm text-rose-300" data-testid="login-error">{{ error }}</p>

        <button
          class="mt-6 h-11 rounded-xl bg-accent text-sm font-medium text-white disabled:opacity-50"
          type="submit"
          data-testid="login-submit"
          :disabled="submitting"
        >
          Enter console
        </button>
      </form>
    </div>
  </div>
</template>
