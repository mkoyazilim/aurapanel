<script setup>
import { ref, onMounted } from 'vue'
import Layout from '../components/Layout.vue'

const servers = ref([])
const loading = ref(true)
const form = ref({ name: '', ip_address: '' })
const error = ref('')
const notice = ref('')

async function loadServers() {
  loading.value = true
  try {
    const res = await fetch('/api/v1/cluster')
    if (res.ok) servers.value = await res.json()
  } finally {
    loading.value = false
  }
}

async function addServer() {
  error.value = ''
  notice.value = ''
  if (!form.value.name || !form.value.ip_address) return
  const res = await fetch('/api/v1/cluster', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(form.value)
  })
  if (!res.ok) {
    const data = await res.json()
    error.value = data.error || 'Failed to add server'
    return
  }
  notice.value = 'Cluster node added successfully.'
  form.value = { name: '', ip_address: '' }
  loadServers()
}

async function deleteServer(id) {
  if (!confirm('Are you sure you want to remove this server from cluster?')) return
  await fetch(`/api/v1/cluster/${id}`, { method: 'DELETE' })
  loadServers()
}

async function checkHealth(id) {
  const server = servers.value.find(s => s.id === id)
  if (server) server.status = 'checking...'
  const res = await fetch(`/api/v1/cluster/${id}/health`)
  if (res.ok) {
    const data = await res.json()
    if (server) server.status = data.status
  }
}

onMounted(loadServers)
</script>

<template>
  <Layout>
    <div class="page">
      <h1>Cluster Servers <span style="font-size: 14px; color: var(--muted); font-weight: normal; margin-left: 8px;">Manage multi-server nodes</span></h1>
      <div v-if="error" class="alert error">{{ error }}</div>
      <div v-if="notice" class="alert ok">{{ notice }}</div>

      <div class="card">
        <div class="row">
          <div style="flex: 1">
            <label>Server Name</label>
            <input v-model="form.name" placeholder="node-2" required />
          </div>
          <div style="flex: 1">
            <label>IP Address</label>
            <input v-model="form.ip_address" placeholder="192.168.1.100" required />
          </div>
          <button class="btn primary" style="margin-top: 18px" @click="addServer">➕ Add Node</button>
        </div>
      </div>

      <div class="card">
        <h2>Cluster Servers</h2>
        <div v-if="loading" class="muted">Loading servers...</div>
        <table v-else>
          <thead>
            <tr>
              <th>ID</th>
              <th>Name</th>
              <th>IP Address</th>
              <th>API Key</th>
              <th>Status</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="s in servers" :key="s.id">
              <td class="mono text-sm">{{ s.id }}</td>
              <td><strong>{{ s.name }}</strong></td>
              <td>{{ s.ip_address }}</td>
              <td class="mono text-sm text-muted">********</td>
              <td>
                <span class="badge" :class="{
                  'ok': s.status === 'online' || s.status === 'active',
                  'error': s.status === 'offline',
                  'warning': s.status === 'checking...'
                }">{{ s.status }}</span>
              </td>
              <td>
                <button class="btn" @click="checkHealth(s.id)" style="margin-right: 8px;">Health Check</button>
                <button class="btn danger" @click="deleteServer(s.id)">Remove</button>
              </td>
            </tr>
            <tr v-if="!servers.length"><td colspan="6" class="muted">No cluster servers found.</td></tr>
          </tbody>
        </table>
      </div>
    </div>
  </Layout>
</template>
