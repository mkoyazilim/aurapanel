<template>
  <div class="space-y-5">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <div>
        <h1 class="text-2xl font-bold text-white">Website Manage</h1>
        <p class="text-gray-400 text-sm mt-1">{{ domain }}</p>
      </div>
      <div class="flex gap-2">
        <button class="btn-secondary" @click="goBack">Geri</button>
        <button class="btn-secondary" @click="refreshAll">Yenile</button>
      </div>
    </div>

    <div v-if="error" class="aura-card border-red-500/30 bg-red-500/5 text-red-400">{{ error }}</div>

    <div class="aura-card space-y-4">
      <div class="flex flex-wrap items-center gap-2">
        <span class="text-sm text-gray-400">Durum:</span>
        <span :class="isSuspended ? 'text-yellow-400' : 'text-brand-400'">{{ isSuspended ? 'suspended' : 'active' }}</span>
        <span class="text-gray-500">|</span>
        <span class="text-sm text-gray-400">SSL:</span>
        <span :class="site.ssl ? 'text-brand-400' : 'text-yellow-400'">{{ site.ssl ? 'aktif' : 'yok' }}</span>
      </div>
      <div class="grid grid-cols-1 md:grid-cols-2 gap-3">
        <input v-model="form.owner" class="aura-input" placeholder="Owner" />
        <select v-model="form.php_version" class="aura-input">
          <option v-for="v in phpVersions" :key="v" :value="v">PHP {{ v }}</option>
        </select>
        <input v-model="form.package" class="aura-input" placeholder="Package" />
        <input v-model="form.email" class="aura-input" placeholder="Admin Email" />
      </div>
      <div class="flex flex-wrap gap-2">
        <button class="btn-primary" @click="saveWebsite">Kaydet</button>
        <button class="btn-secondary" @click="toggleSuspend">{{ isSuspended ? 'Unsuspend' : 'Suspend' }}</button>
        <button class="btn-secondary" @click="issueSsl">SSL Issue</button>
      </div>
    </div>

    <div class="grid grid-cols-1 lg:grid-cols-2 gap-4">
      <div class="aura-card space-y-3">
        <h3 class="text-white font-semibold">Alias</h3>
        <div class="flex gap-2">
          <input v-model="aliasInput" class="aura-input flex-1" placeholder="alias.example.com" />
          <button class="btn-primary" @click="addAlias">Ekle</button>
        </div>
        <div class="space-y-2 max-h-40 overflow-auto">
          <div v-for="a in aliases" :key="a.alias" class="flex items-center justify-between rounded-lg border border-panel-border px-3 py-2 text-sm">
            <span class="text-gray-200">{{ a.alias }}</span>
            <button class="btn-danger px-2 py-1" @click="deleteAlias(a.alias)">Sil</button>
          </div>
        </div>
      </div>

      <div class="aura-card space-y-3">
        <h3 class="text-white font-semibold">OpenBasedir</h3>
        <label class="inline-flex items-center gap-2 text-sm text-gray-300">
          <input v-model="advanced.open_basedir" type="checkbox" class="w-4 h-4" />
          Etkin
        </label>
        <button class="btn-primary" @click="saveOpenBasedir">Kaydet</button>
      </div>

      <div class="aura-card lg:col-span-2 space-y-3">
        <h3 class="text-white font-semibold">Rewrite</h3>
        <textarea v-model="advanced.rewrite_rules" rows="7" class="aura-input w-full font-mono text-xs"></textarea>
        <button class="btn-primary" @click="saveRewrite">Kaydet</button>
      </div>

      <div class="aura-card lg:col-span-2 space-y-3">
        <h3 class="text-white font-semibold">VHost Config</h3>
        <textarea v-model="advanced.vhost_config" rows="10" class="aura-input w-full font-mono text-xs"></textarea>
        <button class="btn-primary" @click="saveVhost">Kaydet</button>
      </div>

      <div class="aura-card lg:col-span-2 space-y-3">
        <h3 class="text-white font-semibold">Custom SSL</h3>
        <textarea v-model="customSsl.cert_pem" rows="6" class="aura-input w-full font-mono text-xs" placeholder="-----BEGIN CERTIFICATE-----"></textarea>
        <textarea v-model="customSsl.key_pem" rows="6" class="aura-input w-full font-mono text-xs" placeholder="-----BEGIN PRIVATE KEY-----"></textarea>
        <button class="btn-primary" @click="saveCustomSsl">Kaydet</button>
      </div>
    </div>

    <div class="aura-card space-y-3">
      <div class="flex gap-2">
        <button class="btn-secondary" :class="logKind === 'access' ? 'border-brand-500 text-brand-300' : ''" @click="changeLogKind('access')">Access</button>
        <button class="btn-secondary" :class="logKind === 'error' ? 'border-brand-500 text-brand-300' : ''" @click="changeLogKind('error')">Error</button>
        <button class="btn-secondary ml-auto" @click="loadLogs">Yenile</button>
      </div>
      <pre class="rounded-lg border border-panel-border bg-panel-dark p-3 text-xs text-gray-200 max-h-[360px] overflow-auto whitespace-pre-wrap">{{ logs.join('\n') || 'Log bulunamadi.' }}</pre>
    </div>
  </div>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import api from '../services/api'

const route = useRoute()
const router = useRouter()

const domain = computed(() => String(route.params.domain || '').toLowerCase())
const phpVersions = ['8.4', '8.3', '8.2', '8.1', '8.0', '7.4']

