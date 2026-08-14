<template>
  <Layout>
    <div class="page">
      <h1>Veritabanları</h1>
      <div v-if="error" class="alert error">{{ error }}</div>
      <div v-if="notice" class="alert ok">{{ notice }}</div>

      <div class="card">
        <div class="row">
          <div style="flex: 1">
            <label>Site</label>
            <select v-model="siteId" @change="loadAll">
              <option v-for="s in sites" :key="s.id" :value="s.id">{{ s.name }}</option>
            </select>
          </div>
          <div style="flex: 1">
            <label>Yeni DB Adı</label>
            <input v-model="newDb" placeholder="wp" />
          </div>
          <button class="btn primary" style="margin-top: 18px" @click="createDb">➕ DB</button>
          <div style="flex: 1">
            <label>Yeni Kullanıcı</label>
            <input v-model="newUser" placeholder="user" />
          </div>
          <button class="btn primary" style="margin-top: 18px" @click="createUser">➕ Kullanıcı</button>
        </div>
      </div>

      <div class="card">
        <h2>Veritabanları</h2>
        <table>
          <thead><tr><th>Ad</th><th>Charset</th><th></th></tr></thead>
          <tbody>
            <tr v-for="d in dbs" :key="d.name">
              <td class="mono">{{ d.name }}</td>
              <td>{{ d.charset }}</td>
              <td>
                <button class="btn" @click="openAdminer(d)">🔌 Adminer Aç</button>
                <button class="btn danger" @click="dropDb(d)">Sil</button>
              </td>
            </tr>
            <tr v-if="!dbs.length"><td colspan="3" class="muted">DB yok.</td></tr>
          </tbody>
        </table>
      </div>

      <div class="card">
        <h2>Kullanıcılar</h2>
        <table>
          <thead><tr><th>Kullanıcı</th><th>Host</th><th></th></tr></thead>
          <tbody>
            <tr v-for="u in users" :key="u.username">
              <td class="mono">{{ u.username }}</td>
              <td>{{ u.host }}</td>
              <td>
                <button class="btn" @click="grant(u)">🔑 Grant</button>
                <button class="btn" @click="resetPw(u)">🔁 Şifre</button>
                <button class="btn danger" @click="dropUser(u)">Sil</button>
              </td>
            </tr>
            <tr v-if="!users.length"><td colspan="3" class="muted">Kullanıcı yok.</td></tr>
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
  if (!confirm(`${d.name} KALICI olarak silinsin mi?`)) return
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
    notice.value = `Kullanıcı oluşturuldu. ŞİFRE (yalnızca bir kez): ${out.password}`
    newUser.value = ''
    await loadAll()
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
    notice.value = `Yeni şifre (yalnızca bir kez): ${out.password}`
  } catch (e) {
    error.value = e.message
  }
}

async function dropUser(u) {
  if (!confirm(`${u.username} silinsin mi?`)) return
  try {
    await api(`/sites/${siteId.value}/db-users/${u.username}`, { method: 'DELETE' })
    await loadAll()
  } catch (e) {
    error.value = e.message
  }
}

async function grant(u) {
  const db = prompt(`"${u.username}" kullanıcısına hangi DB üzerinde yetki verilsin?`, dbs.value[0]?.name)
  if (!db) return
  try {
    await api(`/sites/${siteId.value}/db-grant`, {
      method: 'POST',
      body: { username: u.username.replace(/^[^_]+_/, ''), database: db.replace(/^[^_]+_/, '') },
    })
    notice.value = 'Grant uygulandı.'
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
    notice.value = `Adminer token'ı (15 dk): ${out.token}`
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
