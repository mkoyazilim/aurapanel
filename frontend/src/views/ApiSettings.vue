<template>
  <div class="max-w-4xl mx-auto space-y-6">
    <div class="flex items-center justify-between">
      <div>
        <h1 class="text-2xl font-bold text-white">{{ t('api_settings.title') }}</h1>
        <p class="text-sm text-gray-400 mt-1">
          {{ t('api_settings.subtitle') }}
        </p>
      </div>
    </div>

    <div class="bg-panel-card border border-panel-border rounded-xl shadow-lg overflow-hidden">
      <div class="p-6 space-y-6">
        
        <div>
          <label class="block text-sm font-medium text-gray-300 mb-2">
            {{ t('api_settings.integration_token') }}
          </label>
          <div class="flex items-center gap-3">
            <input
              v-model="token"
              :type="showToken ? 'text' : 'password'"
              class="aura-input flex-1"
              :placeholder="t('api_settings.token_placeholder')"
            />
            <button
              @click="showToken = !showToken"
              class="btn-secondary px-3 py-2 flex-shrink-0"
              :title="t('api_settings.toggle_visibility')"
            >
              <Eye v-if="!showToken" class="w-4 h-4" />
              <EyeOff v-else class="w-4 h-4" />
            </button>
            <button
              @click="generateToken"
              class="btn-secondary px-3 py-2 flex-shrink-0"
              :title="t('api_settings.generate_new_token')"
            >
              <RefreshCw class="w-4 h-4" />
            </button>
          </div>
          <p class="text-xs text-gray-500 mt-2">
            {{ t('api_settings.token_help') }}
          </p>
        </div>

        <div class="rounded-lg border border-panel-border/70 bg-panel-bg/40 p-4">
          <div class="flex items-center justify-between mb-2">
            <p class="text-sm font-medium text-gray-300">{{ t('api_settings.saved_token') }}</p>
            <p class="text-xs text-gray-500">{{ savedStatusText }}</p>
          </div>

          <div v-if="savedToken" class="flex items-center gap-3">
            <input
              :type="showSavedToken ? 'text' : 'password'"
              :value="savedTokenDisplay"
              class="aura-input flex-1"
              readonly
            />
            <button
              @click="showSavedToken = !showSavedToken"
              class="btn-secondary px-3 py-2 flex-shrink-0"
              :title="t('api_settings.show_hide_saved_token')"
            >
              <Eye v-if="!showSavedToken" class="w-4 h-4" />
              <EyeOff v-else class="w-4 h-4" />
            </button>
            <button
              @click="deleteToken"
              :disabled="deleting"
              class="btn-secondary px-3 py-2 flex-shrink-0 text-red-300 hover:text-red-200 disabled:opacity-60"
              :title="t('api_settings.delete_saved_token')"
            >
              <Loader2 v-if="deleting" class="w-4 h-4 animate-spin" />
              <Trash2 v-else class="w-4 h-4" />
            </button>
          </div>

          <p v-else class="text-xs text-gray-500">
            {{ t('api_settings.no_saved_token') }}
          </p>
        </div>

        <div class="flex items-center gap-4 pt-4 border-t border-panel-border/50">
          <button
            @click="saveToken"
            :disabled="saving"
            class="btn-primary"
          >
            <Loader2 v-if="saving" class="w-4 h-4 mr-2 animate-spin" />
            <Save v-else class="w-4 h-4 mr-2" />
            <span>{{ t('api_settings.save_settings') }}</span>
          </button>
        </div>

      </div>
    </div>
  </div>
</template>

<script setup>
import { computed, ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useNotificationStore } from '../stores/notifications'
import api from '../services/api'
import { Save, RefreshCw, Loader2, Eye, EyeOff, Trash2 } from 'lucide-vue-next'

const { t } = useI18n({ useScope: 'global' })
const notifications = useNotificationStore()
const token = ref('')
const showToken = ref(false)
const saving = ref(false)
const deleting = ref(false)
const savedToken = ref('')
const showSavedToken = ref(false)
const lastSavedAt = ref(null)
const silentHeaders = { headers: { 'X-Aura-Silent-Error': '1' } }

const notify = (type, message) => {
  notifications.add({
    title: t('api_settings.notification_title'),
    message,
    type,
    source: 'api-settings',
  })
}

const maskToken = (raw) => {
  const value = String(raw || '')
  if (!value) return ''
  if (value.length <= 10) return '*'.repeat(value.length)
  return `${value.slice(0, 4)}${'*'.repeat(value.length - 8)}${value.slice(-4)}`
}

const savedTokenDisplay = computed(() => {
  if (!savedToken.value) return ''
  return showSavedToken.value ? savedToken.value : maskToken(savedToken.value)
})

const savedStatusText = computed(() => {
  if (!savedToken.value) return t('api_settings.status.not_saved')
  if (!lastSavedAt.value) return t('api_settings.status.saved')
  return t('api_settings.status.saved_at', { time: new Date(lastSavedAt.value).toLocaleString() })
})

const loadToken = async () => {
  try {
    const res = await api.get('/system/reseller-token', silentHeaders)
    const responseToken = res.data?.token ?? res.data?.data?.token ?? ''
    const responseSavedAt = res.data?.saved_at ?? res.data?.data?.saved_at ?? null
    token.value = responseToken
    savedToken.value = token.value
    showSavedToken.value = false
    if (savedToken.value) {
      lastSavedAt.value = responseSavedAt || Date.now()
    } else {
      lastSavedAt.value = null
    }
  } catch (err) {
    const message = err?.response?.data?.message || t('api_settings.errors.load_failed')
    notify('error', message)
  }
}

const generateToken = () => {
  const chars = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789'
  let result = ''
  for (let i = 0; i < 64; i++) {
    result += chars.charAt(Math.floor(Math.random() * chars.length))
  }
  token.value = result
  showToken.value = true
}

const saveToken = async () => {
  if (!token.value) {
    notify('warning', t('api_settings.errors.empty_token'))
    return
  }
  
  saving.value = true
  try {
    await api.post('/system/reseller-token', { token: token.value }, silentHeaders)
    savedToken.value = token.value
    showSavedToken.value = false
    lastSavedAt.value = Date.now()
    notify('success', t('api_settings.success.updated'))
  } catch (err) {
    const message = err?.response?.data?.message || t('api_settings.errors.save_failed')
    notify('error', message)
  } finally {
    saving.value = false
  }
}

const deleteToken = async () => {
  deleting.value = true
  try {
    await api.delete('/system/reseller-token', silentHeaders)
    token.value = ''
    savedToken.value = ''
    showToken.value = false
    showSavedToken.value = false
    lastSavedAt.value = null
    notify('success', t('api_settings.success.deleted'))
  } catch (err) {
    const message = err?.response?.data?.message || t('api_settings.errors.delete_failed')
    notify('error', message)
  } finally {
    deleting.value = false
  }
}

onMounted(() => {
  loadToken()
})
</script>
