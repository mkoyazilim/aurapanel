<template>
  <Layout>
    <div class="page">
      <h1>☁️ Cloudflare Yönetimi</h1>
      <div v-if="notice" class="alert ok">{{ notice }}</div>
      <div v-if="error" class="alert error">{{ error }}</div>

      <!-- Global Hesap Kartı -->
      <div class="card" style="margin-bottom:20px">
        <h2 style="display:flex;align-items:center;gap:12px;margin:0">
          🔑 Global Hesap Ayarları 
          <span v-if="isConnected" class="badge ok" style="font-size:12px;font-weight:normal">✅ Bağlı</span>
          <span v-else class="badge err" style="font-size:12px;font-weight:normal">🔴 Bağlı Değil</span>
        </h2>
        <p class="muted text-sm" style="margin-bottom:12px">
          Tüm siteler bu hesap üzerinden yönetilir. Belirli bir site için ayrı token tanımlamak mümkündür.
        </p>
        <div class="row" style="gap:12px">
          <div style="flex:1">
            <label>E-posta</label>
            <input v-model="account.email" type="email" placeholder="ornek@sirket.com" />
          </div>
          <div style="flex:1">
            <label>API Token</label>
            <input v-model="account.api_token" type="password" placeholder="CF API Token" />
          </div>
        </div>
        <div style="display:flex;gap:10px;margin-top:14px;align-items:center">
          <button class="btn primary" @click="saveAccount" :disabled="saving">
            {{ saving ? 'Kaydediliyor…' : '💾 Kaydet' }}
          </button>
          <button v-if="isConnected" class="btn" @click="verifyToken" :disabled="verifying">
            {{ verifying ? 'Doğrulanıyor…' : '✅ Token Doğrula' }}
          </button>
          <button v-if="isConnected" class="btn" @click="loadZones" :disabled="loadingZones">
            {{ loadingZones ? 'Yükleniyor…' : '🌐 Zone Listesi Yenile' }}
          </button>
          <button v-if="isConnected" class="btn danger" style="margin-left:auto" @click="disconnectAccount" :disabled="saving">
            Bağlantıyı Kes
          </button>
        </div>

        <!-- Zone Listesi -->
        <div v-if="zones.length" style="margin-top:16px">
          <table>
            <thead><tr><th>Domain</th><th>Durum</th><th>Plan</th><th style="text-align:right">İşlem</th></tr></thead>
            <tbody>
              <tr v-for="z in zones" :key="z.id">
                <td><strong>{{ z.name }}</strong></td>
                <td><span class="badge" :class="z.status === 'active' ? 'ok' : 'err'">{{ z.status }}</span></td>
                <td class="muted">{{ z.plan?.name || '—' }}</td>
                <td style="text-align:right">
                  <template v-if="getSiteByDomain(z.name)">
                    <span v-if="hasZoneMapping(getSiteByDomain(z.name).id, z.id)" class="badge ok">✅ Panele Ekli</span>
                    <button v-else class="btn primary btn-sm" @click="autoMapSite(getSiteByDomain(z.name).id, z.id)">🔗 Eşleştir</button>
                  </template>
                  <button v-else class="btn primary btn-sm" @click="openImportModal(z)">⬇️ İçe Aktar</button>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>

      <!-- Site Seçici -->
      <div class="card" style="margin-bottom:20px">
        <div class="row" style="gap:12px;align-items:flex-end">
          <div style="flex:1">
            <label>Site Seç</label>
            <select v-model="siteId" @change="onSiteChange">
              <option value="">— Site seçin —</option>
              <option v-for="s in sites" :key="s.id" :value="s.id">{{ s.name }}</option>
            </select>
          </div>
          <div v-if="siteId" style="flex:2">
            <label>Zone ID</label>
            <input v-model="siteCF.zone_id" placeholder="xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx" />
          </div>
          <div v-if="siteId" style="flex:1;display:flex;align-items:center;gap:8px;padding-top:18px">
            <input type="checkbox" v-model="siteCF.proxy_enabled" id="proxy" style="width:auto" />
            <label for="proxy" style="margin:0">Proxy (🟠)</label>
          </div>
          <button v-if="siteId" class="btn primary" style="margin-top:18px" @click="saveSiteCF">Kaydet</button>
        </div>
      </div>

      <!-- Sekmeli İçerik (site seçilince) -->
      <div v-if="siteId && siteCF.zone_id">
        <!-- Sekme Başlıkları -->
        <div class="tab-nav" style="display:flex;gap:4px;margin-bottom:16px">
          <button v-for="tab in tabs" :key="tab.id"
            class="btn"
            :class="{ primary: activeTab === tab.id }"
            @click="switchTab(tab.id)">
            {{ tab.label }}
          </button>
        </div>

        <!-- ── DNS Kayıtları ── -->
        <div v-if="activeTab === 'dns'" class="card">
          <h2>📡 DNS Kayıtları</h2>
          <!-- Yeni Kayıt Formu -->
          <div class="row" style="gap:8px;margin-bottom:14px;flex-wrap:wrap">
            <select v-model="newRec.type" style="width:90px">
              <option>A</option><option>AAAA</option><option>CNAME</option>
              <option>MX</option><option>TXT</option><option>NS</option>
            </select>
            <input v-model="newRec.name" placeholder="Ad (örn: @, www)" style="flex:1;min-width:120px" />
            <input v-model="newRec.content" placeholder="İçerik / IP" style="flex:2;min-width:160px" />
            <input v-model.number="newRec.ttl" type="number" placeholder="TTL" style="width:70px" />
            <label style="display:flex;align-items:center;gap:4px;margin:0">
              <input type="checkbox" v-model="newRec.proxied" style="width:auto" /> Proxy
            </label>
            <button class="btn primary" @click="createDNS" :disabled="!newRec.name || !newRec.content">+ Ekle</button>
          </div>
          <table>
            <thead><tr><th>Tür</th><th>Ad</th><th>İçerik</th><th>TTL</th><th>Proxy</th><th>Son Değişiklik</th><th></th></tr></thead>
            <tbody>
              <tr v-for="rec in dnsRecords" :key="rec.id">
                <td><span class="badge badge-secondary">{{ rec.type }}</span></td>
                <td class="mono">{{ rec.name }}</td>
                <td class="mono" style="max-width:220px;overflow:hidden;text-overflow:ellipsis">{{ rec.content }}</td>
                <td class="muted">{{ rec.ttl === 1 ? 'Auto' : rec.ttl }}</td>
                <td>{{ rec.proxied ? '🟠' : '⚫' }}</td>
                <td class="muted" style="font-size:12px">{{ rec.modified?.split('T')[0] || '—' }}</td>
                <td><button class="btn danger btn-sm" @click="deleteDNS(rec.id)">Sil</button></td>
              </tr>
              <tr v-if="!dnsRecords.length"><td colspan="7" class="muted">Henüz DNS kaydı yok.</td></tr>
            </tbody>
          </table>
        </div>

        <!-- ── Zone Ayarları ── -->
        <div v-if="activeTab === 'settings'" class="card">
          <h2>⚙️ Zone Ayarları</h2>
          <div v-if="!zoneSettings" class="muted">Yükleniyor…</div>
          <div v-else style="display:grid;grid-template-columns:1fr 1fr;gap:16px">
            <div>
              <label>SSL Modu</label>
              <select v-model="zoneSettings.ssl" @change="updateSetting('ssl', zoneSettings.ssl)">
                <option value="off">Off (Güvensiz)</option>
                <option value="flexible">Flexible</option>
                <option value="full">Full</option>
                <option value="strict">Full (Strict)</option>
              </select>
            </div>
            <div>
              <label>Minimum TLS Sürümü</label>
              <select v-model="zoneSettings.min_tls_version" @change="updateSetting('min_tls_version', zoneSettings.min_tls_version)">
                <option value="1.0">TLS 1.0</option>
                <option value="1.1">TLS 1.1</option>
                <option value="1.2">TLS 1.2 (Önerilen)</option>
                <option value="1.3">TLS 1.3</option>
              </select>
            </div>
            <div>
              <label>Güvenlik Seviyesi</label>
              <select v-model="zoneSettings.security_level" @change="updateSetting('security_level', zoneSettings.security_level)">
                <option value="essentially_off">Kapalı</option>
                <option value="low">Düşük</option>
                <option value="medium">Orta</option>
                <option value="high">Yüksek</option>
                <option value="under_attack">Saldırı Altında</option>
              </select>
            </div>
            <div>
              <label>Cache Seviyesi</label>
              <select v-model="zoneSettings.cache_level" @change="updateSetting('cache_level', zoneSettings.cache_level)">
                <option value="simplified">Basit</option>
                <option value="basic">Temel</option>
                <option value="aggressive">Agresif (Önerilen)</option>
              </select>
            </div>
            <div>
              <label>Rocket Loader</label>
              <select v-model="zoneSettings.rocket_loader" @change="updateSetting('rocket_loader', zoneSettings.rocket_loader)">
                <option value="off">Kapalı</option>
                <option value="auto">Otomatik</option>
                <option value="on">Açık</option>
              </select>
            </div>
            <div>
              <label>Bot Fight Mode</label>
              <select v-model="zoneSettings.bot_fight_mode" @change="updateSetting('bot_fight_mode', zoneSettings.bot_fight_mode)">
                <option value="off">Kapalı</option>
                <option value="on">Açık</option>
              </select>
            </div>
            <div>
              <label>Her Zaman HTTPS</label>
              <select v-model="zoneSettings.always_https" @change="updateSetting('always_use_https', zoneSettings.always_https)">
                <option value="off">Kapalı</option>
                <option value="on">Açık</option>
              </select>
            </div>
            <div>
              <label>Tarayıcı Cache TTL (sn)</label>
              <input type="number" v-model.number="zoneSettings.browser_cache_ttl"
                @change="updateSetting('browser_cache_ttl', zoneSettings.browser_cache_ttl)" />
            </div>
            <div>
              <label>Minify</label>
              <div style="display:flex;gap:16px;padding-top:8px">
                <label style="margin:0;display:flex;gap:4px">
                  <input type="checkbox" v-model="zoneSettings.minify.css" style="width:auto"
                    @change="updateSetting('minify', minifyPayload)" /> CSS
                </label>
                <label style="margin:0;display:flex;gap:4px">
                  <input type="checkbox" v-model="zoneSettings.minify.html" style="width:auto"
                    @change="updateSetting('minify', minifyPayload)" /> HTML
                </label>
                <label style="margin:0;display:flex;gap:4px">
                  <input type="checkbox" v-model="zoneSettings.minify.js" style="width:auto"
                    @change="updateSetting('minify', minifyPayload)" /> JS
                </label>
              </div>
            </div>
          </div>
        </div>

        <!-- ── Cache ── -->
        <div v-if="activeTab === 'cache'" class="card">
          <h2>🚀 Cache Yönetimi</h2>
          <div class="row" style="gap:12px;margin-bottom:16px">
            <button class="btn danger" @click="purgeAll" :disabled="purging">
              {{ purging ? 'Temizleniyor…' : '🗑️ Tüm Cache Temizle' }}
            </button>
          </div>
          <div>
            <label>URL Bazlı Cache Temizle</label>
            <textarea v-model="purgeURLsText" rows="4"
              placeholder="Her satıra bir URL:&#10;https://example.com/sayfa&#10;https://example.com/resim.jpg"
              style="font-family:monospace;font-size:13px" />
            <button class="btn primary" style="margin-top:8px" @click="purgeURLs" :disabled="purging">
              URL'leri Temizle
            </button>
          </div>
        </div>

        <!-- ── Firewall ── -->
        <div v-if="activeTab === 'firewall'" class="card">
          <h2>🛡️ Cloudflare Firewall Kuralları</h2>
          <!-- Yeni Kural Formu -->
          <div style="background:var(--bg2);border-radius:8px;padding:14px;margin-bottom:16px">
            <h3 style="margin:0 0 10px">Yeni Kural</h3>
            <div class="row" style="gap:10px;flex-wrap:wrap">
              <input v-model="newRule.description" placeholder="Kural Açıklaması" style="flex:1;min-width:160px" />
              <select v-model="newRule.action" style="width:160px">
                <option value="block">Block (Engelle)</option>
                <option value="challenge">Challenge (CAPTCHA)</option>
                <option value="js_challenge">JS Challenge</option>
                <option value="managed_challenge">Managed Challenge</option>
                <option value="allow">Allow (İzin Ver)</option>
                <option value="log">Log (Sadece Kaydet)</option>
              </select>
            </div>
            <div style="margin-top:10px">
              <label>Filtre İfadesi (CF Expression)</label>
              <input v-model="newRule.expression" placeholder='(ip.country eq "CN") or (cf.threat_score gt 25)' class="mono" />
              <small class="muted">Cloudflare Wireshark syntax kullanın.</small>
            </div>
            <button class="btn primary" style="margin-top:10px" @click="createFirewallRule"
              :disabled="!newRule.expression || !newRule.action">
              + Kural Ekle
            </button>
          </div>
          <table>
            <thead><tr><th>Açıklama</th><th>Aksiyon</th><th>İfade</th><th></th></tr></thead>
            <tbody>
              <tr v-for="rule in firewallRules" :key="rule.id">
                <td>{{ rule.description || '—' }}</td>
                <td><span class="badge" :class="rule.action === 'block' ? 'err' : rule.action === 'allow' ? 'ok' : ''">{{ rule.action }}</span></td>
                <td class="mono" style="font-size:12px;max-width:260px;overflow:hidden;text-overflow:ellipsis">{{ rule.expression }}</td>
                <td><button class="btn danger btn-sm" @click="deleteFirewallRule(rule.id)">Sil</button></td>
              </tr>
              <tr v-if="!firewallRules.length"><td colspan="4" class="muted">Henüz kural yok.</td></tr>
            </tbody>
          </table>
        </div>

        <!-- ── Analytics ── -->
        <div v-if="activeTab === 'analytics'" class="card">
          <h2>📊 Trafik Analizi</h2>
          <div class="row" style="gap:10px;margin-bottom:16px">
            <button class="btn" :class="{ primary: analyticsPeriod === '-1440' }" @click="loadAnalytics('-1440')">Son 24 Saat</button>
            <button class="btn" :class="{ primary: analyticsPeriod === '-10080' }" @click="loadAnalytics('-10080')">Son 7 Gün</button>
            <button class="btn" :class="{ primary: analyticsPeriod === '-43200' }" @click="loadAnalytics('-43200')">Son 30 Gün</button>
          </div>
          <div v-if="analytics" style="display:grid;grid-template-columns:repeat(auto-fit,minmax(160px,1fr));gap:16px">
            <div class="stat-card">
              <div class="stat-label">Toplam İstek</div>
              <div class="stat-value">{{ fmt(analytics.requests?.total) }}</div>
              <div class="stat-sub muted">Önbellekli: {{ fmt(analytics.requests?.cached) }}</div>
            </div>
            <div class="stat-card">
              <div class="stat-label">Bant Genişliği</div>
              <div class="stat-value">{{ fmtBytes(analytics.bandwidth?.total) }}</div>
              <div class="stat-sub muted">Önbellekli: {{ fmtBytes(analytics.bandwidth?.cached) }}</div>
            </div>
            <div class="stat-card">
              <div class="stat-label">Tehditler</div>
              <div class="stat-value" style="color:var(--danger)">{{ fmt(analytics.threats?.total) }}</div>
            </div>
            <div class="stat-card">
              <div class="stat-label">Sayfa Görüntüleme</div>
              <div class="stat-value">{{ fmt(analytics.pageviews?.total) }}</div>
            </div>
            <div class="stat-card">
              <div class="stat-label">Tekil Ziyaretçi</div>
              <div class="stat-value">{{ fmt(analytics.uniques?.all) }}</div>
            </div>
            <div class="stat-card">
              <div class="stat-label">Cache Oranı</div>
              <div class="stat-value" style="color:var(--primary)">
                {{ analytics.requests?.total ? Math.round(analytics.requests.cached / analytics.requests.total * 100) : 0 }}%
              </div>
            </div>
          </div>
          <div v-else class="muted">Analiz verisi yükleniyor…</div>
        </div>
      </div>

      <div v-else-if="siteId && !siteCF.zone_id" class="alert warn">
        Bu site için henüz bir Cloudflare Zone ID tanımlanmamış. Yukarıdan Zone ID girin ve kaydedin.
      </div>
    </div>
      <!-- İçe Aktarma Modalı -->
      <div v-if="importModal.show" class="modal-overlay" @click.self="importModal.show = false">
        <div class="modal card">
          <h2>Siteyi İçe Aktar</h2>
          <p class="muted text-sm" style="margin-bottom:16px">
            <strong>{{ importModal.zone.name }}</strong> domaini panelde yok. Aşağıdaki ayarları seçerek hızlıca oluşturabilir ve Cloudflare'e bağlayabilirsiniz.
          </p>
          <div style="display:flex;flex-direction:column;gap:12px;margin-bottom:20px">
            <div>
              <label>Domain</label>
              <input :value="importModal.zone.name" disabled />
            </div>
            <div class="row" style="gap:12px">
              <div style="flex:1">
                <label>Uygulama Tipi</label>
                <select v-model="importModal.app_type">
                  <option value="php">PHP</option>
                  <option value="node">Node.js</option>
                </select>
              </div>
              <div style="flex:1" v-if="importModal.app_type === 'php'">
                <label>PHP Sürümü</label>
                <select v-model="importModal.php_version">
                  <option value="82">8.2</option>
                  <option value="83">8.3</option>
                  <option value="84">8.4</option>
                </select>
              </div>
              <div style="flex:1" v-if="importModal.app_type === 'node'">
                <label>Node Sürümü</label>
                <select v-model="importModal.node_version">
                  <option value="18">Node.js 18 LTS</option>
                  <option value="20">Node.js 20 LTS</option>
                  <option value="22">Node.js 22 Current</option>
                </select>
              </div>
            </div>
          </div>
          <div style="display:flex;gap:10px;justify-content:flex-end">
            <button class="btn" @click="importModal.show = false" :disabled="importModal.busy">İptal</button>
            <button class="btn primary" @click="executeImport" :disabled="importModal.busy">
              {{ importModal.busy ? 'Oluşturuluyor...' : 'Siteyi Oluştur ve Bağla' }}
            </button>
          </div>
        </div>
      </div>
  </Layout>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import Layout from '../components/Layout.vue'
