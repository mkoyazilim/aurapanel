<template>
  <div>
    <h2 class="text-2xl font-bold mb-6">Git Deploy</h2>

    <div v-if="loading" class="text-gray-500">
      Loading...
    </div>

    <div v-else-if="!gitConfigured" class="bg-white p-6 rounded-lg shadow-sm border border-gray-200 dark:bg-gray-800 dark:border-gray-700">
      <h3 class="text-lg font-medium mb-4">Setup Git Deployment</h3>
      <p class="text-gray-600 dark:text-gray-400 mb-6">
        Connect this site to a Git repository to enable automated deployments.
      </p>

      <form @submit.prevent="setupGit" class="space-y-4 max-w-2xl">
        <div>
          <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Repository URL</label>
          <input v-model="form.repo_url" type="text" placeholder="https://x-access-token:ghp_xxx@github.com/user/repo.git" required
                 class="w-full px-3 py-2 border rounded-md dark:bg-gray-700 dark:border-gray-600 focus:ring-2 focus:ring-blue-500">
          <p class="text-xs text-gray-500 mt-1">For private repos, include your Personal Access Token (PAT) in the URL.</p>
        </div>

        <div>
          <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Branch</label>
          <input v-model="form.branch" type="text" placeholder="main" required
                 class="w-full px-3 py-2 border rounded-md dark:bg-gray-700 dark:border-gray-600 focus:ring-2 focus:ring-blue-500">
        </div>

        <div>
          <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Deploy Path</label>
          <input v-model="form.deploy_path" type="text" placeholder="/" required
                 class="w-full px-3 py-2 border rounded-md dark:bg-gray-700 dark:border-gray-600 focus:ring-2 focus:ring-blue-500">
          <p class="text-xs text-gray-500 mt-1">Relative to the site's home directory. E.g. "/", "/public_html", "/app"</p>
        </div>

        <div>
          <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Deploy Script</label>
          <textarea v-model="form.deploy_script" rows="4" placeholder="npm install && npm run build"
                    class="w-full px-3 py-2 border rounded-md dark:bg-gray-700 dark:border-gray-600 font-mono text-sm focus:ring-2 focus:ring-blue-500"></textarea>
          <p class="text-xs text-gray-500 mt-1">Executed in the deploy path after pull/clone.</p>
        </div>

        <button type="submit" class="bg-blue-600 text-white px-4 py-2 rounded-md hover:bg-blue-700 transition-colors">
          Save Configuration
        </button>
      </form>
    </div>

    <div v-else class="space-y-6">
      <div class="bg-white p-6 rounded-lg shadow-sm border border-gray-200 dark:bg-gray-800 dark:border-gray-700">
        <div class="flex justify-between items-start mb-6">
          <div>
            <h3 class="text-lg font-medium mb-1">Git Deployment Active</h3>
            <p class="text-sm text-gray-500">Repository: {{ gitInfo.repo_url.replace(/:[^:@]+@/, ':***@') }}</p>
          </div>
          <div class="flex space-x-2">
            <button @click="deploy" :disabled="deploying"
                    class="bg-blue-600 text-white px-4 py-2 rounded-md hover:bg-blue-700 transition-colors disabled:opacity-50">
              {{ deploying ? 'Deploying...' : 'Deploy Now' }}
            </button>
            <button @click="deleteGit" class="bg-red-100 text-red-600 px-4 py-2 rounded-md hover:bg-red-200 transition-colors">
              Remove
            </button>
          </div>
        </div>

        <div class="grid grid-cols-2 gap-4 mb-6">
          <div>
            <span class="block text-sm text-gray-500">Branch</span>
            <span class="font-medium">{{ gitInfo.branch }}</span>
          </div>
          <div>
            <span class="block text-sm text-gray-500">Status</span>
            <span :class="{'text-yellow-600': gitInfo.status === 'pending' || gitInfo.status === 'deploying', 'text-green-600': gitInfo.status === 'success', 'text-red-600': gitInfo.status === 'failed'}" class="font-medium capitalize">
              {{ gitInfo.status }}
            </span>
          </div>
          <div>
            <span class="block text-sm text-gray-500">Deploy Path</span>
            <span class="font-medium">{{ gitInfo.deploy_path }}</span>
          </div>
          <div>
            <span class="block text-sm text-gray-500">Last Deployed</span>
            <span class="font-medium">{{ gitInfo.last_deployed_at || 'Never' }}</span>
          </div>
        </div>

        <div class="mt-6 border-t pt-4 dark:border-gray-700">
          <h4 class="text-sm font-medium mb-2">Webhook URL</h4>
          <p class="text-sm text-gray-500 mb-2">Configure this URL in your Git provider (GitHub, GitLab, etc.) to trigger automatic deployments.</p>
          <div class="flex">
            <input type="text" readonly :value="webhookUrl" class="w-full px-3 py-2 bg-gray-100 border rounded-l-md text-sm font-mono dark:bg-gray-900 dark:border-gray-700">
            <button @click="copyWebhook" class="bg-gray-200 px-4 py-2 rounded-r-md hover:bg-gray-300 text-sm font-medium dark:bg-gray-700 dark:hover:bg-gray-600 transition-colors">
              Copy
            </button>
          </div>
        </div>
      </div>
      
      <div class="bg-white p-6 rounded-lg shadow-sm border border-gray-200 dark:bg-gray-800 dark:border-gray-700">
        <h3 class="text-lg font-medium mb-4">Edit Configuration</h3>
        <form @submit.prevent="setupGit" class="space-y-4 max-w-2xl">
          <div>
            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Repository URL</label>
            <input v-model="form.repo_url" type="text" required
                   class="w-full px-3 py-2 border rounded-md dark:bg-gray-700 dark:border-gray-600 focus:ring-2 focus:ring-blue-500">
          </div>
          <div>
            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Branch</label>
            <input v-model="form.branch" type="text" required
                   class="w-full px-3 py-2 border rounded-md dark:bg-gray-700 dark:border-gray-600 focus:ring-2 focus:ring-blue-500">
          </div>
          <div>
            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Deploy Path</label>
            <input v-model="form.deploy_path" type="text" required
                   class="w-full px-3 py-2 border rounded-md dark:bg-gray-700 dark:border-gray-600 focus:ring-2 focus:ring-blue-500">
          </div>
          <div>
            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Deploy Script</label>
            <textarea v-model="form.deploy_script" rows="4"
                      class="w-full px-3 py-2 border rounded-md dark:bg-gray-700 dark:border-gray-600 font-mono text-sm focus:ring-2 focus:ring-blue-500"></textarea>
          </div>
          <button type="submit" class="bg-blue-600 text-white px-4 py-2 rounded-md hover:bg-blue-700 transition-colors">
            Update Configuration
          </button>
        </form>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted, computed } from 'vue'
import { useRoute } from 'vue-router'
import { api } from '../api'

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
    alert('Git configuration saved successfully')
    await fetchGitStatus()
  } catch (error) {
    alert(`Failed to save git configuration: ${error.message}`)
  }
}

const deploy = async () => {
  try {
    await api.post(`/sites/${siteId}/git/deploy`)
    deploying.value = true
    startPolling()
  } catch (error) {
    alert(`Deploy failed: ${error.message}`)
  }
}

const deleteGit = async () => {
  if (!confirm('Are you sure you want to remove Git integration for this site?')) return
  try {
    await api.delete(`/sites/${siteId}/git`)
    gitConfigured.value = false
    gitInfo.value = {}
  } catch (error) {
    alert(`Failed to remove Git: ${error.message}`)
  }
}

const copyWebhook = async () => {
  try {
    await navigator.clipboard.writeText(webhookUrl.value)
    alert('Webhook URL copied to clipboard')
  } catch (e) {
    alert('Failed to copy. Please manually copy the URL.')
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
