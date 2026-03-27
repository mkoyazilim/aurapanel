<template>
  <div class="space-y-6">
    <div class="flex items-center justify-between">
      <div>
        <h1 class="text-2xl font-bold text-white">Reseller & ACL Center</h1>
        <p class="text-gray-400 mt-1">Reseller quota, white-label ve ACL policy yonetimi.</p>
      </div>
      <button class="btn-secondary" @click="loadAll">Yenile</button>
    </div>

    <div v-if="error" class="aura-card border-red-500/30 bg-red-500/5 text-red-400">{{ error }}</div>
    <div v-if="success" class="aura-card border-green-500/30 bg-green-500/5 text-green-300">{{ success }}</div>

    <div class="border-b border-panel-border">
      <nav class="flex gap-5">
        <button @click="tab='quotas'" :class="tabClass('quotas')">Quotas</button>
        <button @click="tab='whitelabel'" :class="tabClass('whitelabel')">White Label</button>
        <button @click="tab='policies'" :class="tabClass('policies')">ACL Policies</button>
        <button @click="tab='assignments'" :class="tabClass('assignments')">ACL Assignments</button>
      </nav>
    </div>

    <div v-if="tab==='quotas'" class="aura-card space-y-4">
      <h2 class="text-white font-semibold">Reseller Quota</h2>
      <div class="grid grid-cols-1 md:grid-cols-6 gap-3">
        <select v-model="quotaForm.username" class="aura-input">
          <option disabled value="">Kullanici</option>
          <option v-for="u in users" :key="u.username" :value="u.username">{{ u.username }}</option>
        </select>
        <input v-model="quotaForm.plan" class="aura-input" placeholder="Plan" />
        <input v-model.number="quotaForm.disk_gb" type="number" class="aura-input" placeholder="Disk GB" />
        <input v-model.number="quotaForm.bandwidth_gb" type="number" class="aura-input" placeholder="Bandwidth GB" />
        <input v-model.number="quotaForm.max_sites" type="number" class="aura-input" placeholder="Max Sites" />
        <button class="btn-primary" @click="saveQuota">Kaydet</button>
      </div>
      <div class="overflow-x-auto">
        <table class="w-full text-sm">
          <thead>
            <tr class="border-b border-panel-border text-gray-400">
              <th class="text-left py-2 px-2">User</th>
              <th class="text-left py-2 px-2">Plan</th>
              <th class="text-left py-2 px-2">Disk</th>
              <th class="text-left py-2 px-2">Bandwidth</th>
              <th class="text-left py-2 px-2">Max Sites</th>
              <th class="text-right py-2 px-2">Islem</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="q in quotas" :key="q.username" class="border-b border-panel-border/40">
              <td class="py-2 px-2 text-white">{{ q.username }}</td>
              <td class="py-2 px-2 text-gray-300">{{ q.plan }}</td>
              <td class="py-2 px-2 text-gray-300">{{ q.disk_gb }} GB</td>
              <td class="py-2 px-2 text-gray-300">{{ q.bandwidth_gb }} GB</td>
              <td class="py-2 px-2 text-gray-300">{{ q.max_sites }}</td>
              <td class="py-2 px-2 text-right">
                <button class="btn-secondary px-2 py-1 text-xs" @click="quotaForm = { ...q }">Duzenle</button>
              </td>
            </tr>
            <tr v-if="quotas.length===0"><td colspan="6" class="text-center py-6 text-gray-500">Quota kaydi yok.</td></tr>
          </tbody>
        </table>
      </div>
    </div>

    <div v-if="tab==='whitelabel'" class="aura-card space-y-4">
      <h2 class="text-white font-semibold">White Label</h2>
      <div class="grid grid-cols-1 md:grid-cols-4 gap-3">
        <select v-model="wlForm.username" class="aura-input">
          <option disabled value="">Kullanici</option>
          <option v-for="u in users" :key="u.username" :value="u.username">{{ u.username }}</option>
        </select>
        <input v-model="wlForm.panel_name" class="aura-input" placeholder="Panel Name" />
        <input v-model="wlForm.logo_url" class="aura-input md:col-span-2" placeholder="https://.../logo.png" />
      </div>
      <button class="btn-primary" @click="saveWhiteLabel">Kaydet</button>
      <div class="overflow-x-auto">
        <table class="w-full text-sm">
          <thead>
            <tr class="border-b border-panel-border text-gray-400">
              <th class="text-left py-2 px-2">User</th>
              <th class="text-left py-2 px-2">Panel Name</th>
              <th class="text-left py-2 px-2">Logo URL</th>
              <th class="text-right py-2 px-2">Islem</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="w in whiteLabels" :key="w.username" class="border-b border-panel-border/40">
              <td class="py-2 px-2 text-white">{{ w.username }}</td>
              <td class="py-2 px-2 text-gray-300">{{ w.panel_name }}</td>
              <td class="py-2 px-2 text-gray-400 font-mono text-xs break-all">{{ w.logo_url || '-' }}</td>
              <td class="py-2 px-2 text-right">
                <button class="btn-secondary px-2 py-1 text-xs" @click="wlForm = { ...w }">Duzenle</button>
              </td>
            </tr>
            <tr v-if="whiteLabels.length===0"><td colspan="4" class="text-center py-6 text-gray-500">White-label kaydi yok.</td></tr>
          </tbody>
        </table>
      </div>
    </div>

    <div v-if="tab==='policies'" class="aura-card space-y-4">
      <h2 class="text-white font-semibold">ACL Policies</h2>
      <div class="grid grid-cols-1 md:grid-cols-4 gap-3">
        <input v-model="policyForm.name" class="aura-input" placeholder="Policy Name" />
        <input v-model="policyForm.description" class="aura-input" placeholder="Description" />
        <input v-model="policyPermissionsRaw" class="aura-input md:col-span-2" placeholder="permissions (comma): websites.read,sites.write" />
      </div>
      <div class="flex gap-2">
        <button class="btn-primary" @click="savePolicy">Kaydet</button>
        <button class="btn-secondary" @click="resetPolicyForm">Temizle</button>
      </div>
      <div class="space-y-2">
        <div v-for="p in policies" :key="p.id" class="aura-card border border-panel-border/60">
          <div class="flex items-center justify-between gap-3">
            <div>
              <p class="text-white font-semibold">{{ p.name }}</p>
              <p class="text-xs text-gray-400">{{ p.description }}</p>
              <p class="text-xs text-gray-500 mt-1 font-mono">{{ (p.permissions || []).join(', ') }}</p>
            </div>
            <div class="flex gap-2">
              <button class="btn-secondary px-2 py-1 text-xs" @click="editPolicy(p)">Duzenle</button>
              <button class="btn-danger px-2 py-1 text-xs" @click="deletePolicy(p.id)">Sil</button>
            </div>
          </div>
        </div>
        <p v-if="policies.length===0" class="text-sm text-gray-500">Policy yok.</p>
      </div>
    </div>

    <div v-if="tab==='assignments'" class="aura-card space-y-4">
      <h2 class="text-white font-semibold">ACL Assignments</h2>
      <div class="grid grid-cols-1 md:grid-cols-3 gap-3">
        <select v-model="assignmentForm.username" class="aura-input">
          <option disabled value="">Kullanici</option>
          <option v-for="u in users" :key="u.username" :value="u.username">{{ u.username }}</option>
        </select>
        <select v-model="assignmentForm.policy_id" class="aura-input">
          <option disabled value="">Policy</option>
          <option v-for="p in policies" :key="p.id" :value="p.id">{{ p.name }}</option>
        </select>
        <button class="btn-primary" @click="saveAssignment">Ata</button>
      </div>

      <div class="overflow-x-auto">
        <table class="w-full text-sm">
          <thead>
            <tr class="border-b border-panel-border text-gray-400">
              <th class="text-left py-2 px-2">User</th>
              <th class="text-left py-2 px-2">Policy</th>
              <th class="text-left py-2 px-2">Effective Permissions</th>
              <th class="text-right py-2 px-2">Islem</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="a in assignments" :key="a.username" class="border-b border-panel-border/40">
              <td class="py-2 px-2 text-white">{{ a.username }}</td>
              <td class="py-2 px-2 text-gray-300">{{ policyName(a.policy_id) }}</td>
              <td class="py-2 px-2 text-gray-400 text-xs font-mono break-all">{{ (effectiveMap[a.username] || []).join(', ') }}</td>
              <td class="py-2 px-2 text-right">
                <button class="btn-danger px-2 py-1 text-xs" @click="deleteAssignment(a.username)">Sil</button>
              </td>
            </tr>
            <tr v-if="assignments.length===0"><td colspan="4" class="text-center py-6 text-gray-500">Assignment yok.</td></tr>
          </tbody>
        </table>
      </div>
    </div>
  </div>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import api from '../services/api'

