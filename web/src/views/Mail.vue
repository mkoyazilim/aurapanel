<template>
  <Layout>
    <div class="page">
      <h1 style="margin-bottom: 24px">✉️ {{ $t('mail.title') }}</h1>

      <div v-if="notice" class="alert success">{{ notice }}</div>
      <div v-if="error" class="alert error">{{ error }}</div>

      <!-- Mail Server Durum Kartı -->
      <div class="card" style="margin-bottom: 20px">
        <h2 style="margin: 0 0 12px 0">{{ $t('mail.server_status') }}</h2>
        <div v-if="statusLoading" class="muted">{{ $t('mail.checking_status') }}</div>
        <template v-else-if="mailStatus">
          <div v-if="!mailStatus.installed" style="text-align: center; padding: 20px 0">
            <p class="muted" style="margin-bottom: 16px">{{ $t('mail.not_installed') }}</p>
            <button class="btn primary" :disabled="setupBusy" @click="setupMail">
              {{ setupBusy ? $t('mail.btn_installing') : $t('mail.btn_install') }}
            </button>
          </div>
          <div v-else style="display: flex; gap: 12px; flex-wrap: wrap">
            <span class="badge" :style="badgeStyle(mailStatus.postfix)">
              Postfix: {{ mailStatus.postfix ? $t('mail.active') : 'Durdurulmuş' }}
            </span>
            <span class="badge" :style="badgeStyle(mailStatus.dovecot)">
              Dovecot: {{ mailStatus.dovecot ? $t('mail.active') : 'Durdurulmuş' }}
            </span>
            <span class="badge" :style="badgeStyle(mailStatus.opendkim)">
              OpenDKIM: {{ mailStatus.opendkim ? $t('mail.active') : 'Durdurulmuş' }}
            </span>
          </div>
        </template>
      </div>

      <!-- Site Seçici -->
      <div class="card" style="margin-bottom: 20px">
        <div style="display: flex; gap: 12px; align-items: flex-end">
          <div style="flex: 1">
            <label>Site Seç</label>
            <select v-model="siteId" @change="onSiteChange">
              <option value="">— Site seçin —</option>
              <option v-for="s in sites" :key="s.id" :value="s.id">{{ s.name }}</option>
            </select>
          </div>
          <div v-if="siteDomain" class="muted" style="padding-bottom: 8px">
            Domain: <strong class="mono">{{ siteDomain }}</strong>
          </div>
        </div>
      </div>

      <!-- Sekmeler (site seçilince) -->
      <template v-if="siteId">
        <div style="display: flex; gap: 4px; margin-bottom: 16px">
          <button v-for="tab in tabs" :key="tab.id" class="btn" :class="{ primary: activeTab === tab.id }" @click="activeTab = tab.id">
            {{ tab.label }}
          </button>
        </div>

        <!-- ── {{ $t('mail.tab_accounts') }} Sekmesi ── -->
        <div v-if="activeTab === 'accounts'" class="card">
          <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 16px">
            <h2 style="margin: 0">E-posta {{ $t('mail.tab_accounts') }}ı</h2>
            <button class="btn primary" @click="openCreateModal">Yeni E-posta Hesabı</button>
          </div>

          <div v-if="accountsLoading" class="muted">Yükleniyor…</div>
          <template v-else>
            <table v-if="accounts.length > 0">
              <thead>
                <tr>
                  <th>{{ $t('mail.col_email') }}</th>
                  <th>{{ $t('mail.col_quota') }}</th>
                  <th>Oluşturma Tarihi</th>
                  <th style="text-align: right">İşlemler</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="acc in accounts" :key="acc.id">
                  <td><strong>{{ acc.email }}</strong></td>
                  <td class="mono">{{ acc.quota > 0 ? acc.quota : 'Limitsiz' }}</td>
                  <td>{{ formatDate(acc.created_at) }}</td>
                  <td style="text-align: right">
                    <button class="btn btn-sm" @click="openPasswordModal(acc.email)">{{ $t('mail.btn_change_pw') }}</button>
                    <button class="btn danger btn-sm" style="margin-left: 6px" @click="deleteAccount(acc.email)">{{ $t('mail.btn_delete') }}</button>
                  </td>
                </tr>
              </tbody>
            </table>
            <div v-else class="muted">Bu site için henüz e-posta hesabı yok.</div>
          </template>
        </div>

        <!-- ── {{ $t('mail.tab_dns') }} Sekmesi ── -->
        <div v-if="activeTab === 'dns'" class="card">
          <h2 style="margin: 0 0 16px 0">{{ $t('mail.tab_dns') }}</h2>
          <p class="muted" style="margin-bottom: 16px">
            Mail sunucunuzun düzgün çalışması için aşağıdaki DNS kayıtlarını domain sağlayıcınızdan ekleyin.
          </p>

          <table>
            <thead>
              <tr>
                <th>Tür</th>
                <th>Ad</th>
                <th>{{ $t('mail.value') }}</th>
                <th style="width: 60px"></th>
              </tr>
            </thead>
            <tbody>
              <!-- MX -->
              <tr>
                <td><span class="badge" style="background: #7c3aed; color: #fff">MX</span></td>
                <td class="mono">@</td>
                <td class="mono">10 mail.{{ siteDomain }}</td>
                <td><button class="btn btn-sm" @click="copyText('10 mail.' + siteDomain)">{{ $t('mail.btn_copy') }}</button></td>
              </tr>
              <!-- SPF -->
              <tr>
                <td><span class="badge" style="background: #2563eb; color: #fff">TXT</span></td>
                <td class="mono">@</td>
                <td class="mono" style="word-break: break-all">v=spf1 ip4:{{ serverIP }} ~all</td>
                <td><button class="btn btn-sm" @click="copyText('v=spf1 ip4:' + serverIP + ' ~all')">{{ $t('mail.btn_copy') }}</button></td>
              </tr>
              <!-- DKIM -->
              <tr>
                <td><span class="badge" style="background: #2563eb; color: #fff">TXT</span></td>
                <td class="mono">mail._domainkey</td>
                <td v-if="dkimRecord" class="mono" style="word-break: break-all; max-width: 400px">v=DKIM1; k=rsa; p={{ dkimRecord }}</td>
                <td v-else>
                  <button class="btn btn-sm primary" :disabled="dkimBusy" @click="generateDKIM">
                    {{ dkimBusy ? 'Oluşturuluyor…' : 'DKIM Oluştur' }}
                  </button>
                </td>
                <td v-if="dkimRecord">
                  <button class="btn btn-sm" @click="copyText('v=DKIM1; k=rsa; p=' + dkimRecord)">{{ $t('mail.btn_copy') }}</button>
                </td>
              </tr>
              <!-- DMARC -->
              <tr>
                <td><span class="badge" style="background: #2563eb; color: #fff">TXT</span></td>
                <td class="mono">_dmarc</td>
                <td class="mono" style="word-break: break-all">v=DMARC1; p=none; rua=mailto:postmaster@{{ siteDomain }}</td>
                <td><button class="btn btn-sm" @click="copyText('v=DMARC1; p=none; rua=mailto:postmaster@' + siteDomain)">{{ $t('mail.btn_copy') }}</button></td>
              </tr>
              <!-- PTR -->
              <tr>
                <td><span class="badge" style="background: #64748b; color: #fff">PTR</span></td>
                <td class="mono">{{ serverIP }}</td>
                <td class="muted">PTR kaydını hosting sağlayıcınızın (Contabo vb.) kontrol panelinden ayarlayın.</td>
                <td></td>
              </tr>
            </tbody>
          </table>
        </div>

        <!-- ── {{ $t('mail.tab_webmail') }} Sekmesi ── -->
        <div v-if="activeTab === 'webmail'" class="card">
          <h2 style="margin: 0 0 16px 0">{{ $t('mail.tab_webmail') }}</h2>
          <p class="muted" style="margin-bottom: 16px">
            SnappyMail webmail arayüzünü kullanarak e-postalarınızı tarayıcınızdan yönetebilirsiniz.
            E-posta hesabınızla giriş yapabilirsiniz.
          </p>
          <a :href="webmailURL" target="_blank" class="btn primary">{{ $t('mail.tab_webmail') }}'i Aç ↗</a>
        </div>
      </template>

      <!-- Hesap Oluşturma Modalı -->
      <div v-if="showCreateModal" class="modal-backdrop" @click.self="showCreateModal = false">
        <div class="modal-card">
          <h2>{{ $t('mail.modal_new_account') }}</h2>
          <form @submit.prevent="createAccount">
            <div style="display: flex; gap: 8px; margin-bottom: 16px">
              <div style="flex: 1">
                <label>Kullanıcı adı</label>
                <input v-model="form.local_part" type="text" required placeholder="hello" />
              </div>
              <div style="padding-top: 32px; font-weight: 600">@</div>
              <div style="flex: 1">
                <label>Domain</label>
                <input :value="siteDomain" type="text" disabled style="background: rgba(0,0,0,0.05); color: #666; cursor: not-allowed" />
              </div>
            </div>
            <label>{{ $t('mail.label_pw') }}</label>
            <input v-model="form.password" type="password" required placeholder="Güçlü bir şifre girin" />
            <label style="margin-top: 16px">Kota (MB) — 0 = Limitsiz</label>
            <input v-model.number="form.quota_mb" type="number" min="0" />

            <div style="display: flex; justify-content: flex-end; gap: 8px; margin-top: 24px">
              <button type="button" class="btn" @click="showCreateModal = false">{{ $t('mail.btn_cancel') }}</button>
              <button type="submit" class="btn primary" :disabled="submitting">
                {{ submitting ? 'Ekleniyor…' : 'Hesap Oluştur' }}
              </button>
            </div>
          </form>
        </div>
      </div>

      <!-- {{ $t('mail.btn_change_pw') }}me Modalı -->
      <div v-if="showPasswordModal" class="modal-backdrop" @click.self="showPasswordModal = false">
        <div class="modal-card">
          <h2>{{ $t('mail.btn_change_pw') }}</h2>
          <p class="muted" style="margin-bottom: 16px">{{ selectedEmail }}</p>
          <form @submit.prevent="changePassword">
            <label>{{ $t('mail.label_new_pw') }}</label>
            <input v-model="passwordForm.password" type="password" required placeholder="Yeni şifre" />
            <label style="margin-top: 16px">Şifre Tekrar</label>
            <input v-model="passwordForm.password_confirm" type="password" required placeholder="Şifreyi tekrar girin" />

            <div v-if="passwordForm.password && passwordForm.password_confirm && passwordForm.password !== passwordForm.password_confirm" class="alert error" style="margin-top: 12px">
              Şifreler eşleşmiyor.
            </div>

            <div style="display: flex; justify-content: flex-end; gap: 8px; margin-top: 24px">
              <button type="button" class="btn" @click="showPasswordModal = false">{{ $t('mail.btn_cancel') }}</button>
              <button type="submit" class="btn primary" :disabled="submitting || passwordForm.password !== passwordForm.password_confirm">
                {{ submitting ? 'Kaydediliyor…' : 'Şifreyi Değiştir' }}
              </button>
            </div>
          </form>
        </div>
      </div>
    </div>
  </Layout>
