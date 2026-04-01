<template>
  <div class="space-y-6">
    <div v-if="error" class="aura-card border-red-500/30 bg-red-500/5 text-red-300">
      {{ error }}
    </div>

    <div class="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-4 gap-6">
      <div
        v-for="card in serverStatusCards"
        :key="card.name"
        class="aura-card border border-panel-border/80 bg-panel-card/95 min-h-[148px]"
      >
        <div class="flex items-center justify-between mb-4">
          <div class="text-sm text-gray-400">{{ card.name }}</div>
          <component :is="card.icon" class="w-5 h-5" :class="card.iconColor" />
        </div>
        <div class="text-4xl font-bold text-white tracking-tight">{{ card.value }}</div>
        <div class="mt-3 text-xs text-gray-500">{{ card.detail }}</div>
        <div v-if="card.subdetail" class="mt-1 text-xs text-gray-500">{{ card.subdetail }}</div>
      </div>
    </div>

    <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6">
      <div v-for="stat in stats" :key="stat.name" class="aura-card hover:translate-y-[-2px]">
        <div class="flex items-center justify-between mb-4">
          <div class="text-gray-400 font-medium">{{ stat.name }}</div>
          <component :is="stat.icon" class="w-5 h-5" :class="stat.iconColor" />
        </div>
        <div class="text-3xl font-bold text-white">{{ stat.value }}</div>
        <div class="mt-2 text-sm" :class="stat.trend > 0 ? 'text-brand-400' : 'text-gray-500'">
          {{ t('dashboard.trend_week', { sign: stat.trend > 0 ? '+' : '', trend: stat.trend }) }}
        </div>
      </div>
    </div>

    <div class="grid grid-cols-1 lg:grid-cols-3 gap-6">
      <div class="lg:col-span-2 aura-card min-h-[400px]">
        <div class="flex items-center justify-between mb-6">
          <h2 class="text-lg font-semibold text-white">{{ t('dashboard.system_load_map') }}</h2>
          <button class="btn-secondary text-sm px-3 py-1.5" @click="loadDashboard" :disabled="loading">
            {{ loading ? t('dashboard.refreshing') : t('dashboard.refresh') }}
          </button>
        </div>
        <div class="h-[300px] border border-panel-border rounded-xl bg-panel-darker/40 p-4">
          <div class="mb-3 flex items-center justify-between text-xs text-gray-400">
            <div class="flex flex-wrap items-center gap-3">
              <span class="inline-flex items-center gap-1.5 rounded-md bg-sky-500/10 px-2 py-1 text-sky-300">
                <span class="h-2 w-2 rounded-full bg-sky-400"></span>
                CPU {{ latestCpuUsage }}%
              </span>
              <span class="inline-flex items-center gap-1.5 rounded-md bg-emerald-500/10 px-2 py-1 text-emerald-300">
                <span class="h-2 w-2 rounded-full bg-emerald-400"></span>
                RAM {{ latestRamUsage }}%
              </span>
              <span class="inline-flex items-center gap-1.5 rounded-md bg-amber-500/10 px-2 py-1 text-amber-300">
                <span class="h-2 w-2 rounded-full bg-amber-400"></span>
                LOAD {{ latestLoadPercent }}%
              </span>
            </div>
            <span class="text-[11px] text-gray-500">{{ chartPointCount }} pts</span>
          </div>

          <div class="relative h-[220px] overflow-hidden rounded-lg border border-panel-border/60 bg-panel-card/20">
            <svg class="h-full w-full" viewBox="0 0 100 100" preserveAspectRatio="none" role="img" aria-label="System load trend chart">
              <line x1="0" y1="20" x2="100" y2="20" class="stroke-panel-border/50" stroke-width="0.4" />
              <line x1="0" y1="40" x2="100" y2="40" class="stroke-panel-border/40" stroke-width="0.35" />
              <line x1="0" y1="60" x2="100" y2="60" class="stroke-panel-border/40" stroke-width="0.35" />
              <line x1="0" y1="80" x2="100" y2="80" class="stroke-panel-border/40" stroke-width="0.35" />
              <path v-if="cpuAreaPath" :d="cpuAreaPath" fill="rgba(56, 189, 248, 0.12)" />
              <path v-if="cpuLinePath" :d="cpuLinePath" fill="none" stroke="rgb(56, 189, 248)" stroke-width="1.1" stroke-linecap="round" />
              <path v-if="ramLinePath" :d="ramLinePath" fill="none" stroke="rgb(16, 185, 129)" stroke-width="1.1" stroke-linecap="round" />
              <path v-if="loadLinePath" :d="loadLinePath" fill="none" stroke="rgb(245, 158, 11)" stroke-width="1.1" stroke-linecap="round" />
            </svg>

            <div v-if="chartPointCount === 0" class="absolute inset-0 flex items-center justify-center">
              <div class="text-center">
                <Activity class="mx-auto mb-3 h-10 w-10 text-brand-500 opacity-60" :class="loading ? 'animate-pulse' : ''" />
                <p class="text-sm text-gray-400">{{ t('dashboard.refreshing') }}</p>
              </div>
            </div>
          </div>

          <div class="mt-3 text-center">
            <p class="text-gray-400 font-medium">{{ t('dashboard.uptime', { value: uptimeHuman }) }}</p>
            <p class="text-sm text-gray-500 mt-1">{{ t('dashboard.load_avg', { value: loadAvg }) }}</p>
          </div>
        </div>
      </div>

      <div class="aura-card">
        <h2 class="text-lg font-semibold text-white mb-6">{{ t('dashboard.sre_log') }}</h2>
        <div class="space-y-4" v-if="logs.length">
          <div v-for="log in logs" :key="log.id" class="flex gap-4">
            <div class="mt-1 relative flex items-center justify-center">
              <div class="w-2 h-2 rounded-full ring-4 ring-panel-darker" :class="log.color"></div>
              <div class="absolute top-3 bottom-[-16px] w-[1px] bg-panel-border last:hidden"></div>
            </div>
            <div>
              <p class="text-sm font-medium text-gray-200">{{ log.title }}</p>
              <p class="text-xs text-gray-500 mt-0.5">{{ log.time }}</p>
            </div>
          </div>
        </div>
        <p v-else class="text-sm text-gray-500">{{ t('dashboard.empty_logs') }}</p>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import { useI18n } from 'vue-i18n'
