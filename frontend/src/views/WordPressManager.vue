<template>
  <div class="space-y-6">
    <div class="flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
      <div>
        <h1 class="text-2xl font-bold text-white">{{ t('wordpress_manager.title') }}</h1>
        <p class="mt-1 text-gray-400">{{ t('wordpress_manager.subtitle') }}</p>
      </div>
      <div class="flex flex-wrap items-center gap-3">
        <router-link :to="{ path: '/app-runtime', query: { tab: 'cms' } }" class="btn-secondary">{{ t('wordpress_manager.cms_installation') }}</router-link>
        <button class="btn-secondary flex items-center gap-2" :disabled="loadingSites" @click="loadSites">
          <RefreshCw class="h-4 w-4" :class="{ 'animate-spin': loadingSites }" />
          {{ t('wordpress_manager.refresh') }}
        </button>
        <button class="btn-primary flex items-center gap-2" :disabled="scanningSites" @click="scanSites">
          <Search class="h-4 w-4" :class="{ 'animate-spin': scanningSites }" />
          {{ t('wordpress_manager.scan') }}
        </button>
      </div>
    </div>

    <div v-if="notice.message" :class="['rounded-xl border px-4 py-3 text-sm font-medium', notice.type === 'success' ? 'border-green-500/30 bg-green-500/10 text-green-300' : 'border-red-500/30 bg-red-500/10 text-red-300']">
      {{ notice.message }}
    </div>

    <div class="grid gap-6 xl:grid-cols-[320px_minmax(0,1fr)]">
      <section class="rounded-2xl border border-panel-border bg-panel-card">
        <div class="border-b border-panel-border px-5 py-4">
          <div class="flex items-center justify-between">
            <div>
              <h2 class="text-lg font-semibold text-white">{{ t('wordpress_manager.sites_title') }}</h2>
              <p class="text-xs text-gray-400">{{ t('wordpress_manager.records_found', { count: sites.length }) }}</p>
            </div>
            <div class="rounded-full bg-brand-500/10 px-3 py-1 text-xs font-semibold text-brand-300">{{ t('wordpress_manager.toolkit') }}</div>
          </div>
        </div>

        <div v-if="loadingSites" class="flex items-center justify-center px-5 py-10 text-gray-400">
          <Loader2 class="mr-2 h-5 w-5 animate-spin" />
          {{ t('wordpress_manager.loading_sites') }}
        </div>

        <div v-else-if="sites.length === 0" class="space-y-4 px-5 py-10 text-center">
          <div class="mx-auto flex h-16 w-16 items-center justify-center rounded-2xl bg-panel-dark text-brand-300">
            <Globe class="h-8 w-8" />
          </div>
          <div>
            <p class="font-semibold text-white">{{ t('wordpress_manager.empty_title') }}</p>
            <p class="mt-2 text-sm text-gray-400">{{ t('wordpress_manager.empty_body') }}</p>
          </div>
          <div class="flex justify-center gap-3">
            <router-link :to="{ path: '/app-runtime', query: { tab: 'cms' } }" class="btn-secondary">{{ t('wordpress_manager.go_install') }}</router-link>
            <button class="btn-primary" @click="scanSites">{{ t('wordpress_manager.rescan') }}</button>
          </div>
        </div>

        <div v-else class="max-h-[72vh] space-y-2 overflow-y-auto p-3">
          <button
            v-for="site in sites"
            :key="site.domain"
            class="w-full rounded-xl border px-4 py-3 text-left transition"
            :class="selectedDomain === site.domain ? 'border-brand-500/40 bg-brand-500/10 shadow-[0_0_0_1px_rgba(16,185,129,0.2)]' : 'border-panel-border bg-panel-dark/70 hover:border-panel-border/80 hover:bg-panel-dark'"
            @click="selectedDomain = site.domain"
          >
            <div class="flex items-start justify-between gap-3">
              <div class="min-w-0">
                <p class="truncate font-semibold text-white">{{ site.domain }}</p>
                <p class="mt-1 truncate text-xs text-gray-400">{{ site.title || site.site_url || site.docroot }}</p>
              </div>
              <span class="shrink-0 rounded-full px-2 py-1 text-[11px] font-semibold" :class="site.status === 'active' ? 'bg-green-500/15 text-green-300' : 'bg-yellow-500/15 text-yellow-300'">
                {{ site.status }}
              </span>
            </div>
            <div class="mt-3 flex flex-wrap gap-2 text-[11px] text-gray-400">
              <span class="rounded-full bg-panel-card px-2 py-1">WP {{ site.wordpress_version }}</span>
              <span class="rounded-full bg-panel-card px-2 py-1">PHP {{ site.php_version }}</span>
              <span class="rounded-full bg-panel-card px-2 py-1">{{ site.owner }}</span>
            </div>
          </button>
        </div>
      </section>

      <section class="min-w-0 space-y-6">
        <div v-if="!selectedSite" class="rounded-2xl border border-dashed border-panel-border bg-panel-card px-8 py-16 text-center">
          <p class="text-lg font-semibold text-white">{{ t('wordpress_manager.select_site') }}</p>
          <p class="mt-2 text-sm text-gray-400">{{ t('wordpress_manager.select_site_body') }}</p>
        </div>

        <template v-else>
          <div class="rounded-2xl border border-panel-border bg-panel-card p-6">
            <div class="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
              <div>
                <div class="flex flex-wrap items-center gap-3">
                  <h2 class="text-2xl font-bold text-white">{{ selectedSite.domain }}</h2>
                  <span class="rounded-full bg-brand-500/10 px-3 py-1 text-xs font-semibold text-brand-300">{{ selectedSite.wordpress_version }}</span>
                  <span class="rounded-full bg-panel-dark px-3 py-1 text-xs font-semibold text-gray-300">{{ selectedSite.php_version }}</span>
                </div>
                <p class="mt-2 text-sm text-gray-400">{{ selectedSite.title || selectedSite.site_url || selectedSite.docroot }}</p>
              </div>
              <div class="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
                <div class="rounded-xl border border-panel-border bg-panel-dark/80 px-4 py-3">
                  <p class="text-xs uppercase tracking-[0.2em] text-gray-500">{{ t('wordpress_manager.stats.owner') }}</p>
                  <p class="mt-2 font-semibold text-white">{{ selectedSite.owner }}</p>
                </div>
                <div class="rounded-xl border border-panel-border bg-panel-dark/80 px-4 py-3">
                  <p class="text-xs uppercase tracking-[0.2em] text-gray-500">{{ t('wordpress_manager.stats.plugins') }}</p>
                  <p class="mt-2 font-semibold text-white">{{ selectedSite.active_plugins }} / {{ selectedSite.total_plugins }}</p>
                </div>
                <div class="rounded-xl border border-panel-border bg-panel-dark/80 px-4 py-3">
                  <p class="text-xs uppercase tracking-[0.2em] text-gray-500">{{ t('wordpress_manager.stats.theme') }}</p>
                  <p class="mt-2 truncate font-semibold text-white">{{ selectedSite.active_theme || '-' }}</p>
                </div>
                <div class="rounded-xl border border-panel-border bg-panel-dark/80 px-4 py-3">
                  <p class="text-xs uppercase tracking-[0.2em] text-gray-500">{{ t('wordpress_manager.stats.database') }}</p>
                  <p class="mt-2 font-semibold text-white">{{ selectedSite.db_engine }}</p>
                </div>
              </div>
            </div>

            <div class="mt-6 grid gap-4 md:grid-cols-2 xl:grid-cols-3">
              <div class="rounded-xl border border-panel-border bg-panel-dark/70 p-4">
                <p class="text-xs uppercase tracking-[0.2em] text-gray-500">{{ t('wordpress_manager.stats.site_url') }}</p>
                <a v-if="selectedSite.site_url" :href="selectedSite.site_url" class="mt-2 block break-all text-sm text-brand-300 hover:text-white" target="_blank" rel="noreferrer">
                  {{ selectedSite.site_url }}
                </a>
                <p v-else class="mt-2 text-sm text-gray-400">{{ t('wordpress_manager.stats.not_found') }}</p>
              </div>
              <div class="rounded-xl border border-panel-border bg-panel-dark/70 p-4">
                <p class="text-xs uppercase tracking-[0.2em] text-gray-500">{{ t('wordpress_manager.stats.admin_email') }}</p>
                <p class="mt-2 break-all text-sm text-white">{{ selectedSite.admin_email || '-' }}</p>
              </div>
              <div class="rounded-xl border border-panel-border bg-panel-dark/70 p-4">
                <p class="text-xs uppercase tracking-[0.2em] text-gray-500">{{ t('wordpress_manager.stats.docroot') }}</p>
                <p class="mt-2 break-all font-mono text-xs text-gray-300">{{ selectedSite.docroot }}</p>
              </div>
            </div>
          </div>

          <div class="border-b border-panel-border">
            <nav class="flex flex-wrap gap-6">
              <button v-for="tabItem in tabs" :key="tabItem.value" class="border-b-2 pb-3 text-sm font-medium transition" :class="activeTab === tabItem.value ? 'border-brand-500 text-brand-300' : 'border-transparent text-gray-400 hover:text-white'" @click="activeTab = tabItem.value">
                {{ tabItem.label }}
              </button>
            </nav>
          </div>

          <div v-if="activeTab === 'overview'" class="grid gap-4 lg:grid-cols-2">
            <div class="rounded-2xl border border-panel-border bg-panel-card p-5">
              <h3 class="text-lg font-semibold text-white">{{ t('wordpress_manager.overview.summary') }}</h3>
              <dl class="mt-4 space-y-3 text-sm">
                <div class="flex items-start justify-between gap-4">
                  <dt class="text-gray-400">{{ t('wordpress_manager.stats.db_name') }}</dt>
                  <dd class="text-right text-white">{{ selectedSite.db_name || '-' }}</dd>
                </div>
                <div class="flex items-start justify-between gap-4">
                  <dt class="text-gray-400">{{ t('wordpress_manager.stats.db_user') }}</dt>
                  <dd class="text-right text-white">{{ selectedSite.db_user || '-' }}</dd>
                </div>
                <div class="flex items-start justify-between gap-4">
                  <dt class="text-gray-400">{{ t('wordpress_manager.stats.db_host') }}</dt>
                  <dd class="text-right text-white">{{ selectedSite.db_host || '-' }}</dd>
                </div>
                <div class="flex items-start justify-between gap-4">
                  <dt class="text-gray-400">{{ t('wordpress_manager.stats.status') }}</dt>
                  <dd class="text-right text-white">{{ selectedSite.status }}</dd>
                </div>
              </dl>
            </div>

            <div class="rounded-2xl border border-panel-border bg-panel-card p-5">
              <h3 class="text-lg font-semibold text-white">{{ t('wordpress_manager.overview.next_steps') }}</h3>
              <div class="mt-4 space-y-3 text-sm text-gray-300">
                <p>{{ t('wordpress_manager.overview.step_plugins') }}</p>
                <p>{{ t('wordpress_manager.overview.step_backups') }}</p>
                <p>{{ t('wordpress_manager.overview.step_install') }}</p>
              </div>
            </div>
          </div>
          <div v-else-if="activeTab === 'plugins'" class="rounded-2xl border border-panel-border bg-panel-card">
            <div class="flex flex-col gap-3 border-b border-panel-border px-5 py-4 lg:flex-row lg:items-center lg:justify-between">
              <div>
                <h3 class="text-lg font-semibold text-white">{{ t('wordpress_manager.plugins.title') }}</h3>
                <p class="text-sm text-gray-400">{{ t('wordpress_manager.plugins.count', { count: plugins.length }) }}</p>
              </div>
              <div class="flex flex-wrap gap-2">
                <button class="btn-secondary" :disabled="pluginsLoading" @click="loadPlugins">{{ t('wordpress_manager.plugins.refresh') }}</button>
                <button class="btn-secondary" :disabled="pluginActionLoading" @click="updatePlugins(true)">{{ t('wordpress_manager.plugins.update_all') }}</button>
                <button class="btn-primary" :disabled="pluginActionLoading || selectedPlugins.length === 0" @click="updatePlugins(false)">{{ t('wordpress_manager.plugins.update_selected') }}</button>
                <button class="btn-danger" :disabled="pluginActionLoading || selectedPlugins.length === 0" @click="deletePlugins">{{ t('wordpress_manager.plugins.delete_selected') }}</button>
              </div>
            </div>
            <div v-if="pluginsLoading" class="px-5 py-10 text-center text-gray-400">
              <Loader2 class="mx-auto mb-2 h-6 w-6 animate-spin" />
              {{ t('wordpress_manager.plugins.loading') }}
            </div>
            <div v-else class="overflow-x-auto">
              <table class="w-full text-sm">
                <thead>
                  <tr class="border-b border-panel-border text-gray-400">
                    <th class="px-4 py-3 text-left font-medium"><input type="checkbox" class="rounded border-panel-border bg-panel-dark" :checked="allPluginsSelected" @change="toggleAllPlugins" /></th>
                    <th class="px-4 py-3 text-left font-medium">{{ t('wordpress_manager.plugins.name') }}</th>
                    <th class="px-4 py-3 text-left font-medium">{{ t('wordpress_manager.plugins.version') }}</th>
                    <th class="px-4 py-3 text-left font-medium">{{ t('wordpress_manager.plugins.status') }}</th>
                    <th class="px-4 py-3 text-left font-medium">{{ t('wordpress_manager.plugins.update') }}</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="plugin in plugins" :key="plugin.name" class="border-b border-panel-border/60">
                    <td class="px-4 py-3"><input v-model="selectedPlugins" type="checkbox" class="rounded border-panel-border bg-panel-dark" :value="plugin.name" /></td>
                    <td class="px-4 py-3"><p class="font-medium text-white">{{ plugin.title || plugin.name }}</p><p class="text-xs text-gray-500">{{ plugin.name }}</p></td>
                    <td class="px-4 py-3 text-gray-300">{{ plugin.version || '-' }}</td>
                    <td class="px-4 py-3"><span class="rounded-full px-2 py-1 text-xs font-semibold" :class="plugin.status === 'active' ? 'bg-green-500/15 text-green-300' : 'bg-gray-500/15 text-gray-300'">{{ plugin.status }}</span></td>
                    <td class="px-4 py-3 text-gray-300">{{ plugin.update || '-' }}</td>
                  </tr>
                  <tr v-if="plugins.length === 0"><td colspan="5" class="px-4 py-10 text-center text-gray-500">{{ t('wordpress_manager.plugins.empty') }}</td></tr>
                </tbody>
              </table>
            </div>
          </div>

          <div v-else-if="activeTab === 'themes'" class="rounded-2xl border border-panel-border bg-panel-card">
            <div class="flex flex-col gap-3 border-b border-panel-border px-5 py-4 lg:flex-row lg:items-center lg:justify-between">
              <div>
                <h3 class="text-lg font-semibold text-white">{{ t('wordpress_manager.themes.title') }}</h3>
                <p class="text-sm text-gray-400">{{ t('wordpress_manager.themes.count', { count: themes.length }) }}</p>
              </div>
              <div class="flex flex-wrap gap-2">
                <button class="btn-secondary" :disabled="themesLoading" @click="loadThemes">{{ t('wordpress_manager.themes.refresh') }}</button>
                <button class="btn-secondary" :disabled="themeActionLoading" @click="updateThemes(true)">{{ t('wordpress_manager.themes.update_all') }}</button>
                <button class="btn-primary" :disabled="themeActionLoading || selectedThemes.length === 0" @click="updateThemes(false)">{{ t('wordpress_manager.themes.update_selected') }}</button>
                <button class="btn-danger" :disabled="themeActionLoading || selectedThemes.length === 0" @click="deleteThemes">{{ t('wordpress_manager.themes.delete_selected') }}</button>
              </div>
            </div>
            <div v-if="themesLoading" class="px-5 py-10 text-center text-gray-400">
              <Loader2 class="mx-auto mb-2 h-6 w-6 animate-spin" />
              {{ t('wordpress_manager.themes.loading') }}
            </div>
            <div v-else class="overflow-x-auto">
              <table class="w-full text-sm">
                <thead>
                  <tr class="border-b border-panel-border text-gray-400">
                    <th class="px-4 py-3 text-left font-medium"><input type="checkbox" class="rounded border-panel-border bg-panel-dark" :checked="allThemesSelected" @change="toggleAllThemes" /></th>
                    <th class="px-4 py-3 text-left font-medium">{{ t('wordpress_manager.themes.name') }}</th>
                    <th class="px-4 py-3 text-left font-medium">{{ t('wordpress_manager.themes.version') }}</th>
                    <th class="px-4 py-3 text-left font-medium">{{ t('wordpress_manager.themes.status') }}</th>
                    <th class="px-4 py-3 text-left font-medium">{{ t('wordpress_manager.themes.update') }}</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="theme in themes" :key="theme.name" class="border-b border-panel-border/60">
                    <td class="px-4 py-3"><input v-model="selectedThemes" type="checkbox" class="rounded border-panel-border bg-panel-dark" :value="theme.name" /></td>
                    <td class="px-4 py-3"><p class="font-medium text-white">{{ theme.title || theme.name }}</p><p class="text-xs text-gray-500">{{ theme.name }}</p></td>
                    <td class="px-4 py-3 text-gray-300">{{ theme.version || '-' }}</td>
                    <td class="px-4 py-3"><span class="rounded-full px-2 py-1 text-xs font-semibold" :class="theme.status === 'active' ? 'bg-green-500/15 text-green-300' : 'bg-gray-500/15 text-gray-300'">{{ theme.status }}</span></td>
                    <td class="px-4 py-3 text-gray-300">{{ theme.update || '-' }}</td>
                  </tr>
                  <tr v-if="themes.length === 0"><td colspan="5" class="px-4 py-10 text-center text-gray-500">{{ t('wordpress_manager.themes.empty') }}</td></tr>
                </tbody>
              </table>
            </div>
          </div>

          <div v-else-if="activeTab === 'backups'" class="space-y-4">
            <div class="rounded-2xl border border-panel-border bg-panel-card p-5">
              <div class="flex flex-col gap-3 lg:flex-row lg:items-end lg:justify-between">
                <div>
                  <h3 class="text-lg font-semibold text-white">{{ t('wordpress_manager.backups.title') }}</h3>
                  <p class="text-sm text-gray-400">{{ t('wordpress_manager.backups.subtitle') }}</p>
                </div>
                <div class="flex flex-wrap items-center gap-3">
                  <div class="rounded-full border border-panel-border bg-panel-dark px-3 py-2 text-xs font-semibold text-gray-300">
                    {{ t('wordpress_manager.backups.full') }}
                  </div>
                  <button class="btn-secondary" :disabled="backupsLoading" @click="loadBackups">{{ t('wordpress_manager.backups.refresh') }}</button>
                  <button class="btn-primary" :disabled="backupActionLoading" @click="createBackup">{{ t('wordpress_manager.backups.create') }}</button>
                </div>
              </div>
            </div>

            <div class="overflow-x-auto rounded-2xl border border-panel-border bg-panel-card">
              <table class="w-full text-sm">
                <thead>
                  <tr class="border-b border-panel-border text-gray-400">
                    <th class="px-4 py-3 text-left font-medium">{{ t('wordpress_manager.backups.file') }}</th>
                    <th class="px-4 py-3 text-left font-medium">{{ t('wordpress_manager.backups.type') }}</th>
                    <th class="px-4 py-3 text-left font-medium">{{ t('wordpress_manager.backups.size') }}</th>
                    <th class="px-4 py-3 text-left font-medium">{{ t('wordpress_manager.backups.date') }}</th>
                    <th class="px-4 py-3 text-right font-medium">{{ t('wordpress_manager.backups.action') }}</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="backup in backups" :key="backup.id" class="border-b border-panel-border/60">
                    <td class="px-4 py-3"><p class="font-medium text-white">{{ backup.file_name }}</p><p class="text-xs text-gray-500">{{ backup.id }}</p></td>
                    <td class="px-4 py-3 text-gray-300">{{ backup.backup_type }}</td>
                    <td class="px-4 py-3 text-gray-300">{{ formatBytes(backup.size_bytes) }}</td>
                    <td class="px-4 py-3 text-gray-300">{{ formatDate(backup.created_at) }}</td>
                    <td class="px-4 py-3 text-right"><div class="flex justify-end gap-2"><button class="btn-secondary px-3 py-2 text-xs" @click="downloadBackup(backup)">{{ t('wordpress_manager.backups.download') }}</button><button class="btn-primary px-3 py-2 text-xs" :disabled="backupActionLoading" @click="restoreBackup(backup)">{{ t('wordpress_manager.backups.restore') }}</button></div></td>
                  </tr>
                  <tr v-if="backups.length === 0"><td colspan="5" class="px-4 py-10 text-center text-gray-500">{{ t('wordpress_manager.backups.empty') }}</td></tr>
                </tbody>
              </table>
            </div>
          </div>

          <div v-else-if="activeTab === 'staging'" class="space-y-4">
            <div class="rounded-2xl border border-panel-border bg-panel-card p-5">
              <div class="flex flex-col gap-4 xl:flex-row xl:items-end xl:justify-between">
                <div>
                  <h3 class="text-lg font-semibold text-white">{{ t('wordpress_manager.staging.title') }}</h3>
                  <p class="text-sm text-gray-400">{{ t('wordpress_manager.staging.subtitle') }}</p>
                </div>
                <div class="flex flex-col gap-3 sm:flex-row sm:items-center">
                  <input v-model="stagingDomain" class="aura-input min-w-[260px]" :placeholder="t('wordpress_manager.staging.placeholder')" />
                  <button class="btn-secondary" :disabled="stagingLoading" @click="loadStaging">{{ t('wordpress_manager.staging.refresh') }}</button>
                  <button class="btn-primary" :disabled="stagingActionLoading" @click="createStaging">{{ t('wordpress_manager.staging.create') }}</button>
                </div>
              </div>
            </div>

            <div class="overflow-x-auto rounded-2xl border border-panel-border bg-panel-card">
              <table class="w-full text-sm">
                <thead>
                  <tr class="border-b border-panel-border text-gray-400">
                    <th class="px-4 py-3 text-left font-medium">{{ t('wordpress_manager.staging.source') }}</th>
                    <th class="px-4 py-3 text-left font-medium">{{ t('wordpress_manager.staging.target') }}</th>
                    <th class="px-4 py-3 text-left font-medium">{{ t('wordpress_manager.staging.owner') }}</th>
                    <th class="px-4 py-3 text-left font-medium">{{ t('wordpress_manager.staging.date') }}</th>
                    <th class="px-4 py-3 text-left font-medium">{{ t('wordpress_manager.staging.status') }}</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="item in stagingEntries" :key="item.id" class="border-b border-panel-border/60">
                    <td class="px-4 py-3 text-white">{{ item.source_domain }}</td>
                    <td class="px-4 py-3 text-brand-300">{{ item.staging_domain }}</td>
                    <td class="px-4 py-3 text-gray-300">{{ item.owner }}</td>
                    <td class="px-4 py-3 text-gray-300">{{ formatDate(item.created_at) }}</td>
                    <td class="px-4 py-3"><span class="rounded-full bg-green-500/15 px-2 py-1 text-xs font-semibold text-green-300">{{ item.status }}</span></td>
                  </tr>
                  <tr v-if="stagingEntries.length === 0"><td colspan="5" class="px-4 py-10 text-center text-gray-500">{{ t('wordpress_manager.staging.empty') }}</td></tr>
                </tbody>
              </table>
            </div>
          </div>
        </template>
      </section>
    </div>
  </div>
