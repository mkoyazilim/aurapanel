<template>
  <div class="space-y-6">
    <div class="flex items-center justify-between gap-3">
      <div>
        <h1 class="text-2xl font-bold text-white">SFTP Manager</h1>
        <p class="text-gray-400 mt-1">OpenSSH tabanli SFTP kullanici yonetimi.</p>
      </div>
      <button class="btn-primary" @click="showCreate = true">SFTP Kullanici Ekle</button>
    </div>

    <div v-if="error" class="aura-card border-red-500/30 bg-red-500/5 text-red-400">{{ error }}</div>
    <div v-if="success" class="aura-card border-green-500/30 bg-green-500/5 text-green-300">{{ success }}</div>

    <div class="aura-card space-y-4">
      <div class="flex justify-end">
        <button class="btn-secondary" @click="loadUsers">Yenile</button>
      </div>

      <div class="overflow-x-auto">
        <table class="w-full text-sm">
          <thead>
            <tr class="border-b border-panel-border text-gray-400">
              <th class="text-left py-2 px-2">Username</th>
              <th class="text-left py-2 px-2">Home</th>
              <th class="text-left py-2 px-2">Created</th>
              <th class="text-right py-2 px-2">Islem</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="item in users" :key="item.username" class="border-b border-panel-border/50">
              <td class="py-2 px-2 text-white font-mono">{{ item.username }}</td>
              <td class="py-2 px-2 text-gray-300 font-mono text-xs break-all">{{ item.home_dir }}</td>
              <td class="py-2 px-2 text-gray-400">{{ formatTime(item.created_at) }}</td>
              <td class="py-2 px-2 text-right">
                <div class="flex justify-end gap-2">
                  <button class="btn-secondary px-2 py-1 text-xs" @click="openReset(item.username)">Sifre</button>
                  <button class="btn-danger px-2 py-1 text-xs" @click="removeUser(item.username)">Sil</button>
                </div>
              </td>
            </tr>
            <tr v-if="users.length === 0">
              <td colspan="4" class="text-center py-8 text-gray-500">SFTP kullanicisi bulunamadi.</td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <Teleport to="body">
      <div v-if="showCreate" class="fixed inset-0 bg-black/60 flex items-center justify-center z-50 p-4">
        <div class="bg-panel-card border border-panel-border rounded-2xl p-6 w-full max-w-lg">
          <h2 class="text-xl font-bold text-white mb-4">SFTP Kullanici Olustur</h2>
          <div class="grid grid-cols-1 md:grid-cols-2 gap-3">
            <div>
              <label class="block text-sm text-gray-400 mb-1">Username</label>
              <input v-model="createForm.username" class="aura-input w-full" placeholder="siteuser" />
            </div>
            <div>
              <label class="block text-sm text-gray-400 mb-1">Password</label>
              <input v-model="createForm.password" type="password" class="aura-input w-full" />
            </div>
            <div class="md:col-span-2">
              <label class="block text-sm text-gray-400 mb-1">Home Directory</label>
              <input v-model="createForm.home_dir" class="aura-input w-full" placeholder="/home/siteuser" />
            </div>
          </div>
          <div class="flex gap-3 mt-6">
            <button class="btn-secondary flex-1" @click="showCreate = false">Iptal</button>
            <button class="btn-primary flex-1" @click="createUser">Olustur</button>
          </div>
        </div>
      </div>
    </Teleport>

    <Teleport to="body">
      <div v-if="showReset" class="fixed inset-0 bg-black/60 flex items-center justify-center z-50 p-4">
        <div class="bg-panel-card border border-panel-border rounded-2xl p-6 w-full max-w-md">
          <h2 class="text-xl font-bold text-white mb-4">SFTP Sifre Guncelle</h2>
          <p class="text-sm text-gray-400 mb-3">Kullanici: <span class="text-white font-mono">{{ resetForm.username }}</span></p>
          <input v-model="resetForm.new_password" type="password" class="aura-input w-full" placeholder="Yeni sifre" />
          <div class="flex gap-3 mt-6">
            <button class="btn-secondary flex-1" @click="showReset = false">Iptal</button>
            <button class="btn-primary flex-1" @click="updatePassword">Guncelle</button>
          </div>
        </div>
      </div>
    </Teleport>
  </div>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import api from '../services/api'

const users = ref([])
const error = ref('')
const success = ref('')
const showCreate = ref(false)
const showReset = ref(false)

const createForm = ref({
  username: '',
  password: '',
  home_dir: '/home',
})

const resetForm = ref({
  username: '',
  new_password: '',
})

function apiErrorMessage(e, fallback) {
  return e?.response?.data?.message || e?.message || fallback
}

function formatTime(ts) {
  const value = Number(ts || 0)
  if (!value) return '-'
  return new Date(value * 1000).toLocaleString()
}

async function loadUsers() {
  error.value = ''
  try {
    const res = await api.get('/sftp/list')
    users.value = res.data?.data || []
  } catch (e) {
    error.value = apiErrorMessage(e, 'SFTP kullanici listesi alinamadi')
  }
}

async function createUser() {
  error.value = ''
  success.value = ''
  if (!createForm.value.username || !createForm.value.password || !createForm.value.home_dir) {
    error.value = 'username, password ve home_dir zorunludur.'
    return
  }
  try {
    const res = await api.post('/sftp/create', createForm.value)
    success.value = res.data?.message || 'SFTP kullanici olusturuldu.'
    showCreate.value = false
    createForm.value = { username: '', password: '', home_dir: '/home' }
    await loadUsers()
  } catch (e) {
    error.value = apiErrorMessage(e, 'SFTP kullanici olusturulamadi')
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
    error.value = 'username ve yeni sifre zorunludur.'
    return
  }
  try {
    const res = await api.post('/sftp/password', resetForm.value)
    success.value = res.data?.message || 'SFTP sifresi guncellendi.'
    showReset.value = false
  } catch (e) {
    error.value = apiErrorMessage(e, 'SFTP sifresi guncellenemedi')
  }
}

async function removeUser(username) {
  if (!confirm(`${username} kullanicisi silinsin mi?`)) return
  error.value = ''
  success.value = ''
  try {
    const res = await api.post('/sftp/delete', { username })
    success.value = res.data?.message || 'SFTP kullanici silindi.'
    await loadUsers()
  } catch (e) {
    error.value = apiErrorMessage(e, 'SFTP kullanici silinemedi')
  }
}

onMounted(loadUsers)
</script>