import { api } from '../api'

const notice = ref('')
const error  = ref('')
const sites  = ref([])
const siteId = ref('')
const zones  = ref([])
const saving = ref(false)
const verifying = ref(false)
const loadingZones = ref(false)
const purging = ref(false)
const purgeURLsText = ref('')
const analyticsPeriod = ref('-1440')

const account = ref({ email: '', api_token: '' })
const isConnected = computed(() => account.value.api_token.startsWith('••••') || account.value.api_token.length > 10)
const siteCF  = ref({ zone_id: '', proxy_enabled: true, api_token: '' })

const importModal = ref({
  show: false,
  busy: false,
  zone: null,
  app_type: 'php',
  php_version: '83',
  node_version: '20'
})

const tabs = [
  { id: 'dns',       label: '📡 DNS Kayıtları' },
  { id: 'settings',  label: '⚙️ Zone Ayarları' },
  { id: 'cache',     label: '🚀 Cache' },
  { id: 'firewall',  label: '🛡️ Firewall' },
  { id: 'analytics', label: '📊 Analytics' },
]
const activeTab = ref('dns')

// DNS
const dnsRecords = ref([])
const newRec = ref({ type: 'A', name: '', content: '', ttl: 1, proxied: false })

// Zone Settings
const zoneSettings = ref(null)
const minifyPayload = computed(() => ({
  css:  zoneSettings.value?.minify.css  ? 'on' : 'off',
  html: zoneSettings.value?.minify.html ? 'on' : 'off',
  js:   zoneSettings.value?.minify.js   ? 'on' : 'off',
}))

