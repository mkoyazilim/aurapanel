import axios from 'axios'
import { useAuthStore } from '../stores/auth'

// API Gateway Base URL
const defaultBaseUrl = typeof window !== 'undefined'
  ? '/api/v1'
  : 'http://127.0.0.1:8090/api/v1'

const api = axios.create({
  baseURL: import.meta.env.VITE_API_BASE_URL || defaultBaseUrl,
  timeout: 10000,
  headers: {
    'Content-Type': 'application/json',
    'Accept': 'application/json'
  }
})

// Request Interceptor (Auth Token Enjeksiyonu)
api.interceptors.request.use(config => {
  const authStore = useAuthStore()
  if (authStore.token && authStore.isTokenExpired(authStore.token)) {
    authStore.logout()
    window.location.href = '/login'
    return Promise.reject(new Error('Session expired'))
  }
  if (authStore.token) {
    config.headers.Authorization = `Bearer ${authStore.token}`
  }
  return config
}, error => {
  return Promise.reject(error)
})

// Response Interceptor (Hata Kontrolü)
api.interceptors.response.use(response => {
  return response
}, error => {
  const authStore = useAuthStore()
  // Token süresi dolduysa / 401 Unauthorized
  if (error.response && error.response.status === 401) {
    authStore.logout()
    window.location.href = '/login'
  }
  return Promise.reject(error)
})

export default api
