<template>
  <div class="space-y-6 max-w-5xl">
    <div class="flex items-center justify-between gap-3">
      <div>
        <h1 class="text-2xl font-bold text-white">Mail Tuning</h1>
        <p class="text-gray-400 mt-1">
          Postfix tarafindaki temel limitleri panelden guvenli sekilde yonetin.
        </p>
      </div>
      <button class="btn-secondary" @click="loadConfig">Yenile</button>
    </div>

    <div v-if="error" class="aura-card border-red-500/30 bg-red-500/5 text-red-400">{{ error }}</div>
    <div v-if="success" class="aura-card border-green-500/30 bg-green-500/5 text-green-300">{{ success }}</div>

    <div class="aura-card space-y-6">
      <div class="rounded-xl border border-brand-500/15 bg-brand-500/5 px-4 py-3 text-sm text-gray-300">
        Kayit sonrasinda Postfix yeniden baslatilir. Degerleri byte veya adet olarak girin.
      </div>

      <div class="grid grid-cols-1 md:grid-cols-3 gap-4">
        <div class="rounded-xl border border-panel-border bg-panel-dark/40 p-4">
          <label class="block text-sm text-gray-400 mb-2">Mesaj boyutu limiti</label>
          <input v-model.trim="form.message_size_limit" type="text" class="aura-input w-full" placeholder="10485760" />
          <p class="mt-2 text-xs text-gray-500">Varsayilan: 10 MB</p>
        </div>

        <div class="rounded-xl border border-panel-border bg-panel-dark/40 p-4">
          <label class="block text-sm text-gray-400 mb-2">Mailbox boyutu limiti</label>
          <input v-model.trim="form.mailbox_size_limit" type="text" class="aura-input w-full" placeholder="51200000" />
          <p class="mt-2 text-xs text-gray-500">Varsayilan: 50 MB</p>
        </div>

        <div class="rounded-xl border border-panel-border bg-panel-dark/40 p-4">
          <label class="block text-sm text-gray-400 mb-2">SMTP istemci baglanti limiti</label>
          <input
            v-model.trim="form.smtpd_client_connection_count_limit"
            type="text"
            class="aura-input w-full"
            placeholder="50"
          />
          <p class="mt-2 text-xs text-gray-500">Ayni istemci icin eszamanli baglanti sayisi</p>
        </div>
      </div>

      <div class="flex flex-wrap gap-3 justify-end">
        <button class="btn-secondary" @click="resetDefaults">Varsayilanlar</button>
        <button class="btn-primary" :disabled="saving" @click="saveConfig">
          {{ saving ? 'Kaydediliyor...' : 'Kaydet ve Uygula' }}
        </button>
      </div>
    </div>
  </div>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import api from '../services/api'

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
    error.value = apiErrorMessage(err, 'Mail tuning ayarlari yuklenemedi.')
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
    success.value = 'Mail tuning ayarlari kaydedildi ve Postfix yeniden baslatildi.'
  } catch (err) {
    error.value = apiErrorMessage(err, 'Mail tuning ayarlari kaydedilemedi.')
  } finally {
    saving.value = false
  }
}

onMounted(loadConfig)
</script>