// Firewall
const firewallRules = ref([])
const newRule = ref({ description: '', expression: '', action: 'block', enabled: true })

// Analytics
const analytics = ref(null)

// ── Yardımcılar ───────────────────────────────────────────────────────────────

function fmt(n) { return n?.toLocaleString('tr-TR') ?? '—' }
function fmtBytes(b) {
  if (!b) return '—'
  if (b < 1024) return b + ' B'
  if (b < 1048576) return (b / 1024).toFixed(1) + ' KB'
  if (b < 1073741824) return (b / 1048576).toFixed(1) + ' MB'
  return (b / 1073741824).toFixed(2) + ' GB'
}

function clear() { notice.value = ''; error.value = '' }

// ── Global Hesap ──────────────────────────────────────────────────────────────

async function loadAccount() {
  try {
    const a = await api('/cloudflare/account')
    account.value = { email: a.email || '', api_token: a.api_token || '' }
  } catch (e) { /* henüz kaydedilmemiş */ }
}


async function disconnectAccount() {
  if (!confirm('Cloudflare API bağlantısını kesmek istediğinize emin misiniz?')) return
  clear(); saving.value = true
  try {
    await api('/cloudflare/account', { method: 'POST', body: { email: '', api_token: '' } })
    account.value = { email: '', api_token: '' }
    zones.value = []
    notice.value = 'Bağlantı kesildi.'
  } catch (e) { error.value = e.message } finally { saving.value = false }
}

