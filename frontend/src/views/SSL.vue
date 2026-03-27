<template>
  <div class="space-y-6">
    <div class="flex items-center justify-between gap-3">
      <div>
        <h1 class="text-2xl font-bold text-white">SSL Manager</h1>
        <p class="text-gray-400 mt-1">Manage SSL, Hostname SSL ve Mail Server SSL akislarini yonetin.</p>
      </div>
      <button class="btn-secondary" @click="refreshAll">Yenile</button>
    </div>

    <div class="aura-card space-y-3">
      <p class="text-xs text-gray-400 uppercase tracking-wide">Current Binding Status</p>
      <div class="grid grid-cols-1 md:grid-cols-2 gap-3 text-sm">
        <div class="rounded-lg border border-panel-border p-3 bg-panel-hover/30">
          <p class="text-gray-400">Hostname SSL</p>
          <p class="text-white font-mono mt-1">{{ bindings.hostname_ssl_domain || '-' }}</p>
        </div>
        <div class="rounded-lg border border-panel-border p-3 bg-panel-hover/30">
          <p class="text-gray-400">Mail SSL</p>
          <p class="text-white font-mono mt-1">{{ bindings.mail_ssl_domain || '-' }}</p>
        </div>
      </div>
      <p v-if="bindings.updated_at" class="text-xs text-gray-500">
        Son guncelleme: {{ formatTime(bindings.updated_at * 1000) }}
      </p>
    </div>

    <div class="border-b border-panel-border">
      <nav class="flex gap-6">
        <button @click="setTab('manage')" :class="tabClass('manage')">Manage SSL</button>
        <button @click="setTab('hostname')" :class="tabClass('hostname')">Hostname SSL</button>
        <button @click="setTab('mail')" :class="tabClass('mail')">MailServer SSL</button>
      </nav>
    </div>

    <div v-if="error" class="aura-card border-red-500/30 bg-red-500/5 text-red-400">{{ error }}</div>
    <div v-if="success" class="aura-card border-green-500/30 bg-green-500/5 text-green-300">{{ success }}</div>

    <div v-if="tab === 'manage'" class="aura-card space-y-4">
      <div class="grid grid-cols-1 lg:grid-cols-3 gap-3">
        <div>
          <label class="block text-sm text-gray-400 mb-1">Website Domain</label>
          <select v-model="manageForm.domain" class="aura-input w-full" @change="loadCertificateDetails">
            <option value="" disabled>Domain secin</option>
            <option v-for="d in domains" :key="d" :value="d">{{ d }}</option>
          </select>
        </div>
        <div>
          <label class="block text-sm text-gray-400 mb-1">Admin Email</label>
          <input v-model="manageForm.email" type="email" class="aura-input w-full" placeholder="admin@example.com" />
        </div>
        <div>
          <label class="block text-sm text-gray-400 mb-1">Webroot (optional)</label>
          <input v-model="manageForm.webroot" class="aura-input w-full" placeholder="/usr/local/lsws/Example/html" />
        </div>
      </div>

      <div class="flex flex-wrap gap-2">
        <button class="btn-secondary" @click="loadCertificateDetails" :disabled="!manageForm.domain || loadingDetails">
          {{ loadingDetails ? 'Kontrol ediliyor...' : 'SSL Detaylarini Getir' }}
        </button>
        <button class="btn-primary" @click="issueManageSsl" :disabled="!manageForm.domain || !manageForm.email || issuingManage">
          {{ issuingManage ? 'SSL olusturuluyor...' : 'Issue SSL' }}
        </button>
      </div>

      <div v-if="details" class="rounded-xl border border-panel-border p-4 bg-panel-hover/20">
        <div class="grid grid-cols-1 md:grid-cols-2 gap-3 text-sm">
          <div>
            <p class="text-gray-400">Status</p>
            <p class="text-white mt-1">{{ details.status }}</p>
          </div>
          <div>
            <p class="text-gray-400">Issuer</p>
            <p class="text-white mt-1">{{ details.issuer || '-' }}</p>
          </div>
          <div>
            <p class="text-gray-400">Expiry</p>
            <p class="text-white mt-1">{{ details.expiry_date || '-' }}</p>
          </div>
          <div>
            <p class="text-gray-400">Days Remaining</p>
            <p class="text-white mt-1">{{ details.days_remaining ?? '-' }}</p>
          </div>
        </div>
      </div>
    </div>

    <div v-if="tab === 'hostname'" class="aura-card space-y-4">
      <p class="text-sm text-amber-300 bg-amber-500/10 border border-amber-500/30 rounded-lg px-3 py-2">
        Hostname sertifikasi uretilir. Panel TLS listener binding adimi ayri olarak uygulanmalidir.
      </p>
      <div class="grid grid-cols-1 lg:grid-cols-3 gap-3">
        <div>
          <label class="block text-sm text-gray-400 mb-1">Hostname</label>
          <input v-model="hostnameForm.domain" class="aura-input w-full" placeholder="panel.example.com" />
        </div>
        <div>
          <label class="block text-sm text-gray-400 mb-1">Email</label>
          <input v-model="hostnameForm.email" type="email" class="aura-input w-full" placeholder="admin@example.com" />
        </div>
        <div>
          <label class="block text-sm text-gray-400 mb-1">Webroot (optional)</label>
          <input v-model="hostnameForm.webroot" class="aura-input w-full" placeholder="/usr/local/lsws/Example/html" />
        </div>
      </div>
      <button class="btn-primary" @click="issueHostnameSsl" :disabled="!hostnameForm.domain || !hostnameForm.email || issuingHostname">
        {{ issuingHostname ? 'Hostname SSL olusturuluyor...' : 'Issue Hostname SSL' }}
      </button>
    </div>

    <div v-if="tab === 'mail'" class="aura-card space-y-4">
      <p class="text-sm text-amber-300 bg-amber-500/10 border border-amber-500/30 rounded-lg px-3 py-2">
        Mail server sertifikasi uretilir ve Postfix/Dovecot baglantisi otomatik uygulanir.
      </p>
      <div class="grid grid-cols-1 lg:grid-cols-3 gap-3">
        <div>
          <label class="block text-sm text-gray-400 mb-1">Mail Hostname</label>
          <input v-model="mailForm.domain" class="aura-input w-full" placeholder="mail.example.com" />
        </div>
        <div>
          <label class="block text-sm text-gray-400 mb-1">Email</label>
          <input v-model="mailForm.email" type="email" class="aura-input w-full" placeholder="admin@example.com" />
        </div>
        <div>
          <label class="block text-sm text-gray-400 mb-1">Webroot (optional)</label>
          <input v-model="mailForm.webroot" class="aura-input w-full" placeholder="/usr/local/lsws/Example/html" />
        </div>
      </div>
      <button class="btn-primary" @click="issueMailSsl" :disabled="!mailForm.domain || !mailForm.email || issuingMail">
        {{ issuingMail ? 'Mail SSL olusturuluyor...' : 'Issue Mail SSL' }}
      </button>
    </div>
  </div>
