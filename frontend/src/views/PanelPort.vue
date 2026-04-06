<template>
  <div class="space-y-6 max-w-4xl">
    <div>
      <h1 class="text-2xl font-bold text-white flex items-center gap-2">
        <Settings2 class="w-6 h-6 text-indigo-400" />
        {{ t('panel_port.title') }}
      </h1>
      <p class="text-gray-400 mt-1">
        {{ t('panel_port.subtitle') }}
      </p>
    </div>

    <div class="border-b border-panel-border">
      <nav class="flex gap-6">
        <button
          @click="setTab('port')"
          :class="[
            'pb-3 text-sm font-medium transition',
            activeTab === 'port' ? 'text-indigo-300 border-b-2 border-indigo-400' : 'text-gray-400 hover:text-white'
          ]"
        >
          {{ t('panel_port.tabs.port') }}
        </button>
        <button
          @click="setTab('reverse-domain')"
          :class="[
            'pb-3 text-sm font-medium transition',
            activeTab === 'reverse-domain' ? 'text-sky-300 border-b-2 border-sky-400' : 'text-gray-400 hover:text-white'
          ]"
        >
          {{ t('panel_port.tabs.reverse_domain') }}
        </button>
        <button
          @click="setTab('web-stack')"
          :class="[
            'pb-3 text-sm font-medium transition',
            activeTab === 'web-stack' ? 'text-emerald-300 border-b-2 border-emerald-400' : 'text-gray-400 hover:text-white'
          ]"
        >
          {{ t('panel_port.tabs.web_stack') }}
        </button>
      </nav>
    </div>

    <div v-if="activeTab === 'port'" class="space-y-6">
      <div class="bg-amber-500/10 border border-amber-500/30 rounded-xl p-4">
        <p class="text-amber-300 font-semibold text-sm">{{ t('panel_port.notice_title') }}</p>
        <p class="text-amber-100/90 text-sm mt-1">
          {{ t('panel_port.notice_body') }}
        </p>
      </div>

      <div class="bg-blue-500/10 border border-blue-500/30 rounded-xl p-4 text-sm">
        <p class="text-blue-300 font-semibold">{{ t('panel_port.current_config') }}</p>
        <p class="text-blue-100 mt-1">
          {{ t('panel_port.current_port') }}: <span class="font-bold">{{ currentPort }}</span>
          <span class="text-blue-200/80">({{ gatewayAddr }})</span>
        </p>
      </div>

      <div class="bg-panel-card border border-panel-border rounded-xl p-6 space-y-5">
        <div>
          <label class="block text-xs text-gray-400 uppercase tracking-wide mb-2">{{ t('panel_port.new_port') }}</label>
          <input
            v-model.number="port"
            type="number"
            min="1"
            max="65535"
            class="w-full bg-panel-darker border border-panel-border rounded-lg px-4 py-3 text-white focus:outline-none focus:border-indigo-400"
            :placeholder="t('panel_port.port_placeholder')"
          />
          <p class="text-xs text-gray-500 mt-2">{{ t('panel_port.valid_range') }}</p>
        </div>

        <label class="flex items-center gap-3 text-sm text-gray-300">
          <input v-model="openFirewall" type="checkbox" class="accent-indigo-500" />
          {{ t('panel_port.open_firewall') }}
        </label>

        <div class="pt-2">
          <button
            @click="changePort"
            :disabled="loading || saving"
            class="px-5 py-2.5 rounded-lg bg-indigo-600 hover:bg-indigo-500 disabled:opacity-50 text-white font-medium transition inline-flex items-center gap-2"
          >
            <Loader2 v-if="saving" class="w-4 h-4 animate-spin" />
            <Save v-else class="w-4 h-4" />
            <span>{{ saving ? t('panel_port.changing') : t('panel_port.change') }}</span>
          </button>
        </div>
      </div>

      <div v-if="portMessage" class="bg-emerald-500/10 border border-emerald-500/30 rounded-xl p-4 text-sm text-emerald-200">
        <p class="font-semibold">{{ portMessage }}</p>
        <p class="mt-1">{{ t('panel_port.reconnect_url') }}: <span class="font-mono">{{ newAccessUrl }}</span></p>
      </div>

      <div v-if="portWarnings.length" class="bg-yellow-500/10 border border-yellow-500/30 rounded-xl p-4 text-sm text-yellow-200">
        <p class="font-semibold mb-2">{{ t('panel_port.warnings') }}</p>
        <ul class="space-y-1">
          <li v-for="item in portWarnings" :key="item">- {{ item }}</li>
        </ul>
      </div>

      <div v-if="firewallActions.length" class="bg-panel-card border border-panel-border rounded-xl p-4 text-sm text-gray-300">
        <p class="font-semibold text-white mb-2">{{ t('panel_port.firewall_actions') }}</p>
        <ul class="space-y-1">
          <li v-for="item in firewallActions" :key="item">- {{ item }}</li>
        </ul>
      </div>

      <div v-if="portError" class="bg-red-500/10 border border-red-500/30 rounded-xl p-4 text-sm text-red-200">
        {{ portError }}
      </div>
    </div>

    <div v-else-if="activeTab === 'reverse-domain'" class="space-y-6">
      <div class="bg-sky-500/10 border border-sky-500/30 rounded-xl p-4">
        <p class="text-sky-300 font-semibold text-sm">{{ t('panel_port.reverse.title') }}</p>
        <p class="text-sky-100/90 text-sm mt-1">{{ t('panel_port.reverse.subtitle') }}</p>
      </div>

      <div class="bg-blue-500/10 border border-blue-500/30 rounded-xl p-4 text-sm space-y-1">
        <p class="text-blue-300 font-semibold">{{ t('panel_port.reverse.current_upstream') }}</p>
        <p class="text-blue-100 font-mono">{{ reverseGatewayUpstream }}</p>
      </div>

      <div class="bg-panel-card border border-panel-border rounded-xl p-6 space-y-5">
        <label class="flex items-center gap-3 text-sm text-gray-300">
          <input v-model="reverseEnabled" type="checkbox" class="accent-sky-500" />
          {{ t('panel_port.reverse.enabled') }}
        </label>

        <div>
          <label class="block text-xs text-gray-400 uppercase tracking-wide mb-2">{{ t('panel_port.reverse.domain') }}</label>
          <input
            v-model.trim="reverseDomain"
            type="text"
            class="w-full bg-panel-darker border border-panel-border rounded-lg px-4 py-3 text-white focus:outline-none focus:border-sky-400"
            :placeholder="t('panel_port.reverse.domain_placeholder')"
          />
        </div>

        <div>
          <label class="block text-xs text-gray-400 uppercase tracking-wide mb-2">{{ t('panel_port.reverse.vhost_conf_path') }}</label>
          <input
            v-model.trim="reverseVhostPath"
            type="text"
            class="w-full bg-panel-darker border border-panel-border rounded-lg px-4 py-3 text-white focus:outline-none focus:border-sky-400 font-mono text-sm"
            :placeholder="t('panel_port.reverse.vhost_conf_placeholder')"
          />
        </div>

        <div class="pt-2">
          <button
            @click="saveReverseDomain"
            :disabled="reverseLoading || reverseSaving"
            class="px-5 py-2.5 rounded-lg bg-sky-600 hover:bg-sky-500 disabled:opacity-50 text-white font-medium transition inline-flex items-center gap-2"
          >
            <Loader2 v-if="reverseSaving" class="w-4 h-4 animate-spin" />
            <Save v-else class="w-4 h-4" />
            <span>{{ reverseSaving ? t('panel_port.reverse.saving') : t('panel_port.reverse.save') }}</span>
          </button>
        </div>
      </div>

      <div v-if="reverseMessage" class="bg-emerald-500/10 border border-emerald-500/30 rounded-xl p-4 text-sm text-emerald-200">
        {{ reverseMessage }}
      </div>

      <div v-if="reverseWarnings.length" class="bg-yellow-500/10 border border-yellow-500/30 rounded-xl p-4 text-sm text-yellow-200">
        <p class="font-semibold mb-2">{{ t('panel_port.reverse.warnings') }}</p>
        <ul class="space-y-1">
          <li v-for="item in reverseWarnings" :key="item">- {{ item }}</li>
        </ul>
      </div>

      <div v-if="reverseError" class="bg-red-500/10 border border-red-500/30 rounded-xl p-4 text-sm text-red-200">
        {{ reverseError }}
      </div>
    </div>

    <div v-else class="space-y-6">
      <div class="bg-emerald-500/10 border border-emerald-500/30 rounded-xl p-4">
        <p class="text-emerald-300 font-semibold text-sm">{{ t('panel_port.web_stack.title') }}</p>
        <p class="text-emerald-100/90 text-sm mt-1">{{ t('panel_port.web_stack.subtitle') }}</p>
      </div>

      <div class="bg-blue-500/10 border border-blue-500/30 rounded-xl p-4 text-sm space-y-1">
        <p class="text-blue-300 font-semibold">{{ t('panel_port.web_stack.current_mode') }}</p>
        <p class="text-blue-100">
          {{ webStackModeLabel(webStackCurrentMode) }}
        </p>
        <p class="text-blue-100/80 text-xs">{{ t('panel_port.web_stack.status.nginx_active') }}: {{ webStackNginxActive ? t('panel_port.web_stack.yes') : t('panel_port.web_stack.no') }}</p>
        <p class="text-blue-100/80 text-xs">{{ t('panel_port.web_stack.status.openlitespeed_active') }}: {{ webStackOpenLiteSpeedActive ? t('panel_port.web_stack.yes') : t('panel_port.web_stack.no') }}</p>
      </div>

      <div class="bg-panel-card border border-panel-border rounded-xl p-6 space-y-5">
        <div>
          <p class="block text-xs text-gray-400 uppercase tracking-wide mb-3">{{ t('panel_port.web_stack.desired_mode') }}</p>
          <div class="space-y-3">
            <label class="flex items-start gap-3 text-sm text-gray-300 p-3 rounded-lg border border-panel-border">
              <input
                v-model="webStackMode"
                type="radio"
                name="web-stack-mode"
                value="ols-only"
                class="accent-emerald-500 mt-1"
              />
              <span>
                <span class="block text-white font-medium">{{ t('panel_port.web_stack.modes.ols_only.label') }}</span>
                <span class="block text-gray-400 text-xs mt-1">{{ t('panel_port.web_stack.modes.ols_only.description') }}</span>
              </span>
            </label>
            <label class="flex items-start gap-3 text-sm text-gray-300 p-3 rounded-lg border border-panel-border">
              <input
                v-model="webStackMode"
                type="radio"
                name="web-stack-mode"
                value="nginx-edge"
                class="accent-emerald-500 mt-1"
              />
              <span>
                <span class="block text-white font-medium">{{ t('panel_port.web_stack.modes.nginx_edge.label') }}</span>
                <span class="block text-gray-400 text-xs mt-1">{{ t('panel_port.web_stack.modes.nginx_edge.description') }}</span>
              </span>
            </label>
          </div>
        </div>

        <div class="pt-2">
          <button
            @click="saveWebStackMode"
            :disabled="webStackLoading || webStackSaving"
            class="px-5 py-2.5 rounded-lg bg-emerald-600 hover:bg-emerald-500 disabled:opacity-50 text-white font-medium transition inline-flex items-center gap-2"
          >
            <Loader2 v-if="webStackSaving" class="w-4 h-4 animate-spin" />
            <Save v-else class="w-4 h-4" />
            <span>{{ webStackSaving ? t('panel_port.web_stack.saving') : t('panel_port.web_stack.save') }}</span>
          </button>
        </div>
      </div>

      <div v-if="webStackMessage" class="bg-emerald-500/10 border border-emerald-500/30 rounded-xl p-4 text-sm text-emerald-200">
        {{ webStackMessage }}
      </div>

      <div v-if="webStackWarnings.length" class="bg-yellow-500/10 border border-yellow-500/30 rounded-xl p-4 text-sm text-yellow-200">
        <p class="font-semibold mb-2">{{ t('panel_port.web_stack.warnings') }}</p>
        <ul class="space-y-1">
          <li v-for="item in webStackWarnings" :key="item">- {{ item }}</li>
        </ul>
      </div>

      <div v-if="webStackOutput" class="bg-panel-card border border-panel-border rounded-xl p-4">
        <p class="font-semibold text-white text-sm mb-2">{{ t('panel_port.web_stack.last_output') }}</p>
        <pre class="text-xs text-gray-300 whitespace-pre-wrap break-words">{{ webStackOutput }}</pre>
      </div>

      <div v-if="webStackScriptPath" class="bg-panel-card border border-panel-border rounded-xl p-4 text-xs text-gray-400">
        {{ t('panel_port.web_stack.script_path') }}: <span class="font-mono">{{ webStackScriptPath }}</span>
      </div>

      <div v-if="webStackError" class="bg-red-500/10 border border-red-500/30 rounded-xl p-4 text-sm text-red-200">
        {{ webStackError }}
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { Loader2, Save, Settings2 } from 'lucide-vue-next'
import api from '../services/api'

