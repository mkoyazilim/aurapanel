<template>
  <div class="space-y-6">
    <div v-if="error" class="aura-card border-red-500/30 bg-red-500/5 text-red-300">
      {{ error }}
    </div>

    <div class="aura-card border border-panel-border/80 bg-panel-card/95">
      <div class="flex flex-col gap-4 xl:flex-row xl:items-start xl:justify-between">
        <div>
          <div class="flex items-center gap-3">
            <ArrowUpCircle class="h-6 w-6" :class="updateStatus.update_available ? 'text-amber-400' : 'text-brand-400'" />
            <div>
              <p class="text-xs font-semibold uppercase tracking-[0.2em] text-gray-500">Release Channel</p>
              <h2 class="text-xl font-bold text-white">{{ updateStatus.current_version }}</h2>
            </div>
          </div>
          <p class="mt-3 text-sm text-gray-400">
            {{ updateStatus.update_available ? 'A newer GitHub release is available for this panel.' : 'This server is currently running the configured panel version.' }}
          </p>
        </div>

        <div class="inline-flex items-center rounded-full px-4 py-2 text-sm font-semibold"
          :class="updateStatus.update_available ? 'bg-amber-500/15 text-amber-300 border border-amber-500/30' : 'bg-brand-500/10 text-brand-300 border border-brand-500/20'">
          {{ updateStatus.update_available ? 'Update Available' : 'Up to Date' }}
        </div>
      </div>

      <div class="mt-5 grid grid-cols-1 gap-4 md:grid-cols-3">
        <div class="rounded-xl border border-panel-border/70 bg-panel-darker/60 p-4">
          <p class="text-xs uppercase tracking-[0.18em] text-gray-500">Current Version</p>
          <p class="mt-2 text-lg font-semibold text-white">{{ updateStatus.current_version }}</p>
        </div>
        <div class="rounded-xl border border-panel-border/70 bg-panel-darker/60 p-4">
          <p class="text-xs uppercase tracking-[0.18em] text-gray-500">Latest Release</p>
          <p class="mt-2 text-lg font-semibold text-white">{{ updateStatus.latest_version || 'Not checked yet' }}</p>
          <p v-if="updateStatus.published_at" class="mt-1 text-xs text-gray-500">{{ formatReleaseDate(updateStatus.published_at) }}</p>
        </div>
        <div class="rounded-xl border border-panel-border/70 bg-panel-darker/60 p-4">
          <p class="text-xs uppercase tracking-[0.18em] text-gray-500">Source</p>
          <p class="mt-2 text-lg font-semibold text-white">{{ updateStatus.source || 'GitHub Releases' }}</p>
          <p v-if="updateStatus.checked_at" class="mt-1 text-xs text-gray-500">Last checked: {{ formatReleaseDate(updateStatus.checked_at) }}</p>
        </div>
      </div>

      <div v-if="updateStatus.release_notes || updateStatus.error || updateStatus.release_url" class="mt-4 rounded-xl border border-panel-border/60 bg-black/10 p-4">
        <p v-if="updateStatus.release_notes" class="text-sm text-gray-300">{{ updateStatus.release_notes }}</p>
        <p v-if="updateStatus.error" class="text-sm text-yellow-300">{{ updateStatus.error }}</p>
        <a
          v-if="updateStatus.release_url"
          :href="updateStatus.release_url"
          target="_blank"
          rel="noreferrer"
          class="mt-3 inline-flex items-center text-sm font-medium text-brand-300 hover:text-brand-200 transition"
        >
          View GitHub release
        </a>
      </div>
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
        <div class="flex items-center justify-center h-[300px] border-2 border-dashed border-panel-border rounded-xl">
          <div class="text-center">
            <Activity class="w-12 h-12 text-brand-500 mx-auto mb-3 opacity-50" :class="loading ? 'animate-pulse' : ''" />
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
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { Server, Globe, Database, ShieldCheck, Activity, Cpu, MemoryStick, HardDrive, Clock, ArrowUpCircle } from 'lucide-vue-next'
import api from '../services/api'

const { t } = useI18n({ useScope: 'global' })

const loading = ref(false)
const error = ref('')
const uptimeHuman = ref(t('dashboard.na'))
const loadAvg = ref(t('dashboard.na'))
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
const updateStatus = ref({
  current_version: 'Aura Panel V1',
  latest_version: '',
  update_available: false,
  release_name: '',
  release_url: '',
  release_notes: '',
  published_at: '',
  source: 'GitHub Releases',
  checked_at: '',
  error: '',
})

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

function formatReleaseDate(value) {
  if (!value) return t('dashboard.na')
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) {
    return value
  }
  return date.toLocaleString()
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

    uptimeHuman.value = metrics.uptime_human || t('dashboard.na')
    loadAvg.value = metrics.load_avg || t('dashboard.na')

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

    try {
      const updateRes = await api.get('/status/update', {
        headers: {
          'X-Aura-Silent-Error': '1',
        },
      })
      updateStatus.value = {
        ...updateStatus.value,
        ...(updateRes.data?.data || {}),
      }
    } catch {
      updateStatus.value = {
        ...updateStatus.value,
        current_version: updateStatus.value.current_version || 'Aura Panel V1',
        error: 'Unable to check GitHub releases right now.',
      }
    }
  } catch (err) {
    error.value = err?.response?.data?.message || err?.message || t('dashboard.load_failed')
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  loadDashboard()
})
</script>
