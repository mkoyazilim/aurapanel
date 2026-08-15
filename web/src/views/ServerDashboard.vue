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
        <!-- Sol kolon: Metrikler + Firewall -->
        <div style="flex: 2; display: flex; flex-direction: column; gap: 16px;">

          <!-- Kaynak Metrikleri -->
          <div class="card">
            <h2>📊 Anlık Kaynak Tüketimi</h2>
            <div class="metrics-row">
              <div class="metric-box">
                <div class="metric-title">CPU Kullanımı</div>
                <div class="metric-value">{{ metrics.cpu_percent.toFixed(1) }}%</div>
                <div class="progress-bar"><div class="fill" :style="{width: metrics.cpu_percent + '%', background: metrics.cpu_percent > 85 ? '#ef4444' : metrics.cpu_percent > 60 ? '#f59e0b' : 'var(--primary)'}"></div></div>
              </div>
              <div class="metric-box">
                <div class="metric-title">RAM ({{ metrics.ram_used_mb }}MB / {{ metrics.ram_total_mb }}MB)</div>
                <div class="metric-value">{{ metrics.ram_percent.toFixed(1) }}%</div>
                <div class="progress-bar"><div class="fill" :style="{width: metrics.ram_percent + '%', background: metrics.ram_percent > 90 ? '#ef4444' : metrics.ram_percent > 70 ? '#f59e0b' : 'var(--primary)'}"></div></div>
              </div>
              <div class="metric-box">
                <div class="metric-title">Disk ({{ metrics.disk_used_gb }}GB / {{ metrics.disk_total_gb }}GB)</div>
                <div class="metric-value">{{ metrics.disk_percent.toFixed(1) }}%</div>
                <div class="progress-bar"><div class="fill" :style="{width: metrics.disk_percent + '%', background: metrics.disk_percent > 90 ? '#ef4444' : 'var(--primary)'}"></div></div>
              </div>
            </div>
            <div class="muted text-sm" style="margin-top: 16px;">Uptime: {{ formatUptime(metrics.uptime_sec) }}</div>
          </div>

          <!-- Güvenlik Duvarı -->
          <div class="card">
            <div style="display: flex; justify-content: space-between; align-items: center;">
              <h2 style="margin: 0;">🛡️ Güvenlik Duvarı (nftables)</h2>
              <button class="btn btn-sm" @click="showAddRule = true">+ Port Ekle</button>
            </div>

            <div v-if="fwLoading" class="muted text-sm" style="margin-top: 12px;">Kurallar yükleniyor...</div>
            <div v-else>
              <!-- Açık Port Listesi -->
              <table class="fw-table" style="margin-top: 16px;">
                <thead>
                  <tr>
                    <th>Port</th>
                    <th>Protokol</th>
                    <th>Açıklama</th>
                    <th>İşlem</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-if="!fwRules.length">
                    <td colspan="4" class="muted text-sm" style="text-align:center; padding: 12px;">Kural bulunamadı.</td>
                  </tr>
                  <tr v-for="rule in fwRules" :key="rule.port + rule.proto">
                    <td><strong>{{ rule.port }}</strong></td>
                    <td><span class="badge ok">{{ rule.proto }}</span></td>
                    <td class="muted text-sm">{{ rule.comment || (rule.port === 22 ? 'SSH' : rule.port === 80 ? 'HTTP' : rule.port === 443 ? 'HTTPS' : '—') }}</td>
                    <td>
                      <button
                        v-if="![22, 80, 443].includes(rule.port)"
                        class="btn btn-sm btn-danger"
                        @click="deleteRule(rule)"
                        :disabled="fwBusy"
                      >Kapat</button>
                      <span v-else class="muted text-sm">Korumalı</span>
                    </td>
                  </tr>
                </tbody>
              </table>

              <!-- Port Ekle Formu -->
              <div v-if="showAddRule" class="add-rule-form">
                <div style="display: flex; gap: 8px; flex-wrap: wrap; align-items: flex-end;">
                  <div>
                    <label class="form-label">Port</label>
                    <input v-model.number="newPort" type="number" class="input" min="1" max="65535" placeholder="8080" style="width: 100px;" />
                  </div>
                  <div>
                    <label class="form-label">Protokol</label>
                    <select v-model="newProto" class="input">
                      <option value="tcp">TCP</option>
                      <option value="udp">UDP</option>
                    </select>
                  </div>
                  <div style="flex: 1;">
                    <label class="form-label">Açıklama (opsiyonel)</label>
                    <input v-model="newComment" type="text" class="input" maxlength="64" placeholder="ör. node-app" />
                  </div>
                  <button class="btn" @click="addRule" :disabled="fwBusy || !newPort">{{ fwBusy ? 'Ekleniyor...' : 'Ekle' }}</button>
                  <button class="btn btn-secondary" @click="showAddRule = false">İptal</button>
                </div>
              </div>
            </div>
          </div>

          <!-- SSH Port Değiştir -->
          <div class="card">
            <h2>🔑 SSH Port Değiştir</h2>
            <div class="alert warn text-sm" style="margin-bottom: 16px;">
              ⚠️ <strong>Dikkat:</strong> Bu işlem SSH bağlantı portunu değiştirir. Yeni port güvenlik duvarında önce açılır, sonra sshd yeniden yüklenir. İşlem sonrası yeni portla bağlanın.
            </div>
            <div style="display: flex; gap: 12px; align-items: flex-end; flex-wrap: wrap;">
              <div>
                <label class="form-label">Mevcut SSH Portu</label>
                <input v-model.number="sshOldPort" type="number" class="input" style="width: 100px;" />
              </div>
              <div>
                <label class="form-label">Yeni SSH Portu</label>
                <input v-model.number="sshNewPort" type="number" class="input" min="1" max="65535" placeholder="2244" style="width: 100px;" />
              </div>
              <button class="btn btn-danger" @click="changeSSHPort" :disabled="sshBusy || !sshNewPort">
                {{ sshBusy ? 'Değiştiriliyor...' : '🔄 SSH Portunu Değiştir' }}
              </button>
            </div>
            <div v-if="sshMsg" class="text-sm" :class="sshOk ? 'text-success' : 'text-error'" style="margin-top: 8px;">{{ sshMsg }}</div>
          </div>

          <!-- Panel Port Değiştir -->
          <div class="card">
            <h2>⚙️ Panel Port Değiştir</h2>
            <div class="alert info text-sm" style="margin-bottom: 16px;">
              Panel şu an <strong>127.0.0.1:8080</strong> adresinde dinleniyor. Port değiştirirseniz sayfa erişilemez olur — yeni portla gidin.
            </div>
            <div style="display: flex; gap: 12px; align-items: flex-end; flex-wrap: wrap;">
              <div>
                <label class="form-label">Yeni Panel Portu</label>
                <input v-model.number="panelNewPort" type="number" class="input" min="1" max="65535" placeholder="9090" style="width: 100px;" />
              </div>
              <button class="btn btn-danger" @click="changePanelPort" :disabled="panelBusy || !panelNewPort">
                {{ panelBusy ? 'Değiştiriliyor...' : '🔄 Panel Portunu Değiştir' }}
              </button>
            </div>
            <div v-if="panelMsg" class="text-sm" :class="panelOk ? 'text-success' : 'text-error'" style="margin-top: 8px;">{{ panelMsg }}</div>
          </div>

        </div>

        <!-- Sağ kolon: Servisler -->
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
                  <button class="btn btn-sm" @click="restartService(name)" title="Yeniden Başlat">🔄</button>
                </div>
              </div>
            </div>
          </div>

          <!-- Hızlı Bilgi -->
          <div class="card">
            <h2>ℹ️ Sunucu Bilgisi</h2>
            <div class="info-row"><span class="muted text-sm">IP Adresi</span><strong>185.190.140.62</strong></div>
            <div class="info-row"><span class="muted text-sm">SSH Portu</span><strong>{{ sshOldPort }}</strong></div>
            <div class="info-row"><span class="muted text-sm">Panel Portu</span><strong>8080</strong></div>
            <div class="info-row"><span class="muted text-sm">Firewall</span><strong>nftables (aktif)</strong></div>
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