</template>

<script setup>
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { Globe, Loader2, RefreshCw, Search } from 'lucide-vue-next'
import api from '../services/api'

const { t } = useI18n({ useScope: 'global' })

const tabs = [
  { value: 'overview', label: t('wordpress_manager.tabs.overview') },
  { value: 'plugins', label: t('wordpress_manager.tabs.plugins') },
  { value: 'themes', label: t('wordpress_manager.tabs.themes') },
  { value: 'backups', label: t('wordpress_manager.tabs.backups') },
  { value: 'staging', label: t('wordpress_manager.tabs.staging') },
]

const sites = ref([])
const selectedDomain = ref('')
const activeTab = ref('overview')
const loadingSites = ref(false)
const scanningSites = ref(false)
const plugins = ref([])
const themes = ref([])
const backups = ref([])
const stagingEntries = ref([])
const selectedPlugins = ref([])
const selectedThemes = ref([])
const pluginsLoading = ref(false)
const themesLoading = ref(false)
const backupsLoading = ref(false)
const stagingLoading = ref(false)
const pluginActionLoading = ref(false)
const themeActionLoading = ref(false)
const backupActionLoading = ref(false)
const stagingActionLoading = ref(false)
const stagingDomain = ref('')
const notice = ref({ message: '', type: 'success' })