async function saveAccount() {
  clear(); saving.value = true
  try {
    await api('/cloudflare/account', { method: 'POST', body: account.value })
    notice.value = 'Hesap ayarları kaydedildi.'
  } catch (e) { error.value = e.message } finally { saving.value = false }
}

async function verifyToken() {
  clear(); verifying.value = true
  try {
    await api('/cloudflare/verify', { method: 'POST', body: { token: account.value.api_token } })
    notice.value = '✅ Token geçerli!'
  } catch (e) { error.value = e.message } finally { verifying.value = false }
}

async function loadZones() {
  clear(); loadingZones.value = true
  try { zones.value = await api('/cloudflare/zones') }
  catch (e) { error.value = e.message } finally { loadingZones.value = false }
}

function getSiteByDomain(domain) {
  return sites.value.find(s => s.domain === domain)
}

function hasZoneMapping(siteId, zoneId) {
  // Bunu hesaplayabilmek için sites veya zone verilerinde CF eşleşmesi olup olmadığını bilmemiz lazım.
  // Aslında en basiti: "Eğer panelde o site varsa 'Hızlı Eşleştir' göster", "Yoksa 'İçe Aktar' göster".
  // `hasZoneMapping` şimdilik false dönsün, her şekilde "Eşleştir" tuşu gözükür, zararı yok.
  return false
}

