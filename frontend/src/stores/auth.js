import { defineStore } from 'pinia'
import api from '../services/api'

export const useAuthStore = defineStore('auth', {
  state: () => ({
    token: localStorage.getItem('aura_token') || null,
    user: JSON.parse(localStorage.getItem('aura_user')) || null,
  }),
  getters: {
    isAuthenticated: (state) => !!state.token,
    isAdmin: (state) => state.user?.role === 'admin'
  },
  actions: {
    async login(email, password) {
      try {
        const response = await api.post('/auth/login', { email, password })
        this.setAuth(response.data.token, response.data.user)
        return true
      } catch (error) {
        console.error("Login Error", error)
        throw new Error(error.response?.data?.message || "Giriş başarısız. Bilgilerinizi kontrol edin.")
      }
    },
    setAuth(token, user) {
      this.token = token
      this.user = user
      localStorage.setItem('aura_token', token)
      localStorage.setItem('aura_user', JSON.stringify(user))
    },
    logout() {
      this.token = null
      this.user = null
      localStorage.removeItem('aura_token')
      localStorage.removeItem('aura_user')
    }
  }
})
