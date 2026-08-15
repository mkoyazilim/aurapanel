<script setup>
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import Layout from '../components/Layout.vue'
import { api } from '../api.js'

const { t } = useI18n()
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
    error.value = e.message || t('users.load_error')
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
    error.value = e.message || t('users.create_error')
    return
  }
  notice.value = t('users.create_success')
  form.value = { username: '', password: '', role: 'user' }
  loadUsers()
}

async function deleteUser(id) {
  if (!confirm(t('users.confirm_delete'))) return
  try {
    await api.delete(`/users/${id}`)
  } catch (e) {
    error.value = e.message || t('users.delete_error')
    return
  }
  loadUsers()
}

onMounted(loadUsers)
</script>

<template>
  <Layout>
    <div class="page">
      <h1>{{ $t('users.title') }}</h1>
      <div v-if="error" class="alert error">{{ error }}</div>
      <div v-if="notice" class="alert ok">{{ notice }}</div>

      <div class="card">
        <div class="row">
          <div style="flex: 1">
            <label>{{ $t('users.username_label') }}</label>
            <input v-model="form.username" required />
          </div>
          <div style="flex: 1">
            <label>{{ $t('users.password_label') }}</label>
            <input v-model="form.password" type="password" required />
          </div>
          <div style="flex: 1">
            <label>{{ $t('users.role_label') }}</label>
            <select v-model="form.role">
              <option value="user">{{ $t('users.role_user') }}</option>
              <option value="reseller">{{ $t('users.role_reseller') }}</option>
              <option value="admin">{{ $t('users.role_admin') }}</option>
            </select>
          </div>
          <button class="btn primary" style="margin-top: 18px" @click="createUser">➕ {{ $t('users.create_user') }}</button>
        </div>
      </div>

      <div class="card">
        <h2>{{ $t('users.subtitle') }}</h2>
        <div v-if="loading" class="muted">{{ $t('users.loading') }}</div>
        <table v-else>
          <thead>
            <tr>
              <th>{{ $t('users.table_id') }}</th>
              <th>{{ $t('users.table_username') }}</th>
              <th>{{ $t('users.table_role') }}</th>
              <th>{{ $t('users.table_status') }}</th>
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
                <button class="btn danger" @click="deleteUser(u.ID)">{{ $t('users.delete_button') }}</button>
              </td>
            </tr>
            <tr v-if="!users.length"><td colspan="5" class="muted">{{ $t('users.no_users') }}</td></tr>
          </tbody>
        </table>
      </div>
    </div>
  </Layout>
</template>