const selectedSite = computed(() => sites.value.find(site => site.domain === selectedDomain.value) || null)
const allPluginsSelected = computed(() => plugins.value.length > 0 && selectedPlugins.value.length === plugins.value.length)
const allThemesSelected = computed(() => themes.value.length > 0 && selectedThemes.value.length === themes.value.length)

function setNotice(message, type = 'success') {
  notice.value = { message, type }
  setTimeout(() => {
    if (notice.value.message === message) notice.value = { message: '', type: 'success' }
  }, 3500)
}

function formatDate(timestamp) {
  if (!timestamp) return '-'
  return new Date(timestamp * 1000).toLocaleString()
}

function formatBytes(bytes) {
  const size = Number(bytes || 0)
  if (!Number.isFinite(size) || size <= 0) return '-'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let value = size
  let unitIndex = 0
  while (value >= 1024 && unitIndex < units.length - 1) {
    value /= 1024
    unitIndex += 1
  }
  return `${value.toFixed(value >= 10 || unitIndex === 0 ? 0 : 1)} ${units[unitIndex]}`
}

function buildDefaultStagingDomain(domain) {
  return domain ? `staging.${domain}` : ''
}

async function loadSites() {
  loadingSites.value = true
  try {
    const res = await api.get('/wordpress/sites')
    applySites(res.data?.data || [])
  } catch (error) {
    sites.value = []
    selectedDomain.value = ''
    setNotice(error?.response?.data?.message || t('wordpress_manager.notices.sites_failed'), 'error')
  } finally {
    loadingSites.value = false
  }
}