const { t } = useI18n({ useScope: 'global' })
const route = useRoute()
const router = useRouter()

const normalizeTab = (value) => {
  if (value === 'reverse-domain') return 'reverse-domain'
  if (value === 'web-stack') return 'web-stack'
  return 'port'
}
const activeTab = ref(normalizeTab(typeof route.query.tab === 'string' ? route.query.tab : ''))

watch(
  () => route.query.tab,
  (value) => {
    const normalized = normalizeTab(typeof value === 'string' ? value : '')
    if (activeTab.value !== normalized) {
      activeTab.value = normalized
    }
  },
)

function setTab(tab) {
  const normalized = normalizeTab(tab)
  activeTab.value = normalized
  const currentTab = normalizeTab(typeof route.query.tab === 'string' ? route.query.tab : '')
  if (currentTab === normalized) return

  const nextQuery = { ...route.query }
  if (normalized === 'port') {
    delete nextQuery.tab
  } else {
    nextQuery.tab = normalized
  }
  router.replace({ path: route.path, query: nextQuery }).catch(() => {})
}

const loading = ref(false)
const saving = ref(false)
const port = ref(8090)
const currentPort = ref(8090)
const gatewayAddr = ref(':8090')
const openFirewall = ref(true)
const portMessage = ref('')
const portError = ref('')
const portWarnings = ref([])
const firewallActions = ref([])

