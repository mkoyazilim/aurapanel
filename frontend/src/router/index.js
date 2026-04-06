import { createRouter, createWebHistory } from 'vue-router'
import DashboardLayout from '../layouts/DashboardLayout.vue'
import Dashboard from '../views/Dashboard.vue'
import Websites from '../views/Websites.vue'
import Login from '../views/Login.vue'
import { useAuthStore } from '../stores/auth'
import { canAccessPath } from '../security/rbac'

const routes = [
  {
    path: '/login',
    name: 'Login',
    component: Login,
    meta: { requiresGuest: true }
  },
  {
    path: '/',
    component: DashboardLayout,
    meta: { requiresAuth: true },
    children: [
      {
        path: '',
        name: 'Dashboard',
        component: Dashboard
      },
      {
        path: 'websites',
        name: 'Websites',
        component: Websites
      },
      {
        path: 'websites/:domain',
        name: 'WebsiteManage',
        component: () => import('../views/WebsiteManage.vue')
      },
      {
        path: 'packages',
        name: 'Packages',
        component: () => import('../views/Packages.vue')
      },
      {
        path: 'users',
        name: 'Users',
        component: () => import('../views/Users.vue')
      },
      {
        path: 'databases',
        name: 'Databases',
        component: () => import('../views/Databases.vue')
      },
      {
        path: 'emails',
        name: 'Emails',
        component: () => import('../views/Emails.vue')
      },
      {
        path: 'ftp',
        name: 'FTP',
        component: () => import('../views/FTP.vue')
      },
      {
        path: 'sftp',
        name: 'SFTP',
        component: () => import('../views/SFTP.vue')
      },
      {
        path: 'dns',
        name: 'DNS',
        component: () => import('../views/DNS.vue')
      },
      {
        path: 'ssl',
        name: 'SSL',
        component: () => import('../views/SSL.vue')
      },
      {
        path: 'security',
        name: 'Security',
        component: () => import('../views/Security.vue')
      },
      {
        path: 'app-runtime',
        name: 'AppRuntime',
        component: () => import('../views/AppRuntime.vue')
      },
      {
        path: 'wordpress',
        name: 'WordPressManager',
        component: () => import('../views/WordPressManager.vue')
      },
      {
        path: 'plugins',
        name: 'PluginSDK',
        component: () => import('../views/PluginSDK.vue')
      },
      {
        path: 'minio',
        name: 'MinIO',
        component: () => import('../views/MinIO.vue')
      },
      {
        path: 'cron-jobs',
        name: 'CronJobs',
        component: () => import('../views/CronJobs.vue')
      },
      {
        path: 'log-viewer',
        name: 'LogViewer',
        component: () => import('../views/LogViewer.vue')
      },
      {
        path: 'federated',
        name: 'Federated',
        component: () => import('../views/Federated.vue')
      },
      {
        path: 'ops-center',
        name: 'OpsCenter',
        component: () => import('../views/OpsCenter.vue'),
        meta: { title: 'Ops Center' }
      },
      // Docker Manager routes
      {
        path: 'docker/images',
        name: 'Docker Images',
        component: () => import('../views/Docker.vue'),
        meta: { dockerTab: 'images' }
      },
      {
        path: 'docker/containers',
        name: 'Docker Containers',
        component: () => import('../views/Docker.vue'),
        meta: { dockerTab: 'containers' }
      },
      {
        path: 'docker/create',
        name: 'Docker Create',
        component: () => import('../views/Docker.vue'),
        meta: { dockerTab: 'create' }
      },
      // Docker Apps routes
      {
        path: 'docker/apps',
        name: 'Docker App Store',
        component: () => import('../views/DockerApps.vue'),
        meta: { dockerAppsTab: 'templates' }
      },
      {
        path: 'docker/apps/installed',
        name: 'Docker Installed Apps',
        component: () => import('../views/DockerApps.vue'),
        meta: { dockerAppsTab: 'installed' }
      },
      {
        path: 'docker/apps/packages',
        name: 'Docker Packages',
        component: () => import('../views/DockerApps.vue'),
        meta: { dockerAppsTab: 'packages' }
      },
      // CloudFlare
      {
        path: 'cloudflare',
        name: 'CloudFlare',
        component: () => import('../views/CloudFlare.vue')
      },
      // File Manager
      {
        path: 'filemanager',
        name: 'FileManager',
        component: () => import('../views/FileManager.vue')
      },
      // PHP Management
      {
        path: 'php',
        name: 'PHP',
        component: () => import('../views/PHP.vue')
      },
      // Server Status
      {
        path: 'server-status',
        name: 'ServerStatus',
        component: () => import('../views/ServerStatus.vue')
      },
      {
        path: 'cloudlinux',
        name: 'CloudLinux',
        component: () => import('../views/CloudLinux.vue')
      },
      {
        path: 'panel-control',
        name: 'PanelPort',
        component: () => import('../views/PanelPort.vue')
      },
      {
        path: 'panel-update',
        name: 'PanelUpdate',
        component: () => import('../views/PanelUpdate.vue')
      },
      {
        path: 'panel-port',
        redirect: (to) => ({ path: '/panel-control', query: to.query, hash: to.hash })
      },
      {
        path: 'backups',
        name: 'Backups',
        component: () => import('../views/Backup.vue')
      },
      {
        path: 'ols-tuning',
        name: 'OlsTuning',
        component: () => import('../views/OlsTuning.vue')
      },
      {
        path: 'mail-tuning',
        name: 'MailTuning',
        component: () => import('../views/MailTuning.vue'),
        meta: { title: 'Mail Tuning' }
      },
      {
        path: 'reseller',
        name: 'Reseller',
        component: () => import('../views/Reseller.vue')
      },
      {
        path: 'activity-log',
        name: 'ActivityLog',
        component: () => import('../views/ActivityLog.vue'),
        meta: { title: 'Activity Log' }
      },
      {
        path: 'db-backup',
        name: 'DbBackup',
        component: () => import('../views/DbBackup.vue'),
        meta: { title: 'DB Backup' }
      },
      {
        path: 'terminal',
        name: 'Terminal',
        component: () => import('../views/Terminal.vue'),
        meta: { title: 'Web Terminal' }
      },
      {
        path: 'migration',
        name: 'Migration',
        component: () => import('../views/MigrationWizard.vue'),
        meta: { title: 'Migration Wizard' }
      },
      {
        path: 'ai-tools',
        name: 'AITools',
        component: () => import('../views/AITools.vue')
      },
      {
        path: 'api-settings',
        name: 'ApiSettings',
        component: () => import('../views/ApiSettings.vue')
      }
    ]
  }
]