import { Server, Globe, Database, ShieldCheck, Activity, Cpu, MemoryStick, HardDrive, Clock } from 'lucide-vue-next'
import api from '../services/api'

const { t } = useI18n({ useScope: 'global' })

const loading = ref(false)
const error = ref('')
const uptimeHuman = ref(t('dashboard.na'))
const loadAvg = ref(t('dashboard.na'))
const chartSeriesLimit = 48
const cpuHistory = ref([])
const ramHistory = ref([])
const loadHistory = ref([])
let metricsPollTimer = null
const serverStatusCards = ref([
  { name: t('server_status.cpu'), value: '0%', detail: t('dashboard.na'), subdetail: '', icon: Cpu, iconColor: 'text-blue-400' },
  { name: t('server_status.ram'), value: '0%', detail: t('dashboard.na'), subdetail: '', icon: MemoryStick, iconColor: 'text-green-400' },
  { name: t('server_status.disk'), value: '0%', detail: t('dashboard.na'), subdetail: '', icon: HardDrive, iconColor: 'text-fuchsia-400' },
  { name: t('server_status.uptime'), value: t('dashboard.na'), detail: t('dashboard.na'), subdetail: '', icon: Clock, iconColor: 'text-orange-400' },
])

const stats = ref([
  { name: t('dashboard.stats.active_websites'), value: '0', icon: Globe, iconColor: 'text-blue-400', trend: 0 },
  { name: t('dashboard.stats.server_uptime'), value: t('dashboard.na'), icon: Server, iconColor: 'text-brand-400', trend: 0 },
  { name: t('dashboard.stats.databases'), value: '0', icon: Database, iconColor: 'text-orange-400', trend: 0 },
  { name: t('dashboard.stats.threats_blocked'), value: '0', icon: ShieldCheck, iconColor: 'text-red-400', trend: 0 },
])

const logs = ref([])

