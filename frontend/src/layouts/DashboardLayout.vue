<template>
  <div class="min-h-screen bg-panel-darker flex text-gray-100">
    <!-- Sidebar -->
    <aside class="w-64 bg-panel-dark border-r border-panel-border flex flex-col transition-all duration-300">
      <div class="h-16 flex items-center px-6 border-b border-panel-border">
        <div class="w-full flex items-center">
          <img
            src="/aurapanel-logo.png"
            alt="AuraPanel Logo"
            class="h-8 w-auto max-w-[180px] object-contain"
          />
        </div>
      </div>
      
      <nav class="flex-1 px-4 py-6 space-y-1 overflow-y-auto">
        <router-link to="/" class="sidebar-link" exact-active-class="sidebar-link-active">
          <LayoutDashboard class="w-5 h-5 mr-3" />
          <span>{{ t('menu.dashboard') }}</span>
        </router-link>
        
        <router-link to="/websites" class="sidebar-link" active-class="sidebar-link-active">
          <Globe class="w-5 h-5 mr-3" />
          <span>{{ t('menu.websites') }}</span>
        </router-link>

        <router-link to="/packages" class="sidebar-link" active-class="sidebar-link-active">
          <Box class="w-5 h-5 mr-3" />
          <span>{{ t('menu.packages') }}</span>
        </router-link>

        <router-link to="/users" class="sidebar-link" active-class="sidebar-link-active">
          <Users class="w-5 h-5 mr-3" />
          <span>{{ t('menu.users') }}</span>
        </router-link>

        <router-link to="/databases" class="sidebar-link" active-class="sidebar-link-active">
          <Database class="w-5 h-5 mr-3" />
          <span>{{ t('menu.databases') }}</span>
        </router-link>

        <router-link to="/auradb" class="sidebar-link" active-class="sidebar-link-active">
          <Table2 class="w-5 h-5 mr-3" />
          <span>AuraDB Explorer</span>
        </router-link>

        <router-link to="/emails" class="sidebar-link" active-class="sidebar-link-active">
          <Mail class="w-5 h-5 mr-3" />
          <span>{{ t('menu.emails') }}</span>
        </router-link>

        <router-link to="/ftp" class="sidebar-link" active-class="sidebar-link-active">
          <KeyRound class="w-5 h-5 mr-3" />
          <span>FTP</span>
        </router-link>

        <router-link to="/sftp" class="sidebar-link" active-class="sidebar-link-active">
          <KeyRound class="w-5 h-5 mr-3" />
          <span>SFTP</span>
        </router-link>

        <router-link to="/backups" class="sidebar-link" active-class="sidebar-link-active">
          <HardDrive class="w-5 h-5 mr-3" />
          <span>Backups</span>
        </router-link>

        <router-link to="/dns" class="sidebar-link" active-class="sidebar-link-active">
          <Network class="w-5 h-5 mr-3" />
          <span>{{ t('menu.dns') }}</span>
        </router-link>

        <div class="mt-2">
          <button @click="sslMenuOpen = !sslMenuOpen" class="sidebar-link w-full justify-between" :class="{ 'text-blue-400': isSslRoute }">
            <div class="flex items-center">
              <Lock class="w-5 h-5 mr-3" />
              <span>SSL</span>
            </div>
            <ChevronDown class="w-4 h-4 transition-transform duration-200" :class="{ 'rotate-180': sslMenuOpen }" />
          </button>

          <transition name="accordion">
            <div v-show="sslMenuOpen" class="ml-4 mt-1 space-y-0.5 border-l border-panel-border/50 pl-3">
              <router-link
                :to="{ path: '/ssl', query: { tab: 'manage' } }"
                class="sidebar-sub-link"
                :class="{ 'sidebar-sub-link-active': isSslTabActive('manage') }"
              >
                <span>Manage SSL</span>
              </router-link>
              <router-link
                :to="{ path: '/ssl', query: { tab: 'hostname' } }"
                class="sidebar-sub-link"
                :class="{ 'sidebar-sub-link-active': isSslTabActive('hostname') }"
              >
                <span>Hostname SSL</span>
              </router-link>
              <router-link
                :to="{ path: '/ssl', query: { tab: 'mail' } }"
                class="sidebar-sub-link"
                :class="{ 'sidebar-sub-link-active': isSslTabActive('mail') }"
              >
                <span>MailServer SSL</span>
              </router-link>
            </div>
          </transition>
        </div>

        <router-link to="/cloudflare" class="sidebar-link" active-class="sidebar-link-active">
          <Cloud class="w-5 h-5 mr-3" />
          <span>CloudFlare</span>
        </router-link>

        <router-link to="/filemanager" class="sidebar-link" active-class="sidebar-link-active">
          <FolderOpen class="w-5 h-5 mr-3" />
          <span>File Manager</span>
        </router-link>

        <router-link to="/php" class="sidebar-link" active-class="sidebar-link-active">
          <Code class="w-5 h-5 mr-3" />
          <span>PHP Yonetimi</span>
        </router-link>

        <router-link to="/ols-tuning" class="sidebar-link" active-class="sidebar-link-active">
          <Settings2 class="w-5 h-5 mr-3" />
          <span>OLS Tuning</span>
        </router-link>

        <router-link to="/reseller" class="sidebar-link" active-class="sidebar-link-active">
          <Users class="w-5 h-5 mr-3" />
          <span>Reseller & ACL</span>
        </router-link>

        <router-link to="/server-status" class="sidebar-link" active-class="sidebar-link-active">
          <Activity class="w-5 h-5 mr-3" />
          <span>Server Status</span>
        </router-link>

        <router-link to="/panel-port" class="sidebar-link" active-class="sidebar-link-active">
          <Settings2 class="w-5 h-5 mr-3" />
          <span>Panel Port</span>
        </router-link>

        <router-link to="/app-runtime" class="sidebar-link" active-class="sidebar-link-active">
          <TerminalSquare class="w-5 h-5 mr-3" />
          <span>App Runtime</span>
        </router-link>

        <router-link to="/minio" class="sidebar-link" active-class="sidebar-link-active">
          <HardDrive class="w-5 h-5 mr-3" />
          <span>MinIO</span>
        </router-link>

        <router-link to="/cron-jobs" class="sidebar-link" active-class="sidebar-link-active">
          <Clock3 class="w-5 h-5 mr-3" />
          <span>Cron Jobs</span>
        </router-link>

        <router-link to="/log-viewer" class="sidebar-link" active-class="sidebar-link-active">
          <ScrollText class="w-5 h-5 mr-3" />
          <span>Log Viewer</span>
        </router-link>

        <router-link to="/federated" class="sidebar-link" active-class="sidebar-link-active">
          <Network class="w-5 h-5 mr-3" />
          <span>Federated</span>
        </router-link>

        <!-- Security Accordion Menu -->
        <div class="mt-2">
          <button @click="securityMenuOpen = !securityMenuOpen" class="sidebar-link w-full justify-between" :class="{ 'text-blue-400': isSecurityRoute }">
            <div class="flex items-center">
              <Shield class="w-5 h-5 mr-3" />
              <span>{{ t('menu.security') }}</span>
            </div>
            <ChevronDown class="w-4 h-4 transition-transform duration-200" :class="{ 'rotate-180': securityMenuOpen }" />
          </button>

          <transition name="accordion">
            <div v-show="securityMenuOpen" class="ml-4 mt-1 space-y-0.5 border-l border-panel-border/50 pl-3">
              <router-link :to="{ path: '/security', query: { tab: 'overview' } }" class="sidebar-sub-link" active-class="sidebar-sub-link-active">
                <span>Overview</span>
              </router-link>
              <router-link :to="{ path: '/security', query: { tab: 'firewall' } }" class="sidebar-sub-link" active-class="sidebar-sub-link-active">
                <span>Firewall</span>
              </router-link>
              <router-link :to="{ path: '/security', query: { tab: 'waf' } }" class="sidebar-sub-link" active-class="sidebar-sub-link-active">
                <span>ML-WAF</span>
              </router-link>
              <router-link :to="{ path: '/security', query: { tab: '2fa' } }" class="sidebar-sub-link" active-class="sidebar-sub-link-active">
                <span>2FA (TOTP)</span>
              </router-link>
              <router-link :to="{ path: '/security', query: { tab: 'ssh' } }" class="sidebar-sub-link" active-class="sidebar-sub-link-active">
                <span>SSH Keys</span>
              </router-link>
              <router-link :to="{ path: '/security', query: { tab: 'hardening' } }" class="sidebar-sub-link" active-class="sidebar-sub-link-active">
                <span>Hardening</span>
              </router-link>
            </div>
          </transition>
        </div>

        <!-- Docker Accordion Menu -->
        <div class="mt-2">
          <button @click="dockerMenuOpen = !dockerMenuOpen" class="sidebar-link w-full justify-between" :class="{ 'text-blue-400': isDockerRoute }">
            <div class="flex items-center">
              <Container class="w-5 h-5 mr-3" />
              <span>Docker</span>
            </div>
            <ChevronDown class="w-4 h-4 transition-transform duration-200" :class="{ 'rotate-180': dockerMenuOpen }" />
          </button>

          <transition name="accordion">
            <div v-show="dockerMenuOpen" class="ml-4 mt-1 space-y-0.5 border-l border-panel-border/50 pl-3">
              <!-- Docker Manager Sub-group -->
              <div class="pt-1 pb-1">
                <span class="text-[10px] uppercase tracking-wider text-gray-500 font-semibold px-2">Manager</span>
              </div>
              <router-link to="/docker/images" class="sidebar-sub-link" active-class="sidebar-sub-link-active">
                <span>Manage Images</span>
              </router-link>
              <router-link to="/docker/containers" class="sidebar-sub-link" active-class="sidebar-sub-link-active">
                <span>Manage Containers</span>
              </router-link>
              <router-link to="/docker/create" class="sidebar-sub-link" active-class="sidebar-sub-link-active">
                <span>Create Container</span>
              </router-link>

              <!-- Docker Apps Sub-group -->
              <div class="pt-2 pb-1">
                <span class="text-[10px] uppercase tracking-wider text-gray-500 font-semibold px-2">Apps</span>
              </div>
              <router-link to="/docker/apps" class="sidebar-sub-link" active-class="sidebar-sub-link-active">
                <span>App Store</span>
              </router-link>
              <router-link to="/docker/apps/installed" class="sidebar-sub-link" active-class="sidebar-sub-link-active">
                <span>Installed Apps</span>
              </router-link>
              <router-link to="/docker/apps/packages" class="sidebar-sub-link" active-class="sidebar-sub-link-active">
                <span>Docker Packages</span>
              </router-link>
            </div>
          </transition>
        </div>
      </nav>

      <div class="p-4 border-t border-panel-border text-sm text-gray-400">
        <div class="flex items-center gap-2 mb-2">
          <ShieldAlert class="w-4 h-4 text-brand-500" />
          <span>Zero-Trust Active</span>
        </div>
        <div>Server Load: <span class="text-brand-400">0.45</span></div>
      </div>
    </aside>

    <!-- Main Content -->
    <main class="flex-1 flex flex-col min-w-0 overflow-hidden">
      <!-- Topbar -->
      <header class="h-16 bg-panel-dark/50 backdrop-blur-md border-b border-panel-border flex items-center justify-between px-8 sticky top-0 z-10">
        <div class="flex items-center">
          <h1 class="text-xl font-semibold text-white">{{ $route.name }}</h1>
        </div>
        
        <div class="flex items-center gap-4">
          <div class="relative">
            <button
              class="relative text-gray-400 hover:text-white transition-colors"
              @click="notificationOpen = !notificationOpen"
              title="Notifications"
            >
              <Bell class="w-5 h-5" />
              <span
                v-if="unreadCount > 0"
                class="absolute -top-2 -right-2 min-w-[18px] h-[18px] px-1 rounded-full bg-red-500 text-white text-[10px] leading-[18px] text-center font-semibold"
              >
                {{ unreadCount > 99 ? '99+' : unreadCount }}
              </span>
            </button>

            <div
              v-show="notificationOpen"
              class="absolute right-0 mt-3 w-[360px] bg-panel-card border border-panel-border rounded-xl shadow-2xl z-50 overflow-hidden"
            >
              <div class="px-4 py-3 border-b border-panel-border flex items-center justify-between">
                <div>
                  <p class="text-sm font-semibold text-white">Notifications</p>
                  <p class="text-xs text-gray-400">{{ unreadCount }} unread</p>
                </div>
                <div class="flex items-center gap-2">
                  <button class="text-xs text-brand-400 hover:text-brand-300 transition-colors" @click="markAllNotificationsRead">
                    Mark all read
                  </button>
                  <button class="text-xs text-red-400 hover:text-red-300 transition-colors" @click="clearNotifications">
                    Clear
                  </button>
                </div>
              </div>

              <div v-if="notifications.length === 0" class="px-4 py-8 text-center text-sm text-gray-500">
                No notifications yet.
              </div>

              <div v-else class="max-h-[380px] overflow-auto">
                <button
                  v-for="item in notifications"
                  :key="item.id"
                  class="w-full text-left px-4 py-3 border-b border-panel-border/60 hover:bg-panel-dark/60 transition-colors"
                  @click="openNotification(item.id)"
                >
                  <div class="flex items-start gap-3">
                    <span class="mt-1.5 w-2 h-2 rounded-full" :class="notificationDotClass(item.type)"></span>
                    <div class="min-w-0 flex-1">
                      <p class="text-sm font-medium text-white truncate">{{ item.title }}</p>
                      <p class="text-xs text-gray-400 mt-0.5 break-words">{{ item.message }}</p>
                      <p class="text-[11px] text-gray-500 mt-1">{{ formatNotificationTime(item.createdAt) }}</p>
                    </div>
                    <span v-if="!item.read" class="text-[10px] text-brand-400 font-semibold">NEW</span>
                  </div>
                </button>
              </div>
            </div>
          </div>

          <div class="flex items-center gap-3 pl-5 border-l border-panel-border relative group">
            <div class="w-8 h-8 rounded-full bg-gradient-to-tr from-brand-600 to-brand-400 flex items-center justify-center text-sm font-bold text-white shadow-lg">
              {{ authStore.user ? authStore.user.name.charAt(0) : 'A' }}
            </div>
            <div class="text-sm cursor-pointer" @click="toggleMenu = !toggleMenu">
              <p class="font-medium text-white">{{ authStore.user ? authStore.user.name : 'Admin' }}</p>
              <p class="text-xs text-gray-500">{{ authStore.user ? authStore.user.email : 'root@server' }}</p>
            </div>
            <ChevronDown class="w-4 h-4 text-gray-500 cursor-pointer" @click="toggleMenu = !toggleMenu" />
            
            <!-- Dropdown -->
            <div v-show="toggleMenu" class="absolute top-12 right-0 w-48 bg-panel-card border border-panel-border rounded-lg shadow-xl py-2 z-50">
              <button @click="handleLogout" class="w-full text-left px-4 py-2 text-sm text-red-400 hover:bg-panel-dark transition-colors">
                GÃ¼venli Ã‡Ä±kÄ±ÅŸ (Logout)
              </button>
            </div>
          </div>
        </div>
      </header>

      <!-- Page Content -->
      <div class="flex-1 overflow-auto p-8">
        <div class="max-w-7xl mx-auto">
          <router-view v-slot="{ Component }">
            <transition name="fade" mode="out-in">
              <component :is="Component" />
            </transition>
          </router-view>
        </div>
      </div>
    </main>

    <Teleport to="body">
      <div v-if="commandOpen" class="fixed inset-0 z-[120] bg-black/60 p-4" @click.self="commandOpen = false">
        <div class="mx-auto max-w-2xl bg-panel-card border border-panel-border rounded-2xl shadow-2xl overflow-hidden">
          <div class="p-4 border-b border-panel-border">
            <input
              v-model="commandQuery"
              class="aura-input"
              placeholder="Ctrl+K ile hizli gecis... (orn: dns, security, logs)"
            />
          </div>
          <div class="max-h-96 overflow-auto p-2 space-y-1">
            <button
              v-for="item in filteredCommandItems"
              :key="item.path"
              class="w-full text-left px-3 py-2 rounded-lg hover:bg-panel-dark transition text-sm flex items-center justify-between"
              @click="openCommandRoute(item.path)"
            >
              <span>{{ item.label }}</span>
              <span class="text-xs text-gray-500">{{ item.path }}</span>
            </button>
          </div>
        </div>
      </div>
    </Teleport>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onBeforeUnmount, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter, useRoute } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import { useNotificationStore } from '../stores/notifications'