const reverseLoading = ref(false)
const reverseSaving = ref(false)
const reverseEnabled = ref(false)
const reverseDomain = ref('')
const reverseVhostPath = ref('/usr/local/lsws/conf/vhosts/Example/vhconf.conf')
const reverseGatewayUpstream = ref('127.0.0.1:8090')
const reverseMessage = ref('')
const reverseError = ref('')
const reverseWarnings = ref([])

const webStackLoading = ref(false)
const webStackSaving = ref(false)
const webStackMode = ref('ols-only')
const webStackCurrentMode = ref('ols-only')
const webStackNginxActive = ref(false)
const webStackOpenLiteSpeedActive = ref(false)
const webStackSupportedModes = ref(['ols-only', 'nginx-edge'])
const webStackScriptPath = ref('')
const webStackMessage = ref('')
const webStackError = ref('')
const webStackWarnings = ref([])
const webStackOutput = ref('')

const newAccessUrl = computed(() => {
  if (typeof window === 'undefined') {
    return `http://YOUR_SERVER_IP:${port.value}`
  }
  return `${window.location.protocol}//${window.location.hostname}:${port.value}`
})

const loadPanelPort = async () => {
  loading.value = true
  portError.value = ''
  try {
    const response = await api.get('/status/panel-port')
    if (response.data?.status !== 'success') {
      throw new Error(response.data?.message || t('panel_port.messages.load_failed'))
    }

    const payload = response.data?.data || {}
    const fetchedPort = Number(payload.current_port || 8090)
    currentPort.value = fetchedPort
    port.value = fetchedPort
    gatewayAddr.value = payload.gateway_addr || `:${fetchedPort}`
    reverseGatewayUpstream.value = `127.0.0.1:${fetchedPort}`
  } catch (err) {
    portError.value = err?.message || t('panel_port.messages.load_failed')
  } finally {
    loading.value = false
  }
}

