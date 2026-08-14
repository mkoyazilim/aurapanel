<template>
  <Layout>
    <div class="page">
      <h1>{{ $t('menu.backups') }}</h1>
      <div v-if="error" class="alert error">{{ error }}</div>
      <div v-if="notice" class="alert ok">{{ notice }}</div>

      <div class="card">
        <div class="row">
          <div style="flex: 1">
            <label>{{ $t('backups.site') }}</label>
            <select v-model="siteId" @change="load">
              <option v-for="s in sites" :key="s.id" :value="s.id">{{ s.name }}</option>
            </select>
          </div>
          <div style="flex: 1">
            <label>{{ $t('backups.kind') }}</label>
            <select v-model="kind">
              <option value="files">{{ $t('backups.kind_files') }}</option>
              <option value="full">{{ $t('backups.kind_full') }}</option>
              <option value="db">{{ $t('backups.kind_db') }}</option>
            </select>
          </div>
          <button class="btn primary" style="margin-top: 18px" :disabled="busy" @click="run">
            {{ busy ? $t('backups.taking') : '💾 ' + $t('backups.take_backup') }}
          </button>
        </div>
      </div>

      <div class="card">
        <h2>{{ $t('backups.history') }}</h2>
        <table>
          <thead><tr><th>{{ $t('backups.name') }}</th><th>{{ $t('backups.kind') }}</th><th>{{ $t('backups.status') }}</th><th>{{ $t('backups.date') }}</th></tr></thead>
          <tbody>
            <tr v-for="b in backups" :key="b.id">
              <td class="mono">{{ b.location }}</td>
              <td>{{ b.kind }}</td>
              <td><span class="badge" :class="b.status === 'success' ? 'ok' : 'err'">{{ b.status }}</span></td>
              <td class="muted">{{ b.created_at }}</td>
            </tr>
            <tr v-if="!backups.length"><td colspan="4" class="muted">{{ $t('backups.empty') }}</td></tr>
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
const kind = ref('files')
const backups = ref([])
const error = ref('')
const notice = ref('')
const busy = ref(false)

async function load() {
  if (!siteId.value) return
  try {
    backups.value = await api(`/sites/${siteId.value}/backups`)
  } catch (e) {
    error.value = e.message
  }
}

async function run() {
  if (!siteId.value) return
  busy.value = true
  error.value = ''
  notice.value = ''
  try {
    const out = await api(`/sites/${siteId.value}/backups/run`, {
      method: 'POST',
      body: { kind: kind.value },
    })
    notice.value = t('backups.success', { name: out.name })
    await load()
  } catch (e) {
    error.value = e.message
  } finally {
    busy.value = false
  }
}

onMounted(async () => {
  sites.value = await api('/sites').catch(() => [])
  if (sites.value.length) {
    siteId.value = sites.value[0].id
    await load()
  }
})
</script>
