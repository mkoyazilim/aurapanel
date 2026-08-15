
<template>
  <Layout>
    <div class="page">
      <div style="margin-bottom: 20px">
        <h1 style="margin: 0">{{ $t('menu.settings') }}</h1>
      </div>

      <div v-if="error" class="alert error">{{ error }}</div>
      <div v-if="notice" class="alert ok">{{ notice }}</div>

      <div class="settings-grid">
        <!-- Sol Sütun: Güvenlik & Hesap -->
        <div class="settings-col">
          <!-- Şifre Değiştir -->
          <div class="card">
            <h2>🔑 {{ $t('settings.change_password') }}</h2>
            <div v-if="auth.user?.must_change_password" class="alert warn">
              ⚠️ {{ $t('settings.must_change_pw') }}
            </div>
            <div style="margin-top: 10px">
              <label>{{ $t('settings.current_pw') }}</label>
              <input v-model="oldPw" type="password" />
            </div>
            <div style="margin-top: 10px">
              <label>{{ $t('settings.new_pw') }}</label>
              <input v-model="newPw" type="password" />
            </div>
            <button class="btn primary" style="margin-top: 14px" @click="changePw">{{ $t('settings.change') }}</button>
          </div>

          <!-- TOTP 2FA -->
          <div class="card">
            <h2>🛡️ {{ $t('settings.two_factor') }}</h2>
            <template v-if="!me.totp_enabled">
              <p class="muted text-sm" style="margin-bottom: 12px">{{ $t('settings.totp_description') }}</p>
              <button class="btn" @click="mfaStart">{{ $t('settings.setup_totp') }}</button>
              <div v-if="mfaSecret" style="margin-top: 12px">
                <div class="alert warn">{{ $t('settings.totp_secret_desc') }}</div>
                <div class="mono" style="padding: 8px; background: rgba(0,0,0,0.04); border-radius: 8px; word-break: break-all">
                  {{ mfaSecret }}
                </div>
                <div class="muted text-sm" style="margin: 8px 0">{{ $t('settings.otpauth_url_label') }} <span class="mono">{{ mfaUrl }}</span></div>
                <label>{{ $t('settings.verify_code') }}</label>
                <input v-model="mfaCode" inputmode="numeric" maxlength="6" />
                <button class="btn primary" style="margin-top: 10px" @click="mfaEnable">{{ $t('settings.verify_enable') }}</button>
              </div>
            </template>
            <template v-else>
              <div style="display: flex; align-items: center; justify-content: space-between; margin-top: 8px">
                <span class="badge ok">✓ {{ $t('settings.totp_enabled') }}</span>
                <button class="btn danger btn-sm" @click="mfaDisable">{{ $t('settings.turn_off') }}</button>
              </div>
            </template>
          </div>

          <!-- Bot Koruması -->
          <div class="card">
            <h2>🤖 {{ $t('settings.security') }}</h2>
            <div>
              <label>{{ $t('settings.provider') }}</label>
              <select v-model="captchaProvider">
                <option value="">{{ $t('settings.provider_disabled') }}</option>
                <option value="turnstile">{{ $t('settings.captcha_turnstile') }}</option>
                <option value="recaptcha">{{ $t('settings.captcha_recaptcha') }}</option>
              </select>
            </div>
            <template v-if="captchaProvider">
              <div style="margin-top: 10px">
                <label>{{ $t('settings.site_key') }}</label>
                <input v-model="captchaSiteKey" :placeholder="$t('settings.site_key_placeholder')" />
              </div>
              <div style="margin-top: 10px">
                <label>{{ $t('settings.secret_key') }}</label>
                <input v-model="captchaSecret" type="password" :placeholder="$t('settings.secret_placeholder')" />
              </div>
            </template>
            <button class="btn primary" style="margin-top: 14px" @click="saveCaptchaSettings">{{ $t('settings.save_settings') }}</button>
          </div>
        </div>

        <!-- Sağ Sütun: Uzak Depolama & API -->
        <div class="settings-col">
          <!-- S3 / Cloudflare R2 -->
          <div class="card">
            <h2>☁️ {{ $t('settings.s3_title') }}</h2>
            <p class="muted text-sm" style="margin-bottom: 12px">{{ $t('settings.s3_subtitle') }}</p>
            
            <div style="margin-bottom: 12px">
              <label style="margin: 0; display: flex; align-items: center; gap: 8px; cursor: pointer">
                <input type="checkbox" v-model="s3Form.enabled" style="width: auto" />
                <strong>{{ $t('settings.s3_enabled_label') }}</strong>
              </label>
            </div>

            <template v-if="s3Form.enabled">
              <div style="margin-top: 10px">
                <label>{{ $t('settings.s3_endpoint') }}</label>
                <input v-model="s3Form.endpoint" :placeholder="$t('settings.s3_endpoint_placeholder')" />
                <small class="muted" style="display: block; margin-top: 2px">{{ $t('settings.s3_providers_hint') }}</small>
              </div>

              <div class="row" style="margin-top: 10px">
                <div style="flex: 1">
                  <label>{{ $t('settings.s3_bucket') }}</label>
                  <input v-model="s3Form.bucket" :placeholder="$t('settings.s3_bucket_placeholder')" />
                </div>
                <div style="flex: 1">
                  <label>{{ $t('settings.s3_region') }}</label>
                  <input v-model="s3Form.region" :placeholder="$t('settings.s3_region_placeholder')" />
                </div>
              </div>

              <div style="margin-top: 10px">
                <label>{{ $t('settings.s3_access_key') }}</label>
                <input v-model="s3Form.access_key" :placeholder="$t('settings.s3_access_key_placeholder')" />
              </div>

              <div style="margin-top: 10px">
                <label>{{ $t('settings.s3_secret_key') }}</label>
                <input v-model="s3Form.secret_key" type="password" :placeholder="s3Form.has_secret ? $t('settings.s3_secret_existing') : $t('settings.s3_secret_key_placeholder')" />
              </div>
            </template>

            <div style="display: flex; gap: 10px; margin-top: 14px">
              <button class="btn primary" @click="saveS3Settings">{{ $t('settings.save_settings') }}</button>
              <button v-if="s3Form.enabled" class="btn" @click="testS3Connection" :disabled="testingS3">
                {{ testingS3 ? $t('settings.s3_testing') : '🔌 ' + $t('settings.s3_test_btn') }}
              </button>
            </div>
          </div>

          <!-- Personal Access Tokens (PAT) -->
          <div class="card">
            <h2>🔑 {{ $t('settings.pat') }}</h2>
            <div class="row">
              <input v-model="patName" :placeholder="$t('settings.token_name')" style="flex: 1" />
              <button class="btn primary" @click="patCreate">{{ $t('settings.create') }}</button>
            </div>
            <div v-if="newToken" class="alert warn mono" style="margin-top: 10px">
              {{ $t('settings.new_token_warning') }} {{ newToken }}
            </div>
            <table style="margin-top: 12px">
              <thead><tr><th>{{ $t('settings.name') }}</th><th>{{ $t('settings.created_at') }}</th><th>{{ $t('settings.last_used') }}</th><th></th></tr></thead>
              <tbody>
                <tr v-for="p in pats" :key="p.id">
                  <td><strong>{{ p.name }}</strong></td>
                  <td class="muted">{{ p.created_at }}</td>
                  <td class="muted">{{ p.last_used_at || '—' }}</td>
                  <td style="text-align: right"><button class="btn danger btn-sm" @click="patDelete(p.id)">{{ $t('common.delete') }}</button></td>
                </tr>
                <tr v-if="!pats.length"><td colspan="4" class="muted">{{ $t('settings.no_pats') }}</td></tr>
              </tbody>
            </table>
          </div>
        </div>
      </div>
    </div>
  </Layout>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import Layout from '../components/Layout.vue'
