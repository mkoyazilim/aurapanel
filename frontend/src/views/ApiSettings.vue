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
import { ref, onMounted } from 'vue'
import { useNotificationStore } from '../stores/notifications'
import api from '../services/api'
import { Save, RefreshCw, Loader2, Eye, EyeOff } from 'lucide-vue-next'

const notifications = useNotificationStore()
const token = ref('')
const showToken = ref(false)
const saving = ref(false)

const loadToken = async () => {
  try {
    const res = await api.get('/system/reseller-token')
    if (res.data?.token) {
      token.value = res.data.token
    }
  } catch (err) {
    notifications.showError('Failed to load API token')
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
    notifications.showWarning('Token cannot be empty')
    return
  }
  
  saving.value = true
  try {
    await api.post('/system/reseller-token', { token: token.value })
    notifications.showSuccess('API token updated successfully')
  } catch (err) {
    notifications.showError('Failed to save API token')
  } finally {
    saving.value = false
  }
}

onMounted(() => {
  loadToken()
})
</script>
