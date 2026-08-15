<script setup>
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import Layout from '../components/Layout.vue'

const { t } = useI18n()

const me = ref(null)
const sites = ref([])
const loading = ref(true)
const error = ref('')

async function loadMe() {
  const res = await fetch('/api/v1/reseller/me')
  if (res.ok) {
    me.value = await res.json()
  } else {
    const d = await res.json().catch(() => ({}))
    error.value = d.error || t('resellerdashboard.failed_to_load_info')
  }
}

async function loadSites() {
  const res = await fetch('/api/v1/reseller/my/sites')
  if (res.ok) {
    const data = await res.json()
    sites.value = Array.isArray(data) ? data : []
  }
}

function pct(used, max) {
  if (!max || max <= 0) return 0
  return Math.min(100, Math.round(((used ?? 0) / max) * 100))
}

function barColor(p) {
  if (p >= 90) return '#ef4444'
  if (p >= 70) return '#f59e0b'
  return 'var(--primary, #3b82f6)'
}

const metrics = [
  { key: 'sites',     label: t('resellerdashboard.sites'),            usedKey: 'site_count',           maxKey: 'max_sites',      icon: '<svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" d="M12 21a9.004 9.004 0 0 0 8.716-6.747M12 21a9.004 9.004 0 0 1-8.716-6.747M12 21c2.485 0 4.5-4.03 4.5-9S14.485 3 12 3m0 18c-2.485 0-4.5-4.03-4.5-9S9.515 3 12 3m0 0a8.997 8.997 0 0 1 7.843 4.582M12 3a8.997 8.997 0 0 0-7.843 4.582m15.686 0A11.953 11.953 0 0 1 12 10.5c-2.998 0-5.74-1.1-7.843-2.918m15.686 0A8.959 8.959 0 0 1 21 12c0 .778-.099 1.533-.284 2.253m0 0A17.919 17.919 0 0 1 12 16.5c-3.162 0-6.133-.815-8.716-2.247m0 0A9.015 9.015 0 0 1 3 12c0-1.605.42-3.113 1.157-4.418" /></svg>' },
  { key: 'databases', label: t('resellerdashboard.databases'),       usedKey: 'db_count',             maxKey: 'max_databases',  icon: '<svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" d="M20.25 6.375c0 2.278-3.694 4.125-8.25 4.125S3.75 8.653 3.75 6.375m16.5 0c0-2.278-3.694-4.125-8.25-4.125S3.75 4.097 3.75 6.375m16.5 0v11.25c0 2.278-3.694 4.125-8.25 4.125s-8.25-1.847-8.25-4.125V6.375m16.5 0v3.75m-16.5-3.75v3.75m16.5 0v3.75C20.25 16.153 16.556 18 12 18s-8.25-1.847-8.25-4.125v-3.75m16.5 0c0 2.278-3.694 4.125-8.25 4.125s-8.25-1.847-8.25-4.125" /></svg>' },
  { key: 'disk',      label: t('resellerdashboard.disk_gb'),            usedKey: 'disk_used_gb',         maxKey: 'max_disk_gb',    icon: '<svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" d="M20.25 7.5l-.625 10.632a2.25 2.25 0 0 1-2.247 2.118H6.622a2.25 2.25 0 0 1-2.247-2.118L3.75 7.5M10 11.25h4M3.375 7.5h17.25c.621 0 1.125-.504 1.125-1.125v-1.5c0-.621-.504-1.125-1.125-1.125H3.375c-.621 0-1.125.504-1.125 1.125v1.5c0 .621.504 1.125 1.125 1.125Z" /></svg>' },
  { key: 'bw',        label: t('resellerdashboard.bandwidth_gb'), usedKey: 'bandwidth_used_gb',    maxKey: 'max_bandwidth_gb', icon: '<svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" d="M3 7.5 7.5 3m0 0L12 7.5M7.5 3v13.5m13.5 0L16.5 21m0 0L12 16.5m4.5 4.5V7.5" /></svg>' },
]

