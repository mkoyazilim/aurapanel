<script setup>
import { ref, onMounted } from 'vue'
import Layout from '../components/Layout.vue'

const servers   = ref([])
const loading   = ref(true)
const form      = ref({ name: '', ip_address: '' })
const error     = ref('')
const notice    = ref('')

// Key rotate
const rotatedKey  = ref('')
const rotatingSrv = ref(null)

// Site create modal
const siteModal = ref(false)
const siteSrvID = ref('')
const siteForm  = ref({ domain: '', php_version: 'lsphp83' })

// Metrics modal
const metricsModal = ref(false)
const metricsData  = ref(null)
const metricsSrv   = ref(null)

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
  error.value  = ''
  notice.value = ''
  if (!form.value.name || !form.value.ip_address) return
  const res = await fetch('/api/v1/cluster', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(form.value)
  })
  if (!res.ok) {
    const d = await res.json()
    error.value = d.error || 'Failed to add server'
    return
  }
  notice.value = 'Cluster node added.'
  form.value = { name: '', ip_address: '' }
  loadServers()
}

async function deleteServer(id) {
  if (!confirm('Remove this server from cluster?')) return
  await fetch(`/api/v1/cluster/${id}`, { method: 'DELETE' })
  loadServers()
}

async function checkHealth(id) {
  const srv = servers.value.find(s => s.id === id)
  if (srv) srv.status = 'checking...'
  const res = await fetch(`/api/v1/cluster/${id}/health`)
  if (res.ok) {
    const d = await res.json()
    if (srv) srv.status = d.status
  }
}

async function rotateKey(id) {
  if (!confirm('Rotate API key? The old key will be invalidated immediately.')) return
  rotatingSrv.value = id
  rotatedKey.value  = ''
  const res = await fetch(`/api/v1/servers/${id}/rotate-key`, { method: 'POST' })
  rotatingSrv.value = null
  if (!res.ok) {
    const d = await res.json()
    error.value = d.error || 'Key rotation failed'
    return
  }
  const d = await res.json()
  rotatedKey.value = d.api_key
  notice.value = 'Key rotated — copy it now, it will not be shown again.'
}

function openSiteModal(id) {
  siteSrvID.value = id
  siteForm.value  = { domain: '', php_version: 'lsphp83' }
  siteModal.value = true
}

async function createSite() {
  error.value = ''
  const res = await fetch(`/api/v1/servers/${siteSrvID.value}/sites`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(siteForm.value)
  })
  siteModal.value = false
  if (!res.ok) {
    const d = await res.json()
    error.value = d.error || 'Site create failed'
    return
  }
  notice.value = 'Site creation queued on remote agent.'
}

async function showMetrics(srv) {
  metricsData.value = null
  metricsSrv.value  = srv
  metricsModal.value = true
  const res = await fetch('/api/v1/cluster/metrics')
  if (res.ok) {
    const all = await res.json()
    metricsData.value = all.find(m => m.server_id === srv.id) || null
  }
}

onMounted(loadServers)
</script>

