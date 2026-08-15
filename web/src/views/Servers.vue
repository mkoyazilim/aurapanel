<script setup>
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import Layout from '../components/Layout.vue'
import { api } from '../api.js'

const { t } = useI18n()

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
    servers.value = await api.get('/cluster') || []
  } catch (e) {
    error.value = e.message || t('servers.load_error')
  } finally {
    loading.value = false
  }
}

async function addServer() {
  error.value  = ''
  notice.value = ''
  if (!form.value.name || !form.value.ip_address) return
  try {
    await api.post('/cluster', form.value)
    notice.value = t('servers.node_added')
    form.value = { name: '', ip_address: '' }
    loadServers()
  } catch (e) {
    error.value = e.message || t('servers.add_error')
  }
}

async function deleteServer(id) {
  if (!confirm(t('servers.confirm_remove'))) return
  try {
    await api.delete(`/cluster/${id}`)
    loadServers()
  } catch (e) { error.value = e.message }
}

async function checkHealth(id) {
  const srv = servers.value.find(s => s.id === id)
  if (srv) srv.status = t('servers.status_checking')
  try {
    const d = await api.get(`/cluster/${id}/health`)
    if (srv) srv.status = d.status
  } catch {}
}

async function rotateKey(id) {
  if (!confirm(t('servers.confirm_rotate'))) return
  rotatingSrv.value = id
  rotatedKey.value  = ''
  try {
    const d = await api.post(`/servers/${id}/rotate-key`)
    rotatedKey.value = d.api_key
    notice.value = t('servers.key_rotated')
  } catch (e) {
    error.value = e.message || t('servers.rotate_failed')
  }
  rotatingSrv.value = null
}

function openSiteModal(id) {
  siteSrvID.value = id
  siteForm.value  = { domain: '', php_version: 'lsphp83' }
  siteModal.value = true
}

async function createSite() {
  error.value = ''
  try {
    await api.post(`/servers/${siteSrvID.value}/sites`, siteForm.value)
    notice.value = t('servers.site_queued')
    siteModal.value = false
  } catch (e) {
    error.value = e.message || t('servers.site_create_failed')
  }
}

async function showMetrics(srv) {
  metricsData.value = null
  metricsSrv.value  = srv
  metricsModal.value = true
  try {
    const all = await api.get('/cluster/metrics') || []
    metricsData.value = all.find(m => m.server_id === srv.id) || null
  } catch (e) { error.value = e.message }
}

onMounted(loadServers)
</script>

