import { createRng } from '@/lib/prng'
import { initials as toInitials } from '@/lib/format'

/**
 * Procedural, offline icon generation. No AI model, no network — a seeded PRNG
 * drives a handful of tasteful SVG styles. Deterministic per (seed, style), so
 * regenerating with the same inputs is stable; change the seed for a new look.
 *
 * The generated SVG is returned as a data URL so it can be used interchangeably
 * with uploaded images (as an <img> src or CSS background) without id collisions.
 */

export type IconStyle = 'gradient' | 'mesh' | 'geometric'

export const ICON_STYLES: { value: IconStyle; label: string }[] = [
  { value: 'gradient', label: 'Gradient' },
  { value: 'mesh', label: 'Mesh' },
  { value: 'geometric', label: 'Geometric' },
]

const SIZE = 80

function hsl(h: number, s: number, l: number): string {
  return `hsl(${Math.round(h)} ${Math.round(s)}% ${Math.round(l)}%)`
}

function gradientSvg(seed: string, label: string): string {
  const rng = createRng(seed)
  const h1 = rng.range(0, 360)
  const h2 = (h1 + rng.range(30, 90)) % 360
  const angle = rng.pick([0, 45, 90, 135])
  const rad = (angle * Math.PI) / 180
  const x2 = (Math.cos(rad) * 0.5 + 0.5).toFixed(3)
  const y2 = (Math.sin(rad) * 0.5 + 0.5).toFixed(3)
  return `
    <defs>
      <linearGradient id="g" x1="0" y1="0" x2="${x2}" y2="${y2}">
        <stop offset="0" stop-color="${hsl(h1, 68, 58)}"/>
        <stop offset="1" stop-color="${hsl(h2, 70, 44)}"/>
      </linearGradient>
    </defs>
    <rect width="${SIZE}" height="${SIZE}" rx="20" fill="url(#g)"/>
    <text x="50%" y="52%" dy="0.03em" text-anchor="middle" dominant-baseline="middle"
      font-family="Inter, system-ui, sans-serif" font-size="30" font-weight="600"
      fill="#fff" fill-opacity="0.95" letter-spacing="0.5">${label}</text>`
}

function meshSvg(seed: string): string {
  const rng = createRng(seed)
  const baseH = rng.range(0, 360)
  const blobs = Array.from({ length: 4 }, (_, i) => {
    const h = (baseH + i * rng.range(25, 60)) % 360
    const cx = rng.range(5, 75)
    const cy = rng.range(5, 75)
    const r = rng.range(28, 52)
    return `<circle cx="${cx.toFixed(1)}" cy="${cy.toFixed(1)}" r="${r.toFixed(1)}" fill="${hsl(h, 72, 56)}" fill-opacity="0.9"/>`
  }).join('')
  return `
    <defs>
      <clipPath id="c"><rect width="${SIZE}" height="${SIZE}" rx="20"/></clipPath>
      <filter id="b"><feGaussianBlur stdDeviation="9"/></filter>
    </defs>
    <g clip-path="url(#c)">
      <rect width="${SIZE}" height="${SIZE}" fill="${hsl(baseH, 40, 22)}"/>
      <g filter="url(#b)">${blobs}</g>
    </g>`
}

function geometricSvg(seed: string): string {
  const rng = createRng(seed)
  const h1 = rng.range(0, 360)
  const h2 = (h1 + rng.range(120, 220)) % 360
  const bg = hsl(h1, 30, 18)
  const cells = 4
  const unit = SIZE / cells
  let shapes = ''
  for (let y = 0; y < cells; y++) {
    for (let x = 0; x < cells; x++) {
      if (rng.next() < 0.5) continue
      const c = rng.next() < 0.5 ? hsl(h1, 70, 60) : hsl(h2, 70, 58)
      const px = x * unit
      const py = y * unit
      if (rng.next() < 0.4) {
        shapes += `<circle cx="${px + unit / 2}" cy="${py + unit / 2}" r="${unit / 2}" fill="${c}"/>`
      } else {
        const rot = rng.pick([0, 90, 180, 270])
        shapes += `<path d="M${px} ${py} h${unit} v${unit} Z" fill="${c}" transform="rotate(${rot} ${px + unit / 2} ${py + unit / 2})"/>`
      }
    }
  }
  return `
    <defs><clipPath id="c"><rect width="${SIZE}" height="${SIZE}" rx="20"/></clipPath></defs>
    <g clip-path="url(#c)"><rect width="${SIZE}" height="${SIZE}" fill="${bg}"/>${shapes}</g>`
}

/** Build the inner SVG markup for a given style. */
export function generateIconSvg(seed: string, style: IconStyle, name: string): string {
  const label = toInitials(name)
  const inner =
    style === 'gradient'
      ? gradientSvg(seed, label)
      : style === 'mesh'
        ? meshSvg(seed)
        : geometricSvg(seed)
  return `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 ${SIZE} ${SIZE}" width="${SIZE}" height="${SIZE}">${inner}</svg>`
}

/** Encode an SVG string as a data URL suitable for <img src> / background-image. */
export function svgToDataUrl(svg: string): string {
  const compact = svg.replace(/\s+/g, ' ').trim()
  return `data:image/svg+xml,${encodeURIComponent(compact)}`
}

/** Convenience: generate a ready-to-use data URL. */
export function generateIconDataUrl(seed: string, style: IconStyle, name: string): string {
  return svgToDataUrl(generateIconSvg(seed, style, name))
}
