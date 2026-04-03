<template>
  <div class="space-y-6">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <div>
        <h1 class="text-2xl font-bold text-white">{{ t('ai_tools.title') }}</h1>
        <p class="mt-1 text-gray-400">
          {{ t('ai_tools.subtitle') }}
        </p>
      </div>
      <button class="btn-secondary" :disabled="loading" @click="loadAll">
        {{ loading ? t('ai_tools.actions.loading') : t('ai_tools.actions.refresh') }}
      </button>
    </div>

    <div v-if="notice" class="rounded-lg border px-3 py-2 text-sm" :class="noticeClass">
      {{ notice }}
    </div>

    <div class="grid grid-cols-1 gap-4 md:grid-cols-3">
      <div class="aura-card">
        <p class="text-xs uppercase tracking-[0.12em] text-gray-500">{{ t('ai_tools.overview.active_provider') }}</p>
        <p class="mt-2 text-lg font-semibold text-white">{{ provider.active_provider || '-' }}</p>
      </div>
      <div class="aura-card">
        <p class="text-xs uppercase tracking-[0.12em] text-gray-500">{{ t('ai_tools.overview.policy_status') }}</p>
        <p class="mt-2 text-lg font-semibold" :class="policy.enabled ? 'text-emerald-300' : 'text-amber-200'">
          {{ policy.enabled ? t('ai_tools.overview.enabled') : t('ai_tools.overview.disabled') }}
        </p>
      </div>
      <div class="aura-card">
        <p class="text-xs uppercase tracking-[0.12em] text-gray-500">{{ t('ai_tools.overview.execution_history') }}</p>
        <p class="mt-2 text-lg font-semibold text-white">{{ history.length }}</p>
      </div>
    </div>

    <div class="grid grid-cols-1 gap-4 xl:grid-cols-2">
      <div class="aura-card space-y-4">
        <div class="flex items-center justify-between">
          <h2 class="text-lg font-semibold text-white">{{ t('ai_tools.provider.title') }}</h2>
          <button class="btn-primary px-3 py-1.5 text-sm" :disabled="savingProvider" @click="saveProvider">
            {{ savingProvider ? t('ai_tools.actions.saving') : t('ai_tools.actions.save_provider') }}
          </button>
        </div>

        <label class="space-y-1 text-sm text-gray-300">
          <span>{{ t('ai_tools.provider.active_provider') }}</span>
          <select v-model="provider.active_provider" class="aura-input w-full">
            <option value="deepseek">deepseek</option>
            <option value="gemini">gemini</option>
          </select>
        </label>

        <div class="rounded-lg border border-panel-border p-3 space-y-2">
          <p class="text-sm font-semibold text-white">{{ t('ai_tools.provider.deepseek') }}</p>
          <label class="inline-flex items-center gap-2 text-sm text-gray-300">
            <input v-model="provider.deepseek.enabled" type="checkbox" class="h-4 w-4 accent-brand-500" />
            <span>{{ t('ai_tools.provider.enabled') }}</span>
          </label>
          <input v-model.trim="provider.deepseek.model" class="aura-input w-full" :placeholder="t('ai_tools.provider.model_placeholder')" />
          <input v-model.trim="provider.deepseek.base_url" class="aura-input w-full" :placeholder="t('ai_tools.provider.base_url_placeholder')" />
          <input
            v-model.trim="providerSecrets.deepseek_api_key"
            type="password"
            class="aura-input w-full"
            :placeholder="t('ai_tools.provider.new_api_key_placeholder')"
          />
          <label class="inline-flex items-center gap-2 text-xs text-gray-400">
            <input v-model="providerSecrets.clear_deepseek_key" type="checkbox" class="h-3.5 w-3.5 accent-brand-500" />
            <span>{{ t('ai_tools.provider.clear_key') }}</span>
          </label>
          <p class="text-xs text-gray-500">{{ t('ai_tools.provider.stored_key') }}: {{ provider.deepseek.masked_api_key || '-' }}</p>
        </div>

        <div class="rounded-lg border border-panel-border p-3 space-y-2">
          <p class="text-sm font-semibold text-white">{{ t('ai_tools.provider.gemini') }}</p>
          <label class="inline-flex items-center gap-2 text-sm text-gray-300">
            <input v-model="provider.gemini.enabled" type="checkbox" class="h-4 w-4 accent-brand-500" />
            <span>{{ t('ai_tools.provider.enabled') }}</span>
          </label>
          <input v-model.trim="provider.gemini.model" class="aura-input w-full" :placeholder="t('ai_tools.provider.model_placeholder')" />
          <input v-model.trim="provider.gemini.base_url" class="aura-input w-full" :placeholder="t('ai_tools.provider.base_url_placeholder')" />
          <input
            v-model.trim="providerSecrets.gemini_api_key"
            type="password"
            class="aura-input w-full"
            :placeholder="t('ai_tools.provider.new_api_key_placeholder')"
          />
          <label class="inline-flex items-center gap-2 text-xs text-gray-400">
            <input v-model="providerSecrets.clear_gemini_key" type="checkbox" class="h-3.5 w-3.5 accent-brand-500" />
            <span>{{ t('ai_tools.provider.clear_key') }}</span>
          </label>
          <p class="text-xs text-gray-500">{{ t('ai_tools.provider.stored_key') }}: {{ provider.gemini.masked_api_key || '-' }}</p>
        </div>
      </div>

      <div class="aura-card space-y-4">
        <div class="flex items-center justify-between">
          <h2 class="text-lg font-semibold text-white">{{ t('ai_tools.policy.title') }}</h2>
          <button class="btn-primary px-3 py-1.5 text-sm" :disabled="savingPolicy" @click="savePolicy">
            {{ savingPolicy ? t('ai_tools.actions.saving') : t('ai_tools.actions.save_policy') }}
          </button>
        </div>

        <div class="grid grid-cols-1 gap-3 sm:grid-cols-2">
          <label class="inline-flex items-center gap-2 text-sm text-gray-300">
            <input v-model="policy.enabled" type="checkbox" class="h-4 w-4 accent-brand-500" />
            <span>{{ t('ai_tools.policy.ai_enabled') }}</span>
          </label>
          <label class="inline-flex items-center gap-2 text-sm text-gray-300">
            <input v-model="policy.allow_shell" type="checkbox" class="h-4 w-4 accent-brand-500" />
            <span>{{ t('ai_tools.policy.allow_shell') }}</span>
          </label>
          <label class="inline-flex items-center gap-2 text-sm text-gray-300">
            <input v-model="policy.allow_privileged_shell" type="checkbox" class="h-4 w-4 accent-brand-500" />
            <span>{{ t('ai_tools.policy.allow_privileged_shell') }}</span>
          </label>
          <label class="inline-flex items-center gap-2 text-sm text-gray-300">
            <input v-model="policy.allow_service_control" type="checkbox" class="h-4 w-4 accent-brand-500" />
            <span>{{ t('ai_tools.policy.allow_service_control') }}</span>
          </label>
          <label class="inline-flex items-center gap-2 text-sm text-gray-300">
            <input v-model="policy.allow_malware_scan" type="checkbox" class="h-4 w-4 accent-brand-500" />
            <span>{{ t('ai_tools.policy.allow_malware_scan') }}</span>
          </label>
          <label class="inline-flex items-center gap-2 text-sm text-gray-300">
            <input v-model="policy.require_confirm_token" type="checkbox" class="h-4 w-4 accent-brand-500" />
            <span>{{ t('ai_tools.policy.require_confirm_token') }}</span>
          </label>
        </div>

        <input v-model.trim="policy.confirm_token" class="aura-input w-full" :placeholder="t('ai_tools.policy.confirm_token_placeholder')" />
        <div class="grid grid-cols-1 gap-3 sm:grid-cols-3">
          <input v-model.number="policy.max_command_timeout_seconds" type="number" min="2" max="120" class="aura-input" :placeholder="t('ai_tools.policy.max_timeout_placeholder')" />
          <input v-model.number="policy.max_output_chars" type="number" min="512" max="20000" class="aura-input" :placeholder="t('ai_tools.policy.max_output_placeholder')" />
          <input v-model.trim="policy.default_cwd" class="aura-input" :placeholder="t('ai_tools.policy.default_cwd_placeholder')" />
        </div>
        <textarea v-model="allowedPrefixesText" class="aura-input min-h-28 w-full font-mono text-xs" :placeholder="t('ai_tools.policy.allowed_prefixes_placeholder')" />
      </div>
    </div>

    <div class="aura-card space-y-4">
      <div class="flex flex-wrap items-center justify-between gap-3">
        <h2 class="text-lg font-semibold text-white">{{ t('ai_tools.planner.title') }}</h2>
        <div class="flex items-center gap-3">
          <label class="inline-flex items-center gap-2 text-sm text-gray-300">
            <input v-model="autoAskAndRun" type="checkbox" class="h-4 w-4 accent-brand-500" />
            <span>{{ t('ai_tools.planner.auto_ask_and_run') }}</span>
          </label>
          <label class="inline-flex items-center gap-2 text-sm text-gray-300">
            <input v-model="executeDryRun" type="checkbox" class="h-4 w-4 accent-brand-500" />
            <span>{{ t('ai_tools.planner.dry_run') }}</span>
          </label>
          <input v-model.trim="confirmTokenInput" class="aura-input w-52" :placeholder="t('ai_tools.planner.confirm_token_placeholder')" />
          <button class="btn-primary px-3 py-1.5 text-sm" :disabled="planning" @click="generatePlan">
            {{ planning ? t('ai_tools.actions.planning') : t('ai_tools.actions.generate_plan') }}
          </button>
          <button class="btn-secondary px-3 py-1.5 text-sm" :disabled="executing || !plan.id" @click="executePlanAll">
            {{ executing ? t('ai_tools.actions.running') : t('ai_tools.actions.execute_plan_all') }}
          </button>
        </div>
      </div>

      <textarea v-model.trim="prompt" class="aura-input min-h-32 w-full" :placeholder="t('ai_tools.planner.prompt_placeholder')" />

      <div v-if="plan.id" class="rounded-lg border border-panel-border p-3 space-y-3">
        <p class="text-sm text-gray-300">
          <span class="font-semibold text-white">{{ t('ai_tools.planner.plan_label') }}:</span> {{ plan.summary }}
          <span class="text-gray-500"> - {{ plan.provider }}/{{ plan.model }}</span>
        </p>
        <div class="space-y-2">
          <div v-for="step in plan.steps" :key="step.id" class="rounded-lg border border-panel-border/80 bg-panel-dark p-3">
            <div class="flex flex-wrap items-center justify-between gap-2">
              <p class="text-sm font-semibold text-white">
                {{ step.tool }} <span class="text-xs text-gray-400">({{ step.risk }})</span>
              </p>
              <button class="btn-primary px-2 py-1 text-xs" :disabled="executingStepId === step.id" @click="executePlanStep(step)">
                {{ executingStepId === step.id ? t('ai_tools.actions.running') : t('ai_tools.actions.execute_step') }}
              </button>
            </div>
            <p class="mt-1 text-xs text-gray-400">{{ step.reason }}</p>
            <pre class="mt-2 overflow-auto rounded bg-black/30 p-2 text-xs text-gray-300">{{ formatJSON(step.args) }}</pre>
          </div>
        </div>
      </div>
    </div>

    <div class="grid grid-cols-1 gap-4 xl:grid-cols-2">
      <div class="aura-card space-y-4">
        <h2 class="text-lg font-semibold text-white">{{ t('ai_tools.quick_execute.title') }}</h2>
        <div class="space-y-2">
          <button
            v-for="item in catalog"
            :key="item.id"
            class="btn-secondary w-full justify-start text-left"
            :disabled="executing || !item.enabled"
            @click="runCatalogTool(item)"
          >
            {{ item.id }} - {{ item.enabled ? t('ai_tools.quick_execute.run') : t('ai_tools.quick_execute.blocked') }}
          </button>
        </div>

        <div class="rounded-lg border border-panel-border p-3 space-y-2">
          <p class="text-sm font-semibold text-white">{{ t('ai_tools.quick_execute.shell_runner') }}</p>
          <input v-model.trim="shellForm.command" class="aura-input w-full font-mono text-xs" :placeholder="t('ai_tools.quick_execute.command_placeholder')" />
          <div class="grid grid-cols-1 gap-2 sm:grid-cols-3">
            <input v-model.trim="shellForm.cwd" class="aura-input" :placeholder="t('ai_tools.quick_execute.cwd_placeholder')" />
            <input v-model.number="shellForm.timeout_seconds" type="number" min="2" max="120" class="aura-input" :placeholder="t('ai_tools.quick_execute.timeout_placeholder')" />
            <label class="inline-flex items-center gap-2 text-sm text-gray-300">
              <input v-model="shellForm.privileged" type="checkbox" class="h-4 w-4 accent-brand-500" />
              <span>{{ t('ai_tools.quick_execute.privileged') }}</span>
            </label>
          </div>
          <button class="btn-primary w-full" :disabled="executing" @click="runShell">
            {{ t('ai_tools.actions.run_shell') }}
          </button>
        </div>
      </div>

      <div class="aura-card space-y-4">
        <h2 class="text-lg font-semibold text-white">{{ t('ai_tools.result.title') }}</h2>
        <pre class="max-h-96 overflow-auto rounded bg-panel-dark p-3 text-xs text-gray-300">{{ formatJSON(lastResult) }}</pre>
      </div>
    </div>

    <div class="aura-card space-y-3">
      <h2 class="text-lg font-semibold text-white">{{ t('ai_tools.history.title') }}</h2>
      <div class="overflow-auto rounded-lg border border-panel-border">
        <table class="min-w-full text-left text-xs">
          <thead class="bg-panel-dark text-gray-400">
            <tr>
              <th class="px-3 py-2">{{ t('ai_tools.history.headers.tool') }}</th>
              <th class="px-3 py-2">{{ t('ai_tools.history.headers.status') }}</th>
              <th class="px-3 py-2">{{ t('ai_tools.history.headers.risk') }}</th>
              <th class="px-3 py-2">{{ t('ai_tools.history.headers.dry') }}</th>
              <th class="px-3 py-2">{{ t('ai_tools.history.headers.by') }}</th>
              <th class="px-3 py-2">{{ t('ai_tools.history.headers.time') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="entry in history" :key="entry.id" class="border-t border-panel-border/70 text-gray-200">
              <td class="px-3 py-2">{{ entry.tool }}</td>
              <td class="px-3 py-2">{{ entry.status }}</td>
              <td class="px-3 py-2">{{ entry.risk }}</td>
              <td class="px-3 py-2">{{ entry.dry_run ? t('ai_tools.history.yes') : t('ai_tools.history.no') }}</td>
              <td class="px-3 py-2">{{ entry.requested_by || '-' }}</td>
              <td class="px-3 py-2">{{ formatTimestamp(entry.requested_at) }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import api from '../services/api'

const { t } = useI18n({ useScope: 'global' })

const AI_TOOLS_AUTO_ASK_AND_RUN_KEY = 'aurapanel:ai-tools:auto-ask-and-run'
const AI_TOOLS_DRY_RUN_KEY = 'aurapanel:ai-tools:dry-run'

const loading = ref(false)
const planning = ref(false)
const savingProvider = ref(false)
const savingPolicy = ref(false)
const executing = ref(false)
const executingStepId = ref('')
const notice = ref('')
const noticeType = ref('info')

const provider = ref({
  active_provider: 'deepseek',
  deepseek: { enabled: true, model: 'deepseek-chat', base_url: 'https://api.deepseek.com/v1', masked_api_key: '' },
  gemini: { enabled: false, model: 'gemini-2.5-flash', base_url: 'https://generativelanguage.googleapis.com/v1beta', masked_api_key: '' },
})
const providerSecrets = ref({
  deepseek_api_key: '',
  gemini_api_key: '',
  clear_deepseek_key: false,
  clear_gemini_key: false,
})
const policy = ref({
  enabled: true,
  allow_shell: true,
  allow_privileged_shell: false,
  allow_service_control: true,
  allow_malware_scan: true,
  require_confirm_token: true,
  confirm_token: 'APPLY_AI_TOOLS',
  max_command_timeout_seconds: 20,
  max_output_chars: 4000,
  default_cwd: '/home',
})
const allowedPrefixesText = ref('')
const catalog = ref([])
const history = ref([])
const plan = ref({ id: '', summary: '', provider: '', model: '', steps: [] })
const prompt = ref(t('ai_tools.planner.default_prompt'))
const executeDryRun = ref(true)
const autoAskAndRun = ref(true)
const confirmTokenInput = ref('')
const lastResult = ref({})
const shellForm = ref({
  command: 'uptime && df -h && free -m',
  cwd: '/home',
  timeout_seconds: 12,
  privileged: false,
})

const noticeClass = computed(() => {
  if (noticeType.value === 'error') return 'border-red-500/30 bg-red-500/10 text-red-300'
  if (noticeType.value === 'warning') return 'border-amber-500/30 bg-amber-500/10 text-amber-200'
  return 'border-emerald-500/30 bg-emerald-500/10 text-emerald-300'
})

function setNotice(message, type = 'info') {
  notice.value = String(message || '').trim()
  noticeType.value = type
}

function formatJSON(value) {
  try {
    return JSON.stringify(value ?? {}, null, 2)
  } catch {
    return '{}'
  }
}

function formatTimestamp(unixSeconds) {
  const value = Number(unixSeconds || 0)
  if (!Number.isFinite(value) || value <= 0) return '-'
  return new Date(value * 1000).toLocaleString()
}

function parsePrefixesText() {
  return String(allowedPrefixesText.value || '')
    .split(/\r?\n|,/)
    .map(item => item.trim())
    .filter(Boolean)
}

function parseStoredBool(value, fallback) {
  const normalized = String(value || '').trim().toLowerCase()
  if (normalized === '1' || normalized === 'true' || normalized === 'yes' || normalized === 'on') return true
  if (normalized === '0' || normalized === 'false' || normalized === 'no' || normalized === 'off') return false
  return fallback
}

function loadUiPreferences() {
  if (typeof window === 'undefined') return
  const autoAskRaw = window.localStorage.getItem(AI_TOOLS_AUTO_ASK_AND_RUN_KEY)
  const dryRunRaw = window.localStorage.getItem(AI_TOOLS_DRY_RUN_KEY)
  autoAskAndRun.value = parseStoredBool(autoAskRaw, true)
  executeDryRun.value = parseStoredBool(dryRunRaw, true)
}

async function loadAll() {
  loading.value = true
  try {
    const [statusRes, catalogRes, historyRes] = await Promise.all([
      api.get('/ai/tools/status'),
      api.get('/ai/tools/catalog'),
      api.get('/ai/tools/history?limit=100'),
    ])
    const statusData = statusRes.data?.data || {}
    provider.value = statusData.provider || provider.value
    policy.value = statusData.policy || policy.value
    allowedPrefixesText.value = Array.isArray(policy.value.allowed_command_prefixes)
      ? policy.value.allowed_command_prefixes.join('\n')
      : ''
    catalog.value = Array.isArray(catalogRes.data?.data?.tools) ? catalogRes.data.data.tools : []
    history.value = Array.isArray(historyRes.data?.data) ? historyRes.data.data : []
    shellForm.value.cwd = policy.value.default_cwd || '/home'
  } catch (err) {
    setNotice(err?.response?.data?.message || err?.message || t('ai_tools.messages.load_failed'), 'error')
  } finally {
    loading.value = false
  }
}

async function saveProvider() {
  savingProvider.value = true
  try {
    const payload = {
      active_provider: provider.value.active_provider,
      deepseek: {
        enabled: !!provider.value.deepseek?.enabled,
        model: provider.value.deepseek?.model || '',
        base_url: provider.value.deepseek?.base_url || '',
        api_key: providerSecrets.value.deepseek_api_key || '',
        clear_api_key: !!providerSecrets.value.clear_deepseek_key,
      },
      gemini: {
        enabled: !!provider.value.gemini?.enabled,
        model: provider.value.gemini?.model || '',
        base_url: provider.value.gemini?.base_url || '',
        api_key: providerSecrets.value.gemini_api_key || '',
        clear_api_key: !!providerSecrets.value.clear_gemini_key,
      },
    }
    await api.post('/ai/tools/provider', payload)
    providerSecrets.value.deepseek_api_key = ''
    providerSecrets.value.gemini_api_key = ''
    providerSecrets.value.clear_deepseek_key = false
    providerSecrets.value.clear_gemini_key = false
    await loadAll()
    setNotice(t('ai_tools.messages.provider_saved'), 'success')
  } catch (err) {
    setNotice(err?.response?.data?.message || err?.message || t('ai_tools.messages.provider_save_failed'), 'error')
  } finally {
    savingProvider.value = false
  }
}

async function savePolicy() {
  savingPolicy.value = true
  try {
    const payload = {
      enabled: !!policy.value.enabled,
      allow_shell: !!policy.value.allow_shell,
      allow_privileged_shell: !!policy.value.allow_privileged_shell,
      allow_service_control: !!policy.value.allow_service_control,
      allow_malware_scan: !!policy.value.allow_malware_scan,
      require_confirm_token: !!policy.value.require_confirm_token,
      confirm_token: policy.value.confirm_token || '',
      max_command_timeout_seconds: Number(policy.value.max_command_timeout_seconds || 20),
      max_output_chars: Number(policy.value.max_output_chars || 4000),
      default_cwd: policy.value.default_cwd || '/home',
      allowed_command_prefixes: parsePrefixesText(),
    }
    await api.post('/ai/tools/policy', payload)
    await loadAll()
    setNotice(t('ai_tools.messages.policy_saved'), 'success')
  } catch (err) {
    setNotice(err?.response?.data?.message || err?.message || t('ai_tools.messages.policy_save_failed'), 'error')
  } finally {
    savingPolicy.value = false
  }
}

async function generatePlan() {
  planning.value = true
  try {
    const res = await api.post('/ai/tools/plan', { prompt: prompt.value })
    const data = res.data?.data || {}
    plan.value = data.plan || { id: '', summary: '', provider: '', model: '', steps: [] }
    if (data.fallback_reason) {
      setNotice(t('ai_tools.messages.fallback_plan', { reason: data.fallback_reason }), 'warning')
    } else {
      setNotice(t('ai_tools.messages.plan_created'), 'success')
    }

    await maybeAutoExecuteAfterApproval()
  } catch (err) {
    setNotice(err?.response?.data?.message || err?.message || t('ai_tools.messages.plan_create_failed'), 'error')
  } finally {
    planning.value = false
  }
}

async function maybeAutoExecuteAfterApproval() {
  if (!autoAskAndRun.value) return
  if (!plan.value?.id || !Array.isArray(plan.value?.steps) || plan.value.steps.length === 0) return
  if (typeof window === 'undefined') return

  const summaryRows = plan.value.steps
    .map((step, index) => `${index + 1}. ${step.tool} (${step.risk || t('ai_tools.planner.risk_fallback')})`)
    .join('\n')

  const approved = window.confirm(
    t('ai_tools.dialogs.auto_execute_confirm', {
      summary: summaryRows,
      dryRun: executeDryRun.value ? t('ai_tools.history.yes') : t('ai_tools.history.no'),
    }),
  )
  if (!approved) {
    setNotice(t('ai_tools.messages.awaiting_manual_approval'), 'warning')
    return
  }

  if (
    !executeDryRun.value &&
    policy.value?.require_confirm_token &&
    !String(confirmTokenInput.value || '').trim()
  ) {
    const token = window.prompt(
      t('ai_tools.dialogs.confirm_token_prompt', {
        expected: policy.value?.confirm_token || t('ai_tools.dialogs.hidden_token'),
      }),
      '',
    )
    if (token === null || !String(token).trim()) {
      setNotice(t('ai_tools.messages.auto_execute_cancelled_no_token'), 'warning')
      return
    }
    confirmTokenInput.value = String(token).trim()
  }

  await executePlanAll()
}

async function executePlanStep(step) {
  if (!plan.value?.id || !step?.id) return
  executingStepId.value = step.id
  try {
    const res = await api.post('/ai/tools/execute', {
      plan_id: plan.value.id,
      step_id: step.id,
      dry_run: !!executeDryRun.value,
      confirm_token: confirmTokenInput.value || '',
    })
    lastResult.value = res.data?.data || {}
    await loadAll()
    setNotice(t('ai_tools.messages.step_executed'), 'success')
  } catch (err) {
    setNotice(err?.response?.data?.message || err?.message || t('ai_tools.messages.step_execute_failed'), 'error')
  } finally {
    executingStepId.value = ''
  }
}

async function executePlanAll() {
  if (!plan.value?.id) return
  executing.value = true
  try {
    const res = await api.post('/ai/tools/execute', {
      plan_id: plan.value.id,
      execute_all: true,
      dry_run: !!executeDryRun.value,
      confirm_token: confirmTokenInput.value || '',
    })
    lastResult.value = res.data?.data || {}
    await loadAll()
    setNotice(t('ai_tools.messages.plan_executed_all'), 'success')
  } catch (err) {
    setNotice(err?.response?.data?.message || err?.message || t('ai_tools.messages.plan_execute_all_failed'), 'error')
  } finally {
    executing.value = false
  }
}

async function runCatalogTool(item) {
  if (!item?.id) return
  executing.value = true
  try {
    const args = item.id === 'shell_command'
      ? { ...shellForm.value }
      : { ...(item.default_args || {}) }
    const res = await api.post('/ai/tools/execute', {
      tool: item.id,
      args,
      dry_run: !!executeDryRun.value,
      confirm_token: confirmTokenInput.value || '',
      prompt: t('ai_tools.quick_execute.backend_prompt'),
    })
    lastResult.value = res.data?.data || {}
    await loadAll()
    setNotice(t('ai_tools.messages.tool_executed', { tool: item.id }), 'success')
  } catch (err) {
    setNotice(err?.response?.data?.message || err?.message || t('ai_tools.messages.quick_execute_failed'), 'error')
  } finally {
    executing.value = false
  }
}

async function runShell() {
  await runCatalogTool({ id: 'shell_command' })
}

watch(autoAskAndRun, (value) => {
  if (typeof window === 'undefined') return
  window.localStorage.setItem(AI_TOOLS_AUTO_ASK_AND_RUN_KEY, value ? '1' : '0')
})

watch(executeDryRun, (value) => {
  if (typeof window === 'undefined') return
  window.localStorage.setItem(AI_TOOLS_DRY_RUN_KEY, value ? '1' : '0')
})

onMounted(() => {
  loadUiPreferences()
  loadAll()
})
</script>
