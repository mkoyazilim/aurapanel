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
      <button class="btn primary" style="margin-top: 16px; width: 100%" :disabled="busy">
        {{ busy ? 'Giriş yapılıyor…' : 'Giriş Yap' }}
      </button>
    </form>
  </div>
</template>

<script setup>
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { useAuth } from '../stores/auth'

const auth = useAuth()
const router = useRouter()
const username = ref('')
const password = ref('')
const totp = ref('')
const error = ref('')
const busy = ref(false)

async function submit() {
  busy.value = true
  error.value = ''
  try {
    await auth.login(username.value, password.value, totp.value)
    router.push('/')
  } catch (e) {
    error.value = e.message
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