import { 
  Activity, 
  LayoutDashboard, 
  Globe, 
  Database, 
  Mail, 
  Users, 
  Box,
  Network,
  Container,
  Bell, 
  ChevronDown,
  ShieldAlert,
  Cloud,
  FolderOpen,
  Code
  ,
  Shield,
  TerminalSquare,
  HardDrive,
  Clock3,
  ScrollText
  ,
  Table2,
  Settings2,
  Lock,
  KeyRound
} from 'lucide-vue-next'

const { t } = useI18n()
const router = useRouter()
const route = useRoute()
const authStore = useAuthStore()
const notificationStore = useNotificationStore()
const toggleMenu = ref(false)
const dockerMenuOpen = ref(false)
const securityMenuOpen = ref(false)
const sslMenuOpen = ref(false)
const commandOpen = ref(false)
const commandQuery = ref('')
const notificationOpen = ref(false)

const notifications = computed(() => notificationStore.orderedItems.slice(0, 50))
const unreadCount = computed(() => notificationStore.unreadCount)

const commandItems = [
  { label: 'Dashboard', path: '/' },
  { label: 'Websites', path: '/websites' },
  { label: 'Users', path: '/users' },
  { label: 'Packages', path: '/packages' },
  { label: 'Databases', path: '/databases' },
  { label: 'AuraDB Explorer', path: '/auradb' },
  { label: 'Emails', path: '/emails' },
  { label: 'FTP', path: '/ftp' },
  { label: 'SFTP', path: '/sftp' },
  { label: 'Backups', path: '/backups' },
  { label: 'DNS', path: '/dns' },
  { label: 'Manage SSL', path: '/ssl?tab=manage' },
  { label: 'Hostname SSL', path: '/ssl?tab=hostname' },
  { label: 'MailServer SSL', path: '/ssl?tab=mail' },
  { label: 'Security', path: '/security' },
  { label: 'App Runtime', path: '/app-runtime' },
  { label: 'MinIO', path: '/minio' },
  { label: 'Cron Jobs', path: '/cron-jobs' },
  { label: 'Log Viewer', path: '/log-viewer' },
  { label: 'Federated', path: '/federated' },
  { label: 'File Manager', path: '/filemanager' },
  { label: 'PHP', path: '/php' },
  { label: 'OLS Tuning', path: '/ols-tuning' },
  { label: 'Reseller & ACL', path: '/reseller' },
  { label: 'Server Status', path: '/server-status' },
  { label: 'Panel Port', path: '/panel-port' },
  { label: 'Docker Images', path: '/docker/images' },
  { label: 'Docker Containers', path: '/docker/containers' },
  { label: 'Docker App Store', path: '/docker/apps' }
]

