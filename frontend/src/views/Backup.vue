<template>
  <div class="space-y-6">
    <div class="flex items-center justify-between gap-3">
      <div>
        <h1 class="text-2xl font-bold text-white">Backup Center</h1>
        <p class="text-gray-400 mt-1">Destinations, snapshots, restore ve schedule yonetimi.</p>
      </div>
      <button class="btn-secondary" @click="loadAll">Yenile</button>
    </div>

    <div v-if="error" class="aura-card border-red-500/30 bg-red-500/5 text-red-400">{{ error }}</div>
    <div v-if="success" class="aura-card border-green-500/30 bg-green-500/5 text-green-300">{{ success }}</div>

    <div class="grid grid-cols-1 lg:grid-cols-2 gap-4">
      <div class="aura-card space-y-3">
        <h2 class="text-white font-semibold">Backup Calistir</h2>
        <div class="grid grid-cols-1 md:grid-cols-2 gap-3">
          <select v-model="runForm.domain" class="aura-input" @change="onDomainChange(runForm)">
            <option disabled value="">Domain secin</option>
            <option v-for="d in domains" :key="d" :value="d">{{ d }}</option>
          </select>
          <select v-model="runForm.destination_id" class="aura-input">
            <option disabled value="">Destination secin</option>
            <option v-for="d in destinations" :key="d.id" :value="d.id">{{ d.name }}</option>
          </select>
          <input v-model="runForm.backup_path" class="aura-input md:col-span-2" placeholder="/home/example.com/public_html" />
          <label class="inline-flex items-center gap-2 text-sm text-gray-300 md:col-span-2">
            <input v-model="runForm.incremental" type="checkbox" class="w-4 h-4" />
            Incremental backup
          </label>
        </div>
        <div class="flex gap-2">
          <button class="btn-primary" @click="runBackup">Backup Baslat</button>
          <button class="btn-secondary" @click="loadSnapshots">Snapshotlari Getir</button>
        </div>
      </div>

      <div class="aura-card space-y-3">
        <h2 class="text-white font-semibold">Restore</h2>
        <div class="grid grid-cols-1 md:grid-cols-2 gap-3">
          <select v-model="restoreForm.domain" class="aura-input" @change="onDomainChange(restoreForm)">
            <option disabled value="">Domain secin</option>
            <option v-for="d in domains" :key="d" :value="d">{{ d }}</option>
          </select>
          <select v-model="restoreForm.destination_id" class="aura-input">
            <option disabled value="">Destination secin</option>
            <option v-for="d in destinations" :key="d.id" :value="d.id">{{ d.name }}</option>
          </select>
          <input v-model="restoreForm.backup_path" class="aura-input md:col-span-2" placeholder="/home/example.com/public_html" />
          <input v-model="restoreForm.snapshot_id" class="aura-input md:col-span-2" placeholder="snapshot id" />
        </div>
        <button class="btn-primary" @click="restoreBackup">Restore Baslat</button>
      </div>
    </div>

    <div class="aura-card space-y-4">
      <div class="flex items-center justify-between">
        <h2 class="text-white font-semibold">Destinations</h2>
      </div>
      <div class="grid grid-cols-1 md:grid-cols-5 gap-3">
        <input v-model="destinationForm.name" class="aura-input" placeholder="S3 Main" />
        <input v-model="destinationForm.remote_repo" class="aura-input md:col-span-2" placeholder="s3:https://minio:9000/aura-backups/site" />
        <input v-model="destinationForm.password" type="password" class="aura-input" placeholder="Restic password" />
        <label class="inline-flex items-center gap-2 text-sm text-gray-300">
          <input v-model="destinationForm.enabled" type="checkbox" class="w-4 h-4" />
          Enabled
        </label>
      </div>
      <div class="flex gap-2">
        <button class="btn-primary" @click="saveDestination">Kaydet</button>
        <button class="btn-secondary" @click="resetDestinationForm">Temizle</button>
      </div>
      <div class="overflow-x-auto">
        <table class="w-full text-sm">
          <thead>
            <tr class="border-b border-panel-border text-gray-400">
              <th class="text-left py-2 px-2">Name</th>
              <th class="text-left py-2 px-2">Repository</th>
              <th class="text-left py-2 px-2">Durum</th>
              <th class="text-right py-2 px-2">Islem</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="d in destinations" :key="d.id" class="border-b border-panel-border/40">
              <td class="py-2 px-2 text-white">{{ d.name }}</td>
              <td class="py-2 px-2 text-gray-300 font-mono text-xs break-all">{{ d.remote_repo }}</td>
              <td class="py-2 px-2" :class="d.enabled ? 'text-green-400' : 'text-yellow-400'">{{ d.enabled ? 'enabled' : 'disabled' }}</td>
              <td class="py-2 px-2 text-right">
                <div class="flex justify-end gap-2">
                  <button class="btn-secondary px-2 py-1 text-xs" @click="editDestination(d)">Duzenle</button>
                  <button class="btn-danger px-2 py-1 text-xs" @click="deleteDestination(d.id)">Sil</button>
                </div>
              </td>
            </tr>
            <tr v-if="destinations.length === 0">
              <td colspan="4" class="text-center py-6 text-gray-500">Destination yok.</td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <div class="aura-card space-y-4">
      <h2 class="text-white font-semibold">Schedules</h2>
      <div class="grid grid-cols-1 md:grid-cols-6 gap-3">
        <select v-model="scheduleForm.domain" class="aura-input" @change="onDomainChange(scheduleForm)">
          <option disabled value="">Domain</option>
          <option v-for="d in domains" :key="d" :value="d">{{ d }}</option>
        </select>
        <select v-model="scheduleForm.destination_id" class="aura-input">
          <option disabled value="">Destination</option>
          <option v-for="d in destinations" :key="d.id" :value="d.id">{{ d.name }}</option>
        </select>
        <input v-model="scheduleForm.backup_path" class="aura-input md:col-span-2" placeholder="/home/example.com/public_html" />
        <input v-model="scheduleForm.cron" class="aura-input" placeholder="0 3 * * *" />
        <label class="inline-flex items-center gap-2 text-sm text-gray-300">
          <input v-model="scheduleForm.enabled" type="checkbox" class="w-4 h-4" />
          Enabled
        </label>
        <label class="inline-flex items-center gap-2 text-sm text-gray-300 md:col-span-2">
          <input v-model="scheduleForm.incremental" type="checkbox" class="w-4 h-4" />
          Incremental
        </label>
      </div>
      <div class="flex gap-2">
        <button class="btn-primary" @click="saveSchedule">Schedule Kaydet</button>
        <button class="btn-secondary" @click="resetScheduleForm">Temizle</button>
      </div>
      <div class="overflow-x-auto">
        <table class="w-full text-sm">
          <thead>
            <tr class="border-b border-panel-border text-gray-400">
              <th class="text-left py-2 px-2">Domain</th>
              <th class="text-left py-2 px-2">Cron</th>
              <th class="text-left py-2 px-2">Path</th>
              <th class="text-left py-2 px-2">Destination</th>
              <th class="text-right py-2 px-2">Islem</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="s in schedules" :key="s.id" class="border-b border-panel-border/40">
              <td class="py-2 px-2 text-white">{{ s.domain }}</td>
              <td class="py-2 px-2 text-gray-300 font-mono">{{ s.cron }}</td>
              <td class="py-2 px-2 text-gray-400 font-mono text-xs break-all">{{ s.backup_path }}</td>
              <td class="py-2 px-2 text-gray-300">{{ destinationName(s.destination_id) }}</td>
              <td class="py-2 px-2 text-right">
                <div class="flex justify-end gap-2">
                  <button class="btn-secondary px-2 py-1 text-xs" @click="editSchedule(s)">Duzenle</button>
                  <button class="btn-danger px-2 py-1 text-xs" @click="deleteSchedule(s.id)">Sil</button>
                </div>
              </td>
            </tr>
            <tr v-if="schedules.length === 0">
              <td colspan="5" class="text-center py-6 text-gray-500">Schedule yok.</td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <div class="aura-card space-y-3">
      <h2 class="text-white font-semibold">Snapshots</h2>
      <div class="overflow-x-auto">
        <table class="w-full text-sm">
          <thead>
            <tr class="border-b border-panel-border text-gray-400">
              <th class="text-left py-2 px-2">ID</th>
              <th class="text-left py-2 px-2">Time</th>
              <th class="text-left py-2 px-2">Hostname</th>
              <th class="text-left py-2 px-2">Tags</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="snap in snapshots" :key="snap.id" class="border-b border-panel-border/40">
              <td class="py-2 px-2 font-mono text-white">{{ snap.short_id || snap.id }}</td>
              <td class="py-2 px-2 text-gray-300">{{ snap.time }}</td>
              <td class="py-2 px-2 text-gray-300">{{ snap.hostname || '-' }}</td>
              <td class="py-2 px-2 text-gray-400">{{ (snap.tags || []).join(', ') }}</td>
            </tr>
            <tr v-if="snapshots.length === 0">
              <td colspan="4" class="text-center py-6 text-gray-500">Snapshot bulunamadi.</td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import api from '../services/api'

