<template>
  <div>
    <h2 class="text-2xl font-bold mb-6">Node.js Applications</h2>

    <div v-if="loading" class="text-gray-500">
      Loading...
    </div>
    <div v-else>
      <div v-if="error" class="bg-red-100 text-red-600 p-3 rounded-md mb-4">{{ error }}</div>
      
      <div class="mb-6 flex justify-between items-center">
        <p class="text-gray-600 dark:text-gray-400">Manage Node.js applications running on this site.</p>
        <button @click="showAddModal = true" class="bg-blue-600 text-white px-4 py-2 rounded-md hover:bg-blue-700 transition-colors">
          + New App
        </button>
      </div>

      <div class="grid gap-4">
        <div v-for="app in apps" :key="app.id" class="bg-white p-6 rounded-lg shadow-sm border border-gray-200 dark:bg-gray-800 dark:border-gray-700">
          <div class="flex justify-between items-start mb-4">
            <div>
              <h3 class="text-lg font-bold text-gray-900 dark:text-gray-100">{{ app.app_name }}</h3>
              <p class="text-sm text-gray-500">Path: {{ app.app_path }} • Port: {{ app.port }}</p>
            </div>
            <div class="flex items-center space-x-2">
              <span :class="{'bg-green-100 text-green-800': app.status === 'active', 'bg-red-100 text-red-800': app.status !== 'active'}" class="px-2 py-1 text-xs font-semibold rounded-full capitalize">
                {{ app.status }}
              </span>
              <button @click="restartApp(app.id)" class="text-gray-600 hover:text-blue-600 ml-2" title="Restart">
                <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" class="w-5 h-5"><path stroke-linecap="round" stroke-linejoin="round" d="M16.023 9.348h4.992v-.001M2.985 19.644v-4.992m0 0h4.992m-4.993 0 3.181 3.183a8.25 8.25 0 0 0 13.803-3.7M4.031 9.865a8.25 8.25 0 0 1 13.803-3.7l3.181 3.182m0-4.991v4.99" /></svg>
              </button>
              <button @click="deleteApp(app.id)" class="text-gray-600 hover:text-red-600" title="Delete">
                <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" class="w-5 h-5"><path stroke-linecap="round" stroke-linejoin="round" d="m14.74 9-.346 9m-4.788 0L9.26 9m9.968-3.21c.342.052.682.107 1.022.166m-1.022-.165L18.16 19.673a2.25 2.25 0 0 1-2.244 2.077H8.084a2.25 2.25 0 0 1-2.244-2.077L4.772 5.79m14.456 0a48.108 48.108 0 0 0-3.478-.397m-12 .562c.34-.059.68-.114 1.022-.165m0 0a48.11 48.11 0 0 1 3.478-.397m7.5 0v-.916c0-1.18-.91-2.164-2.09-2.201a51.964 51.964 0 0 0-3.32 0c-1.18.037-2.09 1.022-2.09 2.201v.916m7.5 0a48.667 48.667 0 0 0-7.5 0" /></svg>
              </button>
            </div>
          </div>
          <div class="text-sm bg-gray-50 dark:bg-gray-900 p-3 rounded-md font-mono border dark:border-gray-700">
            Node {{ app.node_version }} • {{ app.startup_script }}
          </div>
        </div>

        <div v-if="apps.length === 0" class="text-center py-12 text-gray-500 border-2 border-dashed rounded-lg dark:border-gray-700">
          No Node.js apps configured yet.
        </div>
      </div>
    </div>

    <!-- Modal for adding App -->
    <div v-if="showAddModal" class="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center p-4 z-50">
      <div class="bg-white dark:bg-gray-800 rounded-lg max-w-lg w-full p-6 shadow-xl">
        <h3 class="text-lg font-bold mb-4">Create Node.js App</h3>
        <form @submit.prevent="createApp" class="space-y-4">
          <div>
            <label class="block text-sm font-medium mb-1">App Name</label>
            <input v-model="form.app_name" type="text" required placeholder="my-api"
                   class="w-full px-3 py-2 border rounded-md dark:bg-gray-700 dark:border-gray-600">
          </div>
          <div class="grid grid-cols-2 gap-4">
            <div>
              <label class="block text-sm font-medium mb-1">Path</label>
              <input v-model="form.app_path" type="text" required placeholder="/"
                     class="w-full px-3 py-2 border rounded-md dark:bg-gray-700 dark:border-gray-600">
            </div>
            <div>
              <label class="block text-sm font-medium mb-1">Port</label>
              <input v-model.number="form.port" type="number" required placeholder="3000" min="1024" max="65535"
                     class="w-full px-3 py-2 border rounded-md dark:bg-gray-700 dark:border-gray-600">
            </div>
          </div>
          <div class="grid grid-cols-2 gap-4">
             <div>
              <label class="block text-sm font-medium mb-1">Node Version</label>
              <select v-model="form.node_version" class="w-full px-3 py-2 border rounded-md dark:bg-gray-700 dark:border-gray-600">
                <option value="18">18.x</option>
                <option value="20">20.x</option>
                <option value="22">22.x</option>
              </select>
            </div>
            <div>
              <label class="block text-sm font-medium mb-1">Startup Script</label>
              <input v-model="form.startup_script" type="text" required placeholder="npm start"
                     class="w-full px-3 py-2 border rounded-md dark:bg-gray-700 dark:border-gray-600">
            </div>
          </div>
          <div>
            <label class="block text-sm font-medium mb-1">Environment Variables (JSON)</label>
            <textarea v-model="form.env_vars" rows="3" placeholder='{"NODE_ENV": "production", "API_KEY": "secret"}'
                      class="w-full px-3 py-2 border rounded-md font-mono text-sm dark:bg-gray-700 dark:border-gray-600"></textarea>
          </div>
          
          <div class="flex justify-end space-x-3 mt-6">
            <button type="button" @click="showAddModal = false" class="px-4 py-2 border rounded-md hover:bg-gray-50 dark:hover:bg-gray-700">
              Cancel
            </button>
            <button type="submit" :disabled="submitting" class="bg-blue-600 text-white px-4 py-2 rounded-md hover:bg-blue-700 disabled:opacity-50">
              {{ submitting ? 'Creating...' : 'Create App' }}
            </button>
          </div>
        </form>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { api } from '../api'

