<template>
  <div>
    <h2 class="text-2xl font-bold mb-6">Staging Environment</h2>

    <div v-if="loading" class="text-gray-500">
      Loading...
    </div>
    <div v-else>
      <div v-if="error" class="bg-red-100 text-red-600 p-3 rounded-md mb-4">{{ error }}</div>

      <div class="bg-white p-6 rounded-lg shadow-sm border border-gray-200 dark:bg-gray-800 dark:border-gray-700">
        <h3 class="text-lg font-bold mb-2">Active Staging</h3>
        <p class="text-gray-600 dark:text-gray-400 mb-6 text-sm">
          A staging environment allows you to test changes before pushing them to production.
        </p>

        <div v-if="envs.length > 0" class="space-y-4">
          <div v-for="env in envs" :key="env.id" class="border p-4 rounded-md flex justify-between items-center dark:border-gray-700">
            <div>
              <p class="font-bold">Staging Site ID: {{ env.staging_site_id }}</p>
              <p class="text-xs text-gray-500">Created At: {{ new Date(env.created_at).toLocaleString() }}</p>
              <span class="inline-block mt-2 bg-green-100 text-green-800 text-xs px-2 py-1 rounded-full uppercase font-semibold">
                {{ env.status }}
              </span>
            </div>
            <div class="space-x-3">
              <button @click="pushToProduction(env.id)" class="btn bg-blue-600 text-white px-4 py-2 hover:bg-blue-700">
                🚀 Push to Production
              </button>
              <button @click="deleteStaging(env.id)" class="btn danger px-4 py-2">
                Delete Staging
              </button>
            </div>
          </div>
        </div>

        <div v-else class="text-center py-8">
          <p class="text-gray-500 mb-4">No staging environment exists for this site.</p>
          <button @click="showCreateModal = true" class="bg-indigo-600 text-white px-4 py-2 rounded hover:bg-indigo-700">
            Create Staging Environment
          </button>
        </div>
      </div>
    </div>

    <!-- Create Modal -->
    <div v-if="showCreateModal" class="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center p-4 z-50">
      <div class="bg-white dark:bg-gray-800 rounded-lg max-w-md w-full p-6 shadow-xl">
        <h3 class="text-lg font-bold mb-4">Create Staging Environment</h3>
        <p class="text-sm text-gray-600 dark:text-gray-400 mb-4">
          This will create a clone of your site files. You must provide a domain or subdomain for the staging site (e.g. staging.example.com).
        </p>
        <form @submit.prevent="createStaging" class="space-y-4">
          <div>
            <label class="block text-sm font-medium mb-1">Staging Domain</label>
            <input v-model="form.domain" type="text" required placeholder="staging.example.com"
                   class="w-full px-3 py-2 border rounded-md dark:bg-gray-700 dark:border-gray-600">
          </div>
          
          <div class="flex justify-end space-x-3 mt-6">
            <button type="button" @click="showCreateModal = false" class="px-4 py-2 border rounded-md">Cancel</button>
            <button type="submit" :disabled="submitting" class="bg-indigo-600 text-white px-4 py-2 rounded-md hover:bg-indigo-700 disabled:opacity-50">
              {{ submitting ? 'Creating...' : 'Create' }}
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

const envs = ref([])
const loading = ref(true)
const error = ref('')
const showCreateModal = ref(false)
const submitting = ref(false)

const form = ref({ domain: '' })

const fetchStaging = async () => {
  try {
    envs.value = await api.get(`/sites/${siteId}/staging`)
  } catch (err) {
    error.value = err.message
  } finally {
    loading.value = false
  }
}

const createStaging = async () => {
  submitting.value = true
  try {
    await api.post(`/sites/${siteId}/staging`, form.value)
    showCreateModal.value = false
    form.value.domain = ''
    await fetchStaging()
  } catch (err) {
    alert(`Error: ${err.message}`)
  } finally {
    submitting.value = false
  }
}

const pushToProduction = async (envId) => {
  if (!confirm('Are you sure you want to push all staging changes to production? This will overwrite production files.')) return
  try {
    await api.post(`/sites/${siteId}/staging/push`)
    alert('Successfully pushed staging to production')
  } catch (err) {
    alert(`Push error: ${err.message}`)
  }
}

const deleteStaging = async (envId) => {
  if (!confirm('Are you sure you want to delete the staging environment?')) return
  try {
    await api.delete(`/sites/${siteId}/staging/${envId}`)
    await fetchStaging()
  } catch (err) {
    alert(`Delete error: ${err.message}`)
  }
}

onMounted(() => {
  fetchStaging()
})
</script>
