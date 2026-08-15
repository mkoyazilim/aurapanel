<template>
  <Layout>
    <div class="page">
      <h1>{{ $t('gitdeploy.title') }}</h1>
      <div v-if="loading" class="muted">{{ $t('gitdeploy.loading') }}</div>
      <div v-else>
        <div v-if="error" class="alert error">{{ error }}</div>
        
        <div v-if="!gitConfigured" class="card text-center">
          <h2>{{ $t('gitdeploy.setup_title') }}</h2>
          <p class="muted" style="margin-bottom: 24px">{{ $t('gitdeploy.setup_desc') }}</p>
          <form @submit.prevent="saveConfig" style="text-align: left; max-width: 500px; margin: 0 auto;">
            <label>{{ $t('gitdeploy.repository') }}</label>
            <input v-model="form.repo_url" type="url" required :placeholder="$t('gitdeploy.repo_url_placeholder')" />
            <p class="muted" style="font-size: 12px; margin-top: 2px;">{{ $t('gitdeploy.repo_url_help') }}</p>
            
            <label style="margin-top: 16px">{{ $t('gitdeploy.branch') }}</label>
            <input v-model="form.branch" type="text" required :placeholder="$t('gitdeploy.branch_placeholder')" />
            
            <button type="submit" class="btn primary" style="width: 100%; margin-top: 24px" :disabled="submitting">
              {{ $t('gitdeploy.save_config') }}
            </button>
          </form>
        </div>

        <div v-else class="card">
          <div style="display: flex; justify-content: space-between; align-items: flex-start; margin-bottom: 16px;">
            <div>
              <h2>{{ $t('gitdeploy.active_title') }}</h2>
              <p><strong>{{ $t('gitdeploy.repository') }}:</strong> <span class="mono">{{ gitInfo.repo_url }}</span></p>
              <p><strong>{{ $t('gitdeploy.branch') }}:</strong> <span class="mono">{{ gitInfo.branch }}</span></p>
              <p><strong>{{ $t('gitdeploy.last_deployed') }}:</strong> {{ gitInfo.last_deployed_at ? new Date(gitInfo.last_deployed_at).toLocaleString() : $t('gitdeploy.never') }}</p>
            </div>
            <span class="badge" :class="gitInfo.status === 'active' ? 'ok' : 'error'">{{ gitInfo.status || 'active' }}</span>
          </div>

          <div style="background: var(--bg-body, #f8fafc); padding: 16px; border-radius: 6px; margin-bottom: 24px;">
            <strong>{{ $t('gitdeploy.webhook_url') }}</strong>
            <p class="muted text-sm">{{ $t('gitdeploy.webhook_desc') }}</p>
            <div style="display: flex; gap: 8px; margin-top: 8px;">
              <input type="text" readonly :value="webhookUrl" style="flex: 1" />
              <button @click="copyWebhook" class="btn">{{ $t('gitdeploy.copy') }}</button>
            </div>
          </div>

          <form @submit.prevent="saveConfig" style="margin-bottom: 24px; border-top: 1px solid var(--border-color, #e2e8f0); padding-top: 24px;">
            <h3>{{ $t('gitdeploy.edit_config') }}</h3>
            <label>{{ $t('gitdeploy.deploy_path') }}</label>
            <input v-model="form.deploy_path" type="text" required :placeholder="$t('gitdeploy.deploy_path_placeholder')" />
            <p class="muted" style="font-size: 12px; margin-top: 2px;">{{ $t('gitdeploy.deploy_path_help') }}</p>
            
            <label style="margin-top: 16px">{{ $t('gitdeploy.deploy_script') }}</label>
            <textarea v-model="form.deploy_script" rows="3" :placeholder="$t('gitdeploy.deploy_script_placeholder')"></textarea>
            <p class="muted" style="font-size: 12px; margin-top: 2px;">{{ $t('gitdeploy.deploy_script_help') }}</p>
            
            <div style="margin-top: 16px; display: flex; gap: 8px;">
              <button type="submit" class="btn primary" :disabled="submitting">{{ $t('gitdeploy.update_config') }}</button>
              <button type="button" @click="triggerDeploy" class="btn" :disabled="deploying">
                {{ deploying ? $t('gitdeploy.deploying') : $t('gitdeploy.deploy_now') }}
              </button>
            </div>
          </form>

          <div style="border-top: 1px solid var(--border-color, #e2e8f0); padding-top: 24px;">
            <button @click="removeGit" class="btn danger">{{ $t('gitdeploy.remove') }}</button>
          </div>
        </div>
      </div>
    </div>
  </Layout>
</template>


<script setup>
import Layout from '../components/Layout.vue'
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
