
<template>
  <Layout>
    <div class="page">
      <div class="dashboard-header">
        <h1>{{ $t('menu.home') }}</h1>
        <button class="btn btn-sm" @click="loadData">🔄 {{ $t('common.refresh') }}</button>
      </div>

      <div v-if="error" class="alert error">{{ error }}</div>

      <!-- Health Status Banner -->
      <div v-if="health" class="health-banner card" :class="health.overall ? 'health-ok' : 'health-warn'">
        <div class="health-summary">
          <span class="health-indicator" :class="health.overall ? 'status-green' : 'status-yellow'"></span>
          <div>
            <strong>{{ health.overall ? $t('dashboard.all_systems_healthy') : $t('dashboard.system_attention_needed') }}</strong>
            <p class="muted text-sm" style="margin: 2px 0 0 0">
              {{ $t('dashboard.last_check') }}: {{ health.checked_at }}
            </p>
          </div>
        </div>
        <div class="health-checks-grid">
          <div v-for="c in health.checks" :key="c.name" class="check-pill" :class="c.ok ? 'pill-ok' : 'pill-warn'">
            <span class="dot"></span>
            <span class="check-name">{{ formatCheckName(c.name) }}</span>
            <span v-if="!c.ok" class="check-msg">({{ c.message }})</span>
          </div>
        </div>
      </div>

      <!-- System Stats Row -->
      <div v-if="sysStats" class="row stats-row" style="margin-bottom: 20px">
        <div class="card stat">
          <div class="muted" style="margin-bottom:8px;">{{ $t('dashboard.cpu_usage', 'CPU Kullanımı') }}</div>
          <div class="stat-value" :style="{ color: sysStats.cpu.usage > 80 ? '#ef4444' : '#3b82f6' }">
            {{ sysStats.cpu.usage.toFixed(1) }}%
          </div>
          <div class="progress-bar-bg" style="height:6px; background:#e2e8f0; border-radius:3px; overflow:hidden; margin-top:8px;">
            <div class="progress-bar-fill" :style="{ width: sysStats.cpu.usage + '%', background: sysStats.cpu.usage > 80 ? '#ef4444' : '#3b82f6', height:'100%' }"></div>
          </div>
          <div class="text-sm muted" style="margin-top:4px;">{{ $t('dashboard.n_cores', { n: sysStats.cpu.cores }) }}</div>
        </div>
        <div class="card stat">
          <div class="muted" style="margin-bottom:8px;">{{ $t('dashboard.ram_usage', 'RAM Kullanımı') }}</div>
          <div class="stat-value" :style="{ color: sysStats.ram.usage > 80 ? '#ef4444' : '#10b981' }">
            {{ sysStats.ram.usage.toFixed(1) }}%
          </div>
          <div class="progress-bar-bg" style="height:6px; background:#e2e8f0; border-radius:3px; overflow:hidden; margin-top:8px;">
            <div class="progress-bar-fill" :style="{ width: sysStats.ram.usage + '%', background: sysStats.ram.usage > 80 ? '#ef4444' : '#10b981', height:'100%' }"></div>
          </div>
          <div class="text-sm muted" style="margin-top:4px;">{{ (sysStats.ram.used / 1024/1024/1024).toFixed(1) }} GB / {{ (sysStats.ram.total / 1024/1024/1024).toFixed(1) }} GB</div>
        </div>
        <div class="card stat">
          <div class="muted" style="margin-bottom:8px;">{{ $t('dashboard.disk_usage', 'Disk Kullanımı') }}</div>
          <div class="stat-value" :style="{ color: sysStats.disk.usage > 80 ? '#ef4444' : '#8b5cf6' }">
            {{ sysStats.disk.usage.toFixed(1) }}%
          </div>
          <div class="progress-bar-bg" style="height:6px; background:#e2e8f0; border-radius:3px; overflow:hidden; margin-top:8px;">
            <div class="progress-bar-fill" :style="{ width: sysStats.disk.usage + '%', background: sysStats.disk.usage > 80 ? '#ef4444' : '#8b5cf6', height:'100%' }"></div>
          </div>
          <div class="text-sm muted" style="margin-top:4px;">{{ (sysStats.disk.used / 1024/1024/1024).toFixed(1) }} GB / {{ (sysStats.disk.total / 1024/1024/1024).toFixed(1) }} GB</div>
        </div>
      </div>

      <!-- Stat Cards -->
      <div class="row stats-row" style="margin-bottom: 20px">
        <div class="card stat">
          <div class="stat-value">{{ status.site_count ?? '—' }}</div>
          <div class="muted">{{ $t('dashboard.site') }}</div>
        </div>
        <div class="card stat">
          <div class="stat-value" :style="{ color: (status.open_drifts > 0) ? '#ef4444' : '#10b981' }">
            {{ status.open_drifts ?? '0' }}
          </div>
          <div class="muted">{{ $t('dashboard.open_drifts') }}</div>
        </div>
        <div class="card stat">
          <div class="stat-value" style="color: #10b981">{{ $t('dashboard.status_active') }}</div>
          <div class="muted">OpenLiteSpeed</div>
        </div>
        <div class="card stat">
          <div class="stat-value" style="color: #10b981">{{ $t('dashboard.status_ready') }}</div>
          <div class="muted">MariaDB</div>
        </div>
      </div>

      <!-- Metrics Chart -->
      <div class="card metrics-card">
        <div class="metrics-header">
          <h2 style="margin: 0">📈 {{ $t('dashboard.resource_usage_title') }}</h2>
          <select v-model="selectedSiteId" @change="loadMetrics" class="site-select">
            <option v-for="s in sites" :key="s.id" :value="s.id">{{ s.name }}</option>
            <option v-if="!sites.length" value="" disabled>{{ $t('dashboard.no_sites_found') }}</option>
          </select>
        </div>
        <div v-if="loadingMetrics" class="muted text-sm" style="padding: 20px 0;">{{ $t('dashboard.loading') }}</div>
        <div v-else-if="chartData.labels && chartData.labels.length" style="position: relative; height: 300px; width: 100%;">
          <Line :data="chartData" :options="chartOptions" />
        </div>
        <div v-else class="muted text-sm" style="padding: 20px 0;">{{ $t('dashboard.no_metrics_data') }}</div>
      </div>

      <!-- Quick Actions -->
      <div class="card quick-actions-card">
        <h2>{{ $t('dashboard.quick_actions') }}</h2>
        <div class="actions-grid">
          <router-link class="action-btn" to="/sites">
            <span class="action-icon">➕</span>
            <div>
              <strong>{{ $t('dashboard.new_site') }}</strong>
              <p class="muted text-sm">{{ $t('dashboard.site_domain_management') }}</p>
            </div>
          </router-link>
          <router-link class="action-btn" to="/files">
            <span class="action-icon">📁</span>
            <div>
              <strong>{{ $t('dashboard.file_manager') }}</strong>
              <p class="muted text-sm">{{ $t('dashboard.file_manager_desc') }}</p>
            </div>
          </router-link>
          <router-link class="action-btn" to="/logs">
            <span class="action-icon">⚡</span>
            <div>
              <strong>{{ $t('dashboard.live_logs') }}</strong>
              <p class="muted text-sm">{{ $t('dashboard.live_logs_desc') }}</p>
            </div>
          </router-link>
          <router-link class="action-btn" to="/cron">
            <span class="action-icon">⏰</span>
            <div>
              <strong>{{ $t('dashboard.cron_jobs') }}</strong>
              <p class="muted text-sm">{{ $t('dashboard.cron_jobs_desc') }}</p>
            </div>
          </router-link>
          <router-link class="action-btn" to="/security">
            <span class="action-icon">🛡️</span>
            <div>
              <strong>{{ $t('dashboard.security_profile') }}</strong>
              <p class="muted text-sm">{{ $t('dashboard.security_profile_desc') }}</p>
            </div>
          </router-link>
          <router-link class="action-btn" to="/drift">
            <span class="action-icon">🕵️</span>
            <div>
              <strong>{{ $t('dashboard.drift_scan') }}</strong>
              <p class="muted text-sm">{{ $t('dashboard.drift_scan_desc') }}</p>
            </div>
          </router-link>
        </div>
      </div>
    </div>
  </Layout>