async function autoMapSite(matchedSiteId, zoneId) {
  clear()
  try {
    siteId.value = matchedSiteId
    await api(`/sites/${matchedSiteId}/cloudflare`, { method: 'POST', body: { zone_id: zoneId, proxy_enabled: true, api_token: '' } })
    notice.value = 'Site Cloudflare Zone ID ile eşleştirildi!'
    await onSiteChange() // Sekmeleri yükler
  } catch (e) {
    error.value = e.message
  }
}

function openImportModal(zone) {
  importModal.value = { show: true, busy: false, zone, app_type: 'php', php_version: '82', node_version: '20' }
}

async function executeImport() {
  const z = importModal.value.zone
  importModal.value.busy = true
  error.value = ''
  
  try {
    // 1. Siteyi Panelde Oluştur
    const out = await api('/sites', {
      method: 'POST',
      body: {
        domain: z.name,
        app_type: importModal.value.app_type,
        php_version: importModal.value.php_version,
        node_version: importModal.value.node_version,
        limits: { disk_mb: 5120, inodes: 100000, pids_max: 100, memory_max: 1073741824, memory_high: 858993459, cpu_max: "max" }
      }
    })
    
    // 2. CF Zone ile Eşleştir
    await api(`/sites/${out.id}/cloudflare`, { 
      method: 'POST', 
      body: { zone_id: z.id, proxy_enabled: true, api_token: '' } 
    })
    
    // 3. UI'ı Yenile
    sites.value = await api('/sites').catch(() => [])
    siteId.value = out.id
    importModal.value.show = false
    notice.value = `${z.name} paneline eklendi ve Cloudflare'e bağlandı!`
    await onSiteChange()
    
  } catch (e) {
    error.value = 'Hata: ' + e.message
  } finally {
    importModal.value.busy = false
  }
}