</template>


<script setup>
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import Layout from '../components/Layout.vue'
import { api } from '../api'

const { t } = useI18n()

// ── Durum ──────────────────────────────────────────────────────────────────────
const notice = ref('')
const error = ref('')
const loading = ref(false)
const submitting = ref(false)

// Mail server status
const mailStatus = ref(null)
const statusLoading = ref(true)
const setupBusy = ref(false)
const serverIP = ref('')

// Sites
const sites = ref([])
const siteId = ref('')
const siteDomain = ref('')

// Accounts
const accounts = ref([])
const accountsLoading = ref(false)

// Tabs
  const tabs = [
    { id: 'accounts', label: '📧 ' + t('mail.tab_accounts') },
    { id: 'dns',      label: '📡 ' + t('mail.tab_dns') },
    { id: 'webmail',  label: '📬 ' + t('mail.tab_webmail') },
]
const activeTab = ref('accounts')

// DKIM
const dkimRecord = ref('')
const dkimBusy = ref(false)

// Modals
const showCreateModal = ref(false)
const showPasswordModal = ref(false)
const selectedEmail = ref('')

const form = ref({
  local_part: '',
  password: '',
  quota_mb: 512,
})

const passwordForm = ref({
  password: '',
  password_confirm: '',
})

// {{ $t('mail.tab_webmail') }}
const webmailURL = computed(() => '/snappymail/')

