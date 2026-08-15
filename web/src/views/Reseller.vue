<script setup>
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import Layout from '../components/Layout.vue'
import { api } from '../api.js'

const { t } = useI18n()

const resellers = ref([])
const loading = ref(true)
const error = ref('')
const notice = ref('')

// Add modal
const showAddModal = ref(false)
const addForm = ref({ username: '', password: '' })
const addError = ref('')

// Quota modal
const showQuotaModal = ref(false)
const quotaResellerId = ref(null)
const quotaForm = ref({ max_sites: 0, max_databases: 0, max_disk_gb: 0, max_bandwidth_gb: 0 })
const quotaError = ref('')

async function loadResellers() {
  loading.value = true
  error.value = ''
  try {
    resellers.value = await api.get('/resellers') || []
  } catch (err) {
    error.value = err.message || t('reseller.load_error')
  } finally {
    loading.value = false
  }
}

function openAddModal() {
  addForm.value = { username: '', password: '' }
  addError.value = ''
  showAddModal.value = true
}

async function createReseller() {
  addError.value = ''
  if (!addForm.value.username || !addForm.value.password) {
    addError.value = t('reseller.username_password_required')
    return
  }
  try {
    await api.post('/resellers', addForm.value)
  } catch (err) {
    addError.value = err.message || t('reseller.create_error')
    return
  }
  notice.value = t('reseller.reseller_created')
  showAddModal.value = false
  loadResellers()
}

async function deleteReseller(id, username) {
  if (!confirm(t('reseller.delete_confirm', { username }))) return
  error.value = ''
  try {
    await api.delete(`/resellers/${id}`)
  } catch (err) {
    error.value = err.message || t('reseller.delete_error')
    return
  }
  notice.value = t('reseller.reseller_deleted')
  loadResellers()
}

async function openQuotaModal(r) {
  quotaError.value = ''
  quotaResellerId.value = r.id
  quotaForm.value = { max_sites: 0, max_databases: 0, max_disk_gb: 0, max_bandwidth_gb: 0 }
  try {
    const q = await api.get(`/resellers/${r.id}/quota`)
    if (q) {
      quotaForm.value = {
        max_sites: q.max_sites ?? 0,
        max_databases: q.max_databases ?? 0,
        max_disk_gb: q.max_disk_gb ?? 0,
        max_bandwidth_gb: q.max_bandwidth_gb ?? 0,
      }
    }
  } catch (err) {
    quotaError.value = t('reseller.quota_load_error', { message: err.message })
  }
  showQuotaModal.value = true
}

async function saveQuota() {
  quotaError.value = ''
  try {
    await api.put(`/resellers/${quotaResellerId.value}/quota`, {
      max_sites: Number(quotaForm.value.max_sites),
      max_databases: Number(quotaForm.value.max_databases),
      max_disk_gb: Number(quotaForm.value.max_disk_gb),
      max_bandwidth_gb: Number(quotaForm.value.max_bandwidth_gb),
    })
  } catch (err) {
    quotaError.value = err.message || t('reseller.save_error')
    return
  }
  notice.value = t('reseller.quota_updated')
  showQuotaModal.value = false
  loadResellers()
}

onMounted(loadResellers)
</script>

