<template>
  <div class="space-y-6">
    <div class="flex items-center justify-between">
      <div>
        <h1 class="text-2xl font-bold text-white">{{ t('users.title') }}</h1>
        <p class="text-gray-400 mt-1">{{ t('users.subtitle') }}</p>
      </div>
      <div class="flex items-center gap-2">
        <button v-if="authStore.isAdmin" class="btn-secondary" @click="openRoleEditor">
          {{ t('users.role_editor_button') }}
        </button>
        <button class="btn-primary" @click="showAddModal = true">
          <UserPlus class="w-5 h-5" />
          {{ t('users.add_new') }}
        </button>
      </div>
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
              <span v-if="user.role === 'reseller'" class="px-2 py-0.5 rounded text-xs font-semibold bg-brand-500/10 text-brand-400 border border-brand-500/20">{{ t('users.role_reseller') }}</span>
              <span v-if="user.role === 'admin'" class="px-2 py-0.5 rounded text-xs font-semibold bg-red-500/10 text-red-400 border border-red-500/20">{{ t('users.role_admin') }}</span>
              <span v-if="user.is_owner" class="px-2 py-0.5 rounded text-xs font-semibold bg-orange-500/10 text-orange-300 border border-orange-500/20">{{ t('users.role_owner') }}</span>
              <span v-if="user.role_policy_name || user.role_policy_id" class="px-2 py-0.5 rounded text-xs font-semibold bg-indigo-500/10 text-indigo-300 border border-indigo-500/20">
                {{ user.role_policy_name || policyNameById(user.role_policy_id) }}
              </span>
            </h3>
            <div class="text-sm text-gray-400 mt-1 flex items-center gap-4">
              <span>{{ user.email }}</span>
              <span v-if="user.parent_username" class="px-2 py-0.5 rounded text-xs font-semibold bg-slate-500/10 text-slate-300 border border-slate-500/20">
                {{ t('users.parent_badge', { parent: user.parent_username }) }}
              </span>
              <span class="flex items-center gap-1"><Globe class="w-4 h-4" /> {{ user.sites }} {{ t('users.sites') }}</span>
            </div>
          </div>
        </div>
        <div class="flex items-center gap-2 w-full sm:w-auto">
          <span :class="user.active ? 'bg-green-500/10 text-green-400 border-green-500/20' : 'bg-gray-500/10 text-gray-400 border-gray-500/20'"
            class="px-2 py-1 rounded text-xs border">{{ user.active ? t('common.active') : t('common.inactive') }}</span>
          <button class="btn-secondary p-2" :title="t('users.edit_user')" @click="openEditModal(user)">
            <Pencil class="w-4 h-4" />
          </button>
          <button class="btn-secondary p-2" :title="t('users.change_password')" @click="openPasswordModal(user)">
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
                <option v-if="authStore.isAdmin" value="admin">{{ t('users.role_admin') }}</option>
              </select>
            </div>
            <div v-if="form.role !== 'admin'">
              <label class="block text-sm text-gray-400 mb-1">{{ t('users.parent_user') }}</label>
              <select v-model="form.parent_username" class="aura-input w-full">
                <option value="">{{ t('users.parent_none') }}</option>
                <option v-for="candidate in parentCandidateUsers" :key="`new-parent-${candidate.username}`" :value="candidate.username">
                  {{ candidate.username }}
                </option>
              </select>
            </div>
            <div>
              <label class="block text-sm text-gray-400 mb-1">{{ t('users.role_policy_label') }}</label>
              <select v-model="form.role_policy_id" class="aura-input w-full">
                <option value="">{{ t('users.role_policy_system_default') }}</option>
                <option v-for="policy in rolePolicies" :key="`new-role-policy-${policy.id}`" :value="policy.id">
                  {{ policy.name }}
                </option>
              </select>
            </div>
            <div>
              <label class="block text-sm text-gray-400 mb-1">{{ t('users.package') }}</label>
              <select v-model="form.package" class="aura-input w-full">
                <option v-for="pkg in addPackageOptions" :key="`user-package-${pkg}`" :value="pkg">{{ pkg }}</option>
              </select>
              <p class="mt-1 text-xs text-gray-500">{{ t('users.role_package_hint') }}</p>
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

    <!-- Edit User Modal -->
    <Teleport to="body">
      <div v-if="showEditModal" class="fixed inset-0 bg-black/60 flex items-center justify-center z-50 p-4">
        <div class="bg-panel-card border border-panel-border rounded-2xl p-8 w-full max-w-md shadow-2xl">
          <h2 class="text-xl font-bold text-white mb-6">{{ t('users.edit_modal_title') }}</h2>
          <div class="space-y-4">
            <div>
              <label class="block text-sm text-gray-400 mb-1">{{ t('users.username') }}</label>
              <input v-model="editForm.username" type="text" class="aura-input w-full opacity-70" readonly />
            </div>
            <div>
              <label class="block text-sm text-gray-400 mb-1">{{ t('users.display_name') }}</label>
              <input v-model="editForm.name" type="text" class="aura-input w-full" />
            </div>
            <div>
              <label class="block text-sm text-gray-400 mb-1">{{ t('users.email') }}</label>
              <input v-model="editForm.email" type="email" class="aura-input w-full" />
            </div>
            <div>
              <label class="block text-sm text-gray-400 mb-1">{{ t('users.role') }}</label>
              <select v-model="editForm.role" class="aura-input w-full" :disabled="editForm.is_owner">
                <option value="user">{{ t('users.role_user') }}</option>
                <option value="reseller">{{ t('users.role_reseller') }}</option>
                <option v-if="authStore.isAdmin" value="admin">{{ t('users.role_admin') }}</option>
              </select>
            </div>
            <div v-if="editForm.role !== 'admin'">
              <label class="block text-sm text-gray-400 mb-1">{{ t('users.parent_user') }}</label>
              <select v-model="editForm.parent_username" class="aura-input w-full">
                <option value="">{{ t('users.parent_none') }}</option>
                <option v-for="candidate in parentCandidateUsers" :key="`edit-parent-${candidate.username}`" :value="candidate.username">
                  {{ candidate.username }}
                </option>
              </select>
            </div>
            <div>
              <label class="block text-sm text-gray-400 mb-1">{{ t('users.role_policy_label') }}</label>
              <select v-model="editForm.role_policy_id" class="aura-input w-full">
                <option value="">{{ t('users.role_policy_system_default') }}</option>
                <option v-for="policy in rolePolicies" :key="`edit-role-policy-${policy.id}`" :value="policy.id">
                  {{ policy.name }}
                </option>
              </select>
            </div>
            <div>
              <label class="block text-sm text-gray-400 mb-1">{{ t('users.package') }}</label>
              <select v-model="editForm.package" class="aura-input w-full">
                <option v-for="pkg in editPackageOptions" :key="`edit-user-package-${pkg}`" :value="pkg">{{ pkg }}</option>
              </select>
            </div>
            <label class="inline-flex items-center gap-2 text-sm text-gray-300">
              <input v-model="editForm.active" type="checkbox" class="h-4 w-4" :disabled="editForm.is_owner" />
              {{ t('users.active_account') }}
            </label>
          </div>
          <div class="flex gap-3 mt-8">
            <button class="btn-secondary flex-1" @click="closeEditModal">{{ t('common.cancel') }}</button>
            <button class="btn-primary flex-1" :disabled="editLoading" @click="updateUser">
              <Loader2 v-if="editLoading" class="w-4 h-4 animate-spin mr-2 inline" />
              {{ t('users.save_changes') }}
            </button>
          </div>
        </div>
      </div>
    </Teleport>

    <!-- Change Password Modal -->
    <Teleport to="body">
      <div v-if="showPasswordModal" class="fixed inset-0 bg-black/60 flex items-center justify-center z-50 p-4">
        <div class="bg-panel-card border border-panel-border rounded-2xl p-8 w-full max-w-md shadow-2xl">
          <h2 class="text-xl font-bold text-white mb-2">{{ t('users.password_modal_title') }}</h2>
          <p class="text-sm text-gray-400 mb-6">
            {{ t('users.password_modal_user') }}: <span class="text-white font-mono">{{ passwordForm.username }}</span>
          </p>
          <div>
            <label class="block text-sm text-gray-400 mb-1">{{ t('users.password_modal_new') }}</label>
            <input v-model="passwordForm.new_password" type="password" class="aura-input w-full" :placeholder="t('users.password_modal_placeholder')" />
          </div>
          <div class="flex gap-3 mt-8">
            <button class="btn-secondary flex-1" @click="closePasswordModal">{{ t('common.cancel') }}</button>
            <button class="btn-primary flex-1" :disabled="passwordLoading" @click="changePassword">
              <Loader2 v-if="passwordLoading" class="w-4 h-4 animate-spin mr-2 inline" />
              {{ t('users.password_modal_save') }}
            </button>
          </div>
        </div>
      </div>
    </Teleport>

    <Teleport to="body">
      <div v-if="showRoleEditorModal" class="fixed inset-0 bg-black/60 flex items-center justify-center z-50 p-4">
        <div class="bg-panel-card border border-panel-border rounded-2xl p-6 w-full max-w-5xl shadow-2xl max-h-[90vh] overflow-y-auto">
          <div class="flex items-center justify-between mb-5">
            <div>
              <h2 class="text-xl font-bold text-white">{{ t('users.role_editor_title') }}</h2>
              <p class="text-sm text-gray-400 mt-1">{{ t('users.role_editor_subtitle') }}</p>
            </div>
            <button class="btn-secondary" @click="closeRoleEditor">{{ t('users.role_editor_close') }}</button>
          </div>

          <div v-if="roleEditorError" class="aura-card border-red-500/30 bg-red-500/5 text-red-400 mb-4">{{ roleEditorError }}</div>

          <div class="grid grid-cols-1 lg:grid-cols-3 gap-6">
            <div class="lg:col-span-2 space-y-4">
              <div>
                <label class="block text-sm text-gray-400 mb-1">{{ t('users.role_editor_name') }}</label>
                <input v-model="policyForm.name" type="text" class="aura-input w-full" :placeholder="t('users.role_editor_name_placeholder')" />
              </div>
              <div>
                <label class="block text-sm text-gray-400 mb-1">{{ t('users.role_editor_description') }}</label>
                <input v-model="policyForm.description" type="text" class="aura-input w-full" :placeholder="t('users.role_editor_description_placeholder')" />
              </div>
              <div>
                <label class="block text-sm text-gray-400 mb-2">{{ t('users.role_editor_permissions') }}</label>
                <div class="grid grid-cols-1 md:grid-cols-2 gap-2">
                  <label
                    v-for="perm in rolePermissionCatalog"
                    :key="perm.key"
                    class="flex items-center gap-2 rounded border border-panel-border bg-panel-darker/50 px-3 py-2 text-sm text-gray-200"
                  >
                    <input
                      type="checkbox"
                      class="h-4 w-4"
                      :checked="policyHasPermission(perm.key)"
                      @change="togglePolicyPermission(perm.key, $event.target.checked)"
                    />
                    <span>{{ permissionLabel(perm) }}</span>
                    <span class="text-[11px] text-gray-500 ml-auto">{{ perm.key }}</span>
                  </label>
                </div>
              </div>
              <div class="flex gap-2">
                <button class="btn-primary" :disabled="roleEditorLoading" @click="savePolicy">
                  <Loader2 v-if="roleEditorLoading" class="w-4 h-4 animate-spin mr-2 inline" />
                  {{ policyForm.id ? t('users.role_editor_update') : t('users.role_editor_save') }}
                </button>
                <button class="btn-secondary" :disabled="roleEditorLoading" @click="resetPolicyForm">{{ t('users.role_editor_reset') }}</button>
              </div>
            </div>

            <div class="space-y-2">
              <h3 class="text-sm font-semibold text-gray-300">{{ t('users.role_editor_list_title') }}</h3>
              <div v-for="policy in rolePolicies" :key="`role-editor-${policy.id}`" class="rounded border border-panel-border bg-panel-darker/50 p-3">
                <div class="flex items-start justify-between gap-2">
                  <div>
                    <p class="text-white font-semibold">{{ policy.name }}</p>
                    <p class="text-xs text-gray-400 mt-0.5">{{ policy.description || '-' }}</p>
                    <p class="text-[11px] text-gray-500 mt-1">{{ t('users.role_editor_permission_count', { count: (policy.permissions || []).length }) }}</p>
                  </div>
                  <div class="flex gap-1">
                    <button class="btn-secondary px-2 py-1 text-xs" @click="editPolicy(policy)">{{ t('users.role_editor_edit') }}</button>
                    <button class="btn-danger px-2 py-1 text-xs" @click="deletePolicy(policy)">{{ t('users.role_editor_delete') }}</button>
                  </div>
                </div>
              </div>
              <div v-if="rolePolicies.length === 0" class="text-sm text-gray-500">{{ t('users.role_editor_empty') }}</div>
            </div>
          </div>
        </div>
      </div>
    </Teleport>
  </div>
