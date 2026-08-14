<script setup>
import { ref, onMounted } from 'vue'
import Layout from '../components/Layout.vue'
import { useRoute } from 'vue-router'

const route  = useRoute()
const siteID = route.params.id

const rules  = ref([])
const wafLog = ref([])
const crs    = ref(null)
const error  = ref('')
const notice = ref('')

// Rule form modal
const ruleModal = ref(false)
const editID    = ref(null)
const ruleForm  = ref({ rule_id: '', phase: 2, action: 'deny', pattern: '', description: '', enabled: true })

// Dry-run test modal
const testModal  = ref(false)
const testForm   = ref({ uri: '/', method: 'GET', body_sample: '', headers: {} })
const testResult = ref(null)
const testLoading = ref(false)

const ACTIONS = ['deny', 'allow', 'log']
const PHASES  = [1, 2, 3, 4, 5]

async function load() {
  const [r, l, c] = await Promise.all([
    fetch(`/api/v1/sites/${siteID}/waf/rules`).then(r => r.ok ? r.json() : []),
    fetch(`/api/v1/sites/${siteID}/waf/log?limit=30`).then(r => r.ok ? r.json() : []),
    fetch('/api/v1/waf/crs').then(r => r.ok ? r.json() : null)
  ])
  rules.value  = r ?? []
  wafLog.value = l ?? []
  crs.value    = c
}

async function saveCRS() {
  const res = await fetch('/api/v1/waf/crs', {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(crs.value)
  })
  if (!res.ok) { const d = await res.json(); error.value = d.error || 'Failed'; return }
  notice.value = 'CRS config saved.'
}

function openAdd() {
  editID.value  = null
  ruleForm.value = { rule_id: '', phase: 2, action: 'deny', pattern: '', description: '', enabled: true }
  ruleModal.value = true
}

function openEdit(rule) {
  editID.value   = rule.id
  ruleForm.value = { ...rule }
  ruleModal.value = true
}

async function saveRule() {
  error.value = notice.value = ''
  const isEdit = editID.value !== null
  const url    = isEdit ? `/api/v1/sites/${siteID}/waf/rules/${editID.value}` : `/api/v1/sites/${siteID}/waf/rules`
  const method = isEdit ? 'PUT' : 'POST'
  const res = await fetch(url, {
    method,
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(ruleForm.value)
  })
  ruleModal.value = false
  if (!res.ok) { const d = await res.json(); error.value = d.error || 'Save failed'; return }
  notice.value = isEdit ? 'Rule updated.' : 'Rule created.'
  load()
}

async function deleteRule(id) {
  if (!confirm('Delete this WAF rule?')) return
  await fetch(`/api/v1/sites/${siteID}/waf/rules/${id}`, { method: 'DELETE' })
  load()
}

async function runTest() {
  testLoading.value = true
  testResult.value  = null
  const res = await fetch(`/api/v1/sites/${siteID}/waf/test`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(testForm.value)
  })
  testLoading.value = false
  if (res.ok) testResult.value = await res.json()
  else { const d = await res.json(); error.value = d.error || 'Test failed' }
}

function actionClass(a) {
  if (a === 'deny') return 'error'
  if (a === 'allow') return 'ok'
  return 'warning'
}

onMounted(load)
</script>