// ── Yardımcılar ────────────────────────────────────────────────────────────────

let noticeTimer = null
function flash(msg, isError = false) {
  if (noticeTimer) clearTimeout(noticeTimer)
  if (isError) {
    error.value = msg
    notice.value = ''
  } else {
    notice.value = msg
    error.value = ''
  }
  noticeTimer = setTimeout(() => { notice.value = ''; error.value = '' }, 5000)
}

function clear() {
  notice.value = ''
  error.value = ''
}

function badgeStyle(active) {
  return active
    ? 'background: #16a34a; color: #fff; padding: 4px 12px; border-radius: 6px; font-size: 13px'
    : 'background: #ea580c; color: #fff; padding: 4px 12px; border-radius: 6px; font-size: 13px'
}

function formatDate(d) {
  if (!d) return '—'
  return new Date(d).toLocaleDateString('tr-TR', { year: 'numeric', month: 'short', day: 'numeric' })
}

async function copyText(text) {
  try {
    await navigator.clipboard.writeText(text)
    flash('Panoya kopyalandı')
  } catch {
    flash('Kopyalama başarısız', true)
  }
}

// ── Mail Sunucu Durumu ─────────────────────────────────────────────────────────

async function loadStatus() {
  statusLoading.value = true
  try {
    const s = await api('/mail/status')
    mailStatus.value = s
    serverIP.value = s.server_ip || ''
  } catch (e) {
    mailStatus.value = { installed: false }
  } finally {
    statusLoading.value = false
  }
}