<template>
  <Layout>
    <div class="page">
      <div class="page-header">
        <h1>{{ $t('reseller.reseller_management') }}</h1>
        <button class="btn primary" @click="openAddModal">
          <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" class="btn-icon"><path stroke-linecap="round" stroke-linejoin="round" d="M12 4.5v15m7.5-7.5h-15" /></svg>
          {{ $t('reseller.new_reseller') }}
        </button>
      </div>

      <div v-if="error" class="alert error">{{ error }}</div>
      <div v-if="notice" class="alert ok">{{ notice }}</div>

      <div class="card">
        <div v-if="loading" class="muted">{{ $t('reseller.loading') }}</div>
        <table v-else>
          <thead>
            <tr>
              <th>{{ $t('reseller.id') }}</th>
              <th>{{ $t('reseller.username') }}</th>
              <th>{{ $t('reseller.status') }}</th>
              <th>{{ $t('reseller.sites') }}</th>
              <th>{{ $t('reseller.created_at') }}</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="r in resellers" :key="r.id">
              <td class="mono">{{ r.id }}</td>
              <td>{{ r.username }}</td>
              <td>
                <span class="badge" :class="r.status === 'active' ? 'ok' : 'error'">{{ r.status }}</span>
              </td>
              <td>{{ r.site_count ?? 0 }} / {{ r.max_sites ?? '∞' }}</td>
              <td class="muted">{{ r.created_at ? new Date(r.created_at).toLocaleDateString('tr-TR') : '—' }}</td>
              <td class="actions-cell">
                <button class="btn" @click="openQuotaModal(r)">{{ $t('reseller.edit_quota') }}</button>
                <button class="btn danger" @click="deleteReseller(r.id, r.username)">{{ $t('reseller.delete') }}</button>
              </td>
            </tr>
            <tr v-if="!resellers.length">
              <td colspan="6" class="muted">{{ $t('reseller.no_resellers') }}</td>
            </tr>
          </tbody>
        </table>
      </div>

      <!-- Add Reseller Modal -->
      <Teleport to="body">
        <div v-if="showAddModal" class="modal-overlay" @click.self="showAddModal = false">
          <div class="modal-box">
            <div class="modal-header">
              <h2>{{ $t('reseller.add_new_reseller') }}</h2>
              <button class="modal-close" @click="showAddModal = false" :title="$t('reseller.close')">✕</button>
            </div>
            <div v-if="addError" class="alert error">{{ addError }}</div>
            <div class="field">
              <label>{{ $t('reseller.username') }}</label>
              <input v-model="addForm.username" :placeholder="$t('reseller.username_placeholder')" autocomplete="off" />
            </div>
            <div class="field">
              <label>{{ $t('reseller.password') }}</label>
              <input v-model="addForm.password" type="password" placeholder="••••••••" autocomplete="new-password" />
            </div>
            <div class="modal-actions">
              <button class="btn" @click="showAddModal = false">{{ $t('reseller.cancel') }}</button>
              <button class="btn primary" @click="createReseller">{{ $t('reseller.create') }}</button>
            </div>
          </div>
        </div>
      </Teleport>

      <!-- Quota Edit Modal -->
      <Teleport to="body">
        <div v-if="showQuotaModal" class="modal-overlay" @click.self="showQuotaModal = false">
          <div class="modal-box">
            <div class="modal-header">
              <h2>{{ $t('reseller.edit_quota') }}</h2>
              <button class="modal-close" @click="showQuotaModal = false" :title="$t('reseller.close')">✕</button>
            </div>
            <div v-if="quotaError" class="alert error">{{ quotaError }}</div>
            <div class="field">
              <label>{{ $t('reseller.max_sites') }}</label>
              <input v-model.number="quotaForm.max_sites" type="number" min="0" step="1" />
            </div>
            <div class="field">
              <label>{{ $t('reseller.max_databases') }}</label>
              <input v-model.number="quotaForm.max_databases" type="number" min="0" step="1" />
            </div>
            <div class="field">
              <label>{{ $t('reseller.max_disk_gb') }}</label>
              <input v-model.number="quotaForm.max_disk_gb" type="number" min="0" step="1" />
            </div>
            <div class="field">
              <label>{{ $t('reseller.max_bandwidth_gb') }}</label>
              <input v-model.number="quotaForm.max_bandwidth_gb" type="number" min="0" step="1" />
            </div>
            <div class="modal-actions">
              <button class="btn" @click="showQuotaModal = false">{{ $t('reseller.cancel') }}</button>
              <button class="btn primary" @click="saveQuota">{{ $t('reseller.save') }}</button>
            </div>
          </div>
        </div>
      </Teleport>
    </div>
  </Layout>
</template>

<style scoped>
.page-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 20px;
}
.page-header h1 { margin: 0; }
.btn-icon { width: 16px; height: 16px; display: inline; vertical-align: middle; margin-right: 4px; }
.actions-cell { display: flex; gap: 8px; }

/* Modal */
.modal-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.35);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 200;
}
.modal-box {
  background: #fff;
  border-radius: 12px;
  padding: 28px 32px;
  min-width: 360px;
  max-width: 480px;
  width: 100%;
  box-shadow: 0 8px 40px rgba(0, 0, 0, 0.18);
}
.modal-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 20px;
}
.modal-header h2 { margin: 0; font-size: 18px; }
.modal-close {
  background: none;
  border: none;
  cursor: pointer;
  font-size: 16px;
  color: #94a3b8;
  padding: 4px 8px;
  border-radius: 6px;
  line-height: 1;
}
.modal-close:hover { background: #f1f5f9; color: #0f172a; }
.field { margin-bottom: 16px; display: flex; flex-direction: column; gap: 6px; }
.field label { font-size: 13px; font-weight: 600; color: #475569; }
.modal-actions { display: flex; justify-content: flex-end; gap: 10px; margin-top: 24px; }
</style>
