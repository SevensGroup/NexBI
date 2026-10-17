const SESSION_KEY = 'musk-demo-session'

export const DEMO_ACCOUNT = 'demo'
export const DEMO_PASSWORD = 'musk888'

export function isAuthed(): boolean {
  return localStorage.getItem(SESSION_KEY) === '1'
}

export function login(account: string, password: string): boolean {
  if (account.trim() !== DEMO_ACCOUNT || password !== DEMO_PASSWORD) return false
  localStorage.setItem(SESSION_KEY, '1')
  return true
}

export function logout(): void {
  localStorage.removeItem(SESSION_KEY)
}