const loading  = ref(true)
const metrics  = ref(null)
const services = ref(null)
let timer      = null

// Firewall
const fwRules    = ref([])
const fwLoading  = ref(true)
const fwBusy     = ref(false)
const showAddRule = ref(false)
const newPort    = ref(null)
const newProto   = ref('tcp')
const newComment = ref('')

// SSH port
const sshOldPort = ref(22)
const sshNewPort = ref(null)
const sshBusy    = ref(false)
const sshMsg     = ref('')
const sshOk      = ref(false)

// Panel port
const panelNewPort = ref(null)
const panelBusy    = ref(false)
const panelMsg     = ref('')
const panelOk      = ref(false)

function formatUptime(seconds) {
  const d = Math.floor(seconds / (3600 * 24))
  const h = Math.floor(seconds % (3600 * 24) / 3600)
  return `${d} Gün ${h} Saat`
}

async function loadData() {
  try {
    metrics.value  = await api('/server/metrics')
    services.value = await api('/server/services')
  } catch (err) {
    console.error(err)
  } finally {
    loading.value = false
  }
}

async function loadFirewall() {
  fwLoading.value = true
  try {
    const data = await api('/server/firewall')
    fwRules.value = data.rules || []
  } catch (err) {
    console.error(err)
  } finally {
    fwLoading.value = false
  }
}

