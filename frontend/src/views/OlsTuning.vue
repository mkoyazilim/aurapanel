<template>
  <div class="space-y-6 max-w-5xl">
    <div class="flex items-center justify-between gap-3">
      <div>
        <h1 class="text-2xl font-bold text-white">OLS Tuning</h1>
        <p class="text-gray-400 mt-1">OpenLiteSpeed global tuning parametrelerini yonetin.</p>
      </div>
      <button class="btn-secondary" @click="loadConfig">Yenile</button>
    </div>

    <div v-if="error" class="aura-card border-red-500/30 bg-red-500/5 text-red-400">{{ error }}</div>
    <div v-if="success" class="aura-card border-green-500/30 bg-green-500/5 text-green-300">{{ success }}</div>

    <div class="aura-card space-y-4">
      <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
        <div>
          <label class="block text-sm text-gray-400 mb-1">Max Connections</label>
          <input v-model.number="form.max_connections" type="number" min="100" max="500000" class="aura-input" />
        </div>
        <div>
          <label class="block text-sm text-gray-400 mb-1">Max SSL Connections</label>
          <input v-model.number="form.max_ssl_connections" type="number" min="100" max="500000" class="aura-input" />
        </div>
        <div>
          <label class="block text-sm text-gray-400 mb-1">Connection Timeout (sec)</label>
          <input v-model.number="form.conn_timeout_secs" type="number" min="30" max="3600" class="aura-input" />
        </div>
        <div>
          <label class="block text-sm text-gray-400 mb-1">KeepAlive Timeout (sec)</label>
          <input v-model.number="form.keep_alive_timeout_secs" type="number" min="1" max="120" class="aura-input" />
        </div>
        <div>
          <label class="block text-sm text-gray-400 mb-1">Max KeepAlive Requests</label>
          <input v-model.number="form.max_keep_alive_requests" type="number" min="10" max="1000000" class="aura-input" />
        </div>
        <div>
          <label class="block text-sm text-gray-400 mb-1">Static Cache Max Age (sec)</label>
          <input v-model.number="form.static_cache_max_age_secs" type="number" min="0" max="31536000" class="aura-input" />
        </div>
      </div>

      <div class="flex flex-wrap gap-6 text-sm text-gray-300">
        <label class="inline-flex items-center gap-2">
          <input v-model="form.gzip_compression" type="checkbox" class="w-4 h-4" />
          Gzip Compression
        </label>
        <label class="inline-flex items-center gap-2">
          <input v-model="form.static_cache_enabled" type="checkbox" class="w-4 h-4" />
          Static Cache
        </label>
      </div>

      <div class="flex gap-3">
        <button class="btn-secondary" @click="saveConfig">Kaydet</button>
        <button class="btn-primary" @click="applyConfig">Kaydet + Uygula</button>
      </div>
    </div>
  </div>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import api from '../services/api'

const error = ref('')
const success = ref('')

const form = ref({
  max_connections: 10000,
  max_ssl_connections: 10000,
  conn_timeout_secs: 300,
  keep_alive_timeout_secs: 5,
  max_keep_alive_requests: 10000,
  gzip_compression: true,
  static_cache_enabled: true,
  static_cache_max_age_secs: 3600,
})

function apiErrorMessage(e, fallback) {
  return e?.response?.data?.message || e?.message || fallback
}

async function loadConfig() {
  error.value = ''
  success.value = ''
  try {
    const res = await api.get('/ols/tuning')
    form.value = { ...form.value, ...(res.data?.data || {}) }
  } catch (e) {
    error.value = apiErrorMessage(e, 'OLS tuning verisi alinamadi')
  }
}

async function saveConfig() {
  error.value = ''
  success.value = ''
  try {
    await api.post('/ols/tuning', form.value)
    success.value = 'OLS tuning ayarlari kaydedildi.'
  } catch (e) {
    error.value = apiErrorMessage(e, 'OLS tuning kaydedilemedi')
  }
}

async function applyConfig() {
  error.value = ''
  success.value = ''
  try {
    const res = await api.post('/ols/tuning/apply', form.value)
    success.value = res.data?.message || 'OLS tuning uygulandi.'
  } catch (e) {
    error.value = apiErrorMessage(e, 'OLS tuning uygulanamadi')
  }
}

onMounted(loadConfig)
</script>
