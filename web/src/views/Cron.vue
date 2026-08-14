<template>
  <Layout>
    <div class="page">
      <div style="margin-bottom: 20px">
        <h1 style="margin: 0; font-size: 22px; display: flex; align-items: center; gap: 8px">
          ⏰ {{ $t('cron.title') }}
        </h1>
        <p class="muted text-sm" style="margin: 4px 0 0 0">{{ $t('cron.subtitle') }}</p>
      </div>

      <div v-if="error" class="alert error">{{ error }}</div>
      <div v-if="notice" class="alert ok">{{ notice }}</div>

      <!-- Kontrol & Site Seçim Kartı -->
      <div class="card">
        <div class="row" style="align-items: flex-end">
          <div style="flex: 2; min-width: 240px">
            <label style="margin: 0 0 6px 0; font-weight: 600">🌐 Site Seçin</label>
            <select v-model="selectedSite" @change="loadCrons" style="width: 100%">
              <option value="" disabled>{{ $t('cron.select_site') }}</option>
              <option v-for="s in sites" :key="s.id" :value="s.id">{{ s.name }} ({{ s.id }})</option>
            </select>
          </div>
          <div style="flex: 1; display: flex; justify-content: flex-end">
            <button v-if="selectedSite" class="btn primary" @click="openCreateModal">
              ➕ {{ $t('cron.add_job') }}
            </button>
          </div>
        </div>
      </div>

      <!-- Site Seçilmemiş Durumu -->
      <div v-if="!selectedSite" class="card empty-state">
        <div style="font-size: 40px; margin-bottom: 12px">⏰</div>
        <p class="muted">{{ $t('cron.select_site_prompt') }}</p>
      </div>

      <!-- Yükleniyor Durumu -->
      <div v-else-if="loading" class="card empty-state">
        <p class="muted">{{ $t('common.loading') }}</p>
      </div>

      <!-- Cron Görevleri Listesi -->
      <div v-else class="card">
        <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 16px">
          <h2 style="margin: 0">{{ selectedSite }} — Zamanlanmış Görevler</h2>
          <span class="badge" style="background: rgba(0,0,0,0.06)">{{ jobs.length }} Görev</span>
        </div>

        <div v-if="jobs.length === 0" class="empty-state" style="padding: 32px">
          <div style="font-size: 32px; margin-bottom: 8px">📭</div>
          <p class="muted">{{ $t('cron.no_jobs') }}</p>
          <button class="btn primary btn-sm" style="margin-top: 10px" @click="openCreateModal">
            İlk Görevi Ekle
          </button>
        </div>

        <table v-else>
          <thead>
            <tr>
              <th>{{ $t('cron.label') }}</th>
              <th>{{ $t('cron.schedule') }}</th>
              <th>{{ $t('cron.command') }}</th>
              <th>{{ $t('cron.status') }}</th>
              <th style="text-align: right">{{ $t('common.actions') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="j in jobs" :key="j.id">
              <td>
                <div style="font-weight: 600">{{ j.label || $t('cron.unlabeled') }}</div>
              </td>
              <td>
                <span class="cron-pill mono">⏱️ {{ j.schedule }}</span>
              </td>
              <td>
                <code class="command-box mono">{{ j.command }}</code>
              </td>
              <td>
                <span class="badge" :class="j.enabled ? 'ok' : 'err'">
                  {{ j.enabled ? '● ' + $t('cron.active') : '○ ' + $t('cron.disabled') }}
                </span>
              </td>
              <td style="text-align: right">
                <button class="btn danger btn-sm" @click="deleteJob(j.id)" title="Görevi Sil">
                  🗑️ {{ $t('common.delete') }}
                </button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>

      <!-- Yeni Görev Ekleme Modalı -->
      <div v-if="showModal" class="modal-backdrop">
        <div class="modal-card">
          <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 16px">
            <h3 style="margin: 0">⏰ {{ $t('cron.create_title') }}</h3>
            <button class="btn btn-sm" @click="showModal = false">✕</button>
          </div>

          <form @submit.prevent="saveJob">
            <div style="margin-bottom: 14px">
              <label style="margin: 0 0 6px 0; font-weight: 600">{{ $t('cron.label') }}</label>
              <input v-model="form.label" type="text" :placeholder="$t('cron.label_placeholder')" />
            </div>

            <div style="margin-bottom: 14px">
              <div style="display: flex; justify-content: space-between; align-items: baseline; margin-bottom: 6px">
                <label style="margin: 0; font-weight: 600">{{ $t('cron.schedule') }} (5 alan)</label>
                <small class="muted">Örn: dakika saat gün ay hafta-günü</small>
              </div>
              <input v-model="form.schedule" type="text" placeholder="0 * * * *" class="mono" required />
              
              <!-- Hazır Şablonlar -->
              <div class="preset-pills" style="margin-top: 8px">
                <button type="button" class="preset-btn" @click="form.schedule = '*/15 * * * *'">15 dk bir</button>
                <button type="button" class="preset-btn" @click="form.schedule = '0 * * * *'">Her saat</button>
                <button type="button" class="preset-btn" @click="form.schedule = '0 0 * * *'">Her gece</button>
                <button type="button" class="preset-btn" @click="form.schedule = '0 0 * * 0'">Haftada bir</button>
              </div>
            </div>

            <div style="margin-bottom: 16px">
              <label style="margin: 0 0 6px 0; font-weight: 600">{{ $t('cron.command') }}</label>
              <input v-model="form.command" type="text" placeholder="/usr/bin/php /srv/aurapanel/sites/.../artisan schedule:run" class="mono" required />
              <small class="muted" style="display: block; margin-top: 4px">{{ $t('cron.command_hint') }}</small>
            </div>

            <div style="display: flex; justify-content: flex-end; gap: 8px; margin-top: 20px">
              <button type="button" class="btn" @click="showModal = false" :disabled="saving">{{ $t('common.cancel') }}</button>
              <button type="submit" class="btn primary" :disabled="saving">
                {{ saving ? $t('common.saving') : '💾 ' + $t('common.save') }}
              </button>
            </div>
          </form>
        </div>
      </div>
    </div>
  </Layout>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import Layout from '../components/Layout.vue'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()
const sites = ref([])
const selectedSite = ref('')
const jobs = ref([])
const loading = ref(false)
const saving = ref(false)
const showModal = ref(false)
const error = ref('')
const notice = ref('')

const form = ref({
  label: '',
  schedule: '0 * * * *',
  command: '',
  enabled: true
})

async function fetchSites() {
  try {
    const res = await fetch('/api/v1/sites')
    if (res.ok) {
      sites.value = await res.json()
      if (sites.value.length > 0) {
        selectedSite.value = sites.value[0].id
        loadCrons()
      }
    }
  } catch (err) {
    console.error('Siteler alınamadı:', err)
  }
}

async function loadCrons() {
  if (!selectedSite.value) return
  loading.value = true
  error.value = ''
  try {
    const res = await fetch(`/api/v1/sites/${selectedSite.value}/crons`)
    if (res.ok) {
      jobs.value = await res.json() || []
    }
  } catch (err) {
    error.value = err.message
  } finally {
    loading.value = false
  }
}

function openCreateModal() {
  form.value = { label: '', schedule: '0 * * * *', command: '', enabled: true }
  showModal.value = true
}

async function saveJob() {
  saving.value = true
  error.value = ''
  notice.value = ''
  try {
    const res = await fetch(`/api/v1/sites/${selectedSite.value}/crons`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(form.value)
    })
    if (res.ok) {
      showModal.value = false
      notice.value = 'Cron görevi başarıyla oluşturuldu.'
      await loadCrons()
    } else {
      const data = await res.json()
      error.value = data.error || t('cron.create_failed')
    }
  } catch (err) {
    error.value = err.message
  } finally {
    saving.value = false
  }
}

async function deleteJob(id) {
  if (!confirm(t('cron.delete_confirm'))) return
  error.value = ''
  notice.value = ''
  try {
    const res = await fetch(`/api/v1/sites/${selectedSite.value}/crons/${id}`, {
      method: 'DELETE'
    })
    if (res.ok) {
      notice.value = 'Cron görevi silindi.'
      await loadCrons()
    } else {
      const data = await res.json()
      error.value = data.error || t('cron.delete_failed')
    }
  } catch (err) {
    error.value = err.message
  }
}

onMounted(fetchSites)
</script>

<style scoped>
.empty-state {
  text-align: center;
  padding: 48px 20px;
}

.cron-pill {
  background: #f1f5f9;
  border: 1px solid #e2e8f0;
  padding: 3px 8px;
  border-radius: 6px;
  font-size: 12px;
  color: #334155;
}

.command-box {
  background: #f8fafc;
  border: 1px solid #e2e8f0;
  padding: 4px 8px;
  border-radius: 6px;
  font-size: 12px;
  color: #0f172a;
  word-break: break-all;
  display: inline-block;
  max-width: 460px;
}

.preset-pills {
  display: flex;
  gap: 6px;
  flex-wrap: wrap;
}

.preset-btn {
  background: #f1f5f9;
  border: 1px solid #e2e8f0;
  border-radius: 6px;
  padding: 3px 8px;
  font-size: 11px;
  color: #475569;
  cursor: pointer;
  transition: all 0.15s ease;
}

.preset-btn:hover {
  background: #e2e8f0;
  color: #1e293b;
}

.modal-backdrop {
  position: fixed;
  inset: 0;
  background: rgba(15, 23, 42, 0.6);
  backdrop-filter: blur(4px);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
}

.modal-card {
  background: var(--card, #ffffff);
  border: 1px solid var(--border, #e2e8f0);
  border-radius: 12px;
  width: 100%;
  max-width: 520px;
  padding: 24px;
  box-shadow: 0 20px 25px -5px rgba(0, 0, 0, 0.2);
}
</style>
