import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router'
import { useAuthStore } from '@/stores/auth'

/**
 * Routes. `meta.bare` pages (auth/setup/public) render without the app shell;
 * everything else renders inside the sidebar + topbar layout.
 * Auth guards will be added when the backend is wired.
 */
const routes: RouteRecordRaw[] = [
  {
    path: '/',
    name: 'dashboard',
    component: () => import('@/views/DashboardView.vue'),
    meta: { title: 'Dashboard' },
  },
  {
    path: '/resources',
    name: 'resources',
    component: () => import('@/views/ResourcesView.vue'),
    meta: { title: 'Resources' },
  },
  {
    path: '/settings',
    name: 'settings',
    component: () => import('@/views/SettingsView.vue'),
    meta: { title: 'Settings' },
  },
  {
    path: '/cron',
    name: 'cron',
    component: () => import('@/views/CronJobsView.vue'),
    meta: { title: 'Cron Jobs' },
  },
  {
    path: '/setup',
    name: 'setup',
    component: () => import('@/views/SetupView.vue'),
    meta: { title: 'Setup', bare: true },
  },
  {
    path: '/login',
    name: 'login',
    component: () => import('@/views/LoginView.vue'),
    meta: { title: 'Sign in', bare: true },
  },
  {
    path: '/public',
    name: 'public',
    component: () => import('@/views/PublicDashboardView.vue'),
    meta: { title: 'Status', bare: true, public: true },
  },
  {
    path: '/:pathMatch(.*)*',
    name: 'not-found',
    component: () => import('@/views/NotFoundView.vue'),
    meta: { title: 'Not found', bare: true },
  },
]

const adminOnly = new Set(['resources', 'settings', 'cron'])

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes,
  scrollBehavior() {
    return { top: 0 }
  },
})

// Gate navigation on first-run setup, authentication and role.
router.beforeEach(async (to) => {
  const auth = useAuthStore()
  if (!auth.ready) {
    await auth.bootstrap()
  }

  // The public status page is always reachable (it self-gates via the API).
  if (to.meta.public) return true

  if (auth.needsSetup) {
    return to.name === 'setup' ? true : { name: 'setup' }
  }
  if (to.name === 'setup') {
    return { name: 'dashboard' }
  }
  if (!auth.isAuthenticated) {
    return to.name === 'login' ? true : { name: 'login' }
  }
  if (to.name === 'login') {
    return { name: 'dashboard' }
  }
  if (auth.currentUser?.role === 'readonly' && adminOnly.has(String(to.name))) {
    return { name: 'dashboard' }
  }
  return true
})

router.afterEach((to) => {
  const title = (to.meta.title as string) ?? ''
  document.title = title ? `${title} · upmonitor` : 'upmonitor'
})

export default router
