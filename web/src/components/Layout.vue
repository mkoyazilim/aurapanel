<template>
  <div class="layout">
    <aside class="sidebar" :class="{ 'is-collapsed': collapsed }">
      <div class="brand">
        <router-link to="/" class="brand-link">
          <img v-if="!collapsed" src="/logo.png" alt="AuraPanel" class="brand-logo" />
          <img v-else src="/icon.png" alt="AuraPanel" class="brand-icon" />
        </router-link>
      </div>
      <nav>
        <div class="nav-group" v-for="group in menu" :key="group.title">
          <div class="nav-title" v-if="!collapsed">{{ $t('menu.' + group.title) }}</div>
          <div class="nav-title collapsed-dots" v-else>•••</div>
          <router-link v-for="item in group.items" :key="item.to" :to="item.to"
            class="nav-item" active-class="active" :title="collapsed ? $t('menu.' + item.label) : ''">
            <span class="nav-icon" v-html="item.icon"></span>
            <span class="nav-label" v-if="!collapsed">{{ $t('menu.' + item.label) }}</span>
          </router-link>
        </div>
      </nav>
    </aside>
    <main class="content">
      <header class="topbar">
        <button class="toggle-btn" @click="toggleSidebar" :title="$t('menu.toggle')">
          <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" d="M3.75 6.75h16.5M3.75 12h16.5m-16.5 5.25H12" /></svg>
        </button>
        <div class="spacer"></div>
        <select v-model="locale" @change="changeLang" class="lang-select">
          <option value="tr">🇹🇷 TR</option>
          <option value="en">🇬🇧 EN</option>
        </select>
        <span v-if="auth.user" class="muted user-pill">{{ auth.user.username }}</span>
        <button class="btn btn-sm" @click="logout">{{ $t('common.logout') }}</button>
      </header>
      <slot />
    </main>
  </div>
</template>

<script setup>
import { ref, computed } from 'vue'
import { useRouter } from 'vue-router'
import { useAuth } from '../stores/auth'
import { useI18n } from 'vue-i18n'

const auth = useAuth()
const router = useRouter()
const { locale, t } = useI18n()

function changeLang(e) {
  localStorage.setItem('ap_lang', e.target.value)
}

const collapsed = ref(localStorage.getItem('sidebar_collapsed') === '1')

function toggleSidebar() {
  collapsed.value = !collapsed.value
  localStorage.setItem('sidebar_collapsed', collapsed.value ? '1' : '0')
}