async function scanSites() {
  scanningSites.value = true
  try {
    const res = await api.post('/wordpress/scan')
    applySites(res.data?.data || [])
    setNotice(t('wordpress_manager.notices.scan_success'))
  } catch (error) {
    setNotice(error?.response?.data?.message || t('wordpress_manager.notices.scan_failed'), 'error')
  } finally {
    scanningSites.value = false
  }
}

function applySites(items) {
  const previousDomain = selectedDomain.value
  sites.value = Array.isArray(items) ? items : []
  if (!sites.value.length) {
    selectedDomain.value = ''
    return
  }
  if (previousDomain && sites.value.some(site => site.domain === previousDomain)) {
    selectedDomain.value = previousDomain
    loadSelectedSiteData()
    return
  }
  selectedDomain.value = sites.value[0].domain
}

async function loadSelectedSiteData() {
  if (!selectedDomain.value) {
    plugins.value = []
    themes.value = []
    backups.value = []
    stagingEntries.value = []
    return
  }
  await Promise.allSettled([loadPlugins(), loadThemes(), loadBackups(), loadStaging()])
}

async function loadPlugins() {
  if (!selectedDomain.value) return
  pluginsLoading.value = true
  try {
    const res = await api.get('/wordpress/plugins', { params: { domain: selectedDomain.value } })
    plugins.value = res.data?.data || []
    selectedPlugins.value = selectedPlugins.value.filter(name => plugins.value.some(plugin => plugin.name === name))
  } finally {
    pluginsLoading.value = false
  }
}

