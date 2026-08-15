<template>
  <Layout>
    <div class="page">
      <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 24px;">
        <h1 style="margin: 0">{{ $t('mail.mail_server') }}</h1>
        <button class="btn primary" @click="showAddModal = true">{{ $t('mail.add_email_account_btn') }}</button>
      </div>

      <div v-if="loading" class="muted">{{ $t('mail.loading') }}</div>
      <div v-else>
        <div v-if="error" class="alert error">{{ error }}</div>
        
        <div class="card">
          <table v-if="accounts.length > 0">
            <thead>
              <tr>
                <th>{{ $t('mail.email_address') }}</th>
                <th>{{ $t('mail.quota_mb') }}</th>
                <th style="text-align: right">{{ $t('mail.actions') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="acc in accounts" :key="acc.id">
                <td><strong>{{ acc.email }}</strong></td>
                <td class="mono">{{ acc.quota > 0 ? acc.quota : 'Limitsiz' }}</td>
                <td style="text-align: right">
                  <button @click="deleteAccount(acc.id, acc.email)" class="btn danger btn-sm">{{ $t('mail.delete') }}</button>
                </td>
              </tr>
            </tbody>
          </table>
          <div v-else class="muted">{{ $t('mail.no_accounts') }}</div>
        </div>
      </div>

      <div v-if="showAddModal" class="modal-backdrop">
        <div class="modal-card">
          <h2>{{ $t('mail.add_email_account_title') }}</h2>
          <form @submit.prevent="createAccount">
            <div style="display: flex; gap: 8px; margin-bottom: 16px;">
              <div style="flex: 1">
                <label>{{ $t('mail.local_part') }}</label>
                <input v-model="form.local_part" type="text" required :placeholder="$t('mail.placeholder_hello')" />
              </div>
              <div style="padding-top: 32px">@</div>
              <div style="flex: 1">
                <label>{{ $t('mail.domain') }}</label>
                <input v-model="form.domain" type="text" required :placeholder="$t('mail.placeholder_domain')" />
              </div>
            </div>
            <label>{{ $t('mail.password') }}</label>
            <input v-model="form.password" type="password" required :placeholder="$t('mail.placeholder_password')" />
            
            <label style="margin-top: 16px">{{ $t('mail.quota_mb') }} (0 = Limitsiz)</label>
            <input v-model.number="form.quota" type="number" min="0" />

            <div style="display: flex; justify-content: flex-end; gap: 8px; margin-top: 24px;">
              <button type="button" class="btn" @click="showAddModal = false">{{ $t('mail.cancel') }}</button>
              <button type="submit" class="btn primary" :disabled="submitting">
                {{ submitting ? $t('mail.adding') : $t('mail.add_account') }}
              </button>
            </div>
          </form>
        </div>
      </div>
    </div>
  </Layout>
</template>


<script setup>
import Layout from '../components/Layout.vue'
import { ref, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { api } from '../api'

const { t } = useI18n()
const route = useRoute()
const siteId = route.params.id

const accounts = ref([])
const loading = ref(true)
const error = ref('')
const showCreateModal = ref(false)
const submitting = ref(false)

const form = ref({
  domain: '',
  local_part: '',
  password: '',
  quota_mb: 512
})

const fetchAccounts = async () => {
  loading.value = true
  try {
    accounts.value = await api.get(`/sites/${siteId}/mail`)
  } catch (err) {
    error.value = err.message
  } finally {
    loading.value = false
  }
}

const createAccount = async () => {
  submitting.value = true
  try {
    await api.post(`/sites/${siteId}/mail`, form.value)
    showCreateModal.value = false
    form.value.local_part = ''
    form.value.password = ''
    await fetchAccounts()
  } catch (err) {
    alert(t('mail.error_msg', { message: err.message }))
  } finally {
    submitting.value = false
  }
}

const deleteAccount = async (email) => {
  if (!confirm(t('mail.confirm_delete', { email }))) return
  try {
    await api.delete(`/sites/${siteId}/mail/${email}`)
    await fetchAccounts()
  } catch (err) {
    alert(t('mail.delete_error', { message: err.message }))
  }
}

onMounted(() => {
  fetchAccounts()
})
</script>


<style scoped>
.modal-backdrop {
  position: fixed; inset: 0; background: rgba(15, 23, 42, 0.6);
  backdrop-filter: blur(4px); display: flex; align-items: center;
  justify-content: center; z-index: 1000;
}
.modal-card {
  background: var(--bg-card, #ffffff); border: 1px solid var(--border-color, #e2e8f0);
  border-radius: 12px; width: 100%; max-width: 520px; padding: 24px;
  box-shadow: 0 20px 25px -5px rgba(0, 0, 0, 0.2);
}
</style>
