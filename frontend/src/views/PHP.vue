<template>
  <div class="space-y-6 php-theme">
    <div>
      <h1 class="text-2xl font-bold text-white">PHP Yonetimi</h1>
      <p class="text-gray-400 mt-1">Surumler, site atamalari ve php.ini yonetimi.</p>
    </div>

    <div class="border-b border-panel-border">
      <nav class="flex gap-6">
        <button @click="tab='versions'" :class="tabClass('versions')">Versiyonlar</button>
        <button @click="tab='sites'" :class="tabClass('sites')">Site Atamalari</button>
        <button @click="tab='config'" :class="tabClass('config')">php.ini</button>
      </nav>
    </div>

    <div v-if="error" class="aura-card border-red-500/30 bg-red-500/5 text-red-400">{{ error }}</div>
    <div v-if="success" class="aura-card border-green-500/30 bg-green-500/5 text-green-300">{{ success }}</div>

    <div v-if="loading" class="text-center py-10 text-gray-400">Yukleniyor...</div>

    <div v-else>
      <div v-if="tab==='versions'" class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
        <div v-for="v in phpVersions" :key="v.version" class="bg-panel-card border border-panel-border rounded-xl p-5">
          <div class="flex items-center justify-between mb-3">
            <div>
              <p class="text-white font-semibold">PHP {{ v.version }}</p>
              <p class="text-xs" :class="v.eol ? 'text-yellow-400' : 'text-gray-400'">{{ v.eol ? 'EOL' : 'Supported' }}</p>
            </div>
            <span :class="['px-2 py-0.5 rounded text-xs font-medium', v.installed ? 'bg-green-500/15 text-green-400' : 'bg-gray-500/15 text-gray-400']">
              {{ v.installed ? 'Kurulu' : 'Kurulu degil' }}
            </span>
          </div>
          <div class="flex gap-2">
            <button v-if="!v.installed" class="btn-primary flex-1" @click="installPhp(v.version)">Kur</button>
            <button v-else class="btn-danger flex-1" @click="removePhp(v.version)">Kaldir</button>
            <button v-if="v.installed" class="btn-secondary" @click="restartPhp(v.version)">Restart</button>
          </div>
        </div>
      </div>

      <div v-if="tab==='sites'" class="bg-panel-card border border-panel-border rounded-xl overflow-hidden">
        <table class="w-full text-sm">
          <thead>
            <tr class="text-gray-400 border-b border-panel-border">
              <th class="text-left px-4 py-3">Domain</th>
              <th class="text-left px-4 py-3">Mevcut PHP</th>
              <th class="text-left px-4 py-3">Degistir</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="site in siteAssignments" :key="site.domain" class="border-b border-panel-border/50 hover:bg-white/[0.02] transition">
              <td class="px-4 py-3 text-white">{{ site.domain }}</td>
              <td class="px-4 py-3 text-gray-300">PHP {{ site.php_version }}</td>
              <td class="px-4 py-3">
                <select v-model="site.php_version" class="php-field aura-input" @change="changePhp(site)">
                  <option v-for="v in installedVersions" :key="v" :value="v">PHP {{ v }}</option>
                </select>
              </td>
            </tr>
            <tr v-if="siteAssignments.length===0">
              <td colspan="3" class="p-4 text-center text-gray-500">Site bulunamadi</td>
            </tr>
          </tbody>
        </table>
      </div>

      <div v-if="tab==='config'" class="space-y-3">
        <div class="flex items-center gap-3">
          <select v-model="selectedConfigVersion" class="php-field aura-input max-w-xs" @change="loadPhpIni">
            <option v-for="v in installedVersions" :key="v" :value="v">PHP {{ v }}</option>
          </select>
          <button class="btn-secondary" @click="loadPhpIni">Oku</button>
          <button class="btn-primary" @click="savePhpIni">Kaydet</button>
        </div>
        <textarea v-model="phpIniContent" rows="20" class="aura-input w-full font-mono text-xs"></textarea>
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import api from '../services/api'

const tab = ref('versions')
const loading = ref(false)
const error = ref('')
const success = ref('')

