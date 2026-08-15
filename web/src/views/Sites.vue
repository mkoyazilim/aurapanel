
<template>
  <Layout>
    <div class="page">
      <h1>{{ $t('menu.sites') }}</h1>
      <div v-if="error" class="alert error">{{ error }}</div>
      <div v-if="notice" class="alert ok">{{ notice }}</div>

      <div class="card">
        <h2>{{ $t('sites.new_site') }}</h2>
        <div class="row">
          <div style="flex: 2">
            <label>{{ $t('sites.domain') }}</label>
            <input v-model="newSite.domain" :placeholder="$t('sites.placeholder_domain')" />
          </div>
          <div style="flex: 1">
            <label>{{ $t('sites.app_type', 'Proje Tipi') }}</label>
            <select v-model="newSite.appType">
              <option value="php">{{ $t('sites.app_php', 'Standart (PHP / HTML)') }}</option>
              <option value="nodejs">{{ $t('sites.app_node', 'Node.js Uygulaması') }}</option>
            </select>
          </div>
          <div style="flex: 1" v-if="newSite.appType === 'php'">
            <label>{{ $t('sites.php_version') }}</label>
            <select v-model="newSite.php">
              <option>8.2</option><option selected>8.3</option><option>8.4</option>
            </select>
          </div>
          <div style="flex: 1" v-if="newSite.appType === 'nodejs'">
            <label>{{ $t('nodejs.node_version_label', 'Node Versiyonu') }}</label>
            <select v-model="newSite.nodeVersion">
              <option value="18">18.x</option>
              <option value="20">20.x</option>
              <option value="22">22.x</option>
            </select>
          </div>
          <div style="flex: 1" v-if="newSite.appType === 'nodejs'">
            <label>{{ $t('nodejs.port_label', 'Çalışma Portu') }}</label>
            <input v-model.number="newSite.nodePort" type="number" placeholder="3000" />
          </div>
          <button class="btn primary" @click="create" :disabled="busy">{{ $t('sites.create') }}</button>
        </div>
      </div>

      <div class="card">
        <h2>{{ $t('sites.existing_sites') }}</h2>
        <table>
          <thead>
            <tr>
              <th>{{ $t('sites.id') }}</th>
              <th>{{ $t('sites.domain') }}</th>
              <th>{{ $t('sites.linux_user') }}</th>
              <th>{{ $t('sites.status') }}</th>
              <th style="text-align: right">{{ $t('common.actions') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="s in sites" :key="s.id">
              <td class="mono">{{ s.id }}</td>
              <td><strong>{{ s.name }}</strong></td>
              <td class="mono">{{ s.linux_user }}</td>
              <td><span class="badge" :class="s.status === 'active' ? 'ok' : 'warn'">{{ s.status }}</span></td>
              <td style="text-align: right; position: relative; display: flex; justify-content: flex-end; gap: 8px;">
                <router-link :to="'/sites/' + s.id + '/dashboard'" class="btn btn-sm primary">Yönet ➡</router-link>
                <div class="action-dropdown" @click.stop>
                  <button class="btn btn-sm action-btn" @click="toggleDropdown(s.id)">
                    {{ $t('common.actions', 'Hızlı İşlemler') }} ▾
                  </button>
                  <div class="dropdown-menu" v-if="activeDropdown === s.id">
                    <button class="dropdown-item" @click="openWpModal(s); activeDropdown = null">
                      <span>⚡</span> {{ $t('sites.install_wp') }}
                    </button>
                    <router-link :to="'/sites/' + s.id + '/git'" class="dropdown-item">
                      <span>🐙</span> {{ $t('sites.dropdown_git') }}
                    </router-link>
                    <router-link v-if="s.app_type === 'nodejs'" :to="'/sites/' + s.id + '/nodejs'" class="dropdown-item">
                      <span>🟢</span> {{ $t('sites.dropdown_node') }}
                    </router-link>
                    <router-link :to="'/sites/' + s.id + '/staging'" class="dropdown-item">
                      <span>🧪</span> {{ $t('sites.dropdown_staging') }}
                    </router-link>
                    <router-link :to="'/sites/' + s.id + '/cloudflare'" class="dropdown-item">
                      <span>☁️</span> {{ $t('sites.dropdown_cf') }}
                    </router-link>
                    <router-link :to="'/sites/' + s.id + '/mail'" class="dropdown-item">
                      <span>📧</span> {{ $t('sites.dropdown_mail') }}
                    </router-link>
                    <router-link :to="'/sites/' + s.id + '/waf'" class="dropdown-item">
                      <span>🛡️</span> {{ $t('sites.dropdown_waf') }}
                    </router-link>
                    <router-link :to="'/sites/' + s.id + '/cdn'" class="dropdown-item">
                      <span>⚡</span> {{ $t('sites.dropdown_cdn') }}
                    </router-link>
                    <div class="dropdown-divider"></div>
                    <button class="dropdown-item danger" @click="remove(s.id); activeDropdown = null">
                      <span>🗑️</span> {{ $t('common.delete') }}
                    </button>
                  </div>
                </div>
              </td>
            </tr>
            <tr v-if="!sites.length"><td colspan="5" class="muted">{{ $t('sites.no_sites') }}</td></tr>
          </tbody>
        </table>
      </div>

      <!-- WordPress Installer Modal -->
      <div v-if="wpModal.open" class="modal-backdrop">
        <div class="modal-card">
          <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 14px">
            <h2 style="margin: 0">⚡ {{ $t('wp.modal_title') }} ({{ wpModal.site?.name }})</h2>
            <button class="btn btn-sm" @click="wpModal.open = false">✕</button>
          </div>

          <div v-if="wpModal.result" class="alert ok">
            <h4>🎉 {{ $t('wp.install_success') }}</h4>
            <p style="margin: 6px 0 0 0">{{ $t('wp.install_success_desc') }}</p>
            <div class="mono" style="background: rgba(0,0,0,0.05); padding: 8px; border-radius: 6px; margin-top: 8px">
              <div><strong>{{ $t('sites.db_name_label') }}</strong> {{ wpModal.result.db_name }}</div>
              <div><strong>{{ $t('sites.db_user_label') }}</strong> {{ wpModal.result.db_user }}</div>
            </div>
            <div style="margin-top: 10px">
              <a :href="'http://' + wpModal.site.name" target="_blank" class="btn primary btn-sm">
                🌐 {{ $t('wp.open_site') }}
              </a>
            </div>
          </div>

          <template v-else>
            <p class="muted text-sm">{{ $t('wp.modal_desc') }}</p>

            <div style="margin: 14px 0">
              <label>{{ $t('wp.language') }}</label>
              <select v-model="wpModal.language">
                <option value="tr">{{ $t('sites.wp_lang_tr') }}</option>
                <option value="en">{{ $t('sites.wp_lang_en') }}</option>
              </select>
            </div>

            <div style="margin: 14px 0">
              <label>{{ $t('wp.table_prefix') }}</label>
              <input v-model="wpModal.tablePrefix" placeholder="wp_" />
            </div>

            <div class="alert warn text-sm">
              ℹ️ {{ $t('wp.modal_note') }}
            </div>

            <div style="display: flex; gap: 8px; justify-content: flex-end; margin-top: 16px">
              <button class="btn" @click="wpModal.open = false" :disabled="wpModal.busy">{{ $t('common.cancel') }}</button>
              <button class="btn primary" @click="installWordpress" :disabled="wpModal.busy">
                {{ wpModal.busy ? $t('wp.installing') : '⚡ ' + $t('wp.start_install') }}
              </button>
            </div>
          </template>
        </div>
      </div>
    </div>
  </Layout>
</template>

<script setup>
import { onMounted, onUnmounted, reactive, ref } from 'vue'
import Layout from '../components/Layout.vue'
import { api } from '../api'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()

const sites = ref([])
const error = ref('')
const notice = ref('')
const busy = ref(false)
const newSite = reactive({ 
  domain: '', 
  appType: 'php', 
  php: '8.3',
  nodeVersion: '20',
  nodePort: 3000
})

const activeDropdown = ref(null)
function toggleDropdown(id) {
  activeDropdown.value = activeDropdown.value === id ? null : id
}

function closeDropdown() {
  activeDropdown.value = null
}

const wpModal = reactive({
  open: false,
  site: null,
  language: 'tr',
  tablePrefix: 'wp_',
  busy: false,
  result: null
})

function openWpModal(site) {
  wpModal.site = site
  wpModal.language = 'tr'
  wpModal.tablePrefix = 'wp_'
  wpModal.busy = false
  wpModal.result = null
  wpModal.open = true
}

async function installWordpress() {
  if (!wpModal.site) return
  wpModal.busy = true
  error.value = ''
  try {
    const res = await fetch(`/api/v1/sites/${wpModal.site.id}/wordpress/install`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        language: wpModal.language,
        table_prefix: wpModal.tablePrefix
      })
    })
    if (!res.ok) {
      const data = await res.json()
      throw new Error(data.error || t('sites.wp_install_failed'))
    }
    wpModal.result = await res.json()
    notice.value = t('wp.installed_notice', { domain: wpModal.site.name })
  } catch (err) {
    error.value = err.message
  } finally {
    wpModal.busy = false
  }
}

