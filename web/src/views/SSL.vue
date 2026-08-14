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
            <select v-model="siteId" @change="loadInfo">
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

      <div class="card" v-if="info && info.enabled">
        <h2>Aktif Sertifika Bilgileri</h2>
        <div style="background: var(--bg-body); padding: 16px; border-radius: var(--radius); font-size: 14px;">
          <p><strong>Domain:</strong> <span class="mono">{{ info.domain }}</span></p>
          <p><strong>Sağlayıcı (Issuer):</strong> {{ info.issuer }}</p>
          <p><strong>Başlangıç:</strong> {{ new Date(info.not_before).toLocaleString() }}</p>
          <p><strong>Bitiş (Geçerlilik):</strong> {{ new Date(info.not_after).toLocaleString() }}</p>
          <p><strong>Otomatik Yenileme:</strong>
            <span :class="info.auto_renew ? 'badge ok' : 'badge err'">
              {{ info.auto_renew ? 'Açık' : 'Kapalı' }}
            </span>
          </p>
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
const info = ref(null)

async function loadInfo() {
  if (!siteId.value) {
    info.value = null
    return
  }
  try {
    info.value = await api(`/sites/${siteId.value}/ssl`)
  } catch (e) {
    info.value = null
  }
}

async function enableLE() {
  if (!siteId.value) return
  error.value = ''
  notice.value = ''
  try {
    await api(`/sites/${siteId.value}/ssl/letsencrypt`, { method: 'POST', body: {} })
    notice.value = 'Let\'s Encrypt sertifikası kuruldu.'
    await loadInfo()
  } catch (e) {
    error.value = e.message
  }
}

async function disable() {
  if (!siteId.value || !confirm('SSL kapatılsın mı?')) return
  try {
    await api(`/sites/${siteId.value}/ssl/disable`, { method: 'POST', body: {} })
    notice.value = 'SSL kapatıldı.'
    await loadInfo()
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
    await loadInfo()
  } catch (e) {
    error.value = e.message
  }
}

onMounted(async () => {
  sites.value = await api('/sites').catch(() => [])
  if (sites.value.length) {
    siteId.value = sites.value[0].id
    await loadInfo()
  }
})
</script>
