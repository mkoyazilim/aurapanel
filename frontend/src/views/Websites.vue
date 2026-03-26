<template>
  <div class="space-y-6">
    <div class="flex items-center justify-between">
      <div>
        <h1 class="text-2xl font-bold text-white">{{ t('websites.title', 'Websites') }}</h1>
        <p class="text-gray-400 mt-1">Manage your domains, SSL, and PHP settings.</p>
      </div>
      <button class="btn-primary">
        <Plus class="w-5 h-5" />
        {{ t('websites.add_new', 'Add Website') }}
      </button>
    </div>

    <!-- Search & Filter -->
    <div class="flex flex-wrap gap-4 items-center bg-panel-card p-4 rounded-xl border border-panel-border">
      <div class="relative flex-1 min-w-[200px]">
        <Search class="w-5 h-5 text-gray-400 absolute left-3 top-1/2 -translate-y-1/2" />
        <input type="text" class="aura-input pl-10" placeholder="Search domains..." />
      </div>
      <select class="aura-input w-auto min-w-[150px]">
        <option>All PHP Versions</option>
        <option>PHP 8.3</option>
        <option>PHP 8.2</option>
      </select>
    </div>

    <!-- Website List -->
    <div class="space-y-4">
      <div v-for="site in sites" :key="site.domain" class="aura-card flex flex-col sm:flex-row gap-6 justify-between items-start sm:items-center">
        
        <div class="flex items-center gap-4">
          <div class="w-12 h-12 rounded-lg bg-panel-dark flex items-center justify-center border border-panel-border">
            <Globe class="w-6 h-6 text-brand-500" />
          </div>
          <div>
            <h3 class="text-lg font-bold text-white flex items-center gap-2">
              {{ site.domain }}
              <span v-if="site.ssl" class="px-2 py-0.5 rounded text-xs font-semibold bg-brand-500/10 text-brand-400 border border-brand-500/20">SSL Active</span>
            </h3>
            <div class="text-sm text-gray-400 mt-1 flex items-center gap-4">
              <span class="flex items-center gap-1"><HardDrive class="w-4 h-4" /> {{ site.diskUsage }} / {{ site.quota }}</span>
              <span class="flex items-center gap-1"><Cpu class="w-4 h-4" /> PHP {{ site.php }}</span>
            </div>
          </div>
        </div>

        <div class="flex items-center gap-2 w-full sm:w-auto">
          <button class="btn-secondary px-3 py-1.5 text-sm flex-1 sm:flex-none">Manage</button>
          <button class="btn-secondary px-3 py-1.5 text-sm">Cache</button>
          <button class="btn-danger px-2 py-1.5" title="Delete">
            <Trash2 class="w-4 h-4" />
          </button>
        </div>

      </div>
    </div>
  </div>
</template>

<script setup>
import { useI18n } from 'vue-i18n'
import { Plus, Search, Globe, HardDrive, Cpu, Trash2 } from 'lucide-vue-next'

const { t } = useI18n()

const sites = [
  { domain: 'aurapanel.com', ssl: true, diskUsage: '1.2 GB', quota: '5 GB', php: '8.3' },
  { domain: 'example.net', ssl: false, diskUsage: '350 MB', quota: '1 GB', php: '8.2' },
  { domain: 'myshop.store', ssl: true, diskUsage: '4.5 GB', quota: '10 GB', php: '8.3' },
]
</script>
