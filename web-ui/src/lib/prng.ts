/**
 * Deterministic PRNG used for procedural icon generation.
 * FNV-1a hash seeds a mulberry32 generator — the same approach DiceBear uses,
 * so a given seed always yields the same icon (stable across reloads and devices).
 */

export function fnv1a(str: string): number {
  let h = 0x811c9dc5
  for (let i = 0; i < str.length; i++) {
    h ^= str.charCodeAt(i)
    h = Math.imul(h, 0x01000193)
  }
  return h >>> 0
}

export function mulberry32(seed: number): () => number {
  let a = seed >>> 0
  return function () {
    a |= 0
    a = (a + 0x6d2b79f5) | 0
    let t = Math.imul(a ^ (a >>> 15), 1 | a)
    t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296
  }
}

/** A seeded RNG with a few convenience helpers. */
export function createRng(seed: string) {
  const rand = mulberry32(fnv1a(seed))
  return {
    next: rand,
    range: (min: number, max: number) => min + rand() * (max - min),
    int: (min: number, max: number) => Math.floor(min + rand() * (max - min + 1)),
    pick: <T>(arr: readonly T[]): T => arr[Math.floor(rand() * arr.length)]!,
  }
}
