<template>
  <div class="layout">
    <aside class="sidebar">
      <div class="brand"><img src="/icon.png" alt="AuraPanel" height="28" style="vertical-align: middle; margin-right: 8px;" />AuraPanel</div>
      <nav>
        <div class="nav-group" v-for="group in menu" :key="group.title">
          <div class="nav-title">{{ group.title }}</div>
          <router-link v-for="item in group.items" :key="item.to" :to="item.to"
            class="nav-item" active-class="active">
            <span class="nav-icon">{{ item.icon }}</span>{{ item.label }}
          </router-link>
        </div>
      </nav>
    </aside>
    <main class="content">
      <header class="topbar">
        <div class="muted mono">aurapanel</div>
        <div class="spacer"></div>
        <span v-if="auth.user" class="muted">{{ auth.user.username }}</span>
        <button class="btn" @click="logout">Çıkış</button>
      </header>
      <slot />
    </main>
  </div>
</template>

<script setup>
import { useRouter } from 'vue-router'
import { useAuth } from '../stores/auth'

const auth = useAuth()
const router = useRouter()

// Hiyerarşik kategorili menü (gereksinim: kategorili sınıflandırma).
const menu = [
  {
    title: 'Genel',
    items: [{ to: '/', label: 'Ana Sayfa', icon: '🏠' }],
  },
  {
    title: 'Yönetim',
    items: [
      { to: '/sites', label: 'Siteler', icon: '🌐' },
      { to: '/files', label: 'Dosya Yöneticisi', icon: '📁' },
      { to: '/databases', label: 'Veritabanları', icon: '🗄️' },
      { to: '/ssl', label: 'SSL Sertifikaları', icon: '🔒' },
    ],
  },
  {
    title: 'Bakım',
    items: [
      { to: '/backups', label: 'Yedekler', icon: '💾' },
      { to: '/drift', label: 'Drift İzleme', icon: '🧭' },
    ],
  },
  {
    title: 'Sistem',
    items: [{ to: '/settings', label: 'Ayarlar', icon: '⚙️' }],
  },
]

async function logout() {
  await auth.logout()
  router.push('/login')
}
</script>

<style scoped>
.layout { display: flex; min-height: 100vh; }
.sidebar {
  width: 230px;
  background: #ffffff;
  border-right: 1px solid var(--border);
  padding: 16px 10px;
  position: sticky;
  top: 0;
  height: 100vh;
}
.brand { font-weight: 700; font-size: 16px; padding: 6px 12px 16px; }
.nav-group { margin-bottom: 14px; }
.nav-title {
  font-size: 11px;
  text-transform: uppercase;
  letter-spacing: 0.06em;
  color: var(--muted);
  padding: 6px 12px;
}
.nav-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 12px;
  border-radius: 8px;
  color: var(--text);
  font-size: 13px;
  margin-bottom: 2px;
}
.nav-item:hover { background: #f1f5f9; }
.nav-item.active { background: #eef2ff; color: var(--primary); font-weight: 600; }
.nav-icon { width: 18px; text-align: center; }
.content { flex: 1; padding: 0; display: flex; flex-direction: column; }
.topbar {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 20px;
  background: #fff;
  border-bottom: 1px solid var(--border);
  position: sticky;
  top: 0;
}
.content :deep(.page) { padding: 20px; }
</style>