async function loadThemes() {
  if (!selectedDomain.value) return
  themesLoading.value = true
  try {
    const res = await api.get('/wordpress/themes', { params: { domain: selectedDomain.value } })
    themes.value = res.data?.data || []
    selectedThemes.value = selectedThemes.value.filter(name => themes.value.some(theme => theme.name === name))
  } finally {
    themesLoading.value = false
  }
}

async function loadBackups() {
  if (!selectedDomain.value) return
  backupsLoading.value = true
  try {
    const res = await api.get('/wordpress/backups', { params: { domain: selectedDomain.value } })
    backups.value = res.data?.data || []
  } finally {
    backupsLoading.value = false
  }
}

async function loadStaging() {
  if (!selectedDomain.value) return
  stagingLoading.value = true
  try {
    const res = await api.get('/wordpress/staging', { params: { domain: selectedDomain.value } })
    stagingEntries.value = res.data?.data || []
  } finally {
    stagingLoading.value = false
  }
}

function toggleAllPlugins(event) {
  selectedPlugins.value = event.target.checked ? plugins.value.map(plugin => plugin.name) : []
}

function toggleAllThemes(event) {
  selectedThemes.value = event.target.checked ? themes.value.map(theme => theme.name) : []
}

async function updatePlugins(updateAll) {
  pluginActionLoading.value = true
  try {
    await api.post('/wordpress/plugins/update', { domain: selectedDomain.value, names: updateAll ? [] : selectedPlugins.value, all: updateAll })
    setNotice(t('wordpress_manager.notices.plugins_updated'))
    await Promise.all([loadPlugins(), loadSites()])
  } catch (error) {
    setNotice(error?.response?.data?.message || t('wordpress_manager.notices.plugins_update_failed'), 'error')
  } finally {
    pluginActionLoading.value = false
  }
}

