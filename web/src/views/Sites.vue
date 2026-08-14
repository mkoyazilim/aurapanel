<template>
  <Layout>
    <div class="page">
      <h1>{{ $t('menu.sites') }}</h1>
      <div v-if="error" class="alert error">{{ error }}</div>

      <div class="card">
        <h2>{{ $t('sites.new_site') }}</h2>
        <div class="row">
          <div style="flex: 2">
            <label>{{ $t('sites.domain') }}</label>
            <input v-model="newSite.domain" placeholder="example.com" />
          </div>
          <div style="flex: 1">
            <label>{{ $t('sites.php_version') }}</label>
            <select v-model="newSite.php">
              <option>8.2</option><option selected>8.3</option><option>8.4</option>
            </select>
          </div>
          <button class="btn primary" @click="create" :disabled="busy">{{ $t('sites.create') }}</button>
        </div>
      </div>

      <div class="card">
        <h2>{{ $t('sites.existing_sites') }}</h2>
        <table>
          <thead><tr><th>ID</th><th>{{ $t('sites.domain') }}</th><th>{{ $t('sites.linux_user') }}</th><th>{{ $t('sites.status') }}</th><th></th></tr></thead>
          <tbody>
            <tr v-for="s in sites" :key="s.id">
              <td class="mono">{{ s.id }}</td>
              <td>{{ s.name }}</td>
              <td class="mono">{{ s.linux_user }}</td>
              <td><span class="badge" :class="s.status === 'active' ? 'ok' : 'warn'">{{ s.status }}</span></td>
              <td><button class="btn danger" @click="remove(s.id)">{{ $t('common.delete') }}</button></td>
            </tr>
            <tr v-if="!sites.length"><td colspan="5" class="muted">{{ $t('sites.no_sites') }}</td></tr>
          </tbody>
        </table>
      </div>
    </div>
  </Layout>
</template>

<script setup>
import { onMounted, reactive, ref } from 'vue'
import Layout from '../components/Layout.vue'
import { api } from '../api'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()

const sites = ref([])
const error = ref('')
const busy = ref(false)
const newSite = reactive({ domain: '', php: '8.3' })

async function load() {
  try {
    sites.value = await api('/sites')
  } catch (e) {
    error.value = e.message
  }
}

async function create() {
  busy.value = true
  error.value = ''
  try {
    await api('/sites', {
      method: 'POST',
      body: { domain: newSite.domain, php_version: newSite.php, aliases: [], limits: {} },
    })
    newSite.domain = ''
    await load()
  } catch (e) {
    error.value = e.message
  } finally {
    busy.value = false
  }
}

async function remove(id) {
  if (!confirm(t('sites.delete_confirm', { id }))) return
  try {
    await api(`/sites/${id}`, { method: 'DELETE' })
    await load()
  } catch (e) {
    error.value = e.message
  }
}

onMounted(load)
</script>
