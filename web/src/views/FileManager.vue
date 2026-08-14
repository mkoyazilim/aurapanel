<template>
  <Layout>
    <div class="page">
      <h1>{{ $t('menu.files') }}</h1>
      <div v-if="error" class="alert error">{{ error }}</div>

      <div class="card">
        <div class="row">
          <div style="flex: 1">
            <label>{{ $t('files.site') }}</label>
            <select v-model="siteId" @change="loadDir">
              <option v-for="s in sites" :key="s.id" :value="s.id">{{ s.name }}</option>
            </select>
          </div>
          <div class="muted mono" style="margin-top: 18px">/{{ path }}</div>
          <div class="spacer"></div>
          <button class="btn" @click="mkdir">📁 {{ $t('files.folder') }}</button>
          <button class="btn" @click="uploadInput.click()">⬆️ {{ $t('files.upload') }}</button>
          <input ref="uploadInput" type="file" style="display: none" @change="upload" />
        </div>
      </div>

      <div class="card">
        <table>
          <thead><tr><th>{{ $t('files.name') }}</th><th>{{ $t('files.size') }}</th><th>{{ $t('files.perms') }}</th><th></th></tr></thead>
          <tbody>
            <tr v-for="e in entries" :key="e.path">
              <td>
                <span v-if="e.IsDir" class="clickable" @click="enter(e)">📁 {{ e.name }}</span>
                <span v-else>{{ iconFor(e.name) }} {{ e.name }}</span>
              </td>
              <td class="mono">{{ e.IsDir ? '—' : humanSize(e.size) }}</td>
              <td class="muted">{{ new Date(e.mod_time || e.ModTime).toLocaleString() }}</td>
              <td>
                <button class="btn" v-if="!e.IsDir" @click="openEditor(e.path)">✏️ {{ $t('files.edit') }}</button>
                <button class="btn danger" @click="remove(e.path)">{{ $t('files.delete') }}</button>
              </td>
            </tr>
            <tr v-if="!entries.length"><td colspan="4" class="muted">{{ $t('files.empty') }}</td></tr>
          </tbody>
        </table>
      </div>

      <div class="card" v-if="editing">
        <div class="row">
          <h2 style="margin: 0">{{ editing }}</h2>
          <div class="spacer"></div>
          <button class="btn danger" @click="editing = null">{{ $t('files.cancel') }}</button>
          <button class="btn primary" @click="save">💾 {{ $t('files.save') }}</button>
        </div>
        <div ref="editorEl" style="height: 420px; border: 1px solid var(--border); border-radius: 8px; margin-top: 12px"></div>
      </div>
    </div>
  </Layout>
</template>

<script setup>
import { nextTick, onMounted, ref } from 'vue'
import Layout from '../components/Layout.vue'
import { api, b64decode, b64encode } from '../api'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()

const sites = ref([])
const siteId = ref('')
const path = ref('')
const entries = ref([])
const error = ref('')
const uploadInput = ref(null)
const editing = ref(null)
const editorEl = ref(null)
let editor = null
let fileHash = ''
let fileMtime = ''

function iconFor(name) {
  if (/\.(php|html|css|js|json|xml|yml|yaml|md|env|ini|conf|txt)$/i.test(name)) return '📄'
  if (/\.(png|jpe?g|gif|webp|svg)$/i.test(name)) return '🖼️'
  if (/\.(zip|tar|gz)$/i.test(name)) return '📦'
  return '📃'
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
  try {
    entries.value = await api(`/sites/${siteId.value}/files?path=${encodeURIComponent(path.value)}`)
  } catch (e) {
    error.value = e.message
  }
}

function enter(e) {
  path.value = e.path
  loadDir()
}

async function mkdir() {
  const n = prompt(t('files.new_folder_prompt'))
  if (!n) return
  try {
    await api(`/fm/${siteId.value}/mkdir`, { method: 'POST', body: { path: (path.value ? path.value + '/' : '') + n } })
    await loadDir()
  } catch (e) {
    error.value = e.message
  }
}

async function upload(ev) {
  const file = ev.target.files[0]
  if (!file) return
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
    await loadDir()
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
        theme: 'vs',
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
    error.value = ''
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
.clickable { cursor: pointer; color: var(--primary); }
</style>
