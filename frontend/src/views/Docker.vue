<template>
  <div class="space-y-6">
    <!-- Page Header -->
    <div class="flex items-center justify-between">
      <div>
        <h1 class="text-2xl font-bold text-white flex items-center gap-3">
          <svg class="w-7 h-7 text-blue-400" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 3v4M3 5h4M6 17v4m-2-2h4m5-16l2.286 6.857L21 12l-5.714 2.143L13 21l-2.286-6.857L5 12l5.714-2.143L13 3z"/></svg>
          Docker Manager
        </h1>
        <p class="text-gray-400 mt-1">Docker konteyner ve imajlarınızı yönetin</p>
      </div>
    </div>

    <!-- Stats Cards -->
    <div class="grid grid-cols-1 md:grid-cols-3 gap-4">
      <div class="bg-panel-card border border-panel-border rounded-xl p-5">
        <div class="flex items-center justify-between">
          <div>
            <p class="text-sm text-gray-400">Çalışan Konteynerler</p>
            <p class="text-2xl font-bold text-green-400 mt-1">{{ runningCount }}</p>
          </div>
          <div class="p-3 bg-green-500/10 rounded-lg">
            <svg class="w-6 h-6 text-green-400" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7"/></svg>
          </div>
        </div>
      </div>
      <div class="bg-panel-card border border-panel-border rounded-xl p-5">
        <div class="flex items-center justify-between">
          <div>
            <p class="text-sm text-gray-400">Duran Konteynerler</p>
            <p class="text-2xl font-bold text-red-400 mt-1">{{ stoppedCount }}</p>
          </div>
          <div class="p-3 bg-red-500/10 rounded-lg">
            <svg class="w-6 h-6 text-red-400" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"/></svg>
          </div>
        </div>
      </div>
      <div class="bg-panel-card border border-panel-border rounded-xl p-5">
        <div class="flex items-center justify-between">
          <div>
            <p class="text-sm text-gray-400">Toplam İmaj</p>
            <p class="text-2xl font-bold text-blue-400 mt-1">{{ images.length }}</p>
          </div>
          <div class="p-3 bg-blue-500/10 rounded-lg">
            <svg class="w-6 h-6 text-blue-400" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 7v10c0 2.21 3.582 4 8 4s8-1.79 8-4V7M4 7c0 2.21 3.582 4 8 4s8-1.79 8-4M4 7c0-2.21 3.582-4 8-4s8 1.79 8 4"/></svg>
          </div>
        </div>
      </div>
    </div>

    <!-- Tab Navigation -->
    <div class="border-b border-panel-border">
      <nav class="flex gap-6">
        <button @click="activeTab = 'containers'" :class="['pb-3 text-sm font-medium transition-colors', activeTab === 'containers' ? 'text-blue-400 border-b-2 border-blue-400' : 'text-gray-400 hover:text-white']">
          🐳 Konteynerler
        </button>
        <button @click="activeTab = 'images'" :class="['pb-3 text-sm font-medium transition-colors', activeTab === 'images' ? 'text-blue-400 border-b-2 border-blue-400' : 'text-gray-400 hover:text-white']">
          📦 İmajlar
        </button>
        <button @click="activeTab = 'create'" :class="['pb-3 text-sm font-medium transition-colors', activeTab === 'create' ? 'text-blue-400 border-b-2 border-blue-400' : 'text-gray-400 hover:text-white']">
          ➕ Yeni Konteyner
        </button>
      </nav>
    </div>

    <!-- Containers Tab -->
    <div v-if="activeTab === 'containers'" class="bg-panel-card border border-panel-border rounded-xl overflow-hidden">
      <div class="p-4 border-b border-panel-border flex items-center justify-between">
        <h2 class="text-lg font-semibold text-white">Konteyner Listesi</h2>
        <button @click="refreshContainers" class="px-3 py-1.5 bg-panel-hover text-gray-300 rounded-lg text-sm hover:bg-gray-600 transition">
          🔄 Yenile
        </button>
      </div>
      <div class="overflow-x-auto">
        <table class="w-full text-sm">
          <thead>
            <tr class="text-gray-400 border-b border-panel-border">
              <th class="text-left px-4 py-3 font-medium">Ad</th>
              <th class="text-left px-4 py-3 font-medium">İmaj</th>
              <th class="text-left px-4 py-3 font-medium">Durum</th>
              <th class="text-left px-4 py-3 font-medium">Portlar</th>
              <th class="text-left px-4 py-3 font-medium">Oluşturulma</th>
              <th class="text-right px-4 py-3 font-medium">İşlemler</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="c in containers" :key="c.id" class="border-b border-panel-border/50 hover:bg-panel-hover/30 transition">
              <td class="px-4 py-3 text-white font-medium">{{ c.name }}</td>
              <td class="px-4 py-3 text-gray-300">{{ c.image }}</td>
              <td class="px-4 py-3">
                <span :class="['px-2 py-1 rounded-full text-xs font-medium', c.status.includes('Up') ? 'bg-green-500/20 text-green-400' : 'bg-red-500/20 text-red-400']">
                  {{ c.status.includes('Up') ? '● Çalışıyor' : '○ Durdu' }}
                </span>
              </td>
              <td class="px-4 py-3 text-gray-400 text-xs font-mono">{{ c.ports || '—' }}</td>
              <td class="px-4 py-3 text-gray-400 text-xs">{{ c.created }}</td>
              <td class="px-4 py-3 text-right space-x-1">
                <button v-if="!c.status.includes('Up')" @click="containerAction(c.id, 'start')" class="px-2 py-1 bg-green-600/20 text-green-400 rounded text-xs hover:bg-green-600/40 transition">▶ Başlat</button>
                <button v-if="c.status.includes('Up')" @click="containerAction(c.id, 'stop')" class="px-2 py-1 bg-yellow-600/20 text-yellow-400 rounded text-xs hover:bg-yellow-600/40 transition">⏹ Durdur</button>
                <button @click="containerAction(c.id, 'restart')" class="px-2 py-1 bg-blue-600/20 text-blue-400 rounded text-xs hover:bg-blue-600/40 transition">🔄 Yenile</button>
                <button @click="containerAction(c.id, 'remove')" class="px-2 py-1 bg-red-600/20 text-red-400 rounded text-xs hover:bg-red-600/40 transition">🗑 Sil</button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <!-- Images Tab -->
    <div v-if="activeTab === 'images'" class="bg-panel-card border border-panel-border rounded-xl overflow-hidden">
      <div class="p-4 border-b border-panel-border flex items-center justify-between">
        <h2 class="text-lg font-semibold text-white">Docker İmajları</h2>
        <div class="flex gap-2">
          <input v-model="pullImageName" type="text" placeholder="nginx:latest" class="px-3 py-1.5 bg-panel-hover border border-panel-border rounded-lg text-sm text-white placeholder-gray-500 w-48 focus:outline-none focus:border-blue-500">
          <button @click="pullImage" class="px-4 py-1.5 bg-blue-600 text-white rounded-lg text-sm hover:bg-blue-700 transition">
            ⬇️ Pull
          </button>
        </div>
      </div>
      <div class="overflow-x-auto">
        <table class="w-full text-sm">
          <thead>
            <tr class="text-gray-400 border-b border-panel-border">
              <th class="text-left px-4 py-3 font-medium">Repository</th>
              <th class="text-left px-4 py-3 font-medium">Tag</th>
              <th class="text-left px-4 py-3 font-medium">ID</th>
              <th class="text-left px-4 py-3 font-medium">Boyut</th>
              <th class="text-left px-4 py-3 font-medium">Oluşturulma</th>
              <th class="text-right px-4 py-3 font-medium">İşlem</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="img in images" :key="img.id" class="border-b border-panel-border/50 hover:bg-panel-hover/30 transition">
              <td class="px-4 py-3 text-white font-medium">{{ img.repository }}</td>
              <td class="px-4 py-3"><span class="px-2 py-0.5 bg-blue-500/20 text-blue-400 rounded text-xs">{{ img.tag }}</span></td>
              <td class="px-4 py-3 text-gray-400 font-mono text-xs">{{ img.id.substring(0, 12) }}</td>
              <td class="px-4 py-3 text-gray-300">{{ img.size }}</td>
              <td class="px-4 py-3 text-gray-400 text-xs">{{ img.created }}</td>
              <td class="px-4 py-3 text-right">
                <button @click="removeImage(img.id)" class="px-2 py-1 bg-red-600/20 text-red-400 rounded text-xs hover:bg-red-600/40 transition">🗑 Sil</button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <!-- Create New Container Tab -->
    <div v-if="activeTab === 'create'" class="bg-panel-card border border-panel-border rounded-xl p-6">
      <h2 class="text-lg font-semibold text-white mb-6">Yeni Konteyner Oluştur</h2>
      <form @submit.prevent="createContainer" class="space-y-5">
        <div class="grid grid-cols-1 md:grid-cols-2 gap-5">
          <div>
            <label class="block text-sm text-gray-400 mb-1.5">Konteyner Adı</label>
            <input v-model="newContainer.name" type="text" placeholder="my-app" class="w-full px-4 py-2.5 bg-panel-hover border border-panel-border rounded-lg text-white placeholder-gray-500 focus:outline-none focus:border-blue-500">
          </div>
          <div>
            <label class="block text-sm text-gray-400 mb-1.5">İmaj</label>
            <input v-model="newContainer.image" type="text" placeholder="nginx:latest" class="w-full px-4 py-2.5 bg-panel-hover border border-panel-border rounded-lg text-white placeholder-gray-500 focus:outline-none focus:border-blue-500">
          </div>
          <div>
            <label class="block text-sm text-gray-400 mb-1.5">Port Eşleme (virgülle ayırın)</label>
            <input v-model="newContainer.portsStr" type="text" placeholder="80:80, 443:443" class="w-full px-4 py-2.5 bg-panel-hover border border-panel-border rounded-lg text-white placeholder-gray-500 focus:outline-none focus:border-blue-500">
          </div>
          <div>
            <label class="block text-sm text-gray-400 mb-1.5">Ortam Değişkenleri (KEY=VAL)</label>
            <input v-model="newContainer.envStr" type="text" placeholder="MYSQL_ROOT_PASSWORD=secret" class="w-full px-4 py-2.5 bg-panel-hover border border-panel-border rounded-lg text-white placeholder-gray-500 focus:outline-none focus:border-blue-500">
          </div>
          <div>
            <label class="block text-sm text-gray-400 mb-1.5">Volume Bağlantısı</label>
            <input v-model="newContainer.volumesStr" type="text" placeholder="/data:/var/lib/mysql" class="w-full px-4 py-2.5 bg-panel-hover border border-panel-border rounded-lg text-white placeholder-gray-500 focus:outline-none focus:border-blue-500">
          </div>
          <div>
            <label class="block text-sm text-gray-400 mb-1.5">Restart Policy</label>
            <select v-model="newContainer.restart_policy" class="w-full px-4 py-2.5 bg-panel-hover border border-panel-border rounded-lg text-white focus:outline-none focus:border-blue-500">
              <option value="">Yok</option>
              <option value="always">Always</option>
              <option value="unless-stopped">Unless Stopped</option>
              <option value="on-failure">On Failure</option>
            </select>
          </div>
          <div>
            <label class="block text-sm text-gray-400 mb-1.5">RAM Limiti</label>
            <input v-model="newContainer.memory_limit" type="text" placeholder="512m veya 1g" class="w-full px-4 py-2.5 bg-panel-hover border border-panel-border rounded-lg text-white placeholder-gray-500 focus:outline-none focus:border-blue-500">
          </div>
          <div>
            <label class="block text-sm text-gray-400 mb-1.5">CPU Limiti</label>
            <input v-model="newContainer.cpu_limit" type="text" placeholder="0.5 veya 1.0" class="w-full px-4 py-2.5 bg-panel-hover border border-panel-border rounded-lg text-white placeholder-gray-500 focus:outline-none focus:border-blue-500">
          </div>
        </div>
        <div class="pt-4">
          <button type="submit" class="px-6 py-2.5 bg-gradient-to-r from-blue-600 to-indigo-600 text-white rounded-lg font-medium hover:from-blue-700 hover:to-indigo-700 transition-all duration-200 shadow-lg shadow-blue-500/25">
            🐳 Konteyner Oluştur & Başlat
          </button>
        </div>
      </form>
    </div>

    <!-- Notification -->
    <div v-if="notification" :class="['fixed bottom-6 right-6 px-5 py-3 rounded-xl shadow-2xl text-sm font-medium transition-all duration-300 z-50', notification.type === 'success' ? 'bg-green-600 text-white' : 'bg-red-600 text-white']">
      {{ notification.message }}
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import api from '../services/api'

