<template>
  <div class="space-y-6">
    <div class="flex items-center justify-between gap-4">
      <div>
        <h1 class="text-2xl font-bold text-white flex items-center gap-3">
          <Database class="w-7 h-7 text-orange-400" />
          {{ t('database_manager.title') }}
        </h1>
        <p class="text-gray-400 mt-1">{{ t('database_manager.subtitle') }}</p>
      </div>
      <div class="flex items-center gap-2">
        <button
          class="px-4 py-2.5 bg-panel-hover text-gray-200 rounded-lg text-sm font-medium hover:bg-gray-600 transition disabled:opacity-50 disabled:cursor-not-allowed"
          :disabled="launchingTool === 'phpmyadmin'"
          @click="launchDatabaseTool('phpmyadmin')"
        >
          {{ launchingTool === 'phpmyadmin' ? t('common.loading') : t('database_manager.actions.open_phpmyadmin') }}
        </button>
        <button
          class="px-4 py-2.5 bg-panel-hover text-gray-200 rounded-lg text-sm font-medium hover:bg-gray-600 transition disabled:opacity-50 disabled:cursor-not-allowed"
          :disabled="launchingTool === 'pgadmin'"
          @click="launchDatabaseTool('pgadmin')"
        >
          {{ launchingTool === 'pgadmin' ? t('common.loading') : t('database_manager.actions.open_pgadmin') }}
        </button>
        <button
          @click="showCreateModal = true"
          class="px-5 py-2.5 bg-gradient-to-r from-orange-600 to-amber-600 text-white rounded-lg font-medium hover:from-orange-700 hover:to-amber-700 transition"
        >
          <span class="flex items-center gap-2">
            <Plus class="w-5 h-5" />
            {{ t('database_manager.create_button') }}
          </span>
        </button>
      </div>
    </div>

    <div class="border-b border-panel-border">
      <nav class="flex gap-6">
        <button
          @click="engine = 'mariadb'"
          :class="['pb-3 text-sm font-medium transition', engine === 'mariadb' ? 'text-orange-400 border-b-2 border-orange-400' : 'text-gray-400 hover:text-white']"
        >
          {{ t('database_manager.engines.mariadb_mysql') }}
        </button>
        <button
          @click="engine = 'postgresql'"
          :class="['pb-3 text-sm font-medium transition', engine === 'postgresql' ? 'text-blue-400 border-b-2 border-blue-400' : 'text-gray-400 hover:text-white']"
        >
          {{ t('database_manager.engines.postgresql') }}
        </button>
        <button
          @click="engine = 'tuning'"
          :class="['pb-3 text-sm font-medium transition', engine === 'tuning' ? 'text-emerald-400 border-b-2 border-emerald-400' : 'text-gray-400 hover:text-white']"
        >
          <span class="flex items-center gap-2"><Settings2 class="w-4 h-4" /> Tuning & Config</span>
        </button>
      </nav>
    </div>

    <!-- Tuning Tab Content -->
    <div v-if="engine === 'tuning'" class="space-y-6">
      <!-- Engine Selector for Tuning -->
      <div class="flex gap-4 mb-4">
        <button 
          @click="tuningEngine = 'mariadb'; loadTuning()" 
          :class="['px-4 py-2 rounded-lg text-sm font-medium transition', tuningEngine === 'mariadb' ? 'bg-blue-600 text-white' : 'bg-panel-card text-gray-400 border border-panel-border hover:text-white']">
          MariaDB Tuning
        </button>
        <button 
          @click="tuningEngine = 'postgresql'; loadTuning()" 
          :class="['px-4 py-2 rounded-lg text-sm font-medium transition', tuningEngine === 'postgresql' ? 'bg-blue-600 text-white' : 'bg-panel-card text-gray-400 border border-panel-border hover:text-white']">
          PostgreSQL Tuning
        </button>
      </div>

      <!-- MariaDB Tuning Form -->
      <div v-if="tuningEngine === 'mariadb'" class="aura-card">
        <div class="flex items-center justify-between mb-4">
          <div>
            <h2 class="text-lg font-bold text-white">{{ t('database_manager.tuning.mariadb_title') }}</h2>
            <p class="text-sm text-gray-400">{{ t('database_manager.tuning.mariadb_desc') }}</p>
          </div>
          <button class="btn-secondary" @click="loadTuning">{{ t('common.refresh') || 'Yenile' }}</button>
        </div>
        
        <div class="grid grid-cols-1 md:grid-cols-2 gap-6">
          <div>
            <label class="block text-sm text-gray-400 mb-1">Max Connections</label>
            <input v-model="tuningForm.max_connections" type="text" class="aura-input w-full" placeholder="Örn: 151" />
            <p class="text-xs text-gray-500 mt-1">{{ t('database_manager.tuning.max_conn_desc') }}</p>
          </div>
          <div>
            <label class="block text-sm text-gray-400 mb-1">InnoDB Buffer Pool Size</label>
            <input v-model="tuningForm.innodb_buffer_pool_size" type="text" class="aura-input w-full" placeholder="Örn: 128M" />
            <p class="text-xs text-gray-500 mt-1">{{ t('database_manager.tuning.innodb_pool_desc') }}</p>
          </div>
          <div>
            <label class="block text-sm text-gray-400 mb-1">Key Buffer Size</label>
            <input v-model="tuningForm.key_buffer_size" type="text" class="aura-input w-full" placeholder="Örn: 16M" />
            <p class="text-xs text-gray-500 mt-1">{{ t('database_manager.tuning.key_buffer_desc') }}</p>
          </div>
          <div>
            <label class="block text-sm text-gray-400 mb-1">Max Allowed Packet</label>
            <input v-model="tuningForm.max_allowed_packet" type="text" class="aura-input w-full" placeholder="Örn: 16M" />
            <p class="text-xs text-gray-500 mt-1">{{ t('database_manager.tuning.max_packet_desc') }}</p>
          </div>
        </div>
        
        <div class="mt-6 flex justify-end">
          <button class="btn-primary" @click="saveTuning" :disabled="tuningSaving">
            {{ tuningSaving ? t('database_manager.tuning.saving') : t('database_manager.tuning.save_mariadb') }}
          </button>
        </div>
      </div>

      <!-- PostgreSQL Tuning Form -->
      <div v-if="tuningEngine === 'postgresql'" class="aura-card">
        <div class="flex items-center justify-between mb-4">
          <div>
            <h2 class="text-lg font-bold text-white">{{ t('database_manager.tuning.pg_title') }}</h2>
            <p class="text-sm text-gray-400">{{ t('database_manager.tuning.pg_desc') }}</p>
          </div>
          <button class="btn-secondary" @click="loadTuning">{{ t('common.refresh') || 'Yenile' }}</button>
        </div>
        
        <div class="grid grid-cols-1 md:grid-cols-2 gap-6">
          <div>
            <label class="block text-sm text-gray-400 mb-1">Max Connections</label>
            <input v-model="pgTuningForm.max_connections" type="text" class="aura-input w-full" placeholder="Örn: 100" />
            <p class="text-xs text-gray-500 mt-1">{{ t('database_manager.tuning.max_conn_desc') }}</p>
          </div>
          <div>
            <label class="block text-sm text-gray-400 mb-1">Shared Buffers</label>
            <input v-model="pgTuningForm.shared_buffers" type="text" class="aura-input w-full" placeholder="Örn: 128MB" />
            <p class="text-xs text-gray-500 mt-1">{{ t('database_manager.tuning.shared_buffers_desc') }}</p>
          </div>
          <div>
            <label class="block text-sm text-gray-400 mb-1">Work Mem</label>
            <input v-model="pgTuningForm.work_mem" type="text" class="aura-input w-full" placeholder="Örn: 4MB" />
            <p class="text-xs text-gray-500 mt-1">{{ t('database_manager.tuning.work_mem_desc') }}</p>
          </div>
          <div>
            <label class="block text-sm text-gray-400 mb-1">Maintenance Work Mem</label>
            <input v-model="pgTuningForm.maintenance_work_mem" type="text" class="aura-input w-full" placeholder="Örn: 64MB" />
            <p class="text-xs text-gray-500 mt-1">{{ t('database_manager.tuning.maintenance_work_mem_desc') }}</p>
          </div>
        </div>
        
        <div class="mt-6 flex justify-end">
          <button class="btn-primary" @click="saveTuning" :disabled="tuningSaving">
            {{ tuningSaving ? t('database_manager.tuning.saving') : t('database_manager.tuning.save_pg') }}
          </button>
        </div>
      </div>
    </div>

    <div v-if="engine !== 'tuning'" class="grid grid-cols-1 md:grid-cols-3 gap-4">
      <div class="bg-panel-card border border-panel-border rounded-xl p-5">
        <p class="text-sm text-gray-400">{{ t('database_manager.stats.databases') }}</p>
        <p class="text-2xl font-bold text-white mt-1">{{ currentDatabases.length }}</p>
      </div>
      <div class="bg-panel-card border border-panel-border rounded-xl p-5">
        <p class="text-sm text-gray-400">{{ t('database_manager.stats.users') }}</p>
        <p class="text-2xl font-bold text-white mt-1">{{ currentUsers.length }}</p>
      </div>
      <div class="bg-panel-card border border-panel-border rounded-xl p-5">
        <p class="text-sm text-gray-400">{{ t('database_manager.stats.engine') }}</p>
        <p class="text-2xl font-bold text-white mt-1">{{ currentEngineLabel }}</p>
      </div>
    </div>

    <div v-if="engine !== 'tuning'" class="bg-panel-card border border-panel-border rounded-xl overflow-hidden">
      <div class="p-4 border-b border-panel-border flex items-center justify-between">
        <h2 class="text-lg font-semibold text-white">{{ t('database_manager.sections.databases', { engine: currentEngineLabel }) }}</h2>
        <button @click="loadData" class="px-3 py-1.5 bg-panel-hover text-gray-300 rounded-lg text-sm hover:bg-gray-600 transition">{{ t('database_manager.actions.refresh') }}</button>
      </div>
      <div class="overflow-x-auto">
        <table class="w-full text-sm">
          <thead>
            <tr class="text-gray-400 border-b border-panel-border">
              <th class="text-left px-4 py-3 font-medium">{{ t('database_manager.table.database') }}</th>
              <th class="text-left px-4 py-3 font-medium">{{ t('database_manager.table.size') }}</th>
              <th class="text-left px-4 py-3 font-medium">{{ t('database_manager.table.tables') }}</th>
              <th class="text-left px-4 py-3 font-medium">{{ t('database_manager.table.engine') }}</th>
              <th class="text-right px-4 py-3 font-medium">{{ t('database_manager.table.action') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="db in currentDatabases" :key="db.name" class="border-b border-panel-border/50 hover:bg-white/[0.02] transition">
              <td class="px-4 py-3 text-white font-medium font-mono">{{ db.name }}</td>
              <td class="px-4 py-3 text-gray-300">{{ db.size }}</td>
              <td class="px-4 py-3 text-gray-400">{{ db.tables }}</td>
              <td class="px-4 py-3">
                <span :class="['px-2 py-0.5 rounded text-xs font-medium', db.engine === 'mariadb' ? 'bg-orange-500/15 text-orange-400' : 'bg-blue-500/15 text-blue-400']">
                  {{ db.engine === 'mariadb' ? t('database_manager.engines.mariadb') : t('database_manager.engines.postgresql') }}
                </span>
              </td>
              <td class="px-4 py-3 text-right space-x-2">
                <button @click="goAttachToWebsite(db)" class="px-2 py-1 bg-blue-600/20 text-blue-300 rounded text-xs hover:bg-blue-600/40 transition">{{ t('database_manager.actions.attach_to_site') }}</button>
                <button @click="dropDatabase(db.name)" class="px-2 py-1 bg-red-600/20 text-red-400 rounded text-xs hover:bg-red-600/40 transition">{{ t('database_manager.actions.delete') }}</button>
              </td>
            </tr>
            <tr v-if="currentDatabases.length === 0">
              <td colspan="5" class="px-4 py-12 text-center text-gray-500">{{ t('database_manager.table.empty_databases') }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <div v-if="engine !== 'tuning'" class="bg-panel-card border border-panel-border rounded-xl overflow-hidden">
      <div class="p-4 border-b border-panel-border">
        <h2 class="text-lg font-semibold text-white">{{ t('database_manager.sections.users') }}</h2>
      </div>
      <div class="overflow-x-auto">
        <table class="w-full text-sm">
          <thead>
            <tr class="text-gray-400 border-b border-panel-border">
              <th class="text-left px-4 py-3 font-medium">{{ t('database_manager.table.user') }}</th>
              <th class="text-left px-4 py-3 font-medium">{{ t('database_manager.table.host') }}</th>
              <th class="text-left px-4 py-3 font-medium">{{ t('database_manager.table.engine') }}</th>
              <th class="text-right px-4 py-3 font-medium">{{ t('database_manager.table.action') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="u in currentUsers" :key="u.username" class="border-b border-panel-border/50 hover:bg-white/[0.02] transition">
              <td class="px-4 py-3 text-white font-medium font-mono">{{ u.username }}</td>
              <td class="px-4 py-3 text-gray-400">{{ u.host }}</td>
              <td class="px-4 py-3">
                <span :class="['px-2 py-0.5 rounded text-xs font-medium', u.engine === 'mariadb' ? 'bg-orange-500/15 text-orange-400' : 'bg-blue-500/15 text-blue-400']">
                  {{ u.engine === 'mariadb' ? t('database_manager.engines.mariadb') : t('database_manager.engines.postgresql') }}
                </span>
              </td>
              <td class="px-4 py-3 text-right space-x-2">
                <button @click="rotatePassword(u)" class="px-2 py-1 bg-indigo-600/20 text-indigo-300 rounded text-xs hover:bg-indigo-600/40 transition">{{ t('database_manager.actions.password') }}</button>
                <button @click="allowRemote(u)" class="px-2 py-1 bg-teal-600/20 text-teal-300 rounded text-xs hover:bg-teal-600/40 transition">{{ t('database_manager.actions.remote_ip') }}</button>
              </td>
            </tr>
            <tr v-if="currentUsers.length === 0">
              <td colspan="4" class="px-4 py-10 text-center text-gray-500">{{ t('database_manager.table.empty_users') }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <div v-if="engine !== 'tuning'" class="bg-panel-card border border-panel-border rounded-xl overflow-hidden">
      <div class="p-4 border-b border-panel-border flex items-center justify-between">
        <h2 class="text-lg font-semibold text-white">{{ t('database_manager.sections.remote_access') }}</h2>
        <button @click="loadRemoteAccess" class="px-3 py-1.5 bg-panel-hover text-gray-300 rounded-lg text-sm hover:bg-gray-600 transition">{{ t('database_manager.actions.refresh') }}</button>
      </div>
      <div class="overflow-x-auto">
        <table class="w-full text-sm">
          <thead>
            <tr class="text-gray-400 border-b border-panel-border">
              <th class="text-left px-4 py-3 font-medium">{{ t('database_manager.table.engine') }}</th>
              <th class="text-left px-4 py-3 font-medium">{{ t('database_manager.table.user') }}</th>
              <th class="text-left px-4 py-3 font-medium">{{ t('database_manager.table.database') }}</th>
              <th class="text-left px-4 py-3 font-medium">{{ t('database_manager.table.remote') }}</th>
              <th class="text-left px-4 py-3 font-medium">{{ t('database_manager.table.auth') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="rule in currentRemoteRules" :key="`${rule.engine}-${rule.db_user}-${rule.db_name}-${rule.remote}`" class="border-b border-panel-border/50 hover:bg-white/[0.02] transition">
              <td class="px-4 py-3 text-gray-300">{{ rule.engine }}</td>
              <td class="px-4 py-3 text-white font-mono">{{ rule.db_user }}</td>
              <td class="px-4 py-3 text-gray-300">{{ rule.db_name }}</td>
              <td class="px-4 py-3 text-gray-300 font-mono">{{ rule.remote }}</td>
              <td class="px-4 py-3 text-gray-400">{{ rule.auth_method }}</td>
            </tr>
            <tr v-if="currentRemoteRules.length === 0">
              <td colspan="5" class="px-4 py-10 text-center text-gray-500">{{ t('database_manager.table.empty_rules') }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <div v-if="showCreateModal" class="fixed inset-0 bg-black/60 backdrop-blur-sm flex items-center justify-center z-50 p-4" @click.self="showCreateModal = false">
      <div class="bg-panel-card border border-panel-border rounded-2xl w-full max-w-xl p-6 shadow-2xl">
        <h3 class="text-xl font-bold text-white mb-5">{{ t('database_manager.modal.title') }}</h3>

        <div class="space-y-4">
          <div>
            <label class="block text-sm text-gray-400 mb-1">{{ t('database_manager.modal.website') }}</label>
            <select v-model="createForm.site_domain" class="w-full px-4 py-2.5 bg-panel-hover border border-panel-border rounded-lg text-white focus:outline-none focus:border-orange-500">
              <option value="">{{ t('database_manager.modal.standalone') }}</option>
              <option v-for="site in siteOptions" :key="site" :value="site">{{ site }}</option>
            </select>
            <p class="text-xs text-gray-500 mt-2">{{ t('database_manager.modal.website_hint') }}</p>
          </div>

          <div class="rounded-xl border border-panel-border bg-panel-hover/40 p-4 space-y-3">
            <label class="inline-flex items-center gap-2 text-sm text-gray-300">
              <input v-model="createForm.create_website" type="checkbox" class="w-4 h-4 rounded border-panel-border bg-panel-hover" />
              {{ t('database_manager.modal.create_website') }}
            </label>
            <div v-if="createForm.create_website" class="space-y-3">
              <input
                v-model="createForm.new_site_domain"
                type="text"
                class="w-full px-4 py-2.5 bg-panel-hover border border-panel-border rounded-lg text-white placeholder-gray-500 focus:outline-none focus:border-orange-500"
                :placeholder="t('database_manager.modal.new_site_placeholder')"
              />
              <div class="grid grid-cols-1 md:grid-cols-2 gap-3">
                <input v-model="createForm.website_owner" type="text" class="w-full px-4 py-2.5 bg-panel-hover border border-panel-border rounded-lg text-white" :placeholder="t('database_manager.modal.owner_placeholder')" />
                <select v-model="createForm.website_php" class="w-full px-4 py-2.5 bg-panel-hover border border-panel-border rounded-lg text-white">
                  <option value="8.4">PHP 8.4</option>
                  <option value="8.3">PHP 8.3</option>
                  <option value="8.2">PHP 8.2</option>
                  <option value="8.1">PHP 8.1</option>
                  <option value="8.0">PHP 8.0</option>
                  <option value="7.4">PHP 7.4</option>
                </select>
              </div>
              <div class="grid grid-cols-1 md:grid-cols-2 gap-3">
                <select v-model="createForm.website_package" class="w-full px-4 py-2.5 bg-panel-hover border border-panel-border rounded-lg text-white">
                  <option v-for="pkg in websitePackageOptions" :key="`db-create-${pkg}`" :value="pkg">{{ pkg }}</option>
                </select>
                <input v-model="createForm.website_email" type="email" class="w-full px-4 py-2.5 bg-panel-hover border border-panel-border rounded-lg text-white" :placeholder="t('database_manager.modal.email_placeholder')" />
              </div>
              <div class="flex flex-wrap gap-4 text-sm text-gray-300">
                <label class="inline-flex items-center gap-2">
                  <input v-model="createForm.website_mail_domain" type="checkbox" class="w-4 h-4 rounded border-panel-border bg-panel-hover" />
                  {{ t('database_manager.modal.mail_domain') }}
                </label>
                <label class="inline-flex items-center gap-2">
                  <input v-model="createForm.website_apache_backend" type="checkbox" class="w-4 h-4 rounded border-panel-border bg-panel-hover" />
                  {{ t('database_manager.modal.apache_backend') }}
                </label>
              </div>
            </div>
          </div>

          <div>
            <label class="block text-sm text-gray-400 mb-1">{{ t('database_manager.modal.engine') }}</label>
            <select v-model="createForm.engine" class="w-full px-4 py-2.5 bg-panel-hover border border-panel-border rounded-lg text-white focus:outline-none focus:border-orange-500">
              <option value="mariadb">{{ t('database_manager.engines.mariadb_mysql') }}</option>
              <option value="postgresql">{{ t('database_manager.engines.postgresql') }}</option>
            </select>
          </div>

          <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
            <div>
              <label class="block text-sm text-gray-400 mb-1">{{ t('database_manager.modal.db_name') }}</label>
              <input v-model="createForm.db_name" type="text" :placeholder="t('database_manager.modal.db_name_placeholder')" class="w-full px-4 py-2.5 bg-panel-hover border border-panel-border rounded-lg text-white placeholder-gray-500 focus:outline-none focus:border-orange-500">
            </div>
            <div>
              <label class="block text-sm text-gray-400 mb-1">{{ t('database_manager.modal.db_user') }}</label>
              <input v-model="createForm.db_user" type="text" :placeholder="t('database_manager.modal.db_user_placeholder')" class="w-full px-4 py-2.5 bg-panel-hover border border-panel-border rounded-lg text-white placeholder-gray-500 focus:outline-none focus:border-orange-500">
            </div>
          </div>

          <div v-if="targetSiteDomain" class="rounded-xl border border-orange-500/20 bg-orange-500/5 p-4">
            <p class="text-xs uppercase tracking-[0.18em] text-orange-300">{{ t('database_manager.modal.preview_title') }}</p>
            <div class="mt-3 grid grid-cols-1 md:grid-cols-2 gap-3 text-sm">
              <div class="rounded-lg bg-black/20 border border-white/5 px-3 py-2">
                <p class="text-gray-500 text-xs mb-1">{{ t('database_manager.modal.preview_db') }}</p>
                <p class="text-white font-mono">{{ previewDbName }}</p>
              </div>
              <div class="rounded-lg bg-black/20 border border-white/5 px-3 py-2">
                <p class="text-gray-500 text-xs mb-1">{{ t('database_manager.modal.preview_user') }}</p>
                <p class="text-white font-mono">{{ previewDbUser }}</p>
              </div>
            </div>
          </div>

          <div>
            <label class="block text-sm text-gray-400 mb-1">{{ t('database_manager.modal.password') }}</label>
            <input v-model="createForm.db_pass" type="password" :placeholder="t('database_manager.modal.password_placeholder')" class="w-full px-4 py-2.5 bg-panel-hover border border-panel-border rounded-lg text-white placeholder-gray-500 focus:outline-none focus:border-orange-500">
          </div>
        </div>

        <div class="flex gap-3 mt-6">
          <button
            @click="createDatabase"
            :disabled="!canCreate"
            class="flex-1 py-2.5 bg-gradient-to-r from-orange-600 to-amber-600 text-white rounded-lg font-medium hover:from-orange-700 hover:to-amber-700 transition disabled:opacity-50 disabled:cursor-not-allowed"
          >
            {{ t('database_manager.modal.create') }}
          </button>
          <button @click="showCreateModal = false" class="px-5 py-2.5 bg-panel-hover text-gray-300 rounded-lg hover:bg-gray-600 transition">{{ t('database_manager.modal.cancel') }}</button>
        </div>
      </div>
    </div>

    <div v-if="notification" :class="['fixed bottom-6 right-6 px-5 py-3 rounded-xl shadow-2xl text-sm font-medium z-50', notification.type === 'success' ? 'bg-green-600 text-white' : 'bg-red-600 text-white']">
      {{ notification.message }}
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { Database, Plus, Settings2 } from 'lucide-vue-next'
import api from '../services/api'

const route = useRoute()
const router = useRouter()
const { t } = useI18n({ useScope: 'global' })

const normalizeMainEngine = (query) => {
  if (query?.tab === 'tuning') return 'tuning'
  return query?.engine === 'postgresql' ? 'postgresql' : 'mariadb'
}

const normalizeTuningEngine = (query) =>
  query?.tuning_engine === 'postgresql' ? 'postgresql' : 'mariadb'

const engine = ref(normalizeMainEngine(route.query))
const tuningEngine = ref(normalizeTuningEngine(route.query))
const showCreateModal = ref(false)
const createActionConsumed = ref(false)
const notification = ref(null)
const launchingTool = ref('')

const tuningForm = ref({
  max_connections: '',
  innodb_buffer_pool_size: '',
  key_buffer_size: '',
  max_allowed_packet: ''
})

const pgTuningForm = ref({
  max_connections: '',
  shared_buffers: '',
  work_mem: '',
  maintenance_work_mem: ''
})

const tuningSaving = ref(false)

async function loadTuning() {
  try {
    if (tuningEngine.value === 'mariadb') {
      const res = await api.get('/db/mariadb/tuning')
      if (res.data?.data) {
        tuningForm.value = { ...tuningForm.value, ...res.data.data }
      }
    } else {
      const res = await api.get('/db/postgresql/tuning')
      if (res.data?.data) {
        pgTuningForm.value = { ...pgTuningForm.value, ...res.data.data }
      }
    }
  } catch (err) {
    console.error('Tuning load error', err)
  }
}

async function saveTuning() {
  tuningSaving.value = true
  try {
    if (tuningEngine.value === 'mariadb') {
      await api.post('/db/mariadb/tuning', tuningForm.value)
      showNotif('MariaDB Tuning ayarları kaydedildi ve servis yeniden başlatıldı.', 'success')
    } else {
      await api.post('/db/postgresql/tuning', pgTuningForm.value)
      showNotif('PostgreSQL Tuning ayarları kaydedildi ve servis yeniden başlatıldı.', 'success')
    }
  } catch (err) {
    showNotif('Tuning kaydedilemedi: ' + err.message, 'error')
  } finally {
    tuningSaving.value = false
  }
}

const mariadbDatabases = ref([])
const postgresDatabases = ref([])
const mariadbUsers = ref([])
const postgresUsers = ref([])
const mariadbRemoteRules = ref([])
const postgresRemoteRules = ref([])
const sites = ref([])
const hostingPackages = ref([])

const currentDatabases = computed(() => (engine.value === 'mariadb' ? mariadbDatabases.value : postgresDatabases.value))
const currentUsers = computed(() => (engine.value === 'mariadb' ? mariadbUsers.value : postgresUsers.value))
const currentRemoteRules = computed(() => (engine.value === 'mariadb' ? mariadbRemoteRules.value : postgresRemoteRules.value))
const currentEngineLabel = computed(() => (engine.value === 'mariadb'
  ? t('database_manager.engines.mariadb')
  : t('database_manager.engines.postgresql')))
const siteOptions = computed(() => sites.value.map(site => site.domain).filter(Boolean))
const websitePackageOptions = computed(() => {
  const names = new Set(['default'])
  for (const pkg of hostingPackages.value || []) {
    const name = String(pkg?.name || '').trim()
    if (name) names.add(name)
  }
  const ordered = Array.from(names).filter(Boolean)
  const tail = ordered.filter(name => name !== 'default').sort((a, b) => a.localeCompare(b))
  return ['default', ...tail]
})

const createForm = ref({
  engine: 'mariadb',
  site_domain: '',
  create_website: false,
  new_site_domain: '',
  website_owner: 'aura',
  website_php: '8.3',
  website_package: 'default',
  website_email: '',
  website_mail_domain: false,
  website_apache_backend: false,
  db_name: '',
  db_user: '',
  db_pass: '',
})

const targetSiteDomain = computed(() => {
  if (createForm.value.create_website) {
    return String(createForm.value.new_site_domain || '').trim().toLowerCase()
  }
  return String(createForm.value.site_domain || '').trim().toLowerCase()
})

const canCreate = computed(() => {
  const baseReady = Boolean(createForm.value.db_name && createForm.value.db_user && createForm.value.db_pass)
  if (!baseReady) return false
  if (createForm.value.create_website) {
    return Boolean(String(createForm.value.new_site_domain || '').trim())
  }
  return true
})

const sitePrefix = computed(() => {
  const raw = targetSiteDomain.value
  if (!raw) return ''
  return raw
    .split('.')[0]
    .replace(/[^a-z0-9]/g, '')
    .slice(0, 10)
})

const previewDbName = computed(() => buildResolvedName(createForm.value.db_name, 'database'))
const previewDbUser = computed(() => buildResolvedName(createForm.value.db_user, 'user'))

function sanitizeToken(value, fallback, maxLen = 24) {
  const cleaned = String(value || '')
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '_')
    .replace(/^_+|_+$/g, '')
    .slice(0, maxLen)

  return cleaned || fallback
}

function buildResolvedName(value, fallback) {
  const base = sanitizeToken(value, fallback)
  if (!sitePrefix.value) return base
  const prefixed = `${sitePrefix.value}_${base}`
  return prefixed.slice(0, 60).replace(/^_+|_+$/g, '') || base
}

function apiErrorMessage(error, fallbackKey) {
  return error?.response?.data?.message || error?.message || t(fallbackKey)
}

function resolveSiteOwner(domain) {
  const normalized = String(domain || '').trim().toLowerCase()
  if (!normalized) return ''
  const site = (sites.value || []).find(s => String(s.domain || '').trim().toLowerCase() === normalized)
  return String(site?.owner || site?.user || '').trim()
}

const showNotif = (message, type = 'success') => {
  notification.value = { message, type }
  setTimeout(() => {
    notification.value = null
  }, 3500)
}

const launchDatabaseTool = async (tool) => {
  const normalizedTool = tool === 'phpmyadmin' ? 'phpmyadmin' : 'pgadmin'
  launchingTool.value = normalizedTool
  try {
    const endpoint = normalizedTool === 'phpmyadmin'
      ? '/db/tools/phpmyadmin/sso'
      : '/db/tools/pgadmin/sso'
    const response = await api.post(endpoint, { ttl_seconds: 120 })
    const launchUrl = String(response?.data?.data?.url || '').trim()
    if (!launchUrl) {
      throw new Error(t('database_manager.notifications.tool_launch_failed'))
    }

    const openedWindow = window.open(launchUrl, '_blank', 'noopener,noreferrer')
    if (!openedWindow) {
      window.location.href = launchUrl
    }
  } catch (error) {
    showNotif(apiErrorMessage(error, 'database_manager.notifications.tool_launch_failed'), 'error')
  } finally {
    launchingTool.value = ''
  }
}

function syncRouteQuery() {
  const nextQuery = engine.value === 'tuning'
    ? { tab: 'tuning', tuning_engine: tuningEngine.value }
    : engine.value === 'postgresql'
      ? { engine: 'postgresql' }
      : {}

  const currentTab = typeof route.query.tab === 'string' ? route.query.tab : ''
  const currentEngine = typeof route.query.engine === 'string' ? route.query.engine : ''
  const currentTuningEngine = typeof route.query.tuning_engine === 'string' ? route.query.tuning_engine : ''
  const currentDomain = typeof route.query.domain === 'string' ? route.query.domain : ''
  const currentAction = typeof route.query.action === 'string' ? route.query.action : ''

  if (currentDomain) {
    nextQuery.domain = currentDomain
  }
  if (currentAction === 'create') {
    nextQuery.action = 'create'
  }

  const sameQuery = currentTab === (nextQuery.tab || '')
    && currentEngine === (nextQuery.engine || '')
    && currentTuningEngine === (nextQuery.tuning_engine || '')
    && currentDomain === (nextQuery.domain || '')
    && currentAction === (nextQuery.action || '')

  if (!sameQuery) {
    router.replace({ path: '/databases', query: nextQuery })
  }
}

function applyDomainQuery(query) {
  const requestedDomain = String(query?.domain || '').trim().toLowerCase()
  if (!requestedDomain) return

  const matched = siteOptions.value.find((siteDomain) => String(siteDomain || '').trim().toLowerCase() === requestedDomain)
  if (matched) {
    createForm.value.site_domain = matched
  }
}

function resetCreateForm() {
  createForm.value = {
    engine: engine.value,
    site_domain: createForm.value.site_domain || siteOptions.value[0] || '',
    create_website: false,
    new_site_domain: '',
    website_owner: 'aura',
    website_php: '8.3',
    website_package: websitePackageOptions.value[0] || 'default',
    website_email: '',
    website_mail_domain: false,
    website_apache_backend: false,
    db_name: '',
    db_user: '',
    db_pass: '',
  }
}

const loadData = async () => {
  const requests = await Promise.allSettled([
    api.get('/db/mariadb/list'),
    api.get('/db/mariadb/users'),
    api.get('/db/mariadb/remote-access'),
    api.get('/db/postgres/list'),
    api.get('/db/postgres/users'),
    api.get('/db/postgres/remote-access'),
    api.get('/vhost/list'),
    api.get('/packages/list'),
  ])

  mariadbDatabases.value = requests[0].status === 'fulfilled' ? requests[0].value.data?.data || [] : []
  mariadbUsers.value = requests[1].status === 'fulfilled' ? requests[1].value.data?.data || [] : []
  mariadbRemoteRules.value = requests[2].status === 'fulfilled' ? requests[2].value.data?.data || [] : []
  postgresDatabases.value = requests[3].status === 'fulfilled' ? requests[3].value.data?.data || [] : []
  postgresUsers.value = requests[4].status === 'fulfilled' ? requests[4].value.data?.data || [] : []
  postgresRemoteRules.value = requests[5].status === 'fulfilled' ? requests[5].value.data?.data || [] : []
  sites.value = requests[6].status === 'fulfilled' ? requests[6].value.data?.data || [] : []
  hostingPackages.value = requests[7].status === 'fulfilled' ? requests[7].value.data?.data || [] : []

  if (!createForm.value.site_domain && siteOptions.value.length > 0) {
    createForm.value.site_domain = siteOptions.value[0]
  }
  applyDomainQuery(route.query)
}

const loadRemoteAccess = async () => {
  const requests = await Promise.allSettled([
    api.get('/db/mariadb/remote-access'),
    api.get('/db/postgres/remote-access'),
  ])

  mariadbRemoteRules.value = requests[0].status === 'fulfilled' ? requests[0].value.data?.data || [] : []
  postgresRemoteRules.value = requests[1].status === 'fulfilled' ? requests[1].value.data?.data || [] : []
}

const createDatabase = async () => {
  if (!canCreate.value) return

  const eng = createForm.value.engine
  const desiredSiteDomain = targetSiteDomain.value || ''
  const resolvedOwner = createForm.value.create_website
    ? String(createForm.value.website_owner || 'aura').trim()
    : resolveSiteOwner(desiredSiteDomain)

  try {
    if (createForm.value.create_website) {
      await api.post('/vhost', {
        domain: desiredSiteDomain,
        user: createForm.value.website_owner || 'aura',
        php_version: createForm.value.website_php || '8.3',
        package: createForm.value.website_package || 'default',
        email: createForm.value.website_email || undefined,
        mail_domain: !!createForm.value.website_mail_domain,
        apache_backend: !!createForm.value.website_apache_backend,
      })
    }

    let response
    if (eng === 'mariadb') {
      response = await api.post('/db/mariadb/create', {
        db_name: createForm.value.db_name,
        db_user: createForm.value.db_user,
        db_pass: createForm.value.db_pass,
        site_domain: desiredSiteDomain || null,
        owner: resolvedOwner || null,
      })
    } else {
      response = await api.post('/db/postgres/create', {
        db_name: createForm.value.db_name,
        db_user: createForm.value.db_user,
        db_pass: createForm.value.db_pass,
        site_domain: desiredSiteDomain || null,
        owner: resolvedOwner || null,
      })
    }

    const created = response.data?.data || {}
    let attachNote = ''

    if (desiredSiteDomain && created.db_name && created.db_user) {
      try {
        await api.post('/websites/db-links', {
          domain: desiredSiteDomain,
          engine: created.engine || eng,
          db_name: created.db_name,
          db_user: created.db_user,
        })
        const verify = await api.post('/websites/db-links/verify', {
          domain: desiredSiteDomain,
          engine: created.engine || eng,
          db_name: created.db_name,
          db_user: created.db_user,
        })
        const ready = Boolean(verify.data?.data?.ready)
        attachNote = ready
          ? t('database_manager.notifications.connection_verified')
          : t('database_manager.notifications.connection_pending')
      } catch {
        attachNote = t('database_manager.notifications.connection_auto_failed')
      }
    }

    showNotif(`${t('database_manager.notifications.created', { name: created.db_name || previewDbName.value })}${attachNote}`)
    showCreateModal.value = false
    resetCreateForm()
    await loadData()
  } catch (error) {
    showNotif(apiErrorMessage(error, 'database_manager.notifications.create_failed'), 'error')
  }
}

const dropDatabase = async (name) => {
  try {
    if (engine.value === 'mariadb') {
      await api.post('/db/mariadb/drop', { name })
    } else {
      await api.post('/db/postgres/drop', { name })
    }
    showNotif(t('database_manager.notifications.deleted', { name }))
    await loadData()
  } catch (error) {
    showNotif(apiErrorMessage(error, 'database_manager.notifications.delete_failed'), 'error')
  }
}

const rotatePassword = async (user) => {
  const newPassword = window.prompt(t('database_manager.prompts.new_password', { user: user.username }))
  if (!newPassword) return

  try {
    if (user.engine === 'mariadb') {
      await api.post('/db/mariadb/password', {
        db_user: user.username,
        new_password: newPassword,
        host: user.host || null,
      })
    } else {
      await api.post('/db/postgres/password', {
        db_user: user.username,
        new_password: newPassword,
        host: user.host || null,
      })
    }
    showNotif(t('database_manager.notifications.password_updated', { user: user.username }))
  } catch (error) {
    showNotif(apiErrorMessage(error, 'database_manager.notifications.password_failed'), 'error')
  }
}

const allowRemote = async (user) => {
  const remoteIp = window.prompt(t('database_manager.prompts.remote_ip', { user: user.username }))
  if (!remoteIp) return

  const dbName = window.prompt(t('database_manager.prompts.remote_db'), currentDatabases.value[0]?.name || '')
  if (!dbName) return

  let dbPass = ''
  if (user.engine === 'mariadb') {
    dbPass = window.prompt(t('database_manager.prompts.remote_password')) || ''
    if (!dbPass) {
      showNotif(t('database_manager.notifications.remote_required'), 'error')
      return
    }
  }

  const endpoint = user.engine === 'mariadb' ? '/db/mariadb/remote-access' : '/db/postgres/remote-access'
  try {
    await api.post(endpoint, {
      db_user: user.username,
      db_name: dbName,
      remote_ip: remoteIp,
      db_pass: dbPass || null,
    })
    showNotif(t('database_manager.notifications.remote_added', { user: user.username }))
    await loadRemoteAccess()
  } catch (error) {
    showNotif(apiErrorMessage(error, 'database_manager.notifications.remote_failed'), 'error')
  }
}

const goAttachToWebsite = (db) => {
  router.push({
    path: '/websites',
    query: {
      tab: 'db-links',
      engine: db.engine,
      db_name: db.name,
    },
  })
}

watch(engine, (value) => {
  if (value === 'tuning') {
    loadTuning()
  } else {
    createForm.value.engine = value
  }
  syncRouteQuery()
})

watch(tuningEngine, () => {
  if (engine.value === 'tuning') {
    loadTuning()
  }
  syncRouteQuery()
})

watch(
  () => route.query,
  (query) => {
    const nextEngine = normalizeMainEngine(query)
    const nextTuningEngine = normalizeTuningEngine(query)
    if (engine.value !== nextEngine) {
      engine.value = nextEngine
    }
    if (tuningEngine.value !== nextTuningEngine) {
      tuningEngine.value = nextTuningEngine
    }

    applyDomainQuery(query)

    const shouldOpenCreate = query?.action === 'create'
    if (shouldOpenCreate && !createActionConsumed.value) {
      showCreateModal.value = true
      createActionConsumed.value = true
    }
    if (!shouldOpenCreate) {
      createActionConsumed.value = false
    }
  },
  { immediate: true },
)

watch(showCreateModal, (open) => {
  if (open && !createForm.value.site_domain && siteOptions.value.length > 0) {
    createForm.value.site_domain = siteOptions.value[0]
  }
})

watch(siteOptions, () => {
  applyDomainQuery(route.query)
})

watch(websitePackageOptions, (options) => {
  if (!options.length) return
  if (!options.includes(createForm.value.website_package)) {
    createForm.value.website_package = options[0]
  }
}, { immediate: true })

onMounted(async () => {
  await loadData()
  resetCreateForm()
  if (engine.value === 'tuning') {
    await loadTuning()
  }
  syncRouteQuery()
})
</script>