async function addRule() {
  if (!newPort.value || newPort.value < 1 || newPort.value > 65535) {
    alert('Geçerli bir port girin (1-65535)')
    return
  }
  fwBusy.value = true
  try {
    await api('/server/firewall', {
      method: 'POST',
      body: { port: newPort.value, proto: newProto.value, comment: newComment.value }
    })
    showAddRule.value = false
    newPort.value    = null
    newComment.value = ''
    await loadFirewall()
  } catch (e) {
    alert('Hata: ' + e.message)
  } finally {
    fwBusy.value = false
  }
}

async function deleteRule(rule) {
  if (!confirm(`${rule.proto.toUpperCase()} port ${rule.port} kapatılsın mı?`)) return
  fwBusy.value = true
  try {
    await api('/server/firewall', {
      method: 'DELETE',
      body: { port: rule.port, proto: rule.proto }
    })
    await loadFirewall()
  } catch (e) {
    alert('Hata: ' + e.message)
  } finally {
    fwBusy.value = false
  }
}

async function restartService(name) {
  if (!confirm(`${name} servisi yeniden başlatılsın mı?`)) return
  try {
    await api('/server/action', { method: 'POST', body: { action: 'restart', target: name } })
    await loadData()
  } catch (e) { alert('Hata: ' + e.message) }
}

async function toggleFail2ban(currentStatus) {
  const action = currentStatus === 'active' ? 'stop' : 'start'
  try {
    await api('/server/action', { method: 'POST', body: { action, target: 'fail2ban' } })
    await loadData()
  } catch (e) { alert('Hata: ' + e.message) }
}

async function changeSSHPort() {
  if (!sshNewPort.value) return
  if (!confirm(`SSH portu ${sshOldPort.value} → ${sshNewPort.value} olarak değiştirilsin mi?\n\nİşlem sonrası yeni portla (${sshNewPort.value}) bağlanmanız gerekecek!`)) return
  sshBusy.value = true
  sshMsg.value  = ''
  try {
    await api('/server/ssh-port', {
      method: 'PUT',
      body: { new_port: sshNewPort.value, old_port: sshOldPort.value }
    })
    sshOk.value  = true
    sshMsg.value = `✅ SSH portu ${sshNewPort.value} olarak değiştirildi. Lütfen yeni portla bağlanın: ssh root@185.190.140.62 -p ${sshNewPort.value}`
    sshOldPort.value = sshNewPort.value
    sshNewPort.value = null
    await loadFirewall()
  } catch (e) {
    sshOk.value  = false
    sshMsg.value = 'Hata: ' + e.message
  } finally {
    sshBusy.value = false
  }
}

