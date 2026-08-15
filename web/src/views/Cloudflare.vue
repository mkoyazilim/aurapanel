<template>
  <Layout>
    <div class="page">
      <h1>{{ $t('cloudflare.title') }}</h1>
      <div v-if="loading" class="muted">{{ $t('cloudflare.loading') }}</div>
      <div v-else>
        <div v-if="error" class="alert error">{{ error }}</div>
        <form class="card" @submit.prevent="saveSettings">
          <h2>{{ $t('cloudflare.settings') }}</h2>
          <label>{{ $t('cloudflare.api_token') }}</label>
          <input v-model="form.api_token" type="password" required :placeholder="$t('cloudflare.api_token_placeholder')" />
          <p class="muted" style="margin-top: 2px; font-size: 12px">{{ $t('cloudflare.api_token_hint') }}</p>
          
          <label style="margin-top: 16px">{{ $t('cloudflare.zone_id') }}</label>
          <input v-model="form.zone_id" type="text" required :placeholder="$t('cloudflare.zone_id_placeholder')" />
          
          <div style="margin-top: 16px; display: flex; align-items: center; gap: 8px;">
            <input v-model="form.proxy_enabled" type="checkbox" id="proxy_enabled" />
            <label for="proxy_enabled" style="margin: 0;">{{ $t('cloudflare.enable_proxy') }}</label>
          </div>
          
          <div style="margin-top: 24px; display: flex; gap: 8px;">
            <button type="submit" class="btn primary" :disabled="submitting">
              {{ submitting ? $t('cloudflare.saving') : $t('cloudflare.save_settings') }}
            </button>
            <button type="button" @click="purgeCache" :disabled="!form.api_token || !form.zone_id" class="btn danger">
              {{ $t('cloudflare.purge_cache') }}
            </button>
          </div>
        </form>
      </div>
    </div>
  </Layout>
</template>


<script setup>
import Layout from '../components/Layout.vue'
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
