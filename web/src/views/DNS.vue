<script setup>
import { ref, onMounted } from 'vue'
import Layout from '../components/Layout.vue'

const zones = ref([])
const loading = ref(true)
const form = ref({ domain: '', nameservers: 'ns1.example.com,ns2.example.com' })
const error = ref('')
const notice = ref('')

async function loadZones() {
  loading.value = true
  try {
    const res = await fetch('/api/v1/dns/zones')
    if (res.ok) zones.value = await res.json()
  } finally {
    loading.value = false
  }
}

async function createZone() {
  error.value = ''
  notice.value = ''
  if (!form.value.domain) return
  const nsList = form.value.nameservers.split(',').map(s => s.trim()).filter(s => s)
  const res = await fetch('/api/v1/dns/zones', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ domain: form.value.domain, nameservers: nsList })
  })
  if (!res.ok) {
    const data = await res.json()
    error.value = data.error || 'Failed to create zone'
    return
  }
  notice.value = 'DNS Zone created successfully.'
  form.value = { domain: '', nameservers: 'ns1.example.com,ns2.example.com' }
  loadZones()
}

async function deleteZone(domain) {
  if (!confirm(`Are you sure you want to delete the zone for ${domain}?`)) return
  await fetch(`/api/v1/dns/zones/${domain}`, { method: 'DELETE' })
  loadZones()
}

onMounted(loadZones)
</script>

<template>
  <Layout>
    <div class="page">
      <h1>DNS Zones <span style="font-size: 14px; color: var(--muted); font-weight: normal; margin-left: 8px;">PowerDNS Integration</span></h1>
      <div v-if="error" class="alert error">{{ error }}</div>
      <div v-if="notice" class="alert ok">{{ notice }}</div>

      <div class="card">
        <div class="row">
          <div style="flex: 1">
            <label>Domain</label>
            <input v-model="form.domain" placeholder="example.com" required />
          </div>
          <div style="flex: 1">
            <label>Nameservers (comma separated)</label>
            <input v-model="form.nameservers" required />
          </div>
          <button class="btn primary" style="margin-top: 18px" @click="createZone">➕ Create Zone</button>
        </div>
      </div>

      <div class="card">
        <h2>DNS Zones</h2>
        <div v-if="loading" class="muted">Loading zones...</div>
        <table v-else>
          <thead>
            <tr>
              <th>Domain</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="z in zones" :key="z.id">
              <td class="mono"><strong>{{ z.name }}</strong></td>
              <td>
                <button class="btn danger" @click="deleteZone(z.name)">Delete</button>
              </td>
            </tr>
            <tr v-if="zones.length === 0">
              <td colspan="2" class="muted">No zones found or PowerDNS is not configured.</td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
  </Layout>
</template>