async function deletePlugins() {
  if (!selectedPlugins.value.length || !window.confirm(t('wordpress_manager.plugins.delete_confirm'))) return
  pluginActionLoading.value = true
  try {
    await api.delete('/wordpress/plugins', { data: { domain: selectedDomain.value, names: selectedPlugins.value, all: false } })
    selectedPlugins.value = []
    setNotice(t('wordpress_manager.notices.plugins_deleted'))
    await Promise.all([loadPlugins(), loadSites()])
  } catch (error) {
    setNotice(error?.response?.data?.message || t('wordpress_manager.notices.plugins_delete_failed'), 'error')
  } finally {
    pluginActionLoading.value = false
  }
}

async function updateThemes(updateAll) {
  themeActionLoading.value = true
  try {
    await api.post('/wordpress/themes/update', { domain: selectedDomain.value, names: updateAll ? [] : selectedThemes.value, all: updateAll })
    setNotice(t('wordpress_manager.notices.themes_updated'))
    await Promise.all([loadThemes(), loadSites()])
  } catch (error) {
    setNotice(error?.response?.data?.message || t('wordpress_manager.notices.themes_update_failed'), 'error')
  } finally {
    themeActionLoading.value = false
  }
}

async function deleteThemes() {
  if (!selectedThemes.value.length || !window.confirm(t('wordpress_manager.themes.delete_confirm'))) return
  themeActionLoading.value = true
  try {
    await api.delete('/wordpress/themes', { data: { domain: selectedDomain.value, names: selectedThemes.value, all: false } })
    selectedThemes.value = []
    setNotice(t('wordpress_manager.notices.themes_deleted'))
    await Promise.all([loadThemes(), loadSites()])
  } catch (error) {
    setNotice(error?.response?.data?.message || t('wordpress_manager.notices.themes_delete_failed'), 'error')
  } finally {
    themeActionLoading.value = false
  }
}

