<template>
  <div class="space-y-6">
    <div class="flex items-center justify-between gap-3">
      <div>
        <h1 class="text-2xl font-bold text-white">{{ t('ssl_manager.title') }}</h1>
        <p class="mt-1 text-gray-400">{{ t('ssl_manager.subtitle') }}</p>
      </div>
      <button class="btn-secondary" @click="refreshAll">{{ t('ssl_manager.refresh') }}</button>
    </div>

    <div class="aura-card space-y-3">
      <p class="text-xs uppercase tracking-wide text-gray-400">{{ t('ssl_manager.binding_status') }}</p>
      <div class="grid grid-cols-1 gap-3 text-sm md:grid-cols-2">
        <div class="rounded-lg border border-panel-border bg-panel-hover/30 p-3">
          <p class="text-gray-400">{{ t('ssl_manager.hostname_ssl') }}</p>
          <p class="mt-1 font-mono text-white">{{ bindings.hostname_ssl_domain || '-' }}</p>
        </div>
        <div class="rounded-lg border border-panel-border bg-panel-hover/30 p-3">
          <p class="text-gray-400">{{ t('ssl_manager.mail_ssl') }}</p>
          <p class="mt-1 font-mono text-white">{{ bindings.mail_ssl_domain || '-' }}</p>
        </div>
      </div>
      <p v-if="bindings.updated_at" class="text-xs text-gray-500">
        {{ t('ssl_manager.last_updated', { value: formatTime(bindings.updated_at * 1000) }) }}
      </p>
    </div>

    <div class="border-b border-panel-border">
      <nav class="flex gap-6">
        <button @click="setTab('manage')" :class="tabClass('manage')">{{ t('ssl_manager.tabs.manage') }}</button>
        <button @click="setTab('hostname')" :class="tabClass('hostname')">{{ t('ssl_manager.tabs.hostname') }}</button>
        <button @click="setTab('mail')" :class="tabClass('mail')">{{ t('ssl_manager.tabs.mail') }}</button>
        <button @click="setTab('wildcard')" :class="tabClass('wildcard')">{{ t('ssl_manager.tabs.wildcard') }}</button>
      </nav>
    </div>

    <div v-if="error" class="aura-card border-red-500/30 bg-red-500/5 text-red-400">{{ error }}</div>
    <div v-if="success" class="aura-card border-green-500/30 bg-green-500/5 text-green-300">{{ success }}</div>

    <div v-if="tab === 'manage'" class="aura-card space-y-4">
      <div class="grid grid-cols-1 gap-3 lg:grid-cols-3">
        <div>
          <label class="mb-1 block text-sm text-gray-400">{{ t('ssl_manager.manage.website_domain') }}</label>
          <select v-model="manageForm.domain" class="aura-input w-full" @change="loadCertificateDetails">
            <option value="" disabled>{{ t('ssl_manager.placeholders.select_domain') }}</option>
            <option v-for="domainName in domains" :key="domainName" :value="domainName">{{ domainName }}</option>
          </select>
        </div>
        <div>
          <label class="mb-1 block text-sm text-gray-400">{{ t('ssl_manager.manage.admin_email') }}</label>
          <input v-model="manageForm.email" type="email" class="aura-input w-full" :placeholder="t('ssl_manager.placeholders.email')" />
        </div>
        <div>
          <label class="mb-1 block text-sm text-gray-400">{{ t('ssl_manager.manage.webroot') }}</label>
          <input v-model="manageForm.webroot" class="aura-input w-full" :placeholder="t('ssl_manager.placeholders.webroot')" />
        </div>
      </div>

      <div class="flex flex-wrap gap-2">
        <button class="btn-secondary" :disabled="!manageForm.domain || loadingDetails" @click="loadCertificateDetails">
          {{ loadingDetails ? t('ssl_manager.manage.loading_details') : t('ssl_manager.manage.load_details') }}
        </button>
        <button class="btn-primary" :disabled="!manageForm.domain || !manageForm.email || issuingManage" @click="issueManageSsl">
          {{ issuingManage ? t('ssl_manager.manage.issuing') : t('ssl_manager.manage.issue') }}
        </button>
      </div>

      <div v-if="details" class="rounded-xl border border-panel-border bg-panel-hover/20 p-4">
        <div class="grid grid-cols-1 gap-3 text-sm md:grid-cols-2">
          <div>
            <p class="text-gray-400">{{ t('ssl_manager.details.status') }}</p>
            <p class="mt-1 text-white">{{ details.status }}</p>
          </div>
          <div>
            <p class="text-gray-400">{{ t('ssl_manager.details.issuer') }}</p>
            <p class="mt-1 text-white">{{ details.issuer || '-' }}</p>
          </div>
          <div>
            <p class="text-gray-400">{{ t('ssl_manager.details.expiry') }}</p>
            <p class="mt-1 text-white">{{ details.expiry_date || '-' }}</p>
          </div>
          <div>
            <p class="text-gray-400">{{ t('ssl_manager.details.days_remaining') }}</p>
            <p class="mt-1 text-white">{{ details.days_remaining ?? '-' }}</p>
          </div>
        </div>
      </div>
    </div>

    <div v-if="tab === 'hostname'" class="aura-card space-y-4">
      <p class="rounded-lg border border-amber-500/30 bg-amber-500/10 px-3 py-2 text-sm text-amber-300">
        {{ t('ssl_manager.hostname.notice') }}
      </p>
      <div class="grid grid-cols-1 gap-3 lg:grid-cols-3">
        <div>
          <label class="mb-1 block text-sm text-gray-400">{{ t('ssl_manager.hostname.hostname') }}</label>
          <input v-model="hostnameForm.domain" list="ssl-domain-options" class="aura-input w-full" :placeholder="t('ssl_manager.placeholders.hostname')" />
          <datalist id="ssl-domain-options">
            <option v-for="domainName in domainSuggestions" :key="`ssl-domain-${domainName}`" :value="domainName" />
          </datalist>
        </div>
        <div>
          <label class="mb-1 block text-sm text-gray-400">{{ t('ssl_manager.hostname.email') }}</label>
          <input v-model="hostnameForm.email" type="email" class="aura-input w-full" :placeholder="t('ssl_manager.placeholders.email')" />
        </div>
        <div>
          <label class="mb-1 block text-sm text-gray-400">{{ t('ssl_manager.hostname.webroot') }}</label>
          <input v-model="hostnameForm.webroot" class="aura-input w-full" :placeholder="t('ssl_manager.placeholders.webroot')" />
        </div>
      </div>
      <button class="btn-primary" :disabled="!hostnameForm.domain || !hostnameForm.email || issuingHostname" @click="issueHostnameSsl">
        {{ issuingHostname ? t('ssl_manager.hostname.issuing') : t('ssl_manager.hostname.issue') }}
      </button>
    </div>

    <div v-if="tab === 'mail'" class="aura-card space-y-4">
      <p class="rounded-lg border border-amber-500/30 bg-amber-500/10 px-3 py-2 text-sm text-amber-300">
        {{ t('ssl_manager.mail.notice') }}
      </p>
      <div class="grid grid-cols-1 gap-3 lg:grid-cols-3">
        <div>
          <label class="mb-1 block text-sm text-gray-400">{{ t('ssl_manager.mail.hostname') }}</label>
          <select v-model="mailForm.domain" class="aura-input w-full">
            <option value="" disabled>{{ t('ssl_manager.placeholders.mail_hostname') }}</option>
            <option v-for="domainName in mailDomainSuggestions" :key="`ssl-mail-${domainName}`" :value="domainName">{{ domainName }}</option>
          </select>
        </div>
        <div>
          <label class="mb-1 block text-sm text-gray-400">{{ t('ssl_manager.mail.email') }}</label>
          <input v-model="mailForm.email" type="email" class="aura-input w-full" :placeholder="t('ssl_manager.placeholders.email')" />
        </div>
        <div>
          <label class="mb-1 block text-sm text-gray-400">{{ t('ssl_manager.mail.webroot') }}</label>
          <input v-model="mailForm.webroot" class="aura-input w-full" :placeholder="t('ssl_manager.placeholders.webroot')" />
        </div>
      </div>
      <button class="btn-primary" :disabled="!mailForm.domain || !mailForm.email || issuingMail" @click="issueMailSsl">
        {{ issuingMail ? t('ssl_manager.mail.issuing') : t('ssl_manager.mail.issue') }}
      </button>
    </div>

    <div v-if="tab === 'wildcard'" class="aura-card space-y-4">
      <p class="rounded-lg border border-amber-500/30 bg-amber-500/10 px-3 py-2 text-sm text-amber-300">
        {{ t('ssl_manager.wildcard.notice') }}
      </p>
      <div class="grid grid-cols-1 gap-3 lg:grid-cols-3">
        <div>
          <label class="mb-1 block text-sm text-gray-400">{{ t('ssl_manager.wildcard.domain') }}</label>
          <input v-model="wildcardForm.domain" list="ssl-domain-options" class="aura-input w-full" :placeholder="t('ssl_manager.placeholders.domain')" />
        </div>
        <div>
          <label class="mb-1 block text-sm text-gray-400">{{ t('ssl_manager.wildcard.email') }}</label>
          <input v-model="wildcardForm.email" type="email" class="aura-input w-full" :placeholder="t('ssl_manager.placeholders.email')" />
        </div>
        <div>
          <label class="mb-1 block text-sm text-gray-400">{{ t('ssl_manager.wildcard.webroot') }}</label>
          <input v-model="wildcardForm.webroot" class="aura-input w-full" :placeholder="t('ssl_manager.placeholders.webroot')" />
        </div>
      </div>
      <button class="btn-primary" :disabled="!wildcardForm.domain || !wildcardForm.email || issuingWildcard" @click="issueWildcardSsl">
        {{ issuingWildcard ? t('ssl_manager.wildcard.issuing') : t('ssl_manager.wildcard.issue') }}
      </button>
    </div>
  </div>