async function load() {
  try {
    sites.value = await api('/sites')
  } catch (e) {
    error.value = e.message
  }
}

async function create() {
  busy.value = true
  error.value = ''
  notice.value = ''
  try {
    await api('/sites', {
      method: 'POST',
      body: { 
        domain: newSite.domain, 
        app_type: newSite.appType,
        php_version: newSite.php, 
        node_version: newSite.nodeVersion,
        node_port: newSite.nodePort,
        aliases: [], 
        limits: {} 
      },
    })
    newSite.domain = ''
    await load()
  } catch (e) {
    error.value = e.message
  } finally {
    busy.value = false
  }
}

async function remove(id) {
  if (!confirm(t('sites.delete_confirm', { id }))) return
  error.value = ''
  notice.value = ''
  try {
    await api(`/sites/${id}`, { method: 'DELETE' })
    await load()
  } catch (e) {
    error.value = e.message
  }
}

onMounted(load)

onUnmounted(() => {
  document.removeEventListener('click', closeDropdown)
})
</script>

<style scoped>
.modal-backdrop {
  position: fixed;
  inset: 0;
  background: rgba(15, 23, 42, 0.6);
  backdrop-filter: blur(4px);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
}

.modal-card {
  background: var(--bg-card, #ffffff);
  border: 1px solid var(--border-color, #e2e8f0);
  border-radius: 12px;
  width: 100%;
  max-width: 520px;
  padding: 24px;
  box-shadow: 0 20px 25px -5px rgba(0, 0, 0, 0.2);
}

.action-dropdown {
  display: inline-block;
  position: relative;
  text-align: left;
}
.action-btn {
  padding: 6px 12px;
  font-weight: 500;
}
.dropdown-menu {
  position: absolute;
  right: 0;
  top: 100%;
  margin-top: 4px;
  background: var(--bg-card, #ffffff);
  border: 1px solid var(--border-color, #e2e8f0);
  border-radius: 8px;
  min-width: 150px;
  box-shadow: 0 10px 15px -3px rgba(0, 0, 0, 0.1), 0 4px 6px -2px rgba(0, 0, 0, 0.05);
  z-index: 50;
  display: flex;
  flex-direction: column;
  padding: 6px;
}
.dropdown-item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 8px 12px;
  font-size: 13px;
  color: var(--text-color, #1e293b);
  text-decoration: none;
  border: none;
  background: transparent;
  cursor: pointer;
  border-radius: 6px;
  transition: background 0.15s ease;
  text-align: left;
  width: 100%;
}
.dropdown-item:hover {
  background: var(--bg-body, #f8fafc);
}
.dropdown-item.danger {
  color: #ef4444;
}
.dropdown-item.danger:hover {
  background: #fef2f2;
}
.dropdown-divider {
  height: 1px;
  background: var(--border-color, #e2e8f0);
  margin: 4px 0;
}
</style>
