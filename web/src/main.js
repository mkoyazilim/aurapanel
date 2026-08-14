import { createApp } from 'vue'
import { createPinia } from 'pinia'
import App from './App.vue'
import router from './router'
import i18n from './i18n'
import './style.css'

// Tüm `fetch` çağrılarına otomatik olarak CSRF token ekleyen global interceptor.
const originalFetch = window.fetch
window.fetch = async function(resource, config) {
  if (typeof resource === 'string' && resource.startsWith('/api/v1')) {
    const method = (config && config.method) ? config.method.toUpperCase() : 'GET'
    if (method !== 'GET' && method !== 'HEAD') {
      config = config || {}
      config.headers = config.headers || {}
      const csrf = localStorage.getItem('ap_csrf')
      if (csrf) {
        if (config.headers instanceof Headers) {
          config.headers.set('X-CSRF-Token', csrf)
        } else if (Array.isArray(config.headers)) {
          config.headers.push(['X-CSRF-Token', csrf])
        } else {
          config.headers['X-CSRF-Token'] = csrf
        }
      }
    }
  }
  return originalFetch.call(this, resource, config)
}
createApp(App).use(createPinia()).use(router).use(i18n).mount('#app')
