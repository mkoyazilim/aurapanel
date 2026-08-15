<script setup>
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import Layout from '../components/Layout.vue'

const { t } = useI18n()

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
  if (!res.ok) { const d = await res.json(); error.value = d.error || t('externaldns.failed'); return }
  notice.value = t('externaldns.provider_added')
  addForm.value = { name: '', provider: 'cloudflare' }
  cfCreds.value  = { api_token: '', zone_id: '' }
  r53Creds.value = { access_key: '', secret_key: '', region: 'us-east-1' }
  load()
}

async function deleteProvider(id, name) {
  if (!confirm(t('externaldns.delete_provider_confirm', { name }))) return
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
  if (!res.ok) { const d = await res.json(); error.value = d.error || t('externaldns.sync_failed'); return }
  const result = await res.json()
  notice.value = t('externaldns.sync_done_stats', { added: (result.added||[]).length, conflicts: (result.conflicts||[]).length })
  load()
}

function dirClass(d) { return d === 'push' ? 'ok' : 'warning' }
function actClass(a) { return a === 'conflict' ? 'error' : 'ok' }

onMounted(load)
</script>

<template>
  <Layout>
    <div class="page">
      <h1>{{ $t('externaldns.title_external_dns') }} <span style="font-size:14px;color:var(--muted);font-weight:normal;margin-left:8px">{{ $t('externaldns.subtitle_providers') }}</span></h1>
      <div v-if="error"  class="alert error">{{ error }}</div>
      <div v-if="notice" class="alert ok">{{ notice }}</div>

      <div style="display:flex;justify-content:flex-end;margin-bottom:12px">
        <button class="btn primary" @click="addModal=true">{{ $t('externaldns.add_provider') }}</button>
      </div>

      <!-- Providers list -->
      <div class="card">
        <h2>{{ $t('externaldns.dns_providers') }}</h2>
        <div v-if="loading" class="muted">{{ $t('externaldns.loading') }}</div>
        <table v-else>
          <thead>
            <tr><th>{{ $t('externaldns.th_name') }}</th><th>{{ $t('externaldns.th_provider') }}</th><th>{{ $t('externaldns.th_added') }}</th><th>{{ $t('externaldns.th_actions') }}</th></tr>
          </thead>
          <tbody>
            <tr v-for="p in providers" :key="p.id">
              <td><strong>{{ p.name }}</strong></td>
              <td><span class="badge" :class="p.provider==='cloudflare'?'ok':'warning'">{{ p.provider }}</span></td>
              <td class="text-sm muted">{{ p.created_at?.slice(0,10) }}</td>
              <td style="display:flex;gap:6px;flex-wrap:wrap">
                <template v-if="p.provider==='cloudflare'">
                  <button class="btn" @click="openCFRecords(p.id)">{{ $t('externaldns.btn_records') }}</button>
                  <button class="btn primary" @click="openSyncModal(p.id)">{{ $t('externaldns.btn_push_sync') }}</button>
                </template>
                <button class="btn danger" @click="deleteProvider(p.id, p.name)">{{ $t('externaldns.btn_delete') }}</button>
              </td>
            </tr>
            <tr v-if="!providers.length">
              <td colspan="4" class="muted">{{ $t('externaldns.no_providers') }}</td>
            </tr>
          </tbody>
        </table>
      </div>

      <!-- Sync log -->
      <div class="card" style="margin-top:16px">
        <h2>{{ $t('externaldns.sync_log') }}</h2>
        <table>
          <thead>
            <tr><th>{{ $t('externaldns.th_time') }}</th><th>{{ $t('externaldns.th_provider') }}</th><th>{{ $t('externaldns.th_direction') }}</th><th>{{ $t('externaldns.th_action') }}</th><th>{{ $t('externaldns.th_detail') }}</th></tr>
          </thead>
          <tbody>
            <tr v-for="l in syncLog" :key="l.id">
              <td class="text-sm muted">{{ l.created_at?.replace('T',' ').slice(0,19) }}</td>
              <td class="text-sm">{{ l.provider_id }}</td>
              <td><span class="badge" :class="dirClass(l.direction)">{{ l.direction }}</span></td>
              <td><span class="badge" :class="actClass(l.action)">{{ l.action }}</span></td>
              <td class="text-sm muted">{{ l.detail || '—' }}</td>
            </tr>
            <tr v-if="!syncLog.length"><td colspan="5" class="muted">{{ $t('externaldns.no_sync_events') }}</td></tr>
          </tbody>
        </table>
      </div>

      <!-- Add provider modal -->
      <Teleport to="body">
        <div v-if="addModal" class="modal-overlay" @click.self="addModal=false">
          <div class="modal" style="max-width:480px">
            <h3>{{ $t('externaldns.add_dns_provider') }}</h3>
            <label>{{ $t('externaldns.label_name') }}</label>
            <input v-model="addForm.name" :placeholder="$t('externaldns.placeholder_name')" />
            <label style="margin-top:10px">{{ $t('externaldns.label_provider') }}</label>
            <select v-model="addForm.provider">
              <option value="cloudflare">{{ $t('externaldns.opt_cloudflare') }}</option>
              <option value="route53">{{ $t('externaldns.opt_route53') }}</option>
            </select>

            <template v-if="addForm.provider==='cloudflare'">
              <label style="margin-top:10px">{{ $t('externaldns.label_api_token') }}</label>
              <input v-model="cfCreds.api_token" :placeholder="$t('externaldns.placeholder_api_token')" type="password" />
              <label style="margin-top:10px">{{ $t('externaldns.label_zone_id') }}</label>
              <input v-model="cfCreds.zone_id" :placeholder="$t('externaldns.placeholder_zone_id')" />
            </template>
            <template v-else>
              <label style="margin-top:10px">{{ $t('externaldns.label_access_key') }}</label>
              <input v-model="r53Creds.access_key" :placeholder="$t('externaldns.placeholder_access_key')" />
              <label style="margin-top:10px">{{ $t('externaldns.label_secret_key') }}</label>
              <input v-model="r53Creds.secret_key" type="password" :placeholder="$t('externaldns.placeholder_secret_key')" />
              <label style="margin-top:10px">{{ $t('externaldns.label_region') }}</label>
              <input v-model="r53Creds.region" :placeholder="$t('externaldns.placeholder_region')" />
            </template>

            <p class="muted" style="font-size:12px;margin-top:8px">
              {{ $t('externaldns.msg_encrypted') }}
            </p>
            <div style="display:flex;gap:8px;margin-top:14px">
              <button class="btn primary" @click="createProvider">{{ $t('externaldns.btn_save') }}</button>
              <button class="btn" @click="addModal=false">{{ $t('externaldns.btn_cancel') }}</button>
            </div>
          </div>
        </div>
      </Teleport>

      <!-- CF records modal -->
      <Teleport to="body">
        <div v-if="cfModal" class="modal-overlay" @click.self="cfModal=false">
          <div class="modal" style="max-width:680px;width:92vw">
            <h3>{{ $t('externaldns.cf_dns_records') }}</h3>
            <div v-if="cfLoading" class="muted">{{ $t('externaldns.loading') }}</div>
            <table v-else>
              <thead><tr><th>{{ $t('externaldns.th_name') }}</th><th>{{ $t('externaldns.th_type') }}</th><th>{{ $t('externaldns.th_content') }}</th><th>{{ $t('externaldns.th_ttl') }}</th><th>{{ $t('externaldns.th_proxied') }}</th></tr></thead>
              <tbody>
                <tr v-for="r in cfRecords" :key="r.id">
                  <td class="mono text-sm">{{ r.name }}</td>
                  <td><span class="badge">{{ r.type }}</span></td>
                  <td class="text-sm" style="max-width:200px;word-break:break-all">{{ r.content }}</td>
                  <td class="text-sm muted">{{ r.ttl }}</td>
                  <td><span class="badge" :class="r.proxied?'ok':''">{{ r.proxied?'✓':'—' }}</span></td>
                </tr>
                <tr v-if="!cfRecords.length"><td colspan="5" class="muted">{{ $t('externaldns.no_records') }}</td></tr>
              </tbody>
            </table>
            <button class="btn" style="margin-top:14px" @click="cfModal=false">{{ $t('externaldns.btn_close') }}</button>
          </div>
        </div>
      </Teleport>

      <!-- Push sync modal -->
      <Teleport to="body">
        <div v-if="syncModal" class="modal-overlay" @click.self="syncModal=false">
          <div class="modal" style="max-width:700px;width:94vw">
            <h3>{{ $t('externaldns.push_sync_to_cf') }}</h3>
            <p class="muted" style="font-size:13px;margin-bottom:10px">{{ $t('externaldns.push_sync_info') }}</p>
            <div v-for="(rec, i) in syncRecs" :key="i" class="row" style="gap:6px;margin-bottom:6px;flex-wrap:wrap">
              <input v-model="rec.name"    :placeholder="$t('externaldns.placeholder_record_name')" style="flex:2;min-width:80px" />
              <select v-model="rec.type" style="flex:1;min-width:60px">
                <option v-for="t in ['A','AAAA','CNAME','MX','TXT']" :key="t">{{ t }}</option>
              </select>
              <input v-model="rec.content"  :placeholder="$t('externaldns.placeholder_content')" style="flex:3;min-width:100px" />
              <input v-model.number="rec.ttl" type="number" :placeholder="$t('externaldns.placeholder_ttl')" style="width:65px" />
              <label style="display:flex;align-items:center;gap:4px;font-size:13px">
                <input type="checkbox" v-model="rec.proxied" /> {{ $t('externaldns.lbl_proxy') }}
              </label>
              <button class="btn danger" style="padding:2px 8px" @click="removeSyncRow(i)" :disabled="syncRecs.length===1">✕</button>
            </div>
            <button class="btn" style="margin-bottom:12px" @click="addSyncRow">{{ $t('externaldns.btn_add_row') }}</button>
            <div style="display:flex;gap:8px">
              <button class="btn primary" :disabled="syncLoading" @click="doSync">
                {{ syncLoading ? $t('externaldns.btn_syncing') : $t('externaldns.btn_push_to_cf') }}
              </button>
              <button class="btn" @click="syncModal=false">{{ $t('externaldns.btn_cancel') }}</button>
            </div>
          </div>
        </div>
      </Teleport>
    </div>
  </Layout>
</template>
