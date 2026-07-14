import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import type { User, UserRole } from '@/types'
import { api } from '@/api'

/** Authentication + user management, backed by the session-cookie API. */
export const useAuthStore = defineStore('auth', () => {
  const currentUser = ref<User | null>(null)
  const users = ref<User[]>([])
  const needsSetup = ref(false)
  const ready = ref(false)

  const isAuthenticated = computed(() => currentUser.value !== null)
  const isAdmin = computed(() => currentUser.value?.role === 'admin')

  /** Resolve initial auth state (called once by the router guard). */
  async function bootstrap() {
    try {
      const status = await api.setupStatus()
      needsSetup.value = status.needsSetup
    } catch {
      needsSetup.value = false
    }
    if (!needsSetup.value) {
      await fetchMe()
    }
    ready.value = true
  }

  async function fetchMe() {
    try {
      currentUser.value = await api.me()
    } catch {
      currentUser.value = null
    }
  }

  async function setup(username: string, password: string) {
    currentUser.value = await api.setup(username, password)
    needsSetup.value = false
  }

  async function login(username: string, password: string) {
    currentUser.value = await api.login(username, password)
  }

  async function logout() {
    try {
      await api.logout()
    } finally {
      currentUser.value = null
    }
  }

  async function fetchUsers() {
    users.value = await api.listUsers()
  }

  async function addUser(username: string, password: string, role: UserRole) {
    const user = await api.createUser(username, password, role)
    users.value.push(user)
  }

  async function removeUser(id: number) {
    await api.deleteUser(id)
    users.value = users.value.filter((u) => u.id !== id)
  }

  return {
    currentUser,
    users,
    needsSetup,
    ready,
    isAuthenticated,
    isAdmin,
    bootstrap,
    fetchMe,
    setup,
    login,
    logout,
    fetchUsers,
    addUser,
    removeUser,
  }
})
