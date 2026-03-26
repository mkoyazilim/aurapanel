import { createRouter, createWebHistory } from 'vue-router'
import DashboardLayout from '../layouts/DashboardLayout.vue'
import Dashboard from '../views/Dashboard.vue'
import Websites from '../views/Websites.vue'
import Login from '../views/Login.vue'
import { useAuthStore } from '../stores/auth'

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
        path: 'dns',
        name: 'DNS',
        component: () => import('../views/DNS.vue')
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
        path: 'auradb',
        name: 'AuraDB',
        component: () => import('../views/AuraDB.vue')
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
        path: 'panel-port',
        name: 'PanelPort',
        component: () => import('../views/PanelPort.vue')
      }
    ]
  }
]

const router = createRouter({
  history: createWebHistory(),
  routes
})

// Authentication Guard (Zero-Trust Navigation)
router.beforeEach((to, from, next) => {
  const authStore = useAuthStore()
  
  if (to.meta.requiresAuth && !authStore.isAuthenticated) {
    next('/login')
  } else if (to.meta.requiresGuest && authStore.isAuthenticated) {
    next('/')
  } else {
    next()
  }
})

export default router
