<template>
  <Layout>
    <div class="page">
      <h1>{{ $t('menu.files') }} <span style="font-size: 14px; color: var(--muted); font-weight: normal; margin-left: 8px;">Modern File Manager</span></h1>
      <div v-if="error" class="alert error">{{ error }}</div>
      <div v-if="notice" class="alert ok">{{ notice }}</div>

      <!-- Toolbar & Breadcrumb -->
      <div class="card" style="display: flex; flex-direction: column; gap: 16px;">
        <div class="row" style="align-items: flex-end;">
          <div style="flex: 0 0 200px;">
            <label style="display: block; margin-bottom: 6px; color: var(--muted); font-size: 13px;">{{ $t('files.site') }}</label>
            <select v-model="siteId" @change="loadDir" style="width: 100%; height: 38px;">
              <option v-for="s in sites" :key="s.id" :value="s.id">{{ s.name }}</option>
            </select>
          </div>
          
          <!-- Breadcrumb -->
          <div class="breadcrumb" style="flex: 1; display: flex; align-items: center; padding: 0 16px; background: var(--bg); border-radius: 6px; height: 38px; border: 1px solid var(--border);">
            <span class="clickable" @click="goToPath('')">🏠 Root</span>
            <span v-for="(p, i) in pathParts" :key="i">
              <span style="margin: 0 8px; color: var(--muted);">/</span>
              <span class="clickable" @click="goToPath(pathParts.slice(0, i+1).join('/'))">{{ p }}</span>
            </span>
          </div>
        </div>

        <div class="row" style="border-top: 1px solid var(--border); padding-top: 16px;">
          <button class="btn" @click="promptMkdir">📁 {{ $t('files.folder') }}</button>
          <button class="btn" @click="promptNewFile">📄 New File</button>
          <div class="spacer"></div>
          <button class="btn" @click="uploadInput.click()">⬆️ {{ $t('files.upload') }}</button>
          <input ref="uploadInput" type="file" multiple style="display: none" @change="upload" />
        </div>
      </div>

      <!-- File List -->
      <div class="card" @dragover.prevent="dragOver = true" @dragleave.prevent="dragOver = false" @drop.prevent="handleDrop" :class="{'drag-active': dragOver}">
        <div v-if="dragOver" class="drag-overlay">Drop files to upload...</div>
        
        <div v-if="loading" class="muted">Loading directory...</div>
        <table v-else class="file-table">
          <thead>
            <tr>
              <th style="width: 50%">{{ $t('files.name') }}</th>
              <th style="width: 15%">{{ $t('files.size') }}</th>
              <th style="width: 20%">{{ $t('files.perms') }}</th>
              <th style="width: 15%; text-align: right;">Actions</th>
            </tr>
          </thead>
          <tbody>
            <tr v-if="path !== ''" @click="goToPath(pathParts.slice(0, -1).join('/'))" class="clickable-row">
              <td colspan="4">📁 ..</td>
            </tr>
            <tr v-for="e in entries" :key="e.path" class="file-row">
              <td>
                <span v-if="e.IsDir" class="clickable" @click="enter(e)">📁 <strong>{{ e.name }}</strong></span>
                <span v-else>{{ iconFor(e.name) }} {{ e.name }}</span>
              </td>
              <td class="mono">{{ e.IsDir ? '—' : humanSize(e.size) }}</td>
              <td class="muted">{{ new Date(e.mod_time || e.ModTime).toLocaleString() }}</td>
              <td style="text-align: right;">
                <div class="dropdown">
                  <button class="btn btn-sm">⋮</button>
                  <div class="dropdown-menu">
                    <a v-if="!e.IsDir" @click.prevent="openEditor(e.path)">✏️ Edit</a>
                    <a @click.prevent="promptRename(e.path)">📝 Rename</a>
                    <a @click.prevent="promptCopy(e.path)">📄 Copy</a>
                    <a @click.prevent="promptMove(e.path)">🚚 Move</a>
                    <a v-if="!e.IsDir && isArchive(e.name)" @click.prevent="extract(e.path)">📦 Extract</a>
                    <a v-if="e.IsDir" @click.prevent="archiveDir(e.path)">📦 Zip</a>
                    <a class="text-danger" @click.prevent="remove(e.path)">🗑️ Delete</a>
                  </div>
                </div>
              </td>
            </tr>
            <tr v-if="!entries.length && path === ''"><td colspan="4" class="muted">{{ $t('files.empty') }}</td></tr>
          </tbody>
        </table>
      </div>

      <!-- Fullscreen Editor Modal -->
      <div v-if="editing" class="editor-modal">
        <div class="editor-header">
          <h2 style="margin: 0; color: white;">✏️ {{ editing }}</h2>
          <div class="spacer"></div>
          <button class="btn danger" @click="editing = null">{{ $t('files.cancel') }}</button>
          <button class="btn primary" @click="save">💾 {{ $t('files.save') }}</button>
        </div>
        <div ref="editorEl" class="editor-body"></div>
      </div>
    </div>
  </Layout>