const error = ref('')
const success = ref('')
const sites = ref([])
const destinations = ref([])
const schedules = ref([])
const snapshots = ref([])

const destinationForm = ref({
  id: '',
  name: '',
  remote_repo: '',
  password: '',
  enabled: true,
})

const scheduleForm = ref({
  id: '',
  domain: '',
  destination_id: '',
  backup_path: '',
  cron: '0 3 * * *',
  incremental: false,
  enabled: true,
})

const runForm = ref({
  domain: '',
  destination_id: '',
  backup_path: '',
  incremental: false,
})

const restoreForm = ref({
  domain: '',
  destination_id: '',
  backup_path: '',
  snapshot_id: '',
})

const domains = computed(() => (sites.value || []).map(s => s.domain).filter(Boolean))

function apiErrorMessage(e, fallback) {
  return e?.response?.data?.message || e?.message || fallback
}

function onDomainChange(target) {
  if (!target.domain) return
  if (!target.backup_path) {
    target.backup_path = `/home/${target.domain}/public_html`
  }
}

function destinationName(id) {
  return destinations.value.find(x => x.id === id)?.name || id
}

function destinationById(id) {
  return destinations.value.find(x => x.id === id)
}

function backupPayloadFrom(form) {
  const destination = destinationById(form.destination_id)
  if (!destination) throw new Error('Destination secilmedi')
  return {
    domain: form.domain,
    backup_path: form.backup_path,
    remote_repo: destination.remote_repo,
    password: destination.password,
    incremental: !!form.incremental,
  }
}

