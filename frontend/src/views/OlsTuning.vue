<template>
  <div class="space-y-6 max-w-5xl">
    <div class="flex items-center justify-between gap-3">
      <div>
        <h1 class="text-2xl font-bold text-white">{{ t('ols_tuning.title') }}</h1>
        <p class="text-gray-400 mt-1">{{ t('ols_tuning.subtitle') }}</p>
      </div>
      <button class="btn-secondary" @click="loadConfig">{{ t('ols_tuning.refresh') }}</button>
    </div>

    <div v-if="error" class="aura-card border-red-500/30 bg-red-500/5 text-red-400">{{ error }}</div>
    <div v-if="success" class="aura-card border-green-500/30 bg-green-500/5 text-green-300">{{ success }}</div>

    <div class="aura-card space-y-4">
      <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
        <div>
          <label class="block text-sm text-gray-400 mb-1">{{ t('ols_tuning.fields.max_connections') }}</label>
          <input v-model.number="form.max_connections" type="number" min="100" max="500000" class="aura-input" />
        </div>
        <div>
          <label class="block text-sm text-gray-400 mb-1">{{ t('ols_tuning.fields.max_ssl_connections') }}</label>
          <input v-model.number="form.max_ssl_connections" type="number" min="100" max="500000" class="aura-input" />
        </div>
        <div>
          <label class="block text-sm text-gray-400 mb-1">{{ t('ols_tuning.fields.conn_timeout_secs') }}</label>
          <input v-model.number="form.conn_timeout_secs" type="number" min="30" max="3600" class="aura-input" />
        </div>
        <div>
          <label class="block text-sm text-gray-400 mb-1">{{ t('ols_tuning.fields.keep_alive_timeout_secs') }}</label>
          <input v-model.number="form.keep_alive_timeout_secs" type="number" min="1" max="120" class="aura-input" />
        </div>
        <div>
          <label class="block text-sm text-gray-400 mb-1">{{ t('ols_tuning.fields.max_keep_alive_requests') }}</label>
          <input v-model.number="form.max_keep_alive_requests" type="number" min="10" max="1000000" class="aura-input" />
        </div>
        <div>
          <label class="block text-sm text-gray-400 mb-1">{{ t('ols_tuning.fields.static_cache_max_age_secs') }}</label>
          <input v-model.number="form.static_cache_max_age_secs" type="number" min="0" max="31536000" class="aura-input" />
        </div>
      </div>

      <div class="flex flex-wrap gap-6 text-sm text-gray-300">
        <label class="inline-flex items-center gap-2">
          <input v-model="form.gzip_compression" type="checkbox" class="w-4 h-4" />
          {{ t('ols_tuning.flags.gzip_compression') }}
        </label>
        <label class="inline-flex items-center gap-2">
          <input v-model="form.static_cache_enabled" type="checkbox" class="w-4 h-4" />
          {{ t('ols_tuning.flags.static_cache_enabled') }}
        </label>
      </div>

      <div class="flex gap-3">
        <button class="btn-secondary" @click="saveConfig">{{ t('ols_tuning.save') }}</button>
        <button class="btn-primary" @click="applyConfig">{{ t('ols_tuning.save_apply') }}</button>
      </div>
    </div>
  </div>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import api from '../services/api'

const { t } = useI18n({ useScope: 'global' })

const error = ref('')
const success = ref('')

const form = ref({
  max_connections: 10000,
  max_ssl_connections: 10000,
  conn_timeout_secs: 300,
  keep_alive_timeout_secs: 5,
  max_keep_alive_requests: 10000,
  gzip_compression: true,
  static_cache_enabled: false,
  static_cache_max_age_secs: 3600,
})

function toIntOrFallback(value, fallback) {
  const parsed = Number.parseInt(`${value ?? ''}`, 10)
  return Number.isFinite(parsed) ? parsed : fallback
}

function toBoolOrFallback(value, fallback) {
  if (typeof value === 'boolean') {
    return value
  }
  if (typeof value === 'number') {
    return value !== 0
  }
  if (typeof value === 'string') {
    const normalized = value.trim().toLowerCase()
    if (['1', 'true', 'yes', 'on', 'enable', 'enabled'].includes(normalized)) {
      return true
    }
    if (['0', 'false', 'no', 'off', 'disable', 'disabled', ''].includes(normalized)) {
      return false
    }
  }
  return fallback
}

function sanitizeOLSTuningConfig(input = {}) {
  const base = form.value
  return {
    max_connections: toIntOrFallback(input.max_connections, base.max_connections),
    max_ssl_connections: toIntOrFallback(input.max_ssl_connections, base.max_ssl_connections),
    conn_timeout_secs: toIntOrFallback(input.conn_timeout_secs, base.conn_timeout_secs),
    keep_alive_timeout_secs: toIntOrFallback(input.keep_alive_timeout_secs, base.keep_alive_timeout_secs),
    max_keep_alive_requests: toIntOrFallback(input.max_keep_alive_requests, base.max_keep_alive_requests),
    gzip_compression: toBoolOrFallback(input.gzip_compression, base.gzip_compression),
    static_cache_enabled: toBoolOrFallback(input.static_cache_enabled, base.static_cache_enabled),
    static_cache_max_age_secs: toIntOrFallback(input.static_cache_max_age_secs, base.static_cache_max_age_secs),
  }
}

function buildOLSTuningPayload() {
  return sanitizeOLSTuningConfig(form.value)
}

function apiErrorMessage(e, fallbackKey) {
  return e?.response?.data?.message || e?.message || t(fallbackKey)
}

async function loadConfig() {
  error.value = ''
  success.value = ''
  try {
    const res = await api.get('/ols/tuning')
    form.value = sanitizeOLSTuningConfig(res.data?.data || {})
  } catch (e) {
    error.value = apiErrorMessage(e, 'ols_tuning.messages.load_failed')
  }
}

async function saveConfig() {
  error.value = ''
  success.value = ''
  try {
    const res = await api.post('/ols/tuning', buildOLSTuningPayload())
    form.value = sanitizeOLSTuningConfig(res.data?.data || form.value)
    success.value = t('ols_tuning.messages.saved')
  } catch (e) {
    error.value = apiErrorMessage(e, 'ols_tuning.messages.save_failed')
  }
}

async function applyConfig() {
  error.value = ''
  success.value = ''
  try {
    const res = await api.post('/ols/tuning/apply', buildOLSTuningPayload())
    form.value = sanitizeOLSTuningConfig(res.data?.data || form.value)
    success.value = res.data?.message || t('ols_tuning.messages.applied')
  } catch (e) {
    error.value = apiErrorMessage(e, 'ols_tuning.messages.apply_failed')
  }
}

onMounted(loadConfig)
</script>
