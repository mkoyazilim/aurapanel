<template>
  <Layout>
    <div class="page">
      <h1>{{ $t('menu.settings') }}</h1>
      <div v-if="error" class="alert error">{{ error }}</div>
      <div v-if="notice" class="alert ok">{{ notice }}</div>

      <div class="card">
        <h2>{{ $t('settings.change_password') }}</h2>
        <div v-if="auth.user?.must_change_password" class="alert warn">
          ⚠️ {{ $t('settings.must_change_pw') }}
        </div>
        <label>{{ $t('settings.current_pw') }}</label>
        <input v-model="oldPw" type="password" />
        <label>{{ $t('settings.new_pw') }}</label>
        <input v-model="newPw" type="password" />
        <button class="btn primary" style="margin-top: 10px" @click="changePw">{{ $t('settings.change') }}</button>
      </div>

      <div class="card">
        <h2>{{ $t('settings.two_factor') }}</h2>
        <template v-if="!me.totp_enabled">
          <button class="btn" @click="mfaStart">{{ $t('settings.setup_totp') }}</button>
          <div v-if="mfaSecret">
            <div class="alert warn">{{ $t('settings.totp_secret_desc') }}</div>
            <div class="mono" style="padding: 8px; background: #f8fafc; border-radius: 8px; word-break: break-all">
              {{ mfaSecret }}
            </div>
            <div class="muted" style="margin: 8px 0">otpauth URL: <span class="mono">{{ mfaUrl }}</span></div>
            <label>{{ $t('settings.verify_code') }}</label>
            <input v-model="mfaCode" inputmode="numeric" maxlength="6" />
            <button class="btn primary" style="margin-top: 10px" @click="mfaEnable">{{ $t('settings.verify_enable') }}</button>
          </div>
        </template>
        <template v-else>
          <span class="badge ok">{{ $t('settings.totp_enabled') }}</span>
          <button class="btn danger" style="margin-left: 10px" @click="mfaDisable">{{ $t('settings.turn_off') }}</button>
        </template>
      </div>

      <div class="card">
        <h2>{{ $t('settings.security') }}</h2>
        <div class="row">
          <div style="flex: 1">
            <label>{{ $t('settings.provider') }}</label>
            <select v-model="captchaProvider">
              <option value="">{{ $t('settings.provider_disabled') }}</option>
              <option value="turnstile">Cloudflare Turnstile</option>
              <option value="recaptcha">Google reCAPTCHA (v2/v3)</option>
            </select>
          </div>
        </div>
        <template v-if="captchaProvider">
          <div class="row" style="margin-top: 10px;">
            <div style="flex: 1">
              <label>{{ $t('settings.site_key') }}</label>
              <input v-model="captchaSiteKey" placeholder="e.g. 0x4AAAA..." />
            </div>
            <div style="flex: 1">
              <label>{{ $t('settings.secret_key') }}</label>
              <input v-model="captchaSecret" type="password" :placeholder="$t('settings.secret_placeholder')" />
            </div>
          </div>
        </template>
        <button class="btn primary" style="margin-top: 14px" @click="saveCaptchaSettings">{{ $t('settings.save_settings') }}</button>
      </div>

      <div class="card">
        <h2>{{ $t('settings.pat') }}</h2>
        <div class="row">
          <input v-model="patName" :placeholder="$t('settings.token_name')" style="flex: 1" />
          <button class="btn primary" @click="patCreate">{{ $t('settings.create') }}</button>
        </div>
        <div v-if="newToken" class="alert warn mono">
          {{ $t('settings.new_token_warning') }} {{ newToken }}
        </div>
        <table style="margin-top: 12px">
          <thead><tr><th>{{ $t('settings.name') }}</th><th>{{ $t('settings.created_at') }}</th><th>{{ $t('settings.last_used') }}</th><th></th></tr></thead>
          <tbody>
            <tr v-for="p in pats" :key="p.id">
              <td>{{ p.name }}</td>
              <td class="muted">{{ p.created_at }}</td>
              <td class="muted">{{ p.last_used_at || '—' }}</td>
              <td><button class="btn danger" @click="patDelete(p.id)">{{ $t('common.delete') }}</button></td>
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