async function loadSites() {
  const res = await api.get('/vhost/list')
  sites.value = res.data?.data || []
}

async function loadDestinations() {
  const res = await api.get('/backup/destinations')
  destinations.value = res.data?.data || []
}

async function loadSchedules() {
  const res = await api.get('/backup/schedules')
  schedules.value = res.data?.data || []
}

async function loadAll() {
  error.value = ''
  success.value = ''
  try {
    await Promise.all([loadSites(), loadDestinations(), loadSchedules()])
    if (!runForm.value.domain && domains.value.length > 0) {
      runForm.value.domain = domains.value[0]
      runForm.value.backup_path = `/home/${runForm.value.domain}/public_html`
      restoreForm.value.domain = domains.value[0]
      restoreForm.value.backup_path = `/home/${restoreForm.value.domain}/public_html`
      scheduleForm.value.domain = domains.value[0]
      scheduleForm.value.backup_path = `/home/${scheduleForm.value.domain}/public_html`
    }
    if (!runForm.value.destination_id && destinations.value.length > 0) {
      runForm.value.destination_id = destinations.value[0].id
      restoreForm.value.destination_id = destinations.value[0].id
      scheduleForm.value.destination_id = destinations.value[0].id
    }
  } catch (e) {
    error.value = apiErrorMessage(e, 'Backup verileri alinamadi')
  }
}

