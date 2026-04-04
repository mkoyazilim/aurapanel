import { defineStore } from 'pinia'
import api from '../services/api'
import i18n from '../i18n'
import { useNotificationStore } from './notifications'
import { normalizePermissions, normalizeRole } from '../security/rbac'

const TOKEN_KEY = 'aura_token'
const USER_KEY = 'aura_user'
const PERSIST_KEY = 'aura_persist'

function decodeJwtPayload(token) {
  try {
    const parts = String(token || '').split('.')
    if (parts.length < 2) return null
    const base64 = parts[1].replace(/-/g, '+').replace(/_/g, '/')
    const padded = base64 + '='.repeat((4 - (base64.length % 4)) % 4)
    return JSON.parse(atob(padded))
  } catch {
    return null
  }
}

function isTokenExpired(token) {
  const payload = decodeJwtPayload(token)
  if (!payload?.exp) return false
  const exp = Number(payload.exp)
  if (!Number.isFinite(exp)) return false
  return exp * 1000 <= Date.now()
}

function clearStoredAuth() {
  try {
    localStorage.removeItem(TOKEN_KEY)
    localStorage.removeItem(USER_KEY)
    localStorage.removeItem(PERSIST_KEY)
  } catch {
    // no-op
  }
  try {
    sessionStorage.removeItem(TOKEN_KEY)
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

function getInitialAuth() {
  let localToken = null
  let sessionToken = null
  let userRaw = null
  let persistent = false

  try {
    localToken = localStorage.getItem(TOKEN_KEY)
    userRaw = localStorage.getItem(USER_KEY) || userRaw
    persistent = localStorage.getItem(PERSIST_KEY) === '1'
  } catch {
    // no-op
  }

  try {
    sessionToken = sessionStorage.getItem(TOKEN_KEY)
    userRaw = userRaw || sessionStorage.getItem(USER_KEY)
  } catch {
    // no-op
  }

  const token = localToken || sessionToken || null
  let user = null
  if (userRaw) {
    try {
      user = normalizeUserPayload(JSON.parse(userRaw))
    } catch {
      clearStoredAuth()
      return { token: null, user: null, persistent: false }
    }
  }

  if (token && isTokenExpired(token)) {
    clearStoredAuth()
    return { token: null, user: null, persistent: false }
  }

  return { token, user, persistent }
}

const initialAuth = getInitialAuth()

export const useAuthStore = defineStore('auth', {
  state: () => ({
    token: initialAuth.token,
    user: initialAuth.user,
    persistent: initialAuth.persistent,
  }),
  getters: {
    isAuthenticated: (state) => !!state.token,
    role: (state) => normalizeRole(state.user?.role),
    permissions: (state) => normalizePermissions(state.user?.permissions),
    isAdmin: (state) => normalizeRole(state.user?.role) === 'admin',
    isReseller: (state) => normalizeRole(state.user?.role) === 'reseller',
    isUser: (state) => normalizeRole(state.user?.role) === 'user',
  },
  actions: {
    isTokenExpired(token = this.token) {
      if (!token) return true
      return isTokenExpired(token)
    },
    ensureValidSession() {
      if (this.token && this.isTokenExpired(this.token)) {
        this.logout()
        return false
      }
      return !!this.token
    },
    async login(email, password, remember = false, totpToken = '') {
      try {
        const response = await api.post('/auth/login', {
          email,
          password,
          totp_token: totpToken || undefined,
        })
        this.setAuth(response.data.token, response.data.user, remember)
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
      if (!this.token) return null
      try {
        const response = await api.get('/auth/me', {
          headers: { 'X-Aura-Silent-Error': '1' },
        })
        // Backward-compatible: support wrapped (`{data:{...}}`) and flat (`{...}`) responses.
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
    setAuth(token, user, persistent = false) {
      this.token = token
      this.user = normalizeUserPayload(user)
      this.persistent = !!persistent

      clearStoredAuth()
      const target = this.persistent ? localStorage : sessionStorage
      target.setItem(TOKEN_KEY, token)
      target.setItem(USER_KEY, JSON.stringify(this.user))
      if (this.persistent) {
        localStorage.setItem(PERSIST_KEY, '1')
      }
    },
    logout() {
      const hadSession = !!this.token
      this.token = null
      this.user = null
      this.persistent = false
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
      if (this.token) {
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
