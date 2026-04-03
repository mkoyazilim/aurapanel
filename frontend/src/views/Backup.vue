<template>
  <div class="space-y-6">
    <div class="flex items-center justify-between gap-3">
      <div>
        <h1 class="text-2xl font-bold text-white">{{ t('backup_center.title') }}</h1>
        <p class="mt-1 text-gray-400">{{ t('backup_center.subtitle') }}</p>
      </div>
      <button class="btn-secondary" @click="loadAll">{{ t('backup_center.refresh') }}</button>
    </div>

    <div v-if="error" class="aura-card border-red-500/30 bg-red-500/5 text-red-400">{{ error }}</div>
    <div v-if="success" class="aura-card border-green-500/30 bg-green-500/5 text-green-300">{{ success }}</div>

    <div class="grid grid-cols-1 gap-4 lg:grid-cols-2">
      <div class="aura-card space-y-3">
        <h2 class="font-semibold text-white">{{ t('backup_center.run_title') }}</h2>
        <div class="grid grid-cols-1 gap-3 md:grid-cols-2">
          <select v-model="runForm.domain" class="aura-input" @change="onDomainChange(runForm)">
            <option disabled value="">{{ t('backup_center.select_domain') }}</option>
            <option v-for="domainName in domains" :key="domainName" :value="domainName">{{ domainName }}</option>
          </select>
          <select v-model="runForm.destination_id" class="aura-input">
            <option disabled value="">{{ t('backup_center.select_destination') }}</option>
            <option v-for="destination in destinations" :key="destination.id" :value="destination.id">{{ destination.name }}</option>
          </select>
          <input
            v-model="runForm.backup_path"
            class="aura-input md:col-span-2"
            :placeholder="t('backup_center.backup_path_placeholder')"
          />
          <div class="md:col-span-2 rounded-lg border border-panel-border bg-panel-dark/40 px-3 py-2 text-sm text-gray-300">
            {{ t('backup_center.incremental_backup') }}
          </div>
          <input
            v-model.number="runForm.retention_keep"
            type="number"
            min="1"
            max="365"
            class="aura-input md:col-span-2"
            :placeholder="t('backup_center.form.retention_keep')"
          />
        </div>
        <div class="flex gap-2">
          <button class="btn-primary" @click="runBackup">{{ t('backup_center.run_backup') }}</button>
          <button class="btn-secondary" @click="loadSnapshots">{{ t('backup_center.load_snapshots') }}</button>
        </div>
      </div>

      <div class="aura-card space-y-3">
        <h2 class="font-semibold text-white">{{ t('backup_center.restore_title') }}</h2>
        <div class="grid grid-cols-1 gap-3 md:grid-cols-2">
          <select v-model="restoreForm.domain" class="aura-input" @change="onDomainChange(restoreForm)">
            <option disabled value="">{{ t('backup_center.select_domain') }}</option>
            <option v-for="domainName in domains" :key="domainName" :value="domainName">{{ domainName }}</option>
          </select>
          <select v-model="restoreForm.destination_id" class="aura-input">
            <option disabled value="">{{ t('backup_center.select_destination') }}</option>
            <option v-for="destination in destinations" :key="destination.id" :value="destination.id">{{ destination.name }}</option>
          </select>
          <input
            v-model="restoreForm.backup_path"
            class="aura-input md:col-span-2"
            :placeholder="t('backup_center.backup_path_placeholder')"
          />
          <input
            v-model="restoreForm.snapshot_id"
            class="aura-input md:col-span-2"
            :placeholder="t('backup_center.snapshot_placeholder')"
          />
          <label class="inline-flex items-center gap-2 text-sm text-gray-300 md:col-span-2">
            <input v-model="restoreForm.dry_run" type="checkbox" class="h-4 w-4" />
            {{ t('backup_center.dry_run') }}
          </label>
        </div>
        <button class="btn-primary" @click="restoreBackup">{{ t('backup_center.restore_backup') }}</button>
      </div>
    </div>

    <div class="aura-card space-y-4">
      <h2 class="font-semibold text-white">{{ t('backup_center.destinations_title') }}</h2>
      <div class="grid grid-cols-1 gap-3 md:grid-cols-5">
        <input
          v-model="destinationForm.name"
          class="aura-input"
          :placeholder="t('backup_center.form.destination_name')"
        />
        <input
          v-model="destinationForm.remote_repo"
          class="aura-input md:col-span-2"
          :placeholder="t('backup_center.form.destination_repo')"
        />
        <input
          v-model="destinationForm.password"
          type="password"
          class="aura-input"
          :placeholder="t('backup_center.form.destination_password')"
        />
        <input
          v-model.number="destinationForm.retention_keep"
          type="number"
          min="1"
          max="365"
          class="aura-input"
          :placeholder="t('backup_center.form.retention_keep')"
        />
        <label class="inline-flex items-center gap-2 text-sm text-gray-300">
          <input v-model="destinationForm.enabled" type="checkbox" class="h-4 w-4" />
          {{ t('backup_center.enabled') }}
        </label>
      </div>
      <div class="flex gap-2">
        <button class="btn-primary" @click="saveDestination">{{ t('backup_center.save_destination') }}</button>
        <button class="btn-secondary" @click="resetDestinationForm">{{ t('backup_center.reset_destination') }}</button>
      </div>
      <div class="overflow-x-auto">
        <table class="w-full text-sm">
          <thead>
            <tr class="border-b border-panel-border text-gray-400">
              <th class="px-2 py-2 text-left">{{ t('backup_center.table.name') }}</th>
              <th class="px-2 py-2 text-left">{{ t('backup_center.table.repository') }}</th>
              <th class="px-2 py-2 text-left">{{ t('backup_center.table.state') }}</th>
              <th class="px-2 py-2 text-right">{{ t('backup_center.table.action') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="destination in destinations" :key="destination.id" class="border-b border-panel-border/40">
              <td class="px-2 py-2 text-white">{{ destination.name }}</td>
              <td class="break-all px-2 py-2 font-mono text-xs text-gray-300">{{ destination.remote_repo }}</td>
              <td class="px-2 py-2" :class="destination.enabled ? 'text-green-400' : 'text-yellow-400'">
                {{ destination.enabled ? t('backup_center.status.enabled') : t('backup_center.status.disabled') }}
              </td>
              <td class="px-2 py-2 text-right">
                <div class="flex justify-end gap-2">
                  <button class="btn-secondary px-2 py-1 text-xs" @click="editDestination(destination)">{{ t('common.edit') }}</button>
                  <button class="btn-danger px-2 py-1 text-xs" @click="deleteDestination(destination.id)">{{ t('common.delete') }}</button>
                </div>
              </td>
            </tr>
            <tr v-if="destinations.length === 0">
              <td colspan="4" class="py-6 text-center text-gray-500">{{ t('backup_center.empty.destinations') }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <div class="aura-card space-y-4">
      <h2 class="font-semibold text-white">{{ t('backup_center.schedules_title') }}</h2>
      <div class="grid grid-cols-1 gap-3 md:grid-cols-6">
        <select v-model="scheduleForm.domain" class="aura-input" @change="onDomainChange(scheduleForm)">
          <option disabled value="">{{ t('backup_center.select_domain') }}</option>
          <option v-for="domainName in domains" :key="domainName" :value="domainName">{{ domainName }}</option>
        </select>
        <select v-model="scheduleForm.destination_id" class="aura-input">
          <option disabled value="">{{ t('backup_center.destination_short') }}</option>
          <option v-for="destination in destinations" :key="destination.id" :value="destination.id">{{ destination.name }}</option>
        </select>
        <input
          v-model="scheduleForm.backup_path"
          class="aura-input md:col-span-2"
          :placeholder="t('backup_center.backup_path_placeholder')"
        />
        <input
          v-model="scheduleForm.cron"
          class="aura-input"
          :placeholder="t('backup_center.form.cron_placeholder')"
        />
        <label class="inline-flex items-center gap-2 text-sm text-gray-300">
          <input v-model="scheduleForm.enabled" type="checkbox" class="h-4 w-4" />
          {{ t('backup_center.enabled') }}
        </label>
        <div class="md:col-span-2 rounded-lg border border-panel-border bg-panel-dark/40 px-3 py-2 text-sm text-gray-300">
          {{ t('backup_center.incremental') }}
        </div>
        <input
          v-model.number="scheduleForm.retention_keep"
          type="number"
          min="1"
          max="365"
          class="aura-input"
          :placeholder="t('backup_center.form.retention_keep')"
        />
      </div>
      <div class="flex gap-2">
        <button class="btn-primary" @click="saveSchedule">{{ t('backup_center.save_schedule') }}</button>
        <button class="btn-secondary" @click="resetScheduleForm">{{ t('backup_center.reset_schedule') }}</button>
      </div>
      <div class="overflow-x-auto">
        <table class="w-full text-sm">
          <thead>
            <tr class="border-b border-panel-border text-gray-400">
              <th class="px-2 py-2 text-left">{{ t('backup_center.table.domain') }}</th>
              <th class="px-2 py-2 text-left">{{ t('backup_center.table.cron') }}</th>
              <th class="px-2 py-2 text-left">{{ t('backup_center.table.path') }}</th>
              <th class="px-2 py-2 text-left">{{ t('backup_center.table.destination') }}</th>
              <th class="px-2 py-2 text-right">{{ t('backup_center.table.action') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="schedule in schedules" :key="schedule.id" class="border-b border-panel-border/40">
              <td class="px-2 py-2 text-white">{{ schedule.domain }}</td>
              <td class="px-2 py-2 font-mono text-gray-300">{{ schedule.cron }}</td>
              <td class="break-all px-2 py-2 font-mono text-xs text-gray-400">{{ schedule.backup_path }}</td>
              <td class="px-2 py-2 text-gray-300">{{ destinationName(schedule.destination_id) }}</td>
              <td class="px-2 py-2 text-right">
                <div class="flex justify-end gap-2">
                  <button class="btn-secondary px-2 py-1 text-xs" @click="editSchedule(schedule)">{{ t('common.edit') }}</button>
                  <button class="btn-danger px-2 py-1 text-xs" @click="deleteSchedule(schedule.id)">{{ t('common.delete') }}</button>
                </div>
              </td>
            </tr>
            <tr v-if="schedules.length === 0">
              <td colspan="5" class="py-6 text-center text-gray-500">{{ t('backup_center.empty.schedules') }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <div class="aura-card space-y-3">
      <h2 class="font-semibold text-white">{{ t('backup_center.snapshots_title') }}</h2>
      <div class="overflow-x-auto">
        <table class="w-full text-sm">
          <thead>
            <tr class="border-b border-panel-border text-gray-400">
              <th class="px-2 py-2 text-left">{{ t('backup_center.table.id') }}</th>
              <th class="px-2 py-2 text-left">{{ t('backup_center.table.time') }}</th>
              <th class="px-2 py-2 text-left">{{ t('backup_center.table.hostname') }}</th>
              <th class="px-2 py-2 text-left">{{ t('backup_center.table.mode') }}</th>
              <th class="px-2 py-2 text-left">{{ t('backup_center.table.size') }}</th>
              <th class="px-2 py-2 text-left">{{ t('backup_center.table.tags') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="snapshot in snapshots" :key="snapshot.id" class="border-b border-panel-border/40">
              <td class="px-2 py-2 font-mono text-white">{{ snapshot.short_id || snapshot.id }}</td>
              <td class="px-2 py-2 text-gray-300">{{ snapshot.time }}</td>
              <td class="px-2 py-2 text-gray-300">{{ snapshot.hostname || '-' }}</td>
              <td class="px-2 py-2 text-gray-300">{{ t('backup_center.mode.full') }}</td>
              <td class="px-2 py-2 text-gray-300">{{ formatSize(snapshot.size_bytes) }}</td>
              <td class="px-2 py-2 text-gray-400">{{ (snapshot.tags || []).join(', ') }}</td>
            </tr>
            <tr v-if="snapshots.length === 0">
              <td colspan="6" class="py-6 text-center text-gray-500">{{ t('backup_center.empty.snapshots') }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <div class="grid grid-cols-1 gap-4 xl:grid-cols-2">
      <div class="aura-card space-y-4">
        <div>
          <h2 class="font-semibold text-white">{{ t('backup_center.sla.title') }}</h2>
          <p class="text-sm text-gray-400">{{ t('backup_center.sla.subtitle') }}</p>
        </div>
        <div class="grid grid-cols-2 gap-3 text-sm">
          <div class="rounded-lg border border-panel-border bg-panel-dark/40 p-3">
            <p class="text-gray-400">{{ t('backup_center.sla.total_domains') }}</p>
            <p class="mt-1 text-xl font-semibold text-white">{{ slaSummary.total_domains || 0 }}</p>
          </div>
          <div class="rounded-lg border border-panel-border bg-panel-dark/40 p-3">
            <p class="text-gray-400">{{ t('backup_center.sla.coverage') }}</p>
            <p class="mt-1 text-xl font-semibold text-white">{{ slaSummary.coverage_percent || 0 }}%</p>
          </div>
          <div class="rounded-lg border border-panel-border bg-panel-dark/40 p-3">
            <p class="text-gray-400">{{ t('backup_center.sla.backup_healthy') }}</p>
            <p class="mt-1 text-xl font-semibold text-white">{{ slaSummary.backup_healthy_domains || 0 }}</p>
          </div>
          <div class="rounded-lg border border-panel-border bg-panel-dark/40 p-3">
            <p class="text-gray-400">{{ t('backup_center.sla.drill_healthy') }}</p>
            <p class="mt-1 text-xl font-semibold text-white">{{ slaSummary.drill_healthy_domains || 0 }}</p>
          </div>
        </div>
        <div class="overflow-x-auto">
          <table class="w-full text-sm">
            <thead>
              <tr class="border-b border-panel-border text-gray-400">
                <th class="px-2 py-2 text-left">{{ t('backup_center.table.domain') }}</th>
                <th class="px-2 py-2 text-left">{{ t('backup_center.sla.latest_age') }}</th>
                <th class="px-2 py-2 text-left">{{ t('backup_center.sla.last_drill') }}</th>
                <th class="px-2 py-2 text-left">{{ t('backup_center.sla.score') }}</th>
                <th class="px-2 py-2 text-left">{{ t('backup_center.sla.status') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="item in slaDomains" :key="`sla-${item.domain}`" class="border-b border-panel-border/40">
                <td class="px-2 py-2 text-white">{{ item.domain }}</td>
                <td class="px-2 py-2 text-gray-300">{{ item.latest_snapshot_age_minutes >= 0 ? `${item.latest_snapshot_age_minutes}m` : '-' }}</td>
                <td class="px-2 py-2 text-gray-300">
                  <span v-if="item.last_drill_at">{{ formatDateTime(item.last_drill_at) }}</span>
                  <span v-else>-</span>
                </td>
                <td class="px-2 py-2 text-gray-300">{{ item.score }}</td>
                <td class="px-2 py-2">
                  <span :class="slaStatusClass(item.status)">{{ item.status }}</span>
                </td>
              </tr>
              <tr v-if="slaDomains.length === 0">
                <td colspan="5" class="py-6 text-center text-gray-500">{{ t('backup_center.sla.empty') }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>

      <div class="aura-card space-y-4">
        <div>
          <h2 class="font-semibold text-white">{{ t('backup_center.drill.title') }}</h2>
          <p class="text-sm text-gray-400">{{ t('backup_center.drill.subtitle') }}</p>
        </div>
        <div class="grid grid-cols-1 gap-3 md:grid-cols-2">
          <select v-model="drillForm.domain" class="aura-input">
            <option disabled value="">{{ t('backup_center.select_domain') }}</option>
            <option v-for="domainName in domains" :key="`drill-domain-${domainName}`" :value="domainName">{{ domainName }}</option>
          </select>
          <input v-model="drillForm.snapshot_id" class="aura-input" :placeholder="t('backup_center.snapshot_placeholder')" />
          <input v-model="drillForm.target_domain" class="aura-input md:col-span-2" :placeholder="t('backup_center.drill.target_domain')" />
        </div>
        <div class="flex gap-2">
          <button class="btn-primary" :disabled="drillLoading" @click="runRestoreDrill">{{ t('backup_center.drill.run') }}</button>
          <button class="btn-secondary" @click="loadDrillHistory">{{ t('backup_center.drill.load_history') }}</button>
        </div>
        <div class="overflow-x-auto">
          <table class="w-full text-sm">
            <thead>
              <tr class="border-b border-panel-border text-gray-400">
                <th class="px-2 py-2 text-left">{{ t('backup_center.table.time') }}</th>
                <th class="px-2 py-2 text-left">{{ t('backup_center.table.domain') }}</th>
                <th class="px-2 py-2 text-left">{{ t('backup_center.table.id') }}</th>
                <th class="px-2 py-2 text-left">{{ t('backup_center.sla.status') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="item in drillHistory" :key="item.id" class="border-b border-panel-border/40">
                <td class="px-2 py-2 text-gray-300">{{ formatDateTime(item.checked_at) }}</td>
                <td class="px-2 py-2 text-white">{{ item.domain }}</td>
                <td class="px-2 py-2 font-mono text-xs text-gray-300">{{ item.snapshot_short_id || item.snapshot_id }}</td>
                <td class="px-2 py-2">
                  <span :class="slaStatusClass(item.status)">{{ item.status }}</span>
                </td>
              </tr>
              <tr v-if="drillHistory.length === 0">
                <td colspan="4" class="py-6 text-center text-gray-500">{{ t('backup_center.drill.empty') }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import api from '../services/api'

const { t } = useI18n({ useScope: 'global' })

const error = ref('')
const success = ref('')
const sites = ref([])
const destinations = ref([])
const schedules = ref([])
const snapshots = ref([])
const slaSummary = ref({})
const slaDomains = ref([])
const drillHistory = ref([])
const drillLoading = ref(false)

const destinationForm = ref({
  id: '',
  name: '',
  remote_repo: '',
  password: '',
  retention_keep: 14,
  enabled: true,
})

const scheduleForm = ref({
  id: '',
  domain: '',
  destination_id: '',
  backup_path: '',
  cron: '0 3 * * *',
  retention_keep: 14,
  enabled: true,
})

const runForm = ref({
  domain: '',
  destination_id: '',
  backup_path: '',
  retention_keep: 14,
})

const restoreForm = ref({
  domain: '',
  destination_id: '',
  backup_path: '',
  snapshot_id: '',
  dry_run: true,
})

const drillForm = ref({
  domain: '',
  snapshot_id: '',
  target_domain: '',
})

const domains = computed(() => (sites.value || []).map(site => site.domain).filter(Boolean))

function apiErrorMessage(err, fallbackKey) {
  return err?.response?.data?.message || err?.message || t(fallbackKey)
}

function onDomainChange(target) {
  if (!target.domain || target.backup_path) return
  target.backup_path = '/var/backups/aurapanel/sites'
}

function destinationName(id) {
  return destinations.value.find(item => item.id === id)?.name || id
}

function destinationById(id) {
  return destinations.value.find(item => item.id === id)
}

function backupPayloadFrom(form) {
  const destination = destinationById(form.destination_id)
  if (!destination) {
    throw new Error(t('backup_center.messages.destination_not_selected'))
  }
  return {
    domain: form.domain,
    destination_id: form.destination_id,
    backup_path: form.backup_path,
    remote_repo: destination.remote_repo,
    password: destination.password,
    incremental: false,
    retention_keep: Number(form.retention_keep || destination.retention_keep || 14),
  }
}

function formatSize(value) {
  const size = Number(value || 0)
  if (!Number.isFinite(size) || size <= 0) return '-'
  if (size >= 1024 * 1024 * 1024) return `${(size / (1024 * 1024 * 1024)).toFixed(1)} GB`
  if (size >= 1024 * 1024) return `${(size / (1024 * 1024)).toFixed(1)} MB`
  if (size >= 1024) return `${(size / 1024).toFixed(1)} KB`
  return `${size} B`
}

function formatDateTime(value) {
  const ts = Number(value || 0)
  if (!ts) return '-'
  return new Date(ts).toLocaleString()
}

function slaStatusClass(status) {
  const key = String(status || '').toLowerCase()
  if (key === 'healthy' || key === 'success') return 'text-green-400'
  if (key === 'warning') return 'text-yellow-400'
  return 'text-red-400'
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
      runForm.value.backup_path = '/var/backups/aurapanel/sites'
      restoreForm.value.domain = domains.value[0]
      restoreForm.value.backup_path = '/var/backups/aurapanel/sites'
      scheduleForm.value.domain = domains.value[0]
      scheduleForm.value.backup_path = '/var/backups/aurapanel/sites'
      drillForm.value.domain = domains.value[0]
      drillForm.value.target_domain = domains.value[0]
    }
    if (!runForm.value.destination_id && destinations.value.length > 0) {
      runForm.value.destination_id = destinations.value[0].id
      restoreForm.value.destination_id = destinations.value[0].id
      scheduleForm.value.destination_id = destinations.value[0].id
    }
    await Promise.all([loadSLAReport(), loadDrillHistory()])
  } catch (err) {
    error.value = apiErrorMessage(err, 'backup_center.messages.load_failed')
  }
}

async function loadSLAReport() {
  const params = runForm.value.domain ? { domain: runForm.value.domain } : undefined
  const res = await api.get('/backup/sla/report', { params })
  const payload = res.data?.data || {}
  slaSummary.value = payload.summary || {}
  slaDomains.value = Array.isArray(payload.domains) ? payload.domains : []
}

async function loadDrillHistory() {
  const params = drillForm.value.domain ? { domain: drillForm.value.domain, limit: 30 } : { limit: 30 }
  const res = await api.get('/backup/restore-drill/history', { params })
  drillHistory.value = Array.isArray(res.data?.data) ? res.data.data : []
}

async function saveDestination() {
  error.value = ''
  success.value = ''
  try {
    const res = await api.post('/backup/destinations', destinationForm.value)
    success.value = t('backup_center.messages.destination_saved', {
      name: res.data?.data?.name || destinationForm.value.name,
    })
    resetDestinationForm()
    await loadDestinations()
  } catch (err) {
    error.value = apiErrorMessage(err, 'backup_center.messages.destination_save_failed')
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
    retention_keep: 14,
    enabled: true,
  }
}

async function deleteDestination(id) {
  if (!window.confirm(t('backup_center.messages.destination_delete_confirm'))) return
  error.value = ''
  success.value = ''
  try {
    await api.delete('/backup/destinations', { params: { id } })
    success.value = t('backup_center.messages.destination_deleted')
    await loadAll()
  } catch (err) {
    error.value = apiErrorMessage(err, 'backup_center.messages.destination_delete_failed')
  }
}

async function saveSchedule() {
  error.value = ''
  success.value = ''
  try {
    const res = await api.post('/backup/schedules', scheduleForm.value)
    success.value = t('backup_center.messages.schedule_saved', {
      cron: res.data?.data?.cron || scheduleForm.value.cron,
    })
    resetScheduleForm()
    await loadSchedules()
  } catch (err) {
    error.value = apiErrorMessage(err, 'backup_center.messages.schedule_save_failed')
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
    backup_path: '/var/backups/aurapanel/sites',
    cron: '0 3 * * *',
    retention_keep: 14,
    enabled: true,
  }
}

async function deleteSchedule(id) {
  if (!window.confirm(t('backup_center.messages.schedule_delete_confirm'))) return
  error.value = ''
  success.value = ''
  try {
    await api.delete('/backup/schedules', { params: { id } })
    success.value = t('backup_center.messages.schedule_deleted')
    await loadSchedules()
  } catch (err) {
    error.value = apiErrorMessage(err, 'backup_center.messages.schedule_delete_failed')
  }
}

async function runBackup() {
  error.value = ''
  success.value = ''
  try {
    const payload = backupPayloadFrom(runForm.value)
    const res = await api.post('/backup/create', payload)
    success.value = res.data?.message || t('backup_center.messages.backup_started')
    if (res.data?.snapshot_id) {
      restoreForm.value.snapshot_id = res.data.snapshot_id
      drillForm.value.snapshot_id = res.data.snapshot_id
    }
    await Promise.all([loadSnapshots(), loadSLAReport(), loadDrillHistory()])
  } catch (err) {
    error.value = apiErrorMessage(err, 'backup_center.messages.backup_failed')
  }
}

async function loadSnapshots() {
  error.value = ''
  try {
    const payload = backupPayloadFrom(runForm.value)
    const res = await api.post('/backup/snapshots', payload)
    snapshots.value = Array.isArray(res.data?.data) ? res.data.data : []
  } catch (err) {
    error.value = apiErrorMessage(err, 'backup_center.messages.snapshots_failed')
    snapshots.value = []
  }
}

async function restoreBackup() {
  error.value = ''
  success.value = ''
  try {
    const destination = destinationById(restoreForm.value.destination_id)
    if (!destination) {
      throw new Error(t('backup_center.messages.restore_destination_required'))
    }
    const payload = {
      domain: restoreForm.value.domain,
      backup_path: restoreForm.value.backup_path,
      remote_repo: destination.remote_repo,
      password: destination.password,
      snapshot_id: restoreForm.value.snapshot_id,
      dry_run: !!restoreForm.value.dry_run,
    }
    const res = await api.post('/backup/restore', payload)
    success.value = res.data?.message || t('backup_center.messages.restore_started')
  } catch (err) {
    error.value = apiErrorMessage(err, 'backup_center.messages.restore_failed')
  }
}

async function runRestoreDrill() {
  if (!drillForm.value.domain) {
    error.value = t('backup_center.messages.drill_domain_required')
    return
  }
  error.value = ''
  success.value = ''
  drillLoading.value = true
  try {
    const res = await api.post('/backup/restore-drill', {
      domain: drillForm.value.domain,
      snapshot_id: drillForm.value.snapshot_id || undefined,
      target_domain: drillForm.value.target_domain || undefined,
    })
    success.value = res.data?.message || t('backup_center.messages.drill_done')
    await Promise.all([loadSLAReport(), loadDrillHistory()])
  } catch (err) {
    error.value = apiErrorMessage(err, 'backup_center.messages.drill_failed')
  } finally {
    drillLoading.value = false
  }
}

onMounted(loadAll)
</script>
