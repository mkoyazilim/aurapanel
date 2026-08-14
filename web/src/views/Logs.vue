<template>
  <Layout>
    <div class="page-header">
      <div class="page-title-row">
        <div>
          <h2>{{ $t('logs.title') }}</h2>
          <p class="muted">{{ $t('logs.subtitle') }}</p>
        </div>
        <div class="header-actions">
          <select v-model="selectedSite" @change="restartTail" class="select-site">
            <option value="" disabled>{{ $t('logs.select_site') }}</option>
            <option v-for="s in sites" :key="s.id" :value="s.id">{{ s.name }} ({{ s.id }})</option>
          </select>
          <div class="btn-group">
            <button
              class="btn btn-sm"
              :class="logType === 'access' ? 'btn-primary' : ''"
              @click="switchType('access')"
            >
              Access Log
            </button>
            <button
              class="btn btn-sm"
              :class="logType === 'error' ? 'btn-primary' : ''"
              @click="switchType('error')"
            >
              Error Log
            </button>
          </div>
          <button class="btn btn-sm" :class="paused ? 'btn-success' : 'btn-secondary'" @click="togglePause">
            {{ paused ? $t('logs.resume') : $t('logs.pause') }}
          </button>
          <button class="btn btn-sm btn-danger" @click="clearLogs">
            {{ $t('logs.clear') }}
          </button>
        </div>
      </div>
    </div>

    <div v-if="!selectedSite" class="empty-state card">
      <p class="muted">{{ $t('logs.select_site_prompt') }}</p>
    </div>

    <div v-else class="card terminal-container">
      <div class="terminal-header">
        <span class="terminal-status" :class="connected && !paused ? 'live' : 'paused'"></span>
        <span class="mono text-sm">
          {{ selectedSite }} — {{ logType }}.log
          <span v-if="paused">({{ $t('logs.paused_badge') }})</span>
        </span>
      </div>
      <div ref="logBox" class="terminal-body">
        <div v-if="lines.length === 0" class="log-empty">
          <span class="muted mono text-sm">{{ $t('logs.waiting_logs') }}</span>
        </div>
        <div v-for="(l, idx) in lines" :key="idx" class="log-line">
          <span class="log-ts">{{ l.ts }}</span>
          <span class="log-text">{{ l.line }}</span>
        </div>
      </div>
    </div>
  </Layout>
</template>

<script setup>
import { ref, onMounted, onUnmounted, nextTick } from 'vue'
import Layout from '../components/Layout.vue'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()
const sites = ref([])
const selectedSite = ref('')
const logType = ref('access')
const lines = ref([])
const connected = ref(false)
const paused = ref(false)
const logBox = ref(null)
let eventSource = null

async function fetchSites() {
  try {
    const res = await fetch('/api/v1/sites')
    if (res.ok) {
      sites.value = await res.json()
      if (sites.value.length > 0) {
        selectedSite.value = sites.value[0].id
        startTail()
      }
    }
  } catch (err) {
    console.error('Siteler alınamadı:', err)
  }
}

function switchType(type) {
  if (logType.value === type) return
  logType.value = type
  lines.value = []
  restartTail()
}

function togglePause() {
  paused.value = !paused.value
}

function clearLogs() {
  lines.value = []
}

function startTail() {
  stopTail()
  if (!selectedSite.value) return

  const url = `/api/v1/sites/${selectedSite.value}/logs/tail?file=${logType.value}`
  eventSource = new EventSource(url)

  eventSource.onopen = () => {
    connected.value = true
  }

  eventSource.onmessage = (event) => {
    if (paused.value) return
    try {
      const parsed = JSON.parse(event.data)
      lines.value.push(parsed)
      if (lines.value.length > 1000) {
        lines.value.shift()
      }
      nextTick(() => {
        if (logBox.value) {
          logBox.value.scrollTop = logBox.value.scrollHeight
        }
      })
    } catch {
      // plain text fallback
      lines.value.push({ line: event.data, ts: '' })
    }
  }

  eventSource.onerror = () => {
    connected.value = false
  }
}

function stopTail() {
  if (eventSource) {
    eventSource.close()
    eventSource = null
    connected.value = false
  }
}

function restartTail() {
  stopTail()
  lines.value = []
  startTail()
}

onMounted(fetchSites)
onUnmounted(stopTail)
</script>

<style scoped>
.page-header { margin-bottom: 24px; }
.page-title-row { display: flex; justify-content: space-between; align-items: center; }
.header-actions { display: flex; gap: 10px; align-items: center; }
.select-site { padding: 6px 12px; border-radius: 6px; border: 1px solid var(--border-color); background: var(--bg-card); color: var(--text-color); font-size: 14px; }
.btn-group { display: flex; gap: 4px; background: rgba(0,0,0,0.05); padding: 2px; border-radius: 6px; }
.empty-state { padding: 48px; text-align: center; }
.terminal-container { padding: 0; overflow: hidden; background: #0f172a; border-color: #1e293b; color: #f8fafc; }
.terminal-header { padding: 10px 16px; background: #1e293b; display: flex; align-items: center; gap: 8px; border-bottom: 1px solid #334155; }
.terminal-status { width: 8px; height: 8px; border-radius: 50%; display: inline-block; }
.terminal-status.live { background: #22c55e; box-shadow: 0 0 8px #22c55e; }
.terminal-status.paused { background: #eab308; }
.terminal-body { padding: 16px; height: 500px; overflow-y: auto; font-family: ui-monospace, monospace; font-size: 13px; line-height: 1.6; }
.log-line { display: flex; gap: 12px; white-space: pre-wrap; word-break: break-all; margin-bottom: 4px; }
.log-ts { color: #64748b; font-size: 11px; flex-shrink: 0; }
.log-text { color: #e2e8f0; }
.log-empty { text-align: center; padding-top: 100px; }
</style>
