<script setup>
import { ref, onMounted } from 'vue'
import Layout from '../components/Layout.vue'

const providers = ref([])
const syncLog   = ref([])
const loading   = ref(true)
const error     = ref('')
const notice    = ref('')

// Add provider modal
const addModal = ref(false)
const addForm  = ref({ name: '', provider: 'cloudflare', credentials: {} })
const cfCreds  = ref({ api_token: '', zone_id: '' })
const r53Creds = ref({ access_key: '', secret_key: '', region: 'us-east-1' })

// CF records modal
const cfModal      = ref(false)
const cfProviderID = ref(null)
const cfRecords    = ref([])
const cfLoading    = ref(false)
const syncLoading  = ref(false)

// Sync push modal (local records to push)
const syncModal  = ref(false)
const syncRecs   = ref([{ name: '', type: 'A', content: '', ttl: 3600, proxied: false }])

async function load() {
  loading.value = true
  try {
    const [p, l] = await Promise.all([
      fetch('/api/v1/extdns/providers').then(r => r.ok ? r.json() : []),
      fetch('/api/v1/extdns/sync-log?limit=20').then(r => r.ok ? r.json() : [])
    ])
    providers.value = p ?? []
    syncLog.value   = l ?? []
  } finally {
    loading.value = false
  }
}

async function createProvider() {
  error.value = notice.value = ''
  const creds = addForm.value.provider === 'cloudflare'
    ? { api_token: cfCreds.value.api_token, zone_id: cfCreds.value.zone_id }
    : { access_key: r53Creds.value.access_key, secret_key: r53Creds.value.secret_key, region: r53Creds.value.region }

  const res = await fetch('/api/v1/extdns/providers', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ name: addForm.value.name, provider: addForm.value.provider, credentials: creds })
  })
  addModal.value = false
  if (!res.ok) { const d = await res.json(); error.value = d.error || 'Failed'; return }
  notice.value = 'Provider added. Credentials are encrypted.'
  addForm.value = { name: '', provider: 'cloudflare' }
  cfCreds.value  = { api_token: '', zone_id: '' }
  r53Creds.value = { access_key: '', secret_key: '', region: 'us-east-1' }
  load()
}

async function deleteProvider(id, name) {
  if (!confirm(`Delete provider "${name}"?`)) return
  await fetch(`/api/v1/extdns/providers/${id}`, { method: 'DELETE' })
  load()
}

// CF Records
async function openCFRecords(id) {
  cfProviderID.value = id
  cfModal.value      = true
  cfLoading.value    = true
  cfRecords.value    = []
  const res = await fetch(`/api/v1/extdns/providers/${id}/cf/records`)
  if (res.ok) cfRecords.value = await res.json() ?? []
  cfLoading.value = false
}

// CF Sync push
function openSyncModal(id) {
  cfProviderID.value = id
  syncRecs.value     = [{ name: '', type: 'A', content: '', ttl: 3600, proxied: false }]
  syncModal.value    = true
}

function addSyncRow() {
  syncRecs.value.push({ name: '', type: 'A', content: '', ttl: 3600, proxied: false })
}
function removeSyncRow(i) {
  syncRecs.value.splice(i, 1)
}

async function doSync() {
  syncLoading.value = true
  error.value = notice.value = ''
  const res = await fetch(`/api/v1/extdns/providers/${cfProviderID.value}/cf/sync`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ records: syncRecs.value })
  })
  syncModal.value  = false
  syncLoading.value = false
  if (!res.ok) { const d = await res.json(); error.value = d.error || 'Sync failed'; return }
  const result = await res.json()
  notice.value = `Sync done — added: ${(result.added||[]).length}, conflicts: ${(result.conflicts||[]).length}`
  load()
}

function dirClass(d) { return d === 'push' ? 'ok' : 'warning' }
function actClass(a) { return a === 'conflict' ? 'error' : 'ok' }

onMounted(load)
</script>

