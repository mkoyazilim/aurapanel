<template>
  <div class="space-y-6 max-w-5xl">
    <div class="flex items-center justify-between gap-3">
      <div>
        <h1 class="text-2xl font-bold text-white">{{ t('mail_tuning.title') }}</h1>
        <p class="text-gray-400 mt-1">
          {{ t('mail_tuning.subtitle') }}
        </p>
      </div>
      <button class="btn-secondary" @click="loadConfig">{{ t('common.refresh') }}</button>
    </div>

    <div v-if="error" class="aura-card border-red-500/30 bg-red-500/5 text-red-400">{{ error }}</div>
    <div v-if="success" class="aura-card border-green-500/30 bg-green-500/5 text-green-300">{{ success }}</div>

    <div class="aura-card space-y-6">
      <div class="rounded-xl border border-brand-500/15 bg-brand-500/5 px-4 py-3 text-sm text-gray-300">
        {{ t('mail_tuning.notice') }}
      </div>

      <div class="grid grid-cols-1 md:grid-cols-3 gap-4">
        <div class="rounded-xl border border-panel-border bg-panel-dark/40 p-4">
          <label class="block text-sm text-gray-400 mb-2">{{ t('mail_tuning.message_size_limit') }}</label>
          <input v-model.trim="form.message_size_limit" type="text" class="aura-input w-full" :placeholder="t('mail_tuning.placeholders.message_size_limit')" />
          <p class="mt-2 text-xs text-gray-500">{{ t('mail_tuning.default_10mb') }}</p>
        </div>

        <div class="rounded-xl border border-panel-border bg-panel-dark/40 p-4">
          <label class="block text-sm text-gray-400 mb-2">{{ t('mail_tuning.mailbox_size_limit') }}</label>
          <input v-model.trim="form.mailbox_size_limit" type="text" class="aura-input w-full" :placeholder="t('mail_tuning.placeholders.mailbox_size_limit')" />
          <p class="mt-2 text-xs text-gray-500">{{ t('mail_tuning.default_50mb') }}</p>
        </div>

        <div class="rounded-xl border border-panel-border bg-panel-dark/40 p-4">
          <label class="block text-sm text-gray-400 mb-2">{{ t('mail_tuning.client_connection_limit') }}</label>
          <input
            v-model.trim="form.smtpd_client_connection_count_limit"
            type="text"
            class="aura-input w-full"
            :placeholder="t('mail_tuning.placeholders.client_connection_limit')"
          />
          <p class="mt-2 text-xs text-gray-500">{{ t('mail_tuning.client_connection_hint') }}</p>
        </div>
      </div>

      <div class="flex flex-wrap gap-3 justify-end">
        <button class="btn-secondary" @click="resetDefaults">{{ t('mail_tuning.reset_defaults') }}</button>
        <button class="btn-primary" :disabled="saving" @click="saveConfig">
          {{ saving ? t('mail_tuning.saving') : t('mail_tuning.save_apply') }}
        </button>
      </div>
    </div>
  </div>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import api from '../services/api'

const { t } = useI18n({ useScope: 'global' })
const error = ref('')
const success = ref('')
const saving = ref(false)

const defaultForm = () => ({
  message_size_limit: '10485760',
  mailbox_size_limit: '51200000',
  smtpd_client_connection_count_limit: '50',
})

const form = ref(defaultForm())

function apiErrorMessage(error, fallback) {
  return error?.response?.data?.message || error?.message || fallback
}

function resetDefaults() {
  form.value = defaultForm()
}

async function loadConfig() {
  error.value = ''
  success.value = ''
  try {
    const res = await api.get('/mail/tuning')
    form.value = { ...defaultForm(), ...(res.data?.data || {}) }
  } catch (err) {
    error.value = apiErrorMessage(err, t('mail_tuning.load_failed'))
  }
}

async function saveConfig() {
  error.value = ''
  success.value = ''
  saving.value = true
  try {
    const payload = {
      message_size_limit: String(form.value.message_size_limit || '').trim(),
      mailbox_size_limit: String(form.value.mailbox_size_limit || '').trim(),
      smtpd_client_connection_count_limit: String(form.value.smtpd_client_connection_count_limit || '').trim(),
    }
    await api.post('/mail/tuning', payload)
    success.value = t('mail_tuning.save_success')
  } catch (err) {
    error.value = apiErrorMessage(err, t('mail_tuning.save_failed'))
  } finally {
    saving.value = false
  }
}

onMounted(loadConfig)
</script>

