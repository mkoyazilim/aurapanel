<template>
  <div class="min-h-screen bg-panel-darker flex items-center justify-center p-4">
    <!-- Login Container -->
    <div class="w-full max-w-md">
      <!-- Logo / Title -->
      <div class="text-center mb-8">
        <div class="inline-flex items-center justify-center w-16 h-16 rounded-2xl bg-gradient-to-tr from-brand-600 to-brand-400 mb-4 shadow-lg shadow-brand-500/20">
          <Activity class="w-8 h-8 text-white" />
        </div>
        <h1 class="text-3xl font-bold text-white tracking-wide">Aura<span class="text-brand-400">Panel</span></h1>
        <p class="text-gray-400 mt-2">Web Hosting Control Panel</p>
      </div>

      <!-- Login Card -->
      <div class="aura-card p-8">
        <h2 class="text-xl font-semibold text-white mb-6">Sisteme Giriş Yapın</h2>
        
        <form @submit.prevent="handleLogin" class="space-y-4">
          <div v-if="errorMsg" class="p-3 bg-red-500/10 border border-red-500/20 rounded-lg text-red-400 text-sm flex items-center gap-2">
            <AlertCircle class="w-4 h-4" />
            {{ errorMsg }}
          </div>

          <div class="space-y-1">
            <label class="text-sm font-medium text-gray-300">E-Posta / Kullanıcı Adı</label>
            <div class="relative">
              <User class="w-5 h-5 text-gray-500 absolute left-3 top-1/2 -translate-y-1/2" />
              <input v-model="email" type="text" class="aura-input pl-10" placeholder="admin@domain.com" required />
            </div>
          </div>

          <div class="space-y-1">
            <div class="flex items-center justify-between">
              <label class="text-sm font-medium text-gray-300">Şifre</label>
              <a href="#" class="text-sm text-brand-400 hover:text-brand-300">Şifremi Unuttum?</a>
            </div>
            <div class="relative">
              <KeyRound class="w-5 h-5 text-gray-500 absolute left-3 top-1/2 -translate-y-1/2" />
              <input v-model="password" type="password" class="aura-input pl-10" placeholder="••••••••" required />
            </div>
          </div>

          <div class="pt-2">
            <button type="submit" class="w-full btn-primary justify-center py-2.5 text-lg" :disabled="loading">
              <Loader2 v-if="loading" class="w-5 h-5 animate-spin" />
              <LogOut v-else class="w-5 h-5 rotate-180" />
              {{ loading ? 'Giriş Yapılıyor...' : 'Giriş Yap' }}
            </button>
          </div>
        </form>
      </div>

      <p class="text-center text-xs text-gray-500 mt-8">
        AuraPanel v0.1.0 &bull; Secure AI-SRE Protected &bull; Open Source
      </p>
    </div>
  </div>
</template>

<script setup>
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import { Activity, User, KeyRound, LogOut, AlertCircle, Loader2 } from 'lucide-vue-next'

const router = useRouter()
const authStore = useAuthStore()

const email = ref('admin@server.com')
const password = ref('password123')
const errorMsg = ref('')
const loading = ref(false)

const handleLogin = async () => {
  errorMsg.value = ''
  loading.value = true
  
  try {
    await authStore.login(email.value, password.value)
    router.push('/')
  } catch (err) {
    errorMsg.value = err.message || 'Giriş yapılamadı.'
  } finally {
    loading.value = false
  }
}
</script>
