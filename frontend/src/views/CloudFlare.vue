<template>
  <div class="cloudflare-view space-y-6">
    <div class="flex items-center justify-between">
      <div>
        <h1 class="flex items-center gap-3 text-2xl font-bold text-white">
          <Cloud class="h-7 w-7 text-orange-400" />
          {{ t('cloudflare_manager.title') }}
        </h1>
        <p class="mt-1 text-gray-400">{{ t('cloudflare_manager.subtitle') }}</p>
      </div>
    </div>

    <div class="rounded-xl border border-panel-border bg-panel-card p-5">
      <div class="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
        <div>
          <p class="text-sm font-semibold text-white">{{ t('cloudflare_manager.server_status.title') }}</p>
          <p class="mt-1 text-sm text-gray-400">
            {{
              serverStatus.configured
                ? t('cloudflare_manager.server_status.ready', {
                    source: serverStatus.credential_source,
                    email: serverStatus.email_hint || '-',
                  })
                : t('cloudflare_manager.server_status.missing')
            }}
          </p>
        </div>
        <div class="flex flex-wrap items-center gap-2">
          <button
            v-if="cfEmail || cfApiKey"
            class="rounded-lg bg-gradient-to-r from-cyan-600 to-sky-600 px-4 py-2 text-sm font-medium text-white transition hover:from-cyan-700 hover:to-sky-700 disabled:cursor-not-allowed disabled:opacity-60"
            :disabled="serverSaving"
            @click="saveServerAuth"
          >
            {{ serverSaving ? t('cloudflare_manager.server_status.saving') : t('cloudflare_manager.server_status.save_action') }}
          </button>
          <span class="rounded-full px-3 py-1 text-xs font-medium" :class="serverStatus.configured ? 'bg-green-500/15 text-green-400' : 'bg-yellow-500/15 text-yellow-300'">
            {{ serverStatus.configured ? t('cloudflare_manager.server_status.configured') : t('cloudflare_manager.server_status.not_configured') }}
          </span>
          <span class="rounded-full px-3 py-1 text-xs font-medium" :class="serverStatus.auto_sync ? 'bg-cyan-500/15 text-cyan-300' : 'bg-gray-500/15 text-gray-400'">
            {{ serverStatus.auto_sync ? t('cloudflare_manager.server_status.auto_sync_on') : t('cloudflare_manager.server_status.auto_sync_off') }}
          </span>
        </div>
      </div>
    </div>

    <div v-if="!connected" class="rounded-xl border border-panel-border bg-panel-card p-6">
      <h2 class="cf-connect-title mb-4 text-lg font-semibold">{{ t('cloudflare_manager.connect_title') }}</h2>
      <div v-if="serverStatus.configured" class="mb-4 rounded-xl border border-green-500/20 bg-green-500/10 p-4 text-sm text-green-200">
        {{ t('cloudflare_manager.server_status.server_auth_active') }}
      </div>
      <div class="grid grid-cols-1 gap-4 md:grid-cols-2">
        <div>
          <label class="cf-field-label mb-1 block text-sm">{{ t('cloudflare_manager.email') }}</label>
          <input v-model="cfEmail" type="email" placeholder="user@example.com" class="aura-input" />
        </div>
        <div>
          <label class="cf-field-label mb-1 block text-sm">{{ t('cloudflare_manager.api_key') }}</label>
          <input v-model="cfApiKey" type="password" placeholder="****************" class="aura-input" />
        </div>
      </div>
      <button
        class="mt-4 rounded-lg bg-gradient-to-r from-orange-600 to-amber-600 px-6 py-2.5 font-medium text-white transition hover:from-orange-700 hover:to-amber-700"
        :disabled="loading"
        @click="connectCf"
      >
        {{ loading ? t('cloudflare_manager.connecting') : t('cloudflare_manager.connect') }}
      </button>
    </div>

    <template v-if="connected">
      <div class="border-b border-panel-border">
        <nav class="flex gap-6">
          <button
            v-for="tabItem in tabs"
            :key="tabItem.id"
            class="flex items-center gap-2 pb-3 text-sm font-medium transition"
            :class="activeTab === tabItem.id ? 'border-b-2 border-orange-400 text-orange-400' : 'text-gray-400 hover:text-white'"
            @click="activeTab = tabItem.id"
          >
            {{ tabItem.label }}
          </button>
        </nav>
      </div>

      <div v-if="activeTab === 'zones'">
        <div class="overflow-hidden rounded-xl border border-panel-border bg-panel-card">
          <div class="flex items-center justify-between border-b border-panel-border p-4">
            <h2 class="text-lg font-semibold text-white">{{ t('cloudflare_manager.zones.title') }}</h2>
            <button class="rounded-lg bg-panel-hover px-3 py-1.5 text-sm text-gray-300 transition hover:bg-gray-600" @click="loadZones">
              {{ t('cloudflare_manager.refresh') }}
            </button>
          </div>
          <table class="w-full text-sm">
            <thead>
              <tr class="border-b border-panel-border text-gray-400">
                <th class="px-4 py-3 text-left">{{ t('cloudflare_manager.zones.domain') }}</th>
                <th class="px-4 py-3 text-left">{{ t('cloudflare_manager.zones.status') }}</th>
                <th class="px-4 py-3 text-left">{{ t('cloudflare_manager.zones.plan') }}</th>
                <th class="px-4 py-3 text-left">{{ t('cloudflare_manager.zones.nameservers') }}</th>
                <th class="px-4 py-3 text-right">{{ t('cloudflare_manager.zones.action') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-if="zones.length === 0">
                <td colspan="5" class="px-4 py-4 text-center text-gray-500">{{ t('cloudflare_manager.zones.empty') }}</td>
              </tr>
              <tr v-for="zone in zones" :key="zone.id" class="border-b border-panel-border/50 transition hover:bg-white/[0.02]">
                <td class="px-4 py-3 font-medium text-white">{{ zone.name }}</td>
                <td class="px-4 py-3">
                  <span :class="['rounded px-2 py-0.5 text-xs font-medium', zone.status === 'active' ? 'bg-green-500/15 text-green-400' : 'bg-yellow-500/15 text-yellow-400']">{{ zone.status }}</span>
                </td>
                <td class="px-4 py-3 text-gray-300">{{ zone.plan }}</td>
                <td class="px-4 py-3 font-mono text-xs text-gray-400">{{ zone.name_servers?.join(', ') }}</td>
                <td class="px-4 py-3 text-right">
                  <button class="rounded bg-orange-600/20 px-3 py-1 text-xs text-orange-400 transition hover:bg-orange-600/40" @click="selectZone(zone)">
                    {{ t('cloudflare_manager.zones.manage_dns') }}
                  </button>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>

      <div v-if="activeTab === 'dns'" class="space-y-4">
        <div class="flex items-center gap-3">
          <span class="text-sm text-gray-400">{{ t('cloudflare_manager.dns.zone_label') }}:</span>
          <span class="font-semibold text-white">{{ selectedZone?.name || t('cloudflare_manager.dns.not_selected') }}</span>
          <button v-if="selectedZone" class="ml-auto rounded-lg bg-gradient-to-r from-orange-600 to-amber-600 px-4 py-2 text-sm text-white transition hover:from-orange-700 hover:to-amber-700" @click="showAddDns = true">
            {{ t('cloudflare_manager.dns.add_record') }}
          </button>
        </div>
        <div class="overflow-hidden rounded-xl border border-panel-border bg-panel-card">
          <table class="w-full text-sm">
            <thead>
              <tr class="border-b border-panel-border text-gray-400">
                <th class="px-4 py-3 text-left">{{ t('cloudflare_manager.dns.type') }}</th>
                <th class="px-4 py-3 text-left">{{ t('cloudflare_manager.dns.name') }}</th>
                <th class="px-4 py-3 text-left">{{ t('cloudflare_manager.dns.value') }}</th>
                <th class="px-4 py-3 text-left">{{ t('cloudflare_manager.dns.ttl') }}</th>
                <th class="px-4 py-3 text-left">{{ t('cloudflare_manager.dns.proxy') }}</th>
                <th class="px-4 py-3 text-right">{{ t('cloudflare_manager.dns.action') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-if="dnsRecords.length === 0">
                <td colspan="6" class="px-4 py-4 text-center text-gray-500">{{ t('cloudflare_manager.dns.empty') }}</td>
              </tr>
              <tr v-for="record in dnsRecords" :key="record.id" class="border-b border-panel-border/50 transition hover:bg-white/[0.02]">
                <td class="px-4 py-3">
                  <span :class="['rounded px-2 py-0.5 text-xs font-bold', dnsTypeBadge(record.type)]">{{ record.type }}</span>
                </td>
                <td class="px-4 py-3 font-mono text-xs text-white">{{ record.name }}</td>
                <td class="max-w-[200px] truncate px-4 py-3 font-mono text-xs text-gray-300">{{ record.content }}</td>
                <td class="px-4 py-3 text-gray-400">{{ record.ttl === 1 ? t('cloudflare_manager.dns.auto') : record.ttl }}</td>
                <td class="px-4 py-3 text-xs" :class="record.proxied ? 'text-orange-400' : 'text-gray-500'">
                  {{ record.proxied ? t('cloudflare_manager.dns.proxy_on') : t('cloudflare_manager.dns.proxy_off') }}
                </td>
                <td class="px-4 py-3 text-right">
                  <button class="rounded bg-red-600/20 px-2 py-1 text-xs text-red-400 transition hover:bg-red-600/40" @click="deleteDnsRecord(record.id)">
                    {{ t('common.delete') }}
                  </button>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>

      <div v-if="activeTab === 'ssl'" class="grid grid-cols-1 gap-4 md:grid-cols-2">
        <div class="rounded-xl border border-panel-border bg-panel-card p-5">
          <h3 class="mb-4 font-semibold text-white">{{ t('cloudflare_manager.ssl.mode_title') }}</h3>
          <div class="space-y-3">
            <label
              v-for="mode in sslModes"
              :key="mode.value"
              class="flex cursor-pointer items-start gap-3 rounded-lg border p-3 transition hover:bg-white/[0.03]"
              :class="selectedSslMode === mode.value ? 'border-orange-500/30 bg-orange-500/10' : 'border-transparent'"
              @click="setSslMode(mode.value)"
            >
              <input v-model="selectedSslMode" type="radio" :value="mode.value" class="mt-1 accent-orange-500" />
              <div>
                <p class="text-sm font-medium text-white">{{ mode.label }}</p>
                <p class="mt-0.5 text-xs text-gray-400">{{ mode.desc }}</p>
              </div>
            </label>
          </div>
        </div>
        <div class="space-y-4">
          <div class="rounded-xl border border-panel-border bg-panel-card p-5">
            <h3 class="mb-3 font-semibold text-white">{{ t('cloudflare_manager.ssl.always_https') }}</h3>
            <button class="rounded-lg px-4 py-2 text-sm transition" :class="alwaysHttps ? 'bg-green-600 text-white' : 'bg-panel-hover text-gray-400'" @click="toggleAlwaysHttps">
              {{ alwaysHttps ? t('cloudflare_manager.ssl.enabled') : t('cloudflare_manager.ssl.disabled') }}
            </button>
          </div>
          <div class="rounded-xl border border-panel-border bg-panel-card p-5">
            <h3 class="mb-3 font-semibold text-white">{{ t('cloudflare_manager.ssl.minify') }}</h3>
            <div class="flex gap-3">
              <label v-for="option in ['JS', 'CSS', 'HTML']" :key="option" class="flex items-center gap-2 text-sm text-gray-300">
                <input v-model="minifyOptions[option.toLowerCase()]" type="checkbox" class="accent-orange-500" />
                {{ option }}
              </label>
            </div>
            <button class="mt-3 rounded-lg bg-orange-600/20 px-4 py-1.5 text-sm text-orange-400 transition hover:bg-orange-600/40" @click="saveMinify">
              {{ t('cloudflare_manager.ssl.save') }}
            </button>
          </div>
        </div>
      </div>

      <div v-if="activeTab === 'cache'" class="grid grid-cols-1 gap-4 md:grid-cols-2">
        <div class="rounded-xl border border-panel-border bg-panel-card p-6 text-center">
          <div class="mx-auto mb-4 flex h-16 w-16 items-center justify-center rounded-full bg-red-500/10">
            <Trash2 class="h-8 w-8 text-red-400" />
          </div>
          <h3 class="mb-2 text-lg font-semibold text-white">{{ t('cloudflare_manager.cache.purge_all_title') }}</h3>
          <p class="mb-4 text-sm text-gray-400">{{ t('cloudflare_manager.cache.purge_all_desc') }}</p>
          <button class="rounded-lg bg-red-600/20 px-6 py-2.5 font-medium text-red-400 transition hover:bg-red-600/40" @click="purgeAllCache">
            {{ t('cloudflare_manager.cache.purge_all') }}
          </button>
        </div>
        <div class="rounded-xl border border-panel-border bg-panel-card p-6">
          <h3 class="mb-3 font-semibold text-white">{{ t('cloudflare_manager.cache.purge_urls_title') }}</h3>
          <textarea
            v-model="purgeUrls"
            rows="4"
            class="w-full rounded-lg border border-panel-border bg-panel-hover px-4 py-2.5 font-mono text-sm text-white placeholder-gray-500 focus:border-orange-500 focus:outline-none"
            :placeholder="t('cloudflare_manager.cache.purge_urls_placeholder')"
          ></textarea>
          <button class="mt-3 rounded-lg bg-orange-600/20 px-5 py-2 text-sm text-orange-400 transition hover:bg-orange-600/40" @click="purgeSpecificCache">
            {{ t('cloudflare_manager.cache.purge_urls') }}
          </button>
        </div>
      </div>

      <div v-if="activeTab === 'security'" class="grid grid-cols-1 gap-4 md:grid-cols-2">
        <div class="rounded-xl border border-panel-border bg-panel-card p-5">
          <h3 class="mb-4 font-semibold text-white">{{ t('cloudflare_manager.security.level_title') }}</h3>
          <div class="space-y-2">
            <button
              v-for="level in securityLevels"
              :key="level.value"
              class="w-full rounded-lg px-4 py-3 text-left text-sm transition"
              :class="selectedSecurityLevel === level.value ? 'border border-orange-500/30 bg-orange-500/20 text-orange-400' : 'bg-panel-hover text-gray-300 hover:bg-gray-600'"
              @click="setSecurityLevel(level.value)"
            >
              {{ level.label }}
            </button>
          </div>
        </div>
        <div class="space-y-4">
          <div class="rounded-xl border border-panel-border bg-panel-card p-5">
            <h3 class="mb-3 font-semibold text-white">{{ t('cloudflare_manager.security.attack_title') }}</h3>
            <p class="mb-3 text-sm text-gray-400">{{ t('cloudflare_manager.security.attack_desc') }}</p>
            <button
              class="rounded-lg px-5 py-2.5 text-sm font-medium transition"
              :class="selectedSecurityLevel === 'under_attack' ? 'animate-pulse bg-red-600 text-white' : 'bg-red-600/20 text-red-400 hover:bg-red-600/40'"
              @click="setSecurityLevel('under_attack')"
            >
              {{ t('cloudflare_manager.security.attack_button') }}
            </button>
          </div>
          <div class="rounded-xl border border-panel-border bg-panel-card p-5">
            <h3 class="mb-3 font-semibold text-white">{{ t('cloudflare_manager.security.dev_title') }}</h3>
            <p class="mb-3 text-sm text-gray-400">{{ t('cloudflare_manager.security.dev_desc') }}</p>
            <button class="rounded-lg px-5 py-2.5 text-sm transition" :class="devMode ? 'bg-yellow-600 text-white' : 'bg-panel-hover text-gray-400'" @click="toggleDevMode">
              {{ devMode ? t('cloudflare_manager.ssl.enabled') : t('cloudflare_manager.ssl.disabled') }}
            </button>
          </div>
        </div>
      </div>

      <div v-if="activeTab === 'analytics'" class="space-y-4">
        <div class="rounded-xl border border-panel-border bg-panel-card p-5">
          <div class="flex flex-wrap items-center gap-3">
            <label class="text-sm text-gray-400">Zone</label>
            <select
              v-model="analyticsZoneId"
              class="min-w-[220px] rounded-lg border border-panel-border bg-panel-hover px-3 py-2 text-sm text-white focus:border-orange-500 focus:outline-none"
            >
              <option value="" disabled>Select zone</option>
              <option v-for="zone in zones" :key="zone.id" :value="zone.id">{{ zone.name }}</option>
            </select>
            <button
              class="rounded-lg bg-gradient-to-r from-orange-600 to-amber-600 px-4 py-2 text-sm font-medium text-white transition hover:from-orange-700 hover:to-amber-700"
              :disabled="analyticsLoading || !analyticsZoneId"
              @click="loadAnalytics"
            >
              {{ analyticsLoading ? 'Loading...' : 'Load Analytics' }}
            </button>
          </div>
        </div>

        <div v-if="analyticsError" class="rounded-xl border border-red-500/30 bg-red-500/10 p-4 text-sm text-red-300">
          {{ analyticsError }}
        </div>

        <div class="grid grid-cols-1 gap-4 md:grid-cols-3">
          <div class="rounded-xl border border-panel-border bg-panel-card p-5">
            <p class="text-xs uppercase tracking-wide text-gray-400">Requests</p>
            <p class="mt-2 text-2xl font-semibold text-white">{{ analyticsSummary.requests }}</p>
          </div>
          <div class="rounded-xl border border-panel-border bg-panel-card p-5">
            <p class="text-xs uppercase tracking-wide text-gray-400">Page Views</p>
            <p class="mt-2 text-2xl font-semibold text-white">{{ analyticsSummary.pageViews }}</p>
          </div>
          <div class="rounded-xl border border-panel-border bg-panel-card p-5">
            <p class="text-xs uppercase tracking-wide text-gray-400">Bandwidth</p>
            <p class="mt-2 text-2xl font-semibold text-white">{{ analyticsSummary.bandwidth }}</p>
          </div>
        </div>

        <div class="rounded-xl border border-panel-border bg-panel-card p-5">
          <h3 class="mb-3 text-sm font-semibold text-white">Raw Analytics Response</h3>
          <pre class="max-h-[360px] overflow-auto rounded-lg bg-panel-darker p-3 text-xs text-gray-300">{{ formattedAnalytics }}</pre>
        </div>
      </div>
    </template>

    <div v-if="showAddDns" class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm" @click.self="showAddDns = false">
      <div class="w-full max-w-lg rounded-2xl border border-panel-border bg-panel-card p-6 shadow-2xl">
        <h3 class="mb-5 text-xl font-bold text-white">{{ t('cloudflare_manager.dns.modal_title') }}</h3>
        <div class="space-y-4">
          <div>
            <label class="mb-1 block text-sm text-gray-400">{{ t('cloudflare_manager.dns.type') }}</label>
            <select v-model="newDns.type" class="aura-input">
              <option v-for="type in ['A', 'AAAA', 'CNAME', 'MX', 'TXT', 'NS', 'SRV', 'CAA']" :key="type" :value="type">{{ type }}</option>
            </select>
          </div>
          <div>
            <label class="mb-1 block text-sm text-gray-400">{{ t('cloudflare_manager.dns.name') }}</label>
            <input v-model="newDns.name" type="text" placeholder="@, www, mail" class="aura-input" />
          </div>
          <div>
            <label class="mb-1 block text-sm text-gray-400">{{ t('cloudflare_manager.dns.value') }}</label>
            <input v-model="newDns.content" type="text" placeholder="93.184.216.34" class="aura-input" />
          </div>
          <div class="flex items-center gap-4">
            <label class="flex items-center gap-2 text-sm text-gray-300">
              <input v-model="newDns.proxied" type="checkbox" class="accent-orange-500" />
              {{ t('cloudflare_manager.dns.proxied') }}
            </label>
            <select v-model="newDns.ttl" class="rounded-lg border border-panel-border bg-panel-hover px-3 py-2 text-sm text-white focus:outline-none">
              <option :value="1">{{ t('cloudflare_manager.dns.ttl_auto') }}</option>
              <option :value="300">{{ t('cloudflare_manager.dns.ttl_5m') }}</option>
              <option :value="3600">{{ t('cloudflare_manager.dns.ttl_1h') }}</option>
              <option :value="86400">{{ t('cloudflare_manager.dns.ttl_1d') }}</option>
            </select>
          </div>
        </div>
        <div class="mt-6 flex gap-3">
          <button class="flex-1 rounded-lg bg-gradient-to-r from-orange-600 to-amber-600 py-2.5 font-medium text-white transition hover:from-orange-700 hover:to-amber-700" @click="addDnsRecord">
            {{ t('common.add') }}
          </button>
          <button class="rounded-lg bg-panel-hover px-5 py-2.5 text-gray-300 transition hover:bg-gray-600" @click="showAddDns = false">
            {{ t('common.cancel') }}
          </button>
        </div>
      </div>
    </div>

    <div v-if="notification" :class="['fixed bottom-6 right-6 z-50 rounded-xl px-5 py-3 text-sm font-medium text-white shadow-2xl', notification.type === 'success' ? 'bg-green-600' : 'bg-red-600']">
      {{ notification.message }}
    </div>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { Cloud, Trash2 } from 'lucide-vue-next'
import api from '../services/api'

const { t } = useI18n({ useScope: 'global' })

const cfEmail = ref('')
const cfApiKey = ref('')
const connected = ref(false)
const notification = ref(null)
const activeTab = ref('zones')
const loading = ref(false)
const cfStorageKey = 'aurapanel.cloudflare.auth.v1'
const serverStatus = ref({
  configured: false,
  auto_sync: false,
  credential_source: 'none',
  email_hint: '',
})
const serverSaving = ref(false)

const tabs = [
  { id: 'zones', label: t('cloudflare_manager.tabs.zones') },
  { id: 'dns', label: t('cloudflare_manager.tabs.dns') },
  { id: 'ssl', label: t('cloudflare_manager.tabs.ssl') },
  { id: 'cache', label: t('cloudflare_manager.tabs.cache') },
  { id: 'security', label: t('cloudflare_manager.tabs.security') },
  { id: 'analytics', label: 'Analytics' },
]

const zones = ref([])
const selectedZone = ref(null)
const dnsRecords = ref([])
const showAddDns = ref(false)
const newDns = reactive({ type: 'A', name: '', content: '', proxied: false, ttl: 1 })
const selectedSslMode = ref('full')
const selectedSecurityLevel = ref('medium')
const devMode = ref(false)
const alwaysHttps = ref(true)
const minifyOptions = reactive({ js: true, css: true, html: false })
const purgeUrls = ref('')
const analyticsZoneId = ref('')
const analyticsLoading = ref(false)
const analyticsData = ref(null)
const analyticsError = ref('')

const sslModes = [
  { value: 'off', label: t('cloudflare_manager.ssl.modes.off.label'), desc: t('cloudflare_manager.ssl.modes.off.desc') },
  { value: 'flexible', label: t('cloudflare_manager.ssl.modes.flexible.label'), desc: t('cloudflare_manager.ssl.modes.flexible.desc') },
  { value: 'full', label: t('cloudflare_manager.ssl.modes.full.label'), desc: t('cloudflare_manager.ssl.modes.full.desc') },
  { value: 'strict', label: t('cloudflare_manager.ssl.modes.strict.label'), desc: t('cloudflare_manager.ssl.modes.strict.desc') },
]

const securityLevels = [
  { value: 'off', label: t('cloudflare_manager.security.levels.off') },
  { value: 'low', label: t('cloudflare_manager.security.levels.low') },
  { value: 'medium', label: t('cloudflare_manager.security.levels.medium') },
  { value: 'high', label: t('cloudflare_manager.security.levels.high') },
  { value: 'under_attack', label: t('cloudflare_manager.security.levels.under_attack') },
]

const showNotif = (message, type = 'success') => {
  notification.value = { message, type }
  setTimeout(() => {
    notification.value = null
  }, 3000)
}

const readStoredCfAuth = () => {
  if (typeof window === 'undefined') return null
  try {
    const raw = window.localStorage.getItem(cfStorageKey)
    if (!raw) return null
    const parsed = JSON.parse(raw)
    if (!parsed || typeof parsed !== 'object') return null
    return parsed
  } catch {
    return null
  }
}

const saveStoredCfAuth = () => {
  if (typeof window === 'undefined') return
  if (!cfEmail.value || !cfApiKey.value) return
  const payload = {
    email: cfEmail.value,
    api_key: cfApiKey.value,
    zone_id: selectedZone.value?.id || '',
  }
  window.localStorage.setItem(cfStorageKey, JSON.stringify(payload))
}

const clearStoredCfAuth = () => {
  if (typeof window === 'undefined') return
  window.localStorage.removeItem(cfStorageKey)
}

const authPayload = () => {
  const payload = {}
  if (cfApiKey.value) payload.api_key = cfApiKey.value
  if (cfEmail.value) payload.email = cfEmail.value
  return payload
}

const loadServerStatus = async () => {
  try {
    const { data } = await api.get('/cloudflare/status')
    serverStatus.value = {
      configured: Boolean(data?.data?.configured),
      auto_sync: Boolean(data?.data?.auto_sync),
      credential_source: data?.data?.credential_source || 'none',
      email_hint: data?.data?.email_hint || '',
    }
  } catch {
    serverStatus.value = { configured: false, auto_sync: false, credential_source: 'none', email_hint: '' }
  }
}

const connectCf = async (opts = {}) => {
  const { silent = false, auto = false, preferredZoneId = '' } = opts
  const hasManualCreds = Boolean(cfEmail.value && cfApiKey.value)
  if (!hasManualCreds && !serverStatus.value.configured) {
    if (auto) return
    showNotif(t('cloudflare_manager.messages.credentials_required'), 'error')
    return
  }
  loading.value = true
  try {
    const { data } = await api.post('/cloudflare/zones', authPayload())
    zones.value = data.data || []
    connected.value = true
    const preferred = preferredZoneId || readStoredCfAuth()?.zone_id || ''
    const matchedZone = zones.value.find(zone => zone.id === preferred) || zones.value[0] || null
    selectedZone.value = matchedZone
    saveStoredCfAuth()
    if (!silent) {
      showNotif(t('cloudflare_manager.messages.connected', { count: zones.value.length }))
    }
  } catch (err) {
    if (auto) {
      connected.value = false
      clearStoredCfAuth()
      return
    }
    showNotif(err.response?.data?.error || t('cloudflare_manager.messages.connect_failed'), 'error')
  } finally {
    loading.value = false
  }
}

const saveServerAuth = async () => {
  if (!cfEmail.value || !cfApiKey.value) {
    showNotif(t('cloudflare_manager.messages.credentials_required'), 'error')
    return
  }
  serverSaving.value = true
  try {
    const { data } = await api.post('/cloudflare/server-auth', {
      email: cfEmail.value,
      api_key: cfApiKey.value,
      auto_sync: true,
    })
    serverStatus.value = {
      configured: Boolean(data?.data?.configured),
      auto_sync: Boolean(data?.data?.auto_sync),
      credential_source: data?.data?.credential_source || 'none',
      email_hint: data?.data?.email_hint || '',
    }
    showNotif(t('cloudflare_manager.messages.server_auth_saved'))
    if (!connected.value) {
      await connectCf({ silent: true })
    }
  } catch (err) {
    showNotif(err.response?.data?.message || t('cloudflare_manager.messages.server_auth_failed'), 'error')
  } finally {
    serverSaving.value = false
  }
}

const loadZones = connectCf

const selectZone = async zone => {
  selectedZone.value = zone
  saveStoredCfAuth()
  analyticsZoneId.value = zone?.id || ''
  activeTab.value = 'dns'
  dnsRecords.value = []
  try {
    const { data } = await api.post('/cloudflare/dns/list', { ...authPayload(), zone_id: zone.id })
    dnsRecords.value = data.data || []
  } catch (err) {
    showNotif(err.response?.data?.error || t('cloudflare_manager.messages.dns_failed'), 'error')
  }
}

const getNestedNumber = (obj, paths) => {
  for (const path of paths) {
    const value = path.reduce((acc, key) => (acc && acc[key] !== undefined ? acc[key] : undefined), obj)
    if (typeof value === 'number' && Number.isFinite(value)) {
      return value
    }
  }
  return 0
}

const formatNumber = value => new Intl.NumberFormat().format(Number(value || 0))

const formatBytes = value => {
  const num = Number(value || 0)
  if (!Number.isFinite(num) || num <= 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB', 'PB']
  let size = num
  let idx = 0
  while (size >= 1024 && idx < units.length - 1) {
    size /= 1024
    idx += 1
  }
  const rounded = idx === 0 ? size.toFixed(0) : size.toFixed(2)
  return `${rounded} ${units[idx]}`
}

const analyticsSummary = computed(() => {
  const raw = analyticsData.value || {}
  const requests = getNestedNumber(raw, [
    ['result', 'totals', 'requests', 'all'],
    ['result', 'totals', 'requests'],
    ['result', 'totals', 'visits'],
  ])
  const pageViews = getNestedNumber(raw, [
    ['result', 'totals', 'pageviews', 'all'],
    ['result', 'totals', 'pageviews'],
    ['result', 'totals', 'uniques', 'all'],
  ])
  const bandwidthBytes = getNestedNumber(raw, [
    ['result', 'totals', 'bandwidth', 'all'],
    ['result', 'totals', 'bandwidth'],
  ])
  return {
    requests: formatNumber(requests),
    pageViews: formatNumber(pageViews),
    bandwidth: formatBytes(bandwidthBytes),
  }
})

const formattedAnalytics = computed(() => JSON.stringify(analyticsData.value || {}, null, 2))

const loadAnalytics = async () => {
  if (!analyticsZoneId.value) {
    showNotif('Please select a zone first.', 'error')
    return
  }
  analyticsLoading.value = true
  analyticsError.value = ''
  try {
    const { data } = await api.post('/cloudflare/analytics', {
      ...authPayload(),
      zone_id: analyticsZoneId.value,
    })
    analyticsData.value = data.data || {}
    showNotif('Cloudflare analytics loaded.')
  } catch (err) {
    analyticsError.value = err.response?.data?.message || err.message || 'Analytics request failed.'
    showNotif(analyticsError.value, 'error')
  } finally {
    analyticsLoading.value = false
  }
}

const addDnsRecord = async () => {
  try {
    await api.post('/cloudflare/dns/create', { ...authPayload(), zone_id: selectedZone.value.id, ...newDns })
    showNotif(t('cloudflare_manager.messages.dns_added', { type: newDns.type }))
    showAddDns.value = false
    selectZone(selectedZone.value)
  } catch (err) {
    showNotif(err.response?.data?.error || t('cloudflare_manager.messages.dns_add_failed'), 'error')
  }
}

const deleteDnsRecord = async recordId => {
  if (!window.confirm(t('cloudflare_manager.dns.delete_confirm'))) return
  try {
    await api.post('/cloudflare/dns/delete', { ...authPayload(), zone_id: selectedZone.value.id, record_id: recordId })
    showNotif(t('cloudflare_manager.messages.dns_deleted'))
    selectZone(selectedZone.value)
  } catch (err) {
    showNotif(err.response?.data?.error || t('cloudflare_manager.messages.dns_delete_failed'), 'error')
  }
}

const setSslMode = async mode => {
  selectedSslMode.value = mode
  try {
    await api.post('/cloudflare/ssl', { ...authPayload(), zone_id: selectedZone.value?.id || zones.value[0]?.id, mode })
    showNotif(t('cloudflare_manager.messages.ssl_updated', { mode }))
  } catch (err) {
    showNotif(err.response?.data?.error || t('cloudflare_manager.messages.ssl_update_failed'), 'error')
  }
}

const purgeAllCache = async () => {
  try {
    await api.post('/cloudflare/cache/purge', { ...authPayload(), zone_id: selectedZone.value?.id || zones.value[0]?.id, purge_everything: true })
    showNotif(t('cloudflare_manager.messages.cache_purged'))
  } catch (err) {
    showNotif(err.response?.data?.error || t('cloudflare_manager.messages.cache_failed'), 'error')
  }
}

const purgeSpecificCache = async () => {
  const files = purgeUrls.value.split('\n').filter(item => item.trim())
  if (!files.length) {
    showNotif(t('cloudflare_manager.messages.cache_url_required'), 'error')
    return
  }
  try {
    await api.post('/cloudflare/cache/purge', { ...authPayload(), zone_id: selectedZone.value?.id || zones.value[0]?.id, files })
    showNotif(t('cloudflare_manager.messages.cache_urls_purged', { count: files.length }))
    purgeUrls.value = ''
  } catch (err) {
    showNotif(err.response?.data?.error || t('cloudflare_manager.messages.cache_failed'), 'error')
  }
}

const setSecurityLevel = async level => {
  selectedSecurityLevel.value = level
  try {
    await api.post('/cloudflare/security', { ...authPayload(), zone_id: selectedZone.value?.id || zones.value[0]?.id, level })
    showNotif(t('cloudflare_manager.messages.security_updated'))
  } catch (err) {
    showNotif(err.response?.data?.error || t('cloudflare_manager.messages.security_failed'), 'error')
  }
}

const toggleDevMode = async () => {
  const newValue = !devMode.value
  try {
    await api.post('/cloudflare/devmode', { ...authPayload(), zone_id: selectedZone.value?.id || zones.value[0]?.id, enabled: newValue })
    devMode.value = newValue
    showNotif(t('cloudflare_manager.messages.dev_mode_updated', { state: devMode.value ? t('cloudflare_manager.ssl.enabled') : t('cloudflare_manager.ssl.disabled') }))
  } catch (err) {
    showNotif(err.response?.data?.error || t('cloudflare_manager.messages.dev_mode_failed'), 'error')
  }
}

const toggleAlwaysHttps = () => {
  alwaysHttps.value = !alwaysHttps.value
  showNotif(t('cloudflare_manager.messages.always_https_updated', { state: alwaysHttps.value ? t('cloudflare_manager.ssl.enabled') : t('cloudflare_manager.ssl.disabled') }))
}

const saveMinify = () => showNotif(t('cloudflare_manager.messages.minify_saved'))

const dnsTypeBadge = type => {
  const map = {
    A: 'bg-blue-500/15 text-blue-400',
    AAAA: 'bg-indigo-500/15 text-indigo-400',
    CNAME: 'bg-green-500/15 text-green-400',
    MX: 'bg-purple-500/15 text-purple-400',
    TXT: 'bg-yellow-500/15 text-yellow-400',
    NS: 'bg-pink-500/15 text-pink-400',
    SRV: 'bg-cyan-500/15 text-cyan-400',
  }
  return map[type] || 'bg-gray-500/15 text-gray-400'
}

onMounted(async () => {
  await loadServerStatus()
  const saved = readStoredCfAuth()
  if (saved) {
    cfEmail.value = saved.email || ''
    cfApiKey.value = saved.api_key || ''
    if (cfEmail.value && cfApiKey.value) {
      await connectCf({ silent: true, auto: true, preferredZoneId: saved.zone_id || '' })
      return
    }
  }
  if (serverStatus.value.configured) {
    await connectCf({ silent: true, auto: true })
  }
})
</script>

<style scoped>
.cloudflare-view .cf-connect-title,
.cloudflare-view .cf-field-label {
  color: rgb(253 186 116);
}

.cloudflare-view :is(input[type='text'], input[type='email'], input[type='password'], input[type='number'], input[type='search'], input[type='url'], textarea, select) {
  background-color: rgb(15 23 42) !important;
  color: rgb(253 186 116) !important;
  border-color: rgb(249 115 22 / 0.45) !important;
  caret-color: rgb(251 146 60);
}

.cloudflare-view :is(input[type='text'], input[type='email'], input[type='password'], input[type='number'], input[type='search'], input[type='url'], textarea)::placeholder {
  color: rgb(251 146 60 / 0.65) !important;
}
</style>
