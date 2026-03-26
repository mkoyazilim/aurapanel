<template>
  <div class="space-y-6 max-w-4xl">
    <div>
      <h1 class="text-2xl font-bold text-white flex items-center gap-2">
        <Settings2 class="w-6 h-6 text-indigo-400" />
        Change AuraPanel Port
      </h1>
      <p class="text-gray-400 mt-1">
        Panel access port configuration. Default port is 8090.
      </p>
    </div>

    <div class="bg-amber-500/10 border border-amber-500/30 rounded-xl p-4">
      <p class="text-amber-300 font-semibold text-sm">Important Notice</p>
      <p class="text-amber-100/90 text-sm mt-1">
        After changing the port, reconnect using the new URL. Keep firewall access enabled for the selected port.
      </p>
    </div>

    <div class="bg-blue-500/10 border border-blue-500/30 rounded-xl p-4 text-sm">
      <p class="text-blue-300 font-semibold">Current Configuration</p>
      <p class="text-blue-100 mt-1">
        Current panel port: <span class="font-bold">{{ currentPort }}</span>
        <span class="text-blue-200/80">({{ gatewayAddr }})</span>
      </p>
    </div>

    <div class="bg-panel-card border border-panel-border rounded-xl p-6 space-y-5">
      <div>
        <label class="block text-xs text-gray-400 uppercase tracking-wide mb-2">New Port Number</label>
        <input
          v-model.number="port"
          type="number"
          min="1"
          max="65535"
          class="w-full bg-panel-darker border border-panel-border rounded-lg px-4 py-3 text-white focus:outline-none focus:border-indigo-400"
          placeholder="8090"
        />
        <p class="text-xs text-gray-500 mt-2">Valid range: 1 - 65535. Common ports: 8090, 8443, 7080.</p>
      </div>

      <label class="flex items-center gap-3 text-sm text-gray-300">
        <input v-model="openFirewall" type="checkbox" class="accent-indigo-500" />
        Try to open firewall for the new panel port automatically
      </label>

      <div class="pt-2">
        <button
          @click="changePort"
          :disabled="loading || saving"
          class="px-5 py-2.5 rounded-lg bg-indigo-600 hover:bg-indigo-500 disabled:opacity-50 text-white font-medium transition inline-flex items-center gap-2"
        >
          <Loader2 v-if="saving" class="w-4 h-4 animate-spin" />
          <Save v-else class="w-4 h-4" />
          <span>{{ saving ? 'Changing...' : 'Change Port' }}</span>
        </button>
      </div>
    </div>

    <div v-if="message" class="bg-emerald-500/10 border border-emerald-500/30 rounded-xl p-4 text-sm text-emerald-200">
      <p class="font-semibold">{{ message }}</p>
      <p class="mt-1">Reconnect URL: <span class="font-mono">{{ newAccessUrl }}</span></p>
    </div>

    <div v-if="warnings.length" class="bg-yellow-500/10 border border-yellow-500/30 rounded-xl p-4 text-sm text-yellow-200">
      <p class="font-semibold mb-2">Warnings</p>
      <ul class="space-y-1">
        <li v-for="item in warnings" :key="item">- {{ item }}</li>
      </ul>
    </div>

    <div v-if="firewallActions.length" class="bg-panel-card border border-panel-border rounded-xl p-4 text-sm text-gray-300">
      <p class="font-semibold text-white mb-2">Firewall Actions</p>
      <ul class="space-y-1">
        <li v-for="item in firewallActions" :key="item">- {{ item }}</li>
      </ul>
    </div>

    <div v-if="error" class="bg-red-500/10 border border-red-500/30 rounded-xl p-4 text-sm text-red-200">
      {{ error }}
    </div>
  </div>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { Loader2, Save, Settings2 } from 'lucide-vue-next'
import api from '../services/api'

const loading = ref(false)
const saving = ref(false)
const port = ref(8090)
const currentPort = ref(8090)
const gatewayAddr = ref(':8090')
const openFirewall = ref(true)
const message = ref('')
const error = ref('')
const warnings = ref([])
const firewallActions = ref([])

const newAccessUrl = computed(() => {
  if (typeof window === 'undefined') {
    return `http://YOUR_SERVER_IP:${port.value}`
  }
  return `${window.location.protocol}//${window.location.hostname}:${port.value}`
})

const loadPanelPort = async () => {
  loading.value = true
  error.value = ''
  try {
    const response = await api.get('/status/panel-port')
    if (response.data?.status !== 'success') {
      throw new Error(response.data?.message || 'Panel port bilgisi alinamadi.')
    }

    const payload = response.data?.data || {}
    const fetchedPort = Number(payload.current_port || 8090)
    currentPort.value = fetchedPort
    port.value = fetchedPort
    gatewayAddr.value = payload.gateway_addr || `:${fetchedPort}`
  } catch (err) {
    error.value = err?.message || 'Panel port bilgisi okunamadi.'
  } finally {
    loading.value = false
  }
}

const changePort = async () => {
  error.value = ''
  message.value = ''
  warnings.value = []
  firewallActions.value = []

  const targetPort = Number(port.value)
  if (!Number.isInteger(targetPort) || targetPort < 1 || targetPort > 65535) {
    error.value = 'Port araligi 1-65535 olmali.'
    return
  }

  saving.value = true
  try {
    const response = await api.post('/status/panel-port', {
      port: targetPort,
      open_firewall: openFirewall.value
    })

    if (response.data?.status !== 'success') {
      throw new Error(response.data?.message || 'Port guncellenemedi.')
    }

    const payload = response.data?.data || {}
    currentPort.value = targetPort
    gatewayAddr.value = payload.gateway_addr || `:${targetPort}`
    firewallActions.value = Array.isArray(payload.firewall_actions) ? payload.firewall_actions : []
    warnings.value = Array.isArray(payload.warnings) ? payload.warnings : []

    const restartNote = payload.restart_scheduled
      ? ' Gateway restart planlandi.'
      : ''
    message.value = `Panel portu ${targetPort} olarak guncellendi.${restartNote}`
  } catch (err) {
    const apiMessage = err?.response?.data?.message
    if (!apiMessage && err?.message && err.message.toLowerCase().includes('network')) {
      message.value = `Baglanti kesildi. Gateway restart olmus olabilir, yeni porttan tekrar baglan: ${newAccessUrl.value}`
      return
    }
    error.value = apiMessage || err?.message || 'Port guncelleme basarisiz.'
  } finally {
    saving.value = false
  }
}

onMounted(loadPanelPort)
</script>
