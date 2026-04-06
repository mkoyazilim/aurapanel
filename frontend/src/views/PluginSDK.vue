<template>
  <div class="space-y-6">
    <div class="flex items-center justify-between gap-3">
      <div>
        <h1 class="text-2xl font-bold text-white">{{ t('plugin_sdk.title') }}</h1>
        <p class="text-gray-400 mt-1">{{ t('plugin_sdk.subtitle') }}</p>
      </div>
      <button class="btn-secondary" @click="loadAll">{{ t('plugin_sdk.refresh') }}</button>
    </div>

    <div v-if="error" class="aura-card border-red-500/30 bg-red-500/5 text-red-400">{{ error }}</div>
    <div v-if="success" class="aura-card border-green-500/30 bg-green-500/5 text-green-300">{{ success }}</div>

    <div class="grid grid-cols-1 gap-4 xl:grid-cols-3">
      <div class="aura-card space-y-4 xl:col-span-2">
        <h2 class="text-lg font-semibold text-white">{{ t('plugin_sdk.form.title') }}</h2>
        <div class="grid grid-cols-1 gap-3 md:grid-cols-2">
          <input v-model="form.id" class="aura-input" :placeholder="t('plugin_sdk.form.id')" />
          <input v-model="form.name" class="aura-input" :placeholder="t('plugin_sdk.form.name')" />
          <input v-model="form.version" class="aura-input" :placeholder="t('plugin_sdk.form.version')" />
          <input v-model="form.author" class="aura-input" :placeholder="t('plugin_sdk.form.author')" />
          <input v-model="form.entrypoint" class="aura-input md:col-span-2" :placeholder="t('plugin_sdk.form.entrypoint')" />
          <textarea v-model="form.description" rows="2" class="aura-input md:col-span-2" :placeholder="t('plugin_sdk.form.description')" />
          <input v-model="form.hooks_csv" class="aura-input md:col-span-2" :placeholder="t('plugin_sdk.form.hooks')" />
          <input v-model="form.permissions_csv" class="aura-input md:col-span-2" :placeholder="t('plugin_sdk.form.permissions')" />
          <textarea v-model="form.config_schema" rows="3" class="aura-input md:col-span-2 font-mono text-xs" :placeholder="t('plugin_sdk.form.config_schema')" />
          <label class="inline-flex items-center gap-2 text-sm text-gray-300 md:col-span-2">
            <input v-model="form.enabled" type="checkbox" class="h-4 w-4" />
            {{ t('plugin_sdk.form.enabled') }}
          </label>
        </div>
        <div class="flex gap-2">
          <button class="btn-primary" :disabled="saving" @click="savePlugin">{{ t('plugin_sdk.form.save') }}</button>
          <button class="btn-secondary" @click="resetForm">{{ t('plugin_sdk.form.reset') }}</button>
        </div>
      </div>

      <div class="aura-card space-y-3">
        <h2 class="text-lg font-semibold text-white">{{ t('plugin_sdk.sdk_info.title') }}</h2>
        <p class="text-xs text-gray-400">{{ t('plugin_sdk.sdk_info.version') }}: <span class="text-white">{{ sdkInfo.manifest_version || '-' }}</span></p>
        <div>
          <p class="text-xs uppercase tracking-wide text-gray-500 mb-1">{{ t('plugin_sdk.sdk_info.required_fields') }}</p>
          <div class="flex flex-wrap gap-1">
            <span v-for="field in sdkInfo.required_fields || []" :key="`required-${field}`" class="rounded border border-panel-border px-2 py-1 text-xs text-gray-300">{{ field }}</span>
          </div>
        </div>
        <div>
          <p class="text-xs uppercase tracking-wide text-gray-500 mb-1">{{ t('plugin_sdk.sdk_info.supported_hooks') }}</p>
          <div class="flex flex-wrap gap-1">
            <span v-for="hook in sdkInfo.supported_hooks || []" :key="`hook-${hook}`" class="rounded border border-brand-500/30 bg-brand-500/10 px-2 py-1 text-xs text-brand-300">{{ hook }}</span>
          </div>
        </div>
      </div>
    </div>

    <div class="aura-card space-y-3">
      <h2 class="text-lg font-semibold text-white">{{ t('plugin_sdk.list.title') }}</h2>
      <div class="overflow-x-auto">
        <table class="w-full text-sm">
          <thead>
            <tr class="border-b border-panel-border text-gray-400">
              <th class="px-2 py-2 text-left">{{ t('plugin_sdk.list.name') }}</th>
              <th class="px-2 py-2 text-left">{{ t('plugin_sdk.list.entrypoint') }}</th>
              <th class="px-2 py-2 text-left">{{ t('plugin_sdk.list.hooks') }}</th>
              <th class="px-2 py-2 text-left">{{ t('plugin_sdk.list.status') }}</th>
              <th class="px-2 py-2 text-right">{{ t('plugin_sdk.list.action') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="item in plugins" :key="item.id" class="border-b border-panel-border/40">
              <td class="px-2 py-2">
                <p class="font-semibold text-white">{{ item.name }}</p>
                <p class="text-xs text-gray-500">{{ item.id }} - v{{ item.version }}</p>
              </td>
              <td class="px-2 py-2 font-mono text-xs text-gray-300">{{ item.entrypoint }}</td>
              <td class="px-2 py-2 text-gray-300">{{ (item.hooks || []).join(', ') || '-' }}</td>
              <td class="px-2 py-2">
                <span :class="item.enabled ? 'text-green-400' : 'text-yellow-400'">
                  {{ item.enabled ? t('common.active') : t('common.inactive') }}
                </span>
              </td>
              <td class="px-2 py-2 text-right">
                <div class="flex justify-end gap-2">
                  <button class="btn-secondary px-2 py-1 text-xs" @click="editPlugin(item)">{{ t('common.edit') }}</button>
                  <button class="btn-secondary px-2 py-1 text-xs" @click="togglePlugin(item, !item.enabled)">
                    {{ item.enabled ? t('plugin_sdk.list.disable') : t('plugin_sdk.list.enable') }}
                  </button>
                  <button class="btn-danger px-2 py-1 text-xs" @click="deletePlugin(item)">{{ t('common.delete') }}</button>
                </div>
              </td>
            </tr>
            <tr v-if="plugins.length === 0">
              <td colspan="5" class="py-6 text-center text-gray-500">{{ t('plugin_sdk.list.empty') }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
  </div>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import api from '../services/api'

const { t } = useI18n({ useScope: 'global' })

const plugins = ref([])
const sdkInfo = ref({})
const saving = ref(false)
const error = ref('')
const success = ref('')

const form = ref({
  id: '',
  name: '',
  version: '0.1.0',
  author: '',
  entrypoint: '',
  description: '',
  hooks_csv: '',
  permissions_csv: '',
  config_schema: '',
  enabled: true,
})

function apiErrorMessage(err, fallbackKey) {
  return err?.response?.data?.message || err?.message || t(fallbackKey)
}

function normalizeCSV(value) {
  return String(value || '')
    .split(',')
    .map((item) => item.trim())
    .filter(Boolean)
}

function resetForm() {
  form.value = {
    id: '',
    name: '',
    version: '0.1.0',
    author: '',
    entrypoint: '',
    description: '',
    hooks_csv: '',
    permissions_csv: '',
    config_schema: '',
    enabled: true,
  }
}

function editPlugin(item) {
  form.value = {
    id: item.id || '',
    name: item.name || '',
    version: item.version || '0.1.0',
    author: item.author || '',
    entrypoint: item.entrypoint || '',
    description: item.description || '',
    hooks_csv: (item.hooks || []).join(', '),
    permissions_csv: (item.permissions || []).join(', '),
    config_schema: item.config_schema || '',
    enabled: !!item.enabled,
  }
}

async function loadPlugins() {
  const res = await api.get('/plugins/list')
  plugins.value = Array.isArray(res.data?.data) ? res.data.data : []
}

async function loadSDKInfo() {
  const res = await api.get('/plugins/sdk/info')
  sdkInfo.value = res.data?.data || {}
}

async function loadAll() {
  error.value = ''
  success.value = ''
  try {
    await Promise.all([loadPlugins(), loadSDKInfo()])
  } catch (err) {
    error.value = apiErrorMessage(err, 'plugin_sdk.messages.load_failed')
  }
}

async function savePlugin() {
  error.value = ''
  success.value = ''
  saving.value = true
  try {
    await api.post('/plugins/save', {
      id: form.value.id,
      name: form.value.name,
      version: form.value.version,
      author: form.value.author,
      entrypoint: form.value.entrypoint,
      description: form.value.description,
      hooks: normalizeCSV(form.value.hooks_csv),
      permissions: normalizeCSV(form.value.permissions_csv),
      config_schema: form.value.config_schema,
      enabled: !!form.value.enabled,
    })
    success.value = t('plugin_sdk.messages.saved')
    resetForm()
    await loadPlugins()
  } catch (err) {
    error.value = apiErrorMessage(err, 'plugin_sdk.messages.save_failed')
  } finally {
    saving.value = false
  }
}

async function togglePlugin(item, enabled) {
  error.value = ''
  success.value = ''
  try {
    await api.post('/plugins/toggle', { id: item.id, enabled })
    success.value = t('plugin_sdk.messages.toggled')
    await loadPlugins()
  } catch (err) {
    error.value = apiErrorMessage(err, 'plugin_sdk.messages.toggle_failed')
  }
}

async function deletePlugin(item) {
  if (!window.confirm(t('plugin_sdk.messages.delete_confirm', { name: item.name || item.id }))) return
  error.value = ''
  success.value = ''
  try {
    await api.post('/plugins/delete', { id: item.id })
    success.value = t('plugin_sdk.messages.deleted')
    await loadPlugins()
  } catch (err) {
    error.value = apiErrorMessage(err, 'plugin_sdk.messages.delete_failed')
  }
}

onMounted(loadAll)
</script>
