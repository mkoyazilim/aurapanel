<template>
  <div class="min-h-screen bg-panel-darker flex items-center justify-center p-4">
    <div class="w-full max-w-md">
      <div class="mb-4 flex justify-end">
        <LanguageSwitcher />
      </div>

      <div class="text-center mb-8">
        <div class="flex justify-center mb-4">
          <img
            src="/aurapanel-logo.png"
            alt="AuraPanel Logo"
            class="h-[64px] w-auto max-w-[370px] object-contain drop-shadow-[0_10px_20px_rgba(0,0,0,0.35)]"
          />
        </div>
      </div>

      <div class="aura-card p-8">
        <h2 class="text-xl font-semibold text-white">{{ t('login.title') }}</h2>
        <p class="mt-2 mb-6 text-sm leading-6 text-slate-300">
          {{ t('login.subtitle') }}
        </p>

        <form @submit.prevent="handleLogin" class="space-y-4">
          <div v-if="errorMsg" class="p-3 bg-red-500/10 border border-red-500/20 rounded-lg text-red-400 text-sm flex items-center gap-2">
            <AlertCircle class="w-4 h-4" />
            {{ errorMsg }}
          </div>

          <div class="space-y-1">
            <label class="text-sm font-medium text-gray-300">{{ t('login.email_label') }}</label>
            <div class="relative">
              <User class="w-5 h-5 text-gray-500 absolute left-3 top-1/2 -translate-y-1/2" />
              <input v-model="email" type="email" inputmode="email" autocomplete="username" autocapitalize="none" spellcheck="false" class="aura-input pl-10" :placeholder="t('login.email_placeholder')" required />
            </div>
          </div>

          <div class="space-y-1">
            <div class="flex items-center justify-between">
              <label class="text-sm font-medium text-gray-300">{{ t('login.password_label') }}</label>
            </div>
            <div class="relative">
              <KeyRound class="w-5 h-5 text-gray-500 absolute left-3 top-1/2 -translate-y-1/2" />
              <input v-model="password" type="password" autocomplete="current-password" class="aura-input pl-10" :placeholder="t('login.password_placeholder')" required />
            </div>
          </div>

          <div v-if="requires2fa" class="space-y-1">
            <label class="text-sm font-medium text-gray-300">{{ t('login.twofa_label') }}</label>
            <div class="relative">
              <KeyRound class="w-5 h-5 text-gray-500 absolute left-3 top-1/2 -translate-y-1/2" />
              <input v-model="totpToken" type="text" inputmode="numeric" class="aura-input pl-10" :placeholder="t('login.twofa_placeholder')" maxlength="6" required />
            </div>
          </div>

          <label class="inline-flex items-center gap-2 text-sm text-gray-300">
            <input v-model="rememberMe" type="checkbox" class="w-4 h-4 rounded border-panel-border bg-panel-hover" />
            {{ t('login.remember_me') }}
          </label>

          <div class="pt-2">
            <button type="submit" class="w-full btn-primary justify-center py-2.5 text-lg" :disabled="loading">
              <Loader2 v-if="loading" class="w-5 h-5 animate-spin" />
              <LogOut v-else class="w-5 h-5 rotate-180" />
              {{ loading ? t('login.submitting') : t('login.submit') }}
            </button>
          </div>

          <div class="rounded-lg border border-sky-500/20 bg-sky-500/10 px-4 py-3 text-sm text-sky-100">
            {{ t('login.hint') }}
          </div>

          <div v-if="showDemoQuickAccess" class="rounded-lg border border-emerald-500/30 bg-emerald-500/10 px-4 py-3">
            <p class="text-sm font-semibold text-emerald-200">Demo Quick Access</p>
            <p class="mt-1 text-xs text-emerald-100/90">
              One-click login for role preview on demo environment.
            </p>
            <div class="mt-3 grid grid-cols-1 gap-2 sm:grid-cols-3">
              <button
                v-for="profile in demoProfiles"
                :key="profile.key"
                type="button"
                class="rounded-md border border-emerald-400/40 bg-emerald-500/20 px-3 py-2 text-xs font-semibold text-emerald-100 hover:bg-emerald-500/30 disabled:cursor-not-allowed disabled:opacity-60"
                :disabled="loading"
                @click="quickDemoLogin(profile)"
              >
                {{ profile.label }}
              </button>
            </div>
          </div>
        </form>
      </div>

      <p class="text-center text-xs text-gray-500 mt-8">
        {{ t('login.footer_tagline') }}
      </p>
      <p class="text-center text-xs text-gray-600 mt-2">
        {{ t('login.footer_credit') }}
      </p>
    </div>
  </div>
</template>

<script setup>
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '../stores/auth'
import { User, KeyRound, LogOut, AlertCircle, Loader2 } from 'lucide-vue-next'
import LanguageSwitcher from '../components/LanguageSwitcher.vue'

const router = useRouter()
const authStore = useAuthStore()
const { t } = useI18n({ useScope: 'global' })

const email = ref('')
const password = ref('')
const rememberMe = ref(false)
const errorMsg = ref('')
const loading = ref(false)
const requires2fa = ref(false)
const totpToken = ref('')
const showDemoQuickAccess = typeof window !== 'undefined' && window.location.hostname.toLowerCase() === 'demo.aurapanel.info'
const demoProfiles = [
  { key: 'admin', label: 'Admin Panel', email: 'demo@aurapanel.info', password: '1234567' },
  { key: 'reseller', label: 'Reseller Panel', email: 'demo.reseller@aurapanel.info', password: '1234567' },
  { key: 'user', label: 'User Panel', email: 'demo.user@aurapanel.info', password: '1234567' },
]

const handleLogin = async () => {
  errorMsg.value = ''
  loading.value = true

  try {
    await authStore.login(email.value, password.value, rememberMe.value, totpToken.value)
    router.push('/')
  } catch (err) {
    errorMsg.value = err.message || t('login.error_default')
    if (err.requires2fa) {
      requires2fa.value = true
    }
  } finally {
    loading.value = false
  }
}

const quickDemoLogin = async (profile) => {
  if (!profile || loading.value) {
    return
  }

  requires2fa.value = false
  totpToken.value = ''
  rememberMe.value = false
  email.value = profile.email
  password.value = profile.password
  await handleLogin()
}
</script>
