
<template>
  <Layout>
    <div class="page">
      <h1>{{ $t('menu.databases') }}</h1>
      <div v-if="error" class="alert error">{{ error }}</div>
      <div v-if="notice" class="alert ok">{{ notice }}</div>

      <div class="card">
        <div class="row">
          <div style="flex: 1">
            <label>{{ $t('databases.site') }}</label>
            <select v-model="siteId" @change="loadAll">
              <option v-for="s in sites" :key="s.id" :value="s.id">{{ s.name }}</option>
            </select>
          </div>
          <div style="flex: 1">
            <label>{{ $t('databases.new_db_name') }}</label>
            <input v-model="newDb" :placeholder="$t('databases.placeholder_db')" />
          </div>
          <button class="btn primary" style="margin-top: 18px" @click="createDb">➕ {{ $t('databases.add_db') }}</button>
          <div style="flex: 1">
            <label>{{ $t('databases.new_user') }}</label>
            <input v-model="newUser" :placeholder="$t('databases.placeholder_user')" />
          </div>
          <button class="btn primary" style="margin-top: 18px" @click="createUser">➕ {{ $t('databases.add_user') }}</button>
        </div>
      </div>

      <div class="card">
        <h2>{{ $t('menu.databases') }}</h2>
        <table>
          <thead><tr><th>{{ $t('databases.name') }}</th><th>{{ $t('databases.charset') }}</th><th></th></tr></thead>
          <tbody>
            <tr v-for="d in dbs" :key="d.name">
              <td class="mono">{{ d.name }}</td>
              <td>{{ d.charset }}</td>
              <td>
                <button class="btn" @click="openAdminer(d)">🔌 {{ $t('databases.open_adminer') }}</button>
                <button class="btn danger" @click="dropDb(d)">{{ $t('common.delete') }}</button>
              </td>
            </tr>
            <tr v-if="!dbs.length"><td colspan="3" class="muted">{{ $t('databases.no_db') }}</td></tr>
          </tbody>
        </table>
      </div>

      <div class="card">
        <h2>{{ $t('databases.user') }}</h2>
        <table>
          <thead><tr><th>{{ $t('databases.user') }}</th><th>{{ $t('databases.host') }}</th><th></th></tr></thead>
          <tbody>
            <tr v-for="u in users" :key="u.username">
              <td class="mono">{{ u.username }}</td>
              <td>{{ u.host }}</td>
              <td>
                <button class="btn" @click="grant(u)">🔑 {{ $t('databases.grant') }}</button>
                <button class="btn" @click="resetPw(u)">🔁 {{ $t('databases.password') }}</button>
                <button class="btn danger" @click="dropUser(u)">{{ $t('common.delete') }}</button>
              </td>
            </tr>
            <tr v-if="!users.length"><td colspan="3" class="muted">{{ $t('databases.no_users') }}</td></tr>
          </tbody>
        </table>
      </div>
    </div>
  </Layout>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import Layout from '../components/Layout.vue'
import { api } from '../api'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()

const sites = ref([])
const siteId = ref('')
const dbs = ref([])
const users = ref([])
const newDb = ref('')
const newUser = ref('')
const error = ref('')
const notice = ref('')

async function loadAll() {
  if (!siteId.value) return
  error.value = ''
  try {
    dbs.value = await api(`/sites/${siteId.value}/databases`) || []
  } catch (e) {
    error.value = e.message
    dbs.value = []
  }
  
  try {
    users.value = await api(`/sites/${siteId.value}/db-users`) || []
  } catch (e) {
    users.value = []
  }
}

async function createDb() {
  if (!newDb.value) return
  try {
    await api(`/sites/${siteId.value}/databases`, { method: 'POST', body: { name: newDb.value } })
    newDb.value = ''
    await loadAll()
  } catch (e) {
    error.value = e.message
  }
}

async function dropDb(d) {
  if (!confirm(t('databases.delete_db_confirm', { name: d.name }))) return
  error.value = ''
  try {
    await api(`/sites/${siteId.value}/databases/${d.name}`, { method: 'DELETE' })
    await loadAll()
  } catch (e) {
    error.value = e.message
  }
}

async function createUser() {
  if (!newUser.value) return
  try {
    const out = await api(`/sites/${siteId.value}/db-users`, {
      method: 'POST',
      body: { username: newUser.value, password: '' },
    })
    notice.value = t('databases.user_created_notice', { password: out.password })
    newUser.value = ''
    await loadAll()
  } catch (e) {
    error.value = e.message
  }
}

async function dropUser(u) {
  if (!confirm(t('databases.delete_user_confirm', { name: u.username }))) return
  error.value = ''
  try {
    await api(`/sites/${siteId.value}/db-users/${u.username}`, { method: 'DELETE' })
    await loadAll()
  } catch (e) {
    error.value = e.message
  }
}

async function grant(u) {
  const db = prompt(t('databases.grant_prompt'), dbs.value[0]?.name)
  if (!db) return
  try {
    await api(`/sites/${siteId.value}/db-grant`, {
      method: 'POST',
      body: { username: u.username.replace(/^[^_]+_/, ''), database: db.replace(/^[^_]+_/, '') },
    })
    notice.value = t('databases.grant_success')
  } catch (e) {
    error.value = e.message
  }
}

async function openAdminer(d) {
  try {
    const out = await api('/adminer/open', {
      method: 'POST',
      body: { site_id: siteId.value, database_id: d.id },
    })
    notice.value = t('databases.adminer_token_notice', { token: out.token })
  } catch (e) {
    error.value = e.message
  }
}

async function resetPw(u) {
  try {
    const out = await api(`/sites/${siteId.value}/db-users/${u.username}/password`, {
      method: 'POST',
      body: { password: '' },
    })
    notice.value = t('databases.new_password_notice', { password: out.password })
  } catch (e) {
    error.value = e.message
  }
}

onMounted(async () => {
  sites.value = await api('/sites').catch(() => [])
  if (sites.value.length) {
    siteId.value = sites.value[0].id
    await loadAll()
  }
})
</script>
