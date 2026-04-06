<template>
  <div class="space-y-6">
    <div>
      <h1 class="flex items-center gap-3 text-2xl font-bold text-white">
        <svg class="h-7 w-7 text-purple-400" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M20 7l-8-4-8 4m16 0l-8 4m8-4v10l-8 4m0-10L4 7m8 4v10M4 7v10l8 4"/></svg>
        {{ t('docker_apps_screen.title') }}
      </h1>
      <p class="mt-1 text-gray-400">{{ t('docker_apps_screen.subtitle') }}</p>
    </div>

    <div class="flex items-center justify-between border-b border-panel-border">
      <nav class="flex gap-6">
        <button class="pb-3 text-sm font-medium transition" :class="tab === 'templates' ? 'border-b-2 border-purple-400 text-purple-400' : 'text-gray-400 hover:text-white'" @click="tab = 'templates'">
          {{ t('docker_apps_screen.tabs.templates') }}
        </button>
        <button class="pb-3 text-sm font-medium transition" :class="tab === 'installed' ? 'border-b-2 border-purple-400 text-purple-400' : 'text-gray-400 hover:text-white'" @click="tab = 'installed'">
          {{ t('docker_apps_screen.tabs.installed') }}
        </button>
        <button class="pb-3 text-sm font-medium transition" :class="tab === 'packages' ? 'border-b-2 border-purple-400 text-purple-400' : 'text-gray-400 hover:text-white'" @click="tab = 'packages'">
          {{ t('docker_apps_screen.tabs.packages') }}
        </button>
      </nav>
      <button class="mb-3 rounded-lg bg-panel-hover px-3 py-1.5 text-sm text-gray-300 transition hover:bg-gray-600" @click="loadData">
        {{ t('docker_apps_screen.refresh') }}
      </button>
    </div>

    <div v-if="tab === 'templates'" class="grid grid-cols-1 gap-4 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
      <div v-for="template in templates" :key="template.id" class="group flex flex-col justify-between rounded-xl border border-panel-border bg-panel-card p-5 transition-all duration-200 hover:border-purple-500/50">
        <div>
          <div class="mb-3 flex items-start justify-between">
            <span class="text-3xl">{{ template.icon || '[APP]' }}</span>
            <div class="flex flex-wrap justify-end gap-2">
              <span class="rounded bg-purple-500/15 px-2 py-0.5 text-xs font-medium text-purple-400">{{ template.category || t('docker_apps_screen.templates.default_icon') }}</span>
              <span v-if="template.provisioning" class="rounded bg-sky-500/15 px-2 py-0.5 text-xs font-medium uppercase tracking-wide text-sky-300">{{ template.provisioning }}</span>
            </div>
          </div>
          <h3 class="mb-1 text-lg font-semibold text-white">{{ template.name }}</h3>
          <p class="mb-4 line-clamp-2 text-sm leading-relaxed text-gray-400" :title="template.description">{{ template.description }}</p>
          <div class="mb-4 font-mono text-xs text-gray-500">{{ template.image }}</div>
          <div v-if="template.default_ports?.length" class="mb-4 text-xs text-gray-500">
            {{ t('docker_apps_screen.templates.default_ports') }}: {{ template.default_ports.join(', ') }}
          </div>
        </div>
        <button class="mt-2 w-full rounded-lg bg-purple-600/20 py-2 text-sm font-medium text-purple-400 transition-all duration-200 hover:bg-purple-600 hover:text-white" @click="openInstallModal(template)">
          {{ t('docker_apps_screen.templates.install') }}
        </button>
      </div>
      <div v-if="templates.length === 0" class="col-span-full py-8 text-center text-gray-500">
        {{ t('docker_apps_screen.templates.empty') }}
      </div>
    </div>

    <div v-if="tab === 'installed'" class="overflow-hidden rounded-xl border border-panel-border bg-panel-card">
      <div class="flex items-center justify-between border-b border-panel-border p-4">
        <h2 class="text-lg font-semibold text-white">{{ t('docker_apps_screen.installed.title') }}</h2>
      </div>
      <div class="overflow-x-auto">
        <table class="w-full text-sm">
          <thead>
            <tr class="border-b border-panel-border text-gray-400">
              <th class="px-4 py-3 text-left font-medium">{{ t('docker_apps_screen.installed.app') }}</th>
              <th class="px-4 py-3 text-left font-medium">{{ t('docker_apps_screen.installed.image') }}</th>
              <th class="px-4 py-3 text-left font-medium">{{ t('docker_apps_screen.installed.status') }}</th>
              <th class="px-4 py-3 text-left font-medium">{{ t('docker_apps_screen.installed.ports') }}</th>
              <th class="px-4 py-3 text-left font-medium">{{ t('docker_apps_screen.installed.package') }}</th>
              <th class="px-4 py-3 text-right font-medium">{{ t('docker_apps_screen.installed.action') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-if="installedApps.length === 0">
              <td colspan="6" class="px-4 py-4 text-center text-gray-500">{{ t('docker_apps_screen.installed.empty') }}</td>
            </tr>
            <tr v-for="app in installedApps" :key="app.name" class="border-b border-panel-border/50 transition hover:bg-panel-hover/30">
              <td class="px-4 py-3 font-medium text-white">{{ app.name }}</td>
              <td class="px-4 py-3 font-mono text-xs text-gray-400">{{ app.image }}</td>
              <td class="px-4 py-3">
                <span :class="['rounded-full px-2 py-1 text-xs font-medium', app.status.includes('Up') ? 'bg-green-500/20 text-green-400' : 'bg-red-500/20 text-red-400']">
                  {{ app.status.includes('Up') ? t('docker_apps_screen.installed.running') : t('docker_apps_screen.installed.stopped') }}
                </span>
              </td>
              <td class="px-4 py-3 font-mono text-xs text-gray-400">{{ app.ports || '-' }}</td>
              <td class="px-4 py-3">
                <span class="rounded bg-blue-500/15 px-2 py-0.5 text-xs text-blue-400">{{ app.package || t('docker_apps_screen.installed.unlimited') }}</span>
              </td>
              <td class="px-4 py-3 text-right">
                <button class="rounded bg-red-600/20 px-2 py-1 text-xs text-red-400 transition hover:bg-red-600/40" @click="removeApp(app.name)">
                  {{ t('docker_apps_screen.installed.remove') }}
                </button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <div v-if="tab === 'packages'" class="grid grid-cols-1 gap-5 md:grid-cols-3">
      <div v-if="packages.length === 0" class="col-span-full py-8 text-center text-gray-500">
        {{ t('docker_apps_screen.packages.empty') }}
      </div>
      <div v-for="pkg in packages" :key="pkg.id" class="rounded-xl border border-panel-border bg-panel-card p-6 text-center transition hover:border-purple-500/40">
        <div class="mb-3 text-4xl">{{ packageLabel(pkg.name) }}</div>
        <h3 class="mb-2 text-xl font-bold text-white">{{ pkg.name }}</h3>
        <div class="mb-5 space-y-2 text-sm text-gray-400">
          <div>{{ t('docker_apps_screen.packages.memory') }}: <span class="font-medium text-white">{{ pkg.memory_limit }}</span></div>
          <div>{{ t('docker_apps_screen.packages.cpu') }}: <span class="font-medium text-white">{{ pkg.cpu_limit }}</span></div>
          <div>{{ t('docker_apps_screen.packages.max_containers') }}: <span class="font-medium text-white">{{ pkg.max_containers || t('docker_apps_screen.packages.unlimited') }}</span></div>
        </div>
      </div>
    </div>

    <div v-if="showModal" class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm" @click.self="showModal = false">
      <div class="w-full max-w-lg rounded-2xl border border-panel-border bg-panel-card p-6 shadow-2xl">
        <h3 class="mb-1 text-xl font-bold text-white">{{ selectedTemplate?.icon }} {{ selectedTemplate?.name }} {{ t('docker_apps_screen.modal.title_suffix') }}</h3>
        <p class="mb-5 text-sm text-gray-400">{{ selectedTemplate?.description }}</p>
        <div class="space-y-4">
          <div>
            <label class="mb-1 block text-sm text-gray-400">{{ t('docker_apps_screen.modal.app_name') }}</label>
            <input v-model="installForm.app_name" type="text" :placeholder="t('docker_apps_screen.modal.app_name_placeholder', { id: selectedTemplate?.id || 'app' })" class="aura-input" />
          </div>
          <div>
            <label class="mb-1 block text-sm text-gray-400">{{ t('docker_apps_screen.modal.package') }}</label>
            <select v-model="installForm.package_id" class="aura-input">
              <option value="">{{ t('docker_apps_screen.modal.unlimited') }}</option>
              <option v-for="pkg in packages" :key="pkg.id" :value="pkg.id">{{ pkg.name }} ({{ pkg.memory_limit }} RAM, {{ pkg.cpu_limit }} CPU)</option>
            </select>
          </div>
          <div>
            <label class="mb-1 block text-sm text-gray-400">{{ t('docker_apps_screen.modal.extra_env') }}</label>
            <input v-model="installForm.custom_env_str" type="text" :placeholder="t('docker_apps_screen.modal.extra_env_placeholder')" class="aura-input" />
            <span class="mt-1 inline-block text-xs text-gray-500">{{ t('docker_apps_screen.modal.extra_env_hint') }}</span>
          </div>
        </div>
        <div class="mt-6 flex gap-3">
          <button class="flex flex-1 items-center justify-center rounded-lg bg-gradient-to-r from-purple-600 to-indigo-600 py-2.5 font-medium text-white transition hover:from-purple-700 hover:to-indigo-700" @click="installApp">
            <span v-if="installing" class="mr-2 h-5 w-5 animate-spin rounded-full border-2 border-white/30 border-t-white"></span>
            {{ installing ? t('docker_apps_screen.modal.installing') : t('docker_apps_screen.modal.install') }}
          </button>
          <button class="rounded-lg bg-panel-hover px-5 py-2.5 text-gray-300 transition hover:bg-gray-600" :disabled="installing" @click="showModal = false">
            {{ t('docker_apps_screen.modal.cancel') }}
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
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute } from 'vue-router'
import api from '../services/api'

const { t } = useI18n({ useScope: 'global' })
const route = useRoute()
const tab = ref(route.meta.dockerAppsTab || 'templates')
const showModal = ref(false)
const selectedTemplate = ref(null)
const notification = ref(null)
const installing = ref(false)

const templates = ref([])
const installedApps = ref([])
const packages = ref([])
const installForm = ref({ app_name: '', package_id: '', custom_env_str: '' })

const showNotif = (message, type = 'success') => {
  notification.value = { message, type }
  setTimeout(() => {
    notification.value = null
  }, 3000)
}

const packageLabel = (name) => {
  const lower = String(name || '').toLowerCase()
  if (lower.includes('start')) return 'S'
  if (lower.includes('pro')) return 'P'
  return 'B'
}

const openInstallModal = template => {
  selectedTemplate.value = template
  installForm.value = { app_name: `my-${template.id}`, package_id: '', custom_env_str: '' }
  showModal.value = true
}

const loadData = async () => {
  try {
    const res = await api.get('/docker/apps/templates')
    templates.value = res.data?.data || []
  } catch (err) {
    showNotif(err.response?.data?.error || t('docker_apps_screen.messages.templates_failed'), 'error')
  }

  try {
    const res = await api.get('/docker/apps/installed')
    installedApps.value = res.data?.data || []
  } catch {
    installedApps.value = []
  }

  try {
    const res = await api.get('/docker/packages')
    packages.value = res.data?.data || []
  } catch {
    packages.value = []
  }
}

const installApp = async () => {
  if (installing.value) return
  installing.value = true
  try {
    await api.post('/docker/apps/install', {
      template_id: selectedTemplate.value.id,
      app_name: installForm.value.app_name || `my-${selectedTemplate.value.id}`,
      package_id: installForm.value.package_id || null,
      custom_env: installForm.value.custom_env_str ? installForm.value.custom_env_str.split(',').map(item => item.trim()) : [],
    })
    showNotif(t('docker_apps_screen.messages.install_success', { name: selectedTemplate.value.name }))
    showModal.value = false
    tab.value = 'installed'
    loadData()
  } catch (err) {
    showNotif(err.response?.data?.error || t('docker_apps_screen.messages.install_failed'), 'error')
  } finally {
    installing.value = false
  }
}

const removeApp = async appName => {
  if (!window.confirm(t('docker_apps_screen.messages.remove_confirm', { name: appName }))) return
  try {
    await api.post('/docker/apps/remove', { app_name: appName })
    showNotif(t('docker_apps_screen.messages.remove_success', { name: appName }))
    loadData()
  } catch (err) {
    showNotif(err.response?.data?.error || t('docker_apps_screen.messages.remove_failed'), 'error')
  }
}

onMounted(loadData)
</script>
