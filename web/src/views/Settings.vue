<template>
  <Layout>
    <div class="page">
      <h1>Ayarlar</h1>
      <div v-if="error" class="alert error">{{ error }}</div>
      <div v-if="notice" class="alert ok">{{ notice }}</div>

      <div class="card">
        <h2>Şifre Değiştir</h2>
        <div v-if="auth.user?.must_change_password" class="alert warn">
          ⚠️ İlk kurulum şifresi kullanılıyor — değiştirmek ZORUNLU.
        </div>
        <label>Mevcut Şifre</label>
        <input v-model="oldPw" type="password" />
        <label>Yeni Şifre (en az 12 karakter)</label>
        <input v-model="newPw" type="password" />
        <button class="btn primary" style="margin-top: 10px" @click="changePw">Değiştir</button>
      </div>

      <div class="card">
        <h2>İki Faktörlü Kimlik Doğrulama (TOTP)</h2>
        <template v-if="!me.totp_enabled">
          <button class="btn" @click="mfaStart">TOTP Kurulumunu Başlat</button>
          <div v-if="mfaSecret">
            <div class="alert warn">Aşağıdaki sırrı kimlik doğrulayıcına ekle (Google Authenticator, Aegis vb.):</div>
            <div class="mono" style="padding: 8px; background: #f8fafc; border-radius: 8px; word-break: break-all">
              {{ mfaSecret }}
            </div>
            <div class="muted" style="margin: 8px 0">otpauth URL: <span class="mono">{{ mfaUrl }}</span></div>
            <label>Doğrulama Kodu</label>
            <input v-model="mfaCode" inputmode="numeric" maxlength="6" />
            <button class="btn primary" style="margin-top: 10px" @click="mfaEnable">Doğrula ve Etkinleştir</button>
          </div>
        </template>
        <template v-else>
          <span class="badge ok">TOTP etkin</span>
          <button class="btn danger" style="margin-left: 10px" @click="mfaDisable">Kapat</button>
        </template>
      </div>

      <div class="card">
        <h2>Güvenlik (Bot Koruması)</h2>
        <div class="row">
          <div style="flex: 1">
            <label>Sağlayıcı (Provider)</label>
            <select v-model="captchaProvider">
              <option value="">Devre Dışı</option>
              <option value="turnstile">Cloudflare Turnstile</option>
              <option value="recaptcha">Google reCAPTCHA (v2/v3)</option>
            </select>
          </div>
        </div>
        <template v-if="captchaProvider">
          <div class="row" style="margin-top: 10px;">
            <div style="flex: 1">
              <label>Site Key (Genel Anahtar)</label>
              <input v-model="captchaSiteKey" placeholder="e.g. 0x4AAAA..." />
            </div>
            <div style="flex: 1">
              <label>Secret Key (Gizli Anahtar)</label>
              <input v-model="captchaSecret" type="password" placeholder="Sadece sunucu bilir..." />
            </div>
          </div>
        </template>
        <button class="btn primary" style="margin-top: 14px" @click="saveCaptchaSettings">Ayarları Kaydet</button>
      </div>

      <div class="card">
        <h2>Kişisel Erişim Token'ları (CLI/API)</h2>
        <div class="row">
          <input v-model="patName" placeholder="Token adı" style="flex: 1" />
          <button class="btn primary" @click="patCreate">Oluştur</button>
        </div>
        <div v-if="newToken" class="alert warn mono">
          YENİ TOKEN (yalnızca bir kez gösterilir): {{ newToken }}
        </div>
        <table style="margin-top: 12px">
          <thead><tr><th>Ad</th><th>Oluşturma</th><th>Son Kullanım</th><th></th></tr></thead>
          <tbody>
            <tr v-for="p in pats" :key="p.id">
              <td>{{ p.name }}</td>
              <td class="muted">{{ p.created_at }}</td>
              <td class="muted">{{ p.last_used_at || '—' }}</td>
              <td><button class="btn danger" @click="patDelete(p.id)">Sil</button></td>
            </tr>
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
import { useAuth } from '../stores/auth'

const auth = useAuth()
const me = ref({})
const oldPw = ref('')
const newPw = ref('')
const mfaSecret = ref('')
const mfaUrl = ref('')
const mfaCode = ref('')
const patName = ref('')
const pats = ref([])
const newToken = ref('')
const error = ref('')
const notice = ref('')

const captchaProvider = ref('')
const captchaSiteKey = ref('')
const captchaSecret = ref('')

async function changePw() {
  error.value = ''
  notice.value = ''
  try {
    await api('/auth/change-password', {
      method: 'POST',
      body: { old_password: oldPw.value, new_password: newPw.value },
    })
    notice.value = 'Şifre değiştirildi.'
    oldPw.value = ''
    newPw.value = ''
    await auth.me()
  } catch (e) {
    error.value = e.message
  }
}

async function mfaStart() {
  try {
    const out = await api('/auth/mfa/start')
    mfaSecret.value = out.secret
    mfaUrl.value = out.otpauth_url
  } catch (e) {
    error.value = e.message
  }
}

async function mfaEnable() {
  try {
    await api('/auth/mfa/enable', {
      method: 'POST',
      body: { secret: mfaSecret.value, code: mfaCode.value },
    })
    notice.value = 'TOTP etkinleştirildi.'
    mfaSecret.value = ''
    await auth.me()
    me.value = await api('/auth/me')
  } catch (e) {
    error.value = e.message
  }
}

async function mfaDisable() {
  try {
    await api('/auth/mfa/disable', { method: 'POST', body: {} })
    notice.value = 'TOTP kapatıldı.'
    me.value = await api('/auth/me')
  } catch (e) {
    error.value = e.message
  }
}

async function saveCaptchaSettings() {
  error.value = ''
  notice.value = ''
  try {
    await api('/settings', {
      method: 'POST',
      body: {
        captcha_provider: captchaProvider.value,
        captcha_sitekey: captchaSiteKey.value,
        captcha_secret: captchaSecret.value,
      },
    })
    notice.value = 'Bot koruması ayarları başarıyla kaydedildi.'
  } catch (e) {
    error.value = e.message
  }
}

async function patCreate() {
  try {
    const out = await api('/auth/pat', { method: 'POST', body: { name: patName.value } })
    newToken.value = out.token
    await loadPats()
  } catch (e) {
    error.value = e.message
  }
}

async function patDelete(id) {
  try {
    await api(`/auth/pat/${id}`, { method: 'DELETE' })
    await loadPats()
  } catch (e) {
    error.value = e.message
  }
}

async function loadPats() {
  pats.value = await api('/auth/pat').catch(() => [])
}

onMounted(async () => {
  me.value = await api('/auth/me').catch(() => ({}))
  try {
    const s = await api('/settings')
    captchaProvider.value = s.captcha_provider || ''
    captchaSiteKey.value = s.captcha_sitekey || ''
    captchaSecret.value = s.captcha_secret || ''
  } catch (e) {
    console.error('Ayarlar okunamadı', e)
  }
  await loadPats()
})
</script>
