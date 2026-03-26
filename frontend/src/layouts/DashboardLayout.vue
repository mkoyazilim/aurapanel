<template>
  <div class="min-h-screen bg-panel-darker flex text-gray-100">
    <!-- Sidebar -->
    <aside class="w-64 bg-panel-dark border-r border-panel-border flex flex-col transition-all duration-300">
      <div class="h-16 flex items-center px-6 border-b border-panel-border">
        <div class="flex items-center gap-2 text-brand-500 font-bold text-xl tracking-wide">
          <Activity class="w-6 h-6" />
          <span>AuraPanel</span>
        </div>
      </div>
      
      <nav class="flex-1 px-4 py-6 space-y-1">
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

        <router-link to="/emails" class="sidebar-link" active-class="sidebar-link-active">
          <Mail class="w-5 h-5 mr-3" />
          <span>{{ t('menu.emails') }}</span>
        </router-link>

        <router-link to="/dns" class="sidebar-link" active-class="sidebar-link-active">
          <Network class="w-5 h-5 mr-3" />
          <span>{{ t('menu.dns') }}</span>
        </router-link>
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
        
        <div class="flex items-center gap-6">
          <button class="text-gray-400 hover:text-white transition-colors">
            <Bell class="w-5 h-5" />
          </button>
          <div class="flex items-center gap-3 pl-6 border-l border-panel-border relative group">
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
                Güvenli Çıkış (Logout)
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
  </div>
</template>

<script setup>
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import { 
  Activity, 
  LayoutDashboard, 
  Globe, 
  Database, 
  Mail, 
  Users, 
  Box,
  Network,
  Bell, 
  ChevronDown,
  ShieldAlert
} from 'lucide-vue-next'

const { t } = useI18n()
const router = useRouter()
const authStore = useAuthStore()
const toggleMenu = ref(false)

const handleLogout = () => {
  authStore.logout()
  router.push('/login')
}
</script>

<style scoped>
.sidebar-link {
  @apply flex items-center px-3 py-2.5 text-sm font-medium rounded-lg text-gray-400 hover:text-white hover:bg-panel-card transition-all duration-200;
}

.sidebar-link-active {
  @apply bg-brand-500/10 text-brand-400 hover:bg-brand-500/10 hover:text-brand-400 border border-brand-500/20 shadow-[inset_0_0_12px_rgba(16,185,129,0.1)];
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