const phpVersions = ref([])
const siteAssignments = ref([])
const selectedConfigVersion = ref('')
const phpIniContent = ref('')

const installedVersions = computed(() => phpVersions.value.filter(v => v.installed).map(v => v.version))

function tabClass(key) {
  return [
    'pb-3 text-sm font-medium transition',
    tab.value === key ? 'text-orange-400 border-b-2 border-orange-400' : 'text-gray-400 hover:text-white',
  ]
}

function apiErrorMessage(e, fallback) {
  return e?.response?.data?.message || e?.message || fallback
}

async function loadData() {
  loading.value = true
  error.value = ''
  success.value = ''
  try {
    const [versionsRes, vhostRes] = await Promise.all([
      api.get('/php/versions'),
      api.get('/vhost/list'),
    ])

    phpVersions.value = versionsRes.data?.data || []
    siteAssignments.value = (vhostRes.data?.data || []).map((site) => ({
      domain: site.domain,
      php_version: site.php_version || site.php || '8.3',
      owner: site.owner || site.user || '',
      package: site.package || '',
      email: site.email || '',
    }))

    if (!selectedConfigVersion.value) {
      selectedConfigVersion.value = installedVersions.value[0] || ''
    }

    if (selectedConfigVersion.value) {
      await loadPhpIni()
    } else {
      phpIniContent.value = ''
    }
  } catch (e) {
    error.value = apiErrorMessage(e, 'PHP verileri alinamadi')
  } finally {
    loading.value = false
  }
}

async function installPhp(version) {
  error.value = ''
  success.value = ''
  try {
    const res = await api.post('/php/install', { version })
    success.value = res.data?.message || `PHP ${version} kuruldu.`
    await loadData()
  } catch (e) {
    error.value = apiErrorMessage(e, 'PHP kurulumu basarisiz')
  }
}

async function removePhp(version) {
  error.value = ''
  success.value = ''
  try {
    const res = await api.post('/php/remove', { version })
    success.value = res.data?.message || `PHP ${version} kaldirildi.`
    await loadData()
  } catch (e) {
    error.value = apiErrorMessage(e, 'PHP kaldirma basarisiz')
  }
}

async function restartPhp(version) {
  error.value = ''
  success.value = ''
  try {
    const res = await api.post('/php/restart', { version })
    success.value = res.data?.message || `PHP ${version} restart edildi.`
  } catch (e) {
    error.value = apiErrorMessage(e, 'PHP restart basarisiz')
  }
}

async function changePhp(site) {
  error.value = ''
  success.value = ''
  try {
    const res = await api.post('/vhost/update', {
      domain: site.domain,
      php_version: site.php_version,
      owner: site.owner || undefined,
      package: site.package || undefined,
      email: site.email || undefined,
    })
    success.value = res.data?.message || `${site.domain} icin PHP guncellendi.`
  } catch (e) {
    error.value = apiErrorMessage(e, 'Site PHP atamasi basarisiz')
  }
}

async function loadPhpIni() {
  if (!selectedConfigVersion.value) {
    phpIniContent.value = ''
    return
  }

  error.value = ''
  try {
    const res = await api.post('/php/ini/get', { version: selectedConfigVersion.value })
    phpIniContent.value = String(res.data?.data || '')
  } catch (e) {
    error.value = apiErrorMessage(e, 'php.ini okunamadi')
  }
}

async function savePhpIni() {
  if (!selectedConfigVersion.value) return
  error.value = ''
  success.value = ''
  try {
    const res = await api.post('/php/ini/save', {
      version: selectedConfigVersion.value,
      content: phpIniContent.value,
    })
    success.value = res.data?.message || 'php.ini kaydedildi.'
  } catch (e) {
    error.value = apiErrorMessage(e, 'php.ini kaydedilemedi')
  }
}

onMounted(loadData)
</script>

<style scoped>
.php-theme .php-field {
  background-color: #1f2d44 !important;
  color: #fb923c !important;
  border-color: rgba(251, 146, 60, 0.45) !important;
}

.php-theme .php-field option {
  background: #1b263a;
  color: #fb923c;
}
</style>
