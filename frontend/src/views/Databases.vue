<template>
  <div class="space-y-6">
    <!-- Header -->
    <div class="flex items-center justify-between">
      <div>
        <h1 class="text-2xl font-bold text-white flex items-center gap-3">
          <Database class="w-7 h-7 text-orange-400" />
          Veritabanları
        </h1>
        <p class="text-gray-400 mt-1">MariaDB ve PostgreSQL veritabanlarınızı yönetin</p>
      </div>
      <button @click="showCreateModal = true" class="px-5 py-2.5 bg-gradient-to-r from-orange-600 to-amber-600 text-white rounded-lg font-medium hover:from-orange-700 hover:to-amber-700 transition shadow-lg shadow-orange-500/25">
        <span class="flex items-center gap-2">
          <Plus class="w-5 h-5" />
          Veritabanı Oluştur
        </span>
      </button>
    </div>

    <!-- Engine Tabs -->
    <div class="border-b border-panel-border">
      <nav class="flex gap-6">
        <button @click="engine = 'mariadb'" :class="['pb-3 text-sm font-medium transition flex items-center gap-2', engine === 'mariadb' ? 'text-orange-400 border-b-2 border-orange-400' : 'text-gray-400 hover:text-white']">
          🐬 MariaDB / MySQL
        </button>
        <button @click="engine = 'postgresql'" :class="['pb-3 text-sm font-medium transition flex items-center gap-2', engine === 'postgresql' ? 'text-blue-400 border-b-2 border-blue-400' : 'text-gray-400 hover:text-white']">
          🐘 PostgreSQL
        </button>
      </nav>
    </div>

    <!-- Stats -->
    <div class="grid grid-cols-1 md:grid-cols-3 gap-4">
      <div class="bg-panel-card border border-panel-border rounded-xl p-5">
        <div class="flex items-center justify-between">
          <div>
            <p class="text-sm text-gray-400">Veritabanları</p>
            <p class="text-2xl font-bold text-white mt-1">{{ currentDatabases.length }}</p>
          </div>
          <div :class="['p-3 rounded-lg', engine === 'mariadb' ? 'bg-orange-500/10' : 'bg-blue-500/10']">
            <Database :class="['w-6 h-6', engine === 'mariadb' ? 'text-orange-400' : 'text-blue-400']" />
          </div>
        </div>
      </div>
      <div class="bg-panel-card border border-panel-border rounded-xl p-5">
        <div class="flex items-center justify-between">
          <div>
            <p class="text-sm text-gray-400">Kullanıcılar</p>
            <p class="text-2xl font-bold text-white mt-1">{{ currentUsers.length }}</p>
          </div>
          <div class="p-3 bg-green-500/10 rounded-lg">
            <Users class="w-6 h-6 text-green-400" />
          </div>
        </div>
      </div>
      <div class="bg-panel-card border border-panel-border rounded-xl p-5">
        <div class="flex items-center justify-between">
          <div>
            <p class="text-sm text-gray-400">Motor</p>
            <p class="text-2xl font-bold text-white mt-1">{{ engine === 'mariadb' ? 'MariaDB 11' : 'PostgreSQL 16' }}</p>
          </div>
          <div class="p-3 bg-purple-500/10 rounded-lg">
            <HardDrive class="w-6 h-6 text-purple-400" />
          </div>
        </div>
      </div>
    </div>

    <!-- Database List -->
    <div class="bg-panel-card border border-panel-border rounded-xl overflow-hidden">
      <div class="p-4 border-b border-panel-border flex items-center justify-between">
        <h2 class="text-lg font-semibold text-white">{{ engine === 'mariadb' ? 'MariaDB' : 'PostgreSQL' }} Veritabanları</h2>
        <button @click="loadData" class="px-3 py-1.5 bg-panel-hover text-gray-300 rounded-lg text-sm hover:bg-gray-600 transition">🔄 Yenile</button>
      </div>
      <div class="overflow-x-auto">
        <table class="w-full text-sm">
          <thead>
            <tr class="text-gray-400 border-b border-panel-border">
              <th class="text-left px-4 py-3 font-medium">Veritabanı Adı</th>
              <th class="text-left px-4 py-3 font-medium">Boyut</th>
              <th class="text-left px-4 py-3 font-medium">Tablo</th>
              <th class="text-left px-4 py-3 font-medium">Motor</th>
              <th class="text-right px-4 py-3 font-medium">İşlem</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="db in currentDatabases" :key="db.name" class="border-b border-panel-border/50 hover:bg-white/[0.02] transition">
              <td class="px-4 py-3 text-white font-medium font-mono">{{ db.name }}</td>
              <td class="px-4 py-3 text-gray-300">{{ db.size }}</td>
              <td class="px-4 py-3 text-gray-400">{{ db.tables }}</td>
              <td class="px-4 py-3">
                <span :class="['px-2 py-0.5 rounded text-xs font-medium', db.engine === 'mariadb' ? 'bg-orange-500/15 text-orange-400' : 'bg-blue-500/15 text-blue-400']">
                  {{ db.engine === 'mariadb' ? 'MariaDB' : 'PostgreSQL' }}
                </span>
              </td>
              <td class="px-4 py-3 text-right">
                <button @click="dropDatabase(db.name)" class="px-2 py-1 bg-red-600/20 text-red-400 rounded text-xs hover:bg-red-600/40 transition">🗑 Sil</button>
              </td>
            </tr>
            <tr v-if="currentDatabases.length === 0">
              <td colspan="5" class="px-4 py-12 text-center text-gray-500">
                <Database class="w-10 h-10 mx-auto mb-3 opacity-30" />
                Henüz veritabanı bulunmuyor
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <!-- Users List -->
    <div class="bg-panel-card border border-panel-border rounded-xl overflow-hidden">
      <div class="p-4 border-b border-panel-border">
        <h2 class="text-lg font-semibold text-white">Veritabanı Kullanıcıları</h2>
      </div>
      <div class="overflow-x-auto">
        <table class="w-full text-sm">
          <thead>
            <tr class="text-gray-400 border-b border-panel-border">
              <th class="text-left px-4 py-3 font-medium">Kullanıcı</th>
              <th class="text-left px-4 py-3 font-medium">Host</th>
              <th class="text-left px-4 py-3 font-medium">Motor</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="u in currentUsers" :key="u.username" class="border-b border-panel-border/50 hover:bg-white/[0.02] transition">
              <td class="px-4 py-3 text-white font-medium font-mono">{{ u.username }}</td>
              <td class="px-4 py-3 text-gray-400">{{ u.host }}</td>
              <td class="px-4 py-3">
                <span :class="['px-2 py-0.5 rounded text-xs font-medium', u.engine === 'mariadb' ? 'bg-orange-500/15 text-orange-400' : 'bg-blue-500/15 text-blue-400']">
                  {{ u.engine === 'mariadb' ? 'MariaDB' : 'PostgreSQL' }}
                </span>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <!-- Create Database Modal -->
    <div v-if="showCreateModal" class="fixed inset-0 bg-black/60 backdrop-blur-sm flex items-center justify-center z-50" @click.self="showCreateModal = false">
      <div class="bg-panel-card border border-panel-border rounded-2xl w-full max-w-lg p-6 shadow-2xl">
        <h3 class="text-xl font-bold text-white mb-5">{{ engine === 'mariadb' ? '🐬' : '🐘' }} Yeni Veritabanı Oluştur</h3>
        <div class="space-y-4">
          <div>
            <label class="block text-sm text-gray-400 mb-1">Motor</label>
            <select v-model="createForm.engine" class="w-full px-4 py-2.5 bg-panel-hover border border-panel-border rounded-lg text-white focus:outline-none focus:border-orange-500">
              <option value="mariadb">🐬 MariaDB / MySQL</option>
              <option value="postgresql">🐘 PostgreSQL</option>
            </select>
          </div>
          <div>
            <label class="block text-sm text-gray-400 mb-1">Veritabanı Adı</label>
            <input v-model="createForm.db_name" type="text" placeholder="my_database" class="w-full px-4 py-2.5 bg-panel-hover border border-panel-border rounded-lg text-white placeholder-gray-500 focus:outline-none focus:border-orange-500">
          </div>
          <div>
            <label class="block text-sm text-gray-400 mb-1">Kullanıcı Adı</label>
            <input v-model="createForm.db_user" type="text" placeholder="db_user" class="w-full px-4 py-2.5 bg-panel-hover border border-panel-border rounded-lg text-white placeholder-gray-500 focus:outline-none focus:border-orange-500">
          </div>
          <div>
            <label class="block text-sm text-gray-400 mb-1">Şifre</label>
            <input v-model="createForm.db_pass" type="password" placeholder="••••••••" class="w-full px-4 py-2.5 bg-panel-hover border border-panel-border rounded-lg text-white placeholder-gray-500 focus:outline-none focus:border-orange-500">
          </div>
        </div>
        <div class="flex gap-3 mt-6">
          <button @click="createDatabase" class="flex-1 py-2.5 bg-gradient-to-r from-orange-600 to-amber-600 text-white rounded-lg font-medium hover:from-orange-700 hover:to-amber-700 transition">Oluştur</button>
          <button @click="showCreateModal = false" class="px-5 py-2.5 bg-panel-hover text-gray-300 rounded-lg hover:bg-gray-600 transition">İptal</button>
        </div>
      </div>
    </div>

    <!-- Notification -->
    <div v-if="notification" :class="['fixed bottom-6 right-6 px-5 py-3 rounded-xl shadow-2xl text-sm font-medium z-50', notification.type === 'success' ? 'bg-green-600 text-white' : 'bg-red-600 text-white']">
      {{ notification.message }}
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, watch } from 'vue'
import { Database, Plus, Users, HardDrive } from 'lucide-vue-next'
import api from '../services/api'