const router = createRouter({
  history: createWebHistory(),
  routes
})

const CHUNK_RELOAD_GUARD_KEY = 'aurapanel:chunk-reload-once'

// Authentication Guard (Zero-Trust Navigation)
router.beforeEach((to, from, next) => {
  const authStore = useAuthStore()
  
  if (to.meta.requiresAuth && !authStore.isAuthenticated) {
    next('/login')
  } else if (to.meta.requiresGuest && authStore.isAuthenticated) {
    next('/')
  } else if (to.meta.requiresAuth && !canAccessPath(to.path, authStore.role, authStore.permissions)) {
    next('/')
  } else {
    next()
  }
})

router.onError((error, to) => {
  const message = String(error?.message || '')
  const isChunkLoadError =
    /Failed to fetch dynamically imported module|Importing a module script failed|ChunkLoadError|Loading chunk/i.test(
      message
    )

  if (!isChunkLoadError || typeof window === 'undefined') {
    return
  }

  const alreadyRetried = window.sessionStorage.getItem(CHUNK_RELOAD_GUARD_KEY) === '1'
  if (alreadyRetried) {
    window.sessionStorage.removeItem(CHUNK_RELOAD_GUARD_KEY)
    return
  }

  window.sessionStorage.setItem(CHUNK_RELOAD_GUARD_KEY, '1')
  const targetPath = to?.fullPath || `${window.location.pathname}${window.location.search}${window.location.hash}`
  window.location.replace(targetPath)
})

router.afterEach(() => {
  if (typeof window !== 'undefined') {
    window.sessionStorage.removeItem(CHUNK_RELOAD_GUARD_KEY)
  }
})

export default router