</template>

<script setup>
import { nextTick, onMounted, ref, computed } from 'vue'
import Layout from '../components/Layout.vue'
import { api, b64decode, b64encode } from '../api'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()

const sites = ref([])
const siteId = ref('')
const path = ref('')
const entries = ref([])
const error = ref('')
const notice = ref('')
const loading = ref(false)
const dragOver = ref(false)
const uploadInput = ref(null)

const editing = ref(null)
const editorEl = ref(null)
let editor = null
let fileHash = ''
let fileMtime = ''

const pathParts = computed(() => path.value ? path.value.split('/').filter(Boolean) : [])

function iconFor(name) {
  if (/\.(php|html|css|js|json|xml|yml|yaml|md|env|ini|conf|txt)$/i.test(name)) return '📄'
  if (/\.(png|jpe?g|gif|webp|svg)$/i.test(name)) return '🖼️'
  if (/\.(zip|tar|gz|rar)$/i.test(name)) return '📦'
  return '📃'
}

function isArchive(name) {
  return /\.(zip|tar\.gz)$/i.test(name)
}

function humanSize(n) {
  if (!n) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let i = 0
  while (n >= 1024 && i < units.length - 1) { n /= 1024; i++ }
  return n.toFixed(1) + ' ' + units[i]
}

async function loadSites() {
  sites.value = await api('/sites')
  if (sites.value.length) {
    siteId.value = sites.value[0].id
    await loadDir()
  }
}

async function loadDir() {
  if (!siteId.value) return
  error.value = ''
  notice.value = ''
  loading.value = true
  try {
    entries.value = await api(`/sites/${siteId.value}/files?path=${encodeURIComponent(path.value)}`)
  } catch (e) {
    error.value = e.message
  } finally {
    loading.value = false
  }
}

function enter(e) {
  path.value = e.path
  loadDir()
}

function goToPath(p) {
  path.value = p
  loadDir()
}

function getRelPath(name) {
  return path.value ? path.value + '/' + name : name
}

async function promptMkdir() {
  const n = prompt(t('files.new_folder_prompt'))
  if (!n) return
  try {
    await api(`/sites/${siteId.value}/files/mkdir`, { method: 'POST', body: { path: getRelPath(n) } })
    await loadDir()
  } catch (e) {
    error.value = e.message
  }
}

async function promptNewFile() {
  const n = prompt('Enter new file name:')
  if (!n) return
  try {
    const relPath = getRelPath(n)
    await api(`/sites/${siteId.value}/files/content?path=${encodeURIComponent(relPath)}`, {
      method: 'PUT',
      body: { content_b64: '', expected_hash: '', expected_mtime: '' }
    })
    await loadDir()
    openEditor(relPath)
  } catch (e) {
    error.value = e.message
  }
}

async function promptRename(oldPath) {
  const newName = prompt('Enter new name:', oldPath.split('/').pop())
  if (!newName) return
  const toPath = getRelPath(newName)
  try {
    await api(`/sites/${siteId.value}/files/rename`, { method: 'POST', body: { from: oldPath, to: toPath } })
    await loadDir()
  } catch (e) {
    error.value = e.message
  }
}

async function promptCopy(oldPath) {
  const newName = prompt('Copy to:', oldPath + '-copy')
  if (!newName) return
  try {
    await api(`/sites/${siteId.value}/files/copy`, { method: 'POST', body: { from: oldPath, to: newName } })
    await loadDir()
  } catch (e) {
    error.value = e.message
  }
}

async function promptMove(oldPath) {
  const toPath = prompt('Move to path:', oldPath)
  if (!toPath || toPath === oldPath) return
  try {
    await api(`/sites/${siteId.value}/files/rename`, { method: 'POST', body: { from: oldPath, to: toPath } })
    await loadDir()
  } catch (e) {
    error.value = e.message
  }
}

async function extract(relPath) {
  if (!confirm(`Extract ${relPath} to current directory?`)) return
  try {
    const format = relPath.endsWith('.tar.gz') ? 'tar.gz' : 'zip'
    await api(`/sites/${siteId.value}/archive`, { 
      method: 'POST', 
      body: { action: 'extract', format: format, target: `${relPath}|${path.value}` } 
    })
    notice.value = `Extracted ${relPath} successfully.`
    await loadDir()
  } catch (e) {
    error.value = e.message
  }
}

async function archiveDir(relPath) {
  try {
    await api(`/sites/${siteId.value}/archive`, { 
      method: 'POST', 
      body: { action: 'create', format: 'zip', target: relPath + '.zip', sources: [relPath] } 
    })
    notice.value = `Zipped ${relPath} successfully.`
    await loadDir()
  } catch (e) {
    error.value = e.message
  }
}

async function handleDrop(ev) {
  dragOver.value = false
  const files = ev.dataTransfer.files
  if (!files.length) return
  for(let i=0; i<files.length; i++) {
    await processUpload(files[i])
  }
  await loadDir()
}

