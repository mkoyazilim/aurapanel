<script setup>
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import Layout from '../components/Layout.vue'
import { useRoute } from 'vue-router'

const { t } = useI18n()

const route  = useRoute()
const siteID = route.params.id

const stats   = ref(null)  // { summary, history }
const rules   = ref([])
const error   = ref('')
const notice  = ref('')

// Purge
const purgeURLs     = ref('')
const purgeLoading  = ref(false)
const cfPurgeURLs   = ref('')
const cfPurgeLoading = ref(false)

// Rule modal
const ruleModal  = ref(false)
const editRuleID = ref(null)
const ruleForm   = ref({ pattern: '', cache_level: 'standard', ttl: 0, enabled: true })

const CACHE_LEVELS = ['bypass', 'standard', 'aggressive']

async function load() {
  const [s, r] = await Promise.all([
    fetch(`/api/v1/sites/${siteID}/cdn/stats?limit=20`).then(r => r.ok ? r.json() : null),
    fetch(`/api/v1/sites/${siteID}/cdn/rules`).then(r => r.ok ? r.json() : [])
  ])
  stats.value = s
  rules.value = r ?? []
}

const hitRatio = computed(() => {
  const s = stats.value?.summary
  if (!s) return 0
  const total = s.hits + s.misses
  return total > 0 ? ((s.hits / total) * 100).toFixed(1) : 0
})

async function purgeOLS() {
  purgeLoading.value = true
  error.value = notice.value = ''
  const urls = purgeURLs.value.split('\n').map(u => u.trim()).filter(u => u)
  const res = await fetch(`/api/v1/sites/${siteID}/cdn/purge`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ urls, purge_all: urls.length === 0 })
  })
  purgeLoading.value = false
  purgeURLs.value = ''
  if (!res.ok) { const d = await res.json(); error.value = d.error || t('cdn.purge_failed'); return }
  const d = await res.json()
  notice.value = t('cdn.ols_cache_purged', { purged: d.purged })
  load()
}

async function purgeCF() {
  cfPurgeLoading.value = true
  error.value = notice.value = ''
  const urls = cfPurgeURLs.value.split('\n').map(u => u.trim()).filter(u => u)
  const res = await fetch(`/api/v1/sites/${siteID}/cdn/cf-purge`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ urls, purge_all: urls.length === 0 })
  })
  cfPurgeLoading.value = false
  cfPurgeURLs.value = ''
  if (!res.ok) { const d = await res.json(); error.value = d.error || t('cdn.cf_purge_failed'); return }
  notice.value = t('cdn.cf_cache_purged')
  load()
}

function openAdd() {
  editRuleID.value = null
  ruleForm.value   = { pattern: '', cache_level: 'standard', ttl: 0, enabled: true }
  ruleModal.value  = true
}

function openEdit(rule) {
  editRuleID.value = rule.id
  ruleForm.value   = { ...rule }
  ruleModal.value  = true
}

async function saveRule() {
  error.value = notice.value = ''
  const isEdit = editRuleID.value !== null
  const url    = isEdit ? `/api/v1/sites/${siteID}/cdn/rules/${editRuleID.value}` : `/api/v1/sites/${siteID}/cdn/rules`
  const res = await fetch(url, {
    method: isEdit ? 'PUT' : 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(ruleForm.value)
  })
  ruleModal.value = false
  if (!res.ok) { const d = await res.json(); error.value = d.error || t('cdn.save_failed'); return }
  notice.value = isEdit ? t('cdn.rule_updated') : t('cdn.rule_added')
  load()
}

async function deleteRule(id) {
  if (!confirm(t('cdn.delete_rule_confirm'))) return
  await fetch(`/api/v1/sites/${siteID}/cdn/rules/${id}`, { method: 'DELETE' })
  load()
}

function levelClass(l) {
  if (l === 'bypass') return 'error'
  if (l === 'aggressive') return 'ok'
  return ''
}

function barColor(pct) {
  if (pct >= 80) return '#22c55e'
  if (pct >= 50) return '#f59e0b'
  return '#ef4444'
}

onMounted(load)
</script>