async function setupMail() {
  setupBusy.value = true
  clear()
  try {
    await api.post('/mail/setup')
    flash('Mail sunucusu başarıyla kuruldu')
    await loadStatus()
  } catch (e) {
    flash(e.message, true)
  } finally {
    setupBusy.value = false
  }
}

// ── Siteler & {{ $t('mail.tab_accounts') }} ─────────────────────────────────────────────────────────

async function loadSites() {
  try {
    sites.value = await api('/sites') || []
  } catch (e) {
    console.error('Site listesi alınamadı', e)
  }
}

async function onSiteChange() {
  const site = sites.value.find(s => String(s.id) === String(siteId.value))
  siteDomain.value = site ? site.name : ''
  accounts.value = []
  dkimRecord.value = ''
  activeTab.value = 'accounts'
  if (siteId.value) {
    await Promise.all([fetchAccounts(), fetchDKIM()])
  }
}

async function fetchAccounts() {
  accountsLoading.value = true
  try {
    accounts.value = await api(`/sites/${siteId.value}/mail`) || []
  } catch (e) {
    flash(e.message, true)
    accounts.value = []
  } finally {
    accountsLoading.value = false
  }
}

async function fetchDKIM() {
  if (!siteDomain.value) return
  try {
    const res = await api(`/sites/${siteId.value}/mail/dkim?domain=${encodeURIComponent(siteDomain.value)}`)
    dkimRecord.value = res.public_key || ''
  } catch {
    dkimRecord.value = ''
  }
}

// ── Hesap İşlemleri ────────────────────────────────────────────────────────────

function openCreateModal() {
  form.value = { local_part: '', password: '', quota_mb: 512 }
  showCreateModal.value = true
}

async function createAccount() {
  submitting.value = true
  clear()
  try {
    await api.post(`/sites/${siteId.value}/mail`, {
      domain: siteDomain.value,
      local_part: form.value.local_part,
      password: form.value.password,
      quota_mb: form.value.quota_mb,
    })
    showCreateModal.value = false
    flash('E-posta hesabı oluşturuldu')
    await fetchAccounts()
  } catch (e) {
    flash(e.message, true)
  } finally {
    submitting.value = false
  }
}

async function deleteAccount(email) {
  if (!confirm(`"${email}" hesabını silmek istediğinize emin misiniz?`)) return
  clear()
  try {
    await api.delete(`/sites/${siteId.value}/mail/${encodeURIComponent(email)}`)
    flash('Hesap silindi')
    await fetchAccounts()
  } catch (e) {
    flash(e.message, true)
  }
}

function openPasswordModal(email) {
  selectedEmail.value = email
  passwordForm.value = { password: '', password_confirm: '' }
  showPasswordModal.value = true
}

async function changePassword() {
  if (passwordForm.value.password !== passwordForm.value.password_confirm) return
  submitting.value = true
  clear()
  try {
    await api.put(`/sites/${siteId.value}/mail/${encodeURIComponent(selectedEmail.value)}/password`, {
      password: passwordForm.value.password,
    })
    showPasswordModal.value = false
    flash('Şifre başarıyla değiştirildi')
  } catch (e) {
    flash(e.message, true)
  } finally {
    submitting.value = false
  }
}

// ── DKIM ───────────────────────────────────────────────────────────────────────

async function generateDKIM() {
  dkimBusy.value = true
  clear()
  try {
    const res = await api.post(`/sites/${siteId.value}/mail/dkim`, { domain: siteDomain.value })
    dkimRecord.value = res.public_key || ''
    flash('DKIM anahtarı oluşturuldu')
  } catch (e) {
    flash(e.message, true)
  } finally {
    dkimBusy.value = false
  }
}

// ── Mount ──────────────────────────────────────────────────────────────────────

onMounted(async () => {
  await Promise.all([loadStatus(), loadSites()])
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