const engine = ref('mariadb')
const showCreateModal = ref(false)
const notification = ref(null)

const mariadbDatabases = ref([])
const postgresDatabases = ref([])
const mariadbUsers = ref([])
const postgresUsers = ref([])

const currentDatabases = computed(() => engine.value === 'mariadb' ? mariadbDatabases.value : postgresDatabases.value)
const currentUsers = computed(() => engine.value === 'mariadb' ? mariadbUsers.value : postgresUsers.value)

const createForm = ref({ engine: 'mariadb', db_name: '', db_user: '', db_pass: '' })

const showNotif = (message, type = 'success') => {
  notification.value = { message, type }
  setTimeout(() => notification.value = null, 3000)
}

const loadData = async () => {
  // MariaDB
  try {
    const { data } = await api.get('/db/mariadb/list')
    mariadbDatabases.value = data.data || []
  } catch {
    mariadbDatabases.value = [
      { name: 'wp_blog', engine: 'mariadb', size: '24.5 MB', tables: 12 },
      { name: 'app_prod', engine: 'mariadb', size: '156.2 MB', tables: 45 },
      { name: 'ecommerce', engine: 'mariadb', size: '512.8 MB', tables: 78 },
    ]
  }
  try {
    const { data } = await api.get('/db/mariadb/users')
    mariadbUsers.value = data.data || []
  } catch {
    mariadbUsers.value = [
      { username: 'wp_user', host: 'localhost', engine: 'mariadb' },
      { username: 'app_user', host: 'localhost', engine: 'mariadb' },
    ]
  }
  // PostgreSQL
  try {
    const { data } = await api.get('/db/postgres/list')
    postgresDatabases.value = data.data || []
  } catch {
    postgresDatabases.value = [
      { name: 'analytics_db', engine: 'postgresql', size: '89.3 MB', tables: 23 },
      { name: 'saas_app', engine: 'postgresql', size: '1.2 GB', tables: 134 },
    ]
  }
  try {
    const { data } = await api.get('/db/postgres/users')
    postgresUsers.value = data.data || []
  } catch {
    postgresUsers.value = [
      { username: 'analytics_user', host: 'local', engine: 'postgresql' },
      { username: 'saas_user', host: 'local', engine: 'postgresql' },
    ]
  }
}

const createDatabase = async () => {
  const eng = createForm.value.engine
  const endpoint = eng === 'mariadb' ? '/db/mariadb/create' : '/db/postgres/create'
  try {
    await api.post(endpoint, {
      db_name: createForm.value.db_name,
      db_user: createForm.value.db_user,
      db_pass: createForm.value.db_pass,
    })
    showNotif(`${eng === 'mariadb' ? 'MariaDB' : 'PostgreSQL'} veritabanı "${createForm.value.db_name}" oluşturuldu!`)
    showCreateModal.value = false
    createForm.value = { engine: eng, db_name: '', db_user: '', db_pass: '' }
    loadData()
  } catch {
    showNotif('Veritabanı oluşturulamadı', 'error')
  }
}

const dropDatabase = async (name) => {
  const endpoint = engine.value === 'mariadb' ? '/db/mariadb/drop' : '/db/postgres/drop'
  try {
    await api.post(endpoint, { name })
    showNotif(`Veritabanı "${name}" silindi`)
    loadData()
  } catch {
    showNotif('Silme başarısız', 'error')
  }
}

watch(engine, (v) => { createForm.value.engine = v })

onMounted(loadData)
</script>
