<template>
  <div class="space-y-6">
    <div class="flex items-center justify-between">
      <div>
        <h1 class="text-2xl font-bold text-white flex items-center gap-3">
          <Activity class="w-7 h-7 text-orange-400" />
          Server Status
        </h1>
        <p class="text-gray-400 mt-1">Sunucu metrikleri, servisler ve prosesler</p>
      </div>
      <button
        @click="refreshAll"
        class="px-4 py-2 bg-panel-hover text-gray-300 rounded-lg text-sm hover:bg-gray-600 transition flex items-center"
      >
        <Loader2 v-if="loadingMetrics || loadingServices || loadingProcesses" class="w-4 h-4 animate-spin mr-2" />
        <span>Yenile</span>
      </button>
    </div>

    <div class="grid grid-cols-1 md:grid-cols-4 gap-4">
      <div class="bg-panel-card border border-panel-border rounded-xl p-5">
        <div class="flex items-center justify-between mb-3">
          <p class="text-sm text-gray-400">CPU</p>
          <Cpu class="w-5 h-5 text-blue-400" />
        </div>
        <p class="text-3xl font-bold text-white">{{ metrics.cpu }}%</p>
        <p class="text-xs text-gray-500 mt-2">{{ metrics.cpuCores }} core / {{ metrics.cpuModel }}</p>
      </div>

      <div class="bg-panel-card border border-panel-border rounded-xl p-5">
        <div class="flex items-center justify-between mb-3">
          <p class="text-sm text-gray-400">RAM</p>
          <MemoryStick class="w-5 h-5 text-green-400" />
        </div>
        <p class="text-3xl font-bold text-white">{{ metrics.ram }}%</p>
        <p class="text-xs text-gray-500 mt-2">{{ metrics.ramUsed }} / {{ metrics.ramTotal }}</p>
      </div>

      <div class="bg-panel-card border border-panel-border rounded-xl p-5">
        <div class="flex items-center justify-between mb-3">
          <p class="text-sm text-gray-400">Disk</p>
          <HardDrive class="w-5 h-5 text-purple-400" />
        </div>
        <p class="text-3xl font-bold text-white">{{ metrics.disk }}%</p>
        <p class="text-xs text-gray-500 mt-2">{{ metrics.diskUsed }} / {{ metrics.diskTotal }}</p>
      </div>

      <div class="bg-panel-card border border-panel-border rounded-xl p-5">
        <div class="flex items-center justify-between mb-3">
          <p class="text-sm text-gray-400">Uptime</p>
          <Clock class="w-5 h-5 text-orange-400" />
        </div>
        <p class="text-3xl font-bold text-white">{{ metrics.uptimeDays }}d</p>
        <p class="text-xs text-gray-500 mt-2">{{ metrics.uptimeFull }}</p>
        <p class="text-xs text-gray-500">Load: {{ metrics.loadAvg }}</p>
      </div>
    </div>

    <div class="border-b border-panel-border">
      <nav class="flex gap-6">
        <button
          @click="tab = 'services'"
          :class="['pb-3 text-sm font-medium transition', tab === 'services' ? 'text-orange-400 border-b-2 border-orange-400' : 'text-gray-400 hover:text-white']"
        >
          Services
        </button>
        <button
          @click="tab = 'processes'"
          :class="['pb-3 text-sm font-medium transition', tab === 'processes' ? 'text-orange-400 border-b-2 border-orange-400' : 'text-gray-400 hover:text-white']"
        >
          Processes
        </button>
      </nav>
    </div>

    <div v-if="tab === 'services'" class="grid grid-cols-1 md:grid-cols-2 gap-3">
      <div v-if="loadingServices" class="col-span-1 md:col-span-2 text-center py-6 text-gray-500">Yukleniyor...</div>
      <div v-for="s in services" :key="s.name" class="bg-panel-card border border-panel-border rounded-xl p-4 flex items-center justify-between">
        <div>
          <p class="text-white font-medium text-sm">{{ s.name }}</p>
          <p class="text-gray-500 text-xs">{{ s.desc }}</p>
        </div>
        <div class="flex gap-2">
          <button
            v-if="s.status === 'running'"
            @click="controlService(s.name, 'stop')"
            class="px-2 py-1 bg-red-600/20 text-red-400 rounded text-xs hover:bg-red-600/40 transition"
          >
            Stop
          </button>
          <button
            v-else
            @click="controlService(s.name, 'start')"
            class="px-2 py-1 bg-green-600/20 text-green-400 rounded text-xs hover:bg-green-600/40 transition"
          >
            Start
          </button>
          <button
            @click="controlService(s.name, 'restart')"
            class="px-2 py-1 bg-blue-600/20 text-blue-400 rounded text-xs hover:bg-blue-600/40 transition"
          >
            Restart
          </button>
        </div>
      </div>
    </div>

    <div v-if="tab === 'processes'" class="bg-panel-card border border-panel-border rounded-xl overflow-hidden">
      <div v-if="loadingProcesses" class="p-6 text-center text-gray-500">Yukleniyor...</div>
      <table v-else class="w-full text-sm">
        <thead>
          <tr class="text-gray-400 border-b border-panel-border">
            <th class="text-left px-4 py-3">PID</th>
            <th class="text-left px-4 py-3">User</th>
            <th class="text-left px-4 py-3">CPU</th>
            <th class="text-left px-4 py-3">RAM</th>
            <th class="text-left px-4 py-3">Command</th>
            <th class="text-right px-4 py-3">Action</th>
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
              <button @click="killProcess(p.pid)" class="px-2 py-1 bg-red-600/20 text-red-400 rounded text-xs hover:bg-red-600/40 transition">Kill</button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <div v-if="notification" class="fixed bottom-6 right-6 px-5 py-3 rounded-xl shadow-2xl text-sm font-medium z-50 bg-green-600 text-white">
      {{ notification }}
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted, watch } from 'vue'
import { Activity, Cpu, MemoryStick, HardDrive, Clock, Loader2 } from 'lucide-vue-next'
import api from '../services/api'

const tab = ref('services')
const notification = ref('')
let interval = null

const loadingMetrics = ref(false)
const loadingServices = ref(false)
const loadingProcesses = ref(false)

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

const showNotif = (message) => {
  notification.value = message
  setTimeout(() => {
    notification.value = ''
  }, 2200)
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
  loadingProcesses.value = true
  try {
    const res = await api.get('/status/processes')
    processes.value = Array.isArray(res.data?.data) ? res.data.data : []
  } finally {
    loadingProcesses.value = false
  }
}

const controlService = async (name, action) => {
  try {
    await api.post('/status/service/control', { name, action })
    showNotif(`${name} ${action} tamamlandi`)
    await fetchServices()
  } catch {
    showNotif('Servis aksiyonu basarisiz')
  }
}

const killProcess = async (pid) => {
  try {
    await api.post('/status/service/control', { name: String(pid), action: 'kill' })
    showNotif(`PID ${pid} sonlandirildi`)
    await fetchProcesses()
  } catch {
    showNotif('Proses sonlandirma basarisiz')
  }
}

const refreshAll = async () => {
  await Promise.all([fetchMetrics(), fetchServices(), fetchProcesses()])
  showNotif('Veriler guncellendi')
}

watch(tab, async (value) => {
  if (value === 'services' && !services.value.length) await fetchServices()
  if (value === 'processes' && !processes.value.length) await fetchProcesses()
})

onMounted(async () => {
  await refreshAll()
  interval = setInterval(fetchMetrics, 10000)
})

onUnmounted(() => {
  if (interval) clearInterval(interval)
})
</script>
