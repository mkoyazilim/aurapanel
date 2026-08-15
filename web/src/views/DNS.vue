<script setup>
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import Layout from '../components/Layout.vue'

const { t } = useI18n()

const zones   = ref([])
const loading = ref(true)
const form    = ref({ domain: '', nameservers: 'ns1.example.com,ns2.example.com' })
const error   = ref('')
const notice  = ref('')

// Records modal
const recordsModal   = ref(false)
const recordsDomain  = ref('')
const recordsData    = ref(null)   // zone object with rrsets
const recordsLoading = ref(false)
const recForm = ref({ name: '', type: 'A', ttl: 3600, content: '' })
const delRec  = ref({ name: '', type: '' })

// DNSSEC modal
const dnssecModal  = ref(false)
const dnssecDomain = ref('')
const cryptoKeys   = ref([])
const addKeyForm   = ref({ key_type: 'zsk', algorithm: 'ecdsa256', bits: 256 })

const RECORD_TYPES = ['A','AAAA','CNAME','MX','TXT','SRV','CAA','NS','PTR','SOA']

async function loadZones() {
  loading.value = true
  try {
    const res = await fetch('/api/v1/dns/zones')
    if (res.ok) zones.value = await res.json() ?? []
  } finally {
    loading.value = false
  }
}

async function createZone() {
  error.value = notice.value = ''
  if (!form.value.domain) return
  const nsList = form.value.nameservers.split(',').map(s => s.trim()).filter(s => s)
  const res = await fetch('/api/v1/dns/zones', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ domain: form.value.domain, nameservers: nsList })
  })
  if (!res.ok) { const d = await res.json(); error.value = d.error || t('dns.msg_failed'); return }
  notice.value = t('dns.msg_zone_created')
  form.value = { domain: '', nameservers: 'ns1.example.com,ns2.example.com' }
  loadZones()
}

async function deleteZone(name) {
  const domain = name.replace(/\.$/, '')
  if (!confirm(t('dns.msg_delete_zone_confirm', { domain }))) return
  await fetch(`/api/v1/dns/zones/${domain}`, { method: 'DELETE' })
  loadZones()
}

async function exportZone(name) {
  const domain = name.replace(/\.$/, '')
  const res = await fetch(`/api/v1/dns/zones/${domain}/export`)
  if (!res.ok) { error.value = t('dns.msg_export_failed'); return }
  const text = await res.text()
  const blob = new Blob([text], { type: 'text/plain' })
  const a = document.createElement('a')
  a.href = URL.createObjectURL(blob)
  a.download = domain + '.zone'
  a.click()
}

// ─── Records modal ───────────────────────────────────────────────────────────
async function openRecords(name) {
  recordsDomain.value = name.replace(/\.$/, '')
  recordsModal.value  = true
  recordsLoading.value = true
  recForm.value = { name: '', type: 'A', ttl: 3600, content: '' }
  const res = await fetch(`/api/v1/dns/zones/${recordsDomain.value}/records`)
  if (res.ok) recordsData.value = await res.json()
  recordsLoading.value = false
}

async function addRecord() {
  const res = await fetch(`/api/v1/dns/zones/${recordsDomain.value}/records`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(recForm.value)
  })
  if (!res.ok) { const d = await res.json(); error.value = d.error || t('dns.msg_failed'); return }
  notice.value = t('dns.msg_record_added')
  openRecords(recordsDomain.value + '.')
}

async function deleteRecord(name, type) {
  if (!confirm(t('dns.msg_delete_record_confirm', { type, name }))) return
  const res = await fetch(`/api/v1/dns/zones/${recordsDomain.value}/records`, {
    method: 'DELETE',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ name, type })
  })
  if (!res.ok) { const d = await res.json(); error.value = d.error || t('dns.msg_failed'); return }
  openRecords(recordsDomain.value + '.')
}

// ─── DNSSEC modal ────────────────────────────────────────────────────────────
async function openDNSSEC(name) {
  dnssecDomain.value = name.replace(/\.$/, '')
  dnssecModal.value  = true
  cryptoKeys.value   = []
  const res = await fetch(`/api/v1/dns/zones/${dnssecDomain.value}/cryptokeys`)
  if (res.ok) cryptoKeys.value = await res.json() ?? []
}

async function enableDNSSEC() {
  const res = await fetch(`/api/v1/dns/zones/${dnssecDomain.value}/dnssec`, { method: 'POST' })
  if (!res.ok) { const d = await res.json(); error.value = d.error || t('dns.msg_failed'); return }
  notice.value = t('dns.msg_dnssec_enabled')
  openDNSSEC(dnssecDomain.value + '.')
}

