<template>
  <div class="space-y-6">
    <div class="flex items-center justify-between">
      <div>
        <h1 class="flex items-center gap-3 text-2xl font-bold text-white">
          <svg class="h-7 w-7 text-blue-400" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 3v4M3 5h4M6 17v4m-2-2h4m5-16l2.286 6.857L21 12l-5.714 2.143L13 21l-2.286-6.857L5 12l5.714-2.143L13 3z"/></svg>
          {{ t('docker_manager_screen.title') }}
        </h1>
        <p class="mt-1 text-gray-400">{{ t('docker_manager_screen.subtitle') }}</p>
      </div>
    </div>

    <div class="grid grid-cols-1 gap-4 md:grid-cols-3">
      <div class="rounded-xl border border-panel-border bg-panel-card p-5">
        <div class="flex items-center justify-between">
          <div>
            <p class="text-sm text-gray-400">{{ t('docker_manager_screen.stats.running') }}</p>
            <p class="mt-1 text-2xl font-bold text-green-400">{{ runningCount }}</p>
          </div>
          <div class="rounded-lg bg-green-500/10 p-3">
            <svg class="h-6 w-6 text-green-400" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7"/></svg>
          </div>
        </div>
      </div>
      <div class="rounded-xl border border-panel-border bg-panel-card p-5">
        <div class="flex items-center justify-between">
          <div>
            <p class="text-sm text-gray-400">{{ t('docker_manager_screen.stats.stopped') }}</p>
            <p class="mt-1 text-2xl font-bold text-red-400">{{ stoppedCount }}</p>
          </div>
          <div class="rounded-lg bg-red-500/10 p-3">
            <svg class="h-6 w-6 text-red-400" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"/></svg>
          </div>
        </div>
      </div>
      <div class="rounded-xl border border-panel-border bg-panel-card p-5">
        <div class="flex items-center justify-between">
          <div>
            <p class="text-sm text-gray-400">{{ t('docker_manager_screen.stats.images') }}</p>
            <p class="mt-1 text-2xl font-bold text-blue-400">{{ images.length }}</p>
          </div>
          <div class="rounded-lg bg-blue-500/10 p-3">
            <svg class="h-6 w-6 text-blue-400" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 7v10c0 2.21 3.582 4 8 4s8-1.79 8-4V7M4 7c0 2.21 3.582 4 8 4s8-1.79 8-4M4 7c0-2.21 3.582-4 8-4s8 1.79 8 4"/></svg>
          </div>
        </div>
      </div>
    </div>

    <div class="border-b border-panel-border">
      <nav class="flex gap-6">
        <button class="pb-3 text-sm font-medium transition-colors" :class="activeTab === 'containers' ? 'border-b-2 border-blue-400 text-blue-400' : 'text-gray-400 hover:text-white'" @click="activeTab = 'containers'">
          {{ t('docker_manager_screen.tabs.containers') }}
        </button>
        <button class="pb-3 text-sm font-medium transition-colors" :class="activeTab === 'images' ? 'border-b-2 border-blue-400 text-blue-400' : 'text-gray-400 hover:text-white'" @click="activeTab = 'images'">
          {{ t('docker_manager_screen.tabs.images') }}
        </button>
        <button class="pb-3 text-sm font-medium transition-colors" :class="activeTab === 'create' ? 'border-b-2 border-blue-400 text-blue-400' : 'text-gray-400 hover:text-white'" @click="activeTab = 'create'">
          {{ t('docker_manager_screen.tabs.create') }}
        </button>
      </nav>
    </div>

    <div v-if="activeTab === 'containers'" class="overflow-hidden rounded-xl border border-panel-border bg-panel-card">
      <div class="flex items-center justify-between border-b border-panel-border p-4">
        <h2 class="text-lg font-semibold text-white">{{ t('docker_manager_screen.containers.title') }}</h2>
        <button class="rounded-lg bg-panel-hover px-3 py-1.5 text-sm text-gray-300 transition hover:bg-gray-600 disabled:cursor-not-allowed disabled:opacity-60" :disabled="isRefreshingContainers" @click="refreshContainers()">
          {{ t('docker_manager_screen.containers.refresh') }}
        </button>
      </div>
      <div class="overflow-x-auto">
        <table class="w-full text-sm">
          <thead>
            <tr class="border-b border-panel-border text-gray-400">
              <th class="px-4 py-3 text-left font-medium">{{ t('docker_manager_screen.containers.name') }}</th>
              <th class="px-4 py-3 text-left font-medium">{{ t('docker_manager_screen.containers.image') }}</th>
              <th class="px-4 py-3 text-left font-medium">{{ t('docker_manager_screen.containers.status') }}</th>
              <th class="px-4 py-3 text-left font-medium">{{ t('docker_manager_screen.containers.ports') }}</th>
              <th class="px-4 py-3 text-left font-medium">{{ t('docker_manager_screen.containers.created') }}</th>
              <th class="px-4 py-3 text-right font-medium">{{ t('docker_manager_screen.containers.actions') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="container in containers" :key="container.id" class="border-b border-panel-border/50 transition hover:bg-panel-hover/30">
              <td class="px-4 py-3 font-medium text-white">{{ container.name }}</td>
              <td class="px-4 py-3 text-gray-300">{{ container.image }}</td>
              <td class="px-4 py-3">
                <span :class="['rounded-full px-2 py-1 text-xs font-medium', container.status.includes('Up') ? 'bg-green-500/20 text-green-400' : 'bg-red-500/20 text-red-400']">
                  {{ container.status.includes('Up') ? t('docker_manager_screen.containers.running') : t('docker_manager_screen.containers.stopped') }}
                </span>
              </td>
              <td class="px-4 py-3 font-mono text-xs text-gray-400">{{ container.ports || t('docker_manager_screen.images.empty_ports') }}</td>
              <td class="px-4 py-3 text-xs text-gray-400">{{ container.created }}</td>
              <td class="space-x-1 px-4 py-3 text-right">
                <button v-if="!container.status.includes('Up')" class="rounded bg-green-600/20 px-2 py-1 text-xs text-green-400 transition hover:bg-green-600/40" @click="containerAction(container.id, 'start')">
                  {{ t('docker_manager_screen.containers.start') }}
                </button>
                <button v-if="container.status.includes('Up')" class="rounded bg-yellow-600/20 px-2 py-1 text-xs text-yellow-400 transition hover:bg-yellow-600/40" @click="containerAction(container.id, 'stop')">
                  {{ t('docker_manager_screen.containers.stop') }}
                </button>
                <button class="rounded bg-blue-600/20 px-2 py-1 text-xs text-blue-400 transition hover:bg-blue-600/40" @click="containerAction(container.id, 'restart')">
                  {{ t('docker_manager_screen.containers.restart') }}
                </button>
                <button class="rounded bg-red-600/20 px-2 py-1 text-xs text-red-400 transition hover:bg-red-600/40" @click="containerAction(container.id, 'remove')">
                  {{ t('docker_manager_screen.containers.remove') }}
                </button>
              </td>
            </tr>
            <tr v-if="containers.length === 0">
              <td colspan="6" class="px-4 py-6 text-center text-gray-500">{{ t('docker_manager_screen.containers.empty') }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <div v-if="activeTab === 'images'" class="overflow-hidden rounded-xl border border-panel-border bg-panel-card">
      <div class="flex items-center justify-between border-b border-panel-border p-4">
        <h2 class="text-lg font-semibold text-white">{{ t('docker_manager_screen.images.title') }}</h2>
        <div class="flex gap-2">
          <button class="rounded-lg bg-panel-hover px-3 py-1.5 text-sm text-gray-300 transition hover:bg-gray-600 disabled:cursor-not-allowed disabled:opacity-60" :disabled="isRefreshingImages" @click="refreshImages()">
            {{ t('docker_manager_screen.containers.refresh') }}
          </button>
          <input v-model="pullImageName" type="text" class="w-48 rounded-lg border border-panel-border bg-panel-hover px-3 py-1.5 text-sm text-white placeholder-gray-500 focus:border-blue-500 focus:outline-none" :placeholder="t('docker_manager_screen.images.pull_placeholder')" />
          <button class="rounded-lg bg-blue-600 px-4 py-1.5 text-sm text-white transition hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-60" :disabled="isPullingImage" @click="pullImage">
            {{ t('docker_manager_screen.images.pull') }}
          </button>
        </div>
      </div>
      <div class="overflow-x-auto">
        <table class="w-full text-sm">
          <thead>
            <tr class="border-b border-panel-border text-gray-400">
              <th class="px-4 py-3 text-left font-medium">{{ t('docker_manager_screen.images.repository') }}</th>
              <th class="px-4 py-3 text-left font-medium">{{ t('docker_manager_screen.images.tag') }}</th>
              <th class="px-4 py-3 text-left font-medium">{{ t('docker_manager_screen.images.id') }}</th>
              <th class="px-4 py-3 text-left font-medium">{{ t('docker_manager_screen.images.size') }}</th>
              <th class="px-4 py-3 text-left font-medium">{{ t('docker_manager_screen.images.created') }}</th>
              <th class="px-4 py-3 text-right font-medium">{{ t('docker_manager_screen.images.action') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="image in images" :key="image.id" class="border-b border-panel-border/50 transition hover:bg-panel-hover/30">
              <td class="px-4 py-3 font-medium text-white">{{ image.repository }}</td>
              <td class="px-4 py-3"><span class="rounded bg-blue-500/20 px-2 py-0.5 text-xs text-blue-400">{{ image.tag }}</span></td>
              <td class="px-4 py-3 font-mono text-xs text-gray-400">{{ image.id.substring(0, 12) }}</td>
              <td class="px-4 py-3 text-gray-300">{{ image.size }}</td>
              <td class="px-4 py-3 text-xs text-gray-400">{{ image.created }}</td>
              <td class="px-4 py-3 text-right">
                <button class="rounded bg-red-600/20 px-2 py-1 text-xs text-red-400 transition hover:bg-red-600/40" @click="removeImage(image.id)">
                  {{ t('docker_manager_screen.containers.remove') }}
                </button>
              </td>
            </tr>
            <tr v-if="images.length === 0">
              <td colspan="6" class="px-4 py-6 text-center text-gray-500">{{ t('docker_manager_screen.containers.empty') }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <div v-if="activeTab === 'create'" class="rounded-xl border border-panel-border bg-panel-card p-6">
      <h2 class="mb-6 text-lg font-semibold text-white">{{ t('docker_manager_screen.create.title') }}</h2>
      <form class="space-y-5" @submit.prevent="createContainer">
        <div class="grid grid-cols-1 gap-5 md:grid-cols-2">
          <div>
            <label class="mb-1.5 block text-sm text-gray-400">{{ t('docker_manager_screen.create.name') }}</label>
            <input v-model="newContainer.name" type="text" placeholder="my-app" class="w-full rounded-lg border border-panel-border bg-panel-hover px-4 py-2.5 text-white placeholder-gray-500 focus:border-blue-500 focus:outline-none" />
          </div>
          <div>
            <label class="mb-1.5 block text-sm text-gray-400">{{ t('docker_manager_screen.create.image') }}</label>
            <input v-model="newContainer.image" type="text" placeholder="nginx:latest" class="w-full rounded-lg border border-panel-border bg-panel-hover px-4 py-2.5 text-white placeholder-gray-500 focus:border-blue-500 focus:outline-none" />
          </div>
          <div>
            <label class="mb-1.5 block text-sm text-gray-400">{{ t('docker_manager_screen.create.ports') }}</label>
            <input v-model="newContainer.portsStr" type="text" placeholder="80:80, 443:443" class="w-full rounded-lg border border-panel-border bg-panel-hover px-4 py-2.5 text-white placeholder-gray-500 focus:border-blue-500 focus:outline-none" />
          </div>
          <div>
            <label class="mb-1.5 block text-sm text-gray-400">{{ t('docker_manager_screen.create.env') }}</label>
            <input v-model="newContainer.envStr" type="text" placeholder="MYSQL_ROOT_PASSWORD=secret" class="w-full rounded-lg border border-panel-border bg-panel-hover px-4 py-2.5 text-white placeholder-gray-500 focus:border-blue-500 focus:outline-none" />
          </div>
          <div>
            <label class="mb-1.5 block text-sm text-gray-400">{{ t('docker_manager_screen.create.volumes') }}</label>
            <input v-model="newContainer.volumesStr" type="text" placeholder="/data:/var/lib/mysql" class="w-full rounded-lg border border-panel-border bg-panel-hover px-4 py-2.5 text-white placeholder-gray-500 focus:border-blue-500 focus:outline-none" />
          </div>
          <div>
            <label class="mb-1.5 block text-sm text-gray-400">{{ t('docker_manager_screen.create.restart_policy') }}</label>
            <select v-model="newContainer.restart_policy" class="w-full rounded-lg border border-panel-border bg-panel-hover px-4 py-2.5 text-white focus:border-blue-500 focus:outline-none">
              <option value="">{{ t('docker_manager_screen.create.none') }}</option>
              <option value="always">Always</option>
              <option value="unless-stopped">Unless Stopped</option>
              <option value="on-failure">On Failure</option>
            </select>
          </div>
          <div>
            <label class="mb-1.5 block text-sm text-gray-400">{{ t('docker_manager_screen.create.memory') }}</label>
            <input v-model="newContainer.memory_limit" type="text" placeholder="512m or 1g" class="w-full rounded-lg border border-panel-border bg-panel-hover px-4 py-2.5 text-white placeholder-gray-500 focus:border-blue-500 focus:outline-none" />
          </div>
          <div>
            <label class="mb-1.5 block text-sm text-gray-400">{{ t('docker_manager_screen.create.cpu') }}</label>
            <input v-model="newContainer.cpu_limit" type="text" placeholder="0.5 or 1.0" class="w-full rounded-lg border border-panel-border bg-panel-hover px-4 py-2.5 text-white placeholder-gray-500 focus:border-blue-500 focus:outline-none" />
          </div>
        </div>
        <div class="pt-4">
          <button type="submit" class="rounded-lg bg-gradient-to-r from-blue-600 to-indigo-600 px-6 py-2.5 font-medium text-white shadow-lg shadow-blue-500/25 transition-all duration-200 hover:from-blue-700 hover:to-indigo-700">
            {{ t('docker_manager_screen.create.button') }}
          </button>
        </div>
      </form>
    </div>

    <div v-if="notification" :class="['fixed bottom-6 right-6 z-50 rounded-xl px-5 py-3 text-sm font-medium text-white shadow-2xl', notification.type === 'success' ? 'bg-green-600' : 'bg-red-600']">
      {{ notification.message }}
    </div>
  </div>
</template>

<script setup>
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute } from 'vue-router'
import api from '../services/api'

const { t } = useI18n({ useScope: 'global' })
const route = useRoute()
const activeTab = ref(route.meta.dockerTab || 'containers')
const containers = ref([])
const images = ref([])
const pullImageName = ref('')
const notification = ref(null)
const isRefreshingContainers = ref(false)
const isRefreshingImages = ref(false)
const isPullingImage = ref(false)
let refreshInterval = null

const newContainer = ref({
  name: '',
  image: '',
  portsStr: '',
  envStr: '',
  volumesStr: '',
  restart_policy: '',
  memory_limit: '',
  cpu_limit: '',
})

const runningCount = computed(() => (Array.isArray(containers.value) ? containers.value : []).filter(container => String(container.status || '').includes('Up')).length)
const stoppedCount = computed(() => (Array.isArray(containers.value) ? containers.value : []).filter(container => !String(container.status || '').includes('Up')).length)

const showNotif = (message, type = 'success') => {
  notification.value = { message, type }
  setTimeout(() => {
    notification.value = null
  }, 3000)
}

const refreshContainers = async ({ silent = false } = {}) => {
  isRefreshingContainers.value = true
  try {
    const { data } = await api.get('/docker/containers', {
      params: { _ts: Date.now() },
      headers: { 'Cache-Control': 'no-cache' },
    })
    containers.value = Array.isArray(data.data) ? data.data : []
  } catch (err) {
    if (!silent) {
      showNotif(err.response?.data?.error || t('docker_manager_screen.messages.containers_failed'), 'error')
    }
    containers.value = []
  } finally {
    isRefreshingContainers.value = false
  }
}

const refreshImages = async ({ silent = false } = {}) => {
  isRefreshingImages.value = true
  try {
    const { data } = await api.get('/docker/images', {
      params: { _ts: Date.now() },
      headers: { 'Cache-Control': 'no-cache' },
    })
    images.value = Array.isArray(data.data) ? data.data : []
  } catch (err) {
    if (!silent) {
      showNotif(err.response?.data?.error || t('docker_manager_screen.messages.images_failed'), 'error')
    }
    images.value = []
  } finally {
    isRefreshingImages.value = false
  }
}

const containerAction = async (id, action) => {
  try {
    if (action === 'start') {
      await api.post('/docker/containers/start', { id, action })
    } else if (action === 'stop') {
      await api.post('/docker/containers/stop', { id, action })
    } else if (action === 'restart') {
      await api.post('/docker/containers/restart', { id, action })
    } else if (action === 'remove') {
      await api.post('/docker/containers/remove', { id, action })
    } else {
      throw new Error(`Unsupported action: ${action}`)
    }
    showNotif(t('docker_manager_screen.messages.container_action_success', { action }))
    refreshContainers()
  } catch (err) {
    showNotif(err.response?.data?.error || t('docker_manager_screen.messages.container_action_failed', { action }), 'error')
  }
}

const pullImage = async () => {
  if (!pullImageName.value) return
  const { image, tag } = splitImageReference(pullImageName.value)
  isPullingImage.value = true
  try {
    await api.post('/docker/images/pull', { image, tag: tag || 'latest' })
    showNotif(t('docker_manager_screen.messages.image_pulled', { name: pullImageName.value }))
    pullImageName.value = ''
    await refreshImages()
  } catch (err) {
    showNotif(err.response?.data?.error || t('docker_manager_screen.messages.image_pull_failed'), 'error')
  } finally {
    isPullingImage.value = false
  }
}

const removeImage = async id => {
  if (!window.confirm(t('docker_manager_screen.messages.image_remove_confirm'))) return
  try {
    await api.post('/docker/images/remove', { id })
    showNotif(t('docker_manager_screen.messages.image_removed'))
    await refreshImages()
  } catch (err) {
    showNotif(err.response?.data?.error || t('docker_manager_screen.messages.image_remove_failed'), 'error')
  }
}

const createContainer = async () => {
  const payload = {
    name: newContainer.value.name,
    image: newContainer.value.image,
    ports: newContainer.value.portsStr ? newContainer.value.portsStr.split(',').map(item => item.trim()) : [],
    env: newContainer.value.envStr ? newContainer.value.envStr.split(',').map(item => item.trim()) : [],
    volumes: newContainer.value.volumesStr ? newContainer.value.volumesStr.split(',').map(item => item.trim()) : [],
    restart_policy: newContainer.value.restart_policy || null,
    memory_limit: newContainer.value.memory_limit || null,
    cpu_limit: newContainer.value.cpu_limit || null,
  }
  try {
    await api.post('/docker/containers/create', payload)
    showNotif(t('docker_manager_screen.messages.container_created', { name: payload.name }))
    newContainer.value = { name: '', image: '', portsStr: '', envStr: '', volumesStr: '', restart_policy: '', memory_limit: '', cpu_limit: '' }
    activeTab.value = 'containers'
    await refreshContainers()
  } catch (err) {
    showNotif(err.response?.data?.error || t('docker_manager_screen.messages.container_create_failed'), 'error')
  }
}

const splitImageReference = value => {
  const input = String(value || '').trim()
  if (!input) return { image: '', tag: 'latest' }
  const digestIndex = input.indexOf('@')
  if (digestIndex >= 0) {
    return { image: input.slice(0, digestIndex), tag: input.slice(digestIndex + 1) || 'latest' }
  }
  const lastColon = input.lastIndexOf(':')
  const lastSlash = input.lastIndexOf('/')
  if (lastColon > lastSlash) {
    return { image: input.slice(0, lastColon), tag: input.slice(lastColon + 1) || 'latest' }
  }
  return { image: input, tag: 'latest' }
}

const resetAutoRefresh = () => {
  if (refreshInterval) {
    clearInterval(refreshInterval)
  }
  refreshInterval = setInterval(() => {
    if (activeTab.value === 'containers') {
      refreshContainers({ silent: true })
      return
    }
    if (activeTab.value === 'images') {
      refreshImages({ silent: true })
    }
  }, 15000)
}

watch(() => route.meta.dockerTab, tab => {
  if (tab) {
    activeTab.value = tab
  }
})

watch(activeTab, tab => {
  if (tab === 'containers') {
    refreshContainers()
  } else if (tab === 'images') {
    refreshImages()
  }
})

onMounted(() => {
  refreshContainers()
  refreshImages()
  resetAutoRefresh()
})

onUnmounted(() => {
  if (refreshInterval) {
    clearInterval(refreshInterval)
    refreshInterval = null
  }
})
</script>