import { api } from '../api'
import { useAuth } from '../stores/auth'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()
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
    notice.value = t('settings.pw_changed')
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
    notice.value = t('settings.totp_setup_success')
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
    notice.value = t('settings.totp_disabled')
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
    notice.value = t('settings.settings_saved')
  } catch (e) {
    error.value = e.message
  }
}

const s3Form = ref({
  endpoint: '',
  bucket: '',
  region: 'auto',
  access_key: '',
  secret_key: '',
  enabled: false,
  has_secret: false
})
const testingS3 = ref(false)

async function loadS3Settings() {
  try {
    const res = await fetch('/api/v1/settings/s3')
    if (res.ok) {
      const data = await res.json()
      s3Form.value = {
        endpoint: data.endpoint || '',
        bucket: data.bucket || '',
        region: data.region || 'auto',
        access_key: data.access_key || '',
        secret_key: '',
        enabled: !!data.enabled,
        has_secret: !!data.has_secret
      }
    }
  } catch (err) {
    console.error('S3 settings read error:', err)
  }
}

async function saveS3Settings() {
  error.value = ''
  notice.value = ''
  try {
    const res = await fetch('/api/v1/settings/s3', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(s3Form.value)
    })
    if (res.ok) {
      notice.value = t('settings.s3_saved_success')
      await loadS3Settings()
    } else {
      const data = await res.json()
      error.value = data.error || t('settings.s3_save_failed')
    }
  } catch (e) {
    error.value = e.message
  }
}

async function testS3Connection() {
  error.value = ''
  notice.value = ''
  testingS3.value = true
  try {
    const res = await fetch('/api/v1/backups/s3/test', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(s3Form.value)
    })
    if (res.ok) {
      notice.value = t('settings.s3_test_success')
    } else {
      const data = await res.json()
      error.value = t('settings.s3_test_failed') + ': ' + (data.error || t('settings.s3_connection_failed'))
    }
  } catch (e) {
    error.value = e.message
  } finally {
    testingS3.value = false
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
    console.error('Settings read error', e)
  }
  await loadS3Settings()
  await loadPats()
})
</script>

<style scoped>
.settings-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 20px;
  align-items: start;
}

.settings-col {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.settings-col .card {
  margin-bottom: 0;
}

@media (max-width: 1024px) {
  .settings-grid {
    grid-template-columns: 1fr;
  }
}
</style>