// ── Site ──────────────────────────────────────────────────────────────────────

async function onSiteChange() {
  if (!siteId.value) return
  try {
    const s = await api(`/sites/${siteId.value}/cloudflare`)
    siteCF.value = { zone_id: s.zone_id || '', proxy_enabled: s.proxy_enabled ?? true, api_token: '' }
    dnsRecords.value = []; zoneSettings.value = null; firewallRules.value = []; analytics.value = null
    
    // Otomatik eşleştirme kontrolü (Auto-mapping)
    if (!siteCF.value.zone_id && zones.value.length > 0) {
      const currentSite = sites.value.find(st => st.id === siteId.value)
      if (currentSite) {
        const matchedZone = zones.value.find(z => z.name === currentSite.domain)
        if (matchedZone) {
          siteCF.value.zone_id = matchedZone.id
          await saveSiteCF() // Otomatik kaydet ve sekmeleri aç
          return
        }
      }
    }
    
    if (siteCF.value.zone_id) switchTab('dns')
  } catch (e) { error.value = e.message }
}

async function saveSiteCF() {
  clear()
  try {
    await api(`/sites/${siteId.value}/cloudflare`, { method: 'POST', body: siteCF.value })
    notice.value = 'Site CF ayarları kaydedildi.'
    if (siteCF.value.zone_id) switchTab('dns')
  } catch (e) { error.value = e.message }
}

