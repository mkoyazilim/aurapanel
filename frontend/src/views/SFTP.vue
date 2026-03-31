<template>
  <div class="space-y-6">
    <div class="flex items-center justify-between gap-3">
      <div>
        <h1 class="text-2xl font-bold text-white">{{ t('sftp_manager.title') }}</h1>
        <p class="text-gray-400 mt-1">{{ t('sftp_manager.subtitle') }}</p>
      </div>
      <button class="btn-primary" @click="showCreate = true">{{ t('sftp_manager.add_user') }}</button>
    </div>

    <div v-if="error" class="aura-card border-red-500/30 bg-red-500/5 text-red-400">{{ error }}</div>
    <div v-if="success" class="aura-card border-green-500/30 bg-green-500/5 text-green-300">{{ success }}</div>

    <div class="aura-card space-y-4">
      <div class="flex justify-end">
        <button class="btn-secondary" @click="loadUsers">{{ t('sftp_manager.refresh') }}</button>
      </div>

      <div class="overflow-x-auto">
        <table class="w-full text-sm">
          <thead>
            <tr class="border-b border-panel-border text-gray-400">
              <th class="text-left py-2 px-2">{{ t('sftp_manager.table.username') }}</th>
              <th class="text-left py-2 px-2">{{ t('sftp_manager.table.home') }}</th>
              <th class="text-left py-2 px-2">{{ t('sftp_manager.table.created') }}</th>
              <th class="text-right py-2 px-2">{{ t('sftp_manager.table.action') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="item in users" :key="item.username" class="border-b border-panel-border/50">
              <td class="py-2 px-2 text-white font-mono">{{ item.username }}</td>
              <td class="py-2 px-2 text-gray-300 font-mono text-xs break-all">{{ item.home_dir }}</td>
              <td class="py-2 px-2 text-gray-400">{{ formatTime(item.created_at) }}</td>
              <td class="py-2 px-2 text-right">
                <div class="flex justify-end gap-2">
                  <button class="btn-secondary px-2 py-1 text-xs" @click="openReset(item.username)">{{ t('sftp_manager.actions.password') }}</button>
                  <button class="btn-danger px-2 py-1 text-xs" @click="removeUser(item.username)">{{ t('sftp_manager.actions.delete') }}</button>
                </div>
              </td>
            </tr>
            <tr v-if="users.length === 0">
              <td colspan="4" class="text-center py-8 text-gray-500">{{ t('sftp_manager.table.empty') }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <Teleport to="body">
      <div v-if="showCreate" class="fixed inset-0 bg-black/60 flex items-center justify-center z-50 p-4">
        <div class="bg-panel-card border border-panel-border rounded-2xl p-6 w-full max-w-lg">
          <h2 class="text-xl font-bold text-white mb-4">{{ t('sftp_manager.modal.create_title') }}</h2>
          <div class="grid grid-cols-1 md:grid-cols-2 gap-3">
            <div>
              <label class="block text-sm text-gray-400 mb-1">{{ t('sftp_manager.modal.username') }}</label>
              <input v-model="createForm.username" class="aura-input w-full" :placeholder="t('sftp_manager.modal.username_placeholder')" />
            </div>
            <div>
              <label class="block text-sm text-gray-400 mb-1">{{ t('sftp_manager.modal.password') }}</label>
              <input v-model="createForm.password" type="password" class="aura-input w-full" />
            </div>
            <div class="md:col-span-2">
              <label class="block text-sm text-gray-400 mb-1">{{ t('sftp_manager.modal.home_directory') }}</label>
              <input v-model="createForm.home_dir" class="aura-input w-full" :placeholder="t('sftp_manager.modal.home_directory_placeholder')" />
            </div>
          </div>
          <div class="flex gap-3 mt-6">
            <button class="btn-secondary flex-1" @click="showCreate = false">{{ t('sftp_manager.modal.cancel') }}</button>
            <button class="btn-primary flex-1" @click="createUser">{{ t('sftp_manager.modal.create') }}</button>
          </div>
        </div>
      </div>
    </Teleport>

    <Teleport to="body">
      <div v-if="showReset" class="fixed inset-0 bg-black/60 flex items-center justify-center z-50 p-4">
        <div class="bg-panel-card border border-panel-border rounded-2xl p-6 w-full max-w-md">
          <h2 class="text-xl font-bold text-white mb-4">{{ t('sftp_manager.modal.reset_title') }}</h2>
          <p class="text-sm text-gray-400 mb-3">{{ t('sftp_manager.modal.user_label') }}: <span class="text-white font-mono">{{ resetForm.username }}</span></p>
          <input v-model="resetForm.new_password" type="password" class="aura-input w-full" :placeholder="t('sftp_manager.modal.new_password_placeholder')" />
          <div class="flex gap-3 mt-6">
            <button class="btn-secondary flex-1" @click="showReset = false">{{ t('sftp_manager.modal.cancel') }}</button>
            <button class="btn-primary flex-1" @click="updatePassword">{{ t('sftp_manager.modal.update') }}</button>
          </div>
        </div>
      </div>
    </Teleport>
  </div>
</template>

<script setup>
import { onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import api from '../services/api'

const { t, locale } = useI18n({ useScope: 'global' })
const route = useRoute()

const users = ref([])
const error = ref('')
const success = ref('')
const showCreate = ref(false)
const selectedDomain = ref(typeof route.query.domain === 'string' ? route.query.domain : '')
const createActionConsumed = ref(false)
const showReset = ref(false)

function defaultHome(domain) {
  if (domain) {
    return `/home/${domain}/public_html`
  }
  return '/home'
}

const createForm = ref({
  username: '',
  password: '',
  home_dir: defaultHome(selectedDomain.value),
})

const resetForm = ref({
  username: '',
  new_password: '',
})

function apiErrorMessage(e, fallbackKey) {
  return e?.response?.data?.message || e?.message || t(fallbackKey)
}

function formatTime(ts) {
  const value = Number(ts || 0)
  if (!value) return '-'
  return new Date(value * 1000).toLocaleString(locale.value)
}

async function loadUsers() {
  error.value = ''
  try {
    const params = selectedDomain.value ? { domain: selectedDomain.value } : {}
    const res = await api.get('/sftp/list', { params })
    users.value = res.data?.data || []
  } catch (e) {
    error.value = apiErrorMessage(e, 'sftp_manager.messages.list_failed')
  }
}

async function createUser() {
  error.value = ''
  success.value = ''
  if (!createForm.value.username || !createForm.value.password || !createForm.value.home_dir) {
    error.value = t('sftp_manager.messages.required_create')
    return
  }
  try {
    const res = await api.post('/sftp/create', {
      ...createForm.value,
      domain: selectedDomain.value || undefined,
    })
    success.value = res.data?.message || t('sftp_manager.messages.created')
    showCreate.value = false
    createForm.value = { username: '', password: '', home_dir: defaultHome(selectedDomain.value) }
    await loadUsers()
  } catch (e) {
    error.value = apiErrorMessage(e, 'sftp_manager.messages.create_failed')
  }
}

function openReset(username) {
  resetForm.value = { username, new_password: '' }
  showReset.value = true
}

async function updatePassword() {
  error.value = ''
  success.value = ''
  if (!resetForm.value.username || !resetForm.value.new_password) {
    error.value = t('sftp_manager.messages.required_reset')
    return
  }
  try {
    const res = await api.post('/sftp/password', resetForm.value)
    success.value = res.data?.message || t('sftp_manager.messages.password_updated')
    showReset.value = false
  } catch (e) {
    error.value = apiErrorMessage(e, 'sftp_manager.messages.password_failed')
  }
}

async function removeUser(username) {
  if (!window.confirm(t('sftp_manager.messages.delete_confirm', { username }))) return
  error.value = ''
  success.value = ''
  try {
    const res = await api.post('/sftp/delete', { username })
    success.value = res.data?.message || t('sftp_manager.messages.deleted')
    await loadUsers()
  } catch (e) {
    error.value = apiErrorMessage(e, 'sftp_manager.messages.delete_failed')
  }
}

watch(
  () => route.query.domain,
  (value) => {
    selectedDomain.value = typeof value === 'string' ? value : ''
    if (!createForm.value.home_dir || createForm.value.home_dir === '/home') {
      createForm.value.home_dir = defaultHome(selectedDomain.value)
    }
    loadUsers()
  },
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

onMounted(loadUsers)
</script>
