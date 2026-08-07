<template>
  <div class="flex h-[calc(100vh-140px)] gap-0">
    <!-- Left Sidebar: Log Categories -->
    <div class="w-72 flex-shrink-0 border-r border-panel-border bg-panel-dark/50 overflow-y-auto">
      <div class="p-4 border-b border-panel-border">
        <h2 class="text-sm font-semibold text-gray-300 uppercase tracking-wider">{{ t('log_viewer.categories') }}</h2>
      </div>

      <div class="py-1">
        <div v-for="group in catalogGroups" :key="group.key">
          <button
            @click="toggleGroup(group.key)"
            class="w-full flex items-center gap-2 px-4 py-2 text-sm text-gray-400 hover:text-white hover:bg-panel-dark transition"
          >
            <component :is="expandedGroups.has(group.key) ? ChevronDown : ChevronRight" class="w-4 h-4 flex-shrink-0" />
            <component :is="group.icon" class="w-4 h-4 flex-shrink-0" />
            <span>{{ t(`log_viewer.groups.${group.key}`) }}</span>
          </button>

          <div v-if="expandedGroups.has(group.key)" class="ml-6 border-l border-panel-border/50">
            <button
              v-for="source in group.sources"
              :key="source.id"
              @click="selectSource(source)"
              :class="[
                'w-full text-left px-4 py-1.5 text-sm transition truncate block',
                selectedSource?.id === source.id
                  ? 'text-green-400 bg-green-500/10 border-r-2 border-green-400'
                  : 'text-gray-400 hover:text-white'
              ]"
            >
              {{ source.name }}
            </button>
          </div>
        </div>

        <!-- Site Logs -->
        <div v-if="catalog?.sites?.length" class="mt-2">
          <button
            @click="toggleGroup('sites')"
            class="w-full flex items-center gap-2 px-4 py-2 text-sm text-gray-400 hover:text-white hover:bg-panel-dark transition border-t border-panel-border"
          >
            <component :is="expandedGroups.has('sites') ? ChevronDown : ChevronRight" class="w-4 h-4 flex-shrink-0" />
            <Globe class="w-4 h-4 flex-shrink-0" />
            <span>{{ t('log_viewer.groups.sites') }}</span>
          </button>

          <div v-if="expandedGroups.has('sites')" class="ml-6 border-l border-panel-border/50">
            <div v-for="site in catalog.sites" :key="site.domain" class="py-0.5">
              <button
                @click="toggleSite(site.domain)"
                class="w-full flex items-center gap-1.5 px-4 py-1 text-sm text-gray-400 hover:text-white transition"
              >
                <component :is="expandedSites.has(site.domain) ? ChevronDown : ChevronRight" class="w-3 h-3 flex-shrink-0" />
                <span class="truncate">{{ site.domain }}</span>
              </button>
              <div v-if="expandedSites.has(site.domain)" class="ml-4 border-l border-panel-border/30">
                <button
                  v-for="kind in site.logs"
                  :key="kind"
                  @click="selectSource({
                    id: `site_${site.domain}_${kind}`,
                    name: `${kind}.log`,
                    group: 'sites'
                  })"
                  :class="[
                    'w-full text-left pl-7 pr-3 py-1 text-xs transition truncate block',
                    selectedSource?.id === `site_${site.domain}_${kind}`
                      ? 'text-green-400 bg-green-500/10 border-r-2 border-green-400'
                      : 'text-gray-500 hover:text-gray-300'
                  ]"
                >
                  {{ kind }}.log
                </button>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- Right Panel: Log Output -->
    <div class="flex-1 flex flex-col min-w-0">
      <!-- Toolbar -->
      <div class="flex items-center gap-3 px-4 py-3 border-b border-panel-border bg-panel-dark/30">
        <h3 class="text-white font-medium truncate" v-if="selectedSource">
          {{ selectedSource.name }}
        </h3>
        <span class="text-gray-500 text-sm" v-else>{{ t('log_viewer.select_source') }}</span>

        <div class="flex-1"></div>

        <div class="flex items-center gap-2" v-if="selectedSource">
          <label class="text-xs text-gray-500">{{ t('log_viewer.lines') }}</label>
          <select v-model.number="lineCount" class="aura-input text-xs py-1 px-2 w-20" @change="fetchLogs">
            <option :value="50">50</option>
            <option :value="100">100</option>
            <option :value="200">200</option>
            <option :value="500">500</option>
          </select>

          <label class="flex items-center gap-1.5 text-xs text-gray-500 cursor-pointer ml-2">
            <input type="checkbox" v-model="autoRefresh" class="accent-green-500" />
            {{ t('log_viewer.auto_refresh') }}
          </label>

          <button class="btn-secondary text-xs py-1.5 px-3 flex items-center gap-1" @click="fetchLogs">
            <RefreshCw class="w-3.5 h-3.5" :class="{ 'animate-spin': loading }" />
            {{ t('log_viewer.refresh') }}
          </button>
        </div>
      </div>

      <!-- Log Content -->
      <div class="flex-1 overflow-auto bg-[#0a0a0f] p-4" ref="logContainer">
        <div v-if="loading" class="text-gray-500 text-sm">{{ t('common.loading') }}...</div>
        <div v-else-if="error" class="text-red-400 text-sm">{{ error }}</div>
        <pre v-else-if="logLines.length" class="text-xs font-mono text-gray-300 whitespace-pre-wrap break-all leading-relaxed">{{ logLines.join('\n') }}</pre>
        <div v-else class="text-gray-500 text-sm">{{ t('log_viewer.no_logs') }}</div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import api from '../services/api'
