<template>
  <div>
    <h2 class="text-2xl font-bold mb-6">{{ $t('gitdeploy.title') }}</h2>

    <div v-if="loading" class="text-gray-500">
      {{ $t('gitdeploy.loading') }}
    </div>

    <div v-else-if="!gitConfigured" class="bg-white p-6 rounded-lg shadow-sm border border-gray-200 dark:bg-gray-800 dark:border-gray-700">
      <h3 class="text-lg font-medium mb-4">{{ $t('gitdeploy.setup_title') }}</h3>
      <p class="text-gray-600 dark:text-gray-400 mb-6">
        {{ $t('gitdeploy.setup_desc') }}
      </p>

      <form @submit.prevent="setupGit" class="space-y-4 max-w-2xl">
        <div>
          <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">{{ $t('gitdeploy.repo_url') }}</label>
          <input v-model="form.repo_url" type="text" :placeholder="$t('gitdeploy.repo_url_placeholder')" required
                 class="w-full px-3 py-2 border rounded-md dark:bg-gray-700 dark:border-gray-600 focus:ring-2 focus:ring-blue-500">
          <p class="text-xs text-gray-500 mt-1">{{ $t('gitdeploy.repo_url_help') }}</p>
        </div>

        <div>
          <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">{{ $t('gitdeploy.branch') }}</label>
          <input v-model="form.branch" type="text" :placeholder="$t('gitdeploy.branch_placeholder')" required
                 class="w-full px-3 py-2 border rounded-md dark:bg-gray-700 dark:border-gray-600 focus:ring-2 focus:ring-blue-500">
        </div>

        <div>
          <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">{{ $t('gitdeploy.deploy_path') }}</label>
          <input v-model="form.deploy_path" type="text" :placeholder="$t('gitdeploy.deploy_path_placeholder')" required
                 class="w-full px-3 py-2 border rounded-md dark:bg-gray-700 dark:border-gray-600 focus:ring-2 focus:ring-blue-500">
          <p class="text-xs text-gray-500 mt-1">{{ $t('gitdeploy.deploy_path_help') }}</p>
        </div>

        <div>
          <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">{{ $t('gitdeploy.deploy_script') }}</label>
          <textarea v-model="form.deploy_script" rows="4" :placeholder="$t('gitdeploy.deploy_script_placeholder')"
                    class="w-full px-3 py-2 border rounded-md dark:bg-gray-700 dark:border-gray-600 font-mono text-sm focus:ring-2 focus:ring-blue-500"></textarea>
          <p class="text-xs text-gray-500 mt-1">{{ $t('gitdeploy.deploy_script_help') }}</p>
        </div>

        <button type="submit" class="bg-blue-600 text-white px-4 py-2 rounded-md hover:bg-blue-700 transition-colors">
          {{ $t('gitdeploy.save_config') }}
        </button>
      </form>
    </div>

    <div v-else class="space-y-6">
      <div class="bg-white p-6 rounded-lg shadow-sm border border-gray-200 dark:bg-gray-800 dark:border-gray-700">
        <div class="flex justify-between items-start mb-6">
          <div>
            <h3 class="text-lg font-medium mb-1">{{ $t('gitdeploy.active_title') }}</h3>
            <p class="text-sm text-gray-500">{{ $t('gitdeploy.repository') }}: {{ gitInfo.repo_url.replace(/:[^:@]+@/, ':***@') }}</p>
          </div>
          <div class="flex space-x-2">
            <button @click="deploy" :disabled="deploying"
                    class="bg-blue-600 text-white px-4 py-2 rounded-md hover:bg-blue-700 transition-colors disabled:opacity-50">
              {{ deploying ? $t('gitdeploy.deploying') : $t('gitdeploy.deploy_now') }}
            </button>
            <button @click="deleteGit" class="bg-red-100 text-red-600 px-4 py-2 rounded-md hover:bg-red-200 transition-colors">
              {{ $t('gitdeploy.remove') }}
            </button>
          </div>
        </div>

        <div class="grid grid-cols-2 gap-4 mb-6">
          <div>
            <span class="block text-sm text-gray-500">{{ $t('gitdeploy.branch') }}</span>
            <span class="font-medium">{{ gitInfo.branch }}</span>
          </div>
          <div>
            <span class="block text-sm text-gray-500">{{ $t('gitdeploy.status') }}</span>
            <span :class="{'text-yellow-600': gitInfo.status === 'pending' || gitInfo.status === 'deploying', 'text-green-600': gitInfo.status === 'success', 'text-red-600': gitInfo.status === 'failed'}" class="font-medium capitalize">
              {{ gitInfo.status }}
            </span>
          </div>
          <div>
            <span class="block text-sm text-gray-500">{{ $t('gitdeploy.deploy_path') }}</span>
            <span class="font-medium">{{ gitInfo.deploy_path }}</span>
          </div>
          <div>
            <span class="block text-sm text-gray-500">{{ $t('gitdeploy.last_deployed') }}</span>
            <span class="font-medium">{{ gitInfo.last_deployed_at || $t('gitdeploy.never') }}</span>
          </div>
        </div>

        <div class="mt-6 border-t pt-4 dark:border-gray-700">
          <h4 class="text-sm font-medium mb-2">{{ $t('gitdeploy.webhook_url') }}</h4>
          <p class="text-sm text-gray-500 mb-2">{{ $t('gitdeploy.webhook_desc') }}</p>
          <div class="flex">
            <input type="text" readonly :value="webhookUrl" class="w-full px-3 py-2 bg-gray-100 border rounded-l-md text-sm font-mono dark:bg-gray-900 dark:border-gray-700">
            <button @click="copyWebhook" class="bg-gray-200 px-4 py-2 rounded-r-md hover:bg-gray-300 text-sm font-medium dark:bg-gray-700 dark:hover:bg-gray-600 transition-colors">
              {{ $t('gitdeploy.copy') }}
            </button>
          </div>
        </div>
      </div>
      
      <div class="bg-white p-6 rounded-lg shadow-sm border border-gray-200 dark:bg-gray-800 dark:border-gray-700">
        <h3 class="text-lg font-medium mb-4">{{ $t('gitdeploy.edit_config') }}</h3>
        <form @submit.prevent="setupGit" class="space-y-4 max-w-2xl">
          <div>
            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">{{ $t('gitdeploy.repo_url') }}</label>
            <input v-model="form.repo_url" type="text" required
                   class="w-full px-3 py-2 border rounded-md dark:bg-gray-700 dark:border-gray-600 focus:ring-2 focus:ring-blue-500">
          </div>
          <div>
            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">{{ $t('gitdeploy.branch') }}</label>
            <input v-model="form.branch" type="text" required
                   class="w-full px-3 py-2 border rounded-md dark:bg-gray-700 dark:border-gray-600 focus:ring-2 focus:ring-blue-500">
          </div>
          <div>
            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">{{ $t('gitdeploy.deploy_path') }}</label>
            <input v-model="form.deploy_path" type="text" required
                   class="w-full px-3 py-2 border rounded-md dark:bg-gray-700 dark:border-gray-600 focus:ring-2 focus:ring-blue-500">
          </div>
          <div>
            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">{{ $t('gitdeploy.deploy_script') }}</label>
            <textarea v-model="form.deploy_script" rows="4"
                      class="w-full px-3 py-2 border rounded-md dark:bg-gray-700 dark:border-gray-600 font-mono text-sm focus:ring-2 focus:ring-blue-500"></textarea>
          </div>
          <button type="submit" class="bg-blue-600 text-white px-4 py-2 rounded-md hover:bg-blue-700 transition-colors">
            {{ $t('gitdeploy.update_config') }}
          </button>
        </form>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted, computed } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { api } from '../api'

