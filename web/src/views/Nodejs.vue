<template>
  <Layout>
    <div class="page">
      <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 24px;">
        <h1 style="margin: 0">{{ $t('nodejs.title') }}</h1>
        <button @click="showAddModal = true" class="btn primary">{{ $t('nodejs.new_app') }}</button>
      </div>

      <div v-if="loading" class="muted">{{ $t('nodejs.loading') }}</div>
      <div v-else>
        <div v-if="error" class="alert error">{{ error }}</div>
        
        <p class="muted" style="margin-bottom: 24px">{{ $t('nodejs.description') }}</p>

        <div v-if="(apps || []).length > 0" style="display: flex; flex-direction: column; gap: 16px;">
          <div v-for="app in apps" :key="app.id" class="card" style="display: flex; justify-content: space-between; align-items: center;">
            <div>
              <h3>{{ app.app_name }}</h3>
              <p class="muted text-sm" style="margin-top: 4px;">
                {{ $t('nodejs.path_label') }}: <span class="mono">{{ app.app_path }}</span> • 
                {{ $t('nodejs.port_label') }}: <span class="mono">{{ app.port }}</span>
              </p>
            </div>
            <div style="display: flex; align-items: center; gap: 12px;">
              <span class="badge" :class="app.status === 'active' ? 'ok' : 'error'">{{ app.status }}</span>
              <button @click="restartApp(app.id)" class="btn" :title="$t('nodejs.restart_title')">🔄</button>
              <button @click="deleteApp(app.id)" class="btn danger" :title="$t('nodejs.delete_title')">🗑️</button>
            </div>
          </div>
        </div>
        <div v-else class="muted">{{ $t('nodejs.no_apps') }}</div>
      </div>

      <div v-if="showAddModal" class="modal-backdrop">
        <div class="modal-card">
          <h2>{{ $t('nodejs.create_app_title') }}</h2>
          <form @submit.prevent="createApp">
            <label>{{ $t('nodejs.app_name_label') }}</label>
            <input v-model="form.app_name" type="text" required :placeholder="$t('nodejs.app_name_placeholder')" />
            
            <label style="margin-top: 16px">{{ $t('nodejs.path_label') }}</label>
            <input v-model="form.app_path" type="text" required placeholder="/app" />
            
            <div style="display: flex; gap: 8px; margin-top: 16px;">
              <div style="flex: 1">
                <label>{{ $t('nodejs.port_label') }}</label>
                <input v-model.number="form.port" type="number" required />
              </div>
              <div style="flex: 1">
                <label>{{ $t('nodejs.node_version_label') }}</label>
                <select v-model="form.node_version">
                  <option value="18">18.x</option>
                  <option value="20">20.x</option>
                  <option value="22">22.x</option>
                </select>
              </div>
            </div>

            <label style="margin-top: 16px">{{ $t('nodejs.startup_script_label') }}</label>
            <input v-model="form.startup_script" type="text" required :placeholder="$t('nodejs.startup_script_placeholder')" />
            
            <label style="margin-top: 16px">{{ $t('nodejs.env_vars_label') }}</label>
            <textarea v-model="form.env_vars" rows="3" placeholder='{"NODE_ENV": "production"}'></textarea>
            
            <div style="display: flex; justify-content: flex-end; gap: 8px; margin-top: 24px;">
              <button type="button" class="btn" @click="showAddModal = false">{{ $t('nodejs.cancel') }}</button>
              <button type="submit" class="btn primary" :disabled="submitting">
                {{ submitting ? $t('nodejs.creating_app') : $t('nodejs.create_app_btn') }}
              </button>
            </div>
          </form>
        </div>
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

const apps = ref([])
const loading = ref(true)
const error = ref('')
const showAddModal = ref(false)
const submitting = ref(false)

const form = ref({
  app_name: '',
  app_path: '/',
  startup_script: 'npm start',
  port: 3000,
  node_version: '20',
  env_vars: '{}'
})

const fetchApps = async () => {
  try {
    apps.value = await api.get(`/sites/${siteId}/nodejs`) || []
  } catch (err) {
    error.value = err.message
  } finally {
    loading.value = false
  }
}

const createApp = async () => {
  submitting.value = true
  try {
    // Validate JSON
    if (form.value.env_vars) {
      JSON.parse(form.value.env_vars)
    }
    
    await api.post(`/sites/${siteId}/nodejs`, form.value)
    showAddModal.value = false
    form.value = {
      app_name: '',
      app_path: '/',
      startup_script: 'npm start',
      port: 3000,
      node_version: '20',
      env_vars: '{}'
    }
    await fetchApps()
  } catch (err) {
    alert(t('nodejs.error_prefix', { error: err.message }))
  } finally {
    submitting.value = false
  }
}

const deleteApp = async (appId) => {
  if (!confirm(t('nodejs.delete_confirm'))) return
  try {
    await api.delete(`/sites/${siteId}/nodejs/${appId}`)
    await fetchApps()
  } catch (err) {
    alert(t('nodejs.delete_error', { error: err.message }))
  }
}

const restartApp = async (appId) => {
  try {
    await api.post(`/sites/${siteId}/nodejs/${appId}/restart`)
    alert(t('nodejs.restart_success'))
  } catch (err) {
    alert(t('nodejs.restart_error', { error: err.message }))
  }
}

onMounted(() => {
  fetchApps()
})
</script>


<style scoped>
.modal-backdrop {
  position: fixed; inset: 0; background: rgba(15, 23, 42, 0.6);
  backdrop-filter: blur(4px); display: flex; align-items: center;
  justify-content: center; z-index: 1000;
}
.modal-card {
  background: var(--bg-card, #ffffff); border: 1px solid var(--border-color, #e2e8f0);
  border-radius: 12px; width: 100%; max-width: 520px; padding: 24px;
  box-shadow: 0 20px 25px -5px rgba(0, 0, 0, 0.2);
}
</style>
