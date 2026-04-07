<template>
  <div class="flex min-h-0 flex-col">
    <div class="flex items-center justify-between mb-4">
      <div>
        <h1 class="text-2xl font-bold text-white">{{ t('terminal.title') }}</h1>
        <p class="text-gray-400 mt-1">{{ t('terminal.subtitle') }}</p>
      </div>
      <button class="btn-primary" @click="connectTerminal({ manual: true })" :disabled="connected || connecting">
        {{ connected ? t('terminal.connected') : connecting ? t('terminal.connecting') : t('terminal.connect') }}
      </button>
    </div>

    <div class="relative h-[62vh] min-h-[420px] max-h-[820px] w-full rounded-lg border border-panel-border bg-panel-darker p-2 overflow-hidden">
      <div ref="terminalContainer" class="h-full min-h-0 w-full overflow-hidden"></div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted, onBeforeUnmount, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { Terminal } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import '@xterm/xterm/css/xterm.css'
import { useAuthStore } from '../stores/auth'
import { useTheme } from '../composables/useTheme'

const { t } = useI18n({ useScope: 'global' })
const authStore = useAuthStore()
const { theme } = useTheme()

const terminalContainer = ref(null)
const connected = ref(false)
const connecting = ref(false)
let term = null
let fitAddon = null
let ws = null
let resizeHandler = null
let resizeObserver = null
let dataDisposable = null
let heartbeatTimer = null
let reconnectTimer = null
let reconnectAttempts = 0
let shouldReconnect = false
let disposed = false
let connectGeneration = 0

const terminalHeartbeatFrame = '__AURA_HEARTBEAT__'
const terminalHeartbeatIntervalMs = 20_000
const reconnectBaseDelayMs = 1_000
const reconnectMaxDelayMs = 10_000
const reconnectMaxAttempts = 8

function terminalTheme() {
  if (theme.value === 'light') {
    return {
      background: '#fff7ed',
      foreground: '#7c2d12',
      cursor: '#ea580c',
      selectionBackground: 'rgba(249, 115, 22, 0.22)',
    }
  }
  return {
    background: '#000000',
    foreground: '#ffffff',
    cursor: '#34d399',
    selectionBackground: 'rgba(16, 185, 129, 0.22)',
  }
}

function applyTerminalTheme() {
  if (!term) return
  term.options.theme = terminalTheme()
}

function buildTerminalUrl() {
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  const isDevPort = window.location.port === '5173'
  
  // Use the standard port/gateway URL to bypass CORS and proxy connection resets
  // Our gateway middleware now properly forwards the upgrade headers.
  const host = isDevPort ? `${window.location.hostname}:8090` : window.location.host
  return `${protocol}//${host}/api/v1/terminal/ws?token=${encodeURIComponent(authStore.token || '')}`
}

function clearHeartbeat() {
  if (heartbeatTimer) {
    window.clearInterval(heartbeatTimer)
    heartbeatTimer = null
  }
}

function startHeartbeat() {
  clearHeartbeat()
  heartbeatTimer = window.setInterval(() => {
    if (ws && ws.readyState === WebSocket.OPEN) {
      ws.send(terminalHeartbeatFrame)
    }
  }, terminalHeartbeatIntervalMs)
}

function clearReconnectTimer() {
  if (reconnectTimer) {
    window.clearTimeout(reconnectTimer)
    reconnectTimer = null
  }
}

function scheduleReconnect() {
  if (disposed || !shouldReconnect || connected.value || connecting.value) {
    return
  }
  if (reconnectAttempts >= reconnectMaxAttempts) {
    shouldReconnect = false
    return
  }
  reconnectAttempts += 1
  const delay = Math.min(reconnectBaseDelayMs * (2 ** (reconnectAttempts - 1)), reconnectMaxDelayMs)
  clearReconnectTimer()
  reconnectTimer = window.setTimeout(() => {
    connectTerminal({ manual: false })
  }, delay)
}