async function changePanelPort() {
  if (!panelNewPort.value) return
  if (!confirm(`Panel portu ${panelNewPort.value} olarak değiştirilsin mi?\n\nDeğiştirme sonrası yeni adres: http://185.190.140.62:${panelNewPort.value}`)) return
  panelBusy.value = true
  panelMsg.value  = ''
  try {
    await api('/server/panel-port', {
      method: 'PUT',
      body: { new_port: panelNewPort.value }
    })
    panelOk.value  = true
    panelMsg.value = `✅ Panel portu değiştirildi. Yeni adres: http://185.190.140.62:${panelNewPort.value} — şimdi oraya gidin.`
    panelNewPort.value = null
  } catch (e) {
    panelOk.value  = false
    panelMsg.value = 'Hata: ' + e.message
  } finally {
    panelBusy.value = false
  }
}

onMounted(() => {
  loadData()
  loadFirewall()
  timer = setInterval(loadData, 5000)
})

onUnmounted(() => {
  clearInterval(timer)
})
</script>

<style scoped>
.dashboard-grid { display: flex; gap: 24px; align-items: flex-start; }
@media (max-width: 900px) { .dashboard-grid { flex-direction: column; } }
.metrics-row { display: flex; gap: 16px; margin-top: 16px; }
@media (max-width: 700px) { .metrics-row { flex-direction: column; } }
.metric-box { background: rgba(0,0,0,0.02); padding: 16px; border-radius: 8px; flex: 1; }
.metric-title { font-size: 13px; color: var(--muted); margin-bottom: 8px; }
.metric-value { font-size: 24px; font-weight: bold; margin-bottom: 12px; }
.progress-bar { width: 100%; height: 6px; background: #e2e8f0; border-radius: 4px; overflow: hidden; }
.progress-bar .fill { height: 100%; transition: width 0.3s ease, background 0.3s ease; }
.service-list { display: flex; flex-direction: column; gap: 12px; margin-top: 16px; }
.service-item { display: flex; justify-content: space-between; align-items: center; padding: 8px 0; border-bottom: 1px solid var(--border-color); }
.service-item:last-child { border-bottom: none; }
.fw-table { width: 100%; border-collapse: collapse; font-size: 14px; }
.fw-table th { text-align: left; padding: 6px 8px; border-bottom: 2px solid var(--border-color); font-size: 12px; color: var(--muted); text-transform: uppercase; letter-spacing: 0.5px; }
.fw-table td { padding: 8px 8px; border-bottom: 1px solid var(--border-color); }
.fw-table tr:last-child td { border-bottom: none; }
.add-rule-form { margin-top: 16px; padding: 16px; background: rgba(0,0,0,0.03); border-radius: 8px; border: 1px solid var(--border-color); }
.info-row { display: flex; justify-content: space-between; padding: 8px 0; border-bottom: 1px solid var(--border-color); font-size: 14px; }
.info-row:last-child { border-bottom: none; }
.form-label { display: block; font-size: 12px; color: var(--muted); margin-bottom: 4px; }
.btn-danger { background: #ef4444; color: white; border-color: #ef4444; }
.btn-danger:hover { background: #dc2626; }
.btn-secondary { background: transparent; border: 1px solid var(--border-color); }
.text-success { color: #16a34a; }
.text-error { color: #ef4444; }
.alert { padding: 10px 14px; border-radius: 6px; }
.alert.warn { background: #fef3c7; border: 1px solid #f59e0b; color: #92400e; }
.alert.info { background: #eff6ff; border: 1px solid #3b82f6; color: #1e40af; }
</style>