async function createBackup() {
  backupActionLoading.value = true
  try {
    await api.post('/wordpress/backups', { domain: selectedDomain.value, backup_type: 'full' })
    setNotice(t('wordpress_manager.notices.backup_created'))
    await loadBackups()
  } catch (error) {
    setNotice(error?.response?.data?.message || t('wordpress_manager.notices.backup_failed'), 'error')
  } finally {
    backupActionLoading.value = false
  }
}

async function restoreBackup(backup) {
  if (!window.confirm(t('wordpress_manager.backups.restore_confirm', { name: backup.file_name }))) return
  backupActionLoading.value = true
  try {
    await api.post('/wordpress/backups/restore', { id: backup.id })
    setNotice(t('wordpress_manager.notices.backup_restore_success'))
  } catch (error) {
    setNotice(error?.response?.data?.message || t('wordpress_manager.notices.backup_restore_failed'), 'error')
  } finally {
    backupActionLoading.value = false
  }
}

async function downloadBackup(backup) {
  try {
    const res = await api.get('/wordpress/backups/download', { params: { id: backup.id }, responseType: 'blob' })
    const blob = new Blob([res.data], { type: 'application/octet-stream' })
    const url = window.URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url
    link.download = backup.file_name || `${backup.id}.tar.gz`
    document.body.appendChild(link)
    link.click()
    link.remove()
    window.URL.revokeObjectURL(url)
  } catch (error) {
    setNotice(error?.response?.data?.message || t('wordpress_manager.notices.backup_download_failed'), 'error')
  }
}

async function createStaging() {
  if (!stagingDomain.value.trim()) {
    setNotice(t('wordpress_manager.staging.required'), 'error')
    return
  }
  stagingActionLoading.value = true
  try {
    await api.post('/wordpress/staging', { source_domain: selectedDomain.value, staging_domain: stagingDomain.value.trim() })
    setNotice(t('wordpress_manager.notices.staging_created'))
    await Promise.all([loadStaging(), loadSites()])
  } catch (error) {
    setNotice(error?.response?.data?.message || t('wordpress_manager.notices.staging_failed'), 'error')
  } finally {
    stagingActionLoading.value = false
  }
}

watch(selectedDomain, domain => {
  selectedPlugins.value = []
  selectedThemes.value = []
  stagingDomain.value = buildDefaultStagingDomain(domain)
  if (domain) loadSelectedSiteData()
})

onMounted(loadSites)
</script>