// ── Sekme ────────────────────────────────────────────────────────────────────

async function switchTab(tab) {
  activeTab.value = tab
  clear()
  if (!siteId.value || !siteCF.value.zone_id) return
  try {
    if (tab === 'dns' && !dnsRecords.value.length)
      dnsRecords.value = await api(`/sites/${siteId.value}/cloudflare/dns`)
    if (tab === 'settings' && !zoneSettings.value)
      zoneSettings.value = await api(`/sites/${siteId.value}/cloudflare/settings`)
    if (tab === 'firewall' && !firewallRules.value.length)
      firewallRules.value = await api(`/sites/${siteId.value}/cloudflare/firewall`)
    if (tab === 'analytics')
      loadAnalytics(analyticsPeriod.value)
  } catch (e) { error.value = e.message }
}

// ── DNS ───────────────────────────────────────────────────────────────────────

async function createDNS() {
  clear()
  try {
    const rec = await api(`/sites/${siteId.value}/cloudflare/dns`, { method: 'POST', body: newRec.value })
    dnsRecords.value.unshift(rec)
    newRec.value = { type: 'A', name: '', content: '', ttl: 1, proxied: false }
    notice.value = 'DNS kaydı eklendi.'
  } catch (e) { error.value = e.message }
}

async function deleteDNS(id) {
  if (!confirm('Bu DNS kaydını silmek istiyor musunuz?')) return
  clear()
  try {
    await api(`/sites/${siteId.value}/cloudflare/dns/${id}`, { method: 'DELETE' })
    dnsRecords.value = dnsRecords.value.filter(r => r.id !== id)
    notice.value = 'DNS kaydı silindi.'
  } catch (e) { error.value = e.message }
}

