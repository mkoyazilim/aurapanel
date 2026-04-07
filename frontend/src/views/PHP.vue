<template>
  <div class="space-y-6 php-theme">
    <div>
      <h1 class="text-2xl font-bold text-white">{{ t('php.title') }}</h1>
      <p class="text-gray-400 mt-1">{{ t('php.subtitle') }}</p>
    </div>

    <div class="border-b border-panel-border">
      <nav class="flex gap-6">
        <button @click="tab='versions'" :class="tabClass('versions')">{{ t('php.versions_tab') }}</button>
        <button @click="tab='sites'" :class="tabClass('sites')">{{ t('php.sites_tab') }}</button>
        <button @click="tab='extensions'" :class="tabClass('extensions')">{{ t('php.extensions_tab') }}</button>
        <button @click="tab='config'" :class="tabClass('config')">{{ t('php.ini_tab') }}</button>
      </nav>
    </div>

    <div v-if="error" class="aura-card border-red-500/30 bg-red-500/5 text-red-400">{{ error }}</div>
    <div v-if="success" class="aura-card border-green-500/30 bg-green-500/5 text-green-300">{{ success }}</div>

    <div v-if="loading" class="text-center py-10 text-gray-400">{{ t('common.loading') }}</div>

    <div v-else>
      <div v-if="tab==='versions'" class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
        <div v-for="v in phpVersions" :key="v.version" class="bg-panel-card border border-panel-border rounded-xl p-5">
          <div class="flex items-center justify-between mb-3">
            <div>
              <p class="text-white font-semibold">PHP {{ v.version }}</p>
              <p class="text-xs" :class="v.eol ? 'text-yellow-400' : 'text-gray-400'">{{ v.eol ? t('php.eol') : t('php.supported') }}</p>
            </div>
            <span :class="['px-2 py-0.5 rounded text-xs font-medium', v.installed ? 'bg-green-500/15 text-green-400' : 'bg-gray-500/15 text-gray-400']">
              {{ v.installed ? t('php.installed') : t('php.not_installed') }}
            </span>
          </div>
          <div class="flex gap-2">
            <button v-if="!v.installed" class="btn-primary flex-1" @click="installPhp(v.version)">{{ t('php.install') }}</button>
            <button v-else class="btn-danger flex-1" @click="removePhp(v.version)">{{ t('php.remove') }}</button>
            <button v-if="v.installed" class="btn-secondary" @click="restartPhp(v.version)">{{ t('php.restart') }}</button>
          </div>
        </div>
      </div>

      <div v-if="tab==='sites'" class="bg-panel-card border border-panel-border rounded-xl overflow-hidden">
        <table class="w-full text-sm">
          <thead>
            <tr class="text-gray-400 border-b border-panel-border">
              <th class="text-left px-4 py-3">{{ t('php.domain') }}</th>
              <th class="text-left px-4 py-3">{{ t('php.current_php') }}</th>
              <th class="text-left px-4 py-3">{{ t('php.change_php') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="site in siteAssignments" :key="site.domain" class="border-b border-panel-border/50 hover:bg-white/[0.02] transition">
              <td class="px-4 py-3 text-white">{{ site.domain }}</td>
              <td class="px-4 py-3 text-gray-300">PHP {{ site.php_version }}</td>
              <td class="px-4 py-3">
                <select v-model="site.php_version" class="php-field aura-input" @change="changePhp(site)">
                  <option v-for="v in installedVersions" :key="v" :value="v">PHP {{ v }}</option>
                </select>
              </td>
            </tr>
            <tr v-if="siteAssignments.length===0">
              <td colspan="3" class="p-4 text-center text-gray-500">{{ t('php.site_not_found') }}</td>
            </tr>
          </tbody>
        </table>
      </div>

      <div v-if="tab==='extensions'" class="bg-panel-card border border-panel-border rounded-xl overflow-hidden">
        <div class="p-4 border-b border-panel-border flex flex-wrap items-center gap-3">
          <select v-model="selectedExtensionVersion" class="php-field aura-input max-w-xs" @change="loadExtensions">
            <option v-for="v in installedVersions" :key="`ext-${v}`" :value="v">PHP {{ v }}</option>
          </select>
          <button class="btn-secondary" @click="loadExtensions">{{ t('common.refresh') }}</button>
          <span class="text-xs text-gray-400">{{ t('php.package_manager') }}: {{ extensionPackageManager || '-' }}</span>
        </div>
        <div v-if="extensionLoading" class="p-6 text-center text-gray-400">{{ t('common.loading') }}</div>
        <table v-else class="w-full text-sm">
          <thead>
            <tr class="text-gray-400 border-b border-panel-border">
              <th class="text-left px-4 py-3">{{ t('php.extension_name') }}</th>
              <th class="text-left px-4 py-3">{{ t('php.package') }}</th>
              <th class="text-left px-4 py-3">{{ t('php.status') }}</th>
              <th class="text-left px-4 py-3">{{ t('php.action') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="item in extensionItems" :key="item.id" class="border-b border-panel-border/50 hover:bg-white/[0.02] transition">
              <td class="px-4 py-3">
                <p class="text-white font-medium">{{ item.name }}</p>
                <p class="text-xs text-gray-400 mt-0.5">{{ item.description }}</p>
              </td>
              <td class="px-4 py-3 text-gray-300 font-mono text-xs">{{ item.package || '-' }}</td>
              <td class="px-4 py-3">
                <span :class="['px-2 py-0.5 rounded text-xs font-medium', item.installed ? 'bg-green-500/15 text-green-400' : 'bg-gray-500/15 text-gray-400']">
                  {{ item.installed ? t('php.installed') : t('php.not_installed') }}
                </span>
                <span v-if="item.baseline" class="ml-2 px-2 py-0.5 rounded text-xs bg-blue-500/15 text-blue-300">{{ t('php.baseline') }}</span>
              </td>
              <td class="px-4 py-3">
                <div class="flex items-center gap-2">
                  <button
                    class="btn-primary px-3 py-1.5 text-xs"
                    :disabled="item.installed || !item.available || extensionActionKey === `install-${item.id}`"
                    @click="installExtension(item)"
                  >
                    <Loader2 v-if="extensionActionKey === `install-${item.id}`" class="w-3 h-3 animate-spin mr-1 inline" />
                    {{ t('php.install') }}
                  </button>
                  <button
                    class="btn-danger px-3 py-1.5 text-xs"
                    :disabled="!item.installed || extensionActionKey === `remove-${item.id}`"
                    @click="removeExtension(item)"
                  >
                    <Loader2 v-if="extensionActionKey === `remove-${item.id}`" class="w-3 h-3 animate-spin mr-1 inline" />
                    {{ t('php.remove') }}
                  </button>
                </div>
              </td>
            </tr>
            <tr v-if="extensionItems.length===0">
              <td colspan="4" class="p-4 text-center text-gray-500">{{ t('php.extension_list_failed') }}</td>
            </tr>
          </tbody>
        </table>
      </div>

      <div v-if="tab==='config'" class="space-y-3">
        <div class="flex items-center gap-3">
          <select v-model="selectedConfigVersion" class="php-field aura-input max-w-xs" @change="loadPhpIni">
            <option v-for="v in installedVersions" :key="v" :value="v">PHP {{ v }}</option>
          </select>
          <button class="btn-secondary" @click="loadPhpIni">{{ t('php.read_ini') }}</button>
          <button class="btn-primary" @click="savePhpIni">{{ t('common.save') }}</button>
        </div>
        <textarea v-model="phpIniContent" rows="20" class="aura-input w-full font-mono text-xs"></textarea>
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import api from '../services/api'

const route = useRoute()
const router = useRouter()
const { t } = useI18n({ useScope: 'global' })

const normalizeTab = (query) => {
  const value = String(query?.tab || '').trim().toLowerCase()
  if (value === 'sites' || value === 'config' || value === 'extensions') return value
  return 'versions'
}

const tab = ref(normalizeTab(route.query))
const loading = ref(false)
const error = ref('')
const success = ref('')

const phpVersions = ref([])
const siteAssignments = ref([])
const selectedConfigVersion = ref(typeof route.query.version === 'string' ? route.query.version : '')
const selectedExtensionVersion = ref(typeof route.query.version === 'string' ? route.query.version : '')
const phpIniContent = ref('')
const extensionItems = ref([])
const extensionLoading = ref(false)
const extensionActionKey = ref('')
const extensionPackageManager = ref('')

const installedVersions = computed(() => phpVersions.value.filter(v => v.installed).map(v => v.version))

function tabClass(key) {
  return [
    'pb-3 text-sm font-medium transition',
    tab.value === key ? 'text-orange-400 border-b-2 border-orange-400' : 'text-gray-400 hover:text-white',
  ]
}

function apiErrorMessage(e, fallbackKey) {
  return e?.response?.data?.message || e?.message || t(fallbackKey)
}

async function loadData() {
  loading.value = true
  error.value = ''
  success.value = ''
  try {
    const [versionsRes, vhostRes] = await Promise.all([
      api.get('/php/versions'),
      api.get('/vhost/list'),
    ])

    phpVersions.value = versionsRes.data?.data || []
    siteAssignments.value = (vhostRes.data?.data || []).map((site) => ({
      domain: site.domain,
      php_version: site.php_version || site.php || '8.3',
      owner: site.owner || site.user || '',
      package: site.package || '',
      email: site.email || '',
    }))

    if (!selectedConfigVersion.value || !installedVersions.value.includes(selectedConfigVersion.value)) {
      selectedConfigVersion.value = installedVersions.value[0] || ''
    }
    if (!selectedExtensionVersion.value || !installedVersions.value.includes(selectedExtensionVersion.value)) {
      selectedExtensionVersion.value = installedVersions.value[0] || ''
    }

    if (selectedConfigVersion.value && tab.value === 'config') {
      await loadPhpIni()
    } else if (selectedExtensionVersion.value && tab.value === 'extensions') {
      await loadExtensions()
    } else {
      phpIniContent.value = ''
    }
  } catch (e) {
    error.value = apiErrorMessage(e, 'php.messages.load_failed')
  } finally {
    loading.value = false
  }
}

async function installPhp(version) {
  error.value = ''
  success.value = ''
  try {
    const res = await api.post('/php/install', { version })
    success.value = res.data?.message || t('php.messages.installed', { version })
    // Do not immediately reload, let the background job run
    setTimeout(() => {
      loadData()
    }, 5000)
  } catch (e) {
    error.value = apiErrorMessage(e, 'php.messages.install_failed')
  }
}

async function removePhp(version) {
  error.value = ''
  success.value = ''
  if (!confirm(t('php.remove_confirm', { version }))) return
  try {
    const res = await api.post('/php/remove', { version })
    success.value = res.data?.message || t('php.messages.removed', { version })
    setTimeout(() => {
      loadData()
    }, 5000)
  } catch (e) {
    error.value = apiErrorMessage(e, 'php.messages.remove_failed')
  }
}

async function restartPhp(version) {
  error.value = ''
  success.value = ''
  try {
    const res = await api.post('/php/restart', { version })
    success.value = res.data?.message || t('php.messages.restarted', { version })
  } catch (e) {
    error.value = apiErrorMessage(e, 'php.messages.restart_failed')
  }
}

async function changePhp(site) {
  error.value = ''
  success.value = ''
  try {
    const res = await api.post('/vhost/update', {
      domain: site.domain,
      php_version: site.php_version,
      owner: site.owner || undefined,
      package: site.package || undefined,
      email: site.email || undefined,
    })
    success.value = res.data?.message || t('php.messages.site_updated', { domain: site.domain })
  } catch (e) {
    error.value = apiErrorMessage(e, 'php.messages.site_update_failed')
  }
}

async function loadPhpIni() {
  if (!selectedConfigVersion.value) {
    phpIniContent.value = ''
    return
  }

  error.value = ''
  try {
    const res = await api.post('/php/ini/get', { version: selectedConfigVersion.value })
    phpIniContent.value = String(res.data?.data || '')
  } catch (e) {
    error.value = apiErrorMessage(e, 'php.messages.ini_read_failed')
  }
}

async function loadExtensions() {
  if (!selectedExtensionVersion.value) {
    extensionItems.value = []
    extensionPackageManager.value = ''
    return
  }
  extensionLoading.value = true
  error.value = ''
  try {
    const res = await api.get('/php/extensions', { params: { version: selectedExtensionVersion.value } })
    const payload = res.data?.data || {}
    extensionItems.value = Array.isArray(payload.extensions) ? payload.extensions : []
    extensionPackageManager.value = payload.package_manager || ''
  } catch (e) {
    extensionItems.value = []
    extensionPackageManager.value = ''
    error.value = apiErrorMessage(e, 'php.messages.load_failed')
  } finally {
    extensionLoading.value = false
  }
}

async function installExtension(item) {
  if (!item?.id || !selectedExtensionVersion.value) return
  extensionActionKey.value = `install-${item.id}`
  error.value = ''
  success.value = ''
  try {
    const res = await api.post('/php/extensions/install', {
      version: selectedExtensionVersion.value,
      extension: item.id,
    })
    success.value = res.data?.message || t('php.messages.extension_installed', { name: item.name })
    await loadExtensions()
  } catch (e) {
    error.value = apiErrorMessage(e, 'php.messages.install_failed')
  } finally {
    extensionActionKey.value = ''
  }
}

async function removeExtension(item) {
  if (!item?.id || !selectedExtensionVersion.value) return
  extensionActionKey.value = `remove-${item.id}`
  error.value = ''
  success.value = ''
  try {
    const res = await api.post('/php/extensions/remove', {
      version: selectedExtensionVersion.value,
      extension: item.id,
    })
    success.value = res.data?.message || t('php.messages.extension_removed', { name: item.name })
    await loadExtensions()
  } catch (e) {
    error.value = apiErrorMessage(e, 'php.messages.remove_failed')
  } finally {
    extensionActionKey.value = ''
  }
}

async function savePhpIni() {
  if (!selectedConfigVersion.value) return
  error.value = ''
  success.value = ''
  try {
    const res = await api.post('/php/ini/save', {
      version: selectedConfigVersion.value,
      content: phpIniContent.value,
    })
    success.value = res.data?.message || t('php.save_ini_success')
  } catch (e) {
    error.value = apiErrorMessage(e, 'php.messages.ini_save_failed')
  }
}

function syncRouteQuery() {
  const nextQuery = {}
  if (tab.value === 'sites') {
    nextQuery.tab = 'sites'
  }
  if (tab.value === 'extensions') {
    nextQuery.tab = 'extensions'
  }
  if (tab.value === 'config') {
    nextQuery.tab = 'config'
  }
  if (tab.value === 'config' && selectedConfigVersion.value) {
    nextQuery.version = selectedConfigVersion.value
  }
  if (tab.value === 'extensions' && selectedExtensionVersion.value) {
    nextQuery.version = selectedExtensionVersion.value
  }

  const currentTab = typeof route.query.tab === 'string' ? route.query.tab : ''
  const currentVersion = typeof route.query.version === 'string' ? route.query.version : ''
  const sameQuery = currentTab === (nextQuery.tab || '') && currentVersion === (nextQuery.version || '')
  if (!sameQuery) {
    router.replace({ path: '/php', query: nextQuery })
  }
}

watch(
  () => route.query,
  (query) => {
    const nextTab = normalizeTab(query)
    const nextVersion = typeof query.version === 'string' ? query.version : ''
    if (tab.value !== nextTab) {
      tab.value = nextTab
    }
    if (selectedConfigVersion.value !== nextVersion && nextVersion) {
      selectedConfigVersion.value = nextVersion
    }
    if (selectedExtensionVersion.value !== nextVersion && nextVersion) {
      selectedExtensionVersion.value = nextVersion
    }
  },
  { immediate: true },
)

watch(tab, () => {
  if (tab.value === 'config' && selectedConfigVersion.value) {
    loadPhpIni()
  }
  if (tab.value === 'extensions' && selectedExtensionVersion.value) {
    loadExtensions()
  }
  syncRouteQuery()
})

watch(selectedConfigVersion, () => {
  if (tab.value === 'config' && selectedConfigVersion.value) {
    syncRouteQuery()
  }
})

watch(selectedExtensionVersion, () => {
  if (tab.value === 'extensions' && selectedExtensionVersion.value) {
    syncRouteQuery()
  }
})

onMounted(async () => {
  await loadData()
  syncRouteQuery()
})
</script>

<style scoped>
.php-theme .php-field {
  background-color: rgb(var(--panel-darker) / 1) !important;
  color: rgb(var(--brand-600) / 1) !important;
  border-color: rgb(var(--panel-border) / 1) !important;
}

.php-theme .php-field option {
  background: rgb(var(--panel-dark) / 1);
  color: rgb(var(--brand-600) / 1);
}
</style>
