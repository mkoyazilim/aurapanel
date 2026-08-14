<template>
  <div>
    <h2 class="text-2xl font-bold mb-6">Mail Server</h2>

    <div v-if="loading" class="text-gray-500">
      Loading...
    </div>
    <div v-else>
      <div v-if="error" class="bg-red-100 text-red-600 p-3 rounded-md mb-4">{{ error }}</div>
      
      <div class="mb-4 text-right">
        <button @click="showCreateModal = true" class="bg-blue-600 text-white px-4 py-2 rounded-md hover:bg-blue-700">
          + Add Email Account
        </button>
      </div>

      <div class="bg-white p-0 rounded-lg shadow-sm border border-gray-200 dark:bg-gray-800 dark:border-gray-700 overflow-hidden">
        <table class="w-full text-left border-collapse">
          <thead>
            <tr class="bg-gray-50 dark:bg-gray-700 border-b dark:border-gray-600">
              <th class="p-4 font-medium text-gray-600 dark:text-gray-300">Email Address</th>
              <th class="p-4 font-medium text-gray-600 dark:text-gray-300">Domain</th>
              <th class="p-4 font-medium text-gray-600 dark:text-gray-300">Quota (MB)</th>
              <th class="p-4 font-medium text-gray-600 dark:text-gray-300 text-right">Actions</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="a in accounts" :key="a.email" class="border-b dark:border-gray-600 hover:bg-gray-50 dark:hover:bg-gray-750">
              <td class="p-4 font-medium">{{ a.email }}</td>
              <td class="p-4 text-gray-600 dark:text-gray-400">{{ a.domain }}</td>
              <td class="p-4 text-gray-600 dark:text-gray-400">{{ a.quota_mb }}</td>
              <td class="p-4 text-right">
                <button @click="deleteAccount(a.email)" class="text-red-600 hover:text-red-800 text-sm font-medium">Delete</button>
              </td>
            </tr>
            <tr v-if="accounts.length === 0">
              <td colspan="4" class="p-8 text-center text-gray-500">
                No email accounts exist yet.
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <!-- Create Modal -->
    <div v-if="showCreateModal" class="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center p-4 z-50">
      <div class="bg-white dark:bg-gray-800 rounded-lg max-w-md w-full p-6 shadow-xl">
        <h3 class="text-lg font-bold mb-4">Add Email Account</h3>
        <form @submit.prevent="createAccount" class="space-y-4">
          <div class="flex gap-2">
            <div class="flex-1">
              <label class="block text-sm font-medium mb-1">Local Part</label>
              <input v-model="form.local_part" type="text" required placeholder="hello"
                     class="w-full px-3 py-2 border rounded-md dark:bg-gray-700 dark:border-gray-600">
            </div>
            <div class="flex-none pt-8">@</div>
            <div class="flex-1">
              <label class="block text-sm font-medium mb-1">Domain</label>
              <input v-model="form.domain" type="text" required placeholder="example.com"
                     class="w-full px-3 py-2 border rounded-md dark:bg-gray-700 dark:border-gray-600">
            </div>
          </div>
          <div>
            <label class="block text-sm font-medium mb-1">Password</label>
            <input v-model="form.password" type="password" required placeholder="Strong password"
                   class="w-full px-3 py-2 border rounded-md dark:bg-gray-700 dark:border-gray-600">
          </div>
          <div>
            <label class="block text-sm font-medium mb-1">Quota (MB)</label>
            <input v-model.number="form.quota_mb" type="number" required min="1"
                   class="w-full px-3 py-2 border rounded-md dark:bg-gray-700 dark:border-gray-600">
          </div>
          
          <div class="flex justify-end space-x-3 mt-6">
            <button type="button" @click="showCreateModal = false" class="px-4 py-2 border rounded-md">Cancel</button>
            <button type="submit" :disabled="submitting" class="bg-blue-600 text-white px-4 py-2 rounded-md hover:bg-blue-700 disabled:opacity-50">
              {{ submitting ? 'Adding...' : 'Add Account' }}
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

const accounts = ref([])
const loading = ref(true)
const error = ref('')
const showCreateModal = ref(false)
const submitting = ref(false)

const form = ref({
  domain: '',
  local_part: '',
  password: '',
  quota_mb: 512
})

const fetchAccounts = async () => {
  loading.value = true
  try {
    accounts.value = await api.get(`/sites/${siteId}/mail`)
  } catch (err) {
    error.value = err.message
  } finally {
    loading.value = false
  }
}

const createAccount = async () => {
  submitting.value = true
  try {
    await api.post(`/sites/${siteId}/mail`, form.value)
    showCreateModal.value = false
    form.value.local_part = ''
    form.value.password = ''
    await fetchAccounts()
  } catch (err) {
    alert(`Error: ${err.message}`)
  } finally {
    submitting.value = false
  }
}

const deleteAccount = async (email) => {
  if (!confirm(`Are you sure you want to delete ${email}?`)) return
  try {
    await api.delete(`/sites/${siteId}/mail/${email}`)
    await fetchAccounts()
  } catch (err) {
    alert(`Delete error: ${err.message}`)
  }
}

onMounted(() => {
  fetchAccounts()
})
</script>
