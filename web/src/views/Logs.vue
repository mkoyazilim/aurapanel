<template>
  <Layout>
    <div class="page">
      <!-- Başlık ve Kontrol Paneli -->
      <div class="logs-header-card card">
        <div class="logs-title-row">
          <div>
            <h1 style="margin: 0; font-size: 22px; display: flex; align-items: center; gap: 8px">
              📡 {{ $t('logs.title') }}
            </h1>
            <p class="muted text-sm" style="margin: 4px 0 0 0">{{ $t('logs.subtitle') }}</p>
          </div>

          <div class="stream-badge-container">
            <div class="stream-badge" :class="connected && !paused ? 'live' : 'paused'">
              <span class="pulse-dot"></span>
              <span>{{ connected && !paused ? 'CANLI AKIŞ' : (paused ? 'DURAKLATILDI' : 'BAĞLANIYOR...') }}</span>
            </div>
          </div>
        </div>

        <!-- Kontrol Çubuğu -->
        <div class="logs-toolbar">
          <!-- Site Seçimi -->
          <div class="tool-item site-select-wrapper">
            <label class="tool-label">Site:</label>
            <select v-model="selectedSite" @change="restartTail" class="custom-select">
              <option value="" disabled>{{ $t('logs.select_site') }}</option>
              <option v-for="s in sites" :key="s.id" :value="s.id">🌐 {{ s.name }}</option>
            </select>
          </div>

          <!-- Log Türü Tabları -->
          <div class="tool-item">
            <div class="pill-tabs">
              <button
                class="pill-tab"
                :class="{ active: logType === 'access' }"
                @click="switchType('access')"
              >
                📄 Access Log
              </button>
              <button
                class="pill-tab"
                :class="{ active: logType === 'error' }"
                @click="switchType('error')"
              >
                ⚠️ Error Log
              </button>
            </div>
          </div>

          <!-- Anlık Filtreleme / Arama -->
          <div class="tool-item filter-wrapper">
            <div class="search-input-box">
              <span class="search-icon">🔍</span>
              <input
                v-model="searchQuery"
                type="text"
                placeholder="IP, HTTP kod, path veya hata ara..."
                class="search-input"
              />
              <button v-if="searchQuery" @click="searchQuery = ''" class="clear-search-btn">✕</button>
            </div>
          </div>

          <!-- Aksiyon Butonları -->
          <div class="tool-actions">
            <!-- Otomatik Kaydır -->
            <button
              class="btn btn-sm"
              :class="autoScroll ? 'primary' : ''"
              @click="autoScroll = !autoScroll"
              :title="autoScroll ? 'Otomatik Kaydırma Açık' : 'Otomatik Kaydırma Kapalı'"
            >
              📌 {{ autoScroll ? 'Oto-Kaydır: Açık' : 'Oto-Kaydır: Kapalı' }}
            </button>

            <!-- Duraklat / Devam Et -->
            <button
              class="btn btn-sm"
              :class="paused ? 'btn-success' : 'btn-warning'"
              @click="togglePause"
            >
              {{ paused ? '▶️ ' + $t('logs.resume') : '⏸️ ' + $t('logs.pause') }}
            </button>

            <!-- Temizle -->
            <button class="btn btn-sm danger" @click="clearLogs">
              🗑️ {{ $t('logs.clear') }}
            </button>

            <!-- İndir -->
            <button class="btn btn-sm" @click="downloadLogs" title="Görüntülenen logları indir">
              📥 İndir
            </button>
          </div>
        </div>
      </div>

      <!-- Site Seçilmemiş Durumu -->
      <div v-if="!selectedSite" class="empty-state card">
        <div style="font-size: 40px; margin-bottom: 12px">📋</div>
        <p class="muted">{{ $t('logs.select_site_prompt') }}</p>
      </div>

      <!-- Terminal Konsolu -->
      <div v-else class="terminal-card" :class="{ fullscreen: isFullscreen }">
        <!-- Terminal Üst Bar (macOS stili) -->
        <div class="terminal-topbar">
          <div class="mac-buttons">
            <span class="mac-btn red" @click="toggleFullscreen"></span>
            <span class="mac-btn yellow" @click="clearLogs"></span>
            <span class="mac-btn green" @click="togglePause"></span>
          </div>

          <div class="terminal-title mono">
            <span>{{ selectedSite }}</span>
            <span class="sep">/</span>
            <span>{{ logType }}.log</span>
            <span class="lines-count">({{ filteredLines.length }} satır)</span>
          </div>

          <div class="terminal-tools">
            <button class="icon-btn" @click="toggleFullscreen" :title="isFullscreen ? 'Normal Boyut' : 'Tam Ekran'">
              {{ isFullscreen ? '🗗' : '⛶' }}
            </button>
          </div>
        </div>

        <!-- Terminal Gövdesi -->
        <div ref="logBox" class="terminal-body" @scroll="handleScroll">
          <div v-if="filteredLines.length === 0" class="log-empty">
            <div class="spinner-pulse" v-if="connected && !paused && !searchQuery"></div>
            <p class="mono text-sm" style="color: #94a3b8">
              {{ searchQuery ? 'Aranan kriterle eşleşen log satırı bulunamadı.' : $t('logs.waiting_logs') }}
            </p>
          </div>

          <div
            v-for="(l, idx) in filteredLines"
            :key="idx"
            class="terminal-row"
            :class="getRowClass(l.line)"
          >
            <span class="row-num mono">{{ idx + 1 }}</span>
            <span v-if="l.ts" class="row-ts mono">{{ formatTs(l.ts) }}</span>
            <span class="row-content mono" v-html="highlightLine(l.line)"></span>
          </div>
        </div>
      </div>
    </div>
  </Layout>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted, nextTick } from 'vue'
