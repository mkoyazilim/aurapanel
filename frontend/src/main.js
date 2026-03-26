import { createApp } from 'vue'
import { createPinia } from 'pinia'
import { createI18n } from 'vue-i18n'
import './style.css'
import App from './App.vue'
import router from './router'

import en from './locales/en.json'
import tr from './locales/tr.json'

const i18n = createI18n({
  legacy: false,
  locale: 'tr',
  fallbackLocale: 'en',
  messages: { en, tr }
})

const app = createApp(App)

app.use(createPinia())
app.use(router)
app.use(i18n)

app.mount('#app')