const chartPointCount = computed(() =>
  Math.max(cpuHistory.value.length, ramHistory.value.length, loadHistory.value.length),
)
const latestCpuUsage = computed(() =>
  cpuHistory.value.length ? Math.round(cpuHistory.value[cpuHistory.value.length - 1]) : 0,
)
const latestRamUsage = computed(() =>
  ramHistory.value.length ? Math.round(ramHistory.value[ramHistory.value.length - 1]) : 0,
)
const latestLoadPercent = computed(() =>
  loadHistory.value.length ? Math.round(loadHistory.value[loadHistory.value.length - 1]) : 0,
)

function clampPercent(value) {
  const numeric = Number(value)
  if (!Number.isFinite(numeric)) return 0
  return Math.max(0, Math.min(100, numeric))
}

function parseLoadAvgPercent(rawLoadAvg, cpuCores) {
  const firstValue = String(rawLoadAvg || '').trim().split(/\s+/)[0]
  const loadValue = Number.parseFloat(firstValue)
  if (!Number.isFinite(loadValue)) return 0
  const cores = Math.max(1, Number(cpuCores || 1))
  return clampPercent((loadValue / cores) * 100)
}

function pushSeriesPoint(seriesRef, nextValue) {
  const next = [...seriesRef.value, clampPercent(nextValue)]
  if (next.length > chartSeriesLimit) {
    next.shift()
  }
  seriesRef.value = next
}

function toChartY(value) {
  return 95 - (clampPercent(value) / 100) * 90
}

function buildLinePath(series) {
  if (!Array.isArray(series) || series.length === 0) return ''
  const denominator = Math.max(1, series.length - 1)
  return series
    .map((value, index) => {
      const x = 2 + (index / denominator) * 96
      const y = toChartY(value)
      return `${index === 0 ? 'M' : 'L'} ${x.toFixed(2)} ${y.toFixed(2)}`
    })
    .join(' ')
}

function buildAreaPath(series) {
  if (!Array.isArray(series) || series.length === 0) return ''
  const denominator = Math.max(1, series.length - 1)
  const firstX = 2
  const lastX = 2 + (Math.max(0, series.length - 1) / denominator) * 96
  const segments = series
    .map((value, index) => {
      const x = 2 + (index / denominator) * 96
      const y = toChartY(value)
      return `${index === 0 ? 'M' : 'L'} ${x.toFixed(2)} ${y.toFixed(2)}`
    })
    .join(' ')

  return `M ${firstX.toFixed(2)} 95 ${segments.slice(1)} L ${lastX.toFixed(2)} 95 Z`
}

const cpuLinePath = computed(() => buildLinePath(cpuHistory.value))
const ramLinePath = computed(() => buildLinePath(ramHistory.value))
const loadLinePath = computed(() => buildLinePath(loadHistory.value))
const cpuAreaPath = computed(() => buildAreaPath(cpuHistory.value))

function summarizeTime(value) {
  if (!value) return t('dashboard.na')
  const text = String(value)
  return text.length > 28 ? `${text.slice(0, 28)}...` : text
}

function summarizeLine(value, max = 42) {
  if (!value) return t('dashboard.na')
  const text = String(value)
  return text.length > max ? `${text.slice(0, max)}...` : text
}

function updateLiveMetrics(metrics = {}) {
  uptimeHuman.value = metrics.uptime_human || t('dashboard.na')
  loadAvg.value = metrics.load_avg || t('dashboard.na')

  pushSeriesPoint(cpuHistory, metrics.cpu_usage || 0)
  pushSeriesPoint(ramHistory, metrics.ram_usage || 0)
  pushSeriesPoint(loadHistory, parseLoadAvgPercent(metrics.load_avg, metrics.cpu_cores))
}

async function pollLoadMetrics() {
  try {
    const metricsRes = await api.get('/status/metrics')
    const metrics = metricsRes.data?.data || {}
    updateLiveMetrics(metrics)
  } catch {
    // Polling is best-effort; dashboard refresh button remains the authoritative fallback.
  }
}

