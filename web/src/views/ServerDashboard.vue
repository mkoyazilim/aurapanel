<template>
  <Layout>
    <div class="page">
      <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 24px;">
        <h1 style="margin: 0; display: flex; align-items: center; gap: 8px;">
          🖥️ Sunucu Yönetimi
        </h1>
      </div>

      <div v-if="loading" class="muted">Yükleniyor...</div>
      <div v-else class="dashboard-grid">
        <!-- Kaynak Metrikleri -->
        <div class="card" style="flex: 2;">
          <h2>📊 Anlık Kaynak Tüketimi</h2>
          <div class="metrics-row" style="display: flex; gap: 16px; margin-top: 16px;">
            <div class="metric-box">
              <div class="metric-title">CPU Kullanımı</div>
              <div class="metric-value">{{ metrics.cpu_percent.toFixed(1) }}%</div>
              <div class="progress-bar"><div class="fill" :style="{width: metrics.cpu_percent + '%'}"></div></div>
            </div>
            <div class="metric-box">
              <div class="metric-title">RAM Kullanımı ({{ metrics.ram_used_mb }}MB / {{ metrics.ram_total_mb }}MB)</div>
              <div class="metric-value">{{ metrics.ram_percent.toFixed(1) }}%</div>
              <div class="progress-bar"><div class="fill" :style="{width: metrics.ram_percent + '%'}"></div></div>
            </div>
            <div class="metric-box">
              <div class="metric-title">Disk Kullanımı ({{ metrics.disk_used_gb }}GB / {{ metrics.disk_total_gb }}GB)</div>
              <div class="metric-value">{{ metrics.disk_percent.toFixed(1) }}%</div>
              <div class="progress-bar"><div class="fill" :style="{width: metrics.disk_percent + '%'}"></div></div>
            </div>
          </div>
          <div class="muted text-sm" style="margin-top: 16px;">Uptime: {{ formatUptime(metrics.uptime_sec) }}</div>
        </div>

        <!-- Servisler ve Koruma -->
        <div style="flex: 1; display: flex; flex-direction: column; gap: 16px;">
          <div class="card">
            <h2>⚙️ Servis Durumları</h2>
            <div class="service-list">
              <div class="service-item" v-for="(status, name) in services" :key="name">
                <span style="text-transform: capitalize; font-weight: 500;">{{ name }}</span>
                <div style="display: flex; gap: 8px; align-items: center;">
                  <span class="badge" :class="status === 'active' ? 'ok' : 'error'">{{ status }}</span>
                  <button v-if="name === 'fail2ban'" class="btn btn-sm" @click="toggleFail2ban(status)">
                    {{ status === 'active' ? 'Kapat' : 'Aç' }}
                  </button>
                  <button class="btn btn-sm" @click="restartService(name)">🔄</button>
                </div>
              </div>
            </div>
          </div>

          <div class="card">
            <h2>🛡️ Güvenlik ve Portlar</h2>
            <p class="muted text-sm">Sunucu Güvenlik Duvarı (nftables) şu an varsayılan AuraPanel kalkanı ile korunmaktadır.</p>
            <div class="alert warn text-sm" style="margin-top: 12px;">Port açma/kapama ve SSH port değiştirme modülü çok yakında AuraPanel Update Center üzerinden aktif edilecektir!</div>
          </div>
        </div>
      </div>
    </div>
  </Layout>
</template>

<script setup>
import { ref, onMounted, onUnmounted } from 'vue'
import Layout from '../components/Layout.vue'
import { api } from '../api'

const loading = ref(true)
const metrics = ref(null)
const services = ref(null)
let timer = null

function formatUptime(seconds) {
  const d = Math.floor(seconds / (3600*24))
  const h = Math.floor(seconds % (3600*24) / 3600)
  return `${d} Gün ${h} Saat`
}

async function loadData() {
  try {
    metrics.value = await api('/server/metrics')
    services.value = await api('/server/services')
  } catch (err) {
    console.error(err)
  } finally {
    loading.value = false
  }
}

async function restartService(name) {
  if (!confirm(`${name} servisini yeniden başlatmak istiyor musunuz?`)) return
  try {
    await api('/server/action', { method: 'POST', body: { action: 'restart', target: name } })
    await loadData()
  } catch(e) { alert("Hata: " + e.message) }
}

async function toggleFail2ban(currentStatus) {
  const action = currentStatus === 'active' ? 'stop' : 'start'
  try {
    await api('/server/action', { method: 'POST', body: { action: action, target: 'fail2ban' } })
    await loadData()
  } catch(e) { alert("Hata: " + e.message) }
}

onMounted(() => {
  loadData()
  timer = setInterval(loadData, 5000)
})

onUnmounted(() => {
  clearInterval(timer)
})
</script>

<style scoped>
.dashboard-grid { display: flex; gap: 24px; align-items: flex-start; }
@media (max-width: 900px) { .dashboard-grid { flex-direction: column; } }
.metric-box { background: rgba(0,0,0,0.02); padding: 16px; border-radius: 8px; flex: 1; }
.metric-title { font-size: 13px; color: var(--muted); margin-bottom: 8px; }
.metric-value { font-size: 24px; font-weight: bold; margin-bottom: 12px; }
.progress-bar { width: 100%; height: 6px; background: #e2e8f0; border-radius: 4px; overflow: hidden; }
.progress-bar .fill { height: 100%; background: var(--primary); transition: width 0.3s ease; }
.service-list { display: flex; flex-direction: column; gap: 12px; margin-top: 16px; }
.service-item { display: flex; justify-content: space-between; align-items: center; padding: 8px 0; border-bottom: 1px solid var(--border-color); }
.service-item:last-child { border-bottom: none; }
</style>