<template>
  <Layout>
    <div class="page">
      <h1>Advanced WAF <span style="font-size:14px;color:var(--muted);font-weight:normal;margin-left:8px">{{ siteID }}</span></h1>
      <div v-if="error"  class="alert error">{{ error }}</div>
      <div v-if="notice" class="alert ok">{{ notice }}</div>

      <!-- OWASP CRS config -->
      <div v-if="crs" class="card">
        <h2>OWASP CRS Configuration</h2>
        <div class="row" style="gap:12px;flex-wrap:wrap;align-items:flex-end">
          <div>
            <label>CRS Version</label>
            <input v-model="crs.crs_version" style="width:100px" />
          </div>
          <div>
            <label>Paranoia Level (1–4)</label>
            <select v-model.number="crs.paranoia">
              <option v-for="n in [1,2,3,4]" :key="n" :value="n">{{ n }}</option>
            </select>
          </div>
          <div style="display:flex;align-items:center;gap:6px;padding-bottom:2px">
            <input type="checkbox" id="dryrun" v-model="crs.dry_run" />
            <label for="dryrun" style="margin:0">Global Dry-run Mode</label>
          </div>
          <button class="btn primary" @click="saveCRS">Save CRS Config</button>
        </div>
        <p class="muted" style="font-size:12px;margin-top:8px">
          Dry-run: kurallar eşleşmeyi log'lar ama engelleme yapmaz. Paranoia arttıkça kural sayısı artar.
        </p>
      </div>

      <!-- Custom rules -->
      <div class="card" style="margin-top:16px">
        <div style="display:flex;justify-content:space-between;align-items:center">
          <h2 style="margin:0">Custom WAF Rules</h2>
          <div style="display:flex;gap:8px">
            <button class="btn" @click="testModal=true">🔍 Test Rules</button>
            <button class="btn primary" @click="openAdd">➕ Add Rule</button>
          </div>
        </div>
        <table style="margin-top:12px">
          <thead>
            <tr><th>Rule ID</th><th>Phase</th><th>Action</th><th>Pattern</th><th>Description</th><th>Enabled</th><th></th></tr>
          </thead>
          <tbody>
            <tr v-for="r in rules" :key="r.id">
              <td class="mono text-sm">{{ r.rule_id }}</td>
              <td class="text-sm muted">{{ r.phase }}</td>
              <td><span class="badge" :class="actionClass(r.action)">{{ r.action }}</span></td>
              <td class="mono text-sm" style="max-width:160px;word-break:break-all">{{ r.pattern }}</td>
              <td class="text-sm muted">{{ r.description || '—' }}</td>
              <td><span class="badge" :class="r.enabled?'ok':'error'">{{ r.enabled?'on':'off' }}</span></td>
              <td style="display:flex;gap:4px">
                <button class="btn" style="padding:2px 8px" @click="openEdit(r)">Edit</button>
                <button class="btn danger" style="padding:2px 8px" @click="deleteRule(r.id)">✕</button>
              </td>
            </tr>
            <tr v-if="!rules.length"><td colspan="7" class="muted">No custom rules.</td></tr>
          </tbody>
        </table>
      </div>

      <!-- Request log -->
      <div class="card" style="margin-top:16px">
        <h2>WAF Request Log <span class="muted" style="font-size:13px;font-weight:normal">(last 30)</span></h2>
        <table>
          <thead><tr><th>Time</th><th>Rule</th><th>Action</th><th>IP</th><th>Method</th><th>URI</th><th>Dry</th></tr></thead>
          <tbody>
            <tr v-for="l in wafLog" :key="l.id">
              <td class="text-sm muted">{{ l.created_at?.replace('T',' ').slice(0,19) }}</td>
              <td class="mono text-sm">{{ l.rule_id || '—' }}</td>
              <td><span class="badge" :class="actionClass(l.action)">{{ l.action }}</span></td>
              <td class="text-sm">{{ l.client_ip }}</td>
              <td class="text-sm"><span class="badge">{{ l.method }}</span></td>
              <td class="text-sm" style="max-width:180px;word-break:break-all">{{ l.uri }}</td>
              <td><span v-if="l.dry_run" class="badge warning">dry</span></td>
            </tr>
            <tr v-if="!wafLog.length"><td colspan="7" class="muted">No log entries.</td></tr>
          </tbody>
        </table>
      </div>

      <!-- Rule modal -->
      <Teleport to="body">
        <div v-if="ruleModal" class="modal-overlay" @click.self="ruleModal=false">
          <div class="modal" style="max-width:500px">
            <h3>{{ editID ? 'Edit Rule' : 'Add WAF Rule' }}</h3>
            <label>Rule ID</label>
            <input v-model="ruleForm.rule_id" placeholder="rule-001" :disabled="!!editID" />
            <div class="row" style="gap:10px;margin-top:10px">
              <div style="flex:1">
                <label>Phase</label>
                <select v-model.number="ruleForm.phase">
                  <option v-for="p in PHASES" :key="p" :value="p">Phase {{ p }}</option>
                </select>
              </div>
              <div style="flex:1">
                <label>Action</label>
                <select v-model="ruleForm.action">
                  <option v-for="a in ACTIONS" :key="a">{{ a }}</option>
                </select>
              </div>
            </div>
            <label style="margin-top:10px">Pattern (regex)</label>
            <input v-model="ruleForm.pattern" placeholder="(union|select|drop)\s" style="font-family:monospace" />
            <label style="margin-top:10px">Description</label>
            <input v-model="ruleForm.description" placeholder="SQL injection protection" />
            <div style="display:flex;align-items:center;gap:6px;margin-top:10px">
              <input type="checkbox" id="ren" v-model="ruleForm.enabled" />
              <label for="ren" style="margin:0">Enabled</label>
            </div>
            <div style="display:flex;gap:8px;margin-top:14px">
              <button class="btn primary" @click="saveRule">Save</button>
              <button class="btn" @click="ruleModal=false">Cancel</button>
            </div>
          </div>
        </div>
      </Teleport>

      <!-- Dry-run test modal -->
      <Teleport to="body">
        <div v-if="testModal" class="modal-overlay" @click.self="testModal=false">
          <div class="modal" style="max-width:560px">
            <h3>Dry-run Rule Test</h3>
            <label>Method</label>
            <select v-model="testForm.method">
              <option v-for="m in ['GET','POST','PUT','DELETE','PATCH']" :key="m">{{ m }}</option>
            </select>
            <label style="margin-top:10px">URI</label>
            <input v-model="testForm.uri" placeholder="/wp-admin?id=1 UNION SELECT" />
            <label style="margin-top:10px">Body Sample</label>
            <input v-model="testForm.body_sample" placeholder="<script>alert(1)</script>" />
            <button class="btn primary" style="margin-top:12px" :disabled="testLoading" @click="runTest">
              {{ testLoading ? 'Testing…' : 'Run Test' }}
            </button>

            <div v-if="testResult" style="margin-top:14px">
              <div :class="testResult.match_count ? 'alert error' : 'alert ok'">
                {{ testResult.match_count }} rule(s) matched
              </div>
              <div v-for="m in testResult.matches" :key="m.rule_id" style="font-size:13px;margin-top:6px">
                <span class="badge error">{{ m.action }}</span>
                <span class="mono" style="margin-left:6px">{{ m.rule_id }}</span>
                <span class="muted" style="margin-left:6px">field: {{ m.field }}</span>
              </div>
            </div>
            <button class="btn" style="margin-top:14px" @click="testModal=false">Close</button>
          </div>
        </div>
      </Teleport>
    </div>
  </Layout>
</template>
