import { defineStore } from 'pinia'
import api from '../services/api'
import i18n from '../i18n'
import { useNotificationStore } from './notifications'
import { normalizePermissions, normalizeRole } from '../security/rbac'

const USER_KEY = 'aura_user'
const PERSIST_KEY = 'aura_persist'

// Token is no longer stored in localStorage/sessionStorage.
// Authentication relies on the HttpOnly cookie set by the API gateway
// (SameSite=Lax, Secure on HTTPS). This prevents token theft via XSS.

function clearStoredAuth() {
  try {
    localStorage.removeItem(USER_KEY)
    localStorage.removeItem(PERSIST_KEY)
  } catch {
    // no-op
  }
  try {
    sessionStorage.removeItem(USER_KEY)
  } catch {
    // no-op
  }
}

function normalizeUserPayload(raw) {
  if (!raw || typeof raw !== 'object') return null
  const email = typeof raw.email === 'string' ? raw.email : ''
  const username = raw.username || raw.name || (email.includes('@') ? email.split('@')[0] : '')
  const permissions = normalizePermissions(raw.permissions)
  return {
    ...raw,
    username,
    name: raw.name || username,
    permissions,
  }
}

function getInitialUser() {
  let userRaw = null
  let persistent = false

  try {
    userRaw = localStorage.getItem(USER_KEY)
    persistent = localStorage.getItem(PERSIST_KEY) === '1'
  } catch {
    // no-op
  }

  try {
    userRaw = userRaw || sessionStorage.getItem(USER_KEY)
  } catch {
    // no-op
  }

  let user = null
  if (userRaw) {
    try {
      user = normalizeUserPayload(JSON.parse(userRaw))
    } catch {
      clearStoredAuth()
      return { user: null, persistent: false }
    }
  }

  return { user, persistent }
}

const initial = getInitialUser()

export const useAuthStore = defineStore('auth', {
  state: () => ({
    // Session is authenticated when we have a user object (restored from /auth/me via cookie).
    // No token stored client-side — the HttpOnly cookie handles auth transparently.
    user: initial.user,
    persistent: initial.persistent,
    sessionChecked: false, // true after first /auth/me check on page load
  }),
  getters: {
    isAuthenticated: (state) => !!state.user,
    role: (state) => normalizeRole(state.user?.role),
    permissions: (state) => normalizePermissions(state.user?.permissions),
    isAdmin: (state) => normalizeRole(state.user?.role) === 'admin',
    isReseller: (state) => normalizeRole(state.user?.role) === 'reseller',
    isUser: (state) => normalizeRole(state.user?.role) === 'user',
  },
  actions: {
    // Check if we have a valid session via the HttpOnly cookie.
    // Called once on app mount to restore session from cookie.
    async checkSession() {
      if (this.sessionChecked) return !!this.user
      this.sessionChecked = true
      try {
        const response = await api.get('/auth/me', {
          headers: { 'X-Aura-Silent-Error': '1', 'X-Aura-Silent-Loading': '1' },
        })
        const user = normalizeUserPayload(response.data?.data || response.data?.user || response.data || null)
        if (user) {
          this.user = user
          const target = this.persistent ? localStorage : sessionStorage
          target.setItem(USER_KEY, JSON.stringify(user))
          return true
        }
      } catch {
        // Cookie invalid or expired — stay logged out
      }
      this.user = null
      return false
    },
    async login(email, password, remember = false, totpToken = '') {
      try {
        const response = await api.post('/auth/login', {
          email,
          password,
          totp_token: totpToken || undefined,
        })
        // Token is now only in the HttpOnly cookie (set by gateway).
        // We store only user metadata client-side.
        this.user = normalizeUserPayload(response.data.user)
        this.persistent = !!remember
        this.sessionChecked = true

        clearStoredAuth()
        if (this.user) {
          const target = this.persistent ? localStorage : sessionStorage
          target.setItem(USER_KEY, JSON.stringify(this.user))
          if (this.persistent) {
            localStorage.setItem(PERSIST_KEY, '1')
          }
        }

        const notificationStore = useNotificationStore()
        notificationStore.add({
          title: i18n.global.t('auth_messages.welcome_title'),
          message: i18n.global.t('auth_messages.welcome_message', { email: response.data.user?.email || email }),
          type: 'success',
          source: 'auth',
        })
        return true
      } catch (error) {
        console.error('Login Error', error)
        const err = new Error(error.response?.data?.message || i18n.global.t('auth_messages.login_error'))
        err.requires2fa = !!error.response?.data?.requires_2fa
        throw err
      }
    },
    async refreshUserFromServer() {
      if (!this.user) return null
      try {
        const response = await api.get('/auth/me', {
          headers: { 'X-Aura-Silent-Error': '1' },
        })
        const user = normalizeUserPayload(response.data?.data || response.data?.user || response.data || null)
        if (!user) return null
        this.user = user
        const target = this.persistent ? localStorage : sessionStorage
        target.setItem(USER_KEY, JSON.stringify(user))
        return user
      } catch {
        this.logout()
        return null
      }
    },
    logout() {
      const hadSession = !!this.user
      this.user = null
      this.persistent = false
      this.sessionChecked = true
      clearStoredAuth()
      if (hadSession) {
        const notificationStore = useNotificationStore()
        notificationStore.add({
          title: i18n.global.t('auth_messages.signed_out_title'),
          message: i18n.global.t('auth_messages.signed_out_message'),
          type: 'info',
          source: 'auth',
        })
      }
    },
    async secureLogout() {
      if (this.user) {
        try {
          await api.post('/auth/logout', {}, {
            headers: {
              'X-Aura-Silent-Error': '1',
            },
          })
        } catch (error) {
          console.warn('Secure logout notify failed', error)
        }
      }
      this.logout()
    },
    updateUser(patch) {
      if (!this.user) return
      const nextUser = { ...this.user, ...patch }
      this.user = nextUser
      const target = this.persistent ? localStorage : sessionStorage
      target.setItem(USER_KEY, JSON.stringify(nextUser))
    }
  }
})
