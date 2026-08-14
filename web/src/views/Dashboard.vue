<template>
  <Layout>
    <div class="page">
      <div class="dashboard-header">
        <h1>{{ $t('menu.home') }}</h1>
        <button class="btn btn-sm" @click="loadData">🔄 {{ $t('common.refresh') }}</button>
      </div>

      <div v-if="error" class="alert error">{{ error }}</div>

      <!-- Health Status Banner -->
      <div v-if="health" class="health-banner card" :class="health.overall ? 'health-ok' : 'health-warn'">
        <div class="health-summary">
          <span class="health-indicator" :class="health.overall ? 'status-green' : 'status-yellow'"></span>
          <div>
            <strong>{{ health.overall ? 'Tüm Sistemler Sağlıklı ve Çalışıyor' : 'Sistemde Dikkat Gerektiren Durumlar Var' }}</strong>
            <p class="muted text-sm" style="margin: 2px 0 0 0">
              Son Kontrol: {{ health.checked_at }}
            </p>
          </div>
        </div>
        <div class="health-checks-grid">
          <div v-for="c in health.checks" :key="c.name" class="check-pill" :class="c.ok ? 'pill-ok' : 'pill-warn'">
            <span class="dot"></span>
            <span class="check-name">{{ formatCheckName(c.name) }}</span>
            <span v-if="!c.ok" class="check-msg">({{ c.message }})</span>
          </div>
        </div>
      </div>

      <!-- Stat Cards -->
      <div class="row stats-row" style="margin-bottom: 20px">
        <div class="card stat">
          <div class="stat-value">{{ status.site_count ?? '—' }}</div>
          <div class="muted">{{ $t('dashboard.site') }}</div>
        </div>
        <div class="card stat">
          <div class="stat-value" :style="{ color: (status.open_drifts > 0) ? '#ef4444' : '#10b981' }">
            {{ status.open_drifts ?? '0' }}
          </div>
          <div class="muted">{{ $t('dashboard.open_drifts') }}</div>
        </div>
        <div class="card stat">
          <div class="stat-value" style="color: #10b981">Active</div>
          <div class="muted">OpenLiteSpeed</div>
        </div>
        <div class="card stat">
          <div class="stat-value" style="color: #10b981">Ready</div>
          <div class="muted">MariaDB</div>
        </div>
      </div>

      <!-- Quick Actions -->
      <div class="card quick-actions-card">
        <h2>{{ $t('dashboard.quick_actions') }}</h2>
        <div class="actions-grid">
          <router-link class="action-btn" to="/sites">
            <span class="action-icon">➕</span>
            <div>
              <strong>{{ $t('dashboard.new_site') }}</strong>
              <p class="muted text-sm">Site ve domain yönetimi</p>
            </div>
          </router-link>
          <router-link class="action-btn" to="/files">
            <span class="action-icon">📁</span>
            <div>
              <strong>{{ $t('dashboard.file_manager') }}</strong>
              <p class="muted text-sm">Monaco editör ve dosya gezgini</p>
            </div>
          </router-link>
          <router-link class="action-btn" to="/logs">
            <span class="action-icon">⚡</span>
            <div>
              <strong>Canlı Loglar</strong>
              <p class="muted text-sm">Anlık access ve error akışı</p>
            </div>
          </router-link>
          <router-link class="action-btn" to="/cron">
            <span class="action-icon">⏰</span>
            <div>
              <strong>Cron Görevleri</strong>
              <p class="muted text-sm">Zamanlanmış arkaplan işleri</p>
            </div>
          </router-link>
          <router-link class="action-btn" to="/security">
            <span class="action-icon">🛡️</span>
            <div>
              <strong>Güvenlik Profili</strong>
              <p class="muted text-sm">PHP sertleştirme ve limitler</p>
            </div>
          </router-link>
          <router-link class="action-btn" to="/drift">
            <span class="action-icon">🕵️</span>
            <div>
              <strong>{{ $t('dashboard.drift_scan') }}</strong>
              <p class="muted text-sm">İstenen durum kontrolü ve onarım</p>
            </div>
          </router-link>
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
const health = ref(null)
const error = ref('')

function formatCheckName(name) {
  if (name === 'sqlite') return 'SQLite (Metadata)'
  if (name === 'mariadb') return 'MariaDB (Veritabanı)'
  if (name === 'ols') return 'OpenLiteSpeed Servisi'
  if (name === 'ssl_certs') return 'SSL Sertifikaları'
  return name
}

async function loadData() {
  error.value = ''
  try {
    status.value = await api('/status')
  } catch (e) {
    error.value = e.message
  }

  try {
    const res = await fetch('/api/v1/health')
    if (res.ok || res.status === 503) {
      health.value = await res.json()
    }
  } catch (e) {
    console.error('Sağlık kontrolü alınamadı:', e)
  }
}

onMounted(loadData)
</script>

<style scoped>
.dashboard-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 20px; }
.health-banner { margin-bottom: 20px; border-left: 4px solid #10b981; }
.health-banner.health-warn { border-left-color: #f59e0b; }
.health-summary { display: flex; align-items: center; gap: 12px; margin-bottom: 12px; }
.health-indicator { width: 12px; height: 12px; border-radius: 50%; display: inline-block; }
.health-indicator.status-green { background: #10b981; box-shadow: 0 0 8px #10b981; }
.health-indicator.status-yellow { background: #f59e0b; box-shadow: 0 0 8px #f59e0b; }
.health-checks-grid { display: flex; flex-wrap: wrap; gap: 8px; }
.check-pill { display: flex; align-items: center; gap: 6px; padding: 4px 10px; border-radius: 20px; font-size: 12px; font-weight: 500; }
.pill-ok { background: #dcfce7; color: #166534; }
.pill-warn { background: #fef3c7; color: #92400e; }
.pill-ok .dot { width: 6px; height: 6px; border-radius: 50%; background: #16a34a; }
.pill-warn .dot { width: 6px; height: 6px; border-radius: 50%; background: #d97706; }
.stats-row { display: grid; grid-template-columns: repeat(auto-fit, minmax(180px, 1fr)); gap: 16px; }
.stat { text-align: center; padding: 20px; }
.stat-value { font-size: 32px; font-weight: 700; color: var(--primary); margin-bottom: 4px; }
.actions-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(240px, 1fr)); gap: 16px; margin-top: 16px; }
.action-btn { display: flex; align-items: center; gap: 14px; padding: 14px; border-radius: 8px; border: 1px solid var(--border-color, #e2e8f0); text-decoration: none; color: inherit; transition: all 0.2s; background: var(--bg-card, #fff); }
.action-btn:hover { border-color: #3b82f6; transform: translateY(-1px); box-shadow: 0 4px 6px -1px rgba(0,0,0,0.05); }
.action-icon { font-size: 24px; }
</style>
