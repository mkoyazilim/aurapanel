<template>
  <div class="space-y-6">
    <div class="flex items-center justify-between">
      <div>
        <h1 class="text-2xl font-bold text-white">{{ t('cms_installer.title') }}</h1>
        <p class="text-sm text-gray-400 mt-1">{{ t('cms_installer.subtitle') }}</p>
        <p class="text-xs text-brand-300 mt-2">
          {{ t('app_runtime.cms.help_prefix') }}
          <router-link to="/wordpress" class="underline underline-offset-2 hover:text-white">{{ t('app_runtime.wordpress_manager') }}</router-link>
          {{ t('app_runtime.cms.help_suffix') }}
        </p>
      </div>
      <div class="flex items-center gap-3">
        <router-link to="/wordpress" class="btn-secondary">
          {{ t('app_runtime.wordpress_manager') }}
        </router-link>
      </div>
    </div>

    <div class="aura-card space-y-5">
      <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
        <div>
          <label class="block text-sm text-gray-400 mb-1">{{ t('cms_installer.app_type') }}</label>
          <select v-model="cms.app_type" class="aura-input">
            <option value="wordpress">{{ t('cms_installer.options.wordpress') }}</option>
            <option value="laravel">{{ t('cms_installer.options.laravel') }}</option>
          </select>
        </div>
        <div>
          <label class="block text-sm text-gray-400 mb-1">{{ t('cms_installer.domain') }}</label>
          <select v-model="cms.domain" class="aura-input" :disabled="cmsDomainsLoading || cmsDomains.length === 0">
            <option value="" disabled>{{ cmsDomainsLoading ? t('common.loading') : t('websites.select_domain') }}</option>
            <option v-for="domain in cmsDomains" :key="domain" :value="domain">{{ domain }}</option>
          </select>
          <p v-if="!cmsDomainsLoading && cmsDomains.length === 0" class="mt-1 text-xs text-yellow-300">
            {{ t('websites.no_sites') }}
          </p>
        </div>
        <div>
          <label class="block text-sm text-gray-400 mb-1">{{ t('cms_installer.db_name') }}</label>
          <input v-model="cms.db_name" class="aura-input" :placeholder="t('cms_installer.placeholders.db_name')" />
        </div>
        <div>
          <label class="block text-sm text-gray-400 mb-1">{{ t('cms_installer.db_user') }}</label>
          <input v-model="cms.db_user" class="aura-input" :placeholder="t('cms_installer.placeholders.db_user')" />
        </div>
        <div>
          <label class="block text-sm text-gray-400 mb-1">{{ t('cms_installer.db_pass') }}</label>
          <input v-model="cms.db_pass" type="password" class="aura-input" :placeholder="t('cms_installer.placeholders.password')" />
        </div>
        <div>
          <label class="block text-sm text-gray-400 mb-1">{{ t('cms_installer.admin_email') }}</label>
          <input v-model="cms.admin_email" type="email" class="aura-input" :placeholder="t('cms_installer.placeholders.admin_email')" />
        </div>
        <div>
          <label class="block text-sm text-gray-400 mb-1">{{ t('cms_installer.admin_user') }}</label>
          <input v-model="cms.admin_user" class="aura-input" :placeholder="t('cms_installer.placeholders.admin_user')" />
        </div>
        <div>
          <label class="block text-sm text-gray-400 mb-1">{{ t('cms_installer.admin_pass') }}</label>
          <input v-model="cms.admin_pass" type="password" class="aura-input" :placeholder="t('cms_installer.placeholders.password')" />
        </div>
      </div>

      <div class="pt-2">
        <button
          class="btn-primary flex items-center gap-2"
          :disabled="cmsInstalling"
          @click="installCms"
        >
          <span v-if="cmsInstalling">{{ t('cms_installer.installing') }}</span>
          <span v-else>{{ t('cms_installer.install') }}</span>
        </button>
      </div>

      <div v-if="cmsMessage" :class="['px-4 py-3 rounded-lg text-sm font-medium', cmsMessageType === 'success' ? 'bg-green-600/20 text-green-400 border border-green-500/30' : 'bg-red-600/20 text-red-400 border border-red-500/30']">
        {{ cmsMessage }}
      </div>
    </div>
  </div>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import api from '../services/api'

const { t } = useI18n({ useScope: 'global' })

const cmsDomains = ref([])
const cmsDomainsLoading = ref(false)
const cms = ref({
  app_type: 'wordpress',
  domain: '',
  db_name: '',
  db_user: '',
  db_pass: '',
  admin_email: '',
  admin_user: '',
  admin_pass: '',
})
const cmsInstalling = ref(false)
const cmsMessage = ref('')
const cmsMessageType = ref('success')

function normalizeDomain(value) {
  return String(value || '').trim().toLowerCase()
}

async function loadCmsDomains() {
  cmsDomainsLoading.value = true
  try {
    const collected = []
    let page = 1
    let totalPages = 1
    const perPage = 200

    do {
      const res = await api.get('/vhost/list', {
        params: {
          page,
          per_page: perPage,
        },
      })
      const items = Array.isArray(res.data?.data) ? res.data.data : []
      for (const item of items) {
        const domain = normalizeDomain(item?.domain)
        if (domain) {
          collected.push(domain)
        }
      }
      totalPages = Math.max(1, Number(res.data?.pagination?.total_pages || 1))
      page += 1
    } while (page <= totalPages)

    const unique = [...new Set(collected)].sort((a, b) => a.localeCompare(b))
    cmsDomains.value = unique

    const selected = normalizeDomain(cms.value.domain)
    if (!selected || !unique.includes(selected)) {
      cms.value.domain = unique[0] || ''
    }
  } catch {
    cmsDomains.value = []
    cms.value.domain = ''
  } finally {
    cmsDomainsLoading.value = false
  }
}

async function installCms() {
  if (!normalizeDomain(cms.value.domain)) {
    cmsMessageType.value = 'error'
    cmsMessage.value = t('websites.select_domain')
    return
  }
  cmsInstalling.value = true
  cmsMessage.value = ''
  try {
    await api.post('/apps/install', { ...cms.value })
    cmsMessage.value = t('cms_installer.success')
    cmsMessageType.value = 'success'
  } catch (err) {
    cmsMessage.value = err?.response?.data?.message || t('common.error')
    cmsMessageType.value = 'error'
  } finally {
    cmsInstalling.value = false
  }
}

onMounted(() => {
  loadCmsDomains()
})
</script>