const activeTab = ref('containers')
const containers = ref([])
const images = ref([])
const pullImageName = ref('')
const notification = ref(null)

const newContainer = ref({
  name: '',
  image: '',
  portsStr: '',
  envStr: '',
  volumesStr: '',
  restart_policy: '',
  memory_limit: '',
  cpu_limit: '',
})

const runningCount = computed(() => containers.value.filter(c => c.status.includes('Up')).length)
const stoppedCount = computed(() => containers.value.filter(c => !c.status.includes('Up')).length)

const showNotif = (message, type = 'success') => {
  notification.value = { message, type }
  setTimeout(() => notification.value = null, 3000)
}

const refreshContainers = async () => {
  try {
    const { data } = await api.get('/docker/containers')
    containers.value = data.data || []
  } catch {
    // Mock data for dev
    containers.value = [
      { id: 'abc123', name: 'nginx-proxy', image: 'nginx:latest', status: 'Up 3 days', ports: '0.0.0.0:80->80/tcp', created: '3 gün önce' },
      { id: 'def456', name: 'mysql-db', image: 'mariadb:11', status: 'Up 3 days', ports: '3306/tcp', created: '3 gün önce' },
      { id: 'ghi789', name: 'redis-cache', image: 'redis:7-alpine', status: 'Exited (0) 2 hours ago', ports: '6379/tcp', created: '5 gün önce' },
    ]
  }
}

