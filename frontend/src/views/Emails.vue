<template>
  <div class="space-y-6">
    <div class="flex items-center justify-between gap-3">
      <div>
        <h1 class="text-2xl font-bold text-white">{{ t('email_manager.title') }}</h1>
        <p class="text-gray-400 mt-1">{{ t('email_manager.subtitle') }}</p>
      </div>
      <button class="btn-primary" @click="showAddModal = true">{{ t('email_manager.add_mailbox') }}</button>
    </div>

    <div class="border-b border-panel-border">
      <nav class="flex gap-6">
        <button @click="tab='mailboxes'" :class="tabClass('mailboxes')">{{ t('email_manager.tabs.mailboxes') }}</button>
        <button @click="tab='forwards'" :class="tabClass('forwards')">{{ t('email_manager.tabs.forwards') }}</button>
        <button @click="tab='routing'" :class="tabClass('routing')">{{ t('email_manager.tabs.routing') }}</button>
        <button @click="tab='dkim'" :class="tabClass('dkim')">{{ t('email_manager.tabs.dkim') }}</button>
        <button @click="tab='tuning'" :class="tabClass('tuning')">{{ t('email_manager.tuning_tab') }}</button>
        <button @click="tab='premium'" :class="tabClass('premium')">{{ t('email_manager.tabs.premium') }}</button>
      </nav>
    </div>

    <div v-if="error" class="aura-card border-red-500/30 bg-red-500/5 text-red-400">{{ error }}</div>

    <!-- Tuning Tab -->
    <div v-if="tab === 'tuning'" class="space-y-6">
      <div class="aura-card">
        <div class="flex items-center justify-between mb-4">
          <div>
            <h2 class="text-lg font-bold text-white">{{ t('email_manager.tuning.title') }}</h2>
            <p class="text-sm text-gray-400">{{ t('email_manager.tuning.desc') }}</p>
          </div>
          <button class="btn-secondary" @click="loadTuning">{{ t('common.refresh') }}</button>
        </div>
        
        <div class="grid grid-cols-1 md:grid-cols-2 gap-6">
          <div>
            <label class="block text-sm text-gray-400 mb-1">{{ t('email_manager.tuning.message_size_limit') }}</label>
            <input v-model="tuningForm.message_size_limit" type="text" class="aura-input w-full" :placeholder="t('email_manager.tuning_example_message_size')" />
            <p class="text-xs text-gray-500 mt-1">{{ t('email_manager.tuning.msg_size_desc') }}</p>
          </div>
          <div>
            <label class="block text-sm text-gray-400 mb-1">{{ t('email_manager.tuning.mailbox_size_limit') }}</label>
            <input v-model="tuningForm.mailbox_size_limit" type="text" class="aura-input w-full" :placeholder="t('email_manager.tuning_example_mailbox_size')" />
            <p class="text-xs text-gray-500 mt-1">{{ t('email_manager.tuning.mbox_size_desc') }}</p>
          </div>
          <div>
            <label class="block text-sm text-gray-400 mb-1">{{ t('email_manager.tuning.max_client_connections') }}</label>
            <input v-model="tuningForm.smtpd_client_connection_count_limit" type="text" class="aura-input w-full" :placeholder="t('email_manager.tuning_example_client_limit')" />
            <p class="text-xs text-gray-500 mt-1">{{ t('email_manager.tuning.max_conn_desc') }}</p>
          </div>
        </div>
        
        <div class="mt-6 flex justify-end">
          <button class="btn-primary" @click="saveTuning" :disabled="tuningSaving">
            {{ tuningSaving ? t('email_manager.tuning.saving') : t('email_manager.tuning.save') }}
          </button>
        </div>
      </div>
    </div>

    <div v-if="tab === 'premium'" class="space-y-6">
      <div class="grid grid-cols-1 gap-4 xl:grid-cols-3">
        <div class="aura-card space-y-4 xl:col-span-2">
          <div class="flex items-center justify-between gap-2">
            <div>
              <h2 class="text-lg font-semibold text-white">{{ t('email_manager.premium.auth.title') }}</h2>
              <p class="text-sm text-gray-400">{{ t('email_manager.premium.auth.subtitle') }}</p>
            </div>
            <button class="btn-secondary" @click="loadMailAuth">{{ t('common.refresh') }}</button>
          </div>
          <div class="grid grid-cols-1 gap-3 md:grid-cols-2">
            <select v-model="premiumForm.domain" class="aura-input">
              <option disabled value="">{{ t('email_manager.forwards.domain_placeholder') }}</option>
              <option v-for="d in domains" :key="`auth-domain-${d}`" :value="d">{{ d }}</option>
            </select>
            <select v-model="premiumForm.policy" class="aura-input">
              <option value="none">{{ t('email_manager.policy_none') }}</option>
              <option value="quarantine">{{ t('email_manager.policy_quarantine') }}</option>
              <option value="reject">{{ t('email_manager.policy_reject') }}</option>
            </select>
            <input v-model="premiumForm.rua" class="aura-input" :placeholder="t('email_manager.premium.auth.rua')" />
            <input v-model="premiumForm.ruf" class="aura-input" :placeholder="t('email_manager.premium.auth.ruf')" />
          </div>
          <div class="flex gap-2">
            <button class="btn-primary" @click="bootstrapAuth">{{ t('email_manager.premium.auth.bootstrap') }}</button>
          </div>
          <div v-if="mailAuthRecord" class="rounded-xl border border-panel-border p-3 space-y-3">
            <div>
              <p class="text-xs uppercase tracking-wide text-gray-500">SPF</p>
              <p class="text-sm text-gray-300">{{ mailAuthRecord.spf_host }}</p>
              <textarea class="aura-input w-full font-mono text-xs mt-1" rows="2" :value="mailAuthRecord.spf_value" readonly />
            </div>
            <div>
              <p class="text-xs uppercase tracking-wide text-gray-500">DMARC</p>
              <p class="text-sm text-gray-300">{{ mailAuthRecord.dmarc_host }}</p>
              <textarea class="aura-input w-full font-mono text-xs mt-1" rows="3" :value="mailAuthRecord.dmarc_value" readonly />
            </div>
          </div>
        </div>

        <div class="aura-card space-y-3">
          <div class="flex items-center justify-between gap-2">
            <h2 class="text-lg font-semibold text-white">{{ t('email_manager.premium.deliverability.title') }}</h2>
            <button class="btn-secondary" @click="loadDeliverability">{{ t('common.refresh') }}</button>
          </div>
          <div class="rounded-xl border border-panel-border bg-panel-dark/40 p-3">
            <p class="text-xs text-gray-500">{{ t('email_manager.premium.deliverability.score') }}</p>
            <p class="mt-1 text-3xl font-bold text-white">{{ deliverability.score ?? '-' }}</p>
            <p class="text-sm mt-1" :class="deliverabilityRiskClass(deliverability.risk)">{{ deliverability.risk || '-' }}</p>
          </div>
          <div class="space-y-2 text-sm">
            <p class="text-gray-300">{{ t('email_manager.dkim_label') }}: <span :class="deliverabilityCheckClass(deliverability?.checks?.dkim)">{{ yesNo(deliverability?.checks?.dkim) }}</span></p>
            <p class="text-gray-300">{{ t('email_manager.spf_dmarc_label') }}: <span :class="deliverabilityCheckClass(deliverability?.checks?.spf_dmarc)">{{ yesNo(deliverability?.checks?.spf_dmarc) }}</span></p>
            <p class="text-gray-300">{{ t('email_manager.premium.deliverability.mailboxes') }}: <span class="text-white">{{ deliverability?.observability?.mailboxes ?? 0 }}</span></p>
            <p class="text-gray-300">{{ t('email_manager.premium.deliverability.catch_all') }}: <span :class="deliverabilityCheckClass(!(deliverability?.observability?.catch_all_enabled))">{{ deliverability?.observability?.catch_all_enabled ? t('common.active') : t('common.inactive') }}</span></p>
          </div>
        </div>
      </div>

      <div class="aura-card space-y-4">
        <div class="flex items-center justify-between gap-2">
          <div>
            <h2 class="text-lg font-semibold text-white">{{ t('email_manager.premium.webmail_ops.title') }}</h2>
            <p class="text-sm text-gray-400">{{ t('email_manager.premium.webmail_ops.subtitle') }}</p>
          </div>
          <button class="btn-secondary" @click="loadWebmailOps">{{ t('common.refresh') }}</button>
        </div>
        <div class="grid grid-cols-1 gap-3 md:grid-cols-3">
          <div class="rounded-lg border border-panel-border bg-panel-dark/40 p-3">
            <p class="text-xs text-gray-500">{{ t('email_manager.premium.webmail_ops.tokens_total') }}</p>
            <p class="text-xl font-semibold text-white">{{ webmailStatus.tokens_total ?? 0 }}</p>
          </div>
          <div class="rounded-lg border border-panel-border bg-panel-dark/40 p-3">
            <p class="text-xs text-gray-500">{{ t('email_manager.premium.webmail_ops.tokens_active') }}</p>
            <p class="text-xl font-semibold text-white">{{ webmailStatus.tokens_active ?? 0 }}</p>
          </div>
          <div class="rounded-lg border border-panel-border bg-panel-dark/40 p-3">
            <p class="text-xs text-gray-500">{{ t('email_manager.premium.webmail_ops.tokens_expired') }}</p>
            <p class="text-xl font-semibold text-white">{{ webmailStatus.tokens_expired ?? 0 }}</p>
          </div>
        </div>
        <div class="grid grid-cols-1 gap-3 md:grid-cols-4">
          <input v-model="webmailOpsForm.address" class="aura-input md:col-span-2" :placeholder="t('email_manager.premium.webmail_ops.address_placeholder')" />
          <input v-model="webmailOpsForm.token" class="aura-input" :placeholder="t('email_manager.premium.webmail_ops.token_placeholder')" />
          <button class="btn-secondary" @click="revokeWebmailTokens">{{ t('email_manager.premium.webmail_ops.revoke') }}</button>
        </div>
        <div class="flex gap-2">
          <button class="btn-secondary" @click="cleanupExpiredTokens">{{ t('email_manager.premium.webmail_ops.cleanup_expired') }}</button>
        </div>
        <div class="overflow-x-auto">
          <table class="w-full text-sm">
            <thead>
              <tr class="border-b border-panel-border text-gray-400">
                <th class="text-left py-2 px-2">{{ t('email_manager.premium.webmail_ops.table.token') }}</th>
                <th class="text-left py-2 px-2">{{ t('email_manager.premium.webmail_ops.table.address') }}</th>
                <th class="text-left py-2 px-2">{{ t('email_manager.premium.webmail_ops.table.expires') }}</th>
                <th class="text-left py-2 px-2">{{ t('email_manager.premium.webmail_ops.table.status') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="token in webmailTokens" :key="`webmail-token-${token.token_preview}-${token.expires_at}`" class="border-b border-panel-border/40">
                <td class="py-2 px-2 font-mono text-xs text-gray-200">{{ token.token_preview }}</td>
                <td class="py-2 px-2 text-gray-200">{{ token.address }}</td>
                <td class="py-2 px-2 text-gray-300">{{ formatDateTime(token.expires_at) }}</td>
                <td class="py-2 px-2">
                  <span :class="token.expired ? 'text-yellow-400' : 'text-green-400'">
                    {{ token.expired ? t('email_manager.premium.webmail_ops.expired') : t('email_manager.premium.webmail_ops.active') }}
                  </span>
                </td>
              </tr>
              <tr v-if="webmailTokens.length === 0">
                <td colspan="4" class="py-6 text-center text-gray-500">{{ t('email_manager.premium.webmail_ops.empty') }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </div>

    <div v-if="tab === 'mailboxes'" class="space-y-4">
      <div class="flex items-center justify-between">
        <p class="text-sm text-gray-400">{{ t('email_manager.mailbox.count', { count: mailboxes.length }) }}</p>
        <button class="btn-secondary" @click="loadMailboxes">{{ t('email_manager.mailbox.refresh') }}</button>
      </div>

      <div v-if="mailboxes.length === 0" class="aura-card text-center py-12 text-gray-500">{{ t('email_manager.mailbox.empty') }}</div>

      <div v-else class="space-y-3">
        <div v-for="mb in mailboxes" :key="mb.address" class="aura-card flex flex-col sm:flex-row gap-3 justify-between sm:items-center">
          <div>
            <p class="text-white font-semibold">{{ mb.address }}</p>
            <p class="text-xs text-gray-400">{{ t('email_manager.mailbox.quota', { used: mb.used_mb, quota: mb.quota_mb }) }}</p>
          </div>
          <div class="flex gap-2">
            <button class="btn-secondary px-2 py-1 text-xs" @click="resetMailboxPassword(mb.address)">{{ t('email_manager.mailbox.password') }}</button>
            <button class="btn-secondary px-2 py-1 text-xs" @click="generateWebmailSso(mb.address)">{{ t('email_manager.mailbox.webmail_sso') }}</button>
            <button class="btn-danger px-2 py-1 text-xs" @click="deleteMailbox(mb.address)">{{ t('email_manager.mailbox.delete') }}</button>
          </div>
        </div>
      </div>
    </div>

    <div v-if="tab === 'forwards'" class="space-y-4">
      <div class="aura-card space-y-4">
        <h3 class="text-white font-semibold">{{ t('email_manager.forwards.title') }}</h3>
        <div class="grid grid-cols-1 md:grid-cols-3 gap-3">
          <select v-model="forwardForm.domain" class="aura-input">
            <option disabled value="">{{ t('email_manager.forwards.domain_placeholder') }}</option>
            <option v-for="d in domains" :key="d" :value="d">{{ d }}</option>
          </select>
          <input v-model="forwardForm.source" class="aura-input" :placeholder="t('email_manager.forwards.source_placeholder')" />
          <input v-model="forwardForm.target" class="aura-input" :placeholder="t('email_manager.forwards.target_placeholder')" />
        </div>
        <div class="flex gap-2">
          <button class="btn-primary" @click="addForward">{{ t('email_manager.forwards.add') }}</button>
          <button class="btn-secondary" @click="loadForwards">{{ t('email_manager.forwards.refresh') }}</button>
        </div>
      </div>

      <div class="aura-card space-y-3">
        <h3 class="text-white font-semibold">{{ t('email_manager.forwards.catchall_title') }}</h3>
        <div class="grid grid-cols-1 md:grid-cols-3 gap-3">
          <select v-model="catchAllForm.domain" class="aura-input">
            <option disabled value="">{{ t('email_manager.forwards.domain_placeholder') }}</option>
            <option v-for="d in domains" :key="d" :value="d">{{ d }}</option>
          </select>
          <input v-model="catchAllForm.target" class="aura-input" :placeholder="t('email_manager.forwards.catchall_placeholder')" />
          <label class="inline-flex items-center gap-2 text-sm text-gray-300">
            <input v-model="catchAllForm.enabled" type="checkbox" class="w-4 h-4" />
            {{ t('email_manager.forwards.enabled') }}
          </label>
        </div>
        <button class="btn-primary" @click="saveCatchAll">{{ t('email_manager.forwards.save') }}</button>
      </div>

      <div class="aura-card overflow-x-auto">
        <table class="w-full text-sm">
          <thead>
            <tr class="border-b border-panel-border text-gray-400">
              <th class="text-left py-2 px-2">{{ t('email_manager.forwards.table.domain') }}</th>
              <th class="text-left py-2 px-2">{{ t('email_manager.forwards.table.source') }}</th>
              <th class="text-left py-2 px-2">{{ t('email_manager.forwards.table.target') }}</th>
              <th class="text-right py-2 px-2">{{ t('email_manager.forwards.table.action') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="f in forwards" :key="`${f.domain}-${f.source}`" class="border-b border-panel-border/40">
              <td class="py-2 px-2 text-gray-200">{{ f.domain }}</td>
              <td class="py-2 px-2 text-white font-mono">{{ f.source }}</td>
              <td class="py-2 px-2 text-gray-300">{{ f.target }}</td>
              <td class="py-2 px-2 text-right">
                <button class="btn-danger px-2 py-1 text-xs" @click="deleteForward(f)">{{ t('email_manager.mailbox.delete') }}</button>
              </td>
            </tr>
            <tr v-if="forwards.length === 0"><td colspan="4" class="py-8 text-center text-gray-500">{{ t('email_manager.forwards.table.empty') }}</td></tr>
          </tbody>
        </table>
      </div>
    </div>

    <div v-if="tab === 'routing'" class="space-y-4">
      <div class="aura-card space-y-3">
        <h3 class="text-white font-semibold">{{ t('email_manager.routing.title') }}</h3>
        <div class="grid grid-cols-1 md:grid-cols-4 gap-3">
          <select v-model="routingForm.domain" class="aura-input">
            <option disabled value="">{{ t('email_manager.forwards.domain_placeholder') }}</option>
            <option v-for="d in domains" :key="d" :value="d">{{ d }}</option>
          </select>
          <input v-model="routingForm.pattern" class="aura-input" :placeholder="t('email_manager.routing.pattern_placeholder')" />
          <input v-model="routingForm.target" class="aura-input" :placeholder="t('email_manager.routing.target_placeholder')" />
          <input v-model.number="routingForm.priority" type="number" class="aura-input" :placeholder="t('email_manager.routing.priority_placeholder')" />
        </div>
        <button class="btn-primary" @click="addRouting">{{ t('email_manager.routing.add') }}</button>
      </div>

      <div class="aura-card overflow-x-auto">
        <table class="w-full text-sm">
          <thead>
            <tr class="border-b border-panel-border text-gray-400">
              <th class="text-left py-2 px-2">{{ t('email_manager.routing.table.domain') }}</th>
              <th class="text-left py-2 px-2">{{ t('email_manager.routing.table.pattern') }}</th>
              <th class="text-left py-2 px-2">{{ t('email_manager.routing.table.target') }}</th>
              <th class="text-left py-2 px-2">{{ t('email_manager.routing.table.priority') }}</th>
              <th class="text-right py-2 px-2">{{ t('email_manager.routing.table.action') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="r in routingRules" :key="r.id" class="border-b border-panel-border/40">
              <td class="py-2 px-2 text-gray-200">{{ r.domain }}</td>
              <td class="py-2 px-2 text-white font-mono">{{ r.pattern }}</td>
              <td class="py-2 px-2 text-gray-300">{{ r.target }}</td>
              <td class="py-2 px-2 text-gray-300">{{ r.priority }}</td>
              <td class="py-2 px-2 text-right">
                <button class="btn-danger px-2 py-1 text-xs" @click="deleteRouting(r)">{{ t('email_manager.mailbox.delete') }}</button>
              </td>
            </tr>
            <tr v-if="routingRules.length === 0"><td colspan="5" class="py-8 text-center text-gray-500">{{ t('email_manager.routing.table.empty') }}</td></tr>
          </tbody>
        </table>
      </div>
    </div>

    <div v-if="tab === 'dkim'" class="space-y-4">
      <div class="aura-card space-y-3">
        <h3 class="text-white font-semibold">{{ t('email_manager.dkim.title') }}</h3>
        <div class="flex gap-2">
          <select v-model="dkimDomain" class="aura-input max-w-sm">
            <option disabled value="">{{ t('email_manager.forwards.domain_placeholder') }}</option>
            <option v-for="d in domains" :key="d" :value="d">{{ d }}</option>
          </select>
          <button class="btn-secondary" @click="loadDkim">{{ t('email_manager.dkim.fetch') }}</button>
          <button class="btn-primary" @click="rotateDkim">{{ t('email_manager.dkim.rotate') }}</button>
        </div>
        <div v-if="dkimRecord" class="rounded-xl border border-panel-border p-3 space-y-2">
          <p class="text-sm text-gray-300">{{ t('email_manager.dkim.selector') }}: <span class="text-white font-mono">{{ dkimRecord.selector }}</span></p>
          <p class="text-xs text-gray-400">{{ t('email_manager.dkim.txt') }}: {{ dkimRecord.selector }}._domainkey.{{ dkimRecord.domain }}</p>
          <textarea class="aura-input w-full font-mono text-xs" rows="4" :value="dkimRecord.public_key" readonly />
        </div>
      </div>
    </div>

    <Teleport to="body">
      <div v-if="showAddModal" class="fixed inset-0 bg-black/60 flex items-center justify-center z-50 p-4">
        <div class="bg-panel-card border border-panel-border rounded-2xl p-6 w-full max-w-md">
          <h2 class="text-xl font-bold text-white mb-4">{{ t('email_manager.mailbox.modal_title') }}</h2>
          <div class="space-y-3">
            <input v-model="mailboxForm.address" class="aura-input w-full" :placeholder="t('email_manager.mailbox.address_placeholder')" />
            <input v-model="mailboxForm.password" type="password" class="aura-input w-full" :placeholder="t('email_manager.mailbox.password_placeholder')" />
            <input v-model.number="mailboxForm.quota_mb" type="number" class="aura-input w-full" :placeholder="t('email_manager.mailbox.quota_placeholder')" />
          </div>
          <div class="flex gap-3 mt-6">
            <button class="btn-secondary flex-1" @click="showAddModal=false">{{ t('email_manager.mailbox.cancel') }}</button>
            <button class="btn-primary flex-1" @click="addMailbox">{{ t('email_manager.mailbox.create') }}</button>
          </div>
        </div>
      </div>
    </Teleport>
  </div>
</template>

<script setup>
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import api from '../services/api'

const { t } = useI18n({ useScope: 'global' })
const route = useRoute()

function normalizeTab(value) {
  const allowedTabs = ['mailboxes', 'forwards', 'routing', 'dkim', 'tuning', 'premium']
  const normalized = String(value || '').trim().toLowerCase()
  return allowedTabs.includes(normalized) ? normalized : 'mailboxes'
}

const tab = ref(normalizeTab(typeof route.query.tab === 'string' ? route.query.tab : 'mailboxes'))
const error = ref('')
const showAddModal = ref(false)
const createActionConsumed = ref(false)

const tuningForm = ref({
  message_size_limit: '',
  mailbox_size_limit: '',
  smtpd_client_connection_count_limit: ''
})
const tuningSaving = ref(false)
const premiumForm = ref({
  domain: '',
  policy: 'quarantine',
  rua: '',
  ruf: '',
})
const mailAuthRecord = ref(null)
const deliverability = ref({})
const webmailStatus = ref({})
const webmailTokens = ref([])
const webmailOpsForm = ref({
  address: '',
  token: '',
})

async function loadTuning() {
  try {
    const res = await api.get('/mail/tuning')
    if (res.data?.data) {
      tuningForm.value = { ...tuningForm.value, ...res.data.data }
    }
  } catch (err) {
    console.error('Mail tuning load error', err)
  }
}

async function saveTuning() {
  tuningSaving.value = true
  try {
    await api.post('/mail/tuning', tuningForm.value)
    alert(t('email_manager.tuning_save_success'))
  } catch (err) {
    alert(`${t('common.error')}: ${err.response?.data?.message || err.message}`)
  } finally {
    tuningSaving.value = false
  }
}

watch(tab, (newVal) => {
  if (newVal === 'tuning') {
    loadTuning()
  }
  if (newVal === 'premium') {
    loadPremiumData()
  }
})

const mailboxes = ref([])
const forwards = ref([])
const routingRules = ref([])
const dkimRecord = ref(null)
const dkimDomain = ref('')

const sites = ref([])

const mailboxForm = ref({
  address: '',
  password: '',
  quota_mb: 2048,
})

const forwardForm = ref({
  domain: '',
  source: '',
  target: '',
})

const catchAllForm = ref({
  domain: '',
  enabled: false,
  target: '',
})

const routingForm = ref({
  domain: '',
  pattern: '',
  target: '',
  priority: 100,
})

const domains = computed(() => {
  const fromSites = (sites.value || []).map(s => s.domain).filter(Boolean)
  const fromMailbox = (mailboxes.value || []).map(m => m.domain).filter(Boolean)
  return [...new Set([...fromSites, ...fromMailbox])]
})

function applyDomainQuery(queryDomain) {
  const requested = String(queryDomain || '').trim().toLowerCase()
  if (!requested) return

  const matched = domains.value.find((domainName) => String(domainName || '').trim().toLowerCase() === requested)
  if (!matched) return

  forwardForm.value.domain = matched
  catchAllForm.value.domain = matched
  routingForm.value.domain = matched
  dkimDomain.value = matched

  const currentAddress = String(mailboxForm.value.address || '').trim()
  if (!currentAddress) {
    mailboxForm.value.address = `info@${matched}`
  }
}

function applyRouteQuery(query) {
  const queryTab = normalizeTab(typeof query?.tab === 'string' ? query.tab : 'mailboxes')
  if (tab.value !== queryTab) {
    tab.value = queryTab
  }

  applyDomainQuery(query?.domain)

  const shouldCreateMailbox = query?.action === 'create'
  if (shouldCreateMailbox && !createActionConsumed.value) {
    tab.value = 'mailboxes'
    showAddModal.value = true
    createActionConsumed.value = true
  }
  if (!shouldCreateMailbox) {
    createActionConsumed.value = false
  }
}

function tabClass(key) {
  return [
    'pb-3 text-sm font-medium transition',
    tab.value === key ? 'text-brand-400 border-b-2 border-brand-400' : 'text-gray-400 hover:text-white',
  ]
}

function formatDateTime(value) {
  const ts = Number(value || 0)
  if (!ts) return '-'
  return new Date(ts).toLocaleString()
}

function yesNo(value) {
  return value ? t('common.active') : t('common.inactive')
}

function deliverabilityCheckClass(value) {
  return value ? 'text-green-400' : 'text-yellow-400'
}

function deliverabilityRiskClass(value) {
  const key = String(value || '').toLowerCase()
  if (key === 'low') return 'text-green-400'
  if (key === 'medium') return 'text-yellow-400'
  return 'text-red-400'
}

function apiErrorMessage(e, fallbackKey) {
  return e?.response?.data?.message || e?.message || t(fallbackKey)
}

function resolveSiteOwner(domain) {
  const normalized = String(domain || '').trim().toLowerCase()
  if (!normalized) return undefined
  const site = (sites.value || []).find(s => String(s.domain || '').trim().toLowerCase() === normalized)
  const owner = String(site?.owner || site?.user || '').trim()
  return owner || undefined
}

async function loadSites() {
  try {
    const res = await api.get('/vhost/list')
    sites.value = res.data?.data || []
  } catch {
    sites.value = []
  }
}

async function loadMailboxes() {
  try {
    const res = await api.get('/mail/list')
    mailboxes.value = res.data?.data || []
  } catch (e) {
    error.value = apiErrorMessage(e, 'email_manager.messages.mailbox_list_failed')
  }
}

async function addMailbox() {
  const address = String(mailboxForm.value.address || '').trim().toLowerCase()
  if (!address || !address.includes('@')) return
  const [username, domain] = address.split('@')
  const owner = resolveSiteOwner(domain)
  try {
    await api.post('/mail/create', {
      domain,
      username,
      password: mailboxForm.value.password || '',
      quota_mb: Number(mailboxForm.value.quota_mb || 2048),
      owner,
    })
    showAddModal.value = false
    mailboxForm.value = { address: '', password: '', quota_mb: 2048 }
    await loadMailboxes()
  } catch (e) {
    error.value = apiErrorMessage(e, 'email_manager.messages.mailbox_create_failed')
  }
}

async function deleteMailbox(address) {
  if (!window.confirm(t('email_manager.messages.mailbox_delete_confirm', { address }))) return
  try {
    await api.post('/mail/delete', { address })
    await loadMailboxes()
  } catch (e) {
    error.value = apiErrorMessage(e, 'email_manager.messages.mailbox_delete_failed')
  }
}

async function resetMailboxPassword(address) {
  const nextPassword = window.prompt(t('email_manager.messages.mailbox_password_prompt', { address }))
  if (!nextPassword) return
  try {
    await api.post('/mail/password', { address, new_password: nextPassword })
  } catch (e) {
    error.value = apiErrorMessage(e, 'email_manager.messages.mailbox_password_failed')
  }
}

async function loadForwards() {
  try {
    const res = await api.get('/mail/forwards')
    forwards.value = res.data?.data || []
  } catch (e) {
    error.value = apiErrorMessage(e, 'email_manager.messages.forwards_failed')
  }
}

async function addForward() {
  if (!forwardForm.value.domain || !forwardForm.value.source || !forwardForm.value.target) return
  try {
    await api.post('/mail/forwards', {
      domain: forwardForm.value.domain,
      source: forwardForm.value.source,
      target: forwardForm.value.target,
    })
    forwardForm.value.source = ''
    forwardForm.value.target = ''
    await loadForwards()
  } catch (e) {
    error.value = apiErrorMessage(e, 'email_manager.messages.forward_create_failed')
  }
}

async function deleteForward(rule) {
  try {
    await api.delete('/mail/forwards', { data: { domain: rule.domain, source: rule.source } })
    await loadForwards()
  } catch (e) {
    error.value = apiErrorMessage(e, 'email_manager.messages.forward_delete_failed')
  }
}

async function saveCatchAll() {
  if (!catchAllForm.value.domain) return
  try {
    await api.post('/mail/catch-all', {
      domain: catchAllForm.value.domain,
      enabled: !!catchAllForm.value.enabled,
      target: catchAllForm.value.target || '',
    })
  } catch (e) {
    error.value = apiErrorMessage(e, 'email_manager.messages.catchall_failed')
  }
}

async function loadRouting() {
  try {
    const res = await api.get('/mail/routing')
    routingRules.value = res.data?.data || []
  } catch (e) {
    error.value = apiErrorMessage(e, 'email_manager.messages.routing_failed')
  }
}

async function addRouting() {
  if (!routingForm.value.domain || !routingForm.value.pattern || !routingForm.value.target) return
  try {
    await api.post('/mail/routing', {
      domain: routingForm.value.domain,
      pattern: routingForm.value.pattern,
      target: routingForm.value.target,
      priority: Number(routingForm.value.priority || 100),
    })
    routingForm.value.pattern = ''
    routingForm.value.target = ''
    await loadRouting()
  } catch (e) {
    error.value = apiErrorMessage(e, 'email_manager.messages.routing_create_failed')
  }
}

async function deleteRouting(rule) {
  try {
    await api.delete('/mail/routing', { data: { domain: rule.domain, id: rule.id } })
    await loadRouting()
  } catch (e) {
    error.value = apiErrorMessage(e, 'email_manager.messages.routing_delete_failed')
  }
}

async function loadDkim() {
  if (!dkimDomain.value) return
  try {
    const res = await api.get('/mail/dkim', { params: { domain: dkimDomain.value } })
    dkimRecord.value = res.data?.data || null
  } catch (e) {
    error.value = apiErrorMessage(e, 'email_manager.messages.dkim_failed')
  }
}

async function rotateDkim() {
  if (!dkimDomain.value) return
  try {
    const res = await api.post('/mail/dkim/rotate', { domain: dkimDomain.value })
    dkimRecord.value = res.data?.data || null
  } catch (e) {
    error.value = apiErrorMessage(e, 'email_manager.messages.dkim_rotate_failed')
  }
}

async function generateWebmailSso(address) {
  try {
    const res = await api.post('/mail/webmail/sso', { address, ttl_seconds: 300 })
    const url = res.data?.data?.url
    if (url) {
      window.open(url, '_blank', 'noopener,noreferrer')
    }
  } catch (e) {
    error.value = apiErrorMessage(e, 'email_manager.messages.webmail_sso_failed')
  }
}

async function loadMailAuth() {
  if (!premiumForm.value.domain) return
  try {
    const res = await api.get('/mail/auth', { params: { domain: premiumForm.value.domain } })
    mailAuthRecord.value = res.data?.data || null
    if (mailAuthRecord.value) {
      premiumForm.value.policy = mailAuthRecord.value.policy || premiumForm.value.policy
      premiumForm.value.rua = mailAuthRecord.value.rua || premiumForm.value.rua
      premiumForm.value.ruf = mailAuthRecord.value.ruf || premiumForm.value.ruf
    }
  } catch (e) {
    error.value = apiErrorMessage(e, 'email_manager.messages.auth_load_failed')
  }
}

async function bootstrapAuth() {
  if (!premiumForm.value.domain) return
  try {
    const res = await api.post('/mail/auth/bootstrap', {
      domain: premiumForm.value.domain,
      policy: premiumForm.value.policy,
      rua: premiumForm.value.rua,
      ruf: premiumForm.value.ruf,
    })
    mailAuthRecord.value = res.data?.data || null
    await loadDeliverability()
  } catch (e) {
    error.value = apiErrorMessage(e, 'email_manager.messages.auth_bootstrap_failed')
  }
}

async function loadDeliverability() {
  if (!premiumForm.value.domain) return
  try {
    const res = await api.get('/mail/deliverability', { params: { domain: premiumForm.value.domain } })
    deliverability.value = res.data?.data || {}
  } catch (e) {
    error.value = apiErrorMessage(e, 'email_manager.messages.deliverability_failed')
  }
}

async function loadWebmailOps() {
  try {
    const domain = premiumForm.value.domain || ''
    const statusRes = await api.get('/mail/webmail/ops/status', { params: { domain } })
    webmailStatus.value = statusRes.data?.data || {}
    const tokenRes = await api.get('/mail/webmail/ops/tokens', { params: { domain, limit: 50 } })
    webmailTokens.value = Array.isArray(tokenRes.data?.data) ? tokenRes.data.data : []
  } catch (e) {
    error.value = apiErrorMessage(e, 'email_manager.messages.webmail_ops_failed')
  }
}

async function cleanupExpiredTokens() {
  try {
    await api.post('/mail/webmail/ops/cleanup', {
      domain: premiumForm.value.domain || undefined,
    })
    await loadWebmailOps()
  } catch (e) {
    error.value = apiErrorMessage(e, 'email_manager.messages.webmail_ops_failed')
  }
}

async function revokeWebmailTokens() {
  if (!webmailOpsForm.value.token && !webmailOpsForm.value.address && !premiumForm.value.domain) {
    error.value = t('email_manager.messages.webmail_revoke_required')
    return
  }
  try {
    await api.post('/mail/webmail/ops/revoke', {
      token: webmailOpsForm.value.token || undefined,
      address: webmailOpsForm.value.address || undefined,
      domain: premiumForm.value.domain || undefined,
      revoke_expired: !webmailOpsForm.value.token && !webmailOpsForm.value.address,
    })
    webmailOpsForm.value.token = ''
    webmailOpsForm.value.address = ''
    await loadWebmailOps()
  } catch (e) {
    error.value = apiErrorMessage(e, 'email_manager.messages.webmail_ops_failed')
  }
}

async function loadPremiumData() {
  if (!premiumForm.value.domain && domains.value.length > 0) {
    premiumForm.value.domain = domains.value[0]
  }
  await Promise.all([loadMailAuth(), loadDeliverability(), loadWebmailOps()])
}

watch(
  () => route.query,
  (query) => {
    applyRouteQuery(query)
  },
  { immediate: true },
)

watch(domains, () => {
  applyDomainQuery(route.query.domain)
})

onMounted(async () => {
  await Promise.all([loadSites(), loadMailboxes(), loadForwards(), loadRouting()])
  if (domains.value.length > 0) {
    forwardForm.value.domain = domains.value[0]
    catchAllForm.value.domain = domains.value[0]
    routingForm.value.domain = domains.value[0]
    dkimDomain.value = domains.value[0]
    premiumForm.value.domain = domains.value[0]
    premiumForm.value.rua = `mailto:postmaster@${domains.value[0]}`
  }
  applyRouteQuery(route.query)
  if (tab.value === 'premium') {
    await loadPremiumData()
  }
})
</script>