async function upload(ev) {
  const files = ev.target.files
  if (!files.length) return
  for(let i=0; i<files.length; i++) {
    await processUpload(files[i])
  }
  await loadDir()
  uploadInput.value.value = ''
}

async function processUpload(file) {
  error.value = ''
  try {
    const init = await api('/upload/init', {
      method: 'POST',
      body: { site_id: siteId.value, dir: path.value, file_name: file.name, total_size: file.size },
    })
    const chunkSize = 4 * 1024 * 1024
    for (let i = 0; i < file.size; i += chunkSize) {
      const chunk = file.slice(i, i + chunkSize)
      const buf = await chunk.arrayBuffer()
      let bin = ''
      new Uint8Array(buf).forEach((b) => (bin += String.fromCharCode(b)))
      await api('/upload/chunk', {
        method: 'POST',
        body: { upload_id: init.upload_id, index: i / chunkSize, data_b64: btoa(bin), sha256: '' },
      })
    }
    await api('/upload/finalize', { method: 'POST', body: { upload_id: init.upload_id } })
    notice.value = `Uploaded ${file.name} successfully.`
  } catch (e) {
    error.value = e.message
  }
}

async function openEditor(relPath) {
  error.value = ''
  try {
    const out = await api(`/sites/${siteId.value}/files/content?path=${encodeURIComponent(relPath)}`)
    editing.value = relPath
    fileHash = out.sha256
    fileMtime = out.modified_at
    await nextTick()
    const monaco = await import('monaco-editor')
    if (!editor) {
      editor = monaco.editor.create(editorEl.value, {
        value: b64decode(out.content_b64),
        language: 'plaintext',
        theme: 'vs-dark',
        automaticLayout: true,
        minimap: { enabled: false },
      })
    } else {
      editor.setValue(b64decode(out.content_b64))
    }
  } catch (e) {
    error.value = e.message
  }
}

async function save() {
  if (!editing.value || !editor) return
  error.value = ''
  try {
    await api(`/sites/${siteId.value}/files/content?path=${encodeURIComponent(editing.value)}`, {
      method: 'PUT',
      body: {
        content_b64: b64encode(editor.getValue()),
        expected_hash: fileHash,
        expected_mtime: fileMtime,
      },
    })
    const out = await api(`/sites/${siteId.value}/files/content?path=${encodeURIComponent(editing.value)}`)
    fileHash = out.sha256
    fileMtime = out.modified_at
    alert(t('files.saved'))
  } catch (e) {
    error.value = e.message
  }
}

async function remove(relPath) {
  if (!confirm(t('files.delete_confirm', { name: relPath }))) return
  try {
    await api(`/sites/${siteId.value}/files/delete`, { method: 'POST', body: { path: relPath } })
    await loadDir()
  } catch (e) {
    error.value = e.message
  }
}

onMounted(loadSites)
</script>

<style scoped>
.clickable { cursor: pointer; color: var(--primary); font-weight: 500; }
.clickable:hover { text-decoration: underline; }
.align-center { align-items: center; }

/* File Table */
.file-table {
  width: 100%;
  border-collapse: collapse;
}
.file-table th {
  text-align: left;
  padding: 12px 16px;
  border-bottom: 2px solid var(--border);
  color: var(--muted);
  font-weight: 500;
  font-size: 13px;
}
.file-row {
  border-bottom: 1px solid var(--border);
  transition: background 0.2s;
}
.file-row:hover { background: rgba(0,0,0,0.02); }
.file-table td {
  padding: 12px 16px;
  font-size: 14px;
}
.clickable-row { cursor: pointer; transition: background 0.2s; }
.clickable-row:hover { background: rgba(0,0,0,0.04); }

/* Dropdown */
.dropdown { position: relative; display: inline-block; }
.dropdown-menu {
  display: none;
  position: absolute;
  right: 0;
  top: 100%;
  background: #fff;
  min-width: 160px;
  box-shadow: 0 4px 12px rgba(0,0,0,0.1);
  border-radius: 6px;
  border: 1px solid var(--border);
  z-index: 10;
}
.dropdown:hover .dropdown-menu { display: block; }
.dropdown-menu a {
  display: block;
  padding: 8px 16px;
  text-decoration: none;
  color: var(--text);
  font-size: 13px;
  cursor: pointer;
}
.dropdown-menu a:hover { background: var(--bg); }
.text-danger { color: #dc3545 !important; }

/* Drag & Drop */
.drag-active { border: 2px dashed var(--primary); position: relative; }
.drag-overlay {
  position: absolute; inset: 0; background: rgba(255,255,255,0.8);
  display: flex; align-items: center; justify-content: center;
  font-size: 20px; font-weight: bold; color: var(--primary); z-index: 5;
}

/* Modal Editor */
.editor-modal {
  position: fixed; inset: 0; background: #1e1e1e;
  z-index: 9999; display: flex; flex-direction: column;
}
.editor-header {
  padding: 16px 24px; background: #252526;
  border-bottom: 1px solid #333; display: flex; align-items: center;
}
.editor-body { flex: 1; min-height: 0; }
</style>
