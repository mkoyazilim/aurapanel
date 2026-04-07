<template>
  <div class="space-y-6">
    <div class="flex items-center justify-between">
      <div>
        <h1 class="text-2xl font-bold text-white flex items-center gap-3">
          <Activity class="w-7 h-7 text-orange-400" />
          {{ t('server_status.title') }}
        </h1>
        <p class="text-gray-400 mt-1">{{ t('server_status.subtitle') }}</p>
      </div>
      <button
        @click="refreshAll"
        class="px-4 py-2 bg-panel-hover text-gray-300 rounded-lg text-sm hover:bg-gray-600 transition flex items-center"
      >
        <Loader2 v-if="loadingMetrics || loadingServices || loadingProcesses || loadingWatchdog || savingWatchdog" class="w-4 h-4 animate-spin mr-2" />
        <span>{{ t('server_status.refresh') }}</span>
      </button>
    </div>

    <div class="grid grid-cols-1 md:grid-cols-4 gap-4">
      <div class="bg-panel-card border border-panel-border rounded-xl p-5">
        <div class="flex items-center justify-between mb-3">
          <p class="text-sm text-gray-400">{{ t('server_status.cpu') }}</p>
          <Cpu class="w-5 h-5 text-blue-400" />
        </div>
        <p class="text-3xl font-bold text-white">{{ metrics.cpu }}%</p>
        <p class="text-xs text-gray-500 mt-2">{{ metrics.cpuCores }} core / {{ metrics.cpuModel }}</p>
      </div>

      <div class="bg-panel-card border border-panel-border rounded-xl p-5">
        <div class="flex items-center justify-between mb-3">
          <p class="text-sm text-gray-400">{{ t('server_status.ram') }}</p>
          <MemoryStick class="w-5 h-5 text-green-400" />
        </div>
        <p class="text-3xl font-bold text-white">{{ metrics.ram }}%</p>
        <p class="text-xs text-gray-500 mt-2">{{ metrics.ramUsed }} / {{ metrics.ramTotal }}</p>
      </div>

      <div class="bg-panel-card border border-panel-border rounded-xl p-5">
        <div class="flex items-center justify-between mb-3">
          <p class="text-sm text-gray-400">{{ t('server_status.disk') }}</p>
          <HardDrive class="w-5 h-5 text-purple-400" />
        </div>
        <p class="text-3xl font-bold text-white">{{ metrics.disk }}%</p>
        <p class="text-xs text-gray-500 mt-2">{{ metrics.diskUsed }} / {{ metrics.diskTotal }}</p>
      </div>

      <div class="bg-panel-card border border-panel-border rounded-xl p-5">
        <div class="flex items-center justify-between mb-3">
          <p class="text-sm text-gray-400">{{ t('server_status.uptime') }}</p>
          <Clock class="w-5 h-5 text-orange-400" />
        </div>
        <p class="text-3xl font-bold text-white">{{ metrics.uptimeDays }}d</p>
        <p class="text-xs text-gray-500 mt-2">{{ metrics.uptimeFull }}</p>
        <p class="text-xs text-gray-500">{{ t('server_status.load_avg', { value: metrics.loadAvg }) }}</p>
      </div>
    </div>

    <div class="border-b border-panel-border">
      <nav class="flex gap-6">
        <button
          @click="tab = 'services'"
          :class="['pb-3 text-sm font-medium transition', tab === 'services' ? 'text-orange-400 border-b-2 border-orange-400' : 'text-gray-400 hover:text-white']"
        >
          {{ t('server_status.services_tab') }}
        </button>
        <button
          v-if="canViewProcesses"
          @click="tab = 'processes'"
          :class="['pb-3 text-sm font-medium transition', tab === 'processes' ? 'text-orange-400 border-b-2 border-orange-400' : 'text-gray-400 hover:text-white']"
        >
          {{ t('server_status.processes_tab') }}
        </button>
        <button
          v-if="canManageWatchdog"
          @click="tab = 'watchdog'"
          :class="['pb-3 text-sm font-medium transition', tab === 'watchdog' ? 'text-orange-400 border-b-2 border-orange-400' : 'text-gray-400 hover:text-white']"
        >
          {{ t('server_status.watchdog_tab') }}
        </button>
      </nav>
    </div>

    <div v-if="tab === 'services'" class="grid grid-cols-1 md:grid-cols-2 gap-3">
      <div v-if="loadingServices" class="col-span-1 md:col-span-2 text-center py-6 text-gray-500">{{ t('common.loading') }}</div>
      <div v-for="s in services" :key="s.name" class="bg-panel-card border border-panel-border rounded-xl p-4 flex items-center justify-between">
        <div>
          <div class="flex items-center gap-2">
            <p class="text-white font-medium text-sm">{{ s.name }}</p>
            <span
              class="px-2 py-0.5 rounded-full text-[11px] font-medium"
              :class="serviceStatusBadgeClass(s.status)"
            >
              {{ serviceStatusLabel(s.status) }}
            </span>
          </div>
          <p class="text-gray-500 text-xs">{{ s.desc }}</p>
        </div>
        <div v-if="canControlServices" class="flex gap-2">
          <button
            v-if="isServiceRunning(s.status)"
            @click="controlService(s.name, 'stop')"
            class="px-2 py-1 bg-red-600/20 text-red-400 rounded text-xs hover:bg-red-600/40 transition"
          >
            {{ t('server_status.stop') }}
          </button>
          <button
            v-else
            @click="controlService(s.name, 'start')"
            class="px-2 py-1 bg-green-600/20 text-green-400 rounded text-xs hover:bg-green-600/40 transition"
          >
            {{ t('server_status.start') }}
          </button>
          <button
            @click="controlService(s.name, 'restart')"
            class="px-2 py-1 bg-blue-600/20 text-blue-400 rounded text-xs hover:bg-blue-600/40 transition"
          >
            {{ t('server_status.restart') }}
          </button>
        </div>
      </div>
    </div>

    <div v-if="tab === 'processes' && canViewProcesses" class="bg-panel-card border border-panel-border rounded-xl overflow-hidden">
      <div v-if="loadingProcesses" class="p-6 text-center text-gray-500">{{ t('common.loading') }}</div>
      <table v-else class="w-full text-sm">
        <thead>
          <tr class="text-gray-400 border-b border-panel-border">
            <th class="text-left px-4 py-3">PID</th>
            <th class="text-left px-4 py-3">{{ t('server_status.user') }}</th>
            <th class="text-left px-4 py-3">{{ t('server_status.cpu') }}</th>
            <th class="text-left px-4 py-3">{{ t('server_status.ram') }}</th>
            <th class="text-left px-4 py-3">{{ t('server_status.command') }}</th>
            <th class="text-right px-4 py-3">{{ t('server_status.action') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="p in processes" :key="p.pid" class="border-b border-panel-border/50 hover:bg-white/[0.02] transition">
            <td class="px-4 py-2.5 text-gray-400 font-mono text-xs">{{ p.pid }}</td>
            <td class="px-4 py-2.5 text-gray-300 text-xs">{{ p.user }}</td>
            <td class="px-4 py-2.5 text-gray-300 text-xs">{{ p.cpu }}%</td>
            <td class="px-4 py-2.5 text-gray-300 text-xs">{{ p.mem }}%</td>
            <td class="px-4 py-2.5 text-white font-mono text-xs truncate max-w-[260px]">{{ p.command }}</td>
            <td class="px-4 py-2.5 text-right">
              <button v-if="canControlServices" @click="killProcess(p.pid)" class="px-2 py-1 bg-red-600/20 text-red-400 rounded text-xs hover:bg-red-600/40 transition">{{ t('server_status.kill_process') }}</button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <div v-if="tab === 'watchdog' && canManageWatchdog" class="space-y-4">
      <div class="bg-panel-card border border-panel-border rounded-xl p-4">
        <div class="flex flex-col md:flex-row md:items-center md:justify-between gap-3">
          <div>
            <p class="text-white font-medium">{{ t('server_status.watchdog_title') }}</p>
            <p class="text-gray-400 text-xs">{{ t('server_status.watchdog_subtitle') }}</p>
          </div>
          <div class="flex flex-wrap items-center gap-2">
            <span
              class="px-2 py-1 rounded-full text-xs font-medium border"
              :class="watchdogEnabled ? 'bg-green-500/20 text-green-300 border-green-500/30' : 'bg-gray-500/20 text-gray-300 border-gray-500/30'"
            >
              {{ watchdogEnabled ? t('server_status.watchdog_enabled') : t('server_status.watchdog_disabled') }}
            </span>
            <button
              @click="toggleWatchdog(!watchdogEnabled)"
              :disabled="savingWatchdog || loadingWatchdog"
              class="px-3 py-1.5 rounded-lg text-xs transition"
              :class="watchdogEnabled ? 'bg-red-600/20 text-red-300 hover:bg-red-600/40' : 'bg-green-600/20 text-green-300 hover:bg-green-600/40'"
            >
              {{ watchdogEnabled ? t('server_status.watchdog_disable') : t('server_status.watchdog_enable') }}
            </button>
            <button
              @click="showWatchdogLogs = !showWatchdogLogs"
              class="px-3 py-1.5 rounded-lg text-xs transition bg-panel-hover text-gray-200 hover:bg-gray-600"
            >
              {{ showWatchdogLogs ? t('server_status.watchdog_logs_hide') : t('server_status.watchdog_logs_show') }}
            </button>
          </div>
        </div>
      </div>

      <div class="grid grid-cols-1 md:grid-cols-3 gap-3">
        <div class="bg-panel-card border border-panel-border rounded-xl p-4">
          <p class="text-gray-400 text-xs uppercase">{{ t('server_status.watchdog_services') }}</p>
          <p class="text-white text-2xl font-bold mt-1">{{ watchdogSummary.service_count }}</p>
        </div>
        <div class="bg-panel-card border border-panel-border rounded-xl p-4">
          <p class="text-gray-400 text-xs uppercase">{{ t('server_status.watchdog_summary_unhealthy') }}</p>
          <p class="text-red-300 text-2xl font-bold mt-1">{{ watchdogSummary.unhealthy_count }}</p>
        </div>
        <div class="bg-panel-card border border-panel-border rounded-xl p-4">
          <p class="text-gray-400 text-xs uppercase">{{ t('server_status.watchdog_logs') }}</p>
          <p class="text-white text-2xl font-bold mt-1">{{ watchdogSummary.log_count }}</p>
        </div>
      </div>

      <div class="bg-panel-card border border-panel-border rounded-xl p-4 space-y-4">
        <p class="text-white text-sm font-medium">{{ t('server_status.watchdog_settings') }}</p>
        <div class="grid grid-cols-1 md:grid-cols-4 gap-3">
          <label class="space-y-1">
            <span class="text-xs text-gray-400">{{ t('server_status.watchdog_interval') }}</span>
            <input v-model.number="watchdogForm.interval_seconds" type="number" min="10" max="300" class="w-full px-3 py-2 bg-black/20 border border-panel-border rounded-lg text-sm text-white" />
          </label>
          <label class="space-y-1">
            <span class="text-xs text-gray-400">{{ t('server_status.watchdog_threshold') }}</span>
            <input v-model.number="watchdogForm.failure_threshold" type="number" min="1" max="12" class="w-full px-3 py-2 bg-black/20 border border-panel-border rounded-lg text-sm text-white" />
          </label>
          <label class="space-y-1">
            <span class="text-xs text-gray-400">{{ t('server_status.watchdog_cooldown') }}</span>
            <input v-model.number="watchdogForm.cooldown_seconds" type="number" min="10" max="1800" class="w-full px-3 py-2 bg-black/20 border border-panel-border rounded-lg text-sm text-white" />
          </label>
          <label class="space-y-1">
            <span class="text-xs text-gray-400">{{ t('server_status.watchdog_log_limit') }}</span>
            <input v-model.number="watchdogForm.max_log_entries" type="number" min="100" max="5000" class="w-full px-3 py-2 bg-black/20 border border-panel-border rounded-lg text-sm text-white" />
          </label>
        </div>
        <div class="space-y-2">
          <p class="text-xs text-gray-400">{{ t('server_status.watchdog_services') }}</p>
          <div class="grid grid-cols-1 md:grid-cols-2 gap-2">
            <label v-for="item in watchdogSupportedServices" :key="item.name" class="flex items-center gap-2 text-sm text-gray-200">
              <input v-model="watchdogForm.services" type="checkbox" :value="item.name" class="rounded border-panel-border bg-black/30 text-orange-400 focus:ring-orange-500/40" />
              <span class="font-mono text-xs">{{ item.name }}</span>
              <span class="text-gray-500 text-xs">{{ item.desc }}</span>
            </label>
          </div>
        </div>
        <button
          @click="saveWatchdogConfig"
          :disabled="savingWatchdog || loadingWatchdog"
          class="px-4 py-2 rounded-lg text-sm bg-orange-500/20 text-orange-200 hover:bg-orange-500/35 transition inline-flex items-center"
        >
          <Loader2 v-if="savingWatchdog" class="w-4 h-4 animate-spin mr-2" />
          {{ t('server_status.watchdog_save') }}
        </button>
      </div>

      <div class="bg-panel-card border border-panel-border rounded-xl overflow-hidden">
        <div class="px-4 py-3 border-b border-panel-border">
          <p class="text-white text-sm font-medium">{{ t('server_status.watchdog_services') }}</p>
        </div>
        <div v-if="loadingWatchdog" class="p-6 text-center text-gray-500">{{ t('common.loading') }}</div>
        <table v-else class="w-full text-sm">
          <thead>
            <tr class="text-gray-400 border-b border-panel-border">
              <th class="text-left px-4 py-3">{{ t('common.name') }}</th>
              <th class="text-left px-4 py-3">{{ t('common.status') }}</th>
              <th class="text-left px-4 py-3">{{ t('server_status.watchdog_consecutive_failures') }}</th>
              <th class="text-left px-4 py-3">{{ t('server_status.watchdog_last_check') }}</th>
              <th class="text-left px-4 py-3">{{ t('common.error') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="item in watchdogStatus" :key="item.name" class="border-b border-panel-border/50 hover:bg-white/[0.02] transition">
              <td class="px-4 py-2.5 text-white font-mono text-xs">{{ item.name }}</td>
              <td class="px-4 py-2.5 text-xs">
                <span class="px-2 py-0.5 rounded-full border" :class="watchdogStatusBadgeClass(item.last_status)">
                  {{ watchdogStatusLabel(item.last_status) }}
                </span>
              </td>
              <td class="px-4 py-2.5 text-gray-300 text-xs">{{ item.consecutive_failures }}</td>
              <td class="px-4 py-2.5 text-gray-300 text-xs">{{ formatWatchdogTime(item.last_check_at) }}</td>
              <td class="px-4 py-2.5 text-red-300 text-xs">{{ item.last_error || '-' }}</td>
            </tr>
          </tbody>
        </table>
      </div>

      <div v-if="showWatchdogLogs" class="bg-panel-card border border-panel-border rounded-xl overflow-hidden">
        <div class="px-4 py-3 border-b border-panel-border flex items-center justify-between">
          <p class="text-white text-sm font-medium">{{ t('server_status.watchdog_logs') }}</p>
          <button @click="clearWatchdogLogs" class="px-3 py-1 rounded text-xs bg-red-600/20 text-red-300 hover:bg-red-600/40 transition">
            {{ t('server_status.watchdog_logs_clear') }}
          </button>
        </div>
        <div v-if="!watchdogLogs.length" class="p-6 text-center text-gray-500">{{ t('server_status.watchdog_no_logs') }}</div>
        <div v-else class="max-h-[380px] overflow-auto">
          <div v-for="item in watchdogLogs" :key="item.id" class="px-4 py-3 border-b border-panel-border/50">
            <div class="flex items-center gap-2 text-xs">
              <span class="text-gray-500">{{ item.timestamp }}</span>
              <span class="font-mono text-gray-300">{{ item.service }}</span>
              <span
                class="px-2 py-0.5 rounded-full border"
                :class="item.level === 'error' ? 'bg-red-500/20 text-red-300 border-red-500/30' : item.level === 'warn' ? 'bg-yellow-500/20 text-yellow-300 border-yellow-500/30' : 'bg-blue-500/20 text-blue-300 border-blue-500/30'"
              >
                {{ item.level }}
              </span>
            </div>
            <p class="text-sm text-white mt-1">{{ item.message }}</p>
          </div>
        </div>
      </div>
    </div>

    <div v-if="notification" class="fixed bottom-6 right-6 px-5 py-3 rounded-xl shadow-2xl text-sm font-medium z-50 bg-green-600 text-white">
      {{ notification }}
    </div>
  </div>
</template>

<script setup>
import { computed, ref, onMounted, onUnmounted, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { Activity, Cpu, MemoryStick, HardDrive, Clock, Loader2 } from 'lucide-vue-next'
import api from '../services/api'
import { useAuthStore } from '../stores/auth'

const { t } = useI18n({ useScope: 'global' })
const authStore = useAuthStore()

const tab = ref('services')
const notification = ref('')
let interval = null

const loadingMetrics = ref(false)
const loadingServices = ref(false)
const loadingProcesses = ref(false)
const loadingWatchdog = ref(false)
const savingWatchdog = ref(false)
const showWatchdogLogs = ref(true)

const metrics = ref({
  cpu: 0,
  cpuCores: 0,
  cpuModel: '-',
  ram: 0,
  ramUsed: '-',
  ramTotal: '-',
  disk: 0,
  diskUsed: '-',
  diskTotal: '-',
  uptimeDays: 0,
  uptimeFull: '-',
  loadAvg: '-'
})

const services = ref([])
const processes = ref([])
const watchdog = ref({
  enabled: true,
  config: {
    enabled: true,
    interval_seconds: 20,
    failure_threshold: 3,
    cooldown_seconds: 90,
    max_log_entries: 400,
    services: []
  },
  supported_services: [],
  status: [],
  logs: [],
  summary: {
    service_count: 0,
    unhealthy_count: 0,
    log_count: 0
  }
})
const watchdogForm = ref({
  interval_seconds: 20,
  failure_threshold: 3,
  cooldown_seconds: 90,
  max_log_entries: 400,
  services: []
})

const canControlServices = computed(() => authStore.isAdmin)
const canViewProcesses = computed(() => authStore.isAdmin || authStore.isReseller)
const canManageWatchdog = computed(() => authStore.isAdmin)
const watchdogEnabled = computed(() => Boolean(watchdog.value?.enabled))
const watchdogSupportedServices = computed(() => Array.isArray(watchdog.value?.supported_services) ? watchdog.value.supported_services : [])
const watchdogStatus = computed(() => Array.isArray(watchdog.value?.status) ? watchdog.value.status : [])
const watchdogLogs = computed(() => Array.isArray(watchdog.value?.logs) ? watchdog.value.logs : [])
const watchdogSummary = computed(() => watchdog.value?.summary || { service_count: 0, unhealthy_count: 0, log_count: 0 })

const showNotif = (message) => {
  notification.value = message
  setTimeout(() => {
    notification.value = ''
  }, 2200)
}

const normalizeServiceStatus = (value) => {
  const status = String(value || '').trim().toLowerCase()
  if (status === 'running' || status === 'starting' || status === 'failed' || status === 'stopped') {
    return status
  }
  if (status === 'active') return 'running'
  if (status === 'inactive') return 'stopped'
  return 'stopped'
}

const isServiceRunning = (value) => normalizeServiceStatus(value) === 'running'

const serviceStatusLabel = (value) => {
  const status = normalizeServiceStatus(value)
  if (status === 'running') return t('server_status.running')
  if (status === 'starting') return t('server_status.starting')
  if (status === 'failed') return t('server_status.failed')
  return t('server_status.stopped')
}

const serviceStatusBadgeClass = (value) => {
  const status = normalizeServiceStatus(value)
  if (status === 'running') return 'bg-green-500/20 text-green-300 border border-green-500/30'
  if (status === 'starting') return 'bg-blue-500/20 text-blue-300 border border-blue-500/30'
  if (status === 'failed') return 'bg-red-500/20 text-red-300 border border-red-500/30'
  return 'bg-gray-500/20 text-gray-300 border border-gray-500/30'
}

const normalizeWatchdogStatus = (value) => {
  const status = String(value || '').trim().toLowerCase()
  if (status === 'healthy' || status === 'unhealthy' || status === 'unknown') {
    return status
  }
  return 'unknown'
}

const watchdogStatusLabel = (value) => {
  const status = normalizeWatchdogStatus(value)
  if (status === 'healthy') return t('server_status.watchdog_status_healthy')
  if (status === 'unhealthy') return t('server_status.watchdog_status_unhealthy')
  return t('server_status.watchdog_status_unknown')
}

const watchdogStatusBadgeClass = (value) => {
  const status = normalizeWatchdogStatus(value)
  if (status === 'healthy') return 'bg-green-500/20 text-green-300 border-green-500/30'
  if (status === 'unhealthy') return 'bg-red-500/20 text-red-300 border-red-500/30'
  return 'bg-gray-500/20 text-gray-300 border-gray-500/30'
}

const formatWatchdogTime = (unixSeconds) => {
  const value = Number(unixSeconds || 0)
  if (!value) return '-'
  return new Date(value * 1000).toLocaleString()
}

const fetchMetrics = async () => {
  loadingMetrics.value = true
  try {
    const res = await api.get('/status/metrics')

    const payload = res.data?.data || {}
    metrics.value = {
      cpu: Number(payload.cpu_usage || 0),
      cpuCores: Number(payload.cpu_cores || 0),
      cpuModel: payload.cpu_model || '-',
      ram: Number(payload.ram_usage || 0),
      ramUsed: payload.ram_used || '-',
      ramTotal: payload.ram_total || '-',
      disk: Number(payload.disk_usage || 0),
      diskUsed: payload.disk_used || '-',
      diskTotal: payload.disk_total || '-',
      uptimeDays: Math.floor(Number(payload.uptime_seconds || 0) / 86400),
      uptimeFull: payload.uptime_human || '-',
      loadAvg: payload.load_avg || '-'
    }
  } finally {
    loadingMetrics.value = false
  }
}

const fetchServices = async () => {
  loadingServices.value = true
  try {
    const res = await api.get('/status/services')
    services.value = Array.isArray(res.data?.data) ? res.data.data : []
  } finally {
    loadingServices.value = false
  }
}

const fetchProcesses = async () => {
  if (!canViewProcesses.value) {
    processes.value = []
    return
  }
  loadingProcesses.value = true
  try {
    const res = await api.get('/status/processes')
    processes.value = Array.isArray(res.data?.data) ? res.data.data : []
  } catch {
    processes.value = []
  } finally {
    loadingProcesses.value = false
  }
}

const applyWatchdogPayload = (payload = {}) => {
  const config = payload.config || {}
  watchdog.value = {
    enabled: Boolean(payload.enabled),
    config: {
      enabled: Boolean(config.enabled),
      interval_seconds: Number(config.interval_seconds || 20),
      failure_threshold: Number(config.failure_threshold || 3),
      cooldown_seconds: Number(config.cooldown_seconds || 90),
      max_log_entries: Number(config.max_log_entries || 400),
      services: Array.isArray(config.services) ? config.services : []
    },
    supported_services: Array.isArray(payload.supported_services) ? payload.supported_services : [],
    status: Array.isArray(payload.status) ? payload.status : [],
    logs: Array.isArray(payload.logs) ? payload.logs : [],
    summary: payload.summary || { service_count: 0, unhealthy_count: 0, log_count: 0 }
  }

  watchdogForm.value = {
    interval_seconds: Number(config.interval_seconds || 20),
    failure_threshold: Number(config.failure_threshold || 3),
    cooldown_seconds: Number(config.cooldown_seconds || 90),
    max_log_entries: Number(config.max_log_entries || 400),
    services: Array.isArray(config.services) ? [...config.services] : []
  }
}

const fetchWatchdog = async () => {
  if (!canManageWatchdog.value) return
  loadingWatchdog.value = true
  try {
    const res = await api.get('/status/watchdog')
    applyWatchdogPayload(res.data?.data || {})
  } catch {
    showNotif(t('server_status.messages.watchdog_failed'))
  } finally {
    loadingWatchdog.value = false
  }
}

const toggleWatchdog = async (enabled) => {
  if (!canManageWatchdog.value) return
  savingWatchdog.value = true
  try {
    const res = await api.post('/status/watchdog/toggle', { enabled })
    applyWatchdogPayload(res.data?.data || {})
    showNotif(t('server_status.messages.watchdog_toggled'))
  } catch {
    showNotif(t('server_status.messages.watchdog_failed'))
  } finally {
    savingWatchdog.value = false
  }
}

const saveWatchdogConfig = async () => {
  if (!canManageWatchdog.value) return
  savingWatchdog.value = true
  try {
    const payload = {
      interval_seconds: Number(watchdogForm.value.interval_seconds || 20),
      failure_threshold: Number(watchdogForm.value.failure_threshold || 3),
      cooldown_seconds: Number(watchdogForm.value.cooldown_seconds || 90),
      max_log_entries: Number(watchdogForm.value.max_log_entries || 400),
      services: Array.isArray(watchdogForm.value.services) ? watchdogForm.value.services : []
    }
    const res = await api.post('/status/watchdog/config', payload)
    applyWatchdogPayload(res.data?.data || {})
    showNotif(t('server_status.messages.watchdog_saved'))
  } catch {
    showNotif(t('server_status.messages.watchdog_failed'))
  } finally {
    savingWatchdog.value = false
  }
}

const clearWatchdogLogs = async () => {
  if (!canManageWatchdog.value) return
  try {
    const res = await api.post('/status/watchdog/logs/clear')
    applyWatchdogPayload(res.data?.data || {})
    showNotif(t('server_status.messages.watchdog_logs_cleared'))
  } catch {
    showNotif(t('server_status.messages.watchdog_failed'))
  }
}

const controlService = async (name, action) => {
  try {
    await api.post('/status/service/control', { name, action })
    showNotif(t('server_status.messages.service_done', { name, action }))
    await fetchServices()
  } catch {
    showNotif(t('server_status.messages.service_failed'))
  }
}

const killProcess = async (pid) => {
  try {
    await api.post('/status/service/control', { name: String(pid), action: 'kill' })
    showNotif(t('server_status.messages.process_killed', { pid }))
    await fetchProcesses()
  } catch {
    showNotif(t('server_status.messages.process_failed'))
  }
}

const refreshAll = async () => {
  const tasks = [fetchMetrics(), fetchServices()]
  if (canViewProcesses.value) {
    tasks.push(fetchProcesses())
  }
  if (canManageWatchdog.value) {
    tasks.push(fetchWatchdog())
  }
  await Promise.all(tasks)
  showNotif(t('server_status.messages.updated'))
}

watch(tab, async (value) => {
  if (value === 'services' && !services.value.length) await fetchServices()
  if (value === 'processes' && canViewProcesses.value && !processes.value.length) await fetchProcesses()
  if (value === 'watchdog' && canManageWatchdog.value && !watchdogStatus.value.length) await fetchWatchdog()
})

onMounted(async () => {
  if (!canViewProcesses.value && tab.value === 'processes') {
    tab.value = 'services'
  }
  if (!canManageWatchdog.value && tab.value === 'watchdog') {
    tab.value = 'services'
  }
  await refreshAll()
  interval = setInterval(async () => {
    await fetchMetrics()
    if (canManageWatchdog.value && tab.value === 'watchdog') {
      await fetchWatchdog()
    }
  }, 10000)
})

onUnmounted(() => {
  if (interval) clearInterval(interval)
})
</script>