const changePort = async () => {
  portError.value = ''
  portMessage.value = ''
  portWarnings.value = []
  firewallActions.value = []

  const targetPort = Number(port.value)
  if (!Number.isInteger(targetPort) || targetPort < 1 || targetPort > 65535) {
    portError.value = t('panel_port.messages.invalid_port')
    return
  }

  saving.value = true
  try {
    const response = await api.post('/status/panel-port', {
      port: targetPort,
      open_firewall: openFirewall.value,
    })

    if (response.data?.status !== 'success') {
      throw new Error(response.data?.message || t('panel_port.messages.update_failed'))
    }

    const payload = response.data?.data || {}
    currentPort.value = targetPort
    gatewayAddr.value = payload.gateway_addr || `:${targetPort}`
    reverseGatewayUpstream.value = `127.0.0.1:${targetPort}`
    firewallActions.value = Array.isArray(payload.firewall_actions) ? payload.firewall_actions : []
    portWarnings.value = Array.isArray(payload.warnings) ? payload.warnings : []

    const restartNote = payload.restart_scheduled ? ` ${t('panel_port.messages.restart_scheduled')}` : ''
    portMessage.value = t('panel_port.messages.updated', { port: targetPort, restartNote })
  } catch (err) {
    const apiMessage = err?.response?.data?.message
    if (!apiMessage && err?.message && err.message.toLowerCase().includes('network')) {
      portMessage.value = t('panel_port.messages.network_reconnect', { url: newAccessUrl.value })
      return
    }
    portError.value = apiMessage || err?.message || t('panel_port.messages.update_failed')
  } finally {
    saving.value = false
  }
}