<template>
  <Layout>
    <div class="page">
      <h1>External DNS <span style="font-size:14px;color:var(--muted);font-weight:normal;margin-left:8px">Cloudflare · Route53 · F5</span></h1>
      <div v-if="error"  class="alert error">{{ error }}</div>
      <div v-if="notice" class="alert ok">{{ notice }}</div>

      <div style="display:flex;justify-content:flex-end;margin-bottom:12px">
        <button class="btn primary" @click="addModal=true">➕ Add Provider</button>
      </div>

      <!-- Providers list -->
      <div class="card">
        <h2>DNS Providers</h2>
        <div v-if="loading" class="muted">Loading…</div>
        <table v-else>
          <thead>
            <tr><th>Name</th><th>Provider</th><th>Added</th><th>Actions</th></tr>
          </thead>
          <tbody>
            <tr v-for="p in providers" :key="p.id">
              <td><strong>{{ p.name }}</strong></td>
              <td><span class="badge" :class="p.provider==='cloudflare'?'ok':'warning'">{{ p.provider }}</span></td>
              <td class="text-sm muted">{{ p.created_at?.slice(0,10) }}</td>
              <td style="display:flex;gap:6px;flex-wrap:wrap">
                <template v-if="p.provider==='cloudflare'">
                  <button class="btn" @click="openCFRecords(p.id)">Records</button>
                  <button class="btn primary" @click="openSyncModal(p.id)">Push Sync</button>
                </template>
                <button class="btn danger" @click="deleteProvider(p.id, p.name)">Delete</button>
              </td>
            </tr>
            <tr v-if="!providers.length">
              <td colspan="4" class="muted">No providers configured.</td>
            </tr>
          </tbody>
        </table>
      </div>

      <!-- Sync log -->
      <div class="card" style="margin-top:16px">
        <h2>Sync Log</h2>
        <table>
          <thead>
            <tr><th>Time</th><th>Provider</th><th>Direction</th><th>Action</th><th>Detail</th></tr>
          </thead>
          <tbody>
            <tr v-for="l in syncLog" :key="l.id">
              <td class="text-sm muted">{{ l.created_at?.replace('T',' ').slice(0,19) }}</td>
              <td class="text-sm">{{ l.provider_id }}</td>
              <td><span class="badge" :class="dirClass(l.direction)">{{ l.direction }}</span></td>
              <td><span class="badge" :class="actClass(l.action)">{{ l.action }}</span></td>
              <td class="text-sm muted">{{ l.detail || '—' }}</td>
            </tr>
            <tr v-if="!syncLog.length"><td colspan="5" class="muted">No sync events yet.</td></tr>
          </tbody>
        </table>
      </div>

      <!-- Add provider modal -->
      <Teleport to="body">
        <div v-if="addModal" class="modal-overlay" @click.self="addModal=false">
          <div class="modal" style="max-width:480px">
            <h3>Add DNS Provider</h3>
            <label>Name</label>
            <input v-model="addForm.name" placeholder="cloudflare-main" />
            <label style="margin-top:10px">Provider</label>
            <select v-model="addForm.provider">
              <option value="cloudflare">Cloudflare</option>
              <option value="route53">AWS Route53</option>
            </select>

            <template v-if="addForm.provider==='cloudflare'">
              <label style="margin-top:10px">API Token</label>
              <input v-model="cfCreds.api_token" placeholder="CF API Token" type="password" />
              <label style="margin-top:10px">Zone ID</label>
              <input v-model="cfCreds.zone_id" placeholder="Zone ID" />
            </template>
            <template v-else>
              <label style="margin-top:10px">Access Key</label>
              <input v-model="r53Creds.access_key" placeholder="AKIA..." />
              <label style="margin-top:10px">Secret Key</label>
              <input v-model="r53Creds.secret_key" type="password" placeholder="Secret..." />
              <label style="margin-top:10px">Region</label>
              <input v-model="r53Creds.region" placeholder="us-east-1" />
            </template>

            <p class="muted" style="font-size:12px;margin-top:8px">
              🔐 Credentials are encrypted with AES-256 before storage.
            </p>
            <div style="display:flex;gap:8px;margin-top:14px">
              <button class="btn primary" @click="createProvider">Save</button>
              <button class="btn" @click="addModal=false">Cancel</button>
            </div>
          </div>
        </div>
      </Teleport>

      <!-- CF records modal -->
      <Teleport to="body">
        <div v-if="cfModal" class="modal-overlay" @click.self="cfModal=false">
          <div class="modal" style="max-width:680px;width:92vw">
            <h3>Cloudflare DNS Records</h3>
            <div v-if="cfLoading" class="muted">Loading…</div>
            <table v-else>
              <thead><tr><th>Name</th><th>Type</th><th>Content</th><th>TTL</th><th>Proxied</th></tr></thead>
              <tbody>
                <tr v-for="r in cfRecords" :key="r.id">
                  <td class="mono text-sm">{{ r.name }}</td>
                  <td><span class="badge">{{ r.type }}</span></td>
                  <td class="text-sm" style="max-width:200px;word-break:break-all">{{ r.content }}</td>
                  <td class="text-sm muted">{{ r.ttl }}</td>
                  <td><span class="badge" :class="r.proxied?'ok':''">{{ r.proxied?'✓':'—' }}</span></td>
                </tr>
                <tr v-if="!cfRecords.length"><td colspan="5" class="muted">No records.</td></tr>
              </tbody>
            </table>
            <button class="btn" style="margin-top:14px" @click="cfModal=false">Close</button>
          </div>
        </div>
      </Teleport>

      <!-- Push sync modal -->
      <Teleport to="body">
        <div v-if="syncModal" class="modal-overlay" @click.self="syncModal=false">
          <div class="modal" style="max-width:700px;width:94vw">
            <h3>Push Sync to Cloudflare</h3>
            <p class="muted" style="font-size:13px;margin-bottom:10px">Records below will be pushed to Cloudflare. Existing conflicting records will NOT be overwritten.</p>
            <div v-for="(rec, i) in syncRecs" :key="i" class="row" style="gap:6px;margin-bottom:6px;flex-wrap:wrap">
              <input v-model="rec.name"    placeholder="Name" style="flex:2;min-width:80px" />
              <select v-model="rec.type" style="flex:1;min-width:60px">
                <option v-for="t in ['A','AAAA','CNAME','MX','TXT']" :key="t">{{ t }}</option>
              </select>
              <input v-model="rec.content"  placeholder="Content" style="flex:3;min-width:100px" />
              <input v-model.number="rec.ttl" type="number" placeholder="TTL" style="width:65px" />
              <label style="display:flex;align-items:center;gap:4px;font-size:13px">
                <input type="checkbox" v-model="rec.proxied" /> Proxy
              </label>
              <button class="btn danger" style="padding:2px 8px" @click="removeSyncRow(i)" :disabled="syncRecs.length===1">✕</button>
            </div>
            <button class="btn" style="margin-bottom:12px" @click="addSyncRow">+ Row</button>
            <div style="display:flex;gap:8px">
              <button class="btn primary" :disabled="syncLoading" @click="doSync">
                {{ syncLoading ? 'Syncing…' : 'Push to Cloudflare' }}
              </button>
              <button class="btn" @click="syncModal=false">Cancel</button>
            </div>
          </div>
        </div>
      </Teleport>
    </div>
  </Layout>
</template>
