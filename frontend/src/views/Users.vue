<template>
  <div class="space-y-6">
    <div class="flex items-center justify-between">
      <div>
        <h1 class="text-2xl font-bold text-white">{{ t('users.title') }}</h1>
        <p class="text-gray-400 mt-1">{{ t('users.subtitle') }}</p>
      </div>
      <button class="btn-primary" @click="showAddModal = true">
        <UserPlus class="w-5 h-5" />
        {{ t('users.add_new') }}
      </button>
    </div>

    <!-- Loading / Error -->
    <div v-if="loading" class="aura-card text-center py-12">
      <Loader2 class="w-8 h-8 text-brand-500 animate-spin mx-auto mb-3" />
      <p class="text-gray-400">{{ t('common.loading') }}</p>
    </div>
    <div v-else-if="error" class="aura-card border-red-500/30 bg-red-500/5 text-center py-8">
      <p class="text-red-400">{{ error }}</p>
    </div>

    <!-- User List -->
    <div v-else class="space-y-4">
      <div v-for="user in users" :key="user.id"
        class="aura-card flex flex-col sm:flex-row gap-6 justify-between items-start sm:items-center">
        <div class="flex items-center gap-4">
          <div class="w-12 h-12 rounded-full bg-gradient-to-tr from-brand-600 to-panel-border flex items-center justify-center font-bold text-lg text-white">
            {{ user.username.charAt(0).toUpperCase() }}
          </div>
          <div>
            <h3 class="text-lg font-bold text-white flex items-center gap-2">
              {{ user.username }}
              <span class="px-2 py-0.5 rounded text-xs font-semibold bg-panel-dark border border-panel-border">{{ user.package }}</span>
              <span v-if="user.role === 'reseller'" class="px-2 py-0.5 rounded text-xs font-semibold bg-brand-500/10 text-brand-400 border border-brand-500/20">Bayi</span>
              <span v-if="user.role === 'admin'" class="px-2 py-0.5 rounded text-xs font-semibold bg-red-500/10 text-red-400 border border-red-500/20">Admin</span>
            </h3>
            <div class="text-sm text-gray-400 mt-1 flex items-center gap-4">
              <span>{{ user.email }}</span>
              <span class="flex items-center gap-1"><Globe class="w-4 h-4" /> {{ user.sites }} {{ t('users.sites') }}</span>
            </div>
          </div>
        </div>
        <div class="flex items-center gap-2 w-full sm:w-auto">
          <span :class="user.active ? 'bg-green-500/10 text-green-400 border-green-500/20' : 'bg-gray-500/10 text-gray-400 border-gray-500/20'"
            class="px-2 py-1 rounded text-xs border">{{ user.active ? t('common.active') : t('common.inactive') }}</span>
          <button class="btn-secondary p-2" title="Sifre Degistir" @click="openPasswordModal(user)">
            <KeyRound class="w-4 h-4" />
          </button>
          <button class="btn-danger p-2" :title="t('common.delete')" @click="deleteUser(user)">
            <UserMinus class="w-4 h-4" />
          </button>
        </div>
      </div>
      <div v-if="users.length === 0" class="aura-card text-center py-12 text-gray-400">{{ t('common.no_data') }}</div>
    </div>

    <!-- Add Modal -->
    <Teleport to="body">
      <div v-if="showAddModal" class="fixed inset-0 bg-black/60 flex items-center justify-center z-50 p-4">
        <div class="bg-panel-card border border-panel-border rounded-2xl p-8 w-full max-w-md shadow-2xl">
          <h2 class="text-xl font-bold text-white mb-6">{{ t('users.add_modal_title') }}</h2>
          <div class="space-y-4">
            <div>
              <label class="block text-sm text-gray-400 mb-1">{{ t('users.username') }}</label>
              <input v-model="form.username" type="text" class="aura-input w-full" />
            </div>
            <div>
              <label class="block text-sm text-gray-400 mb-1">{{ t('users.email') }}</label>
              <input v-model="form.email" type="email" class="aura-input w-full" />
            </div>
            <div>
              <label class="block text-sm text-gray-400 mb-1">{{ t('users.password') }}</label>
              <input v-model="form.password" type="password" class="aura-input w-full" />
            </div>
            <div>
              <label class="block text-sm text-gray-400 mb-1">{{ t('users.role') }}</label>
              <select v-model="form.role" class="aura-input w-full">
                <option value="user">{{ t('users.role_user') }}</option>
                <option value="reseller">{{ t('users.role_reseller') }}</option>
                <option value="admin">{{ t('users.role_admin') }}</option>
              </select>
            </div>
            <div>
              <label class="block text-sm text-gray-400 mb-1">{{ t('users.package') }}</label>
              <select v-model="form.package" class="aura-input w-full">
                <option v-for="pkg in packageOptions" :key="`user-package-${pkg}`" :value="pkg">{{ pkg }}</option>
              </select>
              <p class="mt-1 text-xs text-gray-500">Rol ile uyumlu paketler listelenir.</p>
            </div>
          </div>
          <div class="flex gap-3 mt-8">
            <button class="btn-secondary flex-1" @click="showAddModal = false">{{ t('common.cancel') }}</button>
            <button class="btn-primary flex-1" :disabled="addLoading" @click="addUser">
              <Loader2 v-if="addLoading" class="w-4 h-4 animate-spin mr-2 inline" />
              {{ t('common.create') }}
            </button>
          </div>
        </div>
      </div>
    </Teleport>

    <!-- Change Password Modal -->
    <Teleport to="body">
      <div v-if="showPasswordModal" class="fixed inset-0 bg-black/60 flex items-center justify-center z-50 p-4">
        <div class="bg-panel-card border border-panel-border rounded-2xl p-8 w-full max-w-md shadow-2xl">
          <h2 class="text-xl font-bold text-white mb-2">Kullanici Sifresi Degistir</h2>
          <p class="text-sm text-gray-400 mb-6">
            Kullanici: <span class="text-white font-mono">{{ passwordForm.username }}</span>
          </p>
          <div>
            <label class="block text-sm text-gray-400 mb-1">Yeni Sifre</label>
            <input v-model="passwordForm.new_password" type="password" class="aura-input w-full" placeholder="En az 8 karakter" />
          </div>
          <div class="flex gap-3 mt-8">
            <button class="btn-secondary flex-1" @click="closePasswordModal">{{ t('common.cancel') }}</button>
            <button class="btn-primary flex-1" :disabled="passwordLoading" @click="changePassword">
              <Loader2 v-if="passwordLoading" class="w-4 h-4 animate-spin mr-2 inline" />
              Kaydet
            </button>
          </div>
        </div>
      </div>
    </Teleport>
  </div>