const refreshImages = async () => {
  try {
    const { data } = await api.get('/docker/images')
    images.value = data.data || []
  } catch {
    images.value = [
      { id: 'sha256:abc123def', repository: 'nginx', tag: 'latest', size: '187MB', created: '2 hafta önce' },
      { id: 'sha256:def456ghi', repository: 'mariadb', tag: '11', size: '405MB', created: '3 hafta önce' },
      { id: 'sha256:ghi789jkl', repository: 'redis', tag: '7-alpine', size: '32MB', created: '1 ay önce' },
    ]
  }
}

const containerAction = async (id, action) => {
  try {
    await api.post(`/docker/containers/${action}`, { id, action })
    showNotif(`Konteyner ${action} başarılı: ${id}`)
    refreshContainers()
  } catch {
    showNotif(`İşlem başarısız: ${action}`, 'error')
  }
}

const pullImage = async () => {
  if (!pullImageName.value) return
  const [image, tag] = pullImageName.value.split(':')
  try {
    await api.post('/docker/images/pull', { image, tag: tag || 'latest' })
    showNotif(`İmaj çekildi: ${pullImageName.value}`)
    pullImageName.value = ''
    refreshImages()
  } catch {
    showNotif('İmaj çekilemedi', 'error')
  }
}

const removeImage = async (id) => {
  try {
    await api.post('/docker/images/remove', { id })
    showNotif('İmaj silindi')
    refreshImages()
  } catch {
    showNotif('Silme başarısız', 'error')
  }
}

const createContainer = async () => {
  const payload = {
    name: newContainer.value.name,
    image: newContainer.value.image,
    ports: newContainer.value.portsStr ? newContainer.value.portsStr.split(',').map(s => s.trim()) : [],
    env: newContainer.value.envStr ? newContainer.value.envStr.split(',').map(s => s.trim()) : [],
    volumes: newContainer.value.volumesStr ? newContainer.value.volumesStr.split(',').map(s => s.trim()) : [],
    restart_policy: newContainer.value.restart_policy || null,
    memory_limit: newContainer.value.memory_limit || null,
    cpu_limit: newContainer.value.cpu_limit || null,
  }
  try {
    await api.post('/docker/containers/create', payload)
    showNotif(`Konteyner "${payload.name}" oluşturuldu!`)
    newContainer.value = { name: '', image: '', portsStr: '', envStr: '', volumesStr: '', restart_policy: '', memory_limit: '', cpu_limit: '' }
    activeTab.value = 'containers'
    refreshContainers()
  } catch {
    showNotif('Konteyner oluşturulamadı', 'error')
  }
}

onMounted(() => {
  refreshContainers()
  refreshImages()
})
</script>
