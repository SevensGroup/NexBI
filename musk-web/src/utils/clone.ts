import { isRef, toRaw } from 'vue'

/** Unwrap Vue reactive / ref proxies so structuredClone can copy agent state. */
export function deepToRaw<T>(value: T): T {
  if (value === null || value === undefined) return value
  const unwrapped = isRef(value) ? deepToRaw(value.value as T) : (toRaw(value) as T)
  if (Array.isArray(unwrapped)) {
    return unwrapped.map((item) => deepToRaw(item)) as T
  }
  if (typeof unwrapped === 'object' && unwrapped !== null) {
    const proto = Object.getPrototypeOf(unwrapped)
    if (proto === Object.prototype || proto === null) {
      const out: Record<string, unknown> = {}
      for (const key of Object.keys(unwrapped)) {
        out[key] = deepToRaw((unwrapped as Record<string, unknown>)[key])
      }
      return out as T
    }
  }
  return unwrapped
}

export function clonePlain<T>(value: T): T {
  return structuredClone(deepToRaw(value))
}