</template>

<script setup>
import { computed, ref, onMounted, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { UserPlus, UserMinus, Globe, Loader2, KeyRound } from 'lucide-vue-next'
import api from '../services/api'

const { t } = useI18n()
const users = ref([])
const loading = ref(true)
const error = ref(null)
const showAddModal = ref(false)
const addLoading = ref(false)
const showPasswordModal = ref(false)
const passwordLoading = ref(false)
const passwordForm = ref({ username: '', new_password: '' })
const hostingPackages = ref([])
const form = ref({ username: '', email: '', password: '', role: 'user', package: 'default' })

const packageOptions = computed(() => {
  const names = new Set(['default'])
  const wantsResellerPackage = form.value.role === 'reseller'

  for (const pkg of hostingPackages.value || []) {
    const name = String(pkg?.name || '').trim()
    const planType = String(pkg?.plan_type || 'hosting').trim().toLowerCase()
    if (!name) continue
    if (wantsResellerPackage && planType !== 'reseller') continue
    if (!wantsResellerPackage && form.value.role !== 'admin' && planType === 'reseller') continue
    names.add(name)
  }

  const ordered = Array.from(names).filter(Boolean)
  const tail = ordered.filter(name => name !== 'default').sort((a, b) => a.localeCompare(b))
  return ['default', ...tail]
})

async function loadUsers() {
  loading.value = true
  error.value = null
  try {
    const res = await api.get('/users/list')
    users.value = res.data.data || []
  } catch (e) {
    error.value = t('common.error')
  } finally {
    loading.value = false
  }
}

async function loadPackages() {
  try {
    const res = await api.get('/packages/list')
    hostingPackages.value = res.data?.data || []
  } catch {
    hostingPackages.value = []
  }
}

async function addUser() {
  if (!form.value.username || !form.value.email) return
  addLoading.value = true
  try {
    await api.post('/users/create', form.value)
    showAddModal.value = false
    form.value = { username: '', email: '', password: '', role: 'user', package: packageOptions.value[0] || 'default' }
    await loadUsers()
  } catch (e) {
    error.value = t('common.error')
  } finally {
    addLoading.value = false
  }
}

async function deleteUser(user) {
  if (!confirm(t('users.confirm_delete'))) return
  try {
    await api.post('/users/delete', { username: user.username })
    await loadUsers()
  } catch (e) {
    error.value = t('common.error')
  }
}

function openPasswordModal(user) {
  passwordForm.value = {
    username: user.username,
    new_password: '',
  }
  showPasswordModal.value = true
}

function closePasswordModal() {
  showPasswordModal.value = false
  passwordForm.value = { username: '', new_password: '' }
}

async function changePassword() {
  if (!passwordForm.value.username || !passwordForm.value.new_password) return
  passwordLoading.value = true
  try {
    await api.post('/users/change-password', {
      username: passwordForm.value.username,
      new_password: passwordForm.value.new_password,
    })
    closePasswordModal()
  } catch (e) {
    error.value = e?.response?.data?.message || t('common.error')
  } finally {
    passwordLoading.value = false
  }
}

watch(packageOptions, (options) => {
  if (!options.length) return
  if (!options.includes(form.value.package)) {
    form.value.package = options[0]
  }
}, { immediate: true })

onMounted(async () => {
  await Promise.all([loadUsers(), loadPackages()])
})
</script>
