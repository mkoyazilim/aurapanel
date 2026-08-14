import { createRouter, createWebHistory } from 'vue-router'
import { useAuth } from './stores/auth'

const routes = [
  { path: '/login', component: () => import('./views/Login.vue'), meta: { public: true } },
  { path: '/', component: () => import('./views/Dashboard.vue') },
  { path: '/sites', component: () => import('./views/Sites.vue') },
  { path: '/files', component: () => import('./views/FileManager.vue') },
  { path: '/databases', component: () => import('./views/Databases.vue') },
  { path: '/ssl', component: () => import('./views/SSL.vue') },
  { path: '/sftp', component: () => import('./views/SFTP.vue') },
  { path: '/backups', component: () => import('./views/Backups.vue') },
  { path: '/drift', component: () => import('./views/Drift.vue') },
  { path: '/settings', component: () => import('./views/Settings.vue') },
]

const router = createRouter({ history: createWebHistory(), routes })

router.beforeEach(async (to) => {
  const auth = useAuth()
  if (to.meta.public) return true
  if (!auth.user) {
    try {
      await auth.me()
    } catch {
      return '/login'
    }
  }
  return true
})

export default router
