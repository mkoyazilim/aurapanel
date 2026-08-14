import { defineStore } from 'pinia'
import { api } from '../api'

export const useAuth = defineStore('auth', {
  state: () => ({
    user: null,
    csrf: localStorage.getItem('ap_csrf') || '',
  }),
  actions: {
    async login(username, password, totp, captcha) {
      const out = await api('/auth/login', {
        method: 'POST',
        body: { username, password, totp: totp || '', captcha: captcha || '' },
      })
      this.csrf = out.csrf_token || ''
      localStorage.setItem('ap_csrf', this.csrf)
      this.user = { username: out.username, must_change_password: out.must_change_password }
    },
    async me() {
      this.user = await api('/auth/me')
    },
    async logout() {
      try {
        await api('/auth/logout', { method: 'POST' })
      } catch {
        /* sunucu kapanmış olabilir */
      }
      this.user = null
      localStorage.removeItem('ap_csrf')
      this.csrf = ''
    },
  },
})