<template>
  <Layout>
    <div class="page">
      <h1>{{ $t('cdn.cdn_management') }} <span style="font-size:14px;color:var(--muted);font-weight:normal;margin-left:8px">{{ siteID }}</span></h1>
      <div v-if="error"  class="alert error">{{ error }}</div>
      <div v-if="notice" class="alert ok">{{ notice }}</div>

      <!-- Stats summary -->
      <div v-if="stats" style="display:grid;grid-template-columns:repeat(auto-fill,minmax(180px,1fr));gap:12px;margin-bottom:20px">
        <div class="card" style="text-align:center">
          <div style="font-size:32px;font-weight:700;color:#22c55e">{{ stats.summary?.hits ?? 0 }}</div>
          <div class="muted">{{ $t('cdn.cache_hits') }}</div>
        </div>
        <div class="card" style="text-align:center">
          <div style="font-size:32px;font-weight:700;color:#ef4444">{{ stats.summary?.misses ?? 0 }}</div>
          <div class="muted">{{ $t('cdn.cache_misses') }}</div>
        </div>
        <div class="card" style="text-align:center">
          <div style="font-size:32px;font-weight:700">{{ stats.summary?.purges ?? 0 }}</div>
          <div class="muted">{{ $t('cdn.purges') }}</div>
        </div>
        <div class="card" style="text-align:center">
          <div style="font-size:32px;font-weight:700" :style="`color:${barColor(hitRatio)}`">{{ hitRatio }}%</div>
          <div class="muted">{{ $t('cdn.hit_ratio') }}</div>
          <div style="background:#e5e7eb;border-radius:4px;height:8px;margin-top:6px">
            <div :style="`width:${hitRatio}%;background:${barColor(hitRatio)};height:8px;border-radius:4px`"></div>
          </div>
        </div>
      </div>

      <!-- Purge panels -->
      <div style="display:grid;grid-template-columns:1fr 1fr;gap:12px;margin-bottom:20px">
        <!-- OLS -->
        <div class="card">
          <h3>{{ $t('cdn.ols_cache_purge') }}</h3>
          <textarea v-model="purgeURLs" rows="3" :placeholder="$t('cdn.purge_placeholder')"
            style="width:100%;font-size:13px;font-family:monospace;resize:vertical"></textarea>
          <button class="btn primary" style="margin-top:8px" :disabled="purgeLoading" @click="purgeOLS">
            {{ purgeLoading ? $t('cdn.purging') : $t('cdn.purge_ols_btn') }}
          </button>
        </div>
        <!-- CF -->
        <div class="card">
          <h3>{{ $t('cdn.cf_cache_purge') }}</h3>
          <textarea v-model="cfPurgeURLs" rows="3" :placeholder="$t('cdn.purge_placeholder')"
            style="width:100%;font-size:13px;font-family:monospace;resize:vertical"></textarea>
          <button class="btn warning" style="margin-top:8px" :disabled="cfPurgeLoading" @click="purgeCF">
            {{ cfPurgeLoading ? $t('cdn.purging') : $t('cdn.purge_cf_btn') }}
          </button>
        </div>
      </div>

      <!-- Cache rules -->
      <div class="card">
        <div style="display:flex;justify-content:space-between;align-items:center">
          <h2 style="margin:0">{{ $t('cdn.cache_rules') }}</h2>
          <button class="btn primary" @click="openAdd">{{ $t('cdn.add_rule_btn') }}</button>
        </div>
        <table style="margin-top:12px">
          <thead>
            <tr><th>{{ $t('cdn.pattern') }}</th><th>{{ $t('cdn.level') }}</th><th>{{ $t('cdn.ttl_s') }}</th><th>{{ $t('cdn.enabled') }}</th><th></th></tr>
          </thead>
          <tbody>
            <tr v-for="r in rules" :key="r.id">
              <td class="mono text-sm">{{ r.pattern }}</td>
              <td><span class="badge" :class="levelClass(r.cache_level)">{{ r.cache_level }}</span></td>
              <td class="text-sm muted">{{ r.ttl || $t('cdn.default_ttl') }}</td>
              <td><span class="badge" :class="r.enabled?'ok':'error'">{{ r.enabled ? $t('cdn.on') : $t('cdn.off') }}</span></td>
              <td style="display:flex;gap:4px">
                <button class="btn" style="padding:2px 8px" @click="openEdit(r)">{{ $t('cdn.edit_btn') }}</button>
                <button class="btn danger" style="padding:2px 8px" @click="deleteRule(r.id)">✕</button>
              </td>
            </tr>
            <tr v-if="!rules.length"><td colspan="5" class="muted">{{ $t('cdn.no_rules') }}</td></tr>
          </tbody>
        </table>
      </div>

      <!-- History chart (simple table) -->
      <div v-if="stats?.history?.length" class="card" style="margin-top:16px">
        <h2>{{ $t('cdn.purge_history') }}</h2>
        <table>
          <thead><tr><th>{{ $t('cdn.time') }}</th><th>{{ $t('cdn.source') }}</th><th>{{ $t('cdn.hits') }}</th><th>{{ $t('cdn.misses') }}</th><th>{{ $t('cdn.purges') }}</th></tr></thead>
          <tbody>
            <tr v-for="h in stats.history" :key="h.id">
              <td class="text-sm muted">{{ h.recorded_at?.replace('T',' ').slice(0,19) }}</td>
              <td><span class="badge" :class="h.source==='cloudflare'?'ok':''">{{ h.source }}</span></td>
              <td class="text-sm">{{ h.hits }}</td>
              <td class="text-sm">{{ h.misses }}</td>
              <td class="text-sm">{{ h.purges }}</td>
            </tr>
          </tbody>
        </table>
      </div>

      <!-- Rule modal -->
      <Teleport to="body">
        <div v-if="ruleModal" class="modal-overlay" @click.self="ruleModal=false">
          <div class="modal" style="max-width:440px">
            <h3>{{ editRuleID ? $t('cdn.edit_cache_rule') : $t('cdn.add_cache_rule') }}</h3>
            <label>{{ $t('cdn.url_pattern') }}</label>
            <input v-model="ruleForm.pattern" :placeholder="$t('cdn.pattern_placeholder')" />
            <label style="margin-top:10px">{{ $t('cdn.cache_level') }}</label>
            <select v-model="ruleForm.cache_level">
              <option v-for="l in CACHE_LEVELS" :key="l">{{ l }}</option>
            </select>
            <label style="margin-top:10px">{{ $t('cdn.ttl_label') }}</label>
            <input v-model.number="ruleForm.ttl" type="number" min="0" />
            <div style="display:flex;align-items:center;gap:6px;margin-top:10px">
              <input type="checkbox" id="cren" v-model="ruleForm.enabled" />
              <label for="cren" style="margin:0">{{ $t('cdn.enabled_label') }}</label>
            </div>
            <div style="display:flex;gap:8px;margin-top:14px">
              <button class="btn primary" @click="saveRule">{{ $t('cdn.save') }}</button>
              <button class="btn" @click="ruleModal=false">{{ $t('cdn.cancel') }}</button>
            </div>
          </div>
        </div>
      </Teleport>
    </div>
  </Layout>
</template>
