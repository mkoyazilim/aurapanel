<template>
  <div class="modal-backdrop" @click.self="emit('close')">
    <div class="modal-card backup-modal">
      <div class="bm-header">
        <h2 style="margin: 0">🗄️ {{ $t('backups.modal_title') }} <span class="mono muted">{{ site.name }}</span></h2>
        <button class="btn btn-sm" @click="emit('close')">✕</button>
      </div>

      <div class="bm-tabs">
        <button :class="['bm-tab', tab === 'instant' && 'active']" @click="tab = 'instant'">
          ⚡ {{ $t('backups.tab_instant') }}
        </button>
        <button :class="['bm-tab', tab === 'schedule' && 'active']" @click="tab = 'schedule'">
          ⏱️ {{ $t('backups.tab_schedule') }}
        </button>
        <button :class="['bm-tab', tab === 'history' && 'active']" @click="tab = 'history'">
          🗂️ {{ $t('backups.tab_history') }}
        </button>
      </div>

      <!-- ── Anlık Yedekleme ─────────────────────────────────────────────── -->
      <div v-if="tab === 'instant'" class="bm-body">
        <template v-if="run.state === 'running'">
          <div class="bm-progress">
            <div class="spinner"></div>
            <h3>{{ $t('backups.backup_running') }}</h3>
            <p class="muted">
              {{ kindLabel(kind) }} · {{ $t('backups.elapsed') }}: <strong>{{ fmtDuration(elapsed) }}</strong>
            </p>
            <p class="muted text-sm">
              {{ $t('backups.running_hint') }}
            </p>
          </div>
        </template>

        <template v-else>
          <div v-if="run.state === 'done'" class="alert ok">
            <strong>✅ {{ $t('backups.run_success') }}</strong>
            <div class="mono text-sm" style="margin-top: 6px">{{ run.rec?.location }}</div>
            <div class="muted text-sm" style="margin-top: 4px">
              {{ $t('backups.size') }}: {{ fmtBytes(run.rec?.size_bytes) }}
            </div>
          </div>
          <div v-else-if="run.state === 'failed'" class="alert error">
            <strong>❌ {{ $t('backups.run_failed') }}</strong>
            <div class="text-sm" style="margin-top: 6px">{{ run.message }}</div>
          </div>
          <div v-else-if="error" class="alert error">{{ error }}</div>

          <p class="muted text-sm" style="margin: 0 0 10px 0">{{ $t('backups.instant_desc') }}</p>

          <div class="bm-kinds">
            <label
              v-for="k in kinds"
              :key="k.value"
              :class="['bm-kind', kind === k.value && 'selected']"
            >
              <input type="radio" v-model="kind" :value="k.value" hidden />
              <span class="bm-kind-icon">{{ k.icon }}</span>
              <strong>{{ k.label }}</strong>
              <small class="muted">{{ k.desc }}</small>
            </label>
          </div>

          <div style="margin: 14px 0">
            <label>{{ $t('backups.destination') }}</label>
            <select v-model="storage">
              <option value="local">📁 {{ $t('backups.local_storage') }}</option>
              <option value="s3">☁️ {{ $t('backups.s3_cloudflare_r2') }}</option>
            </select>
          </div>

          <div class="bm-actions">
            <button class="btn" @click="emit('close')">{{ $t('common.cancel') }}</button>
            <button class="btn primary" @click="start" :disabled="busy">
              💾 {{ $t('backups.start_backup') }}
            </button>
          </div>
        </template>
      </div>

      <!-- ── Zamanlanmış Yedekleme ───────────────────────────────────────── -->
      <div v-else-if="tab === 'schedule'" class="bm-body">
        <div v-if="scheduleSaved" class="alert ok">{{ $t('backups.schedule_saved') }}</div>
        <div v-if="scheduleError" class="alert error">{{ scheduleError }}</div>

        <p class="muted text-sm">{{ $t('backups.schedule_desc') }}</p>

        <label class="bm-check" style="margin-bottom: 12px">
          <input type="checkbox" v-model="schedule.enabled" style="width: auto" />
          <strong>{{ $t('backups.schedule_enable') }}</strong>
        </label>

        <template v-if="schedule.enabled">
          <div class="bm-grid3">
            <div>
              <label>{{ $t('backups.time_label') }}</label>
              <input type="time" v-model="schedule.time" />
            </div>
            <div>
              <label>{{ $t('backups.freq_label') }}</label>
              <select v-model="schedule.frequency">
                <option v-for="f in freqs" :key="f.value" :value="f.value">{{ f.label }}</option>
              </select>
            </div>
            <div>
              <label>{{ $t('backups.kind') }}</label>
              <select v-model="schedule.kind">
                <option v-for="k in kinds" :key="k.value" :value="k.value">{{ k.label }}</option>
              </select>
            </div>
          </div>

          <div v-if="nextRun" class="bm-next-run">
            ⏭️ {{ $t('backups.next_run', { time: nextRun.toLocaleString() }) }}
          </div>
        </template>

        <div class="bm-actions" style="margin-top: 14px">
          <button class="btn" @click="emit('close')">{{ $t('common.cancel') }}</button>
          <button class="btn primary" @click="saveSchedule">⏱️ {{ $t('backups.save_schedule') }}</button>
        </div>
      </div>

      <!-- ── Geçmiş ──────────────────────────────────────────────────────── -->
      <div v-else class="bm-body">
        <div class="bm-actions" style="margin-bottom: 10px">
          <span class="muted text-sm">{{ backups.length }} {{ $t('backups.history') }}</span>
          <button class="btn btn-sm" @click="loadHistory">🔄 {{ $t('common.refresh') }}</button>
        </div>
        <div class="bm-history">
          <table>
            <thead>
              <tr>
                <th>{{ $t('backups.name') }}</th>
                <th>{{ $t('backups.kind') }}</th>
                <th>{{ $t('backups.status') }}</th>
                <th>{{ $t('backups.size') }}</th>
                <th>{{ $t('backups.date') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="b in backups" :key="b.id">
                <td class="mono text-sm">{{ b.location }}</td>
                <td>{{ kindLabel(b.kind) }}</td>
                <td>
                  <span class="badge" :class="b.status === 'success' ? 'ok' : b.status === 'running' ? 'warn' : 'err'">
                    {{ b.status === 'running' ? $t('backups.status_running') : b.status === 'success' ? $t('backups.status_success') : $t('backups.status_failed') }}
                  </span>
                </td>
                <td class="mono text-sm">{{ fmtBytes(b.size_bytes) }}</td>
                <td class="muted text-sm">{{ b.created_at }}</td>
              </tr>
              <tr v-if="!backups.length"><td colspan="5" class="muted">{{ $t('backups.no_backups') }}</td></tr>
            </tbody>
          </table>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed, onMounted, onUnmounted, reactive, ref } from 'vue'
import { api } from '../api'
import { useI18n } from 'vue-i18n'

const props = defineProps({
  site: { type: Object, required: true },
})
const emit = defineEmits(['close'])
const { t } = useI18n()

const tab = ref('instant')
const kind = ref('full')
const storage = ref('local')
const busy = ref(false)
const error = ref('')

const kinds = computed(() => [
  { value: 'full', icon: '💾', label: t('backups.bk_full'), desc: t('backups.bk_full_desc') },
  { value: 'files', icon: '📁', label: t('backups.bk_files'), desc: t('backups.bk_files_desc') },
  { value: 'db', icon: '🗃️', label: t('backups.bk_db'), desc: t('backups.bk_db_desc') },
])

function kindLabel(v) {
  return kinds.value.find((k) => k.value === v)?.label || v
}

// ── Anlık yedek: async başlat → DB kaydı üzerinden izle ──────────────────
const run = reactive({ state: 'idle', id: 0, name: '', message: '', startedAt: 0, rec: null })
const elapsed = ref(0)
let pollTimer = null
let elapsedTimer = null
let polling = false

function stopTimers() {
  if (pollTimer) { clearInterval(pollTimer); pollTimer = null }
  if (elapsedTimer) { clearInterval(elapsedTimer); elapsedTimer = null }
}

async function start() {
  busy.value = true
  error.value = ''
  run.message = ''
  run.startedAt = Date.now()
  elapsed.value = 0
  run.state = 'running'
  try {
    const out = await api(`/sites/${props.site.id}/backups/run`, {
      method: 'POST',
      body: { kind: kind.value, storage: storage.value, async: true },
    })
    run.id = out.id
    run.name = out.name
    elapsedTimer = setInterval(() => {
      elapsed.value = Math.floor((Date.now() - run.startedAt) / 1000)
    }, 1000)
    pollTimer = setInterval(poll, 2000)
    poll()
  } catch (e) {
    stopTimers()
    run.state = 'failed'
    run.message = e.message
  } finally {
    busy.value = false
  }
}

async function poll() {
  if (polling) return
  polling = true
  try {
    const list = await api(`/sites/${props.site.id}/backups`)
    const rec = list.find((b) => b.id === run.id)
    if (!rec || rec.status === 'running') return
    stopTimers()
    run.rec = rec
    run.state = rec.status === 'success' ? 'done' : 'failed'
    if (run.state === 'failed') run.message = t('backups.run_failed')
    loadHistory()
  } catch {
    // Geçici ağ hatası — sonraki poll dener.
  } finally {
    polling = false
  }
}

function fmtBytes(n) {
  if (n === null || n === undefined || n <= 0) return '—'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let v = n
  let i = 0
  while (v >= 1024 && i < units.length - 1) { v /= 1024; i++ }
  return v.toFixed(1) + ' ' + units[i]
}

function fmtDuration(s) {
  s = Math.max(0, Math.floor(s || 0))
  const h = Math.floor(s / 3600)
  const m = Math.floor((s % 3600) / 60)
  const sec = s % 60
  const mm = String(m).padStart(2, '0')
  const ss = String(sec).padStart(2, '0')
  return h > 0 ? `${h}:${mm}:${ss}` : `${mm}:${ss}`
}

// ── Zamanlanmış yedek ─────────────────────────────────────────────────────
const freqs = computed(() => [
  { value: 'daily', label: t('backups.freq_daily') },
  { value: 'monday', label: t('backups.freq_monday') },
  { value: 'tuesday', label: t('backups.freq_tuesday') },
  { value: 'wednesday', label: t('backups.freq_wednesday') },
  { value: 'thursday', label: t('backups.freq_thursday') },
  { value: 'friday', label: t('backups.freq_friday') },
  { value: 'saturday', label: t('backups.freq_saturday') },
  { value: 'sunday', label: t('backups.freq_sunday') },
])

const DAY_INDEX = { monday: 0, tuesday: 1, wednesday: 2, thursday: 3, friday: 4, saturday: 5, sunday: 6 }

const schedule = reactive({ enabled: false, time: '02:00', frequency: 'daily', kind: 'full' })
const scheduleSaved = ref(false)
const scheduleError = ref('')

const nextRun = computed(() => {
  if (!schedule.enabled || !schedule.time) return null
  const [h, m] = schedule.time.split(':').map(Number)
  if (Number.isNaN(h) || Number.isNaN(m)) return null
  const allowed = schedule.frequency === 'daily' ? [0, 1, 2, 3, 4, 5, 6] : [DAY_INDEX[schedule.frequency] ?? 0]
  const now = new Date()
  for (let i = 0; i < 8; i++) {
    const d = new Date(now.getFullYear(), now.getMonth(), now.getDate() + i, h, m, 0, 0)
    const dow = (d.getDay() + 6) % 7 // Pazartesi = 0
    if (allowed.includes(dow) && d > now) return d
  }
  return null
})

async function loadSchedule() {
  try {
    const s = await api(`/sites/${props.site.id}/backup-schedule`)
    schedule.enabled = s.enabled === '1'
    schedule.time = s.time || '02:00'
    schedule.frequency = s.frequency || 'daily'
    schedule.kind = s.kind || 'full'
  } catch {
    // zamanlama yoksa varsayılanlar geçerli
  }
}

async function saveSchedule() {
  scheduleSaved.value = false
  scheduleError.value = ''
  try {
    await api(`/sites/${props.site.id}/backup-schedule`, {
      method: 'POST',
      body: {
        enabled: schedule.enabled ? '1' : '0',
        time: schedule.time,
        frequency: schedule.frequency,
        kind: schedule.kind,
      },
    })
    scheduleSaved.value = true
  } catch (e) {
    scheduleError.value = e.message
  }
}

// ── Geçmiş ────────────────────────────────────────────────────────────────
const backups = ref([])

async function loadHistory() {
  try {
    backups.value = await api(`/sites/${props.site.id}/backups`)
  } catch {
    // boş liste
  }
}

onMounted(() => {
  loadSchedule()
  loadHistory()
})

onUnmounted(stopTimers)
</script>

<style scoped>
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
  background: var(--bg-card, #ffffff);
  border: 1px solid var(--border-color, #e2e8f0);
  border-radius: 12px;
  padding: 24px;
  box-shadow: 0 20px 25px -5px rgba(0, 0, 0, 0.2);
}

.backup-modal {
  width: 100%;
  max-width: 620px;
}

.bm-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 14px;
}

.bm-tabs {
  display: flex;
  gap: 6px;
  border-bottom: 1px solid var(--border-color, #e2e8f0);
  margin-bottom: 16px;
}

.bm-tab {
  background: transparent;
  border: none;
  border-bottom: 2px solid transparent;
  padding: 8px 14px;
  font-size: 13px;
  font-weight: 600;
  color: var(--muted-color, #64748b);
  cursor: pointer;
}

.bm-tab.active {
  color: var(--accent, #2563eb);
  border-bottom-color: var(--accent, #2563eb);
}

.bm-body {
  min-height: 260px;
}

.bm-kinds {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 10px;
}

.bm-kind {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 4px;
  text-align: center;
  padding: 16px 10px;
  border: 2px solid var(--border-color, #e2e8f0);
  border-radius: 10px;
  cursor: pointer;
  transition: border-color 0.15s ease, background 0.15s ease;
}

.bm-kind.selected {
  border-color: var(--accent, #2563eb);
  background: rgba(37, 99, 235, 0.05);
}

.bm-kind-icon {
  font-size: 26px;
}

.bm-kind small {
  font-size: 11px;
  line-height: 1.35;
}

.bm-grid3 {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 10px;
}

.bm-check {
  display: flex;
  align-items: center;
  gap: 8px;
  cursor: pointer;
}

.bm-next-run {
  margin-top: 12px;
  padding: 8px 12px;
  background: var(--bg-body, #f8fafc);
  border: 1px solid var(--border-color, #e2e8f0);
  border-radius: 8px;
  font-size: 13px;
}

.bm-actions {
  display: flex;
  gap: 8px;
  justify-content: flex-end;
  align-items: center;
}

.bm-progress {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 10px;
  padding: 40px 0;
  text-align: center;
}

.bm-progress h3 {
  margin: 6px 0 0 0;
}

.spinner {
  width: 44px;
  height: 44px;
  border: 4px solid var(--border-color, #e2e8f0);
  border-top-color: var(--accent, #2563eb);
  border-radius: 50%;
  animation: bm-spin 1s linear infinite;
}

@keyframes bm-spin {
  to { transform: rotate(360deg); }
}

.bm-history {
  max-height: 280px;
  overflow-y: auto;
  border: 1px solid var(--border-color, #e2e8f0);
  border-radius: 8px;
}

.bm-history table {
  margin: 0;
  border: none;
}

.bm-history th {
  position: sticky;
  top: 0;
  background: var(--bg-card, #ffffff);
}
</style>
