<template>
  <div class="space-y-6">
    <div class="flex flex-col md:flex-row md:items-center justify-between gap-4">
      <div>
        <h1 class="text-2xl font-bold text-white flex items-center gap-3">
          <FolderOpen class="w-7 h-7 text-orange-400" />
          {{ t('filemanager.title') }}
        </h1>
        <p class="text-gray-400 mt-1">{{ t('filemanager.subtitle') }}</p>
      </div>
      <div class="flex gap-2">
        <button v-if="selectedFiles.length > 0" @click="showCompressModal = true" class="px-4 py-2 bg-blue-600/20 text-blue-400 border border-blue-600/30 rounded-lg text-sm hover:bg-blue-600/40 transition">{{ t('filemanager.compress') }}</button>
        <button v-if="selectedFiles.length > 0" @click="trashSelectedItems" class="px-4 py-2 bg-red-600/20 text-red-400 border border-red-600/30 rounded-lg text-sm hover:bg-red-600/40 transition">{{ t('filemanager.trash') }}</button>
        <button v-if="selectedFiles.length > 0" @click="deleteSelectedItems" class="px-4 py-2 bg-red-700/30 text-red-300 border border-red-500/40 rounded-lg text-sm hover:bg-red-700/50 transition">{{ t('filemanager.delete') }}</button>
        <button @click="showUploadModal = true" class="px-4 py-2 bg-gradient-to-r from-orange-600 to-amber-600 text-white rounded-lg text-sm hover:from-orange-700 hover:to-amber-700 transition">{{ t('filemanager.upload') }}</button>
        <button @click="showNewModal = true" class="px-4 py-2 bg-panel-hover text-gray-300 rounded-lg text-sm hover:bg-gray-600 transition">{{ t('filemanager.new_file') }}</button>
        <button @click="showNewFolderModal = true" class="px-4 py-2 bg-panel-hover text-gray-300 rounded-lg text-sm hover:bg-gray-600 transition">{{ t('filemanager.new_folder') }}</button>
      </div>
    </div>

    <!-- Breadcrumb -->
    <div class="flex items-center gap-1 text-sm bg-panel-card border border-panel-border rounded-lg px-4 py-2.5">
      <button v-for="(part, i) in breadcrumb" :key="i" @click="navigateTo(i)"
        :class="['hover:text-orange-400 transition', i === breadcrumb.length - 1 ? 'text-orange-400 font-semibold' : 'text-gray-400']">
        {{ part || '/' }}
      </button>
      <span v-if="breadcrumb.length > 1" class="text-gray-600 mx-1">/</span>
    </div>

    <!-- File List -->
    <div class="bg-panel-card border border-panel-border rounded-xl overflow-hidden">
      <div v-if="loading" class="p-8 text-center text-gray-400">{{ t('common.loading') }}</div>
      <table v-else class="w-full text-sm">
        <thead>
          <tr class="text-gray-400 border-b border-panel-border">
            <th class="text-left px-4 py-3 font-medium w-8">
              <input type="checkbox" :checked="allSelected" @change="toggleAll" class="accent-orange-500">
            </th>
            <th class="text-left px-4 py-3 font-medium">{{ t('filemanager.name') }}</th>
            <th class="text-left px-4 py-3 font-medium">{{ t('filemanager.size') }}</th>
            <th class="text-left px-4 py-3 font-medium">{{ t('filemanager.permissions') }}</th>
            <th class="text-left px-4 py-3 font-medium">{{ t('filemanager.modified') }}</th>
            <th class="text-right px-4 py-3 font-medium">{{ t('filemanager.actions') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-if="currentPath !== '/home'" class="border-b border-panel-border/50 hover:bg-white/[0.02] transition cursor-pointer" @click="goUp">
            <td class="px-4 py-2.5"></td>
            <td class="px-4 py-2.5 text-gray-400 font-mono">..</td>
            <td colspan="4"></td>
          </tr>
          <tr v-for="item in fileList" :key="item.name" class="border-b border-panel-border/50 hover:bg-white/[0.02] transition">
            <td class="px-4 py-2.5">
              <input type="checkbox" :value="item.name" v-model="selectedFiles" class="accent-orange-500">
            </td>
            <td class="px-4 py-2.5">
              <button @click="item.is_dir ? enterDir(item.name) : editFile(item)" class="flex items-center gap-2 text-white hover:text-orange-400 transition font-mono text-xs">
                <span>{{ item.is_dir ? t('filemanager.folder_badge') : fileIcon(item.name) }}</span>
                {{ item.name }}
              </button>
            </td>
            <td class="px-4 py-2.5 text-gray-400 text-xs">{{ item.is_dir ? '--' : formatBytes(item.size) }}</td>
            <td class="px-4 py-2.5 text-gray-400 font-mono text-xs">{{ item.permissions }}</td>
            <td class="px-4 py-2.5 text-gray-400 text-xs">{{ formatModified(item.modified) }}</td>
            <td class="px-4 py-2.5 text-right">
              <div class="flex justify-end gap-1">
                <button v-if="isArchive(item.name)" @click="extractItem(item)" class="px-2 py-1 bg-purple-600/20 text-purple-400 rounded text-xs hover:bg-purple-600/40 transition" :title="t('filemanager.extract')">{{ t('filemanager.extract') }}</button>
                <button v-if="!item.is_dir" @click="editFile(item)" class="px-2 py-1 bg-blue-600/20 text-blue-400 rounded text-xs hover:bg-blue-600/40 transition">{{ t('filemanager.edit') }}</button>
                <button @click="editPermissions(item)" class="px-2 py-1 bg-slate-600/30 text-slate-200 rounded text-xs hover:bg-slate-600/50 transition">{{ t('filemanager.chmod') }}</button>
                <button @click="renameItem(item)" class="px-2 py-1 bg-yellow-600/20 text-yellow-400 rounded text-xs hover:bg-yellow-600/40 transition">{{ t('filemanager.rename') }}</button>
                <button @click="trashSingleItem(item)" class="px-2 py-1 bg-red-600/20 text-red-400 rounded text-xs hover:bg-red-600/40 transition">{{ t('filemanager.trash') }}</button>
                <button @click="deleteSingleItem(item)" class="px-2 py-1 bg-red-700/30 text-red-300 rounded text-xs hover:bg-red-700/50 transition">{{ t('filemanager.delete') }}</button>
              </div>
            </td>
          </tr>
          <tr v-if="fileList.length === 0" class="border-b border-panel-border/50"><td colspan="6" class="p-8 text-center text-gray-500">{{ t('filemanager.empty_dir') }}</td></tr>
        </tbody>
      </table>
    </div>

    <!-- Editor Modal -->
    <div v-if="showEditor" class="fixed inset-0 bg-black/70 backdrop-blur-sm flex items-center justify-center z-50">
      <div class="bg-panel-card border border-panel-border rounded-2xl w-full max-w-4xl h-[80vh] flex flex-col shadow-2xl">
        <div class="flex items-center justify-between p-4 border-b border-panel-border">
          <h3 class="text-white font-semibold font-mono text-sm">{{ editingFile?.name }}</h3>
          <div class="flex gap-2">
            <button @click="saveFile" class="px-4 py-1.5 bg-green-600 text-white rounded-lg text-sm hover:bg-green-700 transition">{{ t('common.save') }}</button>
            <button @click="showEditor = false" class="px-4 py-1.5 bg-panel-hover text-gray-300 rounded-lg text-sm hover:bg-gray-600 transition">{{ t('common.close') }}</button>
          </div>
        </div>
        <textarea v-model="editorContent" class="flex-1 p-4 bg-[#0d1117] text-green-400 font-mono text-sm resize-none focus:outline-none" spellcheck="false"></textarea>
      </div>
    </div>
    <!-- Upload Modal -->
    <div v-if="showUploadModal" class="fixed inset-0 bg-black/60 backdrop-blur-sm flex items-center justify-center z-50" @click.self="closeUploadModal">
      <div class="bg-panel-card border border-panel-border rounded-2xl w-full max-w-md p-6 shadow-2xl">
        <h3 class="text-xl font-bold text-white mb-4">{{ t('filemanager.upload') }}</h3>
        <div
          class="border-2 border-dashed border-panel-border rounded-xl p-8 text-center hover:border-orange-500 transition cursor-pointer"
          @click="triggerUploadSelect"
          @dragover.prevent
          @drop.prevent="handleUploadDrop"
        >
          <p class="text-gray-400">{{ t('filemanager.drag_drop') }}</p>
          <input ref="uploadInputRef" type="file" multiple class="hidden" @change="handleUploadSelection">
          <p v-if="uploadQueue.length" class="mt-3 text-xs text-orange-300 break-all">{{ uploadQueue.map(file => file.name).join(', ') }}</p>
        </div>
        <div class="mt-4 flex gap-3">
          <button
            @click="uploadSelectedFiles"
            :disabled="uploading || uploadQueue.length === 0"
            class="flex-1 py-2.5 bg-gradient-to-r from-orange-600 to-amber-600 text-white rounded-lg font-medium disabled:opacity-50 disabled:cursor-not-allowed"
          >
            {{ uploading ? t('common.loading') : t('filemanager.upload') }}
          </button>
          <button @click="closeUploadModal" class="px-4 py-2.5 bg-panel-hover text-gray-300 rounded-lg font-medium hover:bg-gray-600 transition">{{ t('common.close') }}</button>
        </div>
      </div>
    </div>

    <!-- Compress Modal -->
    <div v-if="showCompressModal" class="fixed inset-0 bg-black/60 backdrop-blur-sm flex items-center justify-center z-50" @click.self="showCompressModal = false">
      <div class="bg-panel-card border border-panel-border rounded-2xl w-full max-w-sm p-6 shadow-2xl">
        <h3 class="text-lg font-bold text-white mb-4">{{ t('filemanager.compress_title') }}</h3>
        <input v-model="compressName" type="text" :placeholder="t('filemanager.placeholders.archive_name')" class="aura-input mb-4">
        <select v-model="compressFormat" class="aura-input mb-4">
          <option value="zip">.zip</option>
          <option value="tar.gz">.tar.gz</option>
        </select>
        <div class="flex gap-3">
          <button @click="compressSelected" class="flex-1 py-2 bg-blue-600 text-white rounded-lg text-sm">{{ t('common.create') }}</button>
          <button @click="showCompressModal = false" class="px-4 py-2 bg-panel-hover text-gray-300 rounded-lg text-sm">{{ t('common.cancel') }}</button>
        </div>
      </div>
    </div>

    <!-- New File Modal -->
    <div v-if="showNewModal" class="fixed inset-0 bg-black/60 backdrop-blur-sm flex items-center justify-center z-50" @click.self="showNewModal = false">
      <div class="bg-panel-card border border-panel-border rounded-2xl w-full max-w-sm p-6 shadow-2xl">
        <h3 class="text-lg font-bold text-white mb-4">{{ t('filemanager.new_file') }}</h3>
        <input v-model="newFileName" type="text" :placeholder="t('filemanager.placeholders.file_name')" class="aura-input">
        <div class="flex gap-3 mt-4">
          <button @click="createFile" class="flex-1 py-2 bg-gradient-to-r from-orange-600 to-amber-600 text-white rounded-lg text-sm">{{ t('common.create') }}</button>
          <button @click="showNewModal = false" class="px-4 py-2 bg-panel-hover text-gray-300 rounded-lg text-sm">{{ t('common.cancel') }}</button>
        </div>
      </div>
    </div>

    <!-- New Folder Modal -->
    <div v-if="showNewFolderModal" class="fixed inset-0 bg-black/60 backdrop-blur-sm flex items-center justify-center z-50" @click.self="showNewFolderModal = false">
      <div class="bg-panel-card border border-panel-border rounded-2xl w-full max-w-sm p-6 shadow-2xl">
        <h3 class="text-lg font-bold text-white mb-4">{{ t('filemanager.new_folder') }}</h3>
        <input v-model="newFolderName" type="text" :placeholder="t('filemanager.placeholders.folder_name')" class="aura-input">
        <div class="flex gap-3 mt-4">
          <button @click="createFolder" class="flex-1 py-2 bg-gradient-to-r from-orange-600 to-amber-600 text-white rounded-lg text-sm">{{ t('common.create') }}</button>
          <button @click="showNewFolderModal = false" class="px-4 py-2 bg-panel-hover text-gray-300 rounded-lg text-sm">{{ t('common.cancel') }}</button>
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
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { FolderOpen } from 'lucide-vue-next'
import api from '../services/api'

const { t } = useI18n()
const route = useRoute()

function resolveRoutePath(query) {
  const requestedPath = String(query?.path || '').trim()
  if (requestedPath.startsWith('/')) {
    return requestedPath
  }

  const requestedDomain = String(query?.domain || '').trim().toLowerCase()
  if (requestedDomain) {
    return `/home/${requestedDomain}/public_html`
  }

  return '/home'
}

const currentPath = ref(resolveRoutePath(route.query))
const fileList = ref([])
const selectedFiles = ref([])
const loading = ref(false)
const notification = ref(null)

const showEditor = ref(false)
const showUploadModal = ref(false)
const showNewModal = ref(false)
const showNewFolderModal = ref(false)
const showCompressModal = ref(false)
const uploadInputRef = ref(null)
const uploadQueue = ref([])
const uploading = ref(false)

const editingFile = ref(null)
const editorContent = ref('')
const newFileName = ref('')
const newFolderName = ref('')
const compressName = ref('archive')
const compressFormat = ref('zip')

const breadcrumb = computed(() => currentPath.value.split('/').filter(Boolean))
const allSelected = computed(() => fileList.value.length > 0 && selectedFiles.value.length === fileList.value.length)

const toggleAll = (e) => {
  if (e.target.checked) {
    selectedFiles.value = fileList.value.map(f => f.name)
  } else {
    selectedFiles.value = []
  }
}

const showNotif = (msg, type = 'success') => {
  notification.value = { message: msg, type }
  setTimeout(() => notification.value = null, 3000)
}

const apiErrorMessage = (error) => {
  const responseData = error?.response?.data
  if (typeof responseData?.message === 'string' && responseData.message.trim()) {
    return responseData.message.trim()
  }
  if (typeof responseData?.error === 'string' && responseData.error.trim()) {
    return responseData.error.trim()
  }
  return t('common.error')
}

const formatBytes = (bytes) => {
  if (bytes === 0) return '0 B'
  const k = 1024, sizes = ['B', 'KB', 'MB', 'GB', 'TB'], i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i]
}

const isArchive = (name) => {
  const ext = name.split('.').pop().toLowerCase()
  return ['zip', 'gz', 'tar', 'tgz'].includes(ext)
}

const fileIcon = (name) => {
  const ext = name.split('.').pop().toLowerCase()
  const map = { php: 'PHP', js: 'JS', html: 'HTML', css: 'CSS', py: 'PY', txt: 'TXT', json: 'JSON', xml: 'XML', log: 'LOG', sh: 'SH', conf: 'CONF', sql: 'SQL', zip: 'ZIP', gz: 'GZ', tar: 'TAR', tgz: 'TGZ', jpg: 'IMG', png: 'IMG', svg: 'IMG', md: 'MD' }
  return map[ext] || 'FILE'
}

const symbolicPermissionsToOctal = (value) => {
  const raw = String(value || '').trim()
  if (/^[0-7]{3,4}$/.test(raw)) return raw.slice(-3)
  const normalized = raw.length === 10 ? raw.slice(1) : raw
  if (!/^[rwx-]{9}$/.test(normalized)) return '644'
  const triads = [normalized.slice(0, 3), normalized.slice(3, 6), normalized.slice(6, 9)]
  const convertTriad = (triad) => {
    let score = 0
    if (triad[0] === 'r') score += 4
    if (triad[1] === 'w') score += 2
    if (triad[2] === 'x') score += 1
    return String(score)
  }
  return triads.map(convertTriad).join('')
}

const formatModified = (value) => {
  const raw = Number(value)
  if (!Number.isFinite(raw) || raw <= 0) return '-'
  const timestampMs = raw < 1e12 ? raw * 1000 : raw
  return new Date(timestampMs).toLocaleString()
}

const loadFiles = async () => {
  loading.value = true
  selectedFiles.value = []
  try {
    const res = await api.post('/files/list', { path: currentPath.value })
    fileList.value = res.data.data || []
  } catch (e) {
    showNotif(t('common.error'), 'error')
    fileList.value = []
  } finally {
    loading.value = false
  }
}

const enterDir = (name) => {
  currentPath.value = currentPath.value.replace(/\/$/, '') + '/' + name
  loadFiles()
}

const goUp = () => {
  const parts = currentPath.value.split('/').filter(Boolean)
  parts.pop()
  currentPath.value = '/' + parts.join('/') || '/'
  loadFiles()
}

const navigateTo = (i) => {
  const parts = breadcrumb.value.slice(0, i + 1)
  currentPath.value = '/' + parts.join('/')
  loadFiles()
}

const editFile = async (item) => {
  editingFile.value = item
  try {
    const path = currentPath.value.replace(/\/$/, '') + '/' + item.name
    const res = await api.post('/files/read', { path })
    editorContent.value = res.data.data || ''
    showEditor.value = true
  } catch (e) {
    showNotif(t('filemanager.messages.read_error'), 'error')
  }
}

const saveFile = async () => {
  try {
    const path = currentPath.value.replace(/\/$/, '') + '/' + editingFile.value.name
    await api.post('/files/write', { path, content: editorContent.value })
    showNotif(t('filemanager.save_success'))
    showEditor.value = false
    loadFiles()
  } catch (e) {
    showNotif(t('common.error'), 'error')
  }
}

const renameItem = async (item) => {
  const newName = prompt(t('common.name') + ':', item.name)
  if (newName && newName !== item.name) {
    try {
      const old_path = currentPath.value.replace(/\/$/, '') + '/' + item.name
      const new_path = currentPath.value.replace(/\/$/, '') + '/' + newName
      await api.post('/files/rename', { old_path, new_path })
      showNotif(t('filemanager.messages.rename_success'))
      loadFiles()
    } catch {
      showNotif(t('filemanager.messages.rename_error'), 'error')
    }
  }
}

const editPermissions = async (item) => {
  const suggested = symbolicPermissionsToOctal(item.permissions)
  const input = prompt(`${t('filemanager.permissions')} (644 / 755 / 775):`, suggested)
  if (input == null) return
  const mode = String(input).trim()
  if (!/^[0-7]{3,4}$/.test(mode)) {
    showNotif(t('filemanager.messages.perm_invalid_example'), 'error')
    return
  }
  try {
    const path = currentPath.value.replace(/\/$/, '') + '/' + item.name
    await api.post('/files/chmod', { path, mode })
    showNotif(t('filemanager.messages.perm_updated'))
    loadFiles()
  } catch {
    showNotif(t('filemanager.messages.perm_update_failed'), 'error')
  }
}

const trashSingleItem = async (item) => {
  if (!confirm(`${t('common.confirm_delete')} (${item.name})`)) return
  try {
    const path = currentPath.value.replace(/\/$/, '') + '/' + item.name
    await api.post('/files/trash', { path })
    showNotif(t('filemanager.messages.trash_success'))
    loadFiles()
  } catch {
    showNotif(t('filemanager.messages.trash_error'), 'error')
  }
}

const trashSelectedItems = async () => {
  if (!selectedFiles.value.length || !confirm(t('filemanager.messages.trash_confirm'))) return
  try {
    for (const name of selectedFiles.value) {
      const path = currentPath.value.replace(/\/$/, '') + '/' + name
      await api.post('/files/trash', { path })
    }
    showNotif(t('filemanager.messages.trash_success'))
    loadFiles()
  } catch {
    showNotif(t('filemanager.messages.trash_partial_error'), 'error')
    loadFiles()
  }
}

const deleteSingleItem = async (item) => {
  if (!confirm(`${t('filemanager.messages.delete_permanent_confirm')} (${item.name})`)) return
  try {
    const path = currentPath.value.replace(/\/$/, '') + '/' + item.name
    await api.post('/files/delete', { path })
    showNotif(t('filemanager.messages.delete_permanent_success'))
    loadFiles()
  } catch {
    showNotif(t('filemanager.messages.delete_permanent_error'), 'error')
  }
}

const deleteSelectedItems = async () => {
  if (!selectedFiles.value.length || !confirm(t('filemanager.messages.delete_selected_permanent_confirm'))) return
  try {
    for (const name of selectedFiles.value) {
      const path = currentPath.value.replace(/\/$/, '') + '/' + name
      await api.post('/files/delete', { path })
    }
    showNotif(t('filemanager.messages.delete_selected_permanent_success'))
    loadFiles()
  } catch {
    showNotif(t('filemanager.messages.delete_selected_permanent_error'), 'error')
    loadFiles()
  }
}
const compressSelected = async () => {
  if (!compressName.value) return
  try {
    const dest_path = currentPath.value.replace(/\/$/, '') + '/' + compressName.value + '.' + compressFormat.value
    const sources = selectedFiles.value.map(name => currentPath.value.replace(/\/$/, '') + '/' + name)
    await api.post('/files/compress', { format: compressFormat.value, dest_path, sources })
    showNotif(t('filemanager.messages.compress_success'))
    showCompressModal.value = false
    loadFiles()
  } catch (e) {
    showNotif(t('filemanager.messages.compress_error'), 'error')
  }
}

const extractItem = async (item) => {
  try {
    const archive_path = currentPath.value.replace(/\/$/, '') + '/' + item.name
    const dest_dir = currentPath.value
    await api.post('/files/extract', { archive_path, dest_dir })
    showNotif(t('filemanager.messages.extract_success'))
    loadFiles()
  } catch (e) {
    showNotif(t('filemanager.messages.extract_error'), 'error')
  }
}

const createFile = async () => {
  if (newFileName.value) {
    try {
      const path = currentPath.value.replace(/\/$/, '') + '/' + newFileName.value
      await api.post('/files/write', { path, content: '' })
      showNotif(t('filemanager.messages.create_file_success'))
      newFileName.value = ''
      showNewModal.value = false
      loadFiles()
    } catch {
      showNotif(t('filemanager.messages.create_file_error'), 'error')
    }
  }
}

const createFolder = async () => {
  if (newFolderName.value) {
    try {
      const path = currentPath.value.replace(/\/$/, '') + '/' + newFolderName.value
      await api.post('/files/create_dir', { path })
      showNotif(t('filemanager.messages.create_folder_success'))
      newFolderName.value = ''
      showNewFolderModal.value = false
      loadFiles()
    } catch {
      showNotif(t('filemanager.messages.create_folder_error'), 'error')
    }
  }
}

const triggerUploadSelect = () => {
  uploadInputRef.value?.click()
}

const handleUploadSelection = (event) => {
  uploadQueue.value = Array.from(event?.target?.files || [])
}

const handleUploadDrop = (event) => {
  uploadQueue.value = Array.from(event?.dataTransfer?.files || [])
}

const resetUploadState = () => {
  uploadQueue.value = []
  if (uploadInputRef.value) {
    uploadInputRef.value.value = ''
  }
}

const closeUploadModal = () => {
  showUploadModal.value = false
  uploading.value = false
  resetUploadState()
}

const uploadSelectedFiles = async () => {
  if (!uploadQueue.value.length) return
  uploading.value = true
  try {
    const formData = new FormData()
    formData.append('path', currentPath.value)
    uploadQueue.value.forEach(file => formData.append('files', file))
    await api.post('/files/upload', formData)
    showNotif(t('filemanager.upload_success'))
    closeUploadModal()
    loadFiles()
  } catch (e) {
    showNotif(apiErrorMessage(e), 'error')
  } finally {
    uploading.value = false
  }
}

watch(
  () => route.query,
  (query) => {
    const nextPath = resolveRoutePath(query)
    if (nextPath && nextPath !== currentPath.value) {
      currentPath.value = nextPath
      loadFiles()
    }
  },
)

onMounted(loadFiles)
</script>

