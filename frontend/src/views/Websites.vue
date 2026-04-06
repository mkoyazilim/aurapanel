<template>
  <div class="space-y-6">
    <div class="flex items-center justify-between gap-4">
      <div>
        <h1 class="text-2xl font-bold text-white">{{ t('websites.title') }}</h1>
        <p class="text-gray-400 mt-1">{{ t('websites.subtitle') }}</p>
      </div>
      <button class="btn-primary" @click="showAddSiteModal = true">
        <Plus class="w-5 h-5" />
        {{ t('websites.add_new') }}
      </button>
    </div>

    <div class="border-b border-panel-border">
      <nav class="flex flex-wrap gap-5">
        <button
          class="pb-3 text-sm font-medium transition"
          :class="activeTab === 'sites' ? 'text-brand-400 border-b-2 border-brand-400' : 'text-gray-400 hover:text-white'"
          @click="activeTab = 'sites'"
        >
          {{ t('websites.tab_sites') }}
        </button>
        <button
          class="pb-3 text-sm font-medium transition"
          :class="activeTab === 'subdomains' ? 'text-brand-400 border-b-2 border-brand-400' : 'text-gray-400 hover:text-white'"
          @click="activeTab = 'subdomains'"
        >
          {{ t('websites.tab_subdomains') }}
        </button>
        <button
          class="pb-3 text-sm font-medium transition"
          :class="activeTab === 'dbLinks' ? 'text-brand-400 border-b-2 border-brand-400' : 'text-gray-400 hover:text-white'"
          @click="activeTab = 'dbLinks'"
        >
          {{ t('websites.tab_db_links') }}
        </button>
        <button
          class="pb-3 text-sm font-medium transition"
          :class="activeTab === 'advanced' ? 'text-brand-400 border-b-2 border-brand-400' : 'text-gray-400 hover:text-white'"
          @click="activeTab = 'advanced'"
        >
          {{ t('websites.tab_advanced') }}
        </button>
      </nav>
    </div>

    <div v-if="error" class="aura-card border-red-500/30 bg-red-500/5 text-red-400">
      {{ error }}
    </div>

    <div v-if="activeTab === 'sites'" class="space-y-4">
      <div class="flex flex-wrap gap-4 items-center bg-panel-card p-4 rounded-xl border border-panel-border">
        <div class="relative flex-1 min-w-[220px]">
          <Search class="w-5 h-5 text-gray-400 absolute left-3 top-1/2 -translate-y-1/2" />
          <input v-model="search" type="text" class="aura-input pl-10" :placeholder="t('common.search')" />
        </div>
        <select v-model="phpFilter" class="aura-input w-auto min-w-[150px]">
          <option value="">{{ t('websites.all_php') }}</option>
          <option v-for="v in phpVersions" :key="v" :value="v">PHP {{ v }}</option>
        </select>
        <select v-model.number="sitesPagination.per_page" class="aura-input w-auto min-w-[120px]">
          <option :value="10">{{ t('websites.per_page', { count: 10 }) }}</option>
          <option :value="20">{{ t('websites.per_page', { count: 20 }) }}</option>
          <option :value="50">{{ t('websites.per_page', { count: 50 }) }}</option>
        </select>
        <button class="btn-secondary" @click="applySiteFilters">{{ t('websites.filter') }}</button>
        <button class="btn-secondary" @click="refreshAll">{{ t('common.refresh') }}</button>
      </div>

      <div class="aura-card border border-panel-border/80 bg-panel-card space-y-3">
        <div class="flex items-center justify-between gap-3">
          <div>
            <h2 class="text-base font-semibold text-white">{{ t('websites.discovered_title') }}</h2>
            <p class="text-xs text-gray-400">{{ t('websites.discovered_subtitle') }}</p>
          </div>
          <button class="btn-secondary px-3 py-1.5 text-sm" :disabled="discoveryLoading" @click="loadDiscoveredSites">
            <Loader2 v-if="discoveryLoading" class="w-4 h-4 animate-spin mr-1 inline" />
            {{ t('websites.discovered_refresh') }}
          </button>
        </div>
        <div v-if="discoveredSites.length === 0" class="rounded-lg border border-dashed border-panel-border px-4 py-3 text-sm text-gray-500">
          {{ t('websites.discovered_empty') }}
        </div>
        <div v-else class="space-y-2">
          <div
            v-for="item in discoveredSites"
            :key="`discover-${item.domain}`"
            class="rounded-lg border border-panel-border/60 bg-panel-dark/60 px-3 py-3 flex flex-col lg:flex-row lg:items-center gap-3 justify-between"
          >
            <div class="space-y-1">
              <p class="text-white font-semibold">{{ item.domain }}</p>
              <p class="text-xs text-gray-400">{{ t('websites.path_label') }}: {{ item.path }}</p>
              <p class="text-xs text-gray-400">{{ t('websites.docroot_label') }}: {{ item.docroot }}</p>
              <div class="flex items-center gap-2 text-[11px]">
                <span class="px-2 py-0.5 rounded border border-panel-border text-gray-300">{{ t('websites.owner_badge', { owner: item._owner || item.owner || item.user || '-' }) }}</span>
                <span :class="item.has_docroot ? 'px-2 py-0.5 rounded border border-emerald-500/30 text-emerald-300' : 'px-2 py-0.5 rounded border border-yellow-500/30 text-yellow-300'">
                  {{ item.has_docroot ? t('websites.docroot_yes') : t('websites.docroot_no') }}
                </span>
                <span :class="item.has_index ? 'px-2 py-0.5 rounded border border-emerald-500/30 text-emerald-300' : 'px-2 py-0.5 rounded border border-yellow-500/30 text-yellow-300'">
                  {{ item.has_index ? t('websites.index_yes') : t('websites.index_no') }}
                </span>
              </div>
            </div>
            <div class="flex items-center gap-2">
              <select v-model="item._php_version" class="aura-input text-sm py-1.5 min-w-[130px]">
                <option v-for="v in phpVersions" :key="`discover-php-${item.domain}-${v}`" :value="v">PHP {{ v }}</option>
              </select>
              <input v-model="item._owner" type="text" class="aura-input text-sm py-1.5 min-w-[120px]" :placeholder="t('websites.owner_placeholder')" />
              <button class="btn-primary px-3 py-1.5 text-sm" :disabled="importingDomain === item.domain" @click="importDiscoveredSite(item)">
                <Loader2 v-if="importingDomain === item.domain" class="w-4 h-4 animate-spin mr-1 inline" />
                {{ t('websites.import_action') }}
              </button>
            </div>
          </div>
        </div>
      </div>

      <div v-if="loading" class="aura-card text-center py-12">
        <Loader2 class="w-8 h-8 text-brand-500 animate-spin mx-auto mb-3" />
        <p class="text-gray-400">{{ t('common.loading') }}</p>
      </div>

      <div v-else-if="filteredSites.length === 0" class="aura-card text-center py-12">
        <Globe class="w-14 h-14 text-gray-600 mx-auto mb-3" />
        <p class="text-gray-300">{{ t('websites.no_sites') }}</p>
      </div>

      <div v-else class="space-y-4">
        <div
          v-for="site in filteredSites"
          :key="site.domain"
          class="aura-card flex flex-col sm:flex-row gap-6 justify-between items-start sm:items-center"
        >
          <div class="flex items-center gap-4">
            <div class="w-12 h-12 rounded-lg bg-panel-dark flex items-center justify-center border border-panel-border">
              <Globe class="w-6 h-6 text-brand-500" />
            </div>
            <div>
              <h3 class="text-lg font-bold text-white flex items-center gap-2">
                {{ site.domain }}
                <span
                  :class="isSuspended(site)
                    ? 'px-2 py-0.5 rounded text-xs font-semibold bg-yellow-500/10 text-yellow-400 border border-yellow-500/20'
                    : 'px-2 py-0.5 rounded text-xs font-semibold bg-brand-500/10 text-brand-400 border border-brand-500/20'"
                >{{ isSuspended(site) ? t('websites.status_suspended') : t('websites.status_active') }}</span>
                <span
                  v-if="site.ssl"
                  class="px-2 py-0.5 rounded text-xs font-semibold bg-brand-500/10 text-brand-400 border border-brand-500/20"
                >{{ t('websites.ssl_active') }}</span>
                <span
                  v-else
                  class="px-2 py-0.5 rounded text-xs font-semibold bg-yellow-500/10 text-yellow-400 border border-yellow-500/20"
                >{{ t('websites.ssl_none') }}</span>
              </h3>
              <div class="text-sm text-gray-400 mt-1 flex flex-wrap items-center gap-4">
                <span class="flex items-center gap-1"><HardDrive class="w-4 h-4" /> {{ site.disk_usage }} / {{ site.quota }}</span>
                <span class="flex items-center gap-1"><Cpu class="w-4 h-4" /> PHP {{ site.php }}</span>
                <span class="flex items-center gap-1"><User class="w-4 h-4" /> {{ site.user || site.owner || '-' }}</span>
                <span class="flex items-center gap-1">{{ t('websites.package_label') }}: {{ site.package || 'default' }}</span>
                <span class="flex items-center gap-1">{{ t('websites.email_label') }}: {{ site.email || `webmaster@${site.domain}` }}</span>
              </div>
            </div>
          </div>

          <div class="flex items-center gap-2 w-full sm:w-auto">
            <button class="btn-secondary px-3 py-1.5 text-sm flex-1 sm:flex-none" @click="issueSSL(site)">
              <ShieldCheck class="w-4 h-4 mr-1 inline" />{{ t('websites.issue_ssl') }}
            </button>
            <button class="btn-secondary px-3 py-1.5 text-sm flex-1 sm:flex-none" @click="toggleSuspend(site)">
              <span class="inline-flex items-center gap-1">
                <PauseCircle v-if="!isSuspended(site)" class="w-4 h-4" />
                <PlayCircle v-else class="w-4 h-4" />
                {{ isSuspended(site) ? t('websites.unsuspend') : t('websites.suspend') }}
              </span>
            </button>
            <button class="btn-secondary px-3 py-1.5 text-sm flex-1 sm:flex-none" @click="openEditSiteModal(site)">
              <Pencil class="w-4 h-4 mr-1 inline" />{{ t('common.edit') }}
            </button>
            <button class="btn-secondary px-3 py-1.5 text-sm flex-1 sm:flex-none" @click="openManagePage(site)">
              {{ t('websites.manage') }}
            </button>
            <button class="btn-secondary px-3 py-1.5 text-sm flex-1 sm:flex-none" @click="openSiteLogs(site, 'access')">
              {{ t('websites.logs') }}
            </button>
            <button class="btn-danger px-2 py-1.5" :title="t('websites.delete_site')" @click="deleteSite(site)">
              <Trash2 class="w-4 h-4" />
            </button>
          </div>
        </div>
      </div>

      <div class="flex items-center justify-between text-sm text-gray-400 bg-panel-card p-3 rounded-xl border border-panel-border">
        <span>{{ t('websites.total_sites', { count: sitesPagination.total }) }}</span>
        <div class="flex items-center gap-2">
          <button class="btn-secondary px-3 py-1" :disabled="sitesPagination.page <= 1" @click="changeSitesPage(-1)">{{ t('websites.previous') }}</button>
          <span>{{ t('websites.page_status', { page: sitesPagination.page, total: Math.max(1, sitesPagination.total_pages || 1) }) }}</span>
          <button class="btn-secondary px-3 py-1" :disabled="sitesPagination.page >= Math.max(1, sitesPagination.total_pages || 1)" @click="changeSitesPage(1)">{{ t('websites.next') }}</button>
        </div>
      </div>
    </div>

    <div v-if="activeTab === 'subdomains'" class="space-y-4">
      <div class="flex items-center justify-between bg-panel-card p-4 rounded-xl border border-panel-border">
        <div>
          <h2 class="text-lg font-semibold text-white">{{ t('websites.subdomain_title') }}</h2>
          <p class="text-sm text-gray-400">{{ t('websites.subdomain_subtitle') }}</p>
        </div>
        <div class="flex items-center gap-2">
          <button class="btn-secondary" @click="loadSubdomains">{{ t('common.refresh') }}</button>
          <button class="btn-primary" @click="showSubdomainModal = true">{{ t('websites.subdomain_add') }}</button>
        </div>
      </div>

      <div class="aura-card overflow-x-auto">
        <table class="w-full text-sm">
          <thead>
            <tr class="border-b border-panel-border text-gray-400">
              <th class="text-left py-3 px-2">{{ t('websites.fqdn') }}</th>
              <th class="text-left py-3 px-2">{{ t('websites.parent_domain') }}</th>
              <th class="text-left py-3 px-2">{{ t('websites.php_short') }}</th>
              <th class="text-left py-3 px-2">{{ t('websites.ssl_short') }}</th>
              <th class="text-left py-3 px-2">{{ t('websites.created_at') }}</th>
              <th class="text-right py-3 px-2">{{ t('common.actions') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="s in subdomains" :key="s.fqdn" class="border-b border-panel-border/40 hover:bg-white/[0.02]">
              <td class="py-3 px-2 text-white font-medium">{{ s.fqdn }}</td>
              <td class="py-3 px-2 text-gray-300">{{ s.parent_domain }}</td>
              <td class="py-3 px-2">
                <select v-model="s.php_version" class="aura-input text-xs py-1 px-2" @change="updateSubdomainPhp(s)">
                  <option v-for="v in phpVersions" :key="v" :value="v">{{ v }}</option>
                </select>
              </td>
              <td class="py-3 px-2">
                <span :class="s.ssl_enabled ? 'text-brand-400' : 'text-yellow-400'">{{ s.ssl_enabled ? t('common.active') : t('websites.none_short') }}</span>
              </td>
              <td class="py-3 px-2 text-gray-400">{{ formatUnix(s.created_at) }}</td>
              <td class="py-3 px-2 text-right">
                <div class="flex justify-end gap-2">
                  <button class="btn-secondary px-2 py-1" @click="convertSubdomain(s)">
                    {{ t('websites.subdomain_convert') }}
                  </button>
                  <button class="btn-danger px-2 py-1" @click="deleteSubdomain(s.fqdn)">
                    <Trash2 class="w-4 h-4" />
                  </button>
                </div>
              </td>
            </tr>
            <tr v-if="subdomains.length === 0">
              <td colspan="6" class="text-center py-10 text-gray-500">{{ t('websites.subdomain_empty') }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <div v-if="activeTab === 'dbLinks'" class="space-y-4">
      <div class="flex items-center justify-between bg-panel-card p-4 rounded-xl border border-panel-border">
        <div>
          <h2 class="text-lg font-semibold text-white">{{ t('websites.db_links_title') }}</h2>
          <p class="text-sm text-gray-400">{{ t('websites.db_links_subtitle') }}</p>
        </div>
        <div class="flex items-center gap-2">
          <button class="btn-secondary" @click="loadDbLinks">{{ t('common.refresh') }}</button>
          <button class="btn-primary" @click="showDbLinkModal = true">{{ t('websites.db_link_add') }}</button>
        </div>
      </div>

      <div class="aura-card overflow-x-auto">
        <table class="w-full text-sm">
          <thead>
            <tr class="border-b border-panel-border text-gray-400">
              <th class="text-left py-3 px-2">{{ t('websites.domain') }}</th>
              <th class="text-left py-3 px-2">{{ t('websites.engine') }}</th>
              <th class="text-left py-3 px-2">{{ t('websites.db_name') }}</th>
              <th class="text-left py-3 px-2">{{ t('websites.db_user') }}</th>
              <th class="text-left py-3 px-2">{{ t('websites.db_host') }}</th>
              <th class="text-left py-3 px-2">{{ t('websites.linked_at') }}</th>
              <th class="text-right py-3 px-2">{{ t('common.actions') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="l in dbLinks" :key="`${l.domain}-${l.engine}-${l.db_name}-${l.db_user}-${l.db_host || 'localhost'}`" class="border-b border-panel-border/40 hover:bg-white/[0.02]">
              <td class="py-3 px-2 text-white font-medium">{{ l.domain }}</td>
              <td class="py-3 px-2">
                <span :class="l.engine === 'mariadb' ? 'text-orange-400' : 'text-blue-400'">{{ l.engine }}</span>
              </td>
              <td class="py-3 px-2 text-gray-300">{{ l.db_name }}</td>
              <td class="py-3 px-2 text-gray-300">{{ l.db_user }}</td>
              <td class="py-3 px-2 text-gray-300">{{ l.db_host || 'localhost' }}</td>
              <td class="py-3 px-2 text-gray-400">{{ formatUnix(l.linked_at) }}</td>
              <td class="py-3 px-2 text-right">
                <div class="flex justify-end gap-2">
                  <button class="btn-danger px-2 py-1" @click="detachDbLink(l)">
                    <Trash2 class="w-4 h-4" />
                  </button>
                </div>
              </td>
            </tr>
            <tr v-if="dbLinks.length === 0">
              <td colspan="7" class="text-center py-10 text-gray-500">{{ t('websites.db_links_empty') }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <div v-if="activeTab === 'advanced'" class="space-y-4">
      <div class="bg-panel-card p-4 rounded-xl border border-panel-border space-y-3">
        <div class="flex flex-wrap items-center gap-3">
          <label class="text-sm text-gray-400">{{ t('websites.domain') }}</label>
          <select v-model="advancedDomain" class="aura-input w-auto min-w-[220px]" @change="refreshAdvanced">
            <option disabled value="">{{ t('websites.select_website') }}</option>
            <option v-for="d in parentDomains" :key="d" :value="d">{{ d }}</option>
          </select>
          <button class="btn-secondary" @click="refreshAdvanced">{{ t('common.refresh') }}</button>
        </div>
        <p class="text-xs text-gray-500">{{ t('websites.advanced_hint') }}</p>
      </div>

      <div v-if="!advancedDomain" class="aura-card text-gray-400 text-sm">
        {{ t('websites.advanced_pick_website') }}
      </div>

      <div v-else class="grid grid-cols-1 lg:grid-cols-2 gap-4">
        <div class="aura-card space-y-3">
          <h3 class="text-white font-semibold">{{ t('websites.domain_alias') }}</h3>
          <div class="flex gap-2">
            <input v-model="aliasForm.alias" type="text" class="aura-input flex-1" :placeholder="t('websites.placeholders.alias')" />
            <button class="btn-primary" @click="addAlias">{{ t('common.add') }}</button>
          </div>
          <div class="space-y-2 max-h-48 overflow-auto">
            <div v-for="a in domainAliases" :key="`${a.domain}-${a.alias}`" class="flex items-center justify-between rounded-lg border border-panel-border px-3 py-2 text-sm">
              <span class="text-gray-200">{{ a.alias }}</span>
              <button class="btn-danger px-2 py-1" @click="deleteAlias(a.alias)">
                <Trash2 class="w-4 h-4" />
              </button>
            </div>
            <p v-if="domainAliases.length === 0" class="text-xs text-gray-500">{{ t('websites.alias_empty') }}</p>
          </div>
        </div>

        <div class="aura-card space-y-3">
          <h3 class="text-white font-semibold">{{ t('websites.open_basedir') }}</h3>
          <label class="inline-flex items-center gap-3 text-sm text-gray-300">
            <input v-model="advancedConfig.open_basedir" type="checkbox" class="w-4 h-4 rounded border-panel-border bg-panel-hover" />
            {{ t('websites.open_basedir_enable') }}
          </label>
          <div>
            <button class="btn-primary" @click="saveOpenBasedir">{{ t('common.save') }}</button>
          </div>
        </div>

        <div class="aura-card lg:col-span-2 space-y-3">
          <h3 class="text-white font-semibold">{{ t('websites.rewrite_rules') }}</h3>
          <textarea v-model="advancedConfig.rewrite_rules" rows="8" class="aura-input w-full font-mono text-xs" :placeholder="t('websites.placeholders.rewrite_rules')"></textarea>
          <div>
            <button class="btn-primary" @click="saveRewrite">{{ t('websites.save_rewrite') }}</button>
          </div>
        </div>

        <div class="aura-card lg:col-span-2 space-y-3">
          <h3 class="text-white font-semibold">{{ t('websites.vhost_config_editor') }}</h3>
          <textarea v-model="advancedConfig.vhost_config" rows="12" class="aura-input w-full font-mono text-xs" :placeholder="t('websites.placeholders.vhost_config')"></textarea>
          <div>
            <button class="btn-primary" @click="saveVhostConfig">{{ t('websites.save_vhost_config') }}</button>
          </div>
        </div>

        <div class="aura-card lg:col-span-2 space-y-3">
          <h3 class="text-white font-semibold">{{ t('websites.custom_ssl') }}</h3>
          <textarea v-model="customSslForm.cert_pem" rows="8" class="aura-input w-full font-mono text-xs" :placeholder="t('websites.placeholders.cert_pem')"></textarea>
          <textarea v-model="customSslForm.key_pem" rows="8" class="aura-input w-full font-mono text-xs" :placeholder="t('websites.placeholders.key_pem')"></textarea>
          <div>
            <button class="btn-primary" @click="saveCustomSsl">{{ t('websites.save_custom_ssl') }}</button>
          </div>
        </div>
      </div>
    </div>

    <Teleport to="body">
      <div v-if="showAddSiteModal" class="fixed inset-0 bg-black/60 flex items-center justify-center z-50 p-4">
        <div class="bg-panel-card border border-panel-border rounded-2xl p-8 w-full max-w-md shadow-2xl">
          <h2 class="text-xl font-bold text-white mb-6">{{ t('websites.add_modal_title') }}</h2>
          <div class="space-y-4">
            <div>
              <label class="block text-sm text-gray-400 mb-1">{{ t('websites.domain') }}</label>
              <input v-model="siteForm.domain" type="text" class="aura-input w-full" :placeholder="t('websites.placeholders.domain')" />
            </div>
            <div>
              <label class="block text-sm text-gray-400 mb-1">{{ t('websites.owner') }}</label>
              <select v-model="siteForm.user" class="aura-input w-full">
                <option v-for="u in owners" :key="u" :value="u">{{ u }}</option>
              </select>
            </div>
            <div>
              <label class="block text-sm text-gray-400 mb-1">{{ t('websites.php_version') }}</label>
              <select v-model="siteForm.php_version" class="aura-input w-full">
                <option v-for="v in phpVersions" :key="v" :value="v">PHP {{ v }}</option>
              </select>
            </div>
            <div>
              <label class="block text-sm text-gray-400 mb-1">{{ t('websites.package_label') }}</label>
              <select v-model="siteForm.package" class="aura-input w-full">
                <option v-for="pkg in packageOptions" :key="`create-${pkg}`" :value="pkg">{{ pkg }}</option>
              </select>
            </div>
            <div>
              <label class="block text-sm text-gray-400 mb-1">{{ t('websites.admin_email') }}</label>
              <input v-model="siteForm.email" type="email" class="aura-input w-full" :placeholder="t('websites.placeholders.admin_email')" />
            </div>
            <label class="inline-flex items-center gap-2 text-sm text-gray-300">
              <input
                v-model="siteForm.mail_domain"
                type="checkbox"
                class="w-4 h-4 rounded border-panel-border bg-panel-hover"
              :disabled="!platformStatus.mail_domain_available"
              />
              {{ t('websites.mail_domain_open') }}
            </label>
            <p class="text-xs" :class="platformStatus.mail_domain_available ? 'text-emerald-300' : 'text-yellow-300'">
              {{ platformStatus.mail_domain_available
                ? t('websites.mail_stack_active', { stack: (platformStatus.detected_mail_stack || []).join(', ') || t('websites.mail_stack_ready') })
                : t('websites.mail_stack_inactive') }}
            </p>
          </div>
          <div class="flex gap-3 mt-8">
            <button class="btn-secondary flex-1" @click="showAddSiteModal = false">{{ t('common.cancel') }}</button>
            <button class="btn-primary flex-1" :disabled="siteActionLoading" @click="addSite">
              <Loader2 v-if="siteActionLoading" class="w-4 h-4 animate-spin mr-2 inline" />
              {{ t('common.create') }}
            </button>
          </div>
        </div>
      </div>
    </Teleport>

    <Teleport to="body">
      <div v-if="showEditSiteModal" class="fixed inset-0 bg-black/60 flex items-center justify-center z-50 p-4">
        <div class="bg-panel-card border border-panel-border rounded-2xl p-8 w-full max-w-md shadow-2xl">
          <h2 class="text-xl font-bold text-white mb-6">{{ t('websites.edit_modal_title') }}</h2>
          <div class="space-y-4">
            <div>
              <label class="block text-sm text-gray-400 mb-1">{{ t('websites.domain') }}</label>
              <input :value="editSiteForm.domain" type="text" class="aura-input w-full opacity-70" disabled />
            </div>
            <div>
              <label class="block text-sm text-gray-400 mb-1">{{ t('websites.owner') }}</label>
              <input v-model="editSiteForm.owner" type="text" class="aura-input w-full" :placeholder="t('websites.owner_placeholder')" />
            </div>
            <div>
              <label class="block text-sm text-gray-400 mb-1">{{ t('websites.php_version') }}</label>
              <select v-model="editSiteForm.php_version" class="aura-input w-full">
                <option v-for="v in phpVersions" :key="v" :value="v">PHP {{ v }}</option>
              </select>
            </div>
            <div>
              <label class="block text-sm text-gray-400 mb-1">{{ t('websites.package_label') }}</label>
              <select v-model="editSiteForm.package" class="aura-input w-full">
                <option v-for="pkg in packageOptions" :key="`edit-${pkg}`" :value="pkg">{{ pkg }}</option>
              </select>
            </div>
            <div>
              <label class="block text-sm text-gray-400 mb-1">{{ t('websites.admin_email') }}</label>
              <input v-model="editSiteForm.email" type="email" class="aura-input w-full" :placeholder="t('websites.placeholders.webmaster_email')" />
            </div>
          </div>
          <div class="flex gap-3 mt-8">
            <button class="btn-secondary flex-1" @click="showEditSiteModal = false">{{ t('common.cancel') }}</button>
            <button class="btn-primary flex-1" :disabled="editSiteActionLoading" @click="updateSite">
              <Loader2 v-if="editSiteActionLoading" class="w-4 h-4 animate-spin mr-2 inline" />
              {{ t('common.save') }}
            </button>
          </div>
        </div>
      </div>
    </Teleport>

    <Teleport to="body">
      <div v-if="showSiteLogsModal" class="fixed inset-0 bg-black/60 flex items-center justify-center z-50 p-4">
        <div class="bg-panel-card border border-panel-border rounded-2xl p-6 w-full max-w-4xl shadow-2xl">
          <div class="flex items-center justify-between mb-4">
            <h2 class="text-xl font-bold text-white">{{ t('websites.site_logs_title', { domain: siteLogsDomain }) }}</h2>
            <button class="btn-secondary px-3 py-1" @click="showSiteLogsModal = false">{{ t('common.close') }}</button>
          </div>
          <div class="flex items-center gap-2 mb-4">
            <button class="btn-secondary px-3 py-1" :class="siteLogsKind === 'access' ? 'border-brand-500 text-brand-300' : ''" @click="switchSiteLogKind('access')">{{ t('websites.log_kind_access') }}</button>
            <button class="btn-secondary px-3 py-1" :class="siteLogsKind === 'error' ? 'border-brand-500 text-brand-300' : ''" @click="switchSiteLogKind('error')">{{ t('websites.log_kind_error') }}</button>
            <button class="btn-secondary px-3 py-1 ml-auto" @click="loadSiteLogs">{{ t('common.refresh') }}</button>
          </div>
          <div class="rounded-xl border border-panel-border bg-panel-dark p-4 h-[420px] overflow-auto">
            <div v-if="siteLogsLoading" class="text-gray-400 text-sm">{{ t('websites.logs_loading') }}</div>
            <pre v-else class="text-xs text-gray-200 whitespace-pre-wrap">{{ siteLogsLines.join('\n') || t('websites.logs_empty') }}</pre>
          </div>
        </div>
      </div>
    </Teleport>

    <Teleport to="body">
      <div v-if="showSubdomainModal" class="fixed inset-0 bg-black/60 flex items-center justify-center z-50 p-4">
        <div class="bg-panel-card border border-panel-border rounded-2xl p-8 w-full max-w-md shadow-2xl">
          <h2 class="text-xl font-bold text-white mb-6">{{ t('websites.subdomain_add') }}</h2>
          <div class="space-y-4">
            <div>
              <label class="block text-sm text-gray-400 mb-1">{{ t('websites.parent_domain') }}</label>
              <select v-model="subdomainForm.parent_domain" class="aura-input w-full">
                <option disabled value="">{{ t('websites.select_domain') }}</option>
                <option v-for="d in parentDomains" :key="d" :value="d">{{ d }}</option>
              </select>
            </div>
            <div>
              <label class="block text-sm text-gray-400 mb-1">{{ t('websites.subdomain_label') }}</label>
              <input v-model="subdomainForm.subdomain" type="text" class="aura-input w-full" :placeholder="t('websites.placeholders.subdomain')" />
            </div>
            <div>
              <label class="block text-sm text-gray-400 mb-1">{{ t('websites.php_version') }}</label>
              <select v-model="subdomainForm.php_version" class="aura-input w-full">
                <option v-for="v in phpVersions" :key="v" :value="v">PHP {{ v }}</option>
              </select>
            </div>
          </div>
          <div class="flex gap-3 mt-8">
            <button class="btn-secondary flex-1" @click="showSubdomainModal = false">{{ t('common.cancel') }}</button>
            <button class="btn-primary flex-1" :disabled="subdomainActionLoading" @click="createSubdomain">
              <Loader2 v-if="subdomainActionLoading" class="w-4 h-4 animate-spin mr-2 inline" />
              {{ t('common.create') }}
            </button>
          </div>
        </div>
      </div>
    </Teleport>

    <Teleport to="body">
      <div v-if="showDbLinkModal" class="fixed inset-0 bg-black/60 flex items-center justify-center z-50 p-4">
        <div class="bg-panel-card border border-panel-border rounded-2xl p-8 w-full max-w-md shadow-2xl">
          <h2 class="text-xl font-bold text-white mb-6">{{ t('websites.db_link_modal_title') }}</h2>
          <div class="space-y-4">
            <div>
              <label class="block text-sm text-gray-400 mb-1">{{ t('websites.domain') }}</label>
              <select v-model="dbLinkForm.domain" class="aura-input w-full">
                <option disabled value="">{{ t('websites.select_website') }}</option>
                <option v-for="d in parentDomains" :key="d" :value="d">{{ d }}</option>
              </select>
            </div>
            <div>
              <label class="block text-sm text-gray-400 mb-1">{{ t('websites.engine') }}</label>
              <select v-model="dbLinkForm.engine" class="aura-input w-full">
                <option value="mariadb">{{ t('websites.engines.mariadb') }}</option>
                <option value="postgresql">{{ t('websites.engines.postgresql') }}</option>
              </select>
            </div>
            <div>
              <label class="block text-sm text-gray-400 mb-1">{{ t('websites.db_name') }}</label>
              <select v-model="dbLinkForm.db_name" class="aura-input w-full">
                <option disabled value="">{{ t('websites.select_database') }}</option>
                <option v-for="d in currentDbNames" :key="d" :value="d">{{ d }}</option>
              </select>
            </div>
            <div>
              <label class="block text-sm text-gray-400 mb-1">{{ t('websites.db_user') }}</label>
              <select v-model="dbLinkForm.db_user_identity" class="aura-input w-full">
                <option disabled value="">{{ t('websites.select_user') }}</option>
                <option v-for="u in currentDbUserOptions" :key="u.identity" :value="u.identity">{{ u.label }}</option>
              </select>
              <p class="mt-2 text-xs text-gray-500">{{ t('websites.selected_host', { host: dbLinkForm.db_host || 'localhost' }) }}</p>
            </div>
          </div>
          <div class="flex gap-3 mt-8">
            <button class="btn-secondary flex-1" @click="showDbLinkModal = false">{{ t('common.cancel') }}</button>
            <button class="btn-primary flex-1" :disabled="dbLinkActionLoading" @click="attachDbLink">
              <Loader2 v-if="dbLinkActionLoading" class="w-4 h-4 animate-spin mr-2 inline" />
              {{ t('websites.db_link_attach') }}
            </button>
          </div>
        </div>
      </div>
    </Teleport>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { Plus, Search, Globe, HardDrive, Cpu, Trash2, Loader2, ShieldCheck, User, PauseCircle, PlayCircle, Pencil } from 'lucide-vue-next'
import { useRoute, useRouter } from 'vue-router'
import api from '../services/api'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()

const loading = ref(true)
const error = ref('')
const activeTab = ref('sites')
const sitesPagination = ref({ page: 1, per_page: 20, total: 0, total_pages: 0 })

const sites = ref([])
const users = ref([])
const subdomains = ref([])
const dbLinks = ref([])
const hostingPackages = ref([])

const mariadbDatabases = ref([])
const postgresDatabases = ref([])
const mariadbUsers = ref([])
const postgresUsers = ref([])
const mariadbSystemUsers = new Set(['root', 'mysql', 'mariadb.sys'])

const search = ref('')
const phpFilter = ref('')
const phpVersions = ref([])

const showAddSiteModal = ref(false)
const showEditSiteModal = ref(false)
const showSiteLogsModal = ref(false)
const showSubdomainModal = ref(false)
const showDbLinkModal = ref(false)
const discoveryLoading = ref(false)
const discoveredSites = ref([])
const importingDomain = ref('')

const siteActionLoading = ref(false)
const editSiteActionLoading = ref(false)
const subdomainActionLoading = ref(false)
const dbLinkActionLoading = ref(false)
const siteLogsLoading = ref(false)

const siteForm = ref({
  domain: '',
  user: '',
  php_version: '8.3',
  package: 'default',
  email: '',
  mail_domain: false,
})
const editSiteForm = ref({
  domain: '',
  owner: '',
  php_version: '8.3',
  package: 'default',
  email: '',
})
const subdomainForm = ref({ parent_domain: '', subdomain: '', php_version: '8.3' })
const dbLinkForm = ref({ domain: '', engine: 'mariadb', db_name: '', db_user: '', db_host: 'localhost', db_user_identity: '' })
const aliases = ref([])
const advancedDomain = ref('')
const advancedConfig = ref({
  open_basedir: false,
  rewrite_rules: '',
  vhost_config: '',
})
const aliasForm = ref({ alias: '' })
const customSslForm = ref({
  cert_pem: '',
  key_pem: '',
})
const siteLogsDomain = ref('')
const siteLogsKind = ref('access')
const siteLogsLines = ref([])
const platformStatus = ref({
  mail_domain_available: false,
  detected_mail_stack: [],
})

const filteredSites = computed(() => sites.value)

const packageOptions = computed(() => {
  const names = new Set(['default'])
  for (const pkg of hostingPackages.value || []) {
    const name = String(pkg?.name || '').trim()
    if (name) names.add(name)
  }
  for (const site of sites.value || []) {
    const name = String(site?.package || '').trim()
    if (name) names.add(name)
  }
  const currentCreate = String(siteForm.value?.package || '').trim()
  const currentEdit = String(editSiteForm.value?.package || '').trim()
  if (currentCreate) names.add(currentCreate)
  if (currentEdit) names.add(currentEdit)

  const ordered = Array.from(names).filter(Boolean)
  const tail = ordered.filter(name => name !== 'default').sort((a, b) => a.localeCompare(b))
  return ['default', ...tail]
})

const owners = computed(() => {
  return users.value
    .map(u => u.username)
    .filter(Boolean)
})

const parentDomains = computed(() =>
  sites.value.map(s => s.domain).filter(Boolean)
)

const currentDbNames = computed(() => {
  const source = dbLinkForm.value.engine === 'mariadb' ? mariadbDatabases.value : postgresDatabases.value
  return source.map(x => x.name).filter(Boolean)
})

const currentDbUserOptions = computed(() => {
  const source = dbLinkForm.value.engine === 'mariadb' ? mariadbUsers.value : postgresUsers.value
  const dedupe = new Set()
  const options = []
  for (const item of source || []) {
    const username = String(item?.username || '').trim()
    if (!username) continue
    if (dbLinkForm.value.engine === 'mariadb' && mariadbSystemUsers.has(username.toLowerCase())) {
      continue
    }
    const host = String(item?.host || 'localhost').trim() || 'localhost'
    const identity = `${username}@${host}`
    const key = identity.toLowerCase()
    if (dedupe.has(key)) continue
    dedupe.add(key)
    options.push({
      identity,
      username,
      host,
      label: identity,
    })
  }
  return options
})

const domainAliases = computed(() =>
  aliases.value.filter(a => a.domain === advancedDomain.value)
)

watch(() => dbLinkForm.value.engine, () => {
  dbLinkForm.value.db_name = currentDbNames.value[0] || ''
  selectDbUserOption()
})

watch(() => dbLinkForm.value.db_user_identity, (identity) => {
  const selected = currentDbUserOptions.value.find(item => item.identity === identity)
  if (!selected) return
  dbLinkForm.value.db_user = selected.username
  dbLinkForm.value.db_host = selected.host
})

watch(showSubdomainModal, (open) => {
  if (open && !subdomainForm.value.parent_domain) {
    subdomainForm.value.parent_domain = parentDomains.value[0] || ''
  }
})

watch(showDbLinkModal, (open) => {
  if (!open) return
  if (!dbLinkForm.value.domain) dbLinkForm.value.domain = parentDomains.value[0] || ''
  if (!dbLinkForm.value.db_name) dbLinkForm.value.db_name = currentDbNames.value[0] || ''
  selectDbUserOption(dbLinkForm.value.db_user)
})

watch(parentDomains, (domains) => {
  if (!domains.includes(advancedDomain.value)) {
    advancedDomain.value = domains[0] || ''
  }
})

watch(currentDbUserOptions, () => {
  if (showDbLinkModal.value) {
    selectDbUserOption(dbLinkForm.value.db_user)
  }
})

watch(() => sitesPagination.value.per_page, async () => {
  if (!loading.value) {
    await applySiteFilters()
  }
})

watch(packageOptions, (options) => {
  if (!options.length) return
  if (!options.includes(siteForm.value.package)) {
    siteForm.value.package = options[0]
  }
  if (!options.includes(editSiteForm.value.package)) {
    editSiteForm.value.package = options[0]
  }
}, { immediate: true })

function apiErrorMessage(err, fallback) {
  return err?.response?.data?.message || err?.message || fallback
}

function formatUnix(ts) {
  if (!ts) return '-'
  try {
    return new Date(ts * 1000).toLocaleString()
  } catch {
    return String(ts)
  }
}

function selectDbUserOption(preferredUsername = '') {
  const options = currentDbUserOptions.value
  if (options.length === 0) {
    dbLinkForm.value.db_user_identity = ''
    dbLinkForm.value.db_user = ''
    dbLinkForm.value.db_host = 'localhost'
    return
  }

  const preferred = String(preferredUsername || '').trim()
  let selected = options.find(item => item.identity === dbLinkForm.value.db_user_identity)
  if (preferred) {
    selected = options.find(item => item.username === preferred) || selected
  }
  if (!selected) {
    selected = options[0]
  }
  dbLinkForm.value.db_user_identity = selected.identity
  dbLinkForm.value.db_user = selected.username
  dbLinkForm.value.db_host = selected.host
}

function isSuspended(site) {
  return String(site?.status || 'active').toLowerCase() === 'suspended'
}

async function loadPhpVersions() {
  try {
    const res = await api.get('/php/versions')
    const all = res.data?.data || []
    phpVersions.value = all.filter(v => v.installed).map(v => v.version)
    if (phpVersions.value.length === 0) {
      phpVersions.value = ['8.4', '8.3', '8.2', '8.1', '8.0', '7.4'] // Fallback
    }
  } catch {
    phpVersions.value = ['8.4', '8.3', '8.2', '8.1', '8.0', '7.4']
  }
}

async function loadSites() {
  try {
    const res = await api.get('/vhost/list', {
      params: {
        search: search.value || undefined,
        php: phpFilter.value || undefined,
        page: sitesPagination.value.page,
        per_page: sitesPagination.value.per_page,
      },
    })
    sites.value = res.data?.data || []
    const pg = res.data?.pagination || {}
    sitesPagination.value.total = Number(pg.total || sites.value.length || 0)
    sitesPagination.value.total_pages = Number(pg.total_pages || 1)
  } catch (e) {
    throw new Error(apiErrorMessage(e, t('websites.errors.site_list_failed')))
  }
}

async function loadUsers() {
  try {
    const res = await api.get('/users/list')
    users.value = res.data?.data || []
  } catch {
    users.value = []
  }
}

async function loadSubdomains() {
  try {
    const res = await api.get('/websites/subdomains')
    subdomains.value = res.data?.data || []
  } catch {
    subdomains.value = []
  }
}

async function loadDbLinks() {
  try {
    const res = await api.get('/websites/db-links')
    const items = Array.isArray(res.data?.data) ? res.data.data : []
    dbLinks.value = items.map(item => ({
      ...item,
      db_host: item?.db_host || 'localhost',
    }))
  } catch {
    dbLinks.value = []
  }
}

async function loadPackages() {
  try {
    const res = await api.get('/packages/list')
    hostingPackages.value = res.data?.data || []
  } catch {
    hostingPackages.value = []
  }
}

async function loadAliases() {
  if (!advancedDomain.value) {
    aliases.value = []
    return
  }
  try {
    const res = await api.get('/websites/aliases', { params: { domain: advancedDomain.value } })
    aliases.value = res.data?.data || []
  } catch {
    aliases.value = []
  }
}

async function loadAdvancedConfig() {
  if (!advancedDomain.value) {
    advancedConfig.value = { open_basedir: false, rewrite_rules: '', vhost_config: '' }
    return
  }
  try {
    const res = await api.get('/websites/advanced-config', { params: { domain: advancedDomain.value } })
    const data = res.data?.data || {}
    advancedConfig.value = {
      open_basedir: !!data.open_basedir,
      rewrite_rules: data.rewrite_rules || '',
      vhost_config: data.vhost_config || '',
    }
  } catch {
    advancedConfig.value = { open_basedir: false, rewrite_rules: '', vhost_config: '' }
  }
}

async function loadCustomSsl() {
  if (!advancedDomain.value) {
    customSslForm.value = { cert_pem: '', key_pem: '' }
    return
  }
  try {
    const res = await api.get('/websites/custom-ssl', { params: { domain: advancedDomain.value } })
    const data = res.data?.data || {}
    customSslForm.value = {
      cert_pem: data.cert_pem || '',
      key_pem: data.key_pem || '',
    }
  } catch {
    customSslForm.value = { cert_pem: '', key_pem: '' }
  }
}

async function refreshAdvanced() {
  if (!advancedDomain.value) return
  await Promise.all([loadAliases(), loadAdvancedConfig(), loadCustomSsl()])
}

async function loadDatabases() {
  const [mariaDbRes, mariaUsersRes, pgDbRes, pgUsersRes] = await Promise.all([
    api.get('/db/mariadb/list'),
    api.get('/db/mariadb/users'),
    api.get('/db/postgres/list'),
    api.get('/db/postgres/users'),
  ])

  mariadbDatabases.value = mariaDbRes.data?.data || []
  mariadbUsers.value = mariaUsersRes.data?.data || []
  postgresDatabases.value = pgDbRes.data?.data || []
  postgresUsers.value = pgUsersRes.data?.data || []
}

async function loadPlatformStatus() {
  try {
    const res = await api.get('/security/status')
    platformStatus.value = {
      mail_domain_available: !!res.data?.data?.mail_domain_available,
      detected_mail_stack: Array.isArray(res.data?.data?.detected_mail_stack) ? res.data.data.detected_mail_stack : [],
    }
  } catch {
    platformStatus.value = {
      mail_domain_available: false,
      detected_mail_stack: [],
    }
  }
}

async function loadDiscoveredSites() {
  discoveryLoading.value = true
  try {
    const res = await api.get('/vhost/discover')
    const items = Array.isArray(res.data?.data) ? res.data.data : []
    discoveredSites.value = items.map((item) => ({
      ...item,
      _php_version: item?.suggested_php || phpVersions.value[0] || '8.3',
      _owner: item?.owner || item?.user || owners.value[0] || '',
    }))
  } catch {
    discoveredSites.value = []
  } finally {
    discoveryLoading.value = false
  }
}

async function refreshAll() {
  loading.value = true
  error.value = ''
  try {
    await Promise.all([loadSites(), loadUsers(), loadSubdomains(), loadDbLinks(), loadDatabases(), loadPackages(), loadPlatformStatus(), loadPhpVersions(), loadDiscoveredSites()])
    if (!advancedDomain.value) {
      advancedDomain.value = parentDomains.value[0] || ''
    }
    await refreshAdvanced()
  } catch (e) {
    error.value = apiErrorMessage(e, t('websites.errors.data_load_failed'))
  } finally {
    loading.value = false
  }
}

async function importDiscoveredSite(item) {
  if (!item?.domain) return
  importingDomain.value = item.domain
  error.value = ''
  try {
    await api.post('/vhost/import', {
      domain: item.domain,
      owner: item._owner || item.owner || item.user || undefined,
      php_version: item._php_version || item.suggested_php || phpVersions.value[0] || '8.3',
      package: 'default',
    })
    await refreshAll()
  } catch (e) {
    error.value = apiErrorMessage(e, t('websites.errors.website_import_failed'))
  } finally {
    importingDomain.value = ''
  }
}

async function applySiteFilters() {
  sitesPagination.value.page = 1
  await loadSites()
}

async function changeSitesPage(delta) {
  const next = sitesPagination.value.page + delta
  const maxPage = Math.max(1, sitesPagination.value.total_pages || 1)
  if (next < 1 || next > maxPage) return
  sitesPagination.value.page = next
  await loadSites()
}

async function loadSiteLogs() {
  if (!siteLogsDomain.value) return
  siteLogsLoading.value = true
  error.value = ''
  try {
    const res = await api.get('/monitor/logs/site', {
      params: {
        domain: siteLogsDomain.value,
        kind: siteLogsKind.value,
        lines: 200,
      },
    })
    siteLogsLines.value = res.data?.data || []
  } catch (e) {
    error.value = apiErrorMessage(e, t('websites.errors.site_logs_failed'))
    siteLogsLines.value = []
  } finally {
    siteLogsLoading.value = false
  }
}

async function openSiteLogs(site, kind = 'access') {
  siteLogsDomain.value = site.domain || ''
  siteLogsKind.value = kind
  showSiteLogsModal.value = true
  await loadSiteLogs()
}

async function switchSiteLogKind(kind) {
  siteLogsKind.value = kind
  await loadSiteLogs()
}

async function addSite() {
  if (!siteForm.value.domain) return
  siteActionLoading.value = true
  error.value = ''
  try {
    await api.post('/vhost', {
      domain: siteForm.value.domain,
      user: siteForm.value.user || undefined,
      php_version: siteForm.value.php_version,
      package: siteForm.value.package || 'default',
      email: siteForm.value.email || undefined,
      mail_domain: !!siteForm.value.mail_domain,
    })
    showAddSiteModal.value = false
    siteForm.value = {
      domain: '',
      user: siteForm.value.user || owners.value[0] || '',
      php_version: '8.3',
      package: siteForm.value.package || 'default',
      email: '',
      mail_domain: false,
    }
    await refreshAll()
  } catch (e) {
    error.value = apiErrorMessage(e, t('websites.errors.site_create_failed'))
  } finally {
    siteActionLoading.value = false
  }
}

async function deleteSite(site) {
  if (!confirm(`${site.domain} ${t('common.confirm_delete')}`)) return
  error.value = ''
  try {
    await api.post('/vhost/delete', { domain: site.domain })
    await refreshAll()
  } catch (e) {
    error.value = apiErrorMessage(e, t('websites.errors.site_delete_failed'))
  }
}

async function issueSSL(site) {
  error.value = ''
  try {
    await api.post('/ssl/issue', {
      domain: site.domain,
      email: `admin@${site.domain}`,
      provider: 'letsencrypt',
    })
    await refreshAll()
  } catch (e) {
    error.value = apiErrorMessage(e, t('websites.errors.ssl_issue_failed'))
  }
}

async function toggleSuspend(site) {
  error.value = ''
  try {
    if (isSuspended(site)) {
      await api.post('/vhost/unsuspend', { domain: site.domain })
    } else {
      await api.post('/vhost/suspend', { domain: site.domain })
    }
    await refreshAll()
  } catch (e) {
    error.value = apiErrorMessage(e, t('websites.errors.website_status_update_failed'))
  }
}

function openEditSiteModal(site) {
  editSiteForm.value = {
    domain: site.domain || '',
    owner: site.owner || site.user || '',
    php_version: site.php_version || site.php || '8.3',
    package: site.package || 'default',
    email: site.email || `webmaster@${site.domain || ''}`,
  }
  showEditSiteModal.value = true
}

function openManagePage(site) {
  if (!site?.domain) return
  router.push(`/websites/${site.domain}`)
}

async function updateSite() {
  if (!editSiteForm.value.domain || !editSiteForm.value.owner || !editSiteForm.value.php_version || !editSiteForm.value.email) {
    return
  }

  editSiteActionLoading.value = true
  error.value = ''
  try {
    await api.post('/vhost/update', {
      domain: editSiteForm.value.domain,
      owner: editSiteForm.value.owner,
      php_version: editSiteForm.value.php_version,
      package: editSiteForm.value.package || 'default',
      email: editSiteForm.value.email,
    })
    showEditSiteModal.value = false
    await refreshAll()
  } catch (e) {
    error.value = apiErrorMessage(e, t('websites.errors.website_update_failed'))
  } finally {
    editSiteActionLoading.value = false
  }
}

async function createSubdomain() {
  if (!subdomainForm.value.parent_domain || !subdomainForm.value.subdomain) return
  subdomainActionLoading.value = true
  error.value = ''
  try {
    await api.post('/websites/subdomains', {
      parent_domain: subdomainForm.value.parent_domain,
      subdomain: subdomainForm.value.subdomain,
      php_version: subdomainForm.value.php_version,
    })
    showSubdomainModal.value = false
    subdomainForm.value = {
      parent_domain: parentDomains.value[0] || '',
      subdomain: '',
      php_version: '8.3',
    }
    await loadSubdomains()
  } catch (e) {
    error.value = apiErrorMessage(e, t('websites.errors.subdomain_create_failed'))
  } finally {
    subdomainActionLoading.value = false
  }
}

async function updateSubdomainPhp(subdomain) {
  error.value = ''
  try {
    await api.post('/websites/subdomains/php', {
      fqdn: subdomain.fqdn,
      php_version: subdomain.php_version,
    })
  } catch (e) {
    error.value = apiErrorMessage(e, t('websites.errors.subdomain_php_update_failed'))
  }
}

async function deleteSubdomain(fqdn) {
  if (!confirm(t('websites.confirm_subdomain_delete', { fqdn }))) return
  const deleteDocroot = confirm(t('websites.confirm_subdomain_delete_docroot'))
  error.value = ''
  try {
    await api.delete('/websites/subdomains', { params: { fqdn, delete_docroot: deleteDocroot } })
    await loadSubdomains()
  } catch (e) {
    error.value = apiErrorMessage(e, t('websites.errors.subdomain_delete_failed'))
  }
}

async function convertSubdomain(subdomain) {
  if (!confirm(t('websites.confirm_subdomain_convert', { fqdn: subdomain.fqdn }))) return
  error.value = ''
  try {
    await api.post('/websites/subdomains/convert', {
      fqdn: subdomain.fqdn,
      owner: subdomain.owner || siteForm.value.user || owners.value[0] || undefined,
      php_version: subdomain.php_version,
    })
    activeTab.value = 'sites'
    await refreshAll()
  } catch (e) {
    error.value = apiErrorMessage(e, t('websites.errors.subdomain_convert_failed'))
  }
}

async function addAlias() {
  if (!advancedDomain.value || !aliasForm.value.alias) return
  error.value = ''
  try {
    await api.post('/websites/aliases', {
      domain: advancedDomain.value,
      alias: aliasForm.value.alias,
    })
    aliasForm.value.alias = ''
    await loadAliases()
  } catch (e) {
    error.value = apiErrorMessage(e, t('websites.errors.alias_add_failed'))
  }
}

async function deleteAlias(alias) {
  if (!advancedDomain.value) return
  if (!confirm(t('websites.confirm_alias_delete', { alias }))) return
  error.value = ''
  try {
    await api.delete('/websites/aliases', {
      params: {
        domain: advancedDomain.value,
        alias,
      },
    })
    await loadAliases()
  } catch (e) {
    error.value = apiErrorMessage(e, t('websites.errors.alias_delete_failed'))
  }
}

async function saveOpenBasedir() {
  if (!advancedDomain.value) return
  error.value = ''
  try {
    await api.post('/websites/open-basedir', {
      domain: advancedDomain.value,
      enabled: !!advancedConfig.value.open_basedir,
    })
    await loadAdvancedConfig()
  } catch (e) {
    error.value = apiErrorMessage(e, t('websites.errors.open_basedir_save_failed'))
  }
}

async function saveRewrite() {
  if (!advancedDomain.value) return
  error.value = ''
  try {
    await api.post('/websites/rewrite', {
      domain: advancedDomain.value,
      rules: advancedConfig.value.rewrite_rules || '',
    })
    await loadAdvancedConfig()
  } catch (e) {
    error.value = apiErrorMessage(e, t('websites.errors.rewrite_save_failed'))
  }
}

async function saveVhostConfig() {
  if (!advancedDomain.value) return
  error.value = ''
  try {
    await api.post('/websites/vhost-config', {
      domain: advancedDomain.value,
      content: advancedConfig.value.vhost_config || '',
    })
    await loadAdvancedConfig()
  } catch (e) {
    error.value = apiErrorMessage(e, t('websites.errors.vhost_config_save_failed'))
  }
}

async function saveCustomSsl() {
  if (!advancedDomain.value) return
  error.value = ''
  try {
    await api.post('/websites/custom-ssl', {
      domain: advancedDomain.value,
      cert_pem: customSslForm.value.cert_pem || '',
      key_pem: customSslForm.value.key_pem || '',
    })
    await loadCustomSsl()
  } catch (e) {
    error.value = apiErrorMessage(e, t('websites.errors.custom_ssl_save_failed'))
  }
}

async function attachDbLink() {
  if (!dbLinkForm.value.domain || !dbLinkForm.value.db_name || !dbLinkForm.value.db_user) return
  dbLinkActionLoading.value = true
  error.value = ''
  try {
    await api.post('/websites/db-links', {
      domain: dbLinkForm.value.domain,
      engine: dbLinkForm.value.engine,
      db_name: dbLinkForm.value.db_name,
      db_user: dbLinkForm.value.db_user,
      db_host: dbLinkForm.value.db_host || 'localhost',
    })
    showDbLinkModal.value = false
    await loadDbLinks()
  } catch (e) {
    error.value = apiErrorMessage(e, t('websites.errors.db_link_create_failed'))
  } finally {
    dbLinkActionLoading.value = false
  }
}

async function detachDbLink(link) {
  if (!confirm(t('websites.confirm_db_link_detach', { domain: link.domain, db_name: link.db_name }))) return
  error.value = ''
  try {
    await api.delete('/websites/db-links', {
      params: {
        domain: link.domain,
        engine: link.engine,
        db_name: link.db_name,
        db_user: link.db_user,
        db_host: link.db_host || 'localhost',
      },
    })
    await loadDbLinks()
  } catch (e) {
    error.value = apiErrorMessage(e, t('websites.errors.db_link_delete_failed'))
  }
}

onMounted(async () => {
  const tab = String(route.query.tab || '').toLowerCase()
  if (tab === 'subdomains') activeTab.value = 'subdomains'
  if (tab === 'db-links' || tab === 'dblink' || tab === 'db') activeTab.value = 'dbLinks'
  if (tab === 'advanced' || tab === 'config' || tab === 'settings') activeTab.value = 'advanced'

  if (route.query.engine) dbLinkForm.value.engine = String(route.query.engine)
  if (route.query.db_name) dbLinkForm.value.db_name = String(route.query.db_name)
  if (route.query.domain) dbLinkForm.value.domain = String(route.query.domain)

  await refreshAll()
})
</script>
