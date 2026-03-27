import { createApp } from 'vue'
import { createPinia } from 'pinia'
import './style.css'
import App from './App.vue'
import router from './router'
import i18n from './i18n'
import { useAuthStore } from './stores/auth'

const app = createApp(App)
const pinia = createPinia()

app.use(pinia)
app.use(router)
app.use(i18n)

async function bootstrap() {
  const authStore = useAuthStore(pinia)
  if (authStore.token) {
    await authStore.refreshUserFromServer()
  }
  app.mount('#app')
}

bootstrap().catch((error) => {
  console.error('AuraPanel bootstrap failed, mounting fallback shell.', error)
  app.mount('#app')
})