onMounted(async () => {
  loading.value = true
  await Promise.all([loadMe(), loadSites()])
  loading.value = false
})
</script>

<template>
  <Layout>
    <div class="page">
      <h1>{{ $t('resellerdashboard.reseller_dashboard') }}</h1>

      <div v-if="error" class="alert error">{{ error }}</div>
      <div v-if="loading" class="muted">{{ $t('resellerdashboard.loading') }}</div>

      <template v-else-if="me">
        <!-- Quota Cards -->
        <div class="quota-grid">
          <div
            v-for="m in metrics"
            :key="m.key"
            class="card quota-card"
          >
            <div class="quota-top">
              <span class="quota-icon" v-html="m.icon"></span>
              <span class="quota-label">{{ m.label }}</span>
            </div>
            <div class="quota-nums">
              <span class="quota-used">{{ me[m.usedKey] ?? 0 }}</span>
              <span class="quota-sep"> / </span>
              <span class="quota-max">{{ me[m.maxKey] ?? '∞' }}</span>
            </div>
            <div class="progress-track">
              <div
                class="progress-fill"
                :style="{
                  width: pct(me[m.usedKey], me[m.maxKey]) + '%',
                  background: barColor(pct(me[m.usedKey], me[m.maxKey])),
                }"
              ></div>
            </div>
            <div class="quota-pct muted">
              {{ me[m.maxKey] ? pct(me[m.usedKey], me[m.maxKey]) + '%' : '—' }}
            </div>
          </div>
        </div>

        <!-- Sites Table -->
        <div class="card" style="margin-top: 24px">
          <h2 style="margin:0 0 16px">{{ $t('resellerdashboard.my_sites') }}</h2>
          <table>
            <thead>
              <tr>
                <th>{{ $t('resellerdashboard.id') }}</th>
                <th>{{ $t('resellerdashboard.domain_name') }}</th>
                <th>{{ $t('resellerdashboard.status') }}</th>
                <th>{{ $t('resellerdashboard.php') }}</th>
                <th>{{ $t('resellerdashboard.linux_user') }}</th>
                <th>{{ $t('resellerdashboard.created_at') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="s in sites" :key="s.id">
                <td class="mono">{{ s.id }}</td>
                <td>{{ s.name }}</td>
                <td>
                  <span class="badge" :class="s.status === 'active' ? 'ok' : 'error'">{{ s.status }}</span>
                </td>
                <td class="mono">{{ s.php_version_id ?? '—' }}</td>
                <td class="mono">{{ s.linux_user ?? '—' }}</td>
                <td class="muted">{{ s.created_at ? new Date(s.created_at).toLocaleDateString('tr-TR') : '—' }}</td>
              </tr>
              <tr v-if="!sites.length">
                <td colspan="6" class="muted">{{ $t('resellerdashboard.no_sites_yet') }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </template>
    </div>
  </Layout>
</template>

<style scoped>
.quota-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(210px, 1fr));
  gap: 16px;
}
.quota-card { padding: 20px; }
.quota-top {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 12px;
}
.quota-icon {
  width: 20px;
  height: 20px;
  display: flex;
  align-items: center;
  color: var(--primary, #3b82f6);
  opacity: 0.85;
  flex-shrink: 0;
}
:deep(.quota-icon svg) { width: 100%; height: 100%; }
.quota-label {
  font-size: 14px;
  font-weight: 600;
  color: #374151;
}
.quota-nums {
  font-size: 22px;
  font-weight: 700;
  color: #0f172a;
  margin-bottom: 10px;
  line-height: 1;
}
.quota-sep { font-size: 16px; color: #94a3b8; }
.quota-max { font-size: 16px; color: #64748b; }
.progress-track {
  height: 8px;
  background: #e2e8f0;
  border-radius: 999px;
  overflow: hidden;
}
.progress-fill {
  height: 100%;
  border-radius: 999px;
  transition: width 0.4s ease, background 0.3s ease;
  min-width: 4px;
}
.quota-pct {
  font-size: 12px;
  margin-top: 6px;
  text-align: right;
}
</style>