function closeSocket() {
  if (!ws) return
  const activeSocket = ws
  ws = null
  activeSocket.onopen = null
  activeSocket.onmessage = null
  activeSocket.onclose = null
  activeSocket.onerror = null
  if (activeSocket.readyState === WebSocket.CONNECTING || activeSocket.readyState === WebSocket.OPEN) {
    activeSocket.close()
  }
}

function ensureTerminal() {
  if (term) return

  term = new Terminal({
    theme: terminalTheme(),
    cursorBlink: true,
  })
  fitAddon = new FitAddon()
  term.loadAddon(fitAddon)
  term.open(terminalContainer.value)
  requestAnimationFrame(() => {
    fitAddon?.fit()
  })

  dataDisposable = term.onData((data) => {
    if (ws && ws.readyState === WebSocket.OPEN) {
      ws.send(data)
    }
  })

  resizeHandler = () => {
    if (fitAddon) fitAddon.fit()
  }
  window.addEventListener('resize', resizeHandler)

  if (typeof ResizeObserver !== 'undefined' && terminalContainer.value) {
    resizeObserver = new ResizeObserver(() => {
      if (fitAddon) fitAddon.fit()
    })
    resizeObserver.observe(terminalContainer.value)
  }
}

watch(theme, () => {
  applyTerminalTheme()
  if (fitAddon) {
    requestAnimationFrame(() => {
      fitAddon?.fit()
    })
  }
})

function connectTerminal({ manual = false } = {}) {
  if (connected.value || connecting.value || disposed) return

  ensureTerminal()
  clearReconnectTimer()

  if (manual) {
    shouldReconnect = true
    reconnectAttempts = 0
  }

  connecting.value = true
  term.writeln(t('terminal.connecting'))
  const wsUrl = buildTerminalUrl()
  const generation = ++connectGeneration
  const socket = new WebSocket(wsUrl)
  ws = socket

  socket.onopen = () => {
    if (generation !== connectGeneration || disposed) {
      socket.close()
      return
    }
    connecting.value = false
    connected.value = true
    reconnectAttempts = 0
    startHeartbeat()
    term.writeln('\r\n' + t('terminal.connected_msg') + '\r\n')
    term.focus()
  }

  socket.onmessage = (event) => {
    if (generation !== connectGeneration || disposed || !term) return
    if (typeof event.data === 'string') {
      term.write(event.data)
      return
    }
    if (event.data instanceof Blob) {
      event.data.text().then((text) => {
        if (generation === connectGeneration && !disposed && term) {
          term.write(text)
        }
      }).catch(() => {})
    }
  }

  socket.onerror = () => {
    if (generation !== connectGeneration || disposed) return
    if (socket.readyState === WebSocket.OPEN || socket.readyState === WebSocket.CONNECTING) {
      socket.close()
    }
  }

  socket.onclose = () => {
    if (generation !== connectGeneration) return
    clearHeartbeat()
    ws = null
    if (!disposed) {
      term?.writeln('\r\n' + t('terminal.disconnected') + '\r\n')
    }
    connecting.value = false
    connected.value = false
    scheduleReconnect()
  }
}

onMounted(() => {
  // optionally connect on mount
})

onBeforeUnmount(() => {
  disposed = true
  shouldReconnect = false
  clearReconnectTimer()
  clearHeartbeat()
  closeSocket()
  if (dataDisposable) dataDisposable.dispose()
  if (resizeHandler) window.removeEventListener('resize', resizeHandler)
  if (resizeObserver) {
    resizeObserver.disconnect()
    resizeObserver = null
  }
  if (term) {
    term.dispose()
    term = null
  }
  fitAddon = null
  connecting.value = false
  connected.value = false
})
</script>

<style scoped>
:deep(.xterm),
:deep(.xterm-viewport),
:deep(.xterm-screen) {
  height: 100%;
}
</style>
