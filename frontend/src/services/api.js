import axios from 'axios'
import i18n from '../i18n'
import { useAuthStore } from '../stores/auth'
import { useNotificationStore } from '../stores/notifications'
import { useRequestStateStore } from '../stores/requestState'

const defaultBaseUrl = typeof window !== 'undefined'
  ? '/api/v1'
  : 'http://127.0.0.1:8090/api/v1'

const api = axios.create({
  baseURL: import.meta.env.VITE_API_BASE_URL || defaultBaseUrl,
  timeout: 60000, // Increased timeout to 60 seconds for long running tasks like SSL issuance
  headers: {
    Accept: 'application/json',
  },
})

const silentErrorHeader = 'X-Aura-Silent-Error'
const silentLoadingHeader = 'X-Aura-Silent-Loading'

function shouldTrackRequestLoading(config) {
  return String(config?.headers?.[silentLoadingHeader] || '').toLowerCase() !== '1'
}

function includesAnyKeyword(input, keywords) {
  const value = String(input || '').toLowerCase()
  return keywords.some((keyword) => value.includes(keyword))
}

function resolveRequestAction(config) {
  const method = String(config?.method || 'get').trim().toLowerCase()
  const url = String(config?.url || '').trim().toLowerCase()
  const isFormData = typeof FormData !== 'undefined' && config?.data instanceof FormData
  if (isFormData) {
    return 'uploading'
  }
  if (method === 'get' || method === 'head' || method === 'options') {
    return 'loading'
  }
  if (
    method === 'delete' ||
    includesAnyKeyword(url, ['/delete', '/remove', '/drop', '/detach', '/revoke'])
  ) {
    return 'deleting'
  }
  if (includesAnyKeyword(url, ['/add', '/create', '/attach', '/import', '/install', '/join'])) {
    return 'adding'
  }
  if (includesAnyKeyword(url, ['/save', '/update', '/set', '/apply', '/reset', '/restart', '/sync'])) {
    return 'updating'
  }
  if (method === 'put' || method === 'patch') {
    return 'updating'
  }
  if (method === 'post') {
    return 'processing'
  }
  return 'processing'
}

function finishTrackedRequest(config) {
  const token = config?.metadata?.requestLoadingToken
  if (!token) {
    return
  }
  const requestStateStore = useRequestStateStore()
  requestStateStore.finish(token)
}

function extractErrorMessage(error) {
  const responseData = error?.response?.data
  if (typeof responseData?.message === 'string' && responseData.message.trim()) {
    return responseData.message.trim()
  }
  if (typeof responseData?.error === 'string' && responseData.error.trim()) {
    return responseData.error.trim()
  }
  if (typeof error?.message === 'string' && error.message.trim()) {
    return error.message.trim()
  }
  return i18n.global.t('api_messages.unknown_error')
}

api.interceptors.request.use(config => {
  const authStore = useAuthStore()
  if (authStore.token && authStore.isTokenExpired(authStore.token)) {
    authStore.logout()
    window.location.href = '/login'
    return Promise.reject(new Error(i18n.global.t('api_messages.session_expired')))
  }
  if (authStore.token) {
    config.headers.Authorization = `Bearer ${authStore.token}`
  }

  const isFormData = typeof FormData !== 'undefined' && config?.data instanceof FormData
  if (isFormData) {
    // Let browser/axios generate multipart boundary automatically.
    delete config.headers['Content-Type']
  } else if (!config.headers['Content-Type']) {
    config.headers['Content-Type'] = 'application/json'
  }

  if (shouldTrackRequestLoading(config)) {
    const requestStateStore = useRequestStateStore()
    const token = requestStateStore.start(resolveRequestAction(config))
    config.metadata = {
      ...(config.metadata || {}),
      requestLoadingToken: token,
    }
  }

  return config
}, error => Promise.reject(error))

api.interceptors.response.use(response => {
  finishTrackedRequest(response?.config)
  return response
}, error => {
  finishTrackedRequest(error?.config)
  const authStore = useAuthStore()
  const notificationStore = useNotificationStore()
  const status = Number(error?.response?.status || 0)
  const reqUrl = String(error?.config?.url || '')
  const silentError = String(error?.config?.headers?.[silentErrorHeader] || '').toLowerCase() === '1'
  const isLoginRequest = reqUrl.includes('/auth/login')

  if (status === 401 && !isLoginRequest) {
    notificationStore.add({
      title: i18n.global.t('api_messages.session_ended_title'),
      message: i18n.global.t('api_messages.session_ended_message'),
      type: 'warning',
      source: 'auth',
    })
    authStore.logout()
    window.location.href = '/login'
    return Promise.reject(error)
  }

  if (!silentError) {
    const statusText = status > 0 ? `HTTP ${status}` : i18n.global.t('api_messages.network_error')
    notificationStore.add({
      title: i18n.global.t('api_messages.api_error_title', { status: statusText }),
      message: reqUrl ? `${reqUrl}: ${extractErrorMessage(error)}` : extractErrorMessage(error),
      type: 'error',
      source: 'api',
    })
  }

  return Promise.reject(error)
})

export default api