</template>

<script setup>
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import api from '../services/api'

const route = useRoute()
const router = useRouter()

const allowedTabs = ['manage', 'hostname', 'mail']
const normalizeTab = (value) => {
  if (typeof value !== 'string') return 'manage'
  return allowedTabs.includes(value) ? value : 'manage'
}

const tab = ref(normalizeTab(route.query.tab))
const error = ref('')
const success = ref('')

const sites = ref([])
const details = ref(null)
const bindings = ref({
  hostname_ssl_domain: null,
  mail_ssl_domain: null,
  updated_at: 0,
})

const loadingDetails = ref(false)
const issuingManage = ref(false)
const issuingHostname = ref(false)
const issuingMail = ref(false)

const manageForm = ref({
  domain: '',
  email: '',
  webroot: '',
})

const hostnameForm = ref({
  domain: '',
  email: '',
  webroot: '',
})

const mailForm = ref({
  domain: '',
  email: '',
  webroot: '',
})

const domains = computed(() => (sites.value || []).map((x) => x.domain).filter(Boolean))

watch(
  () => route.query.tab,
  (value) => {
    tab.value = normalizeTab(value)
  }
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

function apiErrorMessage(e, fallback) {
  return e?.response?.data?.message || e?.message || fallback
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
  } catch {
    sites.value = []
  }
}

async function loadBindings() {
  try {
    const res = await api.get('/ssl/bindings')
    bindings.value = res.data?.data || bindings.value
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
  } catch (e) {
    error.value = apiErrorMessage(e, 'SSL detaylari alinamadi')
  } finally {
    loadingDetails.value = false
  }
}

async function issueManageSsl() {
  error.value = ''
  success.value = ''
  issuingManage.value = true
  try {
    const payload = {
      domain: manageForm.value.domain,
      email: manageForm.value.email,
      webroot: manageForm.value.webroot || undefined,
    }
    const res = await api.post('/ssl/issue', payload)
    success.value = res.data?.message || 'Website SSL olusturuldu.'
    await loadCertificateDetails()
  } catch (e) {
    error.value = apiErrorMessage(e, 'Website SSL olusturulamadi')
  } finally {
    issuingManage.value = false
  }
}

async function issueHostnameSsl() {
  error.value = ''
  success.value = ''
  issuingHostname.value = true
  try {
    const payload = {
      domain: hostnameForm.value.domain,
      email: hostnameForm.value.email,
      webroot: hostnameForm.value.webroot || undefined,
    }
    const res = await api.post('/ssl/hostname/issue', payload)
    const warning = res.data?.warning ? ` (${res.data.warning})` : ''
    success.value = (res.data?.message || 'Hostname SSL olusturuldu.') + warning
    await loadBindings()
  } catch (e) {
    error.value = apiErrorMessage(e, 'Hostname SSL olusturulamadi')
  } finally {
    issuingHostname.value = false
  }
}

async function issueMailSsl() {
  error.value = ''
  success.value = ''
  issuingMail.value = true
  try {
    const payload = {
      domain: mailForm.value.domain,
      email: mailForm.value.email,
      webroot: mailForm.value.webroot || undefined,
    }
    const res = await api.post('/ssl/mail/issue', payload)
    const warning = res.data?.warning ? ` (${res.data.warning})` : ''
    success.value = (res.data?.message || 'Mail SSL olusturuldu.') + warning
    await loadBindings()
  } catch (e) {
    error.value = apiErrorMessage(e, 'Mail SSL olusturulamadi')
  } finally {
    issuingMail.value = false
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