import Layout from '../components/Layout.vue'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()
const sites = ref([])
const selectedSite = ref('')
const logType = ref('access')
const lines = ref([])
const connected = ref(false)
const paused = ref(false)
const autoScroll = ref(true)
const searchQuery = ref('')
const isFullscreen = ref(false)
const logBox = ref(null)
let eventSource = null

const filteredLines = computed(() => {
  if (!searchQuery.value) return lines.value
  const q = searchQuery.value.toLowerCase()
  return lines.value.filter(item => (item.line || '').toLowerCase().includes(q))
})

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

function toggleFullscreen() {
  isFullscreen.value = !isFullscreen.value
}

function downloadLogs() {
  const content = filteredLines.value.map(l => (l.ts ? `[${l.ts}] ` : '') + l.line).join('\n')
  const blob = new Blob([content], { type: 'text/plain;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = `${selectedSite.value}-${logType.value}-${new Date().toISOString().slice(0, 19)}.log`
  a.click()
  URL.revokeObjectURL(url)
}

function formatTs(ts) {
  if (!ts) return ''
  try {
    const d = new Date(ts)
    if (isNaN(d.getTime())) return ts
    return d.toTimeString().split(' ')[0]
  } catch {
    return ts
  }
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
      if (lines.value.length > 2000) {
        lines.value.shift()
      }
      if (autoScroll.value) {
        nextTick(() => {
          if (logBox.value) {
            logBox.value.scrollTop = logBox.value.scrollHeight
          }
        })
      }
    } catch {
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

function handleScroll(e) {
  const el = e.target
  const isNearBottom = el.scrollHeight - el.scrollTop - el.clientHeight < 40
  if (!isNearBottom && autoScroll.value) {
    // kullanıcı yukarı kaydırdı
  }
}

function getRowClass(line) {
  if (!line) return ''
  const l = line.toLowerCase()
  if (l.includes('error') || l.includes('fatal') || l.includes(' 500 ') || l.includes(' 502 ') || l.includes(' 503 ')) {
    return 'row-error'
  }
  if (l.includes('warn') || l.includes(' 404 ') || l.includes(' 403 ')) {
    return 'row-warn'
  }
  if (l.includes(' 200 ') || l.includes(' 201 ') || l.includes(' 204 ')) {
    return 'row-success'
  }
  return ''
}

function highlightLine(line) {
  if (!line) return ''
  let escaped = line
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')

  // HTTP Method Highlights
  escaped = escaped.replace(/\b(GET|POST|PUT|DELETE|PATCH|HEAD|OPTIONS)\b/g, '<span class="hl-method">$1</span>')

  // HTTP Status Codes
  escaped = escaped.replace(/\b(200|201|204)\b/g, '<span class="hl-status-200">$1</span>')
  escaped = escaped.replace(/\b(301|302|304|307|308)\b/g, '<span class="hl-status-300">$1</span>')
  escaped = escaped.replace(/\b(400|401|403|404|405|429)\b/g, '<span class="hl-status-400">$1</span>')
  escaped = escaped.replace(/\b(500|502|503|504)\b/g, '<span class="hl-status-500">$1</span>')

  // IP Addresses
  escaped = escaped.replace(/\b(\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3})\b/g, '<span class="hl-ip">$1</span>')

  // Error Tags
  escaped = escaped.replace(/\[ERROR\]/g, '<span class="hl-tag-error">[ERROR]</span>')
  escaped = escaped.replace(/\[WARN\]/g, '<span class="hl-tag-warn">[WARN]</span>')
  escaped = escaped.replace(/\[INFO\]/g, '<span class="hl-tag-info">[INFO]</span>')

  // Search Query Highlight
  if (searchQuery.value) {
    const q = searchQuery.value.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
    const re = new RegExp(`(${q})`, 'gi')
    escaped = escaped.replace(re, '<mark class="hl-search">$1</mark>')
  }

  return escaped
}

onMounted(fetchSites)
onUnmounted(stopTail)
</script>

<style scoped>
.logs-header-card {
  margin-bottom: 16px;
  padding: 16px 20px;
}

.logs-title-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 16px;
  border-bottom: 1px solid var(--border-color, #e2e8f0);
  padding-bottom: 12px;
}

.stream-badge {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.5px;
  padding: 4px 10px;
  border-radius: 9999px;
  text-transform: uppercase;
}

.stream-badge.live {
  background: rgba(34, 197, 94, 0.15);
  color: #16a34a;
  border: 1px solid rgba(34, 197, 94, 0.3);
}

.stream-badge.paused {
  background: rgba(234, 179, 8, 0.15);
  color: #ca8a04;
  border: 1px solid rgba(234, 179, 8, 0.3);
}

.pulse-dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: currentColor;
  box-shadow: 0 0 6px currentColor;
  animation: pulse 1.8s infinite;
}

@keyframes pulse {
  0% { transform: scale(0.9); opacity: 1; }
  50% { transform: scale(1.3); opacity: 0.5; }
  100% { transform: scale(0.9); opacity: 1; }
}

.logs-toolbar {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 12px;
}

.tool-item {
  display: flex;
  align-items: center;
  gap: 8px;
}

.tool-label {
  font-size: 13px;
  font-weight: 600;
  color: var(--text-muted, #64748b);
}

.custom-select {
  padding: 6px 12px;
  font-size: 13px;
  font-weight: 500;
  border-radius: 8px;
  border: 1px solid var(--border-color, #cbd5e1);
  background: var(--bg-input, #ffffff);
  color: var(--text-color, #1e293b);
  outline: none;
  min-width: 180px;
}

.pill-tabs {
  display: flex;
  background: rgba(0, 0, 0, 0.05);
  padding: 3px;
  border-radius: 8px;
  border: 1px solid rgba(0, 0, 0, 0.05);
}

.pill-tab {
  padding: 5px 12px;
  font-size: 12px;
  font-weight: 600;
  border-radius: 6px;
  border: none;
  background: transparent;
  color: var(--text-muted, #64748b);
  cursor: pointer;
  transition: all 0.15s ease;
}

.pill-tab.active {
  background: var(--primary-color, #3b82f6);
  color: #ffffff;
  box-shadow: 0 2px 4px rgba(59, 130, 246, 0.25);
}

.filter-wrapper {
  flex: 1;
  min-width: 220px;
}

.search-input-box {
  position: relative;
  width: 100%;
}

.search-icon {
  position: absolute;
  left: 10px;
  top: 50%;
  transform: translateY(-50%);
  font-size: 12px;
  opacity: 0.6;
}

.search-input {
  width: 100%;
  padding: 6px 30px 6px 30px;
  font-size: 13px;
  border-radius: 8px;
  border: 1px solid var(--border-color, #cbd5e1);
  background: var(--bg-input, #ffffff);
  color: var(--text-color, #1e293b);
  outline: none;
}

.clear-search-btn {
  position: absolute;
  right: 8px;
  top: 50%;
  transform: translateY(-50%);
  border: none;
  background: transparent;
  font-size: 12px;
  color: #94a3b8;
  cursor: pointer;
}

.tool-actions {
  display: flex;
  gap: 8px;
  align-items: center;
}

/* Terminal Tasarımı */
.terminal-card {
  background: #090d16;
  border-radius: 12px;
  border: 1px solid #1e293b;
  overflow: hidden;
  box-shadow: 0 10px 30px -5px rgba(0, 0, 0, 0.4);
  display: flex;
  flex-direction: column;
  height: calc(100vh - 280px);
  min-height: 520px;
  transition: all 0.2s ease;
}

.terminal-card.fullscreen {
  position: fixed;
  inset: 12px;
  z-index: 2000;
  height: calc(100vh - 24px);
  border-radius: 12px;
}

.terminal-topbar {
  background: #131b2e;
  padding: 10px 16px;
  border-bottom: 1px solid #1e293b;
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.mac-buttons {
  display: flex;
  gap: 6px;
}

.mac-btn {
  width: 11px;
  height: 11px;
  border-radius: 50%;
  display: inline-block;
  cursor: pointer;
}

.mac-btn.red { background: #ef4444; }
.mac-btn.yellow { background: #f59e0b; }
.mac-btn.green { background: #10b981; }

.terminal-title {
  font-size: 12px;
  font-weight: 600;
  color: #94a3b8;
  display: flex;
  align-items: center;
  gap: 6px;
}

.terminal-title .sep { color: #475569; }
.terminal-title .lines-count { color: #64748b; font-size: 11px; font-weight: normal; }

.icon-btn {
  background: transparent;
  border: 1px solid #334155;
  color: #94a3b8;
  border-radius: 6px;
  padding: 2px 8px;
  cursor: pointer;
  font-size: 14px;
}
.icon-btn:hover { background: #1e293b; color: #f8fafc; }

.terminal-body {
  flex: 1;
  padding: 14px 18px;
  overflow-y: auto;
  font-family: 'JetBrains Mono', 'Fira Code', 'Cascadia Code', ui-monospace, SFMono-Regular, monospace;
  font-size: 12.5px;
  line-height: 1.6;
  background: #090d16;
}

.terminal-row {
  display: flex;
  gap: 12px;
  align-items: baseline;
  padding: 2px 6px;
  border-radius: 4px;
  transition: background 0.1s;
}

.terminal-row:hover {
  background: rgba(255, 255, 255, 0.04);
}

.terminal-row.row-error {
  background: rgba(239, 68, 68, 0.12);
  border-left: 2px solid #ef4444;
}

.terminal-row.row-warn {
  background: rgba(245, 158, 11, 0.08);
  border-left: 2px solid #f59e0b;
}

.row-num {
  color: #334155;
  font-size: 11px;
  min-width: 32px;
  text-align: right;
  user-select: none;
  flex-shrink: 0;
}

.row-ts {
  color: #0ea5e9;
  font-size: 11px;
  opacity: 0.8;
  flex-shrink: 0;
  user-select: none;
}

.row-content {
  color: #e2e8f0;
  white-space: pre-wrap;
  word-break: break-all;
  flex: 1;
}

/* Syntax Highlighting */
:deep(.hl-method) { color: #38bdf8; font-weight: 700; }
:deep(.hl-status-200) { color: #4ade80; font-weight: 700; }
:deep(.hl-status-300) { color: #60a5fa; }
:deep(.hl-status-400) { color: #fb923c; font-weight: 700; }
:deep(.hl-status-500) { color: #f87171; font-weight: 700; }
:deep(.hl-ip) { color: #a78bfa; }
:deep(.hl-tag-error) { background: #ef4444; color: #fff; padding: 1px 4px; border-radius: 4px; font-weight: 700; font-size: 11px; }
:deep(.hl-tag-warn) { background: #f59e0b; color: #fff; padding: 1px 4px; border-radius: 4px; font-weight: 700; font-size: 11px; }
:deep(.hl-tag-info) { background: #0284c7; color: #fff; padding: 1px 4px; border-radius: 4px; font-size: 11px; }
:deep(.hl-search) { background: #facc15; color: #0f172a; padding: 0 2px; border-radius: 2px; font-weight: 700; }

.log-empty {
  text-align: center;
  padding: 120px 20px;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
}

.spinner-pulse {
  width: 24px;
  height: 24px;
  border-radius: 50%;
  border: 2px solid #38bdf8;
  border-top-color: transparent;
  animation: spin 1s linear infinite;
}

@keyframes spin {
  to { transform: rotate(360deg); }
}
</style>
