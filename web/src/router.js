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
  { path: '/cron', component: () => import('./views/Cron.vue') },
  { path: '/security', component: () => import('./views/Security.vue') },
  { path: '/logs', component: () => import('./views/Logs.vue') },
  { path: '/backups', component: () => import('./views/Backups.vue') },
  { path: '/cloudflare', component: () => import('./views/Cloudflare.vue') },
  { path: '/drift', component: () => import('./views/Drift.vue') },
  { path: '/settings', component: () => import('./views/Settings.vue') },
  { path: '/server/dashboard', component: () => import('./views/ServerDashboard.vue') },
  { path: '/sites/:id/dashboard', component: () => import('./views/SiteDashboard.vue') },
  { path: '/sites/:id/git', component: () => import('./views/GitDeploy.vue') },
  { path: '/sites/:id/nodejs', component: () => import('./views/Nodejs.vue') },
  { path: '/sites/:id/staging', component: () => import('./views/Staging.vue') },
  { path: '/sites/:id/cloudflare', component: () => import('./views/Cloudflare.vue') },
  { path: '/sites/:id/mail', component: () => import('./views/Mail.vue') },
  { path: '/mail', component: () => import('./views/Mail.vue') },
  { path: '/users', component: () => import('./views/Users.vue') },
  { path: '/cluster', component: () => import('./views/Servers.vue') },
  { path: '/cluster/dashboard', component: () => import('./views/ClusterDashboard.vue') },
  { path: '/dns', component: () => import('./views/DNS.vue') },
  { path: '/resellers', component: () => import('./views/Reseller.vue') },
  { path: '/extdns', component: () => import('./views/ExternalDNS.vue') },
  { path: '/sites/:id/waf', component: () => import('./views/WAF.vue') },
  { path: '/sites/:id/cdn', component: () => import('./views/CDN.vue') },
  { path: '/reseller/dashboard', component: () => import('./views/ResellerDashboard.vue') },
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

router.onError((error, to) => {
  if (
    error?.message?.includes('Failed to fetch dynamically imported module') ||
    error?.message?.includes('Importing a module script failed') ||
    error?.message?.includes('Strict MIME type checking')
  ) {
    window.location.href = to.fullPath
  }
})

export default router