</template>

<script setup>
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import api from '../services/api'

const { t } = useI18n({ useScope: 'global' })
const route = useRoute()
const router = useRouter()

const allowedTabs = ['manage', 'hostname', 'mail', 'wildcard']
const normalizeTab = value => (typeof value === 'string' && allowedTabs.includes(value) ? value : 'manage')

const tab = ref(normalizeTab(route.query.tab))
const error = ref('')
const success = ref('')

const sites = ref([])
const details = ref(null)
const bindings = ref({ hostname_ssl_domain: null, mail_ssl_domain: null, updated_at: 0 })

const loadingDetails = ref(false)
const issuingManage = ref(false)
const issuingHostname = ref(false)
const issuingMail = ref(false)
const issuingWildcard = ref(false)

const manageForm = ref({ domain: '', email: '', webroot: '' })
const hostnameForm = ref({ domain: '', email: '', webroot: '' })
const mailForm = ref({ domain: '', email: '', webroot: '' })
const wildcardForm = ref({ domain: '', email: '', webroot: '' })

const domains = computed(() => (sites.value || []).map(site => site.domain).filter(Boolean))
const domainSuggestions = computed(() => {
  const names = new Set()

  for (const domain of domains.value || []) {
    const value = String(domain || '').trim().toLowerCase()
    if (value) names.add(value)
  }

  if (bindings.value?.hostname_ssl_domain) {
    names.add(String(bindings.value.hostname_ssl_domain).trim().toLowerCase())
  }

  return Array.from(names).sort((a, b) => a.localeCompare(b))
})
const mailDomainSuggestions = computed(() => {
  const names = new Set()

  for (const domain of domains.value || []) {
    const value = String(domain || '').trim().toLowerCase()
    if (value) names.add(`mail.${value}`)
  }

  if (bindings.value?.mail_ssl_domain) {
    names.add(String(bindings.value.mail_ssl_domain).trim().toLowerCase())
  }

  const hostnameBinding = String(bindings.value?.hostname_ssl_domain || '').trim().toLowerCase()
  if (hostnameBinding) {
    const parts = hostnameBinding.split('.').filter(Boolean)
    if (parts.length >= 3) {
      names.add(`mail.${parts.slice(1).join('.')}`)
    }
  }

  return Array.from(names).sort((a, b) => a.localeCompare(b))
})