const { t } = useI18n()
const route = useRoute()
const siteId = route.params.id

const loading = ref(true)
const gitConfigured = ref(false)
const gitInfo = ref({})
const deploying = ref(false)
let pollInterval = null

const form = ref({
  repo_url: '',
  branch: 'main',
  deploy_path: '/',
  deploy_script: 'composer install --no-dev\nnpm install\nnpm run build'
})

const webhookUrl = computed(() => {
  if (!gitInfo.value.webhook_secret) return ''
  const protocol = window.location.protocol
  const host = window.location.host
  return `${protocol}//${host}/api/v1/webhooks/git/${gitInfo.value.webhook_secret}`
})

const fetchGitStatus = async () => {
  try {
    const res = await api.get(`/sites/${siteId}/git`)
    gitConfigured.value = res.configured
    if (res.configured) {
      gitInfo.value = res
      form.value = {
        repo_url: res.repo_url,
        branch: res.branch,
        deploy_path: res.deploy_path,
        deploy_script: res.deploy_script
      }
      if (res.status === 'deploying') {
        deploying.value = true
        startPolling()
      } else {
        deploying.value = false
        stopPolling()
      }
    } else {
      stopPolling()
    }
  } catch (error) {
    console.error('Failed to fetch git status:', error)
  } finally {
    loading.value = false
  }
}

const setupGit = async () => {
  try {
    await api.post(`/sites/${siteId}/git`, form.value)
    alert(t('gitdeploy.config_saved'))
    await fetchGitStatus()
  } catch (error) {
    alert(`${t('gitdeploy.config_failed')}: ${error.message}`)
  }
}

const deploy = async () => {
  try {
    await api.post(`/sites/${siteId}/git/deploy`)
    deploying.value = true
    startPolling()
  } catch (error) {
    alert(`${t('gitdeploy.deploy_failed')}: ${error.message}`)
  }
}

const deleteGit = async () => {
  if (!confirm(t('gitdeploy.confirm_remove'))) return
  try {
    await api.delete(`/sites/${siteId}/git`)
    gitConfigured.value = false
    gitInfo.value = {}
  } catch (error) {
    alert(`${t('gitdeploy.remove_failed')}: ${error.message}`)
  }
}

const copyWebhook = async () => {
  try {
    await navigator.clipboard.writeText(webhookUrl.value)
    alert(t('gitdeploy.webhook_copied'))
  } catch (e) {
    alert(t('gitdeploy.copy_failed'))
  }
}

const startPolling = () => {
  if (pollInterval) return
  pollInterval = setInterval(fetchGitStatus, 3000)
}

const stopPolling = () => {
  if (pollInterval) {
    clearInterval(pollInterval)
    pollInterval = null
  }
}

onMounted(() => {
  fetchGitStatus()
})

onUnmounted(() => {
  stopPolling()
})
</script>