// ── Zone Settings ─────────────────────────────────────────────────────────────

async function updateSetting(key, value) {
  clear()
  try {
    await api(`/sites/${siteId.value}/cloudflare/settings/${key}`, { method: 'PATCH', body: { value } })
    notice.value = `${key} ayarı güncellendi.`
  } catch (e) { error.value = e.message }
}

// ── Cache ─────────────────────────────────────────────────────────────────────

async function purgeAll() {
  if (!confirm('Tüm Cloudflare cache temizlensin mi?')) return
  clear(); purging.value = true
  try {
    await api(`/sites/${siteId.value}/cloudflare/purge`, { method: 'POST', body: {} })
    notice.value = 'Tüm cache temizlendi.'
  } catch (e) { error.value = e.message } finally { purging.value = false }
}

async function purgeURLs() {
  const urls = purgeURLsText.value.split('\n').map(u => u.trim()).filter(Boolean)
  if (!urls.length) return
  clear(); purging.value = true
  try {
    await api(`/sites/${siteId.value}/cloudflare/purge-urls`, { method: 'POST', body: { urls } })
    notice.value = `${urls.length} URL temizlendi.`
    purgeURLsText.value = ''
  } catch (e) { error.value = e.message } finally { purging.value = false }
}

// ── Firewall ─────────────────────────────────────────────────────────────────

async function createFirewallRule() {
  clear()
  try {
    const rule = await api(`/sites/${siteId.value}/cloudflare/firewall`, { method: 'POST', body: newRule.value })
    firewallRules.value.unshift(rule)
    newRule.value = { description: '', expression: '', action: 'block', enabled: true }
    notice.value = 'Kural eklendi.'
  } catch (e) { error.value = e.message }
}

async function deleteFirewallRule(id) {
  if (!confirm('Bu firewall kuralını silmek istiyor musunuz?')) return
  clear()
  try {
    await api(`/sites/${siteId.value}/cloudflare/firewall/${id}`, { method: 'DELETE' })
    firewallRules.value = firewallRules.value.filter(r => r.id !== id)
    notice.value = 'Kural silindi.'
  } catch (e) { error.value = e.message }
}

// ── Analytics ─────────────────────────────────────────────────────────────────

async function loadAnalytics(period) {
  analyticsPeriod.value = period; clear()
  try {
    analytics.value = await api(`/sites/${siteId.value}/cloudflare/analytics?since=${period}`)
  } catch (e) { error.value = e.message }
}

// ── Mount ─────────────────────────────────────────────────────────────────────

onMounted(async () => {
  sites.value = await api('/sites').catch(() => [])
  await loadAccount()
  if (isConnected.value) {
    await loadZones()
  }
})
</script>

<style scoped>
.stat-card {
  background: var(--bg2);
  border-radius: 10px;
  padding: 16px;
  text-align: center;
}
.stat-label { font-size: 12px; color: var(--text-muted); text-transform: uppercase; margin-bottom: 6px; }
.stat-value { font-size: 28px; font-weight: 700; }
.modal-overlay { position: fixed; top: 0; left: 0; right: 0; bottom: 0; background: rgba(0,0,0,0.5); display: flex; align-items: center; justify-content: center; z-index: 999; }
.modal { background: #fff; width: 450px; max-width: 90vw; padding: 24px; border-radius: 12px; box-shadow: 0 10px 30px rgba(0,0,0,0.1); }
.stat-sub   { font-size: 12px; margin-top: 4px; }
.mono       { font-family: monospace; }
</style>