async function disableDNSSEC() {
  if (!confirm(t('dns.msg_disable_dnssec_confirm'))) return
  const res = await fetch(`/api/v1/dns/zones/${dnssecDomain.value}/dnssec`, { method: 'DELETE' })
  if (!res.ok) { const d = await res.json(); error.value = d.error || t('dns.msg_failed'); return }
  notice.value = t('dns.msg_dnssec_disabled')
  cryptoKeys.value = []
}

async function addCryptoKey() {
  const res = await fetch(`/api/v1/dns/zones/${dnssecDomain.value}/cryptokeys`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(addKeyForm.value)
  })
  if (!res.ok) { const d = await res.json(); error.value = d.error || t('dns.msg_failed'); return }
  openDNSSEC(dnssecDomain.value + '.')
}

async function deleteCryptoKey(keyId) {
  if (!confirm(t('dns.msg_delete_key_confirm'))) return
  await fetch(`/api/v1/dns/zones/${dnssecDomain.value}/cryptokeys/${keyId}`, { method: 'DELETE' })
  openDNSSEC(dnssecDomain.value + '.')
}

async function rectify() {
  const res = await fetch(`/api/v1/dns/zones/${dnssecDomain.value}/rectify`, { method: 'POST' })
  if (!res.ok) { const d = await res.json(); error.value = d.error || t('dns.msg_failed'); return }
  notice.value = t('dns.msg_zone_rectified')
}

onMounted(loadZones)
</script>

