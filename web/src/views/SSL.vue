<template>
  <Layout>
    <div class="page">
      <h1>SSL Sertifikaları</h1>
      <div v-if="error" class="alert error">{{ error }}</div>
      <div v-if="notice" class="alert ok">{{ notice }}</div>

      <div class="card">
        <div class="row">
          <div style="flex: 1">
            <label>Site</label>
            <select v-model="siteId">
              <option v-for="s in sites" :key="s.id" :value="s.id">{{ s.name }}</option>
            </select>
          </div>
          <button class="btn primary" style="margin-top: 18px" @click="enableLE">
            🔒 Let's Encrypt Kur
          </button>
          <button class="btn danger" style="margin-top: 18px" @click="disable">
            🔓 SSL'i Kapat
          </button>
        </div>
      </div>

      <div class="card">
        <h2>Custom Sertifika</h2>
        <label>Sertifika (PEM)</label>
        <textarea v-model="certPem" rows="6" class="mono" placeholder="-----BEGIN CERTIFICATE-----"></textarea>
        <label>Özel Anahtar (PEM)</label>
        <textarea v-model="keyPem" rows="6" class="mono" placeholder="-----BEGIN PRIVATE KEY-----"></textarea>
        <button class="btn primary" style="margin-top: 10px" @click="installCustom">Kur</button>
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
const certPem = ref('')
const keyPem = ref('')
const error = ref('')
const notice = ref('')

async function enableLE() {
  if (!siteId.value) return
  error.value = ''
  notice.value = ''
  try {
    await api(`/sites/${siteId.value}/ssl/letsencrypt`, { method: 'POST', body: {} })
    notice.value = 'Let\'s Encrypt sertifikası kuruldu.'
  } catch (e) {
    error.value = e.message
  }
}

async function disable() {
  if (!siteId.value || !confirm('SSL kapatılsın mı?')) return
  try {
    await api(`/sites/${siteId.value}/ssl/disable`, { method: 'POST', body: {} })
    notice.value = 'SSL kapatıldı.'
  } catch (e) {
    error.value = e.message
  }
}

async function installCustom() {
  if (!siteId.value || !certPem.value || !keyPem.value) return
  error.value = ''
  try {
    await api(`/sites/${siteId.value}/ssl/custom`, {
      method: 'POST',
      body: { cert_pem: certPem.value, key_pem: keyPem.value },
    })
    notice.value = 'Custom sertifika kuruldu.'
    certPem.value = ''
    keyPem.value = ''
  } catch (e) {
    error.value = e.message
  }
}

onMounted(async () => {
  sites.value = await api('/sites').catch(() => [])
  if (sites.value.length) siteId.value = sites.value[0].id
})
</script>