function preferredMailHostname() {
  const explicitMailBinding = String(bindings.value?.mail_ssl_domain || '').trim().toLowerCase()
  if (explicitMailBinding) return explicitMailBinding

  const hostnameBinding = String(bindings.value?.hostname_ssl_domain || '').trim().toLowerCase()
  if (hostnameBinding) {
    const parts = hostnameBinding.split('.').filter(Boolean)
    if (parts.length >= 3) {
      return `mail.${parts.slice(1).join('.')}`
    }
  }

  if (sites.value.length > 0) {
    const firstDomain = String(sites.value[0].domain || '').trim().toLowerCase()
    if (firstDomain) return `mail.${firstDomain}`
  }
  return ''
}

function ensureMailDomainDefault() {
  const current = String(mailForm.value.domain || '').trim().toLowerCase()
  if (current) return
  const preferred = preferredMailHostname()
  if (preferred) {
    mailForm.value.domain = preferred
  }
}

watch(
  () => route.query.tab,
  value => {
    tab.value = normalizeTab(value)
  },
)

function tabClass(key) {
  return [
    'pb-3 text-sm font-medium transition',
    tab.value === key ? 'text-brand-400 border-b-2 border-brand-400' : 'text-gray-400 hover:text-white',
  ]
}

function setTab(next) {
  router.replace({ path: '/ssl', query: { tab: normalizeTab(next) } })
}