const isDockerRoute = computed(() => route.path.startsWith('/docker'))
const isSecurityRoute = computed(() => route.path.startsWith('/security'))
const isSslRoute = computed(() => route.path.startsWith('/ssl'))
const isSslTabActive = (tab) => {
  if (!isSslRoute.value) return false
  const selectedTab = typeof route.query.tab === 'string' ? route.query.tab : 'manage'
  return selectedTab === tab
}
const filteredCommandItems = computed(() => {
  const q = commandQuery.value.trim().toLowerCase()
  if (!q) return commandItems
  return commandItems.filter(i => i.label.toLowerCase().includes(q) || i.path.toLowerCase().includes(q))
})

// Auto-open docker menu if on a docker route
if (route.path.startsWith('/docker')) {
  dockerMenuOpen.value = true
}

// Auto-open security menu if on a security route
if (route.path.startsWith('/security')) {
  securityMenuOpen.value = true
}

// Auto-open SSL menu if on an SSL route
if (route.path.startsWith('/ssl')) {
  sslMenuOpen.value = true
}

const handleLogout = () => {
  authStore.logout()
  router.push('/login')
}

const notificationDotClass = (type) => {
  if (type === 'success') return 'bg-green-400'
  if (type === 'warning') return 'bg-yellow-400'
  if (type === 'error') return 'bg-red-400'
  return 'bg-blue-400'
}