</template>

<script setup>
import { onMounted, ref, shallowRef } from 'vue'
import { useI18n } from 'vue-i18n'
import Layout from '../components/Layout.vue'
import { api } from '../api'

// Chart.js imports
import {
  Chart as ChartJS,
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  Title,
  Tooltip,
  Legend,
  Filler
} from 'chart.js'
import { Line } from 'vue-chartjs'

ChartJS.register(
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  Title,
  Tooltip,
  Legend,
  Filler
)

const { t } = useI18n()

const status = ref({})
const health = ref(null)
const sysStats = ref(null)
const error = ref('')
let statsTimer = null

// Metrics states
const sites = ref([])
const selectedSiteId = ref('')
const loadingMetrics = ref(false)
const chartData = shallowRef({})
const chartOptions = {
  responsive: true,
  maintainAspectRatio: false,
  interaction: {
    mode: 'index',
    intersect: false,
  },
  scales: {
    y: {
      type: 'linear',
      display: true,
      position: 'left',
      title: { display: true, text: t('dashboard.cpu_axis_label') },
      min: 0,
      max: 100
    },
    y1: {
      type: 'linear',
      display: true,
      position: 'right',
      title: { display: true, text: t('dashboard.ram_axis_label') },
      min: 0,
      grid: { drawOnChartArea: false }
    }
  },
  plugins: {
    legend: { position: 'top' },
    tooltip: {
      callbacks: {
        label: function(context) {
          let label = context.dataset.label || '';
          if (label) { label += ': '; }
          if (context.parsed.y !== null) {
            label += context.parsed.y.toFixed(2);
            if (context.dataset.yAxisID === 'y') label += '%';
            else label += ' MB';
          }
          return label;
        }
      }
    }
  }
}

