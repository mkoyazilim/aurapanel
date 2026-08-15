<template>
  <Layout>
    <div class="page" v-if="site">
      <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 24px;">
        <div>
          <h1 style="margin: 0; display: flex; align-items: center; gap: 8px;">
            <span class="badge" :class="site.status === 'active' ? 'ok' : 'warn'">{{ site.status }}</span>
            {{ site.name }}
          </h1>
          <p class="muted" style="margin: 4px 0 0 0;">
            Kullanıcı: <strong>{{ site.linux_user }}</strong> | Tür: <strong>{{ site.app_type === 'nodejs' ? 'Node.js' : 'PHP ' + (site.php_version_id || 'Standart') }}</strong>
          </p>
        </div>
        <button class="btn" @click="$router.push('/sites')">⬅️ Tüm Siteler</button>
      </div>

      <div class="dashboard-grid">
        <!-- Sol Kolon: Yönetim Araçları -->
        <div class="dashboard-tools" style="flex: 2; display: grid; grid-template-columns: repeat(auto-fill, minmax(200px, 1fr)); gap: 16px;">
          <router-link :to="'/files?site=' + site.id" class="tool-card">
            <span class="icon">📁</span>
            <h3>Dosya Yöneticisi</h3>
            <p>Dosyaları yönetin</p>
          </router-link>
          
          <router-link :to="'/databases?site=' + site.id" class="tool-card">
            <span class="icon">🗄️</span>
            <h3>Veritabanları</h3>
            <p>MySQL / MariaDB</p>
          </router-link>

          <router-link :to="'/ssl?site=' + site.id" class="tool-card">
            <span class="icon">🔒</span>
            <h3>SSL Yönetimi</h3>
            <p>Let's Encrypt / Custom</p>
          </router-link>

          <router-link v-if="site.app_type === 'nodejs'" :to="'/sites/' + site.id + '/nodejs'" class="tool-card">
            <span class="icon">🟢</span>
            <h3>Node.js Yönetimi</h3>
            <p>PM2 & Proxy Ayarları</p>
          </router-link>

          <router-link :to="'/sites/' + site.id + '/mail'" class="tool-card">
            <span class="icon">📧</span>
            <h3>Mail Hesapları</h3>
            <p>Kurumsal E-posta</p>
          </router-link>

          <router-link :to="'/logs?site=' + site.id" class="tool-card">
            <span class="icon">📊</span>
            <h3>Erişim ve Loglar</h3>
            <p>Hata kayıtları (Error Logs)</p>
          </router-link>
          
          <router-link :to="'/cron?site=' + site.id" class="tool-card">
            <span class="icon">⏱️</span>
            <h3>Cron Jobs</h3>
            <p>Zamanlanmış Görevler</p>
          </router-link>
        </div>

        <!-- Sağ Kolon: Subdomain / Alias -->
        <div style="flex: 1; display: flex; flex-direction: column; gap: 16px;">
          <div class="card">
            <h3>🌐 Alt Alan Adları (Subdomains)</h3>
            <p class="muted text-sm" style="margin-bottom: 12px;">Siteniz altında çalışan, tamamen izole edilmiş alt projeler.</p>
            
            <div style="display: flex; gap: 8px; margin-bottom: 16px;">
              <input v-model="newSubdomain" type="text" placeholder="blog" style="flex: 1" />
              <div style="padding-top: 8px; font-weight: 500;">.{{ site.name }}</div>
              <button class="btn primary" @click="createSubdomain" :disabled="busy">Ekle</button>
            </div>

            <div v-if="subdomains.length > 0">
              <div v-for="sub in subdomains" :key="sub.id" style="display: flex; justify-content: space-between; padding: 8px 0; border-bottom: 1px solid var(--border-color);">
                <a :href="'http://' + sub.name" target="_blank" style="color: var(--primary);">{{ sub.name }}</a>
                <router-link :to="'/sites/' + sub.id + '/dashboard'" class="btn btn-sm">Yönet ➡</router-link>
              </div>
            </div>
            <div v-else class="muted text-sm text-center" style="padding: 16px 0;">Henüz alt alan adı yok.</div>
          </div>
        </div>
      </div>
    </div>
    <div v-else class="page loading">Yükleniyor...</div>
  </Layout>
</template>

<script setup>
import { ref, onMounted, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import Layout from '../components/Layout.vue'
import { api } from '../api'

const route = useRoute()
const router = useRouter()
const siteId = ref(route.params.id)

const site = ref(null)
const subdomains = ref([])
const newSubdomain = ref('')
const busy = ref(false)

async function loadData() {
  try {
    const allSites = await api('/sites')
    site.value = allSites.find(s => s.id === siteId.value)
    
    if (site.value) {
      // Find subdomains (other sites ending with .sitename)
      subdomains.value = allSites.filter(s => s.id !== siteId.value && s.name.endsWith('.' + site.value.name))
    }
  } catch (err) {
    console.error(err)
  }
}

async function createSubdomain() {
  if (!newSubdomain.value || !site.value) return
  busy.value = true
  try {
    const fullDomain = `${newSubdomain.value}.${site.value.name}`
    await api('/sites', {
      method: 'POST',
      body: { 
        domain: fullDomain, 
        app_type: 'php', // Varsayılan PHP
        php_version: '8.3',
        aliases: [], 
        limits: {} 
      },
    })
    newSubdomain.value = ''
    await loadData()
  } catch (e) {
    alert('Hata: ' + e.message)
  } finally {
    busy.value = false
  }
}

watch(() => route.params.id, (newId) => {
  if (newId) {
    siteId.value = newId
    loadData()
  }
})

onMounted(loadData)
</script>

<style scoped>
.dashboard-grid {
  display: flex;
  gap: 24px;
  align-items: flex-start;
}
@media (max-width: 900px) {
  .dashboard-grid {
    flex-direction: column;
  }
}

.tool-card {
  background: var(--bg-card, #ffffff);
  border: 1px solid var(--border-color, #e2e8f0);
  border-radius: 12px;
  padding: 20px;
  text-decoration: none;
  color: inherit;
  transition: all 0.2s;
  display: flex;
  flex-direction: column;
  align-items: center;
  text-align: center;
  box-shadow: 0 4px 6px -1px rgba(0, 0, 0, 0.05);
}
.tool-card:hover {
  transform: translateY(-4px);
  border-color: var(--primary);
  box-shadow: 0 10px 15px -3px rgba(0, 0, 0, 0.1);
}
.tool-card .icon {
  font-size: 32px;
  margin-bottom: 12px;
}
.tool-card h3 {
  margin: 0 0 6px 0;
  font-size: 15px;
}
.tool-card p {
  margin: 0;
  font-size: 12px;
  color: var(--muted);
}
</style>
