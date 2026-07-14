/**
 * Client-side image optimization. Uploaded images are resized and re-encoded to
 * WebP in the browser (Canvas API) before upload, so the Go backend only ever
 * stores small WebP files and needs no image codecs of its own.
 */

const MAX_DIMENSION = 512
const QUALITY = 0.82

function canvasToWebP(canvas: HTMLCanvasElement, quality = QUALITY): Promise<Blob> {
  return new Promise((resolve, reject) => {
    canvas.toBlob(
      (blob) => {
        if (!blob) return reject(new Error('Image encoding failed'))
        if (blob.type !== 'image/webp') {
          return reject(new Error('This browser cannot encode WebP; please try a WebP file'))
        }
        resolve(blob)
      },
      'image/webp',
      quality,
    )
  })
}

/** Resize an uploaded image to fit within MAX_DIMENSION and encode it as WebP. */
export async function optimizeToWebP(file: File): Promise<Blob> {
  const bitmap = await createImageBitmap(file)
  try {
    const scale = Math.min(1, MAX_DIMENSION / Math.max(bitmap.width, bitmap.height))
    const w = Math.max(1, Math.round(bitmap.width * scale))
    const h = Math.max(1, Math.round(bitmap.height * scale))
    const canvas = document.createElement('canvas')
    canvas.width = w
    canvas.height = h
    const ctx = canvas.getContext('2d')
    if (!ctx) throw new Error('Canvas not available')
    ctx.drawImage(bitmap, 0, 0, w, h)
    return await canvasToWebP(canvas)
  } finally {
    bitmap.close()
  }
}

/** Rasterize an SVG string to a square WebP (used for generated icons). */
export async function svgToWebP(svg: string, size = 256): Promise<Blob> {
  const url = 'data:image/svg+xml;charset=utf-8,' + encodeURIComponent(svg)
  const img = new Image()
  img.width = size
  img.height = size
  await new Promise<void>((resolve, reject) => {
    img.onload = () => resolve()
    img.onerror = () => reject(new Error('Could not render icon'))
    img.src = url
  })
  const canvas = document.createElement('canvas')
  canvas.width = size
  canvas.height = size
  const ctx = canvas.getContext('2d')
  if (!ctx) throw new Error('Canvas not available')
  ctx.drawImage(img, 0, 0, size, size)
  return canvasToWebP(canvas)
}
