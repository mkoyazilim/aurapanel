<script setup>
import { ref, onMounted, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from "../api.js"
import Layout from '../components/Layout.vue'

const { t } = useI18n()

const metrics     = ref([])   // []ServerMetrics
const events      = ref([])   // []ClusterEvent
const loadingM    = ref(true)
const loadingE    = ref(true)
const error       = ref('')

let refreshTimer = null

async function loadMetrics() {
  loadingM.value = true
  try {
    metrics.value = await api.get('/cluster/metrics') || []
  } catch (e) {
    error.value = e.message
  } finally {
    loadingM.value = false
  }
}

async function loadEvents() {
  loadingE.value = true
  try {
    events.value = await api.get('/cluster/events?limit=20') || []
  } finally {
    loadingE.value = false
  }
}

async function refresh() {
  await Promise.all([loadMetrics(), loadEvents()])
}

function statusClass(srv) {
  if (srv.error) return 'error'
  const s = (srv.status || '').toLowerCase()
  if (s === 'online' || s === 'active') return 'ok'
  if (s === 'offline') return 'error'
  return 'warning'
}

function barColor(pct) {
  if (pct >= 90) return '#ef4444'
  if (pct >= 70) return '#f59e0b'
  return '#22c55e'
}

function fmtUptime(sec) {
  const h = Math.floor(sec / 3600)
  const m = Math.floor((sec % 3600) / 60)
  return t('clusterdashboard.uptime_format', { h, m })
}

function eventTypeClass(type) {
  if (type === 'health_fail') return 'error'
  if (type === 'key_rotated') return 'warning'
  if (type === 'site_created') return 'ok'
  return ''
}

onMounted(() => {
  refresh()
  refreshTimer = setInterval(refresh, 30_000)
})
onUnmounted(() => clearInterval(refreshTimer))
</script>

<template>
  <Layout>
    <div class="page">
      <h1>{{ $t('clusterdashboard.cluster_dashboard') }}
        <span style="font-size:14px;color:var(--muted);font-weight:normal;margin-left:8px">{{ $t('clusterdashboard.auto_refresh') }}</span>
      </h1>
      <div class="tab-nav">
        <router-link to="/cluster" class="tab-item" active-class="active">{{ $t('servers.title') || 'Küme Yönetimi' }}</router-link>
        <router-link to="/cluster/dashboard" class="tab-item" active-class="active">Dashboard</router-link>
      </div>

      <div v-if="error" class="alert error">{{ error }}</div>

      <!-- Metrics cards -->
      <h2 style="margin-top:24px">{{ $t('clusterdashboard.server_metrics') }}</h2>
      <div v-if="loadingM" class="muted">{{ $t('clusterdashboard.loading_metrics') }}</div>
      <div v-else style="display:grid;grid-template-columns:repeat(auto-fill,minmax(280px,1fr));gap:16px;margin-bottom:24px">
        <div v-for="m in metrics" :key="m.server_id" class="card">
          <div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:12px">
            <div>
              <strong>{{ m.name }}</strong>
              <div class="muted" style="font-size:12px">{{ m.ip_address }}</div>
            </div>
            <span class="badge" :class="statusClass(m)">
              {{ m.error ? $t('clusterdashboard.offline') : m.status }}
            </span>
          </div>
          <div v-if="!m.error">
            <!-- CPU -->
            <div style="display:flex;justify-content:space-between;font-size:13px;margin-bottom:3px">
              <span>{{ $t('clusterdashboard.cpu') }}</span><span>{{ m.cpu_percent?.toFixed(1) }}%</span>
            </div>
            <div style="background:#e5e7eb;border-radius:4px;height:8px;margin-bottom:10px">
              <div :style="`width:${Math.min(m.cpu_percent,100)}%;background:${barColor(m.cpu_percent)};height:8px;border-radius:4px`"></div>
            </div>
            <!-- RAM -->
            <div style="display:flex;justify-content:space-between;font-size:13px;margin-bottom:3px">
              <span>{{ $t('clusterdashboard.ram') }}</span><span>{{ m.ram_percent?.toFixed(1) }}%</span>
            </div>
            <div style="background:#e5e7eb;border-radius:4px;height:8px;margin-bottom:10px">
              <div :style="`width:${Math.min(m.ram_percent,100)}%;background:${barColor(m.ram_percent)};height:8px;border-radius:4px`"></div>
            </div>
            <!-- Uptime -->
            <div style="font-size:12px;color:var(--muted)">{{ $t('clusterdashboard.uptime') }}: {{ fmtUptime(m.uptime_sec || 0) }}</div>
          </div>
          <div v-else class="muted" style="font-size:13px">{{ m.error }}</div>
        </div>
        <div v-if="!metrics.length" class="muted">{{ $t('clusterdashboard.no_nodes') }}</div>
      </div>

      <!-- Event log -->
      <h2>{{ $t('clusterdashboard.recent_events') }}</h2>
      <div v-if="loadingE" class="muted">{{ $t('clusterdashboard.loading_events') }}</div>
      <div v-else class="card">
        <table>
          <thead>
            <tr>
              <th>{{ $t('clusterdashboard.time') }}</th>
              <th>{{ $t('clusterdashboard.server') }}</th>
              <th>{{ $t('clusterdashboard.event') }}</th>
              <th>{{ $t('clusterdashboard.detail') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="e in events" :key="e.id">
              <td class="text-sm muted" style="white-space:nowrap">{{ e.created_at.replace('T',' ').slice(0,19) }}</td>
              <td class="text-sm mono">{{ e.server_id }}</td>
              <td><span class="badge" :class="eventTypeClass(e.event_type)">{{ e.event_type }}</span></td>
              <td class="text-sm muted">{{ e.detail || '—' }}</td>
            </tr>
            <tr v-if="!events.length">
              <td colspan="4" class="muted">{{ $t('clusterdashboard.no_events') }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
  </Layout>
</template>

<style scoped>
.tab-nav {
  display: flex; gap: 16px; border-bottom: 1px solid var(--border-color, #e2e8f0); margin-bottom: 24px;
}
.tab-item {
  padding: 8px 16px; color: var(--muted, #64748b); text-decoration: none; border-bottom: 2px solid transparent; margin-bottom: -1px; font-weight: 500; transition: all 0.2s;
}
.tab-item:hover { color: var(--text, #1e293b); }
.tab-item.active { color: var(--primary, #3b82f6); border-bottom-color: var(--primary, #3b82f6); }
</style>