const route = useRoute()
const siteId = route.params.id

const apps = ref([])
const loading = ref(true)
const error = ref('')
const showAddModal = ref(false)
const submitting = ref(false)

const form = ref({
  app_name: '',
  app_path: '/',
  startup_script: 'npm start',
  port: 3000,
  node_version: '20',
  env_vars: '{}'
})

const fetchApps = async () => {
  try {
    apps.value = await api.get(`/sites/${siteId}/nodejs`)
  } catch (err) {
    error.value = err.message
  } finally {
    loading.value = false
  }
}

const createApp = async () => {
  submitting.value = true
  try {
    // Validate JSON
    if (form.value.env_vars) {
      JSON.parse(form.value.env_vars)
    }
    
    await api.post(`/sites/${siteId}/nodejs`, form.value)
    showAddModal.value = false
    form.value = {
      app_name: '',
      app_path: '/',
      startup_script: 'npm start',
      port: 3000,
      node_version: '20',
      env_vars: '{}'
    }
    await fetchApps()
  } catch (err) {
    alert(`Error: ${err.message}`)
  } finally {
    submitting.value = false
  }
}

const deleteApp = async (appId) => {
  if (!confirm('Are you sure you want to delete this Node.js app?')) return
  try {
    await api.delete(`/sites/${siteId}/nodejs/${appId}`)
    await fetchApps()
  } catch (err) {
    alert(`Error deleting app: ${err.message}`)
  }
}

const restartApp = async (appId) => {
  try {
    await api.post(`/sites/${siteId}/nodejs/${appId}/restart`)
    alert('Restart command sent (Note: restart functionality requires full systemd mapping which may be pending)')
  } catch (err) {
    alert(`Error restarting app: ${err.message}`)
  }
}

onMounted(() => {
  fetchApps()
})
</script>
