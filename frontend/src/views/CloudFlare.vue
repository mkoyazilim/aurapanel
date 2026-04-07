<template>
  <div class="cloudflare-view space-y-6">
    <div class="flex items-center justify-between">
      <div>
        <h1 class="flex items-center gap-3 text-2xl font-bold text-white">
          <Cloud class="h-7 w-7 text-orange-400" />
          {{ t('cloudflare_manager.title') }}
        </h1>
        <p class="mt-1 text-gray-400">{{ t('cloudflare_manager.subtitle') }}</p>
      </div>
    </div>

    <div class="rounded-xl border border-panel-border bg-panel-card p-5">
      <div class="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
        <div>
          <p class="text-sm font-semibold text-white">{{ t('cloudflare_manager.server_status.title') }}</p>
          <p class="mt-1 text-sm text-gray-400">
            {{
              serverStatus.configured
                ? t('cloudflare_manager.server_status.ready', {
                    source: serverStatus.credential_source,
                    email: serverStatus.email_hint || '-',
                  })
                : t('cloudflare_manager.server_status.missing')
            }}
          </p>
        </div>
        <div class="flex flex-wrap items-center gap-2">
          <span class="rounded-full px-3 py-1 text-xs font-medium" :class="serverStatus.configured ? 'bg-green-500/15 text-green-400' : 'bg-yellow-500/15 text-yellow-300'">
            {{ serverStatus.configured ? t('cloudflare_manager.server_status.configured') : t('cloudflare_manager.server_status.not_configured') }}
          </span>
          <span class="rounded-full px-3 py-1 text-xs font-medium" :class="serverStatus.auto_sync ? 'bg-cyan-500/15 text-cyan-300' : 'bg-gray-500/15 text-gray-400'">
            {{ serverStatus.auto_sync ? t('cloudflare_manager.server_status.auto_sync_on') : t('cloudflare_manager.server_status.auto_sync_off') }}
          </span>
          <span class="rounded-full px-3 py-1 text-xs font-medium" :class="connected ? 'bg-emerald-500/15 text-emerald-300' : 'bg-orange-500/15 text-orange-300'">
            {{ connected ? t('cloudflare_manager.login.token_valid') : t('cloudflare_manager.login.token_invalid') }}
          </span>
        </div>
      </div>
    </div>

    <div class="border-b border-panel-border">
      <nav class="flex gap-6">
        <button
          v-for="tabItem in tabs"
          :key="tabItem.id"
          class="flex items-center gap-2 pb-3 text-sm font-medium transition"
          :class="activeTab === tabItem.id ? 'border-b-2 border-orange-400 text-orange-400' : 'text-gray-400 hover:text-white'"
          @click="activeTab = tabItem.id"
        >
          {{ tabItem.label }}
        </button>
      </nav>
    </div>

    <div v-if="activeTab === 'login'" class="rounded-xl border border-panel-border bg-panel-card p-6">
      <h2 class="cf-connect-title mb-1 text-lg font-semibold">{{ t('cloudflare_manager.login.title') }}</h2>
      <p class="mb-4 text-sm text-gray-400">{{ t('cloudflare_manager.login.description') }}</p>
      <div
        class="mb-4 rounded-xl border p-4 text-sm"
        :class="connected ? 'border-green-500/20 bg-green-500/10 text-green-200' : 'border-yellow-500/20 bg-yellow-500/10 text-yellow-200'"
      >
        <p class="font-semibold">{{ t('cloudflare_manager.login.status_label') }}</p>
        <p class="mt-1">{{ connected ? t('cloudflare_manager.login.connected') : t('cloudflare_manager.login.disconnected') }}</p>
        <p class="text-xs opacity-90">
          {{
            connected
              ? t('cloudflare_manager.login.connected_detail', { count: zones.length })
              : t('cloudflare_manager.login.disconnected_detail')
          }}
        </p>
        <p class="mt-1 text-xs font-medium">{{ connected ? t('cloudflare_manager.login.token_valid') : t('cloudflare_manager.login.token_invalid') }}</p>
      </div>
      <div v-if="serverStatus.configured" class="mb-4 rounded-xl border border-green-500/20 bg-green-500/10 p-4 text-sm text-green-200">
        {{ t('cloudflare_manager.server_status.server_auth_active') }}
      </div>
      <div class="grid grid-cols-1 gap-4 md:grid-cols-2">
        <div>
          <label class="cf-field-label mb-1 block text-sm">{{ t('cloudflare_manager.email') }}</label>
          <input v-model="cfEmail" type="email" :placeholder="t('cloudflare_manager.placeholders.email')" class="aura-input" />
        </div>
        <div>
          <label class="cf-field-label mb-1 block text-sm">{{ t('cloudflare_manager.api_key') }}</label>
          <input v-model="cfApiKey" type="password" :placeholder="t('cloudflare_manager.placeholders.api_key')" class="aura-input" />
        </div>
      </div>
      <div class="mt-4 flex flex-wrap items-center gap-3">
        <button
          class="rounded-lg bg-gradient-to-r from-orange-600 to-amber-600 px-6 py-2.5 font-medium text-white transition hover:from-orange-700 hover:to-amber-700"
          :disabled="loading"
          @click="connectCf"
        >
          {{ loading ? t('cloudflare_manager.connecting') : t('cloudflare_manager.connect') }}
        </button>
        <button
          v-if="cfEmail || cfApiKey"
          class="rounded-lg bg-gradient-to-r from-cyan-600 to-sky-600 px-4 py-2 text-sm font-medium text-white transition hover:from-cyan-700 hover:to-sky-700 disabled:cursor-not-allowed disabled:opacity-60"
          :disabled="serverSaving"
          @click="saveServerAuth"
        >
          {{ serverSaving ? t('cloudflare_manager.server_status.saving') : t('cloudflare_manager.server_status.save_action') }}
        </button>
      </div>
    </div>

    <div v-if="activeTab !== 'login' && !connected" class="rounded-xl border border-yellow-500/20 bg-yellow-500/10 p-5 text-sm text-yellow-200">
      <p class="font-semibold">{{ t('cloudflare_manager.messages.login_required') }}</p>
      <button class="mt-3 rounded-lg bg-orange-600/20 px-4 py-2 text-xs font-medium text-orange-300 transition hover:bg-orange-600/40" @click="activeTab = 'login'">
        {{ t('cloudflare_manager.tabs.login') }}
      </button>
    </div>

    <div v-if="activeTab !== 'login' && connected" class="rounded-xl border border-panel-border bg-panel-card p-4">
        <div class="flex flex-wrap items-center gap-3">
          <label class="text-sm text-gray-400">{{ t('cloudflare_manager.scope.select_zone') }}</label>
          <select
            :value="selectedZone?.id || ''"
            class="min-w-[260px] rounded-lg border border-panel-border bg-panel-hover px-3 py-2 text-sm text-white focus:border-orange-500 focus:outline-none"
            @change="onScopeZoneChange($event.target.value)"
          >
            <option value="" disabled>{{ t('cloudflare_manager.scope.select_zone') }}</option>
            <option v-for="zone in zones" :key="`scope-zone-${zone.id}`" :value="zone.id">{{ zone.name }}</option>
          </select>
          <span class="text-xs text-gray-500">{{ t('cloudflare_manager.scope.all_actions_apply') }}</span>
        </div>
    </div>

      <div v-if="activeTab === 'zones' && connected">
        <div class="overflow-hidden rounded-xl border border-panel-border bg-panel-card">
          <div class="flex items-center justify-between border-b border-panel-border p-4">
            <h2 class="text-lg font-semibold text-white">{{ t('cloudflare_manager.zones.title') }}</h2>
            <button class="rounded-lg bg-panel-hover px-3 py-1.5 text-sm text-gray-300 transition hover:bg-gray-600" @click="loadZones">
              {{ t('cloudflare_manager.refresh') }}
            </button>
          </div>
          <table class="w-full text-sm">
            <thead>
              <tr class="border-b border-panel-border text-gray-400">
                <th class="px-4 py-3 text-left">{{ t('cloudflare_manager.zones.domain') }}</th>
                <th class="px-4 py-3 text-left">{{ t('cloudflare_manager.zones.status') }}</th>
                <th class="px-4 py-3 text-left">{{ t('cloudflare_manager.zones.plan') }}</th>
                <th class="px-4 py-3 text-left">{{ t('cloudflare_manager.zones.nameservers') }}</th>
                <th class="px-4 py-3 text-right">{{ t('cloudflare_manager.zones.action') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-if="zones.length === 0">
                <td colspan="5" class="px-4 py-4 text-center text-gray-500">{{ t('cloudflare_manager.zones.empty') }}</td>
              </tr>
              <tr v-for="zone in zones" :key="zone.id" class="border-b border-panel-border/50 transition hover:bg-white/[0.02]">
                <td class="px-4 py-3 font-medium text-white">{{ zone.name }}</td>
                <td class="px-4 py-3">
                  <span :class="['rounded px-2 py-0.5 text-xs font-medium', zone.status === 'active' ? 'bg-green-500/15 text-green-400' : 'bg-yellow-500/15 text-yellow-400']">{{ zone.status }}</span>
                </td>
                <td class="px-4 py-3 text-gray-300">{{ zone.plan }}</td>
                <td class="px-4 py-3 font-mono text-xs text-gray-400">{{ zone.name_servers?.join(', ') }}</td>
                <td class="px-4 py-3 text-right">
                  <button class="rounded bg-orange-600/20 px-3 py-1 text-xs text-orange-400 transition hover:bg-orange-600/40" @click="selectZone(zone, { goToDns: true })">
                    {{ t('cloudflare_manager.zones.manage_dns') }}
                  </button>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>

      <div v-if="activeTab === 'dns' && connected" class="space-y-4">
        <div class="flex items-center gap-3">
          <span class="text-sm text-gray-400">{{ t('cloudflare_manager.dns.zone_label') }}:</span>
          <span class="font-semibold text-white">{{ selectedZone?.name || t('cloudflare_manager.dns.not_selected') }}</span>
          <button v-if="selectedZone" class="ml-auto rounded-lg bg-gradient-to-r from-orange-600 to-amber-600 px-4 py-2 text-sm text-white transition hover:from-orange-700 hover:to-amber-700" @click="showAddDns = true">
            {{ t('cloudflare_manager.dns.add_record') }}
          </button>
        </div>
        <div class="overflow-hidden rounded-xl border border-panel-border bg-panel-card">
          <table class="w-full text-sm">
            <thead>
              <tr class="border-b border-panel-border text-gray-400">
                <th class="px-4 py-3 text-left">{{ t('cloudflare_manager.dns.type') }}</th>
                <th class="px-4 py-3 text-left">{{ t('cloudflare_manager.dns.name') }}</th>
                <th class="px-4 py-3 text-left">{{ t('cloudflare_manager.dns.value') }}</th>
                <th class="px-4 py-3 text-left">{{ t('cloudflare_manager.dns.ttl') }}</th>
                <th class="px-4 py-3 text-left">{{ t('cloudflare_manager.dns.proxy') }}</th>
                <th class="px-4 py-3 text-right">{{ t('cloudflare_manager.dns.action') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-if="dnsRecords.length === 0">
                <td colspan="6" class="px-4 py-4 text-center text-gray-500">{{ t('cloudflare_manager.dns.empty') }}</td>
              </tr>
              <tr v-for="record in dnsRecords" :key="record.id" class="border-b border-panel-border/50 transition hover:bg-white/[0.02]">
                <td class="px-4 py-3">
                  <span :class="['rounded px-2 py-0.5 text-xs font-bold', dnsTypeBadge(record.type)]">{{ record.type }}</span>
                </td>
                <td class="px-4 py-3 font-mono text-xs text-white">{{ record.name }}</td>
                <td class="max-w-[200px] truncate px-4 py-3 font-mono text-xs text-gray-300">{{ record.content }}</td>
                <td class="px-4 py-3 text-gray-400">{{ record.ttl === 1 ? t('cloudflare_manager.dns.auto') : record.ttl }}</td>
                <td class="px-4 py-3 text-xs" :class="record.proxied ? 'text-orange-400' : 'text-gray-500'">
                  {{ record.proxied ? t('cloudflare_manager.dns.proxy_on') : t('cloudflare_manager.dns.proxy_off') }}
                </td>
                <td class="px-4 py-3 text-right">
                  <button class="rounded bg-red-600/20 px-2 py-1 text-xs text-red-400 transition hover:bg-red-600/40" @click="deleteDnsRecord(record.id)">
                    {{ t('common.delete') }}
                  </button>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>

      <div v-if="activeTab === 'ssl' && connected" class="grid grid-cols-1 gap-4 md:grid-cols-2">
        <div class="rounded-xl border border-panel-border bg-panel-card p-5">
          <h3 class="mb-4 font-semibold text-white">{{ t('cloudflare_manager.ssl.mode_title') }}</h3>
          <div class="space-y-3">
            <label
              v-for="mode in sslModes"
              :key="mode.value"
              class="flex cursor-pointer items-start gap-3 rounded-lg border p-3 transition hover:bg-white/[0.03]"
              :class="selectedSslMode === mode.value ? 'border-orange-500/30 bg-orange-500/10' : 'border-transparent'"
              @click="setSslMode(mode.value)"
            >
              <input v-model="selectedSslMode" type="radio" :value="mode.value" class="mt-1 accent-orange-500" />
              <div>
                <p class="text-sm font-medium text-white">{{ mode.label }}</p>
                <p class="mt-0.5 text-xs text-gray-400">{{ mode.desc }}</p>
              </div>
            </label>
          </div>
        </div>
        <div class="space-y-4">
          <div class="rounded-xl border border-panel-border bg-panel-card p-5">
            <h3 class="mb-3 font-semibold text-white">{{ t('cloudflare_manager.ssl.always_https') }}</h3>
            <button class="rounded-lg px-4 py-2 text-sm transition" :class="alwaysHttps ? 'bg-green-600 text-white' : 'bg-panel-hover text-gray-400'" @click="toggleAlwaysHttps">
              {{ alwaysHttps ? t('cloudflare_manager.ssl.enabled') : t('cloudflare_manager.ssl.disabled') }}
            </button>
          </div>
          <div class="rounded-xl border border-panel-border bg-panel-card p-5">
            <h3 class="mb-3 font-semibold text-white">{{ t('cloudflare_manager.ssl.minify') }}</h3>
            <div class="flex gap-3">
              <label v-for="option in ['JS', 'CSS', 'HTML']" :key="option" class="flex items-center gap-2 text-sm text-gray-300">
                <input v-model="minifyOptions[option.toLowerCase()]" type="checkbox" class="accent-orange-500" />
                {{ option }}
              </label>
            </div>
            <button class="mt-3 rounded-lg bg-orange-600/20 px-4 py-1.5 text-sm text-orange-400 transition hover:bg-orange-600/40" @click="saveMinify">
              {{ t('cloudflare_manager.ssl.save') }}
            </button>
          </div>
        </div>
      </div>

      <div v-if="activeTab === 'cache' && connected" class="grid grid-cols-1 gap-4 md:grid-cols-2">
        <div class="rounded-xl border border-panel-border bg-panel-card p-6 text-center">
          <div class="mx-auto mb-4 flex h-16 w-16 items-center justify-center rounded-full bg-red-500/10">
            <Trash2 class="h-8 w-8 text-red-400" />
          </div>
          <h3 class="mb-2 text-lg font-semibold text-white">{{ t('cloudflare_manager.cache.purge_all_title') }}</h3>
          <p class="mb-4 text-sm text-gray-400">{{ t('cloudflare_manager.cache.purge_all_desc') }}</p>
          <button class="rounded-lg bg-red-600/20 px-6 py-2.5 font-medium text-red-400 transition hover:bg-red-600/40" @click="purgeAllCache">
            {{ t('cloudflare_manager.cache.purge_all') }}
          </button>
        </div>
        <div class="rounded-xl border border-panel-border bg-panel-card p-6">
          <h3 class="mb-3 font-semibold text-white">{{ t('cloudflare_manager.cache.purge_urls_title') }}</h3>
          <textarea
            v-model="purgeUrls"
            rows="4"
            class="w-full rounded-lg border border-panel-border bg-panel-hover px-4 py-2.5 font-mono text-sm text-white placeholder-gray-500 focus:border-orange-500 focus:outline-none"
            :placeholder="t('cloudflare_manager.cache.purge_urls_placeholder')"
          ></textarea>
          <button class="mt-3 rounded-lg bg-orange-600/20 px-5 py-2 text-sm text-orange-400 transition hover:bg-orange-600/40" @click="purgeSpecificCache">
            {{ t('cloudflare_manager.cache.purge_urls') }}
          </button>
        </div>
      </div>

      <div v-if="activeTab === 'security' && connected" class="grid grid-cols-1 gap-4 md:grid-cols-2">
        <div class="rounded-xl border border-panel-border bg-panel-card p-5">
          <h3 class="mb-4 font-semibold text-white">{{ t('cloudflare_manager.security.level_title') }}</h3>
          <div class="space-y-2">
            <button
              v-for="level in securityLevels"
              :key="level.value"
              class="w-full rounded-lg px-4 py-3 text-left text-sm transition"
              :class="selectedSecurityLevel === level.value ? 'border border-orange-500/30 bg-orange-500/20 text-orange-400' : 'bg-panel-hover text-gray-300 hover:bg-gray-600'"
              @click="setSecurityLevel(level.value)"
            >
              {{ level.label }}
            </button>
          </div>
        </div>
        <div class="space-y-4">
          <div class="rounded-xl border border-panel-border bg-panel-card p-5">
            <h3 class="mb-3 font-semibold text-white">{{ t('cloudflare_manager.security.attack_title') }}</h3>
            <p class="mb-3 text-sm text-gray-400">{{ t('cloudflare_manager.security.attack_desc') }}</p>
            <button
              class="rounded-lg px-5 py-2.5 text-sm font-medium transition"
              :class="selectedSecurityLevel === 'under_attack' ? 'animate-pulse bg-red-600 text-white' : 'bg-red-600/20 text-red-400 hover:bg-red-600/40'"
              @click="setSecurityLevel('under_attack')"
            >
              {{ t('cloudflare_manager.security.attack_button') }}
            </button>
          </div>
          <div class="rounded-xl border border-panel-border bg-panel-card p-5">
            <h3 class="mb-3 font-semibold text-white">{{ t('cloudflare_manager.security.dev_title') }}</h3>
            <p class="mb-3 text-sm text-gray-400">{{ t('cloudflare_manager.security.dev_desc') }}</p>
            <button class="rounded-lg px-5 py-2.5 text-sm transition" :class="devMode ? 'bg-yellow-600 text-white' : 'bg-panel-hover text-gray-400'" @click="toggleDevMode">
              {{ devMode ? t('cloudflare_manager.ssl.enabled') : t('cloudflare_manager.ssl.disabled') }}
            </button>
          </div>
        </div>
      </div>

      <div v-if="activeTab === 'analytics' && connected" class="space-y-4">
        <div class="rounded-xl border border-panel-border bg-panel-card p-5">
          <div class="flex flex-wrap items-center gap-3">
            <label class="text-sm text-gray-400">{{ t('cloudflare_manager.dns.zone_label') }}</label>
            <span class="font-semibold text-white">{{ selectedZone?.name || t('cloudflare_manager.dns.not_selected') }}</span>
            <button
              class="rounded-lg bg-gradient-to-r from-orange-600 to-amber-600 px-4 py-2 text-sm font-medium text-white transition hover:from-orange-700 hover:to-amber-700"
              :disabled="analyticsLoading || !selectedZone?.id"
              @click="loadAnalytics"
            >
              {{ analyticsLoading ? t('cloudflare_manager.analytics.loading') : t('cloudflare_manager.analytics.load') }}
            </button>
          </div>
        </div>

        <div v-if="analyticsError" class="rounded-xl border border-red-500/30 bg-red-500/10 p-4 text-sm text-red-300">
          {{ analyticsError }}
        </div>

        <div class="grid grid-cols-1 gap-4 md:grid-cols-3">
          <div class="rounded-xl border border-panel-border bg-panel-card p-5">
            <p class="text-xs uppercase tracking-wide text-gray-400">{{ t('cloudflare_manager.analytics.requests') }}</p>
            <p class="mt-2 text-2xl font-semibold text-white">{{ analyticsSummary.requests }}</p>
          </div>
          <div class="rounded-xl border border-panel-border bg-panel-card p-5">
            <p class="text-xs uppercase tracking-wide text-gray-400">{{ t('cloudflare_manager.analytics.page_views') }}</p>
            <p class="mt-2 text-2xl font-semibold text-white">{{ analyticsSummary.pageViews }}</p>
          </div>
          <div class="rounded-xl border border-panel-border bg-panel-card p-5">
            <p class="text-xs uppercase tracking-wide text-gray-400">{{ t('cloudflare_manager.analytics.bandwidth') }}</p>
            <p class="mt-2 text-2xl font-semibold text-white">{{ analyticsSummary.bandwidth }}</p>
          </div>
        </div>

        <div class="rounded-xl border border-panel-border bg-panel-card p-5">
          <h3 class="mb-3 text-sm font-semibold text-white">{{ t('cloudflare_manager.analytics.raw_response') }}</h3>
          <pre class="max-h-[360px] overflow-auto rounded-lg bg-panel-darker p-3 text-xs text-gray-300">{{ formattedAnalytics }}</pre>
        </div>
      </div>

    <div v-if="showAddDns" class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm" @click.self="showAddDns = false">
      <div class="w-full max-w-lg rounded-2xl border border-panel-border bg-panel-card p-6 shadow-2xl">
        <h3 class="mb-5 text-xl font-bold text-white">{{ t('cloudflare_manager.dns.modal_title') }}</h3>
        <div class="space-y-4">
          <div>
            <label class="mb-1 block text-sm text-gray-400">{{ t('cloudflare_manager.dns.type') }}</label>
            <select v-model="newDns.type" class="aura-input">
              <option v-for="type in ['A', 'AAAA', 'CNAME', 'MX', 'TXT', 'NS', 'SRV', 'CAA']" :key="type" :value="type">{{ type }}</option>
            </select>
          </div>
          <div>
            <label class="mb-1 block text-sm text-gray-400">{{ t('cloudflare_manager.dns.name') }}</label>
            <input v-model="newDns.name" type="text" :placeholder="t('cloudflare_manager.placeholders.dns_name')" class="aura-input" />
          </div>
          <div>
            <label class="mb-1 block text-sm text-gray-400">{{ t('cloudflare_manager.dns.value') }}</label>
            <input v-model="newDns.content" type="text" :placeholder="t('cloudflare_manager.placeholders.dns_value')" class="aura-input" />
          </div>
          <div class="flex items-center gap-4">
            <label class="flex items-center gap-2 text-sm text-gray-300">
              <input v-model="newDns.proxied" type="checkbox" class="accent-orange-500" />
              {{ t('cloudflare_manager.dns.proxied') }}
            </label>
            <select v-model="newDns.ttl" class="rounded-lg border border-panel-border bg-panel-hover px-3 py-2 text-sm text-white focus:outline-none">
              <option :value="1">{{ t('cloudflare_manager.dns.ttl_auto') }}</option>
              <option :value="300">{{ t('cloudflare_manager.dns.ttl_5m') }}</option>
              <option :value="3600">{{ t('cloudflare_manager.dns.ttl_1h') }}</option>
              <option :value="86400">{{ t('cloudflare_manager.dns.ttl_1d') }}</option>
            </select>
          </div>
        </div>
        <div class="mt-6 flex gap-3">
          <button class="flex-1 rounded-lg bg-gradient-to-r from-orange-600 to-amber-600 py-2.5 font-medium text-white transition hover:from-orange-700 hover:to-amber-700" @click="addDnsRecord">
            {{ t('common.add') }}
          </button>
          <button class="rounded-lg bg-panel-hover px-5 py-2.5 text-gray-300 transition hover:bg-gray-600" @click="showAddDns = false">
            {{ t('common.cancel') }}
          </button>
        </div>
      </div>
    </div>

    <div v-if="notification" :class="['fixed bottom-6 right-6 z-50 rounded-xl px-5 py-3 text-sm font-medium text-white shadow-2xl', notification.type === 'success' ? 'bg-green-600' : 'bg-red-600']">
      {{ notification.message }}
    </div>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { Cloud, Trash2 } from 'lucide-vue-next'
import api from '../services/api'

const { t } = useI18n({ useScope: 'global' })

const cfEmail = ref('')
const cfApiKey = ref('')
const connected = ref(false)
const notification = ref(null)
const activeTab = ref('login')
const loading = ref(false)
const cfStorageKey = 'aurapanel.cloudflare.auth.v1'
const serverStatus = ref({
  configured: false,
  auto_sync: false,
  credential_source: 'none',
  email_hint: '',
})
const serverSaving = ref(false)

const tabs = [
  { id: 'login', label: t('cloudflare_manager.tabs.login') },
  { id: 'zones', label: t('cloudflare_manager.tabs.zones') },
  { id: 'dns', label: t('cloudflare_manager.tabs.dns') },
  { id: 'ssl', label: t('cloudflare_manager.tabs.ssl') },
  { id: 'cache', label: t('cloudflare_manager.tabs.cache') },
  { id: 'security', label: t('cloudflare_manager.tabs.security') },
  { id: 'analytics', label: t('cloudflare_manager.tabs.analytics') },
]

const zones = ref([])
const selectedZone = ref(null)
const dnsRecords = ref([])
const showAddDns = ref(false)
const newDns = reactive({ type: 'A', name: '', content: '', proxied: false, ttl: 1 })
const selectedSslMode = ref('full')
const selectedSecurityLevel = ref('medium')
const devMode = ref(false)
const alwaysHttps = ref(true)
const minifyOptions = reactive({ js: true, css: true, html: false })
const purgeUrls = ref('')
const analyticsLoading = ref(false)
const analyticsData = ref(null)
const analyticsError = ref('')

const sslModes = [
  { value: 'off', label: t('cloudflare_manager.ssl.modes.off.label'), desc: t('cloudflare_manager.ssl.modes.off.desc') },
  { value: 'flexible', label: t('cloudflare_manager.ssl.modes.flexible.label'), desc: t('cloudflare_manager.ssl.modes.flexible.desc') },
  { value: 'full', label: t('cloudflare_manager.ssl.modes.full.label'), desc: t('cloudflare_manager.ssl.modes.full.desc') },
  { value: 'strict', label: t('cloudflare_manager.ssl.modes.strict.label'), desc: t('cloudflare_manager.ssl.modes.strict.desc') },
]

const securityLevels = [
  { value: 'off', label: t('cloudflare_manager.security.levels.off') },
  { value: 'low', label: t('cloudflare_manager.security.levels.low') },
  { value: 'medium', label: t('cloudflare_manager.security.levels.medium') },
  { value: 'high', label: t('cloudflare_manager.security.levels.high') },
  { value: 'under_attack', label: t('cloudflare_manager.security.levels.under_attack') },
]

const showNotif = (message, type = 'success') => {
  notification.value = { message, type }
  setTimeout(() => {
    notification.value = null
  }, 3000)
}

const readStoredCfAuth = () => {
  if (typeof window === 'undefined') return null
  try {
    const raw = window.localStorage.getItem(cfStorageKey)
    if (!raw) return null
    const parsed = JSON.parse(raw)
    if (!parsed || typeof parsed !== 'object') return null
    return parsed
  } catch {
    return null
  }
}

const saveStoredCfAuth = () => {
  if (typeof window === 'undefined') return
  const payload = { zone_id: selectedZone.value?.id || '' }
  window.localStorage.setItem(cfStorageKey, JSON.stringify(payload))
}

const clearStoredCfAuth = () => {
  if (typeof window === 'undefined') return
  window.localStorage.removeItem(cfStorageKey)
}

const authPayload = () => {
  const payload = {}
  if (cfApiKey.value) payload.api_key = cfApiKey.value
  if (cfEmail.value) payload.email = cfEmail.value
  return payload
}

const loadServerStatus = async () => {
  try {
    const { data } = await api.get('/cloudflare/status')
    serverStatus.value = {
      configured: Boolean(data?.data?.configured),
      auto_sync: Boolean(data?.data?.auto_sync),
      credential_source: data?.data?.credential_source || 'none',
      email_hint: data?.data?.email_hint || '',
    }
  } catch {
    serverStatus.value = { configured: false, auto_sync: false, credential_source: 'none', email_hint: '' }
  }
}

const connectCf = async (opts = {}) => {
  const { silent = false, auto = false, preferredZoneId = '' } = opts
  const hasManualCreds = Boolean(cfEmail.value && cfApiKey.value)
  if (!hasManualCreds && !serverStatus.value.configured) {
    if (auto) return
    showNotif(t('cloudflare_manager.messages.credentials_required'), 'error')
    return
  }
  loading.value = true
  try {
    const { data } = await api.post('/cloudflare/zones', authPayload())
    zones.value = data.data || []
    connected.value = true
    const preferred = preferredZoneId || readStoredCfAuth()?.zone_id || ''
    const matchedZone = zones.value.find(zone => zone.id === preferred) || zones.value[0] || null
    if (matchedZone) {
      await selectZone(matchedZone)
    } else {
      selectedZone.value = null
    }
    if (!silent) {
      showNotif(t('cloudflare_manager.messages.connected', { count: zones.value.length }))
    }
  } catch (err) {
    connected.value = false
    zones.value = []
    selectedZone.value = null
    dnsRecords.value = []
    if (auto) {
      clearStoredCfAuth()
      return
    }
    showNotif(err.response?.data?.error || t('cloudflare_manager.messages.connect_failed'), 'error')
    activeTab.value = 'login'
  } finally {
    loading.value = false
  }
}

const saveServerAuth = async () => {
  if (!cfEmail.value || !cfApiKey.value) {
    showNotif(t('cloudflare_manager.messages.credentials_required'), 'error')
    return
  }
  serverSaving.value = true
  try {
    const { data } = await api.post('/cloudflare/server-auth', {
      email: cfEmail.value,
      api_key: cfApiKey.value,
      auto_sync: true,
    })
    serverStatus.value = {
      configured: Boolean(data?.data?.configured),
      auto_sync: Boolean(data?.data?.auto_sync),
      credential_source: data?.data?.credential_source || 'none',
      email_hint: data?.data?.email_hint || '',
    }
    showNotif(t('cloudflare_manager.messages.server_auth_saved'))
    if (!connected.value) {
      await connectCf({ silent: true })
    }
  } catch (err) {
    showNotif(err.response?.data?.message || t('cloudflare_manager.messages.server_auth_failed'), 'error')
  } finally {
    serverSaving.value = false
  }
}

const loadZones = connectCf

const loadDnsRecords = async zoneId => {
  dnsRecords.value = []
  const { data } = await api.post('/cloudflare/dns/list', { ...authPayload(), zone_id: zoneId })
  dnsRecords.value = data.data || []
}

const applyZoneSettings = config => {
  if (!config || typeof config !== 'object') return
  if (config.ssl_mode) selectedSslMode.value = config.ssl_mode
  if (config.security_level) selectedSecurityLevel.value = config.security_level
  devMode.value = Boolean(config.dev_mode)
  alwaysHttps.value = Boolean(config.always_https)
  minifyOptions.js = Boolean(config.minify_js)
  minifyOptions.css = Boolean(config.minify_css)
  minifyOptions.html = Boolean(config.minify_html)
}

const loadZoneSettings = async zoneId => {
  const { data } = await api.post('/cloudflare/settings', { ...authPayload(), zone_id: zoneId })
  applyZoneSettings(data?.data || {})
}

const selectZone = async (zone, opts = {}) => {
  const { goToDns = false } = opts
  if (!zone?.id) return
  selectedZone.value = zone
  saveStoredCfAuth()
  try {
    await loadZoneSettings(zone.id)
    if (goToDns) {
      activeTab.value = 'dns'
    }
    if (goToDns || activeTab.value === 'dns') {
      await loadDnsRecords(zone.id)
    }
  } catch (err) {
    showNotif(err.response?.data?.error || err.response?.data?.message || t('cloudflare_manager.messages.dns_failed'), 'error')
  }
}

const onScopeZoneChange = async zoneID => {
  const zone = zones.value.find(item => item.id === zoneID)
  if (!zone) return
  await selectZone(zone, { goToDns: activeTab.value === 'dns' })
}

const requireSelectedZone = () => {
  const zoneID = selectedZone.value?.id || ''
  if (!zoneID) {
    showNotif(t('cloudflare_manager.messages.zone_required'), 'error')
    return ''
  }
  return zoneID
}

const getNestedNumber = (obj, paths) => {
  for (const path of paths) {
    const value = path.reduce((acc, key) => (acc && acc[key] !== undefined ? acc[key] : undefined), obj)
    if (typeof value === 'number' && Number.isFinite(value)) {
      return value
    }
  }
  return 0
}

const formatNumber = value => new Intl.NumberFormat().format(Number(value || 0))

const formatBytes = value => {
  const num = Number(value || 0)
  if (!Number.isFinite(num) || num <= 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB', 'PB']
  let size = num
  let idx = 0
  while (size >= 1024 && idx < units.length - 1) {
    size /= 1024
    idx += 1
  }
  const rounded = idx === 0 ? size.toFixed(0) : size.toFixed(2)
  return `${rounded} ${units[idx]}`
}

const analyticsSummary = computed(() => {
  const raw = analyticsData.value || {}
  const requests = getNestedNumber(raw, [
    ['result', 'totals', 'requests', 'all'],
    ['result', 'totals', 'requests'],
    ['result', 'totals', 'visits'],
  ])
  const pageViews = getNestedNumber(raw, [
    ['result', 'totals', 'pageviews', 'all'],
    ['result', 'totals', 'pageviews'],
    ['result', 'totals', 'uniques', 'all'],
  ])
  const bandwidthBytes = getNestedNumber(raw, [
    ['result', 'totals', 'bandwidth', 'all'],
    ['result', 'totals', 'bandwidth'],
  ])
  return {
    requests: formatNumber(requests),
    pageViews: formatNumber(pageViews),
    bandwidth: formatBytes(bandwidthBytes),
  }
})

const formattedAnalytics = computed(() => JSON.stringify(analyticsData.value || {}, null, 2))

const loadAnalytics = async () => {
  const zoneID = requireSelectedZone()
  if (!zoneID) {
    return
  }
  analyticsLoading.value = true
  analyticsError.value = ''
  try {
    const { data } = await api.post('/cloudflare/analytics', {
      ...authPayload(),
      zone_id: zoneID,
    })
    analyticsData.value = data.data || {}
    showNotif(t('cloudflare_manager.messages.analytics_loaded'))
  } catch (err) {
    analyticsError.value = err.response?.data?.message || err.response?.data?.error || err.message || t('cloudflare_manager.messages.analytics_failed')
    showNotif(analyticsError.value, 'error')
  } finally {
    analyticsLoading.value = false
  }
}

const addDnsRecord = async () => {
  const zoneID = requireSelectedZone()
  if (!zoneID) return
  try {
    await api.post('/cloudflare/dns/create', { ...authPayload(), zone_id: zoneID, ...newDns })
    showNotif(t('cloudflare_manager.messages.dns_added', { type: newDns.type }))
    showAddDns.value = false
    await loadDnsRecords(zoneID)
  } catch (err) {
    showNotif(err.response?.data?.error || t('cloudflare_manager.messages.dns_add_failed'), 'error')
  }
}

const deleteDnsRecord = async recordId => {
  const zoneID = requireSelectedZone()
  if (!zoneID) return
  if (!window.confirm(t('cloudflare_manager.dns.delete_confirm'))) return
  try {
    await api.post('/cloudflare/dns/delete', { ...authPayload(), zone_id: zoneID, record_id: recordId })
    showNotif(t('cloudflare_manager.messages.dns_deleted'))
    await loadDnsRecords(zoneID)
  } catch (err) {
    showNotif(err.response?.data?.error || t('cloudflare_manager.messages.dns_delete_failed'), 'error')
  }
}

const setSslMode = async mode => {
  const zoneID = requireSelectedZone()
  if (!zoneID) return
  selectedSslMode.value = mode
  try {
    await api.post('/cloudflare/ssl', { ...authPayload(), zone_id: zoneID, mode })
    showNotif(t('cloudflare_manager.messages.ssl_updated', { mode }))
  } catch (err) {
    showNotif(err.response?.data?.error || t('cloudflare_manager.messages.ssl_update_failed'), 'error')
  }
}

const purgeAllCache = async () => {
  const zoneID = requireSelectedZone()
  if (!zoneID) return
  try {
    await api.post('/cloudflare/cache/purge', { ...authPayload(), zone_id: zoneID, purge_everything: true })
    showNotif(t('cloudflare_manager.messages.cache_purged'))
  } catch (err) {
    showNotif(err.response?.data?.error || t('cloudflare_manager.messages.cache_failed'), 'error')
  }
}

const purgeSpecificCache = async () => {
  const zoneID = requireSelectedZone()
  if (!zoneID) return
  const files = purgeUrls.value.split('\n').filter(item => item.trim())
  if (!files.length) {
    showNotif(t('cloudflare_manager.messages.cache_url_required'), 'error')
    return
  }
  try {
    await api.post('/cloudflare/cache/purge', { ...authPayload(), zone_id: zoneID, files })
    showNotif(t('cloudflare_manager.messages.cache_urls_purged', { count: files.length }))
    purgeUrls.value = ''
  } catch (err) {
    showNotif(err.response?.data?.error || t('cloudflare_manager.messages.cache_failed'), 'error')
  }
}

const setSecurityLevel = async level => {
  const zoneID = requireSelectedZone()
  if (!zoneID) return
  selectedSecurityLevel.value = level
  try {
    await api.post('/cloudflare/security', { ...authPayload(), zone_id: zoneID, level })
    showNotif(t('cloudflare_manager.messages.security_updated'))
  } catch (err) {
    showNotif(err.response?.data?.error || t('cloudflare_manager.messages.security_failed'), 'error')
  }
}

const toggleDevMode = async () => {
  const zoneID = requireSelectedZone()
  if (!zoneID) return
  const newValue = !devMode.value
  try {
    await api.post('/cloudflare/devmode', { ...authPayload(), zone_id: zoneID, enabled: newValue })
    devMode.value = newValue
    showNotif(t('cloudflare_manager.messages.dev_mode_updated', { state: devMode.value ? t('cloudflare_manager.ssl.enabled') : t('cloudflare_manager.ssl.disabled') }))
  } catch (err) {
    showNotif(err.response?.data?.error || t('cloudflare_manager.messages.dev_mode_failed'), 'error')
  }
}

const toggleAlwaysHttps = async () => {
  const zoneID = requireSelectedZone()
  if (!zoneID) return
  const newValue = !alwaysHttps.value
  try {
    await api.post('/cloudflare/ssl/always-https', { ...authPayload(), zone_id: zoneID, enabled: newValue })
    alwaysHttps.value = newValue
    showNotif(t('cloudflare_manager.messages.always_https_updated', { state: alwaysHttps.value ? t('cloudflare_manager.ssl.enabled') : t('cloudflare_manager.ssl.disabled') }))
  } catch (err) {
    showNotif(err.response?.data?.error || t('cloudflare_manager.messages.ssl_update_failed'), 'error')
  }
}

const saveMinify = async () => {
  const zoneID = requireSelectedZone()
  if (!zoneID) return
  try {
    await api.post('/cloudflare/ssl/minify', {
      ...authPayload(),
      zone_id: zoneID,
      js: !!minifyOptions.js,
      css: !!minifyOptions.css,
      html: !!minifyOptions.html,
    })
    showNotif(t('cloudflare_manager.messages.minify_saved'))
  } catch (err) {
    showNotif(err.response?.data?.error || t('cloudflare_manager.messages.ssl_update_failed'), 'error')
  }
}

const dnsTypeBadge = type => {
  const map = {
    A: 'bg-blue-500/15 text-blue-400',
    AAAA: 'bg-indigo-500/15 text-indigo-400',
    CNAME: 'bg-green-500/15 text-green-400',
    MX: 'bg-purple-500/15 text-purple-400',
    TXT: 'bg-yellow-500/15 text-yellow-400',
    NS: 'bg-pink-500/15 text-pink-400',
    SRV: 'bg-cyan-500/15 text-cyan-400',
  }
  return map[type] || 'bg-gray-500/15 text-gray-400'
}

onMounted(async () => {
  await loadServerStatus()
  const saved = readStoredCfAuth()
  if (serverStatus.value.configured) {
    await connectCf({ silent: true, auto: true, preferredZoneId: saved?.zone_id || '' })
  }
})

watch(activeTab, async newTab => {
  if (newTab === 'dns' && selectedZone.value?.id) {
    try {
      await loadDnsRecords(selectedZone.value.id)
    } catch (err) {
      showNotif(err.response?.data?.error || t('cloudflare_manager.messages.dns_failed'), 'error')
    }
  }
})
</script>

<style scoped>
.cloudflare-view .cf-connect-title,
.cloudflare-view .cf-field-label {
  color: rgb(var(--brand-500) / 1);
}

.cloudflare-view :is(input[type='text'], input[type='email'], input[type='password'], input[type='number'], input[type='search'], input[type='url'], textarea, select) {
  background-color: rgb(var(--panel-dark) / 1) !important;
  color: rgb(var(--tone-100) / 1) !important;
  border-color: rgb(var(--panel-border) / 1) !important;
  caret-color: rgb(var(--brand-500) / 1);
}

.cloudflare-view :is(input[type='text'], input[type='email'], input[type='password'], input[type='number'], input[type='search'], input[type='url'], textarea)::placeholder {
  color: rgb(var(--tone-500) / 1) !important;
}

.cloudflare-view select option {
  background-color: rgb(var(--panel-card) / 1);
  color: rgb(var(--tone-100) / 1);
}
</style>
