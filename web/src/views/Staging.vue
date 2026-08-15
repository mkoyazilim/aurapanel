<template>
  <Layout>
    <div class="page">
      <h1>{{ $t('staging.title') }}</h1>
      <div v-if="loading" class="muted">{{ $t('staging.loading') }}</div>
      <div v-else>
        <div v-if="error" class="alert error">{{ error }}</div>
        
        <div class="card">
          <h2>{{ $t('staging.active_staging') }}</h2>
          <p class="muted text-sm" style="margin-bottom: 24px">{{ $t('staging.staging_description') }}</p>

          <div v-if="envs.length > 0" style="display: flex; flex-direction: column; gap: 16px;">
            <div v-for="env in envs" :key="env.id" style="border: 1px solid var(--border-color, #e2e8f0); padding: 16px; border-radius: 6px; display: flex; justify-content: space-between; align-items: center;">
              <div>
                <p><strong>{{ $t('staging.staging_site_id') }}:</strong> <span class="mono">{{ env.staging_site_id }}</span></p>
                <p class="muted text-sm">{{ $t('staging.created_at') }}: {{ new Date(env.created_at).toLocaleString() }}</p>
                <span class="badge" :class="env.status === 'active' ? 'ok' : 'warn'" style="margin-top: 8px; display: inline-block;">{{ env.status }}</span>
              </div>
              <div style="display: flex; gap: 8px;">
                <button @click="pushToProduction(env.id)" class="btn primary">{{ $t('staging.push_to_production') }}</button>
                <button @click="deleteStaging(env.id)" class="btn danger">{{ $t('staging.delete_staging') }}</button>
              </div>
            </div>
          </div>
          
          <div v-else>
            <p class="muted" style="margin-bottom: 24px;">{{ $t('staging.no_staging_environment') }}</p>
            <button @click="showAddModal = true" class="btn primary">{{ $t('staging.create_staging_environment') }}</button>
          </div>
        </div>
      </div>

      <div v-if="showAddModal" class="modal-backdrop">
        <div class="modal-card">
          <h2>{{ $t('staging.create_staging_environment') }}</h2>
          <p class="muted text-sm" style="margin-bottom: 16px;">{{ $t('staging.create_modal_desc') }}</p>
          <form @submit.prevent="createStaging">
            <label>{{ $t('staging.staging_domain_label') }}</label>
            <input v-model="stagingDomain" type="text" required :placeholder="$t('staging.staging_domain_placeholder')" />
            
            <div style="display: flex; justify-content: flex-end; gap: 8px; margin-top: 24px;">
              <button type="button" class="btn" @click="showAddModal = false">{{ $t('staging.cancel') }}</button>
              <button type="submit" class="btn primary" :disabled="submitting">
                {{ submitting ? $t('staging.creating') : $t('staging.create') }}
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

const envs = ref([])
const loading = ref(true)
const error = ref('')
const showCreateModal = ref(false)
const submitting = ref(false)

const form = ref({ domain: '' })

const fetchStaging = async () => {
  try {
    envs.value = await api.get(`/sites/${siteId}/staging`)
  } catch (err) {
    error.value = err.message
  } finally {
    loading.value = false
  }
}

const createStaging = async () => {
  submitting.value = true
  try {
    await api.post(`/sites/${siteId}/staging`, form.value)
    showCreateModal.value = false
    form.value.domain = ''
    await fetchStaging()
  } catch (err) {
    alert(`${t('staging.error')}: ${err.message}`)
  } finally {
    submitting.value = false
  }
}

const pushToProduction = async (envId) => {
  if (!confirm(t('staging.confirm_push'))) return
  try {
    await api.post(`/sites/${siteId}/staging/push`)
    alert(t('staging.success_push'))
  } catch (err) {
    alert(`${t('staging.push_error')}: ${err.message}`)
  }
}

const deleteStaging = async (envId) => {
  if (!confirm(t('staging.confirm_delete'))) return
  try {
    await api.delete(`/sites/${siteId}/staging/${envId}`)
    await fetchStaging()
  } catch (err) {
    alert(`${t('staging.delete_error')}: ${err.message}`)
  }
}

onMounted(() => {
  fetchStaging()
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