async function loadDashboard() {
  loading.value = true
  error.value = ''

  try {
    const [
      vhostsRes,
      mariaRes,
      pgRes,
      ebpfRes,
      metricsRes,
      servicesRes,
    ] = await Promise.all([
      api.get('/vhost/list'),
      api.get('/db/mariadb/list'),
      api.get('/db/postgres/list'),
      api.get('/security/ebpf/events'),
      api.get('/status/metrics'),
      api.get('/status/services'),
    ])

    const websites = Array.isArray(vhostsRes.data?.data) ? vhostsRes.data.data : []
    const mariaDbs = Array.isArray(mariaRes.data?.data) ? mariaRes.data.data : []
    const pgDbs = Array.isArray(pgRes.data?.data) ? pgRes.data.data : []
    const ebpfEvents = Array.isArray(ebpfRes.data?.data) ? ebpfRes.data.data : []
    const metrics = metricsRes.data?.data || {}
    const services = Array.isArray(servicesRes.data?.data) ? servicesRes.data.data : []

    updateLiveMetrics(metrics)

    const runningServices = services.filter(s => String(s.status).toLowerCase() === 'running').length
    const uptimeDays = Math.floor(Number(metrics.uptime_seconds || 0) / 86400)

    stats.value = [
      { name: t('dashboard.stats.active_websites'), value: String(websites.length), icon: Globe, iconColor: 'text-blue-400', trend: 0 },
      { name: t('dashboard.stats.server_uptime'), value: summarizeTime(metrics.uptime_human), icon: Server, iconColor: 'text-brand-400', trend: 0 },
      { name: t('dashboard.stats.databases'), value: String(mariaDbs.length + pgDbs.length), icon: Database, iconColor: 'text-orange-400', trend: 0 },
      { name: t('dashboard.stats.threats_blocked'), value: String(ebpfEvents.length), icon: ShieldCheck, iconColor: 'text-red-400', trend: 0 },
    ]
    serverStatusCards.value = [
      {
        name: t('server_status.cpu'),
        value: `${Math.round(Number(metrics.cpu_usage || 0))}%`,
        detail: summarizeLine(`${metrics.cpu_cores || 0} core / ${metrics.cpu_model || t('dashboard.na')}`),
        subdetail: '',
        icon: Cpu,
        iconColor: 'text-blue-400',
      },
      {
        name: t('server_status.ram'),
        value: `${Math.round(Number(metrics.ram_usage || 0))}%`,
        detail: summarizeLine(`${metrics.ram_used || t('dashboard.na')} / ${metrics.ram_total || t('dashboard.na')}`),
        subdetail: '',
        icon: MemoryStick,
        iconColor: 'text-green-400',
      },
      {
        name: t('server_status.disk'),
        value: `${Math.round(Number(metrics.disk_usage || 0))}%`,
        detail: summarizeLine(`${metrics.disk_used || t('dashboard.na')} / ${metrics.disk_total || t('dashboard.na')}`),
        subdetail: '',
        icon: HardDrive,
        iconColor: 'text-fuchsia-400',
      },
      {
        name: t('server_status.uptime'),
        value: `${uptimeDays}d`,
        detail: summarizeLine(metrics.uptime_human || t('dashboard.na')),
        subdetail: t('server_status.load_avg', { value: metrics.load_avg || t('dashboard.na') }),
        icon: Clock,
        iconColor: 'text-orange-400',
      },
    ]

    const serviceLog = services.slice(0, 2).map((service, index) => ({
      id: `svc-${index}`,
      title: `${service.name}: ${service.status}`,
      time: t('dashboard.service_check'),
      color: String(service.status).toLowerCase() === 'running' ? 'bg-brand-400' : 'bg-yellow-400',
    }))

    const ebpfLog = ebpfEvents.slice(0, 3).map((entry, index) => ({
      id: `evt-${index}`,
      title: String(entry),
      time: t('dashboard.security_event'),
      color: 'bg-red-400',
    }))

    if (!ebpfLog.length && runningServices > 0) {
      serviceLog.unshift({
        id: 'svc-summary',
        title: t('dashboard.running_services', { count: runningServices }),
        time: t('dashboard.runtime_snapshot'),
        color: 'bg-blue-400',
      })
    }

    logs.value = [...ebpfLog, ...serviceLog].slice(0, 5)

  } catch (err) {
    error.value = err?.response?.data?.message || err?.message || t('dashboard.load_failed')
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  loadDashboard()
  metricsPollTimer = setInterval(() => {
    pollLoadMetrics()
  }, 5000)
})

onBeforeUnmount(() => {
  if (metricsPollTimer) {
    clearInterval(metricsPollTimer)
    metricsPollTimer = null
  }
})
</script>