function formatCheckName(name) {
  if (name === 'sqlite') return t('dashboard.check_sqlite')
  if (name === 'mariadb') return t('dashboard.check_mariadb')
  if (name === 'ols') return t('dashboard.check_ols')
  if (name === 'ssl_certs') return t('dashboard.check_ssl_certs')
  return name
}

async function loadMetrics() {
  if (!selectedSiteId.value) return
  loadingMetrics.value = true
  try {
    const metrics = await api(`/sites/${selectedSiteId.value}/metrics?hours=24`)
    if (!metrics || metrics.length === 0) {
      chartData.value = {}
      loadingMetrics.value = false
      return
    }

    const labels = []
    const cpuData = []
    const memData = []

    metrics.forEach(m => {
      // Parse TS to local time
      const date = new Date(m.ts)
      labels.push(date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }))
      cpuData.push(m.cpu_pct)
      memData.push(m.mem_mb)
    })

    chartData.value = {
      labels,
      datasets: [
        {
          label: t('dashboard.cpu_usage'),
          data: cpuData,
          borderColor: '#3b82f6',
          backgroundColor: 'rgba(59, 130, 246, 0.1)',
          yAxisID: 'y',
          tension: 0.4,
          fill: true,
          pointRadius: 2,
          pointHoverRadius: 5
        },
        {
          label: t('dashboard.ram_usage'),
          data: memData,
          borderColor: '#10b981',
          backgroundColor: 'transparent',
          yAxisID: 'y1',
          tension: 0.4,
          pointRadius: 2,
          pointHoverRadius: 5
        }
      ]
    }
  } catch (e) {
    console.error('Metrics fetch error:', e)
  } finally {
    loadingMetrics.value = false
  }
}

async function loadSysStats() {
  try {
    const res = await fetch('/api/v1/system/stats')
    if (res.ok) sysStats.value = await res.json()
  } catch (e) {
    console.error('System stats error:', e)
  }
}

async function loadData() {
  error.value = ''
  try {
    status.value = await api('/status')
  } catch (e) {
    error.value = e.message
  }

  try {
    const res = await fetch('/api/v1/health')
    if (res.ok || res.status === 503) {
      health.value = await res.json()
    }
  } catch (e) {
    console.error('Health check error:', e)
  }

  try {
    sites.value = await api('/sites')
    if (sites.value.length > 0 && !selectedSiteId.value) {
      selectedSiteId.value = sites.value[0].id
      await loadMetrics()
    } else if (selectedSiteId.value) {
      await loadMetrics()
    }
  } catch (e) {
    console.error('Sites fetch error:', e)
  }
}

onMounted(() => {
  loadData()
  loadSysStats()
  statsTimer = setInterval(loadSysStats, 3000)
})

import { onUnmounted } from 'vue'
onUnmounted(() => {
  if (statsTimer) clearInterval(statsTimer)
})
</script>

<style scoped>
.dashboard-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 20px; }
.health-banner { margin-bottom: 20px; border-left: 4px solid #10b981; }
.health-banner.health-warn { border-left-color: #f59e0b; }
.health-summary { display: flex; align-items: center; gap: 12px; margin-bottom: 12px; }
.health-indicator { width: 12px; height: 12px; border-radius: 50%; display: inline-block; }
.health-indicator.status-green { background: #10b981; box-shadow: 0 0 8px #10b981; }
.health-indicator.status-yellow { background: #f59e0b; box-shadow: 0 0 8px #f59e0b; }
.health-checks-grid { display: flex; flex-wrap: wrap; gap: 8px; }
.check-pill { display: flex; align-items: center; gap: 6px; padding: 4px 10px; border-radius: 20px; font-size: 12px; font-weight: 500; }
.pill-ok { background: #dcfce7; color: #166534; }
.pill-warn { background: #fef3c7; color: #92400e; }
.pill-ok .dot { width: 6px; height: 6px; border-radius: 50%; background: #16a34a; }
.pill-warn .dot { width: 6px; height: 6px; border-radius: 50%; background: #d97706; }
.stats-row { display: grid; grid-template-columns: repeat(auto-fit, minmax(180px, 1fr)); gap: 16px; align-items: stretch; }
.stat { text-align: center; padding: 20px; display: flex; flex-direction: column; justify-content: space-between; height: 100%; }
.stat-value { font-size: 32px; font-weight: 700; color: var(--primary); margin-bottom: 4px; }

.metrics-card { margin-bottom: 20px; }
.metrics-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 16px; }
.site-select { max-width: 200px; padding: 6px 10px; }

.actions-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(240px, 1fr)); gap: 16px; margin-top: 16px; }
.action-btn { display: flex; align-items: center; gap: 14px; padding: 14px; border-radius: 8px; border: 1px solid var(--border-color, #e2e8f0); text-decoration: none; color: inherit; transition: all 0.2s; background: var(--bg-card, #fff); }
.action-btn:hover { border-color: #3b82f6; transform: translateY(-1px); box-shadow: 0 4px 6px -1px rgba(0,0,0,0.05); }
.action-icon { font-size: 24px; }
</style>