const menu = computed(() => [
  {
    title: 'general',
    items: [{ to: '/', label: 'home', icon: '<svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" class="w-5 h-5"><path stroke-linecap="round" stroke-linejoin="round" d="m2.25 12 8.954-8.955c.44-.439 1.152-.439 1.591 0L21.75 12M4.5 9.75v10.125c0 .621.504 1.125 1.125 1.125H9.75v-4.875c0-.621.504-1.125 1.125-1.125h2.25c.621 0 1.125.504 1.125 1.125V21h4.125c.621 0 1.125-.504 1.125-1.125V9.75M8.25 21h8.25" /></svg>' }],
  },
  {
    title: 'management',
    items: [
      { to: '/sites', label: 'sites', icon: '<svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" d="M12 21a9.004 9.004 0 0 0 8.716-6.747M12 21a9.004 9.004 0 0 1-8.716-6.747M12 21c2.485 0 4.5-4.03 4.5-9S14.485 3 12 3m0 18c-2.485 0-4.5-4.03-4.5-9S9.515 3 12 3m0 0a8.997 8.997 0 0 1 7.843 4.582M12 3a8.997 8.997 0 0 0-7.843 4.582m15.686 0A11.953 11.953 0 0 1 12 10.5c-2.998 0-5.74-1.1-7.843-2.918m15.686 0A8.959 8.959 0 0 1 21 12c0 .778-.099 1.533-.284 2.253m0 0A17.919 17.919 0 0 1 12 16.5c-3.162 0-6.133-.815-8.716-2.247m0 0A9.015 9.015 0 0 1 3 12c0-1.605.42-3.113 1.157-4.418" /></svg>' },
      { to: '/files', label: 'files', icon: '<svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" d="M3.75 9.776c.112-.017.227-.026.344-.026h15.812c.117 0 .232.009.344.026m-16.5 0a2.25 2.25 0 0 0-1.883 2.542l.857 6a2.25 2.25 0 0 0 2.227 1.932H19.05a2.25 2.25 0 0 0 2.227-1.932l.857-6a2.25 2.25 0 0 0-1.883-2.542m-16.5 0V6A2.25 2.25 0 0 1 6 3.75h3.879a1.5 1.5 0 0 1 1.06.44l2.122 2.12a1.5 1.5 0 0 0 1.06.44H18A2.25 2.25 0 0 1 20.25 9v.776" /></svg>' },
      { to: '/databases', label: 'databases', icon: '<svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" d="M20.25 6.375c0 2.278-3.694 4.125-8.25 4.125S3.75 8.653 3.75 6.375m16.5 0c0-2.278-3.694-4.125-8.25-4.125S3.75 4.097 3.75 6.375m16.5 0v11.25c0 2.278-3.694 4.125-8.25 4.125s-8.25-1.847-8.25-4.125V6.375m16.5 0v3.75m-16.5-3.75v3.75m16.5 0v3.75C20.25 16.153 16.556 18 12 18s-8.25-1.847-8.25-4.125v-3.75m16.5 0c0 2.278-3.694 4.125-8.25 4.125s-8.25-1.847-8.25-4.125" /></svg>' },
      { to: '/sftp', label: 'sftp', icon: '<svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" d="M15.75 5.25a3 3 0 0 1 3 3m3 0a6 6 0 0 1-7.029 5.912c-.563-.097-1.159.026-1.563.43L10.5 17.25H8.25v2.25H6v2.25H2.25v-2.818c0-.597.237-1.17.659-1.591l6.499-6.499c.404-.404.527-1 .43-1.563A6 6 0 1 1 21.75 8.25Z" /></svg>' },
      { to: '/ssl', label: 'ssl', icon: '<svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" d="M16.5 10.5V6.75a4.5 4.5 0 1 0-9 0v3.75m-.75 11.25h10.5a2.25 2.25 0 0 0 2.25-2.25v-6.75a2.25 2.25 0 0 0-2.25-2.25H6.75a2.25 2.25 0 0 0-2.25 2.25v6.75a2.25 2.25 0 0 0 2.25 2.25Z" /></svg>' },
      { to: '/cron', label: 'cron', icon: '<svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" d="M12 6v6h4.5m4.5 0a9 9 0 1 1-18 0 9 9 0 0 1 18 0Z" /></svg>' },
      { to: '/security', label: 'security', icon: '<svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" d="M9 12.75 11.25 15 15 9.75m-3-7.036A11.959 11.959 0 0 1 3.598 6 11.99 11.99 0 0 0 3 9.749c0 5.592 3.824 10.29 9 11.623 5.176-1.332 9-6.03 9-11.622 0-1.31-.21-2.571-.598-3.751h-.152c-3.196 0-6.1-1.248-8.25-3.285Z" /></svg>' },
    ],
  },
  {
    title: 'maintenance',
    items: [
      { to: '/logs', label: 'logs', icon: '<svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" d="M6.75 7.5l3 2.25-3 2.25m4.5 0h3m-9 8.25h13.5A2.25 2.25 0 0 0 21 18V6a2.25 2.25 0 0 0-2.25-2.25H5.25A2.25 2.25 0 0 0 3 6v12a2.25 2.25 0 0 0 2.25 2.25Z" /></svg>' },
      { to: '/backups', label: 'backups', icon: '<svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" d="m20.25 7.5-.625 10.632a2.25 2.25 0 0 1-2.247 2.118H6.622a2.25 2.25 0 0 1-2.247-2.118L3.75 7.5M10 11.25h4M3.375 7.5h17.25c.621 0 1.125-.504 1.125-1.125v-1.5c0-.621-.504-1.125-1.125-1.125H3.375c-.621 0-1.125.504-1.125 1.125v1.5c0 .621.504 1.125 1.125 1.125Z" /></svg>' },
      { to: '/drift', label: 'drift', icon: '<svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3Z" /></svg>' },
    ],
  },
  {
    title: 'system',
    items: [{ to: '/settings', label: 'settings', icon: '<svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" d="M10.343 3.94c.09-.542.56-.94 1.11-.94h1.093c.55 0 1.02.398 1.11.94l.149.894c.07.424.384.764.78.93.398.164.855.142 1.205-.108l.737-.527a1.125 1.125 0 0 1 1.45.12l.773.774c.39.389.44 1.002.12 1.45l-.527.737c-.25.35-.272.806-.107 1.204.165.397.505.71.93.78l.893.15c.543.09.94.559.94 1.109v1.094c0 .55-.397 1.02-.94 1.11l-.894.149c-.424.07-.764.383-.929.78-.165.398-.143.854.107 1.204l.527.738c.32.447.269 1.06-.12 1.45l-.774.773a1.125 1.125 0 0 1-1.449.12l-.738-.527c-.35-.25-.806-.272-1.203-.107-.398.165-.71.505-.781.929l-.149.894c-.09.542-.56.94-1.11.94h-1.094c-.55 0-1.019-.398-1.11-.94l-.148-.894c-.071-.424-.384-.764-.781-.93-.398-.164-.854-.142-1.204.108l-.738.527c-.447.32-1.06.269-1.45-.12l-.773-.774a1.125 1.125 0 0 1-.12-1.45l.527-.737c.25-.35.272-.806.108-1.204-.165-.397-.506-.71-.93-.78l-.894-.15c-.542-.09-.94-.56-.94-1.109v-1.094c0-.55.398-1.02.94-1.11l.894-.149c.424-.07.765-.383.93-.78.165-.398.143-.854-.108-1.204l-.526-.738a1.125 1.125 0 0 1 .12-1.45l.773-.773a1.125 1.125 0 0 1 1.45-.12l.737.527c.35.25.807.272 1.204.107.397-.165.71-.505.78-.929l.15-.894Z" /><path stroke-linecap="round" stroke-linejoin="round" d="M15 12a3 3 0 1 1-6 0 3 3 0 0 1 6 0Z" /></svg>' }],
  },
])