const error = ref('')
const site = ref({})
const form = ref({ owner: 'aura', php_version: '8.3', package: 'default', email: '' })
const aliases = ref([])
const aliasInput = ref('')
const advanced = ref({ open_basedir: false, rewrite_rules: '', vhost_config: '' })
const customSsl = ref({ cert_pem: '', key_pem: '' })
const logKind = ref('access')
const logs = ref([])

const isSuspended = computed(() => String(site.value?.status || 'active').toLowerCase() === 'suspended')

function msg(err, fallback) {
  return err?.response?.data?.message || err?.message || fallback
}

async function loadSite() {
  const res = await api.get('/vhost/list', { params: { search: domain.value, page: 1, per_page: 100 } })
  const data = res.data?.data || []
  site.value = data.find(x => String(x.domain || '').toLowerCase() === domain.value) || {}
  form.value = {
    owner: site.value.owner || site.value.user || 'aura',
    php_version: site.value.php_version || site.value.php || '8.3',
    package: site.value.package || 'default',
    email: site.value.email || `webmaster@${domain.value}`,
  }
}

async function loadAliases() {
  const res = await api.get('/websites/aliases', { params: { domain: domain.value } })
  aliases.value = res.data?.data || []
}

async function loadAdvanced() {
  const [cfgRes, sslRes] = await Promise.all([
    api.get('/websites/advanced-config', { params: { domain: domain.value } }),
    api.get('/websites/custom-ssl', { params: { domain: domain.value } }),
  ])
  const cfg = cfgRes.data?.data || {}
  advanced.value = {
    open_basedir: !!cfg.open_basedir,
    rewrite_rules: cfg.rewrite_rules || '',
    vhost_config: cfg.vhost_config || '',
  }
  const ssl = sslRes.data?.data || {}
  customSsl.value = {
    cert_pem: ssl.cert_pem || '',
    key_pem: ssl.key_pem || '',
  }
}

async function loadLogs() {
  const res = await api.get('/monitor/logs/site', { params: { domain: domain.value, kind: logKind.value, lines: 200 } })
  logs.value = res.data?.data || []
}

async function refreshAll() {
  error.value = ''
  try {
    await Promise.all([loadSite(), loadAliases(), loadAdvanced(), loadLogs()])
  } catch (e) {
    error.value = msg(e, 'Website verileri alinamadi')
  }
}

async function saveWebsite() {
  error.value = ''
  try {
    await api.post('/vhost/update', {
      domain: domain.value,
      owner: form.value.owner,
      php_version: form.value.php_version,
      package: form.value.package,
      email: form.value.email,
    })
    await loadSite()
  } catch (e) {
    error.value = msg(e, 'Website kaydedilemedi')
  }
}

async function toggleSuspend() {
  error.value = ''
  try {
    await api.post(isSuspended.value ? '/vhost/unsuspend' : '/vhost/suspend', { domain: domain.value })
    await loadSite()
  } catch (e) {
    error.value = msg(e, 'Website durumu guncellenemedi')
  }
}

async function issueSsl() {
  error.value = ''
  try {
    await api.post('/ssl/issue', { domain: domain.value, email: form.value.email || `admin@${domain.value}`, provider: 'letsencrypt' })
    await loadSite()
  } catch (e) {
    error.value = msg(e, 'SSL islemi basarisiz')
  }
}

async function addAlias() {
  if (!aliasInput.value) return
  error.value = ''
  try {
    await api.post('/websites/aliases', { domain: domain.value, alias: aliasInput.value })
    aliasInput.value = ''
    await loadAliases()
  } catch (e) {
    error.value = msg(e, 'Alias eklenemedi')
  }
}

async function deleteAlias(alias) {
  error.value = ''
  try {
    await api.delete('/websites/aliases', { params: { domain: domain.value, alias } })
    await loadAliases()
  } catch (e) {
    error.value = msg(e, 'Alias silinemedi')
  }
}

async function saveOpenBasedir() {
  error.value = ''
  try {
    await api.post('/websites/open-basedir', { domain: domain.value, enabled: !!advanced.value.open_basedir })
    await loadAdvanced()
  } catch (e) {
    error.value = msg(e, 'OpenBasedir kaydedilemedi')
  }
}

async function saveRewrite() {
  error.value = ''
  try {
    await api.post('/websites/rewrite', { domain: domain.value, rules: advanced.value.rewrite_rules || '' })
    await loadAdvanced()
  } catch (e) {
    error.value = msg(e, 'Rewrite kaydedilemedi')
  }
}

async function saveVhost() {
  error.value = ''
  try {
    await api.post('/websites/vhost-config', { domain: domain.value, content: advanced.value.vhost_config || '' })
    await loadAdvanced()
  } catch (e) {
    error.value = msg(e, 'VHost config kaydedilemedi')
  }
}

async function saveCustomSsl() {
  error.value = ''
  try {
    await api.post('/websites/custom-ssl', {
      domain: domain.value,
      cert_pem: customSsl.value.cert_pem || '',
      key_pem: customSsl.value.key_pem || '',
    })
    await loadAdvanced()
  } catch (e) {
    error.value = msg(e, 'Custom SSL kaydedilemedi')
  }
}

async function changeLogKind(kind) {
  logKind.value = kind
  await loadLogs()
}

function goBack() {
  router.push('/websites')
}

onMounted(refreshAll)
</script>
