
<template>
  <Layout>
    <div class="page">
      <h1>{{ $t('menu.ssl') }}</h1>
      <div v-if="error" class="alert error">{{ error }}</div>
      <div v-if="notice" class="alert ok">{{ notice }}</div>

      <div class="card">
        <div class="row">
          <div style="flex: 1">
            <label>{{ $t('ssl.site') }}</label>
            <select v-model="siteId" @change="loadInfo">
              <option v-for="s in sites" :key="s.id" :value="s.id">{{ s.name }}</option>
            </select>
          </div>
          <button class="btn primary" style="margin-top: 18px" @click="enableLE">
            🔒 {{ $t('ssl.install_le') }}
          </button>
          <button class="btn danger" style="margin-top: 18px" @click="disable">
            🔓 {{ $t('ssl.disable_ssl') }}
          </button>
        </div>
      </div>

      <div class="card" v-if="info && info.enabled">
        <h2>{{ $t('ssl.active_cert') }}</h2>
        <div style="background: var(--bg-body); padding: 16px; border-radius: var(--radius); font-size: 14px;">
          <p><strong>{{ $t('ssl.domain') }}:</strong> <span class="mono">{{ info.domain }}</span></p>
          <p><strong>{{ $t('ssl.issuer') }}:</strong> {{ info.issuer }}</p>
          <p><strong>{{ $t('ssl.valid_from') }}:</strong> {{ new Date(info.not_before).toLocaleString() }}</p>
          <p><strong>{{ $t('ssl.valid_to') }}:</strong> {{ new Date(info.not_after).toLocaleString() }}</p>
          <p><strong>{{ $t('ssl.auto_renew') }}:</strong>
            <span :class="info.auto_renew ? 'badge ok' : 'badge err'">
              {{ info.auto_renew ? $t('ssl.on') : $t('ssl.off') }}
            </span>
          </p>
        </div>
      </div>

      <div class="card">
        <h2>{{ $t('ssl.custom_cert') }}</h2>
        <label>{{ $t('ssl.cert_pem') }}</label>
        <textarea v-model="certPem" rows="6" class="mono" placeholder="-----BEGIN CERTIFICATE-----"></textarea>
        <label>{{ $t('ssl.key_pem') }}</label>
        <textarea v-model="keyPem" rows="6" class="mono" placeholder="-----BEGIN PRIVATE KEY-----"></textarea>
        <button class="btn primary" style="margin-top: 10px" @click="installCustom">{{ $t('ssl.install') }}</button>
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
    notice.value = t('ssl.le_success')
    await loadInfo()
  } catch (e) {
    error.value = e.message
  }
}

async function disable() {
  if (!siteId.value || !confirm(t('ssl.disable_confirm'))) return
  try {
    await api(`/sites/${siteId.value}/ssl/disable`, { method: 'POST', body: {} })
    notice.value = t('ssl.disable_success')
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
    notice.value = t('ssl.custom_cert_success')
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
