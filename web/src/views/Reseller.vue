<script setup>
import { ref, onMounted } from 'vue'
import Layout from '../components/Layout.vue'

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
    const res = await fetch('/api/v1/resellers')
    if (res.ok) {
      resellers.value = await res.json()
    } else {
      const d = await res.json().catch(() => ({}))
      error.value = d.error || 'Yükleme hatası'
    }
  } catch {
    error.value = 'Bağlantı hatası'
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
    addError.value = 'Kullanıcı adı ve şifre gereklidir.'
    return
  }
  const res = await fetch('/api/v1/resellers', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(addForm.value),
  })
  if (!res.ok) {
    const d = await res.json().catch(() => ({}))
    addError.value = d.error || 'Oluşturma hatası'
    return
  }
  notice.value = 'Reseller oluşturuldu.'
  showAddModal.value = false
  loadResellers()
}

async function deleteReseller(id, username) {
  if (!confirm(`"${username}" adlı reseller silinecek. Emin misiniz?`)) return
  error.value = ''
  const res = await fetch(`/api/v1/resellers/${id}`, { method: 'DELETE' })
  if (!res.ok) {
    const d = await res.json().catch(() => ({}))
    error.value = d.error || 'Silme hatası'
    return
  }
  notice.value = 'Reseller silindi.'
  loadResellers()
}

async function openQuotaModal(r) {
  quotaError.value = ''
  quotaResellerId.value = r.id
  quotaForm.value = { max_sites: 0, max_databases: 0, max_disk_gb: 0, max_bandwidth_gb: 0 }
  const res = await fetch(`/api/v1/resellers/${r.id}/quota`)
  if (res.ok) {
    const q = await res.json()
    quotaForm.value = {
      max_sites: q.max_sites ?? 0,
      max_databases: q.max_databases ?? 0,
      max_disk_gb: q.max_disk_gb ?? 0,
      max_bandwidth_gb: q.max_bandwidth_gb ?? 0,
    }
  }
  showQuotaModal.value = true
}

async function saveQuota() {
  quotaError.value = ''
  const res = await fetch(`/api/v1/resellers/${quotaResellerId.value}/quota`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      max_sites: Number(quotaForm.value.max_sites),
      max_databases: Number(quotaForm.value.max_databases),
      max_disk_gb: Number(quotaForm.value.max_disk_gb),
      max_bandwidth_gb: Number(quotaForm.value.max_bandwidth_gb),
    }),
  })
  if (!res.ok) {
    const d = await res.json().catch(() => ({}))
    quotaError.value = d.error || 'Kaydetme hatası'
    return
  }
  notice.value = 'Kota güncellendi.'
  showQuotaModal.value = false
  loadResellers()
}

onMounted(loadResellers)
</script>

<template>
  <Layout>
    <div class="page">
      <div class="page-header">
        <h1>Reseller Yönetimi</h1>
        <button class="btn primary" @click="openAddModal">
          <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" class="btn-icon"><path stroke-linecap="round" stroke-linejoin="round" d="M12 4.5v15m7.5-7.5h-15" /></svg>
          Yeni Reseller
        </button>
      </div>

      <div v-if="error" class="alert error">{{ error }}</div>
      <div v-if="notice" class="alert ok">{{ notice }}</div>

      <div class="card">
        <div v-if="loading" class="muted">Yükleniyor...</div>
        <table v-else>
          <thead>
            <tr>
              <th>ID</th>
              <th>Kullanıcı Adı</th>
              <th>Durum</th>
              <th>Siteler</th>
              <th>Oluşturulma</th>
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
                <button class="btn" @click="openQuotaModal(r)">Kota Düzenle</button>
                <button class="btn danger" @click="deleteReseller(r.id, r.username)">Sil</button>
              </td>
            </tr>
            <tr v-if="!resellers.length">
              <td colspan="6" class="muted">Henüz reseller yok.</td>
            </tr>
          </tbody>
        </table>
      </div>

      <!-- Add Reseller Modal -->
      <Teleport to="body">
        <div v-if="showAddModal" class="modal-overlay" @click.self="showAddModal = false">
          <div class="modal-box">
            <div class="modal-header">
              <h2>Yeni Reseller Ekle</h2>
              <button class="modal-close" @click="showAddModal = false" title="Kapat">✕</button>
            </div>
            <div v-if="addError" class="alert error">{{ addError }}</div>
            <div class="field">
              <label>Kullanıcı Adı</label>
              <input v-model="addForm.username" placeholder="kullanici_adi" autocomplete="off" />
            </div>
            <div class="field">
              <label>Şifre</label>
              <input v-model="addForm.password" type="password" placeholder="••••••••" autocomplete="new-password" />
            </div>
            <div class="modal-actions">
              <button class="btn" @click="showAddModal = false">İptal</button>
              <button class="btn primary" @click="createReseller">Oluştur</button>
            </div>
          </div>
        </div>
      </Teleport>

      <!-- Quota Edit Modal -->
      <Teleport to="body">
        <div v-if="showQuotaModal" class="modal-overlay" @click.self="showQuotaModal = false">
          <div class="modal-box">
            <div class="modal-header">
              <h2>Kota Düzenle</h2>
              <button class="modal-close" @click="showQuotaModal = false" title="Kapat">✕</button>
            </div>
            <div v-if="quotaError" class="alert error">{{ quotaError }}</div>
            <div class="field">
              <label>Maks. Site Sayısı</label>
              <input v-model.number="quotaForm.max_sites" type="number" min="0" step="1" />
            </div>
            <div class="field">
              <label>Maks. Veritabanı Sayısı</label>
              <input v-model.number="quotaForm.max_databases" type="number" min="0" step="1" />
            </div>
            <div class="field">
              <label>Maks. Disk (GB)</label>
              <input v-model.number="quotaForm.max_disk_gb" type="number" min="0" step="1" />
            </div>
            <div class="field">
              <label>Maks. Bant Genişliği (GB)</label>
              <input v-model.number="quotaForm.max_bandwidth_gb" type="number" min="0" step="1" />
            </div>
            <div class="modal-actions">
              <button class="btn" @click="showQuotaModal = false">İptal</button>
              <button class="btn primary" @click="saveQuota">Kaydet</button>
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
