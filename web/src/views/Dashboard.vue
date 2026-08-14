<template>
  <Layout>
    <div class="page">
      <h1>Ana Sayfa</h1>
      <div v-if="error" class="alert error">{{ error }}</div>
      <div class="row" style="margin-bottom: 16px">
        <div class="card stat">
          <div class="stat-value">{{ status.site_count ?? '—' }}</div>
          <div class="muted">Site</div>
        </div>
        <div class="card stat">
          <div class="stat-value">{{ status.open_drifts ?? '—' }}</div>
          <div class="muted">Açık Drift</div>
        </div>
        <div class="card stat">
          <div class="stat-value">{{ status.db }}</div>
          <div class="muted">Veritabanı</div>
        </div>
      </div>
      <div class="card">
        <h2>Hızlı İşlemler</h2>
        <div class="row">
          <router-link class="btn" to="/sites">➕ Yeni Site</router-link>
          <router-link class="btn" to="/files">📁 Dosya Yöneticisi</router-link>
          <router-link class="btn" to="/drift">🧭 Drift Taraması</router-link>
        </div>
      </div>
    </div>
  </Layout>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import Layout from '../components/Layout.vue'
import { api } from '../api'

const status = ref({})
const error = ref('')

onMounted(async () => {
  try {
    status.value = await api('/status')
  } catch (e) {
    error.value = e.message
  }
})
</script>

<style scoped>
.stat { flex: 1; text-align: center; }
.stat-value { font-size: 28px; font-weight: 700; color: var(--primary); }
</style>
