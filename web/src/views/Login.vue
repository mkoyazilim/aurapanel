<template>
  <div class="login-wrap">
    <form class="card login-card" @submit.prevent="submit">
      <div class="brand"><img src="/logo.png" alt="AuraPanel" height="40" /></div>
      <p class="muted">OpenLiteSpeed için hafif, güvenlik öncelikli panel</p>
      <div v-if="error" class="alert error">{{ error }}</div>
      <label>Kullanıcı adı</label>
      <input v-model="username" autocomplete="username" required />
      <label>Şifre</label>
      <input v-model="password" type="password" autocomplete="current-password" required />
      <label>TOTP kodu (2FA etkinse)</label>
      <input v-model="totp" inputmode="numeric" placeholder="6 haneli kod" />
      
      <div v-if="captchaSiteKey" style="margin-top: 16px;">
        <div v-if="captchaProvider === 'turnstile'" class="cf-turnstile" :data-sitekey="captchaSiteKey"></div>
        <div v-else-if="captchaProvider === 'recaptcha'" class="g-recaptcha" :data-sitekey="captchaSiteKey"></div>
      </div>

      <button class="btn primary" style="margin-top: 16px; width: 100%" :disabled="busy">
        {{ busy ? 'Giriş yapılıyor…' : 'Giriş Yap' }}
      </button>
    </form>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useAuth } from '../stores/auth'
import { api } from '../api'

const auth = useAuth()
const router = useRouter()
const username = ref('')
const password = ref('')
const totp = ref('')
const error = ref('')
const busy = ref(false)

const captchaProvider = ref('')
const captchaSiteKey = ref('')

onMounted(async () => {
  try {
    const s = await api('/settings/public')
    captchaProvider.value = s.captcha_provider || ''
    captchaSiteKey.value = s.captcha_sitekey || ''

    if (captchaSiteKey.value) {
      const script = document.createElement('script')
      script.async = true
      script.defer = true
      if (captchaProvider.value === 'turnstile') {
        script.src = 'https://challenges.cloudflare.com/turnstile/v0/api.js'
      } else if (captchaProvider.value === 'recaptcha') {
        script.src = 'https://www.google.com/recaptcha/api.js'
      }
      document.head.appendChild(script)
    }
  } catch (e) {
    // skip
  }
})

async function submit() {
  busy.value = true
  error.value = ''
  
  let captcha = ''
  if (captchaSiteKey.value) {
    const el = document.querySelector('[name=cf-turnstile-response]') || document.querySelector('[name=g-recaptcha-response]')
    if (el) captcha = el.value
  }

  try {
    await auth.login(username.value, password.value, totp.value, captcha)
    router.push('/')
  } catch (e) {
    error.value = e.message
    if (window.turnstile) window.turnstile.reset()
    if (window.grecaptcha) window.grecaptcha.reset()
  } finally {
    busy.value = false
  }
}
</script>

<style scoped>
.login-wrap {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
}
.login-card { width: 360px; padding: 28px; }
.brand { font-size: 22px; font-weight: 700; margin-bottom: 6px; }
</style>
