<template>
  <div class="space-y-6">
    <div class="flex items-center justify-between gap-3">
      <div>
        <h1 class="text-2xl font-bold text-white">{{ t('ftp_manager.title') }}</h1>
        <p class="text-gray-400 mt-1">{{ t('ftp_manager.subtitle') }}</p>
      </div>
      <button v-if="activeTab === 'ftp'" class="btn-primary" @click="showCreate = true">{{ t('ftp_manager.add_user') }}</button>
    </div>

    <div v-if="error" class="aura-card border-red-500/30 bg-red-500/5 text-red-400">{{ error }}</div>
    <div v-if="success" class="aura-card border-green-500/30 bg-green-500/5 text-green-300">{{ success }}</div>

    <!-- Navigation Tabs -->
    <div class="border-b border-panel-border mb-6">
      <nav class="flex gap-4">
        <button
          @click="activeTab = 'ftp'"
          :class="['pb-3 text-sm font-medium transition', activeTab === 'ftp' ? 'text-emerald-400 border-b-2 border-emerald-400' : 'text-gray-400 hover:text-white']"
        >
          {{ t('ftp_manager.ftp_users_tab') }}
        </button>
        <button
          @click="activeTab = 'tuning'"
          :class="['pb-3 text-sm font-medium transition', activeTab === 'tuning' ? 'text-blue-400 border-b-2 border-blue-400' : 'text-gray-400 hover:text-white']"
        >
          {{ t('ftp_manager.tuning_tab') }}
        </button>
      </nav>
    </div>

    <!-- Tuning Tab -->
    <div v-if="activeTab === 'tuning'" class="space-y-6">
      <div class="aura-card">
        <div class="flex items-center justify-between mb-4">
          <div>
            <h2 class="text-lg font-bold text-white">{{ t('ftp_manager.tuning.title') }}</h2>
            <p class="text-sm text-gray-400">{{ t('ftp_manager.tuning.desc') }}</p>
          </div>
          <button class="btn-secondary" @click="loadTuning">{{ t('common.refresh') }}</button>
        </div>
        
        <div class="grid grid-cols-1 md:grid-cols-2 gap-6">
          <div>
            <label class="block text-sm text-gray-400 mb-1">{{ t('ftp_manager.tuning.passive_port_range') }}</label>
            <input v-model="tuningForm.PassivePortRange" type="text" class="aura-input w-full" :placeholder="t('ftp_manager.tuning.example_passive_port_range')" />
            <p class="text-xs text-gray-500 mt-1">{{ t('ftp_manager.tuning.passive_port_desc') }}</p>
          </div>
          <div>
            <label class="block text-sm text-gray-400 mb-1">{{ t('ftp_manager.tuning.max_clients_number') }}</label>
            <input v-model="tuningForm.MaxClientsNumber" type="text" class="aura-input w-full" :placeholder="t('ftp_manager.tuning.example_max_clients')" />
            <p class="text-xs text-gray-500 mt-1">{{ t('ftp_manager.tuning.max_clients_desc') }}</p>
          </div>
          <div>
            <label class="block text-sm text-gray-400 mb-1">{{ t('ftp_manager.tuning.tls') }}</label>
            <select v-model="tuningForm.TLS" class="aura-input w-full">
              <option value="0">{{ t('ftp_manager.tuning.tls_0') }}</option>
              <option value="1">{{ t('ftp_manager.tuning.tls_1') }}</option>
              <option value="2">{{ t('ftp_manager.tuning.tls_2') }}</option>
            </select>
            <p class="text-xs text-gray-500 mt-1">{{ t('ftp_manager.tuning.tls_desc') }}</p>
          </div>
        </div>
        
        <div class="mt-6 flex justify-end">
          <button class="btn-primary" @click="saveTuning" :disabled="tuningSaving">
            {{ tuningSaving ? t('ftp_manager.tuning.saving') : t('ftp_manager.tuning.save') }}
          </button>
        </div>
      </div>
    </div>

    <!-- FTP Users Tab -->
    <div v-if="activeTab === 'ftp'" class="aura-card space-y-4">
      <div class="flex flex-col md:flex-row md:items-end gap-3">
        <div class="w-full md:max-w-sm">
          <label class="block text-sm text-gray-400 mb-1">{{ t('ftp_manager.domain_filter') }}</label>
          <select v-model="selectedDomain" class="aura-input w-full" @change="onDomainFilterChange">
            <option value="">{{ t('ftp_manager.all_domains') }}</option>
            <option v-for="d in domains" :key="d" :value="d">{{ d }}</option>
          </select>
        </div>
        <button class="btn-secondary" @click="loadFtpUsers">{{ t('ftp_manager.refresh') }}</button>
      </div>

      <div class="overflow-x-auto">
        <table class="w-full text-sm">
          <thead>
            <tr class="border-b border-panel-border text-gray-400">
              <th class="text-left py-2 px-2">{{ t('ftp_manager.table.username') }}</th>
              <th class="text-left py-2 px-2">{{ t('ftp_manager.table.domain') }}</th>
              <th class="text-left py-2 px-2">{{ t('ftp_manager.table.home') }}</th>
              <th class="text-left py-2 px-2">{{ t('ftp_manager.table.created') }}</th>
              <th class="text-right py-2 px-2">{{ t('ftp_manager.table.action') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="item in ftpUsers"
              :key="item.username"
              class="border-b border-panel-border/50"
            >
              <td class="py-2 px-2 text-white font-mono">
                <div class="flex items-center gap-2">
                  <span>{{ item.username }}</span>
                  <span
                    v-if="item.primary"
                    class="inline-flex items-center rounded-full border border-emerald-500/40 bg-emerald-500/10 px-2 py-0.5 text-[10px] font-semibold tracking-wide text-emerald-300"
                  >
                    {{ t('ftp_manager.actions.primary') }}
                  </span>
                </div>
              </td>
              <td class="py-2 px-2 text-gray-300">{{ item.domain || '-' }}</td>
              <td class="py-2 px-2 text-gray-400 font-mono text-xs break-all">{{ item.home_dir }}</td>
              <td class="py-2 px-2 text-gray-400">{{ formatTime(item.created_at) }}</td>
              <td class="py-2 px-2 text-right">
                <div class="flex justify-end gap-2">
                  <button class="btn-secondary px-2 py-1 text-xs" @click="openReset(item.username)">{{ t('ftp_manager.actions.password') }}</button>
                  <button
                    v-if="!item.primary"
                    class="btn-danger px-2 py-1 text-xs"
                    @click="removeUser(item.username)"
                  >{{ t('ftp_manager.actions.delete') }}</button>
                </div>
              </td>
            </tr>
            <tr v-if="ftpUsers.length === 0">
              <td colspan="5" class="text-center py-8 text-gray-500">{{ t('ftp_manager.table.empty') }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <Teleport to="body">
      <div v-if="showCreate" class="fixed inset-0 bg-black/60 flex items-center justify-center z-50 p-4">
        <div class="bg-panel-card border border-panel-border rounded-2xl p-6 w-full max-w-lg">
          <h2 class="text-xl font-bold text-white mb-4">{{ t('ftp_manager.modal.create_title') }}</h2>
          <div class="grid grid-cols-1 md:grid-cols-2 gap-3">
            <div>
              <label class="block text-sm text-gray-400 mb-1">{{ t('ftp_manager.modal.domain_optional') }}</label>
              <select v-model="createForm.domain" class="aura-input w-full" @change="onCreateDomainChange">
                <option value="">{{ t('ftp_manager.modal.not_selected') }}</option>
                <option v-for="d in domains" :key="d" :value="d">{{ d }}</option>
              </select>
            </div>
            <div>
              <label class="block text-sm text-gray-400 mb-1">{{ t('ftp_manager.modal.username') }}</label>
              <input v-model="createForm.username" class="aura-input w-full" :placeholder="t('ftp_manager.modal.username_placeholder')" />
            </div>
            <div class="md:col-span-2">
              <label class="block text-sm text-gray-400 mb-1">{{ t('ftp_manager.modal.password') }}</label>
              <input v-model="createForm.password" type="password" class="aura-input w-full" />
            </div>
            <div class="md:col-span-2">
              <label class="block text-sm text-gray-400 mb-1">{{ t('ftp_manager.modal.home_directory') }}</label>
              <input v-model="createForm.home_dir" class="aura-input w-full" :placeholder="t('ftp_manager.modal.home_directory_placeholder')" />
            </div>
          </div>
          <div class="flex gap-3 mt-6">
            <button class="btn-secondary flex-1" @click="showCreate = false">{{ t('ftp_manager.modal.cancel') }}</button>
            <button class="btn-primary flex-1" @click="createUser">{{ t('ftp_manager.modal.create') }}</button>
          </div>
        </div>
      </div>
    </Teleport>

    <Teleport to="body">
      <div v-if="showReset" class="fixed inset-0 bg-black/60 flex items-center justify-center z-50 p-4">
        <div class="bg-panel-card border border-panel-border rounded-2xl p-6 w-full max-w-md">
          <h2 class="text-xl font-bold text-white mb-4">{{ t('ftp_manager.modal.reset_title') }}</h2>
          <p class="text-sm text-gray-400 mb-3">{{ t('ftp_manager.modal.user_label') }}: <span class="text-white font-mono">{{ resetForm.username }}</span></p>
          <input v-model="resetForm.new_password" type="password" class="aura-input w-full" :placeholder="t('ftp_manager.modal.new_password_placeholder')" />
          <div class="flex gap-3 mt-6">
            <button class="btn-secondary flex-1" @click="showReset = false">{{ t('ftp_manager.modal.cancel') }}</button>
            <button class="btn-primary flex-1" @click="updatePassword">{{ t('ftp_manager.modal.update') }}</button>
          </div>
        </div>
      </div>
    </Teleport>
  </div>
</template>

<script setup>
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import api from '../services/api'

const route = useRoute()
const router = useRouter()
const { t, locale } = useI18n({ useScope: 'global' })

const ftpUsers = ref([])
const sites = ref([])
const error = ref('')
const success = ref('')
const showCreate = ref(false)
const createActionConsumed = ref(false)
const showReset = ref(false)
const selectedDomain = ref(typeof route.query.domain === 'string' ? route.query.domain : '')

const activeTab = ref(route.query.tab === 'tuning' ? 'tuning' : 'ftp')
const tuningForm = ref({
  PassivePortRange: '',
  MaxClientsNumber: '',
  TLS: '2'
})
const tuningSaving = ref(false)

async function loadTuning() {
  try {
    const res = await api.get('/ftp/tuning')
    if (res.data?.data) {
      tuningForm.value = { ...tuningForm.value, ...res.data.data }
    }
  } catch (err) {
    console.error('FTP tuning load error', err)
  }
}

async function saveTuning() {
  tuningSaving.value = true
  try {
    await api.post('/ftp/tuning', tuningForm.value)
    success.value = t('ftp_manager.tuning.save_success')
    setTimeout(() => { success.value = '' }, 3000)
  } catch (err) {
    error.value = `${t('common.error')}: ${err.response?.data?.message || err.message}`
    setTimeout(() => { error.value = '' }, 3000)
  } finally {
    tuningSaving.value = false
  }
}

watch(activeTab, (newVal) => {
  if (newVal === 'tuning') {
    loadTuning()
  } else {
    loadFtpUsers()
  }
  syncRouteQuery()
})

const createForm = ref({
  domain: selectedDomain.value,
  username: '',
  password: '',
  home_dir: '',
})

const resetForm = ref({
  username: '',
  new_password: '',
})

const domains = computed(() => (sites.value || []).map((x) => x.domain).filter(Boolean))

watch(
  () => route.query.domain,
  (value) => {
    selectedDomain.value = typeof value === 'string' ? value : ''
    createForm.value.domain = selectedDomain.value
    if (!createForm.value.home_dir) {
      createForm.value.home_dir = defaultHome(selectedDomain.value)
    }
    if (activeTab.value === 'ftp') {
      loadFtpUsers()
    }
  }
)

watch(
  () => route.query.tab,
  (value) => {
    const nextTab = value === 'tuning' ? 'tuning' : 'ftp'
    if (activeTab.value !== nextTab) {
      activeTab.value = nextTab
    }
  },
  { immediate: true },
)

watch(
  () => route.query.action,
  (value) => {
    const shouldOpenCreate = value === 'create'
    if (shouldOpenCreate && !createActionConsumed.value) {
      showCreate.value = true
      createActionConsumed.value = true
    }
    if (!shouldOpenCreate) {
      createActionConsumed.value = false
    }
  },
  { immediate: true },
)

function formatTime(ts) {
  const value = Number(ts || 0)
  if (!value) return '-'
  return new Date(value * 1000).toLocaleString(locale.value)
}

function apiErrorMessage(e, fallbackKey) {
  return e?.response?.data?.message || e?.message || t(fallbackKey)
}

function defaultHome(domain) {
  if (domain) {
    return `/home/${domain}/public_html`
  }
  return '/home'
}

function onCreateDomainChange() {
  if (!createForm.value.home_dir || createForm.value.home_dir === '/home') {
    createForm.value.home_dir = defaultHome(createForm.value.domain)
  }
}

function onDomainFilterChange() {
  syncRouteQuery()
}

function syncRouteQuery() {
  const nextQuery = {}
  if (activeTab.value === 'tuning') {
    nextQuery.tab = 'tuning'
  }
  if (selectedDomain.value) {
    nextQuery.domain = selectedDomain.value
  }
  const currentAction = typeof route.query.action === 'string' ? route.query.action : ''
  if (currentAction === 'create') {
    nextQuery.action = 'create'
  }

  const currentTab = typeof route.query.tab === 'string' ? route.query.tab : ''
  const currentDomain = typeof route.query.domain === 'string' ? route.query.domain : ''
  const sameQuery = currentTab === (nextQuery.tab || '')
    && currentDomain === (nextQuery.domain || '')
    && currentAction === (nextQuery.action || '')
  if (!sameQuery) {
    router.replace({ path: '/ftp', query: nextQuery })
  }
}

async function loadSites() {
  try {
    const res = await api.get('/vhost/list')
    sites.value = res.data?.data || []
  } catch {
    sites.value = []
  }
}

async function loadFtpUsers() {
  error.value = ''
  try {
    const params = selectedDomain.value ? { domain: selectedDomain.value } : {}
    const res = await api.get('/ftp/list', { params })
    ftpUsers.value = res.data?.data || []
  } catch (e) {
    error.value = apiErrorMessage(e, 'ftp_manager.messages.list_failed')
  }
}

async function createUser() {
  error.value = ''
  success.value = ''

  const payload = {
    username: createForm.value.username,
    password: createForm.value.password,
    home_dir: createForm.value.home_dir || defaultHome(createForm.value.domain),
    domain: createForm.value.domain || undefined,
  }
  if (!payload.username || !payload.password || !payload.home_dir) {
    error.value = t('ftp_manager.messages.required_create')
    return
  }

  try {
    const res = await api.post('/ftp/create', payload)
    success.value = res.data?.message || t('ftp_manager.messages.created')
    showCreate.value = false
    createForm.value = {
      domain: selectedDomain.value,
      username: '',
      password: '',
      home_dir: defaultHome(selectedDomain.value),
    }
    await loadFtpUsers()
  } catch (e) {
    error.value = apiErrorMessage(e, 'ftp_manager.messages.create_failed')
  }
}

function openReset(username) {
  resetForm.value.username = username
  resetForm.value.new_password = ''
  showReset.value = true
}

async function updatePassword() {
  error.value = ''
  success.value = ''

  if (!resetForm.value.username || !resetForm.value.new_password) {
    error.value = t('ftp_manager.messages.required_reset')
    return
  }

  try {
    const res = await api.post('/ftp/password', {
      username: resetForm.value.username,
      new_password: resetForm.value.new_password,
    })
    success.value = res.data?.message || t('ftp_manager.messages.password_updated')
    showReset.value = false
  } catch (e) {
    error.value = apiErrorMessage(e, 'ftp_manager.messages.password_failed')
  }
}

async function removeUser(username) {
  if (!window.confirm(t('ftp_manager.messages.delete_confirm', { username }))) return
  error.value = ''
  success.value = ''
  try {
    const res = await api.post('/ftp/delete', { username })
    success.value = res.data?.message || t('ftp_manager.messages.deleted')
    await loadFtpUsers()
  } catch (e) {
    error.value = apiErrorMessage(e, 'ftp_manager.messages.delete_failed')
  }
}

onMounted(async () => {
  createForm.value.home_dir = defaultHome(createForm.value.domain)
  await loadSites()
  if (activeTab.value === 'tuning') {
    await loadTuning()
  } else {
    await loadFtpUsers()
  }
  syncRouteQuery()
})
</script>