const loadReverseDomain = async () => {
  reverseLoading.value = true
  reverseError.value = ''
  try {
    const response = await api.get('/status/panel-reverse-domain')
    if (response.data?.status !== 'success') {
      throw new Error(response.data?.message || t('panel_port.reverse.messages.load_failed'))
    }

    const payload = response.data?.data || {}
    reverseEnabled.value = Boolean(payload.enabled)
    reverseDomain.value = String(payload.domain || '')
    reverseVhostPath.value = String(payload.vhost_conf_path || '/usr/local/lsws/conf/vhosts/Example/vhconf.conf')
    reverseGatewayUpstream.value = String(payload.gateway_upstream || reverseGatewayUpstream.value)
  } catch (err) {
    reverseError.value = err?.message || t('panel_port.reverse.messages.load_failed')
  } finally {
    reverseLoading.value = false
  }
}

const saveReverseDomain = async () => {
  reverseError.value = ''
  reverseMessage.value = ''
  reverseWarnings.value = []

  const normalizedDomain = String(reverseDomain.value || '').trim().toLowerCase()
  if (reverseEnabled.value && !normalizedDomain) {
    reverseError.value = t('panel_port.reverse.messages.domain_required')
    return
  }

  if (reverseEnabled.value && !/^(?!-)[a-z0-9-]+(\.[a-z0-9-]+)+$/.test(normalizedDomain)) {
    reverseError.value = t('panel_port.reverse.messages.invalid_domain')
    return
  }

  reverseSaving.value = true
  try {
    const response = await api.post('/status/panel-reverse-domain', {
      enabled: reverseEnabled.value,
      domain: normalizedDomain,
      vhost_conf_path: String(reverseVhostPath.value || '').trim(),
    })

    if (response.data?.status !== 'success') {
      throw new Error(response.data?.message || t('panel_port.reverse.messages.update_failed'))
    }

    const payload = response.data?.data || {}
    reverseEnabled.value = Boolean(payload.enabled)
    reverseDomain.value = String(payload.domain || '')
    reverseVhostPath.value = String(payload.vhost_conf_path || reverseVhostPath.value)
    reverseGatewayUpstream.value = String(payload.gateway_upstream || reverseGatewayUpstream.value)
    reverseWarnings.value = Array.isArray(payload.warnings) ? payload.warnings : []

    if (payload.edge_synced) {
      reverseMessage.value = t('panel_port.reverse.messages.updated_synced')
    } else {
      reverseMessage.value = t('panel_port.reverse.messages.updated')
    }
  } catch (err) {
    reverseError.value = err?.response?.data?.message || err?.message || t('panel_port.reverse.messages.update_failed')
  } finally {
    reverseSaving.value = false
  }
}

