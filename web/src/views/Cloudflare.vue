<template>
  <div>
    <h2 class="text-2xl font-bold mb-6">{{ $t('cloudflare.title') }}</h2>

    <div v-if="loading" class="text-gray-500">
      {{ $t('cloudflare.loading') }}
    </div>
    <div v-else>
      <div v-if="error" class="bg-red-100 text-red-600 p-3 rounded-md mb-4">{{ error }}</div>

      <div class="bg-white p-6 rounded-lg shadow-sm border border-gray-200 dark:bg-gray-800 dark:border-gray-700">
        <h3 class="text-lg font-bold mb-4">{{ $t('cloudflare.settings') }}</h3>
        
        <form @submit.prevent="saveSettings" class="space-y-4 max-w-2xl">
          <div>
            <label class="block text-sm font-medium mb-1">{{ $t('cloudflare.api_token') }}</label>
            <input v-model="form.api_token" type="password" required :placeholder="$t('cloudflare.api_token_placeholder')"
                   class="w-full px-3 py-2 border rounded-md dark:bg-gray-700 dark:border-gray-600">
            <p class="text-xs text-gray-500 mt-1">{{ $t('cloudflare.api_token_hint') }}</p>
          </div>
          <div>
            <label class="block text-sm font-medium mb-1">{{ $t('cloudflare.zone_id') }}</label>
            <input v-model="form.zone_id" type="text" required :placeholder="$t('cloudflare.zone_id_placeholder')"
                   class="w-full px-3 py-2 border rounded-md dark:bg-gray-700 dark:border-gray-600">
          </div>
          <div class="flex items-center mt-4">
            <input v-model="form.proxy_enabled" type="checkbox" id="proxy_enabled" class="h-4 w-4 text-blue-600 rounded">
            <label for="proxy_enabled" class="ml-2 block text-sm font-medium text-gray-900 dark:text-gray-300">
              {{ $t('cloudflare.enable_proxy') }}
            </label>
          </div>
          
          <div class="pt-4 flex justify-between items-center">
            <button type="submit" :disabled="submitting" class="bg-blue-600 text-white px-6 py-2 rounded-md hover:bg-blue-700 disabled:opacity-50">
              {{ submitting ? $t('cloudflare.saving') : $t('cloudflare.save_settings') }}
            </button>
            <button type="button" @click="purgeCache" :disabled="!form.api_token || !form.zone_id" class="border border-orange-500 text-orange-600 px-4 py-2 rounded hover:bg-orange-50 dark:hover:bg-gray-700 disabled:opacity-50">
              {{ $t('cloudflare.purge_cache') }}
            </button>
          </div>
        </form>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { api } from '../api'

const { t } = useI18n()
const route = useRoute()
const siteId = route.params.id

const form = ref({
  api_token: '',
  zone_id: '',
  proxy_enabled: true
})
const loading = ref(true)
const error = ref('')
const submitting = ref(false)

const fetchSettings = async () => {
  try {
    const data = await api.get(`/sites/${siteId}/cloudflare`)
    if (data && data.api_token) {
      form.value = {
        api_token: data.api_token,
        zone_id: data.zone_id,
        proxy_enabled: data.proxy_enabled
      }
    }
  } catch (err) {
    error.value = err.message
  } finally {
    loading.value = false
  }
}

const saveSettings = async () => {
  submitting.value = true
  try {
    await api.post(`/sites/${siteId}/cloudflare`, form.value)
    alert(t('cloudflare.settings_saved'))
  } catch (err) {
    alert(t('cloudflare.error', { message: err.message }))
  } finally {
    submitting.value = false
  }
}

const purgeCache = async () => {
  if (!confirm(t('cloudflare.confirm_purge'))) return
  try {
    await api.post(`/sites/${siteId}/cloudflare/purge`)
    alert(t('cloudflare.cache_purged'))
  } catch (err) {
    alert(t('cloudflare.error_purging', { message: err.message }))
  }
}

onMounted(() => {
  fetchSettings()
})
</script>
