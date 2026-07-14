import { request, requestRaw, getBlob } from './http'
import type { Service, User, UserRole, AppSettings, WidgetMode } from '@/types'

export { ApiError } from './http'

export interface ServiceInput {
  name: string
  url: string
  interval: number
  mode: WidgetMode
}

export interface LayoutItem {
  id: string
  x: number
  y: number
  w: number
  h: number
  mode?: WidgetMode
}

/** Typed client for the upmonitor REST API. */
export const api = {
  // Setup & auth.
  setupStatus: () => request<{ needsSetup: boolean }>('GET', '/api/setup/status'),
  setup: (username: string, password: string) =>
    request<User>('POST', '/api/setup', { username, password }),
  login: (username: string, password: string) =>
    request<User>('POST', '/api/auth/login', { username, password }),
  logout: () => request<void>('POST', '/api/auth/logout'),
  me: () => request<User>('GET', '/api/auth/me'),

  // Services & metrics.
  listServices: () => request<Service[]>('GET', '/api/services'),
  createService: (input: ServiceInput) => request<Service>('POST', '/api/services', input),
  updateService: (id: string, input: Partial<ServiceInput>) =>
    request<Service>('PUT', `/api/services/${id}`, input),
  deleteService: (id: string) => request<void>('DELETE', `/api/services/${id}`),
  updateLayout: (items: LayoutItem[]) => request<void>('PATCH', '/api/services/layout', items),
  checkNow: (id: string) => request<Service>('POST', `/api/services/${id}/check`),
  uploadImage: (id: string, blob: Blob) =>
    requestRaw<{ icon: string }>('POST', `/api/services/${id}/image`, blob, 'image/webp'),
  deleteImage: (id: string) => request<void>('DELETE', `/api/services/${id}/image`),

  // Settings, config & users.
  getSettings: () => request<AppSettings>('GET', '/api/settings'),
  updateSettings: (settings: AppSettings) => request<AppSettings>('PUT', '/api/settings', settings),
  setConfigPath: (path: string) =>
    request<AppSettings>('PUT', '/api/settings/config-path', { path }),
  listUsers: () => request<User[]>('GET', '/api/users'),
  createUser: (username: string, password: string, role: UserRole) =>
    request<User>('POST', '/api/users', { username, password, role }),
  deleteUser: (id: number) => request<void>('DELETE', `/api/users/${id}`),

  // Backup / restore.
  exportConfig: () => getBlob('/api/config/export'),
  importConfig: (archive: Blob) =>
    requestRaw<{ ok: boolean; services: number }>(
      'POST',
      '/api/config/import',
      archive,
      'application/zip',
    ),

  // Public dashboard.
  publicServices: () => request<Service[]>('GET', '/api/public/services'),
}