async function saveDestination() {
  error.value = ''
  success.value = ''
  try {
    const res = await api.post('/backup/destinations', destinationForm.value)
    success.value = `Destination kaydedildi: ${res.data?.data?.name || destinationForm.value.name}`
    resetDestinationForm()
    await loadDestinations()
  } catch (e) {
    error.value = apiErrorMessage(e, 'Destination kaydedilemedi')
  }
}

function editDestination(item) {
  destinationForm.value = { ...item }
}

function resetDestinationForm() {
  destinationForm.value = {
    id: '',
    name: '',
    remote_repo: '',
    password: '',
    enabled: true,
  }
}

async function deleteDestination(id) {
  if (!confirm('Destination silinsin mi?')) return
  error.value = ''
  success.value = ''
  try {
    await api.delete('/backup/destinations', { params: { id } })
    success.value = 'Destination silindi.'
    await loadAll()
  } catch (e) {
    error.value = apiErrorMessage(e, 'Destination silinemedi')
  }
}

async function saveSchedule() {
  error.value = ''
  success.value = ''
  try {
    const res = await api.post('/backup/schedules', scheduleForm.value)
    success.value = `Schedule kaydedildi: ${res.data?.data?.cron || scheduleForm.value.cron}`
    resetScheduleForm()
    await loadSchedules()
  } catch (e) {
    error.value = apiErrorMessage(e, 'Schedule kaydedilemedi')
  }
}

function editSchedule(item) {
  scheduleForm.value = { ...item }
}

function resetScheduleForm() {
  scheduleForm.value = {
    id: '',
    domain: domains.value[0] || '',
    destination_id: destinations.value[0]?.id || '',
    backup_path: domains.value[0] ? `/home/${domains.value[0]}/public_html` : '',
    cron: '0 3 * * *',
    incremental: false,
    enabled: true,
  }
}

async function deleteSchedule(id) {
  if (!confirm('Schedule silinsin mi?')) return
  error.value = ''
  success.value = ''
  try {
    await api.delete('/backup/schedules', { params: { id } })
    success.value = 'Schedule silindi.'
    await loadSchedules()
  } catch (e) {
    error.value = apiErrorMessage(e, 'Schedule silinemedi')
  }
}

async function runBackup() {
  error.value = ''
  success.value = ''
  try {
    const payload = backupPayloadFrom(runForm.value)
    const res = await api.post('/backup/create', payload)
    success.value = res.data?.message || 'Backup baslatildi.'
    if (res.data?.snapshot_id) {
      restoreForm.value.snapshot_id = res.data.snapshot_id
    }
    await loadSnapshots()
  } catch (e) {
    error.value = apiErrorMessage(e, 'Backup basarisiz')
  }
}

async function loadSnapshots() {
  error.value = ''
  try {
    const payload = backupPayloadFrom(runForm.value)
    const res = await api.post('/backup/snapshots', payload)
    snapshots.value = Array.isArray(res.data?.data) ? res.data.data : []
  } catch (e) {
    error.value = apiErrorMessage(e, 'Snapshot listesi alinamadi')
    snapshots.value = []
  }
}

async function restoreBackup() {
  error.value = ''
  success.value = ''
  try {
    const destination = destinationById(restoreForm.value.destination_id)
    if (!destination) throw new Error('Destination secin')
    const payload = {
      domain: restoreForm.value.domain,
      backup_path: restoreForm.value.backup_path,
      remote_repo: destination.remote_repo,
      password: destination.password,
      snapshot_id: restoreForm.value.snapshot_id,
    }
    const res = await api.post('/backup/restore', payload)
    success.value = res.data?.message || 'Restore tamamlandi.'
  } catch (e) {
    error.value = apiErrorMessage(e, 'Restore basarisiz')
  }
}

onMounted(loadAll)
</script>
