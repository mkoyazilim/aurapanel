<template>
  <div class="min-h-screen bg-panel-darker flex items-center justify-center p-4">
    <div class="w-full max-w-md">
      <div class="mb-4 flex justify-end gap-3">
        <ThemeSwitcher />
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
import ThemeSwitcher from '../components/ThemeSwitcher.vue'

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

</script>