</template>

<script setup>
import { computed, ref, onMounted, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { UserPlus, UserMinus, Globe, Loader2, KeyRound, Pencil } from 'lucide-vue-next'
import api from '../services/api'
import { ROLE_PERMISSION_CATALOG } from '../security/rbac'
import { useAuthStore } from '../stores/auth'

const { t } = useI18n()
const authStore = useAuthStore()
const users = ref([])
const loading = ref(true)
const error = ref(null)
const showAddModal = ref(false)
const addLoading = ref(false)
const showEditModal = ref(false)
const editLoading = ref(false)
const showPasswordModal = ref(false)
const passwordLoading = ref(false)
const showRoleEditorModal = ref(false)
const roleEditorLoading = ref(false)
const roleEditorError = ref('')
const passwordForm = ref({ username: '', new_password: '' })
const hostingPackages = ref([])
const rolePolicies = ref([])
const rolePermissionCatalog = ROLE_PERMISSION_CATALOG
const form = ref({ username: '', email: '', password: '', role: 'user', parent_username: '', role_policy_id: '', package: 'default' })
const editForm = ref({ username: '', name: '', email: '', role: 'user', parent_username: '', role_policy_id: '', package: 'default', active: true, is_owner: false })
const policyForm = ref({ id: '', name: '', description: '', permissions: [] })

const parentCandidateUsers = computed(() =>
  (users.value || []).filter((item) => {
    const role = String(item?.role || '').toLowerCase()
    return role === 'admin' || role === 'reseller'
  }),
)

function defaultParentForNewUser() {
  if (!authStore.isReseller) return ''
  return String(authStore.user?.username || '').trim()
}

function packageOptionsForRole(role) {
  const names = new Set(['default'])
  const wantsResellerPackage = role === 'reseller'

  for (const pkg of hostingPackages.value || []) {
    const name = String(pkg?.name || '').trim()
    const planType = String(pkg?.plan_type || 'hosting').trim().toLowerCase()
    if (!name) continue
    if (wantsResellerPackage && planType !== 'reseller') continue
    if (!wantsResellerPackage && role !== 'admin' && planType === 'reseller') continue
    names.add(name)
  }

  const ordered = Array.from(names).filter(Boolean)
  const tail = ordered.filter(name => name !== 'default').sort((a, b) => a.localeCompare(b))
  return ['default', ...tail]
}

const addPackageOptions = computed(() => packageOptionsForRole(form.value.role))

function permissionLabel(permission) {
  const labelKey = String(permission?.labelKey || '').trim()
  if (labelKey) {
    const translated = t(labelKey)
    if (translated && translated !== labelKey) {
      return translated
    }
  }
  return String(permission?.label || permission?.key || '')
}
const editPackageOptions = computed(() => packageOptionsForRole(editForm.value.role))

async function loadUsers() {
  loading.value = true
  error.value = null
  try {
    const res = await api.get('/users/list')
    users.value = res.data?.data || []
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

async function loadPolicies() {
  try {
    const res = await api.get('/acl/policies')
    rolePolicies.value = res.data?.data || []
  } catch {
    rolePolicies.value = []
  }
}

function policyNameById(policyID) {
  return rolePolicies.value.find((item) => item.id === policyID)?.name || policyID || t('users.role_policy_system_default')
}

async function addUser() {
  if (!form.value.username || !form.value.email) return
  addLoading.value = true
  try {
    await api.post('/users/create', {
      ...form.value,
      parent_username: form.value.role === 'admin' ? '' : (form.value.parent_username || ''),
      role_policy_id: form.value.role_policy_id || '',
    })
    showAddModal.value = false
    form.value = {
      username: '',
      email: '',
      password: '',
      role: 'user',
      parent_username: defaultParentForNewUser(),
      role_policy_id: '',
      package: addPackageOptions.value[0] || 'default',
    }
    await loadUsers()
  } catch (e) {
    error.value = e?.response?.data?.message || t('common.error')
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

function openEditModal(user) {
  editForm.value = {
    username: user.username || '',
    name: user.name || '',
    email: user.email || '',
    role: user.role || 'user',
    parent_username: user.parent_username || '',
    role_policy_id: user.role_policy_id || '',
    package: user.package || 'default',
    active: !!user.active,
    is_owner: !!user.is_owner,
  }
  showEditModal.value = true
}

function closeEditModal() {
  showEditModal.value = false
  editForm.value = { username: '', name: '', email: '', role: 'user', parent_username: '', role_policy_id: '', package: 'default', active: true, is_owner: false }
}

async function updateUser() {
  if (!editForm.value.username || !editForm.value.email) return
  editLoading.value = true
  try {
    await api.post('/users/update', {
      username: editForm.value.username,
      name: editForm.value.name,
      email: editForm.value.email,
      role: editForm.value.role,
      parent_username: editForm.value.role === 'admin' ? '' : (editForm.value.parent_username || ''),
      role_policy_id: editForm.value.role_policy_id || '',
      package: editForm.value.package,
      active: !!editForm.value.active,
    })
    closeEditModal()
    await loadUsers()
  } catch (e) {
    error.value = e?.response?.data?.message || t('common.error')
  } finally {
    editLoading.value = false
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

function resetPolicyForm() {
  policyForm.value = { id: '', name: '', description: '', permissions: [] }
}

function policyHasPermission(permissionKey) {
  return (policyForm.value.permissions || []).includes(permissionKey)
}

function togglePolicyPermission(permissionKey, checked) {
  const current = new Set(policyForm.value.permissions || [])
  if (checked) {
    current.add(permissionKey)
  } else {
    current.delete(permissionKey)
  }
  policyForm.value.permissions = Array.from(current)
}

function openRoleEditor() {
  if (!authStore.isAdmin) return
  showRoleEditorModal.value = true
  roleEditorError.value = ''
  resetPolicyForm()
  loadPolicies()
}

function closeRoleEditor() {
  showRoleEditorModal.value = false
  roleEditorError.value = ''
  resetPolicyForm()
}

function editPolicy(policy) {
  policyForm.value = {
    id: policy.id || '',
    name: policy.name || '',
    description: policy.description || '',
    permissions: [...(policy.permissions || [])],
  }
}

async function savePolicy() {
  if (!policyForm.value.name?.trim()) {
    roleEditorError.value = t('users.role_editor_name_required')
    return
  }
  roleEditorLoading.value = true
  roleEditorError.value = ''
  try {
    await api.post('/acl/policies', {
      id: policyForm.value.id || '',
      name: policyForm.value.name.trim(),
      description: (policyForm.value.description || '').trim(),
      permissions: [...(policyForm.value.permissions || [])],
    })
    resetPolicyForm()
    await Promise.all([loadPolicies(), loadUsers()])
  } catch (e) {
    roleEditorError.value = e?.response?.data?.message || t('common.error')
  } finally {
    roleEditorLoading.value = false
  }
}

async function deletePolicy(policy) {
  if (!policy?.id) return
  if (!window.confirm(t('users.role_editor_delete_confirm', { name: policy.name }))) return
  roleEditorLoading.value = true
  roleEditorError.value = ''
  try {
    await api.delete('/acl/policies', { params: { id: policy.id } })
    await Promise.all([loadPolicies(), loadUsers()])
    if (policyForm.value.id === policy.id) {
      resetPolicyForm()
    }
  } catch (e) {
    roleEditorError.value = e?.response?.data?.message || t('common.error')
  } finally {
    roleEditorLoading.value = false
  }
}

watch(addPackageOptions, (options) => {
  if (!options.length) return
  if (!options.includes(form.value.package)) {
    form.value.package = options[0]
  }
}, { immediate: true })

watch(editPackageOptions, (options) => {
  if (!options.length) return
  if (!options.includes(editForm.value.package)) {
    editForm.value.package = options[0]
  }
}, { immediate: true })

watch(() => form.value.role, (nextRole) => {
  if (nextRole === 'admin') {
    form.value.parent_username = ''
    return
  }
  if (!form.value.parent_username) {
    form.value.parent_username = defaultParentForNewUser()
  }
})

watch(() => editForm.value.role, (nextRole) => {
  if (nextRole === 'admin') {
    editForm.value.parent_username = ''
  }
})

onMounted(async () => {
  await Promise.all([loadUsers(), loadPackages(), loadPolicies()])
  if (!form.value.parent_username) {
    form.value.parent_username = defaultParentForNewUser()
  }
})
</script>
