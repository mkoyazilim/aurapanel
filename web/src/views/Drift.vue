<template>
  <Layout>
    <div class="page">
      <h1>{{ $t('menu.drift') }}</h1>
      <div v-if="error" class="alert error">{{ error }}</div>
      <div v-if="notice" class="alert ok">{{ notice }}</div>

      <div class="card">
        <div class="row">
          <button class="btn primary" :disabled="busy" @click="scan">🔍 {{ $t('drift.scan') }}</button>
          <label style="margin: 0 8px">
            <input type="checkbox" v-model="autoRepair" @change="setAuto" style="width: auto" />
            {{ $t('drift.auto_repair') }}
          </label>
        </div>
      </div>

      <div class="card">
        <h2>{{ $t('drift.open_drifts') }}</h2>
        <table>
          <thead><tr><th>{{ $t('drift.site') }}</th><th>{{ $t('drift.resource') }}</th><th>{{ $t('drift.expected') }}</th><th>{{ $t('drift.actual') }}</th><th>{{ $t('drift.severity') }}</th><th></th></tr></thead>
          <tbody>
            <tr v-for="e in events" :key="e.id">
              <td class="mono">{{ e.site_id }}</td>
              <td class="mono">{{ e.resource }}</td>
              <td class="muted mono" style="max-width: 180px; overflow: hidden">{{ e.expected }}</td>
              <td class="mono">{{ e.actual }}</td>
              <td><span class="badge" :class="e.severity === 'critical' ? 'err' : 'warn'">{{ e.severity }}</span></td>
              <td><button class="btn" @click="repair(e.site_id)">🔧 {{ $t('drift.repair') }}</button></td>
            </tr>
            <tr v-if="!events.length"><td colspan="6" class="muted">{{ $t('drift.no_drifts') }}</td></tr>
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

const events = ref([])
const autoRepair = ref(false)
const error = ref('')
const notice = ref('')
const busy = ref(false)

async function load() {
  try {
    events.value = await api('/drift')
  } catch (e) {
    error.value = e.message
  }
}

async function scan() {
  busy.value = true
  error.value = ''
  notice.value = ''
  try {
    const out = await api('/drift/scan', { method: 'POST', body: {} })
    notice.value = t('drift.found', { count: out.found })
    await load()
  } catch (e) {
    error.value = e.message
  } finally {
    busy.value = false
  }
}

async function repair(siteId) {
  try {
    await api('/drift/repair', { method: 'POST', body: { site_id: siteId } })
    notice.value = t('drift.repaired', { site: siteId })
    await load()
  } catch (e) {
    error.value = e.message
  }
}

async function setAuto() {
  try {
    await api('/drift/auto-repair', { method: 'PUT', body: { enabled: autoRepair.value } })
  } catch (e) {
    error.value = e.message
  }
}

onMounted(load)
</script>
