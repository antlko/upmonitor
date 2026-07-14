/** Thin fetch wrapper: same-origin, cookie-based auth, typed JSON + errors. */

export class ApiError extends Error {
  status: number
  constructor(status: number, message: string) {
    super(message)
    this.status = status
    this.name = 'ApiError'
  }
}

async function errorFrom(res: Response): Promise<ApiError> {
  let message = res.statusText
  try {
    const body = await res.json()
    if (body && typeof body.error === 'string') message = body.error
  } catch {
    // non-JSON error body; keep status text
  }
  return new ApiError(res.status, message)
}

/** JSON request. `body` is JSON-encoded when present. */
export async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  const init: RequestInit = { method, credentials: 'include' }
  if (body !== undefined) {
    init.headers = { 'Content-Type': 'application/json' }
    init.body = JSON.stringify(body)
  }
  const res = await fetch(path, init)
  if (!res.ok) throw await errorFrom(res)
  if (res.status === 204) return undefined as T
  const ct = res.headers.get('content-type') ?? ''
  if (ct.includes('application/json')) return (await res.json()) as T
  return undefined as T
}

/** Raw binary request (image / archive upload). */
export async function requestRaw<T>(
  method: string,
  path: string,
  body: BodyInit,
  contentType: string,
): Promise<T> {
  const res = await fetch(path, {
    method,
    credentials: 'include',
    headers: { 'Content-Type': contentType },
    body,
  })
  if (!res.ok) throw await errorFrom(res)
  const ct = res.headers.get('content-type') ?? ''
  if (ct.includes('application/json')) return (await res.json()) as T
  return undefined as T
}

/** Download a binary response as a Blob (config export). */
export async function getBlob(path: string): Promise<Blob> {
  const res = await fetch(path, { credentials: 'include' })
  if (!res.ok) throw await errorFrom(res)
  return res.blob()
}