function apiErrorMessage(err, fallbackKey) {
  return err?.response?.data?.message || err?.message || t(fallbackKey)
}

function formatTime(ms) {
  if (!ms) return '-'
  return new Date(ms).toLocaleString()
}

async function loadSites() {
  try {
    const res = await api.get('/vhost/list')
    sites.value = res.data?.data || []
    if (!manageForm.value.domain && sites.value.length > 0) {
      manageForm.value.domain = sites.value[0].domain
      manageForm.value.email = sites.value[0].email || `admin@${sites.value[0].domain}`
    }
    if (!hostnameForm.value.domain && sites.value.length > 0) {
      hostnameForm.value.domain = sites.value[0].domain
    }
    if (!mailForm.value.domain && sites.value.length > 0) {
      ensureMailDomainDefault()
    }
    if (!wildcardForm.value.domain && sites.value.length > 0) {
      wildcardForm.value.domain = sites.value[0].domain
    }
  } catch {
    sites.value = []
  }
}

async function loadBindings() {
  try {
    const res = await api.get('/ssl/bindings')
    bindings.value = res.data?.data || bindings.value
    ensureMailDomainDefault()
  } catch {
    // best effort
  }
}

async function loadCertificateDetails() {
  error.value = ''
  success.value = ''
  if (!manageForm.value.domain) return
  loadingDetails.value = true
  try {
    const res = await api.post('/ssl/details', { domain: manageForm.value.domain })
    details.value = res.data?.data || null
  } catch (err) {
    error.value = apiErrorMessage(err, 'ssl_manager.messages.details_failed')
  } finally {
    loadingDetails.value = false
  }
}

async function issueManageSsl() {
  error.value = ''
  success.value = ''
  issuingManage.value = true
  try {
    const res = await api.post('/ssl/issue', {
      domain: manageForm.value.domain,
      email: manageForm.value.email,
      webroot: manageForm.value.webroot || undefined,
    })
    success.value = res.data?.message || t('ssl_manager.messages.manage_success')
    await loadCertificateDetails()
  } catch (err) {
    error.value = apiErrorMessage(err, 'ssl_manager.messages.manage_failed')
  } finally {
    issuingManage.value = false
  }
}

async function issueHostnameSsl() {
  error.value = ''
  success.value = ''
  issuingHostname.value = true
  try {
    const res = await api.post('/ssl/hostname/issue', {
      domain: hostnameForm.value.domain,
      email: hostnameForm.value.email,
      webroot: hostnameForm.value.webroot || undefined,
    })
    const warning = res.data?.warning ? ` (${res.data.warning})` : ''
    success.value = (res.data?.message || t('ssl_manager.messages.hostname_success')) + warning
    await loadBindings()
  } catch (err) {
    error.value = apiErrorMessage(err, 'ssl_manager.messages.hostname_failed')
  } finally {
    issuingHostname.value = false
  }
}

async function issueMailSsl() {
  error.value = ''
  success.value = ''
  issuingMail.value = true
  try {
    const res = await api.post('/ssl/mail/issue', {
      domain: mailForm.value.domain,
      email: mailForm.value.email,
      webroot: mailForm.value.webroot || undefined,
    })
    const warning = res.data?.warning ? ` (${res.data.warning})` : ''
    success.value = (res.data?.message || t('ssl_manager.messages.mail_success')) + warning
    await loadBindings()
  } catch (err) {
    error.value = apiErrorMessage(err, 'ssl_manager.messages.mail_failed')
  } finally {
    issuingMail.value = false
  }
}

async function issueWildcardSsl() {
  error.value = ''
  success.value = ''
  issuingWildcard.value = true
  try {
    const res = await api.post('/ssl/wildcard/issue', {
      domain: wildcardForm.value.domain,
      email: wildcardForm.value.email,
      webroot: wildcardForm.value.webroot || undefined,
    })
    success.value = res.data?.message || t('ssl_manager.messages.wildcard_success')
  } catch (err) {
    error.value = apiErrorMessage(err, 'ssl_manager.messages.wildcard_failed')
  } finally {
    issuingWildcard.value = false
  }
}

async function refreshAll() {
  await Promise.all([loadSites(), loadBindings()])
  await loadCertificateDetails()
}

onMounted(async () => {
  const normalized = normalizeTab(route.query.tab)
  if (route.query.tab !== normalized) {
    await router.replace({ path: '/ssl', query: { tab: normalized } })
  }
  await refreshAll()
})
</script>