<template>
  <Layout>
    <div class="page">
      <h1>Cluster Servers <span style="font-size:14px;color:var(--muted);font-weight:normal;margin-left:8px">Multi-server node management</span></h1>
      <div v-if="error"  class="alert error">{{ error }}</div>
      <div v-if="notice" class="alert ok">{{ notice }}</div>

      <!-- Rotated key banner -->
      <div v-if="rotatedKey" class="alert ok" style="font-family:monospace;word-break:break-all">
        New API Key: <strong>{{ rotatedKey }}</strong>
        <button class="btn" style="margin-left:12px" @click="rotatedKey=''">Dismiss</button>
      </div>

      <!-- Add node form -->
      <div class="card">
        <div class="row">
          <div style="flex:1">
            <label>Server Name</label>
            <input v-model="form.name" placeholder="node-2" />
          </div>
          <div style="flex:1">
            <label>IP Address</label>
            <input v-model="form.ip_address" placeholder="192.168.1.100" />
          </div>
          <button class="btn primary" style="margin-top:18px" @click="addServer">➕ Add Node</button>
        </div>
      </div>

      <!-- Server list -->
      <div class="card">
        <h2>Cluster Nodes</h2>
        <div v-if="loading" class="muted">Loading servers…</div>
        <table v-else>
          <thead>
            <tr>
              <th>Name</th>
              <th>IP Address</th>
              <th>Status</th>
              <th>Actions</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="s in servers" :key="s.id">
              <td><strong>{{ s.name }}</strong></td>
              <td>{{ s.ip_address }}</td>
              <td>
                <span class="badge" :class="{
                  'ok':      s.status === 'online' || s.status === 'active',
                  'error':   s.status === 'offline',
                  'warning': s.status === 'checking...'
                }">{{ s.status }}</span>
              </td>
              <td style="display:flex;gap:6px;flex-wrap:wrap">
                <button class="btn" @click="checkHealth(s.id)">Health</button>
                <button class="btn" @click="showMetrics(s)">Metrics</button>
                <button class="btn" @click="openSiteModal(s.id)">Add Site</button>
                <button class="btn warning" :disabled="rotatingSrv===s.id" @click="rotateKey(s.id)">
                  {{ rotatingSrv===s.id ? 'Rotating…' : 'Rotate Key' }}
                </button>
                <button class="btn danger" @click="deleteServer(s.id)">Remove</button>
              </td>
            </tr>
            <tr v-if="!servers.length">
              <td colspan="4" class="muted">No cluster nodes found.</td>
            </tr>
          </tbody>
        </table>
      </div>

      <!-- Site create modal -->
      <Teleport to="body">
        <div v-if="siteModal" class="modal-overlay" @click.self="siteModal=false">
          <div class="modal">
            <h3>Create Site on Remote Node</h3>
            <label>Domain</label>
            <input v-model="siteForm.domain" placeholder="example.com" />
            <label style="margin-top:10px">PHP Version</label>
            <select v-model="siteForm.php_version">
              <option value="lsphp83">LSPHP 8.3</option>
              <option value="lsphp82">LSPHP 8.2</option>
              <option value="lsphp81">LSPHP 8.1</option>
            </select>
            <div style="display:flex;gap:8px;margin-top:16px">
              <button class="btn primary" @click="createSite">Create</button>
              <button class="btn" @click="siteModal=false">Cancel</button>
            </div>
          </div>
        </div>
      </Teleport>

      <!-- Metrics modal -->
      <Teleport to="body">
        <div v-if="metricsModal" class="modal-overlay" @click.self="metricsModal=false">
          <div class="modal">
            <h3>{{ metricsSrv?.name }} — Live Metrics</h3>
            <div v-if="!metricsData" class="muted">Loading…</div>
            <template v-else>
              <div v-if="metricsData.error" class="alert error">{{ metricsData.error }}</div>
              <template v-else>
                <div class="row" style="gap:16px;margin-top:12px">
                  <div class="card" style="flex:1;text-align:center">
                    <div style="font-size:28px;font-weight:700">{{ metricsData.cpu_percent?.toFixed(1) }}%</div>
                    <div class="muted">CPU</div>
                  </div>
                  <div class="card" style="flex:1;text-align:center">
                    <div style="font-size:28px;font-weight:700">{{ metricsData.ram_percent?.toFixed(1) }}%</div>
                    <div class="muted">RAM</div>
                  </div>
                  <div class="card" style="flex:1;text-align:center">
                    <div style="font-size:28px;font-weight:700">{{ Math.floor((metricsData.uptime_sec||0)/3600) }}h</div>
                    <div class="muted">Uptime</div>
                  </div>
                </div>
              </template>
            </template>
            <button class="btn" style="margin-top:16px" @click="metricsModal=false">Close</button>
          </div>
        </div>
      </Teleport>
    </div>
  </Layout>
</template>