import { ChevronDown, ChevronRight, Cpu, Globe, HardDrive, Mail, RefreshCw, Server, Shield } from 'lucide-vue-next'

const { t } = useI18n({ useScope: 'global' })

const catalog = ref(null)
const selectedSource = ref(null)
const logLines = ref([])
const lineCount = ref(100)
const loading = ref(false)
const error = ref('')
const autoRefresh = ref(false)
const expandedGroups = ref(new Set(['system', 'webserver', 'panel']))
const expandedSites = ref(new Set())
const logContainer = ref(null)

const groupIcons = {
  system: CpuIcon,
  webserver: Globe,
  panel: ServerIcon,
  security: ShieldIcon,
  database: HardDriveIcon,
  mail: MailIcon,
}

const groupOrder = ['system', 'webserver', 'panel', 'security', 'database', 'mail']

const catalogGroups = computed(() => {
  if (!catalog.value?.categories) return []
  const grouped = {}
  for (const cat of catalog.value.categories) {
    if (!grouped[cat.group]) grouped[cat.group] = { key: cat.group, icon: groupIcons[cat.group], sources: [] }
    grouped[cat.group].sources.push(cat)
  }
  return groupOrder.filter(k => grouped[k]).map(k => grouped[k])
})

function toggleGroup(key) {
  const next = new Set(expandedGroups.value)
  if (next.has(key)) next.delete(key)
  else next.add(key)
  expandedGroups.value = next
}

function toggleSite(domain) {
  const next = new Set(expandedSites.value)
  if (next.has(domain)) next.delete(domain)
  else next.add(domain)
  expandedSites.value = next
}

function selectSource(source) {
  selectedSource.value = source
  fetchLogs()
}

async function fetchLogs() {
  if (!selectedSource.value) return
  loading.value = true
  error.value = ''
  try {
    const res = await api.get('/monitor/logs', { params: { source: selectedSource.value.id, lines: lineCount.value } })
    const data = res.data?.data
    logLines.value = Array.isArray(data?.data) ? data.data : (Array.isArray(data) ? data : [])
  } catch (err) {
    error.value = err?.response?.data?.message || err?.message || t('common.error')
    logLines.value = []
  } finally {
    loading.value = false
    await nextTick()
    if (logContainer.value) {
      logContainer.value.scrollTop = logContainer.value.scrollHeight
    }
  }
}

async function loadCatalog() {
  try {
    const res = await api.get('/monitor/logs/catalog')
    catalog.value = res.data?.data
  } catch {
    // Catalog yüklenemezse boş kalır
  }
}

let refreshTimer = null

watch(autoRefresh, (on) => {
  if (on) {
    refreshTimer = setInterval(fetchLogs, 10000)
  } else {
    clearInterval(refreshTimer)
    refreshTimer = null
  }
})

onMounted(() => {
  loadCatalog()
})

onUnmounted(() => {
  clearInterval(refreshTimer)
})
</script>