const tab = ref('quotas')
const error = ref('')
const success = ref('')

const users = ref([])
const quotas = ref([])
const whiteLabels = ref([])
const policies = ref([])
const assignments = ref([])
const effectiveMap = ref({})

const quotaForm = ref({ username: '', plan: '', disk_gb: 10, bandwidth_gb: 0, max_sites: 1, updated_at: 0 })
const wlForm = ref({ username: '', panel_name: '', logo_url: '', updated_at: 0 })
const policyForm = ref({ id: '', name: '', description: '', permissions: [], updated_at: 0 })
const policyPermissionsRaw = ref('')
const assignmentForm = ref({ username: '', policy_id: '', updated_at: 0 })

function tabClass(key) {
  return [
    'pb-3 text-sm font-medium transition',
    tab.value === key ? 'text-brand-400 border-b-2 border-brand-400' : 'text-gray-400 hover:text-white',
  ]
}

function apiErrorMessage(e, fallback) {
  return e?.response?.data?.message || e?.message || fallback
}

function policyName(id) {
  return policies.value.find(p => p.id === id)?.name || id
}

function resetPolicyForm() {
  policyForm.value = { id: '', name: '', description: '', permissions: [], updated_at: 0 }
  policyPermissionsRaw.value = ''
}

function editPolicy(item) {
  policyForm.value = { ...item }
  policyPermissionsRaw.value = (item.permissions || []).join(', ')
}