async function logout() {
  await auth.logout()
  router.push('/login')
}
</script>

<style scoped>
.layout { display: flex; min-height: 100vh; background: #f8fafc; }
.sidebar {
  width: 260px;
  background: #ffffff;
  border-right: 1px solid var(--border);
  padding: 24px 16px;
  position: sticky;
  top: 0;
  height: 100vh;
  box-shadow: 1px 0 10px rgba(0,0,0,0.02);
  transition: width 0.3s ease, padding 0.3s ease;
  overflow-x: hidden;
  white-space: nowrap;
}
.sidebar.is-collapsed {
  width: 76px;
  padding: 24px 12px;
}
.brand { padding: 4px 6px 20px; display: flex; align-items: center; justify-content: center; }
.brand-link { display: flex; align-items: center; justify-content: center; width: 100%; text-decoration: none; }
.brand-logo { width: 100%; max-width: 210px; height: auto; max-height: 48px; object-fit: contain; }
.brand-icon { width: 34px; height: 34px; object-fit: contain; }
.nav-group { margin-bottom: 24px; }
.nav-title {
  font-size: 12px;
  font-weight: 600;
  color: #94a3b8;
  padding: 0 12px 10px;
  transition: opacity 0.2s;
}
.nav-title.collapsed-dots { text-align: center; padding: 0 0 10px; letter-spacing: 2px; }
.nav-item {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 10px 12px;
  border-radius: 8px;
  color: #475569;
  font-size: 14px;
  font-weight: 500;
  margin-bottom: 4px;
  transition: all 0.2s ease;
  overflow: hidden;
}
.sidebar.is-collapsed .nav-item { padding: 10px; justify-content: center; }
.nav-item:hover { background: #f1f5f9; color: #0f172a; }
.nav-item.active { background: #eff6ff; color: var(--primary); font-weight: 600; }
.nav-icon { width: 20px; height: 20px; display: flex; align-items: center; justify-content: center; opacity: 0.7; flex-shrink: 0; }
.nav-item.active .nav-icon { opacity: 1; }
:deep(.nav-icon svg) { width: 100%; height: 100%; }
.content { flex: 1; padding: 0; display: flex; flex-direction: column; background: #f8fafc; }
.topbar {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 10px 20px;
  background: #fff;
  border-bottom: 1px solid var(--border);
  position: sticky;
  top: 0;
  z-index: 10;
}
.spacer { flex: 1; }
.lang-select {
  width: auto !important;
  min-width: 90px;
  max-width: 96px;
  padding: 5px 8px;
  font-size: 13px;
  font-weight: 500;
  border-radius: 6px;
  cursor: pointer;
  background-color: #fff;
  border: 1px solid var(--border);
}
.user-pill { font-size: 13px; font-weight: 500; }
.toggle-btn {
  background: none; border: none; cursor: pointer; color: #64748b; padding: 4px;
  display: flex; align-items: center; justify-content: center; border-radius: 6px;
}
.toggle-btn:hover { background: #f1f5f9; color: #0f172a; }
.toggle-btn svg { width: 20px; height: 20px; }
.content :deep(.page) { padding: 24px; max-width: 1200px; margin: 0 auto; width: 100%; }
</style>
