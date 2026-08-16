
<template>
  <Layout>
    <div class="page">
      <h1>{{ $t('menu.backups') }}</h1>
      <div v-if="error" class="alert error">{{ error }}</div>
      <div v-if="notice" class="alert ok">{{ notice }}</div>

      <div class="card">
        <div class="row">
          <div style="flex: 1">
            <label>{{ $t('backups.site') }}</label>
            <select v-model="siteId" @change="load">
              <option v-for="s in sites" :key="s.id" :value="s.id">{{ s.name }}</option>
            </select>
          </div>
          <div style="flex: 1">
            <label>{{ $t('backups.kind') }}</label>
            <select v-model="kind">
              <option value="files">{{ $t('backups.kind_files') }}</option>
              <option value="full">{{ $t('backups.kind_full') }}</option>
              <option value="db">{{ $t('backups.kind_db') }}</option>
            </select>
          </div>
          <div style="flex: 1">
            <label>{{ $t('backups.destination') }}</label>
            <select v-model="storage">
              <option value="local">📁 {{ $t('backups.local_storage') }}</option>
              <option value="s3">☁️ {{ $t('backups.s3_cloudflare_r2') }}</option>
            </select>
          </div>
          <button class="btn primary" style="margin-top: 18px" :disabled="busy" @click="run">
            {{ busy ? $t('backups.taking') : '💾 ' + $t('backups.take_backup') }}
          </button>
        </div>
      </div>

      <div class="card">
        <h2>{{ $t('backups.history') }}</h2>
        <table>
          <thead><tr><th>{{ $t('backups.name') }}</th><th>{{ $t('backups.kind') }}</th><th>{{ $t('backups.storage_col') }}</th><th>{{ $t('backups.status') }}</th><th>{{ $t('backups.date') }}</th></tr></thead>
          <tbody>
            <tr v-for="b in backups" :key="b.id">
              <td class="mono">{{ b.location }}</td>
              <td>{{ b.kind }}</td>
              <td>
                <span class="badge" :class="b.storage === 's3' ? 'badge-primary' : 'badge-secondary'">
                  {{ b.storage === 's3' ? '☁️ ' + $t('backups.s3_r2_badge') : '📁 ' + $t('backups.local_badge') }}
                </span>
              </td>
              <td><span class="badge" :class="b.status === 'success' ? 'ok' : 'err'">{{ b.status }}</span></td>
              <td class="muted">{{ b.created_at }}</td>
            </tr>
            <tr v-if="!backups.length"><td colspan="5" class="muted">{{ $t('backups.empty') }}</td></tr>
          </tbody>
        </table>
      </div>

      <!-- Site Bazlı Otomatik Yedekleme -->
      <div class="card" v-if="siteId">
        <h2>⏱️ Otomatik Yedekleme Zamanla</h2>
        <p class="muted text-sm" style="margin-bottom:12px">Bu site için bağımsız otomatik yedekleme zamanı belirleyin. Global ayardan bağımsız çalışır.</p>

        <div style="margin-bottom:12px">
          <label style="margin:0;display:flex;align-items:center;gap:8px;cursor:pointer">
            <input type="checkbox" v-model="siteSchedule.enabled" style="width:auto" />
            <strong>Bu site için otomatik yedekleme aktif</strong>
          </label>
        </div>

        <template v-if="siteSchedule.enabled">
          <div class="row" style="gap:12px;margin-top:10px">
            <div style="flex:1">
              <label>Saat</label>
              <input type="time" v-model="siteSchedule.time" />
            </div>
            <div style="flex:1">
              <label>Sıklık</label>
              <select v-model="siteSchedule.frequency">
                <option value="daily">Her gün</option>
                <option value="monday">Pazartesi</option>
                <option value="tuesday">Salı</option>
                <option value="wednesday">Çarşamba</option>
                <option value="thursday">Perşembe</option>
                <option value="friday">Cuma</option>
                <option value="saturday">Cumartesi</option>
                <option value="sunday">Pazar</option>
              </select>
            </div>
            <div style="flex:1">
              <label>Yedek Türü</label>
              <select v-model="siteSchedule.kind">
                <option value="full">Tam (Dosya + DB)</option>
                <option value="files">Sadece Dosyalar</option>
                <option value="db">Sadece Veritabanı</option>
              </select>
            </div>
          </div>
        </template>

        <div style="margin-top:14px">
          <button class="btn primary" @click="saveSiteSchedule">Kaydet</button>
        </div>
      </div>
    </div>
  </Layout>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import Layout from '../components/Layout.vue'
import { api } from '../api'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()

const sites = ref([])
const siteId = ref('')
const kind = ref('files')
const storage = ref('local')
const backups = ref([])
const error = ref('')
const notice = ref('')
const busy = ref(false)

async function load() {
  if (!siteId.value) return
  try {
    backups.value = await api(`/sites/${siteId.value}/backups`)
    await loadSiteSchedule()
  } catch (e) {
    error.value = e.message
  }
}

async function run() {
  if (!siteId.value) return
  busy.value = true
  error.value = ''
  notice.value = ''
  try {
    const out = await api(`/sites/${siteId.value}/backups/run`, {
      method: 'POST',
      body: { kind: kind.value, storage: storage.value },
    })
    notice.value = t('backups.success', { name: out.name })
    await load()
  } catch (e) {
    error.value = e.message
  } finally {
    busy.value = false
  }
}

// ── Site Bazlı Zamanlama ─────────────────────────────────────────────────────
const siteSchedule = ref({ enabled: false, time: '02:00', frequency: 'daily', kind: 'full' })

async function loadSiteSchedule() {
  if (!siteId.value) return
  try {
    const s = await api(`/sites/${siteId.value}/backup-schedule`)
    siteSchedule.value = {
      enabled: s.enabled === '1',
      time: s.time || '02:00',
      frequency: s.frequency || 'daily',
      kind: s.kind || 'full',
    }
  } catch (e) { console.error('Site schedule read error', e) }
}

async function saveSiteSchedule() {
  error.value = ''
  notice.value = ''
  try {
    await api(`/sites/${siteId.value}/backup-schedule`, {
      method: 'POST',
      body: {
        enabled: siteSchedule.value.enabled ? '1' : '0',
        time: siteSchedule.value.time,
        frequency: siteSchedule.value.frequency,
        kind: siteSchedule.value.kind,
      },
    })
    notice.value = 'Site yedekleme zamanlaması kaydedildi.'
  } catch (e) { error.value = e.message }
}

onMounted(async () => {
  sites.value = await api('/sites').catch(() => [])
  if (sites.value.length) {
    siteId.value = sites.value[0].id
    await load()
    await loadSiteSchedule()
  }
})
</script>
