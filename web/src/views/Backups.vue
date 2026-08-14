<template>
  <Layout>
    <div class="page">
      <h1>Yedekler</h1>
      <div v-if="error" class="alert error">{{ error }}</div>
      <div v-if="notice" class="alert ok">{{ notice }}</div>

      <div class="card">
        <div class="row">
          <div style="flex: 1">
            <label>Site</label>
            <select v-model="siteId" @change="load">
              <option v-for="s in sites" :key="s.id" :value="s.id">{{ s.name }}</option>
            </select>
          </div>
          <div style="flex: 1">
            <label>Tür</label>
            <select v-model="kind">
              <option value="files">Dosyalar</option>
              <option value="full">Tam (dosya + DB)</option>
              <option value="db">Yalnızca DB</option>
            </select>
          </div>
          <button class="btn primary" style="margin-top: 18px" :disabled="busy" @click="run">
            {{ busy ? 'Alınıyor…' : '💾 Yedek Al' }}
          </button>
        </div>
      </div>

      <div class="card">
        <h2>Geçmiş</h2>
        <table>
          <thead><tr><th>Ad</th><th>Tür</th><th>Durum</th><th>Tarih</th></tr></thead>
          <tbody>
            <tr v-for="b in backups" :key="b.id">
              <td class="mono">{{ b.location }}</td>
              <td>{{ b.kind }}</td>
              <td><span class="badge" :class="b.status === 'success' ? 'ok' : 'err'">{{ b.status }}</span></td>
              <td class="muted">{{ b.created_at }}</td>
            </tr>
            <tr v-if="!backups.length"><td colspan="4" class="muted">Yedek yok.</td></tr>
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
const kind = ref('files')
const backups = ref([])
const error = ref('')
const notice = ref('')
const busy = ref(false)

async function load() {
  if (!siteId.value) return
  try {
    backups.value = await api(`/sites/${siteId.value}/backups`)
  } catch (e) {
    error.value = e.message
  }
}

async function run() {
  if (!siteId.value) return
  busy.value = true
  error.value = ''
  notice.value = ''
  try {
    const out = await api(`/sites/${siteId.value}/backups/run`, {
      method: 'POST',
      body: { kind: kind.value },
    })
    notice.value = `Yedek alındı: ${out.name}`
    await load()
  } catch (e) {
    error.value = e.message
  } finally {
    busy.value = false
  }
}

onMounted(async () => {
  sites.value = await api('/sites').catch(() => [])
  if (sites.value.length) {
    siteId.value = sites.value[0].id
    await load()
  }
})
</script>