<template>
  <Layout>
    <div class="page">
      <h1>{{ $t('servers.title') }} <span style="font-size:14px;color:var(--muted);font-weight:normal;margin-left:8px">{{ $t('servers.subtitle') }}</span></h1>
      <div class="tab-nav">
        <router-link to="/cluster" class="tab-item" active-class="active">{{ $t('servers.title') || 'Küme Yönetimi' }}</router-link>
        <router-link to="/cluster/dashboard" class="tab-item" active-class="active">Dashboard</router-link>
      </div>
      <div v-if="error"  class="alert error">{{ error }}</div>
      <div v-if="notice" class="alert ok">{{ notice }}</div>

      <!-- Rotated key banner -->
      <div v-if="rotatedKey" class="alert ok" style="font-family:monospace;word-break:break-all">
        {{ $t('servers.new_api_key') }} <strong>{{ rotatedKey }}</strong>
        <button class="btn" style="margin-left:12px" @click="rotatedKey=''">{{ $t('servers.dismiss') }}</button>
      </div>

      <!-- Add node form -->
      <div class="card">
        <div class="row">
          <div style="flex:1">
            <label>{{ $t('servers.server_name') }}</label>
            <input v-model="form.name" :placeholder="$t('servers.placeholder_node')" />
          </div>
          <div style="flex:1">
            <label>{{ $t('servers.ip_address') }}</label>
            <input v-model="form.ip_address" :placeholder="$t('servers.placeholder_ip')" />
          </div>
          <button class="btn primary" style="margin-top:18px" @click="addServer">➕ {{ $t('servers.add_node') }}</button>
        </div>
      </div>

      <!-- Server list -->
      <div class="card">
        <h2>{{ $t('servers.cluster_nodes') }}</h2>
        <div v-if="loading" class="muted">{{ $t('servers.loading_servers') }}</div>
        <table v-else>
          <thead>
            <tr>
              <th>{{ $t('servers.th_name') }}</th>
              <th>{{ $t('servers.th_ip') }}</th>
              <th>{{ $t('servers.th_status') }}</th>
              <th>{{ $t('servers.th_actions') }}</th>
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
                  'warning': s.status === $t('servers.status_checking')
                }">{{ s.status }}</span>
              </td>
              <td style="display:flex;gap:6px;flex-wrap:wrap">
                <button class="btn" @click="checkHealth(s.id)">{{ $t('servers.action_health') }}</button>
                <button class="btn" @click="showMetrics(s)">{{ $t('servers.action_metrics') }}</button>
                <button class="btn" @click="openSiteModal(s.id)">{{ $t('servers.action_add_site') }}</button>
                <button class="btn warning" :disabled="rotatingSrv===s.id" @click="rotateKey(s.id)">
                  {{ rotatingSrv===s.id ? $t('servers.action_rotating') : $t('servers.action_rotate_key') }}
                </button>
                <button class="btn danger" @click="deleteServer(s.id)">{{ $t('servers.action_remove') }}</button>
              </td>
            </tr>
            <tr v-if="!servers.length">
              <td colspan="4" class="muted">{{ $t('servers.no_nodes') }}</td>
            </tr>
          </tbody>
        </table>
      </div>

      <!-- Site create modal -->
      <Teleport to="body">
        <div v-if="siteModal" class="modal-overlay" @click.self="siteModal=false">
          <div class="modal">
            <h3>{{ $t('servers.create_site_title') }}</h3>
            <label>{{ $t('servers.domain') }}</label>
            <input v-model="siteForm.domain" :placeholder="$t('servers.placeholder_domain')" />
            <label style="margin-top:10px">{{ $t('servers.php_version') }}</label>
            <select v-model="siteForm.php_version">
              <option value="lsphp83">{{ $t('servers.lsphp83') }}</option>
              <option value="lsphp82">{{ $t('servers.lsphp82') }}</option>
              <option value="lsphp81">{{ $t('servers.lsphp81') }}</option>
            </select>
            <div style="display:flex;gap:8px;margin-top:16px">
              <button class="btn primary" @click="createSite">{{ $t('servers.create') }}</button>
              <button class="btn" @click="siteModal=false">{{ $t('servers.cancel') }}</button>
            </div>
          </div>
        </div>
      </Teleport>

      <!-- Metrics modal -->
      <Teleport to="body">
        <div v-if="metricsModal" class="modal-overlay" @click.self="metricsModal=false">
          <div class="modal">
            <h3>{{ metricsSrv?.name }} — {{ $t('servers.live_metrics') }}</h3>
            <div v-if="!metricsData" class="muted">{{ $t('servers.loading') }}</div>
            <template v-else>
              <div v-if="metricsData.error" class="alert error">{{ metricsData.error }}</div>
              <template v-else>
                <div class="row" style="gap:16px;margin-top:12px">
                  <div class="card" style="flex:1;text-align:center">
                    <div style="font-size:28px;font-weight:700">{{ metricsData.cpu_percent?.toFixed(1) }}%</div>
                    <div class="muted">{{ $t('servers.cpu') }}</div>
                  </div>
                  <div class="card" style="flex:1;text-align:center">
                    <div style="font-size:28px;font-weight:700">{{ metricsData.ram_percent?.toFixed(1) }}%</div>
                    <div class="muted">{{ $t('servers.ram') }}</div>
                  </div>
                  <div class="card" style="flex:1;text-align:center">
                    <div style="font-size:28px;font-weight:700">{{ Math.floor((metricsData.uptime_sec||0)/3600) }}h</div>
                    <div class="muted">{{ $t('servers.uptime') }}</div>
                  </div>
                </div>
              </template>
            </template>
            <button class="btn" style="margin-top:16px" @click="metricsModal=false">{{ $t('servers.close') }}</button>
          </div>
        </div>
      </Teleport>
    </div>
  </Layout>
</template>

<style scoped>
.tab-nav {
  display: flex; gap: 16px; border-bottom: 1px solid var(--border-color, #e2e8f0); margin-bottom: 24px;
}
.tab-item {
  padding: 8px 16px; color: var(--muted, #64748b); text-decoration: none; border-bottom: 2px solid transparent; margin-bottom: -1px; font-weight: 500; transition: all 0.2s;
}
.tab-item:hover { color: var(--text, #1e293b); }
.tab-item.active { color: var(--primary, #3b82f6); border-bottom-color: var(--primary, #3b82f6); }
</style>
