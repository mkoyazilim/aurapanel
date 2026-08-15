
<template>
  <Layout>
    <div class="page">
      <h1>{{ $t('menu.sftp') }}</h1>
      <div v-if="error" class="alert error">{{ error }}</div>
      <div v-if="notice" class="alert ok">{{ notice }}</div>
      <div v-if="passwordNotice" class="alert warning" style="background:#fffbeb; color:#92400e; border:1px solid #fcd34d;">
        <strong>{{ $t('sftp.password_warning') }}</strong> {{ $t('sftp.password_desc') }} <code style="font-size: 1.1em; padding: 4px; background:#fef3c7;">{{ passwordNotice }}</code>
      </div>

      <div class="card">
        <div class="row">
          <div style="flex: 1">
            <label>{{ $t('sftp.select_site') }}</label>
            <select v-model="siteId" @change="loadAccounts">
              <option value="" disabled>{{ $t('sftp.please_select') }}</option>
              <option v-for="s in sites" :key="s.id" :value="s.id">{{ s.name }}</option>
            </select>
          </div>
          <div style="flex: 1">
            <label>{{ $t('sftp.new_account') }}</label>
            <input v-model="newUser" :placeholder="$t('sftp.new_account_ph')" />
          </div>
          <button class="btn primary" style="margin-top: 18px" :disabled="!siteId || !newUser" @click="createAccount">➕ {{ $t('sftp.create') }}</button>
        </div>
      </div>

      <div class="card" v-if="siteId">
        <h2>{{ $t('sftp.existing', { id: siteId }) }}</h2>
        <table>
          <thead><tr><th>{{ $t('sftp.account_name') }}</th><th>{{ $t('sftp.port') }}</th><th></th></tr></thead>
          <tbody>
            <tr v-for="a in accounts" :key="a.username">
              <td class="mono">{{ a.username }}</td>
              <td>{{ $t('sftp.port_desc') }}</td>
              <td style="text-align: right;">
                <button class="btn danger" @click="deleteAccount(a.username)">{{ $t('common.delete') }}</button>
              </td>
            </tr>
            <tr v-if="!accounts.length"><td colspan="3" class="muted">{{ $t('sftp.no_accounts') }}</td></tr>
          </tbody>
        </table>
      </div>
    </div>
  </Layout>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import Layout from '../components/Layout.vue'
import { api } from '../api'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()

const sites = ref([])
const siteId = ref('')
const accounts = ref([])
const newUser = ref('')
const error = ref('')
const notice = ref('')
const passwordNotice = ref('')

onMounted(async () => {
  try {
    const raw = await api('/sites')
    sites.value = raw || []
  } catch (e) {
    error.value = e.message
  }
})

async function loadAccounts() {
  if (!siteId.value) return
  error.value = ''
  notice.value = ''
  passwordNotice.value = ''
  try {
    accounts.value = await api(`/sites/${siteId.value}/sftp`) || []
  } catch (e) {
    error.value = e.message
  }
}

async function createAccount() {
  error.value = ''
  notice.value = ''
  passwordNotice.value = ''
  if (!newUser.value) return
  try {
    const res = await api(`/sites/${siteId.value}/sftp`, {
      method: 'POST',
      body: { username: newUser.value, password: '' }
    })
    passwordNotice.value = res.password
    notice.value = t('sftp.created', { name: newUser.value })
    newUser.value = ''
    await loadAccounts()
  } catch (e) {
    error.value = e.message
  }
}

async function deleteAccount(username) {
  if (!confirm(t('sftp.delete_confirm', { name: username }))) return
  error.value = ''
  notice.value = ''
  passwordNotice.value = ''
  try {
    await api(`/sites/${siteId.value}/sftp/${username}`, { method: 'DELETE' })
    await loadAccounts()
  } catch (e) {
    error.value = e.message
  }
}
</script>
