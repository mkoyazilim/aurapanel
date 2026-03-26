import axios from 'axios'
import { useAuthStore } from '../stores/auth'

// API Gateway Base URL (Go Lang Gateway)
const api = axios.create({
  baseURL: import.meta.env.VITE_API_BASE_URL || 'http://localhost:8080/api',
  timeout: 10000,
  headers: {
    'Content-Type': 'application/json',
    'Accept': 'application/json'
  }
})

// Request Interceptor (Auth Token Enjeksiyonu)
api.interceptors.request.use(config => {
  const authStore = useAuthStore()
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
