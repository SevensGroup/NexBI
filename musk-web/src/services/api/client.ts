import axios from 'axios'

const envBase = import.meta.env.VITE_API_BASE_URL || 'http://127.0.0.1:8080'
const useProxy = typeof window !== 'undefined' && import.meta.env.DEV

export const api = axios.create({
  baseURL: useProxy ? '' : envBase,
  timeout: 30_000,
  headers: { 'Content-Type': 'application/json' },
})

/** Mock is opt-in. Default talks to musk-back via Vite `/api` proxy. */
export function isMockMode(): boolean {
  const flag = String(import.meta.env.VITE_USE_MOCK ?? '').toLowerCase()
  return flag === 'true' || flag === '1'
}
