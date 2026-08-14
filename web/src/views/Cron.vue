<template>
  <Layout>
    <div class="page-header">
      <div class="page-title-row">
        <div>
          <h2>{{ $t('cron.title') }}</h2>
          <p class="muted">{{ $t('cron.subtitle') }}</p>
        </div>
        <div class="header-actions">
          <select v-model="selectedSite" @change="loadCrons" class="select-site">
            <option value="" disabled>{{ $t('cron.select_site') }}</option>
            <option v-for="s in sites" :key="s.id" :value="s.id">{{ s.name }} ({{ s.id }})</option>
          </select>
          <button v-if="selectedSite" class="btn btn-primary" @click="openCreateModal">
            + {{ $t('cron.add_job') }}
          </button>
        </div>
      </div>
    </div>

    <div v-if="!selectedSite" class="empty-state card">
      <p class="muted">{{ $t('cron.select_site_prompt') }}</p>
    </div>

    <div v-else-if="loading" class="loading-state">
      <p>{{ $t('common.loading') }}</p>
    </div>

    <div v-else class="card">
      <div v-if="jobs.length === 0" class="empty-state">
        <p class="muted">{{ $t('cron.no_jobs') }}</p>
      </div>

      <table v-else class="table">
        <thead>
          <tr>
            <th>{{ $t('cron.label') }}</th>
            <th>{{ $t('cron.schedule') }}</th>
            <th>{{ $t('cron.command') }}</th>
            <th>{{ $t('cron.status') }}</th>
            <th class="text-right">{{ $t('common.actions') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="j in jobs" :key="j.id">
            <td>
              <strong>{{ j.label || $t('cron.unlabeled') }}</strong>
            </td>
            <td>
              <code class="badge mono">{{ j.schedule }}</code>
            </td>
            <td>
              <code class="mono text-sm">{{ j.command }}</code>
            </td>
            <td>
              <span class="badge" :class="j.enabled ? 'badge-success' : 'badge-muted'">
                {{ j.enabled ? $t('cron.active') : $t('cron.disabled') }}
              </span>
            </td>
            <td class="text-right">
              <button class="btn btn-sm btn-danger" @click="deleteJob(j.id)">
                {{ $t('common.delete') }}
              </button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <!-- Create Modal -->
    <div v-if="showModal" class="modal-backdrop">
      <div class="modal-card">
        <h3>{{ $t('cron.create_title') }}</h3>
        <form @submit.prevent="saveJob">
          <div class="form-group">
            <label>{{ $t('cron.label') }}</label>
            <input v-model="form.label" type="text" :placeholder="$t('cron.label_placeholder')" class="input" />
          </div>
          <div class="form-group">
            <label>{{ $t('cron.schedule') }} (5 {{ $t('cron.fields') }})</label>
            <input v-model="form.schedule" type="text" placeholder="*/15 * * * *" class="input mono" required />
            <small class="muted">{{ $t('cron.schedule_hint') }}</small>
          </div>
          <div class="form-group">
            <label>{{ $t('cron.command') }}</label>
            <input v-model="form.command" type="text" placeholder="/usr/bin/php /srv/aurapanel/sites/.../artisan schedule:run" class="input mono" required />
            <small class="muted">{{ $t('cron.command_hint') }}</small>
          </div>
          <div class="modal-actions">
            <button type="button" class="btn" @click="showModal = false">{{ $t('common.cancel') }}</button>
            <button type="submit" class="btn btn-primary" :disabled="saving">
              {{ saving ? $t('common.saving') : $t('common.save') }}
            </button>
          </div>
        </form>
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
  try {
    const res = await fetch(`/api/v1/sites/${selectedSite.value}/crons`)
    if (res.ok) {
      jobs.value = await res.json() || []
    }
  } catch (err) {
    console.error('Cron listesi hatası:', err)
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
  try {
    const res = await fetch(`/api/v1/sites/${selectedSite.value}/crons`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(form.value)
    })
    if (res.ok) {
      showModal.value = false
      await loadCrons()
    } else {
      const data = await res.json()
      alert(data.error || t('cron.create_failed'))
    }
  } catch (err) {
    alert(err.message)
  } finally {
    saving.value = false
  }
}

async function deleteJob(id) {
  if (!confirm(t('cron.delete_confirm'))) return
  try {
    const res = await fetch(`/api/v1/sites/${selectedSite.value}/crons/${id}`, {
      method: 'DELETE'
    })
    if (res.ok) {
      await loadCrons()
    } else {
      const data = await res.json()
      alert(data.error || t('cron.delete_failed'))
    }
  } catch (err) {
    alert(err.message)
  }
}

onMounted(fetchSites)
</script>

<style scoped>
.page-header { margin-bottom: 24px; }
.page-title-row { display: flex; justify-content: space-between; align-items: center; }
.header-actions { display: flex; gap: 12px; align-items: center; }
.select-site { padding: 8px 12px; border-radius: 6px; border: 1px solid var(--border-color); background: var(--bg-card); color: var(--text-color); font-size: 14px; }
.empty-state { padding: 48px; text-align: center; }
.badge { padding: 4px 8px; border-radius: 4px; font-size: 12px; font-weight: 500; }
.badge-success { background: #dcfce7; color: #166534; }
.badge-muted { background: #f1f5f9; color: #64748b; }
.modal-backdrop { position: fixed; inset: 0; background: rgba(0,0,0,0.5); display: flex; align-items: center; justify-content: center; z-index: 100; }
.modal-card { background: var(--bg-card, #fff); padding: 24px; border-radius: 8px; width: 100%; max-width: 500px; }
.modal-actions { display: flex; justify-content: flex-end; gap: 8px; margin-top: 20px; }
.form-group { margin-bottom: 16px; }
.form-group label { display: block; margin-bottom: 6px; font-size: 13px; font-weight: 500; }
.form-group .input { width: 100%; box-sizing: border-box; }
.form-group small { display: block; margin-top: 4px; font-size: 12px; }
</style>