<template>
  <Layout>
    <div class="page">
      <h1>{{ $t('dns.title') }} <span style="font-size:14px;color:var(--muted);font-weight:normal;margin-left:8px">{{ $t('dns.subtitle') }}</span></h1>
      <div class="tab-nav">
        <router-link to="/dns" class="tab-item" active-class="active">{{ $t('dns.title') || 'DNS Yönetimi' }}</router-link>
        <router-link to="/extdns" class="tab-item" active-class="active">Dış DNS (CF/AWS)</router-link>
      </div>
      <div v-if="error"  class="alert error">{{ error }}</div>
      <div v-if="notice" class="alert ok">{{ notice }}</div>

      <!-- Create zone -->
      <div class="card">
        <div class="row">
          <div style="flex:1">
            <label>{{ $t('dns.domain_label') }}</label>
            <input v-model="form.domain" :placeholder="$t('dns.domain_placeholder')" />
          </div>
          <div style="flex:1">
            <label>{{ $t('dns.nameservers_label') }}</label>
            <input v-model="form.nameservers" />
          </div>
          <button class="btn primary" style="margin-top:18px" @click="createZone">➕ {{ $t('dns.create_zone') }}</button>
        </div>
      </div>

      <!-- Zone list -->
      <div class="card">
        <h2>{{ $t('dns.title') }}</h2>
        <div v-if="loading" class="muted">{{ $t('dns.loading') }}</div>
        <table v-else>
          <thead>
            <tr>
              <th>{{ $t('dns.table_domain') }}</th>
              <th>{{ $t('dns.table_kind') }}</th>
              <th>{{ $t('dns.table_serial') }}</th>
              <th>{{ $t('dns.table_dnssec') }}</th>
              <th>{{ $t('dns.table_actions') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="z in zones" :key="z.id">
              <td class="mono"><strong>{{ z.name }}</strong></td>
              <td class="text-sm muted">{{ z.kind }}</td>
              <td class="text-sm muted">{{ z.serial || '—' }}</td>
              <td>
                <span class="badge" :class="z.dnssec ? 'ok' : ''">{{ z.dnssec ? $t('dns.badge_signed') : $t('dns.badge_off') }}</span>
              </td>
              <td style="display:flex;gap:6px;flex-wrap:wrap">
                <button class="btn" @click="openRecords(z.name)">{{ $t('dns.btn_records') }}</button>
                <button class="btn" @click="openDNSSEC(z.name)">{{ $t('dns.btn_dnssec') }}</button>
                <button class="btn" @click="exportZone(z.name)">{{ $t('dns.btn_export') }}</button>
                <button class="btn danger" @click="deleteZone(z.name)">{{ $t('dns.btn_delete') }}</button>
              </td>
            </tr>
            <tr v-if="!zones.length">
              <td colspan="5" class="muted">{{ $t('dns.empty_zones') }}</td>
            </tr>
          </tbody>
        </table>
      </div>

      <!-- Records modal -->
      <Teleport to="body">
        <div v-if="recordsModal" class="modal-overlay" @click.self="recordsModal=false">
          <div class="modal" style="max-width:720px;width:90vw">
            <h3>{{ $t('dns.modal_records_title', { domain: recordsDomain }) }}</h3>
            <div v-if="recordsLoading" class="muted">{{ $t('dns.loading_records') }}</div>
            <template v-else-if="recordsData">
              <!-- Add record form -->
              <div class="row" style="gap:8px;margin-bottom:12px;flex-wrap:wrap">
                <input v-model="recForm.name" :placeholder="$t('dns.record_name_placeholder')" style="flex:2;min-width:100px" />
                <select v-model="recForm.type" style="flex:1;min-width:80px">
                  <option v-for="t in RECORD_TYPES" :key="t">{{ t }}</option>
                </select>
                <input v-model.number="recForm.ttl" type="number" :placeholder="$t('dns.record_ttl_placeholder')" style="width:70px" />
                <input v-model="recForm.content" :placeholder="$t('dns.record_content_placeholder')" style="flex:3;min-width:120px" />
                <button class="btn primary" @click="addRecord">{{ $t('dns.btn_add') }}</button>
              </div>
              <!-- Existing rrsets -->
              <table>
                <thead><tr><th>{{ $t('dns.table_name') }}</th><th>{{ $t('dns.table_type') }}</th><th>{{ $t('dns.table_ttl') }}</th><th>{{ $t('dns.table_records') }}</th><th></th></tr></thead>
                <tbody>
                  <template v-for="rr in (recordsData.rrsets || [])" :key="rr.name+rr.type">
                    <tr>
                      <td class="mono text-sm">{{ rr.name }}</td>
                      <td class="text-sm"><span class="badge">{{ rr.type }}</span></td>
                      <td class="text-sm muted">{{ rr.ttl }}</td>
                      <td class="text-sm" style="max-width:240px;word-break:break-all">
                        {{ (rr.records||[]).map(rc=>rc.content).join(', ') }}
                      </td>
                      <td>
                        <button class="btn danger" style="padding:2px 8px" @click="deleteRecord(rr.name,rr.type)">✕</button>
                      </td>
                    </tr>
                  </template>
                  <tr v-if="!(recordsData.rrsets||[]).length">
                    <td colspan="5" class="muted">{{ $t('dns.empty_records') }}</td>
                  </tr>
                </tbody>
              </table>
            </template>
            <button class="btn" style="margin-top:16px" @click="recordsModal=false">{{ $t('dns.btn_close') }}</button>
          </div>
        </div>
      </Teleport>

      <!-- DNSSEC modal -->
      <Teleport to="body">
        <div v-if="dnssecModal" class="modal-overlay" @click.self="dnssecModal=false">
          <div class="modal" style="max-width:600px;width:90vw">
            <h3>{{ $t('dns.modal_dnssec_title', { domain: dnssecDomain }) }}</h3>
            <div style="display:flex;gap:8px;margin-bottom:16px">
              <button class="btn primary" @click="enableDNSSEC">{{ $t('dns.btn_enable_dnssec') }}</button>
              <button class="btn warning" @click="disableDNSSEC">{{ $t('dns.btn_disable_dnssec') }}</button>
              <button class="btn" @click="rectify">{{ $t('dns.btn_rectify') }}</button>
            </div>

            <h4>{{ $t('dns.subtitle_crypto_keys') }}</h4>
            <table>
              <thead><tr><th>{{ $t('dns.table_id') }}</th><th>{{ $t('dns.table_type') }}</th><th>{{ $t('dns.table_algorithm') }}</th><th>{{ $t('dns.table_active') }}</th><th>{{ $t('dns.table_ds') }}</th><th></th></tr></thead>
              <tbody>
                <tr v-for="k in cryptoKeys" :key="k.id">
                  <td class="text-sm">{{ k.id }}</td>
                  <td class="text-sm"><span class="badge">{{ k.keytype }}</span></td>
                  <td class="text-sm muted">{{ k.algorithm }}</td>
                  <td><span class="badge" :class="k.active?'ok':'error'">{{ k.active ? $t('dns.badge_active') : $t('dns.badge_inactive') }}</span></td>
                  <td class="text-sm muted" style="max-width:180px;word-break:break-all">
                    {{ (k.ds||[]).join(' | ').slice(0,60) || '—' }}
                  </td>
                  <td><button class="btn danger" style="padding:2px 8px" @click="deleteCryptoKey(k.id)">✕</button></td>
                </tr>
                <tr v-if="!cryptoKeys.length"><td colspan="6" class="muted">{{ $t('dns.empty_crypto_keys') }}</td></tr>
              </tbody>
            </table>

            <h4 style="margin-top:16px">{{ $t('dns.subtitle_add_key') }}</h4>
            <div class="row" style="gap:8px">
              <select v-model="addKeyForm.key_type">
                <option value="zsk">ZSK</option>
                <option value="ksk">KSK</option>
              </select>
              <select v-model="addKeyForm.algorithm">
                <option value="ecdsa256">ECDSA P-256</option>
                <option value="ecdsa384">ECDSA P-384</option>
                <option value="rsasha256">RSA-SHA256</option>
              </select>
              <button class="btn primary" @click="addCryptoKey">{{ $t('dns.btn_add_key') }}</button>
            </div>

            <button class="btn" style="margin-top:16px" @click="dnssecModal=false">{{ $t('dns.btn_close') }}</button>
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
