export async function copyJson(value: unknown): Promise<boolean> {
  try {
    const text = typeof value === 'string' ? value : JSON.stringify(value, null, 2)
    await navigator.clipboard.writeText(text)
    return true
  } catch {
    return false
  }
}

export function safeJson(value: unknown): string {
  try {
    return JSON.stringify(value, null, 2)
  } catch {
    return String(value)
  }
}