const formatNotificationTime = (timestamp) => {
  const value = Number(timestamp || 0)
  if (!value) return '-'
  const diffMs = Date.now() - value
  const diffMin = Math.floor(diffMs / 60000)
  if (diffMin < 1) return 'just now'
  if (diffMin < 60) return `${diffMin} min ago`
  const diffHour = Math.floor(diffMin / 60)
  if (diffHour < 24) return `${diffHour} h ago`
  const diffDay = Math.floor(diffHour / 24)
  if (diffDay < 7) return `${diffDay} d ago`
  return new Date(value).toLocaleString()
}

const openNotification = (id) => {
  notificationStore.markRead(id)
}

const markAllNotificationsRead = () => {
  notificationStore.markAllRead()
}

const clearNotifications = () => {
  notificationStore.clearAll()
  notificationOpen.value = false
}

const onKeydown = (e) => {
  if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'k') {
    e.preventDefault()
    commandOpen.value = !commandOpen.value
    return
  }
  if (e.key === 'Escape' && commandOpen.value) {
    commandOpen.value = false
    return
  }
  if (e.key === 'Escape' && notificationOpen.value) {
    notificationOpen.value = false
  }
}

const openCommandRoute = (path) => {
  commandOpen.value = false
  router.push(path)
}

watch(() => route.fullPath, () => {
  toggleMenu.value = false
  notificationOpen.value = false
})