const webStackModeLabel = (mode) => {
  return mode === 'nginx-edge'
    ? t('panel_port.web_stack.modes.nginx_edge.label')
    : t('panel_port.web_stack.modes.ols_only.label')
}

const loadWebStack = async () => {
  webStackLoading.value = true
  webStackError.value = ''
  try {
    const response = await api.get('/status/web-stack')
    if (response.data?.status !== 'success') {
      throw new Error(response.data?.message || t('panel_port.web_stack.messages.load_failed'))
    }

    const payload = response.data?.data || {}
    const mode = String(payload.mode || 'ols-only')
    webStackCurrentMode.value = mode
    webStackMode.value = mode
    webStackNginxActive.value = Boolean(payload.nginx_active)
    webStackOpenLiteSpeedActive.value = Boolean(payload.openlitespeed_active)
    webStackScriptPath.value = String(payload.script_path || '')
    webStackSupportedModes.value = Array.isArray(payload.supported_modes) ? payload.supported_modes : ['ols-only', 'nginx-edge']
  } catch (err) {
    webStackError.value = err?.response?.data?.message || err?.message || t('panel_port.web_stack.messages.load_failed')
  } finally {
    webStackLoading.value = false
  }
}

const saveWebStackMode = async () => {
  webStackError.value = ''
  webStackMessage.value = ''
  webStackWarnings.value = []
  webStackOutput.value = ''

  if (!webStackSupportedModes.value.includes(webStackMode.value)) {
    webStackError.value = t('panel_port.web_stack.messages.invalid_mode')
    return
  }

  webStackSaving.value = true
  try {
    const response = await api.post('/status/web-stack', {
      mode: webStackMode.value,
    })
    if (response.data?.status !== 'success') {
      throw new Error(response.data?.message || t('panel_port.web_stack.messages.update_failed'))
    }

    const payload = response.data?.data || {}
    webStackCurrentMode.value = String(payload.current_mode || webStackMode.value)
    webStackMode.value = webStackCurrentMode.value
    webStackNginxActive.value = Boolean(payload.nginx_active)
    webStackOpenLiteSpeedActive.value = Boolean(payload.openlitespeed_active)
    webStackScriptPath.value = String(payload.script_path || webStackScriptPath.value)
    webStackWarnings.value = Array.isArray(payload.warnings) ? payload.warnings : []
    webStackOutput.value = String(payload.execution_output || '')

    if (payload.applied) {
      webStackMessage.value = t('panel_port.web_stack.messages.updated', { mode: webStackModeLabel(webStackCurrentMode.value) })
    } else {
      webStackMessage.value = t('panel_port.web_stack.messages.unchanged', { mode: webStackModeLabel(webStackCurrentMode.value) })
    }
  } catch (err) {
    webStackError.value = err?.response?.data?.message || err?.message || t('panel_port.web_stack.messages.update_failed')
  } finally {
    webStackSaving.value = false
  }
}

onMounted(async () => {
  await Promise.all([loadPanelPort(), loadReverseDomain(), loadWebStack()])
})
</script>