async function loadUsers() {
  const res = await api.get('/users/list')
  users.value = res.data?.data || []
}

async function loadQuotas() {
  const res = await api.get('/reseller/quotas')
  quotas.value = res.data?.data || []
}

async function loadWhiteLabels() {
  const res = await api.get('/reseller/whitelabel')
  whiteLabels.value = res.data?.data || []
}

async function loadPolicies() {
  const res = await api.get('/acl/policies')
  policies.value = res.data?.data || []
}

async function loadAssignments() {
  const res = await api.get('/acl/assignments')
  assignments.value = res.data?.data || []
  const map = {}
  await Promise.all((assignments.value || []).map(async (item) => {
    try {
      const perms = await api.get('/acl/effective', { params: { username: item.username } })
      map[item.username] = perms.data?.data || []
    } catch {
      map[item.username] = []
    }
  }))
  effectiveMap.value = map
}

async function loadAll() {
  error.value = ''
  success.value = ''
  try {
    await Promise.all([loadUsers(), loadQuotas(), loadWhiteLabels(), loadPolicies(), loadAssignments()])
  } catch (e) {
    error.value = apiErrorMessage(e, 'Reseller/ACL verileri alinamadi')
  }
}

async function saveQuota() {
  error.value = ''
  success.value = ''
  try {
    await api.post('/reseller/quotas', quotaForm.value)
    success.value = 'Quota kaydedildi.'
    await loadQuotas()
  } catch (e) {
    error.value = apiErrorMessage(e, 'Quota kaydedilemedi')
  }
}

async function saveWhiteLabel() {
  error.value = ''
  success.value = ''
  try {
    await api.post('/reseller/whitelabel', wlForm.value)
    success.value = 'White-label kaydedildi.'
    await loadWhiteLabels()
  } catch (e) {
    error.value = apiErrorMessage(e, 'White-label kaydedilemedi')
  }
}

async function savePolicy() {
  error.value = ''
  success.value = ''
  try {
    const payload = {
      ...policyForm.value,
      permissions: policyPermissionsRaw.value
        .split(',')
        .map(x => x.trim())
        .filter(Boolean),
    }
    await api.post('/acl/policies', payload)
    success.value = 'ACL policy kaydedildi.'
    resetPolicyForm()
    await loadPolicies()
  } catch (e) {
    error.value = apiErrorMessage(e, 'ACL policy kaydedilemedi')
  }
}

async function deletePolicy(id) {
  if (!confirm('Policy silinsin mi?')) return
  error.value = ''
  success.value = ''
  try {
    await api.delete('/acl/policies', { params: { id } })
    success.value = 'ACL policy silindi.'
    await loadAll()
  } catch (e) {
    error.value = apiErrorMessage(e, 'ACL policy silinemedi')
  }
}

async function saveAssignment() {
  error.value = ''
  success.value = ''
  try {
    await api.post('/acl/assignments', assignmentForm.value)
    success.value = 'ACL atamasi kaydedildi.'
    await loadAssignments()
  } catch (e) {
    error.value = apiErrorMessage(e, 'ACL atamasi kaydedilemedi')
  }
}

async function deleteAssignment(username) {
  if (!confirm('Atama silinsin mi?')) return
  error.value = ''
  success.value = ''
  try {
    await api.delete('/acl/assignments', { params: { username } })
    success.value = 'ACL atamasi silindi.'
    await loadAssignments()
  } catch (e) {
    error.value = apiErrorMessage(e, 'ACL atamasi silinemedi')
  }
}

onMounted(loadAll)
</script>