onMounted(() => {
  window.addEventListener('keydown', onKeydown)
})

onBeforeUnmount(() => {
  window.removeEventListener('keydown', onKeydown)
})
</script>

<style scoped>
.sidebar-link {
  @apply flex items-center px-3 py-2.5 text-sm font-medium rounded-lg text-gray-400 hover:text-white hover:bg-panel-card transition-all duration-200;
}

.sidebar-link-active {
  @apply bg-brand-500/10 text-brand-400 hover:bg-brand-500/10 hover:text-brand-400 border border-brand-500/20 shadow-[inset_0_0_12px_rgba(16,185,129,0.1)];
}

.sidebar-sub-link {
  @apply flex items-center px-2 py-1.5 text-xs font-medium rounded-md text-gray-500 hover:text-white hover:bg-white/5 transition-all duration-150;
}

.sidebar-sub-link-active {
  @apply text-blue-400 bg-blue-500/10 hover:text-blue-400;
}

.accordion-enter-active,
.accordion-leave-active {
  transition: max-height 0.2s ease, opacity 0.2s ease;
  max-height: 300px;
  overflow: hidden;
}

.accordion-enter-from,
.accordion-leave-to {
  max-height: 0;
  opacity: 0;
}

.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.2s ease, transform 0.2s ease;
}

.fade-enter-from,
.fade-leave-to {
  opacity: 0;
  transform: translateY(10px);
}
</style>

