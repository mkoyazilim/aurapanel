<script setup>
import { ref, onMounted } from 'vue'
import Layout from '../components/Layout.vue'
import { api } from '../api.js'

const users = ref([])
const loading = ref(true)
const form = ref({ username: '', password: '', role: 'user' })
const error = ref('')
const notice = ref('')

async function loadUsers() {
  loading.value = true
  try {
    users.value = await api.get('/users') || []
  } catch (e) {
    error.value = e.message || 'Yükleme hatası'
  } finally {
    loading.value = false
  }
}

async function createUser() {
  error.value = ''
  notice.value = ''
  if (!form.value.username || !form.value.password) return
  try {
    await api.post('/users', form.value)
  } catch (e) {
    error.value = e.message || 'Oluşturma hatası'
    return
  }
  notice.value = 'Kullanıcı oluşturuldu.'
  form.value = { username: '', password: '', role: 'user' }
  loadUsers()
}

async function deleteUser(id) {
  if (!confirm('Emin misiniz?')) return
  try {
    await api.delete(`/users/${id}`)
  } catch (e) {
    error.value = e.message || 'Silme hatası'
    return
  }
  loadUsers()

  loadUsers()
}

onMounted(loadUsers)
</script>

<template>
  <Layout>
    <div class="page">
      <h1>Users</h1>
      <div v-if="error" class="alert error">{{ error }}</div>
      <div v-if="notice" class="alert ok">{{ notice }}</div>

      <div class="card">
        <div class="row">
          <div style="flex: 1">
            <label>Username</label>
            <input v-model="form.username" required />
          </div>
          <div style="flex: 1">
            <label>Password</label>
            <input v-model="form.password" type="password" required />
          </div>
          <div style="flex: 1">
            <label>Role</label>
            <select v-model="form.role">
              <option value="user">User</option>
              <option value="reseller">Reseller</option>
              <option value="admin">Admin</option>
            </select>
          </div>
          <button class="btn primary" style="margin-top: 18px" @click="createUser">➕ Create User</button>
        </div>
      </div>

      <div class="card">
        <h2>Users</h2>
        <div v-if="loading" class="muted">Loading users...</div>
        <table v-else>
          <thead>
            <tr>
              <th>ID</th>
              <th>Username</th>
              <th>Role</th>
              <th>Status</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="u in users" :key="u.ID">
              <td class="mono">{{ u.ID }}</td>
              <td>{{ u.Username }}</td>
              <td>{{ u.RoleID }}</td>
              <td>
                <span class="badge" :class="u.Status === 'active' ? 'ok' : 'error'">{{ u.Status }}</span>
              </td>
              <td>
                <button class="btn danger" @click="deleteUser(u.ID)">Delete</button>
              </td>
            </tr>
            <tr v-if="!users.length"><td colspan="5" class="muted">No users found.</td></tr>
          </tbody>
        </table>
      </div>
    </div>
  </Layout>
</template>
