<template>
  <Layout>
    <div class="page">
      <h1>Siteler</h1>
      <div v-if="error" class="alert error">{{ error }}</div>

      <div class="card">
        <h2>Yeni Site</h2>
        <div class="row">
          <div style="flex: 2">
            <label>Domain</label>
            <input v-model="newSite.domain" placeholder="example.com" />
          </div>
          <div style="flex: 1">
            <label>PHP Sürümü</label>
            <select v-model="newSite.php">
              <option>8.2</option><option selected>8.3</option><option>8.4</option>
            </select>
          </div>
          <button class="btn primary" @click="create" :disabled="busy">Oluştur</button>
        </div>
      </div>

      <div class="card">
        <h2>Mevcut Siteler</h2>
        <table>
          <thead><tr><th>ID</th><th>Domain</th><th>Linux Kullanıcısı</th><th>Durum</th><th></th></tr></thead>
          <tbody>
            <tr v-for="s in sites" :key="s.id">
              <td class="mono">{{ s.id }}</td>
              <td>{{ s.name }}</td>
              <td class="mono">{{ s.linux_user }}</td>
              <td><span class="badge" :class="s.status === 'active' ? 'ok' : 'warn'">{{ s.status }}</span></td>
              <td><button class="btn danger" @click="remove(s.id)">Sil</button></td>
            </tr>
            <tr v-if="!sites.length"><td colspan="5" class="muted">Henüz site yok.</td></tr>
          </tbody>
        </table>
      </div>
    </div>
  </Layout>
</template>

<script setup>
import { onMounted, reactive, ref } from 'vue'
import Layout from '../components/Layout.vue'
import { api } from '../api'

const sites = ref([])
const error = ref('')
const busy = ref(false)
const newSite = reactive({ domain: '', php: '8.3' })

async function load() {
  try {
    sites.value = await api('/sites')
  } catch (e) {
    error.value = e.message
  }
}

async function create() {
  busy.value = true
  error.value = ''
  try {
    await api('/sites', {
      method: 'POST',
      body: { domain: newSite.domain, php_version: newSite.php, aliases: [], limits: {} },
    })
    newSite.domain = ''
    await load()
  } catch (e) {
    error.value = e.message
  } finally {
    busy.value = false
  }
}

async function remove(id) {
  if (!confirm(`${id} silinsin mi? Bu işlem izolasyon katmanını kaldırır.`)) return
  try {
    await api(`/sites/${id}`, { method: 'DELETE' })
    await load()
  } catch (e) {
    error.value = e.message
  }
}

onMounted(load)
</script>
