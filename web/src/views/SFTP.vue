<template>
  <Layout>
    <div class="page">
      <h1>SFTP Hesapları</h1>
      <div v-if="error" class="alert error">{{ error }}</div>
      <div v-if="notice" class="alert ok">{{ notice }}</div>
      <div v-if="passwordNotice" class="alert warning" style="background:#fffbeb; color:#92400e; border:1px solid #fcd34d;">
        <strong>Önemli:</strong> SFTP Parolası yalnızca bir kez gösterilir! Lütfen kaydedin: <code style="font-size: 1.1em; padding: 4px; background:#fef3c7;">{{ passwordNotice }}</code>
      </div>

      <div class="card">
        <div class="row">
          <div style="flex: 1">
            <label>Site Seçin</label>
            <select v-model="siteId" @change="loadAccounts">
              <option value="" disabled>Lütfen bir site seçin...</option>
              <option v-for="s in sites" :key="s.id" :value="s.id">{{ s.name }}</option>
            </select>
          </div>
          <div style="flex: 1">
            <label>Yeni Hesap Adı</label>
            <input v-model="newUser" placeholder="Kullanıcı adı (Sadece küçük harf ve rakam)" />
          </div>
          <button class="btn primary" style="margin-top: 18px" :disabled="!siteId || !newUser" @click="createAccount">➕ Hesap Oluştur</button>
        </div>
      </div>

      <div class="card" v-if="siteId">
        <h2>{{ siteId }} - Mevcut SFTP Hesapları</h2>
        <table>
          <thead><tr><th>Hesap Adı</th><th>Bağlantı Portu</th><th></th></tr></thead>
          <tbody>
            <tr v-for="a in accounts" :key="a.username">
              <td class="mono">{{ a.username }}</td>
              <td>2222 (veya sunucu SSH portu)</td>
              <td style="text-align: right;">
                <button class="btn danger" @click="deleteAccount(a.username)">Sil</button>
              </td>
            </tr>
            <tr v-if="!accounts.length"><td colspan="3" class="muted">Hiç SFTP hesabı bulunamadı.</td></tr>
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
const accounts = ref([])
const newUser = ref('')
const error = ref('')
const notice = ref('')
const passwordNotice = ref('')

onMounted(async () => {
  try {
    const raw = await api('/sites')
    sites.value = raw || []
  } catch (e) {
    error.value = e.message
  }
})

async function loadAccounts() {
  if (!siteId.value) return
  error.value = ''
  notice.value = ''
  passwordNotice.value = ''
  try {
    accounts.value = await api(`/sites/${siteId.value}/sftp`) || []
  } catch (e) {
    error.value = e.message
  }
}

async function createAccount() {
  error.value = ''
  notice.value = ''
  passwordNotice.value = ''
  if (!newUser.value) return
  try {
    const res = await api(`/sites/${siteId.value}/sftp`, {
      method: 'POST',
      body: JSON.stringify({ username: newUser.value, password: '' })
    })
    passwordNotice.value = res.password
    notice.value = `${newUser.value} hesabı başarıyla oluşturuldu!`
    newUser.value = ''
    await loadAccounts()
  } catch (e) {
    error.value = e.message
  }
}

async function deleteAccount(username) {
  if (!confirm(`'${username}' hesabını silmek istediğinize emin misiniz?`)) return
  error.value = ''
  notice.value = ''
  passwordNotice.value = ''
  try {
    await api(`/sites/${siteId.value}/sftp/${username}`, { method: 'DELETE' })
    notice.value = 'Hesap silindi.'
    await loadAccounts()
  } catch (e) {
    error.value = e.message
  }
}
</script>
