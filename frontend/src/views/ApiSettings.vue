<template>
  <div class="max-w-4xl mx-auto space-y-6">
    <div class="flex items-center justify-between">
      <div>
        <h1 class="text-2xl font-bold text-white">Hosting Integration</h1>
        <p class="text-sm text-gray-400 mt-1">
          Manage your AuraPanel API Token for integration with billing systems (WHMCS, AuraPanel Customer, etc.).
        </p>
      </div>
    </div>

    <div class="bg-panel-card border border-panel-border rounded-xl shadow-lg overflow-hidden">
      <div class="p-6 space-y-6">
        
        <div>
          <label class="block text-sm font-medium text-gray-300 mb-2">
            API Integration Token
          </label>
          <div class="flex items-center gap-3">
            <input
              v-model="token"
              :type="showToken ? 'text' : 'password'"
              class="aura-input flex-1"
              placeholder="Enter your API token here..."
            />
            <button
              @click="showToken = !showToken"
              class="btn-secondary px-3 py-2 flex-shrink-0"
              title="Toggle Visibility"
            >
              <Eye v-if="!showToken" class="w-4 h-4" />
              <EyeOff v-else class="w-4 h-4" />
            </button>
            <button
              @click="generateToken"
              class="btn-secondary px-3 py-2 flex-shrink-0"
              title="Generate New Token"
            >
              <RefreshCw class="w-4 h-4" />
            </button>
          </div>
          <p class="text-xs text-gray-500 mt-2">
            This token is used by the billing system to authenticate requests to the AuraPanel API.
            Ensure you keep this token secure and update it in your billing system if changed.
          </p>
        </div>

        <div class="rounded-lg border border-panel-border/70 bg-panel-bg/40 p-4">
          <div class="flex items-center justify-between mb-2">
            <p class="text-sm font-medium text-gray-300">Saved Token</p>
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
              title="Show/Hide Saved Token"
            >
              <Eye v-if="!showSavedToken" class="w-4 h-4" />
              <EyeOff v-else class="w-4 h-4" />
            </button>
            <button
              @click="deleteToken"
              :disabled="deleting"
              class="btn-secondary px-3 py-2 flex-shrink-0 text-red-300 hover:text-red-200 disabled:opacity-60"
              title="Delete Saved Token"
            >
              <Loader2 v-if="deleting" class="w-4 h-4 animate-spin" />
              <Trash2 v-else class="w-4 h-4" />
            </button>
          </div>

          <p v-else class="text-xs text-gray-500">
            No token is saved yet.
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
            <span>Save Settings</span>
          </button>
        </div>

      </div>
    </div>
  </div>
</template>

<script setup>
import { computed, ref, onMounted } from 'vue'
import { useNotificationStore } from '../stores/notifications'
import api from '../services/api'
import { Save, RefreshCw, Loader2, Eye, EyeOff, Trash2 } from 'lucide-vue-next'

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
    title: 'Hosting Integration',
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
  if (!savedToken.value) return 'Not saved'
  if (!lastSavedAt.value) return 'Saved'
  return `Saved at ${new Date(lastSavedAt.value).toLocaleString()}`
})

const loadToken = async () => {
  try {
    const res = await api.get('/system/reseller-token', silentHeaders)
    token.value = res.data?.token || ''
    savedToken.value = token.value
    showSavedToken.value = false
    if (savedToken.value && !lastSavedAt.value) {
      lastSavedAt.value = Date.now()
    }
  } catch (err) {
    const message = err?.response?.data?.message || 'Failed to load API token'
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
    notify('warning', 'Token cannot be empty')
    return
  }
  
  saving.value = true
  try {
    await api.post('/system/reseller-token', { token: token.value }, silentHeaders)
    savedToken.value = token.value
    showSavedToken.value = false
    lastSavedAt.value = Date.now()
    notify('success', 'API token updated successfully')
  } catch (err) {
    const message = err?.response?.data?.message || 'Failed to save API token'
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
    notify('success', 'API token deleted successfully')
  } catch (err) {
    const message = err?.response?.data?.message || 'Failed to delete API token'
    notify('error', message)
  } finally {
    deleting.value = false
  }
}

onMounted(() => {
  loadToken()
})
</script>
