<template>
  <div class="min-h-screen bg-panel-darker flex text-gray-100">
    <!-- Sidebar -->
    <aside class="w-64 bg-panel-dark border-r border-panel-border flex flex-col transition-all duration-300">
      <div class="h-16 flex items-center px-6 border-b border-panel-border">
        <div class="w-full flex items-center">
          <img
            src="/aurapanel-logo.png"
            alt="AuraPanel Logo"
            class="h-8 w-auto max-w-[180px] object-contain"
          />
        </div>
      </div>
      
      <nav class="flex-1 px-4 py-6 space-y-1 overflow-y-auto">
        <div class="mb-3 rounded-xl border border-panel-border bg-panel-card/85 px-3 py-2.5 shadow-lg shadow-black/10">
          <div class="flex items-center gap-1.5 text-[10px] font-semibold tracking-[0.16em] text-gray-400">
            <Network class="h-3.5 w-3.5 text-brand-400" />
            <span>{{ t('activity_log.ip') }}</span>
          </div>
          <p class="mt-1.5 break-all font-mono text-xs text-gray-100">
            {{ sidebarServerIp || (sidebarIpLoading ? t('common.loading') : '-') }}
          </p>
        </div>

        <button
          class="sticky top-0 z-[5] mb-3 flex w-full items-center justify-between rounded-xl border border-panel-border bg-panel-card/95 px-3 py-2.5 text-xs font-semibold tracking-[0.16em] text-gray-300 shadow-lg shadow-black/10 backdrop-blur-sm transition hover:border-brand-500/30 hover:text-white"
          @click="toggleAllMenus"
        >
          <div class="flex items-center gap-2">
            <component :is="allMenusExpanded ? ChevronsUp : ChevronsDown" class="h-4 w-4 text-brand-400" />
            <span>{{ allMenusExpanded ? t('layout.toggle_all_close') : t('layout.toggle_all_open') }}</span>
          </div>
          <span class="text-[10px] tracking-[0.2em] text-gray-500">{{ t('layout.toggle_all_label') }}</span>
        </button>

        <router-link to="/" class="sidebar-link" exact-active-class="sidebar-link-active">
          <LayoutDashboard class="w-5 h-5 mr-3" />
          <span>{{ t('menu.dashboard') }}</span>
        </router-link>

        <div v-if="canHostingGroup" class="mt-3">
          <button @click="toggleTopLevelMenu('hosting')" class="sidebar-link w-full justify-between" :class="{ 'sidebar-link-section-active': isHostingRoute }">
            <div class="flex items-center">
              <Box class="w-5 h-5 mr-3" />
              <span>{{ t('layout.groups.hosting') }}</span>
            </div>
            <ChevronDown class="w-4 h-4 transition-transform duration-200" :class="{ 'rotate-180': hostingMenuOpen }" />
          </button>

          <transition name="accordion">
            <div v-show="hostingMenuOpen" class="ml-4 mt-1 space-y-0.5 border-l border-panel-border/50 pl-3">
              <router-link v-if="can('/websites')" to="/websites" class="sidebar-sub-link" active-class="sidebar-sub-link-active">
                <span>{{ t('menu.websites') }}</span>
              </router-link>
              <router-link v-if="can('/dns')" to="/dns" class="sidebar-sub-link" active-class="sidebar-sub-link-active">
                <span>{{ t('menu.dns') }}</span>
              </router-link>
              <router-link v-if="can('/filemanager')" to="/filemanager" class="sidebar-sub-link" active-class="sidebar-sub-link-active">
                <span>{{ t('layout.links.file_manager') }}</span>
              </router-link>
              <router-link v-if="can('/migration')" to="/migration" class="sidebar-sub-link" active-class="sidebar-sub-link-active">
                <span>{{ t('routes.MigrationWizard') }}</span>
              </router-link>
              <router-link v-if="can('/packages')" to="/packages" class="sidebar-sub-link" active-class="sidebar-sub-link-active">
                <span>{{ t('menu.packages') }}</span>
              </router-link>
              <router-link v-if="can('/users')" to="/users" class="sidebar-sub-link" active-class="sidebar-sub-link-active">
                <span>{{ t('menu.users') }}</span>
              </router-link>
              <router-link v-if="can('/reseller')" to="/reseller" class="sidebar-sub-link" active-class="sidebar-sub-link-active">
                <span>{{ t('layout.links.reseller_acl') }}</span>
              </router-link>
            </div>
          </transition>
        </div>

        <div v-if="canWebAppsGroup" class="mt-2">
          <button @click="toggleTopLevelMenu('webApps')" class="sidebar-link w-full justify-between" :class="{ 'sidebar-link-section-active': isWebAppsRoute }">
            <div class="flex items-center">
              <Globe class="w-5 h-5 mr-3" />
              <span>{{ t('layout.groups.web_apps') }}</span>
            </div>
            <ChevronDown class="w-4 h-4 transition-transform duration-200" :class="{ 'rotate-180': webAppsMenuOpen }" />
          </button>

          <transition name="accordion">
            <div v-show="webAppsMenuOpen" class="ml-4 mt-1 space-y-0.5 border-l border-panel-border/50 pl-3">
              <router-link v-if="can('/cloudflare')" to="/cloudflare" class="sidebar-sub-link" active-class="sidebar-sub-link-active">
                <span>{{ t('routes.CloudFlare') }}</span>
              </router-link>
              <router-link v-if="can('/wordpress')" to="/wordpress" class="sidebar-sub-link" active-class="sidebar-sub-link-active">
                <span>{{ t('layout.links.wordpress_manager') }}</span>
              </router-link>
              <router-link v-if="can('/plugins')" to="/plugins" class="sidebar-sub-link" active-class="sidebar-sub-link-active">
                <span>{{ t('layout.links.plugin_sdk') }}</span>
              </router-link>
              <router-link v-if="can('/app-runtime')" to="/app-runtime" class="sidebar-sub-link" active-class="sidebar-sub-link-active">
                <span>{{ t('layout.links.app_runtime') }}</span>
              </router-link>
              <router-link v-if="can('/php')" to="/php" class="sidebar-sub-link" active-class="sidebar-sub-link-active">
                <span>{{ t('routes.PHP') }}</span>
              </router-link>
              <router-link v-if="can('/terminal')" to="/terminal" class="sidebar-sub-link" active-class="sidebar-sub-link-active">
                <span>{{ t('layout.links.terminal') }}</span>
              </router-link>
            </div>
          </transition>
        </div>

        <div v-if="canDataAccessGroup" class="mt-2">
          <button @click="toggleTopLevelMenu('dataAccess')" class="sidebar-link w-full justify-between" :class="{ 'sidebar-link-section-active': isDataAccessRoute }">
            <div class="flex items-center">
              <Database class="w-5 h-5 mr-3" />
              <span>{{ t('layout.groups.data_access') }}</span>
            </div>
            <ChevronDown class="w-4 h-4 transition-transform duration-200" :class="{ 'rotate-180': dataAccessMenuOpen }" />
          </button>

          <transition name="accordion">
            <div v-show="dataAccessMenuOpen" class="ml-4 mt-1 space-y-0.5 border-l border-panel-border/50 pl-3">
              <router-link v-if="can('/databases')" to="/databases" class="sidebar-sub-link" active-class="sidebar-sub-link-active">
                <span>{{ t('menu.databases') }}</span>
              </router-link>
              <router-link v-if="can('/emails')" to="/emails" class="sidebar-sub-link" active-class="sidebar-sub-link-active">
                <span>{{ t('menu.emails') }}</span>
              </router-link>
              <router-link v-if="can('/minio')" to="/minio" class="sidebar-sub-link" active-class="sidebar-sub-link-active">
                <span>{{ t('routes.MinIO') }}</span>
              </router-link>

              <button @click="ftpMenuOpen = !ftpMenuOpen" class="sidebar-sub-link w-full justify-between" :class="{ 'sidebar-sub-link-active': isFtpRoute }">
                <div class="flex items-center">
                  <KeyRound class="w-4 h-4 mr-2" />
                  <span>{{ t('layout.groups.ftp_sftp') }}</span>
                </div>
                <ChevronDown class="w-3.5 h-3.5 transition-transform duration-200" :class="{ 'rotate-180': ftpMenuOpen }" />
              </button>

              <transition name="accordion">
                <div v-show="ftpMenuOpen" class="ml-3 mt-1 space-y-0.5 border-l border-panel-border/40 pl-3">
                  <router-link v-if="can('/ftp')" to="/ftp" class="sidebar-sub-link" active-class="sidebar-sub-link-active">
                    <span>{{ t('routes.FTP') }}</span>
                  </router-link>
                  <router-link v-if="can('/sftp')" to="/sftp" class="sidebar-sub-link" active-class="sidebar-sub-link-active">
                    <span>{{ t('routes.SFTP') }}</span>
                  </router-link>
                </div>
              </transition>

              <button @click="backupsMenuOpen = !backupsMenuOpen" class="sidebar-sub-link w-full justify-between" :class="{ 'sidebar-sub-link-active': isBackupsRoute }">
                <div class="flex items-center">
                  <HardDrive class="w-4 h-4 mr-2" />
                  <span>{{ t('layout.groups.backups') }}</span>
                </div>
                <ChevronDown class="w-3.5 h-3.5 transition-transform duration-200" :class="{ 'rotate-180': backupsMenuOpen }" />
              </button>

              <transition name="accordion">
                <div v-show="backupsMenuOpen" class="ml-3 mt-1 space-y-0.5 border-l border-panel-border/40 pl-3">
                  <router-link v-if="can('/backups')" to="/backups" class="sidebar-sub-link" active-class="sidebar-sub-link-active">
                    <span>{{ t('layout.links.file_backups') }}</span>
                  </router-link>
                  <router-link v-if="can('/db-backup')" to="/db-backup" class="sidebar-sub-link" active-class="sidebar-sub-link-active">
                    <span>{{ t('routes.DbBackup') }}</span>
                  </router-link>
                </div>
              </transition>
            </div>
          </transition>
        </div>

        <div v-if="canTuningGroup" class="mt-2">
          <button @click="toggleTopLevelMenu('tuning')" class="sidebar-link w-full justify-between" :class="{ 'sidebar-link-section-active': isTuningRoute }">
            <div class="flex items-center">
              <Activity class="w-5 h-5 mr-3" />
              <span>{{ t('layout.groups.tuning') }}</span>
            </div>
            <ChevronDown class="w-4 h-4 transition-transform duration-200" :class="{ 'rotate-180': tuningMenuOpen }" />
          </button>

          <transition name="accordion">
            <div v-show="tuningMenuOpen" class="ml-4 mt-1 space-y-0.5 border-l border-panel-border/50 pl-3">
              <router-link
                v-if="can('/ols-tuning')"
                to="/ols-tuning"
                class="sidebar-sub-link"
                :class="{ 'sidebar-sub-link-active': route.path === '/ols-tuning' }"
              >
                <span>{{ t('routes.OlsTuning') }}</span>
              </router-link>
              <router-link
                v-if="can('/databases')"
                :to="{ path: '/databases', query: { tab: 'tuning', tuning_engine: 'mariadb' } }"
                class="sidebar-sub-link"
                :class="{ 'sidebar-sub-link-active': isDatabaseTuningActive('mariadb') }"
              >
                <span>{{ t('layout.links.mariadb_tuning') }}</span>
              </router-link>
              <router-link
                v-if="can('/databases')"
                :to="{ path: '/databases', query: { tab: 'tuning', tuning_engine: 'postgresql' } }"
                class="sidebar-sub-link"
                :class="{ 'sidebar-sub-link-active': isDatabaseTuningActive('postgresql') }"
              >
                <span>{{ t('layout.links.postgresql_tuning') }}</span>
              </router-link>
              <router-link
                v-if="can('/ftp')"
                :to="{ path: '/ftp', query: { tab: 'tuning' } }"
                class="sidebar-sub-link"
                :class="{ 'sidebar-sub-link-active': isFtpTuningRoute }"
              >
                <span>{{ t('layout.links.ftp_tuning') }}</span>
              </router-link>
              <router-link
                v-if="can('/php')"
                :to="{ path: '/php', query: { tab: 'config' } }"
                class="sidebar-sub-link"
                :class="{ 'sidebar-sub-link-active': isPhpTuningRoute }"
              >
                <span>{{ t('layout.links.php_config') }}</span>
              </router-link>
              <router-link
                v-if="can('/mail-tuning')"
                to="/mail-tuning"
                class="sidebar-sub-link"
                :class="{ 'sidebar-sub-link-active': route.path === '/mail-tuning' }"
              >
                <span>{{ t('routes.MailTuning') }}</span>
              </router-link>
            </div>
          </transition>
        </div>

        <div v-if="canSecurityGroup" class="mt-2">
          <button @click="toggleTopLevelMenu('securityLogs')" class="sidebar-link w-full justify-between" :class="{ 'sidebar-link-section-active': isSecurityLogsRoute }">
            <div class="flex items-center">
              <Shield class="w-5 h-5 mr-3" />
              <span>{{ t('layout.groups.security_logs') }}</span>
            </div>
            <ChevronDown class="w-4 h-4 transition-transform duration-200" :class="{ 'rotate-180': securityLogsMenuOpen }" />
          </button>

          <transition name="accordion">
            <div v-show="securityLogsMenuOpen" class="ml-4 mt-1 space-y-0.5 border-l border-panel-border/50 pl-3">
              <button @click="sslMenuOpen = !sslMenuOpen" class="sidebar-sub-link w-full justify-between" :class="{ 'sidebar-sub-link-active': isSslRoute }">
                <div class="flex items-center">
                  <Lock class="w-4 h-4 mr-2" />
                  <span>{{ t('routes.SSL') }}</span>
                </div>
                <ChevronDown class="w-3.5 h-3.5 transition-transform duration-200" :class="{ 'rotate-180': sslMenuOpen }" />
              </button>

              <transition name="accordion">
                <div v-show="sslMenuOpen" class="ml-3 mt-1 space-y-0.5 border-l border-panel-border/40 pl-3">
                  <router-link
                    v-if="can('/ssl')"
                    :to="{ path: '/ssl', query: { tab: 'manage' } }"
                    class="sidebar-sub-link"
                    :class="{ 'sidebar-sub-link-active': isSslTabActive('manage') }"
                  >
                    <span>{{ t('layout.links.ssl_manage') }}</span>
                  </router-link>
                  <router-link
                    v-if="can('/ssl')"
                    :to="{ path: '/ssl', query: { tab: 'hostname' } }"
                    class="sidebar-sub-link"
                    :class="{ 'sidebar-sub-link-active': isSslTabActive('hostname') }"
                  >
                    <span>{{ t('layout.links.ssl_hostname') }}</span>
                  </router-link>
                  <router-link
                    v-if="can('/ssl')"
                    :to="{ path: '/ssl', query: { tab: 'mail' } }"
                    class="sidebar-sub-link"
                    :class="{ 'sidebar-sub-link-active': isSslTabActive('mail') }"
                  >
                    <span>{{ t('layout.links.ssl_mail') }}</span>
                  </router-link>
                </div>
              </transition>

              <button @click="securityMenuOpen = !securityMenuOpen" class="sidebar-sub-link w-full justify-between" :class="{ 'sidebar-sub-link-active': isSecurityRoute }">
                <div class="flex items-center">
                  <Shield class="w-4 h-4 mr-2" />
                  <span>{{ t('menu.security') }}</span>
                </div>
                <ChevronDown class="w-3.5 h-3.5 transition-transform duration-200" :class="{ 'rotate-180': securityMenuOpen }" />
              </button>

              <transition name="accordion">
                <div v-show="securityMenuOpen" class="ml-3 mt-1 space-y-0.5 border-l border-panel-border/40 pl-3">
                  <router-link v-if="can('/security')" :to="{ path: '/security', query: { tab: 'overview' } }" class="sidebar-sub-link" active-class="sidebar-sub-link-active">
                    <span>{{ t('layout.links.overview') }}</span>
                  </router-link>
                  <router-link v-if="can('/security')" :to="{ path: '/security', query: { tab: 'firewall' } }" class="sidebar-sub-link" active-class="sidebar-sub-link-active">
                    <span>{{ t('layout.links.firewall') }}</span>
                  </router-link>
                  <router-link v-if="can('/security')" :to="{ path: '/security', query: { tab: 'waf' } }" class="sidebar-sub-link" active-class="sidebar-sub-link-active">
                    <span>{{ t('layout.links.ml_waf') }}</span>
                  </router-link>
                  <router-link v-if="can('/security')" :to="{ path: '/security', query: { tab: 'ddos' } }" class="sidebar-sub-link" active-class="sidebar-sub-link-active">
                    <span>{{ t('layout.links.ddos') }}</span>
                  </router-link>
                  <router-link v-if="can('/security')" :to="{ path: '/security', query: { tab: '2fa' } }" class="sidebar-sub-link" active-class="sidebar-sub-link-active">
                    <span>{{ t('layout.links.totp') }}</span>
                  </router-link>
                  <router-link v-if="can('/security')" :to="{ path: '/security', query: { tab: 'ssh_settings' } }" class="sidebar-sub-link" active-class="sidebar-sub-link-active">
                    <span>{{ t('layout.links.ssh_keys') }}</span>
                  </router-link>
                  <router-link v-if="can('/security')" :to="{ path: '/security', query: { tab: 'hardening' } }" class="sidebar-sub-link" active-class="sidebar-sub-link-active">
                    <span>{{ t('layout.links.hardening') }}</span>
                  </router-link>
                </div>
              </transition>

              <button @click="logsMenuOpen = !logsMenuOpen" class="sidebar-sub-link w-full justify-between" :class="{ 'sidebar-sub-link-active': isLogsRoute }">
                <div class="flex items-center">
                  <ScrollText class="w-4 h-4 mr-2" />
                  <span>{{ t('layout.groups.logs') }}</span>
                </div>
                <ChevronDown class="w-3.5 h-3.5 transition-transform duration-200" :class="{ 'rotate-180': logsMenuOpen }" />
              </button>

              <transition name="accordion">
                <div v-show="logsMenuOpen" class="ml-3 mt-1 space-y-0.5 border-l border-panel-border/40 pl-3">
                  <router-link v-if="can('/activity-log')" to="/activity-log" class="sidebar-sub-link" active-class="sidebar-sub-link-active">
                    <span>{{ t('layout.links.activity_log') }}</span>
                  </router-link>
                  <router-link v-if="can('/log-viewer')" to="/log-viewer" class="sidebar-sub-link" active-class="sidebar-sub-link-active">
                    <span>{{ t('layout.links.log_viewer') }}</span>
                  </router-link>
                </div>
              </transition>
            </div>
          </transition>
        </div>

        <div v-if="canDevopsGroup" class="mt-2">
          <button @click="toggleTopLevelMenu('devops')" class="sidebar-link w-full justify-between" :class="{ 'sidebar-link-section-active': isDevopsRoute }">
            <div class="flex items-center">
              <Container class="w-5 h-5 mr-3" />
              <span>{{ t('layout.groups.devops') }}</span>
            </div>
            <ChevronDown class="w-4 h-4 transition-transform duration-200" :class="{ 'rotate-180': devopsMenuOpen }" />
          </button>

          <transition name="accordion">
            <div v-show="devopsMenuOpen" class="ml-4 mt-1 space-y-0.5 border-l border-panel-border/50 pl-3">
              <button @click="dockerMenuOpen = !dockerMenuOpen" class="sidebar-sub-link w-full justify-between" :class="{ 'sidebar-sub-link-active': isDockerRoute }">
                <div class="flex items-center">
                  <Container class="w-4 h-4 mr-2" />
                  <span>{{ t('menu.docker') }}</span>
                </div>
                <ChevronDown class="w-3.5 h-3.5 transition-transform duration-200" :class="{ 'rotate-180': dockerMenuOpen }" />
              </button>

              <transition name="accordion">
                <div v-show="dockerMenuOpen" class="ml-3 mt-1 space-y-0.5 border-l border-panel-border/40 pl-3">
                  <div class="pt-1 pb-1">
                    <span class="px-2 text-[10px] font-semibold uppercase tracking-wider text-gray-500">{{ t('layout.labels.manager') }}</span>
                  </div>
                  <router-link v-if="can('/docker/images')" to="/docker/images" class="sidebar-sub-link" active-class="sidebar-sub-link-active">
                    <span>{{ t('layout.links.image_manager') }}</span>
                  </router-link>
                  <router-link v-if="can('/docker/containers')" to="/docker/containers" class="sidebar-sub-link" active-class="sidebar-sub-link-active">
                    <span>{{ t('layout.links.container_manager') }}</span>
                  </router-link>
                  <router-link v-if="can('/docker/create')" to="/docker/create" class="sidebar-sub-link" active-class="sidebar-sub-link-active">
                    <span>{{ t('layout.links.create_container') }}</span>
                  </router-link>
                  <div class="pt-2 pb-1">
                    <span class="px-2 text-[10px] font-semibold uppercase tracking-wider text-gray-500">{{ t('layout.labels.apps') }}</span>
                  </div>
                  <router-link v-if="can('/docker/apps')" to="/docker/apps" class="sidebar-sub-link" active-class="sidebar-sub-link-active">
                    <span>{{ t('layout.links.app_store') }}</span>
                  </router-link>
                  <router-link v-if="can('/docker/apps/installed')" to="/docker/apps/installed" class="sidebar-sub-link" active-class="sidebar-sub-link-active">
                    <span>{{ t('layout.links.installed_apps') }}</span>
                  </router-link>
                  <router-link v-if="can('/docker/apps/packages')" to="/docker/apps/packages" class="sidebar-sub-link" active-class="sidebar-sub-link-active">
                    <span>{{ t('layout.links.docker_packages') }}</span>
                  </router-link>
                </div>
              </transition>

              <router-link v-if="can('/cron-jobs')" to="/cron-jobs" class="sidebar-sub-link" active-class="sidebar-sub-link-active">
                <span>{{ t('layout.links.cron_jobs') }}</span>
              </router-link>
              <router-link v-if="can('/federated')" to="/federated" class="sidebar-sub-link" active-class="sidebar-sub-link-active">
                <span>{{ t('routes.Federated') }}</span>
              </router-link>
              <router-link v-if="can('/ops-center')" to="/ops-center" class="sidebar-sub-link" active-class="sidebar-sub-link-active">
                <span>{{ t('routes.OpsCenter') }}</span>
              </router-link>
            </div>
          </transition>
        </div>

        <div v-if="canSystemGroup" class="mt-2">
          <button @click="toggleTopLevelMenu('system')" class="sidebar-link w-full justify-between" :class="{ 'sidebar-link-section-active': isSystemRoute }">
            <div class="flex items-center">
              <Settings2 class="w-5 h-5 mr-3" />
              <span>{{ t('layout.groups.system') }}</span>
            </div>
            <ChevronDown class="w-4 h-4 transition-transform duration-200" :class="{ 'rotate-180': systemMenuOpen }" />
          </button>

          <transition name="accordion">
            <div v-show="systemMenuOpen" class="ml-4 mt-1 space-y-0.5 border-l border-panel-border/50 pl-3">
              <router-link v-if="can('/server-status')" to="/server-status" class="sidebar-sub-link" active-class="sidebar-sub-link-active">
                <span>{{ t('routes.ServerStatus') }}</span>
              </router-link>
              <router-link v-if="can('/cloudlinux')" to="/cloudlinux" class="sidebar-sub-link" active-class="sidebar-sub-link-active">
                <span>{{ t('routes.CloudLinux') }}</span>
              </router-link>
              <router-link v-if="can('/panel-control')" to="/panel-control" class="sidebar-sub-link" active-class="sidebar-sub-link-active">
                <span>{{ t('layout.links.panel_port') }}</span>
              </router-link>
              <router-link v-if="can('/panel-update')" to="/panel-update" class="sidebar-sub-link" active-class="sidebar-sub-link-active">
                <span>{{ t('panel_update.title') }}</span>
              </router-link>
              <router-link v-if="can('/ai-tools')" to="/ai-tools" class="sidebar-sub-link" active-class="sidebar-sub-link-active">
                <span>{{ t('routes.AITools') }}</span>
              </router-link>
              <router-link v-if="can('/api-settings')" to="/api-settings" class="sidebar-sub-link" active-class="sidebar-sub-link-active">
                <span>{{ t('routes.ApiSettings') }}</span>
              </router-link>
            </div>
          </transition>
        </div>
      </nav>

      <div class="p-4 border-t border-panel-border text-sm text-gray-400">
        <div class="flex items-center gap-2 mb-2">
          <ShieldAlert class="w-4 h-4 text-brand-500" />
          <span>{{ t('layout.footer.zero_trust') }}</span>
        </div>
        <div>{{ t('layout.footer.server_load') }}: <span class="text-brand-400">0.45</span></div>
      </div>
    </aside>

    <!-- Main Content -->
    <main class="flex-1 flex flex-col min-w-0 overflow-hidden">
      <!-- Topbar -->
      <header class="h-16 bg-panel-dark/50 backdrop-blur-md border-b border-panel-border flex items-center justify-between px-8 sticky top-0 z-10">
        <div class="flex items-center">
          <h1 class="text-xl font-semibold text-white">{{ pageTitle }}</h1>
        </div>
        
        <div class="flex items-center gap-4">
          <LanguageSwitcher />
          <div v-if="canPanelUpdateAccess" class="flex items-center gap-2">
            <button
              class="inline-flex items-center gap-1.5 rounded-lg border border-panel-border bg-panel-card/70 px-3 py-1.5 text-xs font-semibold text-gray-200 transition hover:border-brand-400/40 hover:text-white disabled:cursor-not-allowed disabled:opacity-60"
              :disabled="releaseCheckLoading"
              @click="checkHeaderUpdateStatus()"
            >
              <RefreshCw class="h-3.5 w-3.5" :class="{ 'animate-spin': releaseCheckLoading }" />
              <span>{{ releaseCheckLoading ? t('layout.update_notice.checking') : t('layout.update_notice.check') }}</span>
            </button>
            <button
              v-if="headerUpdateAvailable"
              class="inline-flex items-center gap-1.5 rounded-lg border border-amber-500/40 bg-amber-500/15 px-3 py-1.5 text-xs font-semibold text-amber-200 transition hover:bg-amber-500/25"
              @click="openPanelUpdatePage"
            >
              <ArrowUpCircle class="h-3.5 w-3.5" />
              <span>{{ t('layout.update_notice.new_version') }}</span>
            </button>
          </div>

          <div class="relative">
            <button
              class="relative text-gray-400 hover:text-white transition-colors"
              @click="notificationOpen = !notificationOpen"
              :title="t('layout.notifications.title')"
            >
              <Bell class="w-5 h-5" />
              <span
                v-if="unreadCount > 0"
                class="absolute -top-2 -right-2 min-w-[18px] h-[18px] px-1 rounded-full bg-red-500 text-white text-[10px] leading-[18px] text-center font-semibold"
              >
                {{ unreadCount > 99 ? '99+' : unreadCount }}
              </span>
            </button>

            <div
              v-show="notificationOpen"
              class="absolute right-0 mt-3 w-[360px] bg-panel-card border border-panel-border rounded-xl shadow-2xl z-50 overflow-hidden"
            >
              <div class="px-4 py-3 border-b border-panel-border flex items-center justify-between">
                <div>
                  <p class="text-sm font-semibold text-white">{{ t('layout.notifications.title') }}</p>
                  <p class="text-xs text-gray-400">{{ t('layout.notifications.unread', { count: unreadCount }) }}</p>
                </div>
                <div class="flex items-center gap-2">
                  <button class="text-xs text-brand-400 hover:text-brand-300 transition-colors" @click="markAllNotificationsRead">
                    {{ t('layout.notifications.mark_all_read') }}
                  </button>
                  <button class="text-xs text-red-400 hover:text-red-300 transition-colors" @click="clearNotifications">
                    {{ t('layout.notifications.clear') }}
                  </button>
                </div>
              </div>

              <div v-if="notifications.length === 0" class="px-4 py-8 text-center text-sm text-gray-500">
                {{ t('layout.notifications.empty') }}
              </div>

              <div v-else class="max-h-[380px] overflow-auto">
                <button
                  v-for="item in notifications"
                  :key="item.id"
                  class="w-full text-left px-4 py-3 border-b border-panel-border/60 hover:bg-panel-dark/60 transition-colors"
                  @click="openNotification(item.id)"
                >
                  <div class="flex items-start gap-3">
                    <span class="mt-1.5 w-2 h-2 rounded-full" :class="notificationDotClass(item.type)"></span>
                    <div class="min-w-0 flex-1">
                      <p class="text-sm font-medium text-white truncate">{{ item.title }}</p>
                      <p class="text-xs text-gray-400 mt-0.5 break-words">{{ item.message }}</p>
                      <p class="text-[11px] text-gray-500 mt-1">{{ formatNotificationTime(item.createdAt) }}</p>
                    </div>
                    <span v-if="!item.read" class="text-[10px] text-brand-400 font-semibold">{{ t('layout.notifications.new') }}</span>
                  </div>
                </button>
              </div>
            </div>
          </div>

          <div class="flex items-center gap-3 pl-5 border-l border-panel-border relative group">
            <div class="w-8 h-8 rounded-full bg-gradient-to-tr from-brand-600 to-brand-400 flex items-center justify-center text-sm font-bold text-white shadow-lg">
              {{ avatarInitial }}
            </div>
            <div class="text-sm cursor-pointer" @click="toggleMenu = !toggleMenu">
              <p class="font-medium text-white">{{ displayName }}</p>
              <p class="text-xs text-gray-500">{{ authStore.user ? authStore.user.email : 'root@server' }}</p>
            </div>
            <ChevronDown class="w-4 h-4 text-gray-500 cursor-pointer" @click="toggleMenu = !toggleMenu" />
            
            <!-- Dropdown -->
            <div v-show="toggleMenu" class="absolute top-12 right-0 w-48 bg-panel-card border border-panel-border rounded-lg shadow-xl py-2 z-50">
              <button @click="handleLogout" class="w-full text-left px-4 py-2 text-sm text-red-400 hover:bg-panel-dark transition-colors">
                {{ t('layout.user_menu.secure_logout') }}
              </button>
            </div>
          </div>
        </div>
      </header>

      <div
        v-if="globalRequestBusy"
        class="fixed right-6 top-20 z-[90] inline-flex items-center gap-2 rounded-xl border border-brand-500/30 bg-panel-card/95 px-3 py-2 text-sm text-brand-200 shadow-2xl backdrop-blur-md"
      >
        <Loader2 class="h-4 w-4 animate-spin text-brand-400" />
        <span>{{ globalRequestMessage }}</span>
        <span v-if="globalRequestCount > 1" class="text-xs text-brand-300/80">({{ globalRequestCount }})</span>
      </div>

      <!-- Page Content -->
      <div class="flex-1 overflow-auto p-8">
        <div class="max-w-7xl mx-auto">
          <router-view v-slot="{ Component }">
            <transition name="fade" mode="out-in">
              <component :is="Component" />
            </transition>
          </router-view>
        </div>
      </div>
    </main>

    <Teleport to="body">
      <div v-if="commandOpen" class="fixed inset-0 z-[120] bg-black/60 p-4" @click.self="commandOpen = false">
        <div class="mx-auto max-w-2xl bg-panel-card border border-panel-border rounded-2xl shadow-2xl overflow-hidden">
          <div class="p-4 border-b border-panel-border">
            <input
              v-model="commandQuery"
              class="aura-input"
              :placeholder="t('layout.command_palette.placeholder')"
            />
          </div>
          <div class="max-h-96 overflow-auto p-2 space-y-1">
            <button
              v-for="item in filteredCommandItems"
              :key="item.path"
              class="w-full text-left px-3 py-2 rounded-lg hover:bg-panel-dark transition text-sm flex items-center justify-between"
              @click="openCommandRoute(item.path)"
            >
              <span>{{ item.label }}</span>
              <span class="text-xs text-gray-500">{{ item.path }}</span>
            </button>
          </div>
        </div>
      </div>
    </Teleport>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onBeforeUnmount, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter, useRoute } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import { useNotificationStore } from '../stores/notifications'
import { useRequestStateStore } from '../stores/requestState'
import { canAccessPath } from '../security/rbac'
import api from '../services/api'
import LanguageSwitcher from '../components/LanguageSwitcher.vue'
import {
  Activity,
  LayoutDashboard,
  Globe,
  Database,
  Mail,
  Users,
  Box,
  Network,
  Container,
  Bell,
  ChevronsDown,
  ChevronsUp,
  ChevronDown,
  ShieldAlert,
  Cloud,
  FolderOpen,
  Code,
  Shield,
  TerminalSquare,
  PanelTop,
  HardDrive,
  Clock3,
  ScrollText,
  Table2,
  Settings2,
  Lock,
  KeyRound,
  BookOpen,
  DatabaseBackup,
  RefreshCw,
  ArrowUpCircle,
  Loader2
} from 'lucide-vue-next'

const { t, locale } = useI18n({ useScope: 'global' })
const router = useRouter()
const route = useRoute()
const authStore = useAuthStore()
const notificationStore = useNotificationStore()
const requestStateStore = useRequestStateStore()
const toggleMenu = ref(false)
const hostingMenuOpen = ref(false)
const webAppsMenuOpen = ref(false)
const dataAccessMenuOpen = ref(false)
const tuningMenuOpen = ref(false)
const securityLogsMenuOpen = ref(false)
const devopsMenuOpen = ref(false)
const systemMenuOpen = ref(false)
const dockerMenuOpen = ref(false)
const securityMenuOpen = ref(false)
const sslMenuOpen = ref(false)
const ftpMenuOpen = ref(false)
const backupsMenuOpen = ref(false)
const logsMenuOpen = ref(false)
const commandOpen = ref(false)
const commandQuery = ref('')
const notificationOpen = ref(false)
const sidebarServerIp = ref('')
const sidebarIpLoading = ref(false)
const releaseCheckLoading = ref(false)
const headerUpdateStatus = ref({
  update_available: false,
  latest_version: '',
  checked_at: '',
})
let releaseStatusInterval = null
const headerUpdateStatusEventName = 'aurapanel:update-status'

const normalizeUpdateAvailable = (value) => {
  const normalized = String(value ?? '').trim().toLowerCase()
  return value === true || value === 1 || normalized === '1' || normalized === 'true' || normalized === 'yes' || normalized === 'on'
}

const normalizeHeaderUpdateStatus = (payload = {}) => ({
  update_available: normalizeUpdateAvailable(payload.update_available),
  latest_version: String(payload.latest_version || ''),
  checked_at: String(payload.checked_at || ''),
})

const notifications = computed(() => notificationStore.orderedItems.slice(0, 50))
const unreadCount = computed(() => notificationStore.unreadCount)
const globalRequestBusy = computed(() => requestStateStore.isBusy)
const globalRequestCount = computed(() => requestStateStore.pendingCount)
const globalRequestMessage = computed(() => {
  const key = requestStateStore.currentKey || 'processing'
  return t(`request_state.${key}`)
})

const routeTitleKeys = {
  Dashboard: 'routes.Dashboard',
  Websites: 'routes.Websites',
  WebsiteManage: 'routes.WebsiteManage',
  Packages: 'routes.Packages',
  Users: 'routes.Users',
  Databases: 'routes.Databases',
  Emails: 'routes.Emails',
  FTP: 'routes.FTP',
  SFTP: 'routes.SFTP',
  DNS: 'routes.DNS',
  SSL: 'routes.SSL',
  Security: 'routes.Security',
  AppRuntime: 'routes.AppRuntime',
  WordPressManager: 'routes.WordPressManager',
  PluginSDK: 'routes.PluginSDK',
  MinIO: 'routes.MinIO',
  CronJobs: 'routes.CronJobs',
  LogViewer: 'routes.LogViewer',
  Federated: 'routes.Federated',
  FileManager: 'routes.FileManager',
  PHP: 'routes.PHP',
  ServerStatus: 'routes.ServerStatus',
  CloudLinux: 'routes.CloudLinux',
  PanelPort: 'routes.PanelPort',
  PanelUpdate: 'panel_update.title',
  AITools: 'routes.AITools',
  Backups: 'routes.Backups',
  OlsTuning: 'routes.OlsTuning',
  MailTuning: 'routes.MailTuning',
  Reseller: 'routes.Reseller',
  ActivityLog: 'routes.ActivityLog',
  DbBackup: 'routes.DbBackup',
  'Docker Images': 'routes.Docker Images',
  'Docker Containers': 'routes.Docker Containers',
  'Docker Create': 'routes.Docker Create',
  'Docker App Store': 'routes.Docker App Store',
  'Docker Installed Apps': 'routes.Docker Installed Apps',
  'Docker Packages': 'routes.Docker Packages',
  CloudFlare: 'routes.CloudFlare',
  MigrationWizard: 'routes.MigrationWizard',
  Migration: 'routes.MigrationWizard',
  OpsCenter: 'routes.OpsCenter',
}

const pageTitle = computed(() => {
  const routeName = String(route.name || '')
  const key = routeTitleKeys[routeName]
  if (key) return t(key)
  const metaTitle = typeof route.meta?.title === 'string' ? route.meta.title : ''
  return metaTitle || routeName
})
const displayName = computed(() => {
  const user = authStore.user || {}
  const fallback = String(user.username || user.email || 'Admin').trim()
  const raw = String(user.name || '').trim()
  return raw || fallback
})
const avatarInitial = computed(() => {
  const firstChar = displayName.value.charAt(0).trim()
  return firstChar ? firstChar.toUpperCase() : 'A'
})

const can = (path) => canAccessPath(path, authStore.role, authStore.permissions)
const canPanelUpdateAccess = computed(() => can('/panel-update'))
const headerUpdateAvailable = computed(() => canPanelUpdateAccess.value && normalizeUpdateAvailable(headerUpdateStatus.value.update_available))
const canHostingGroup = computed(() =>
  ['/websites', '/dns', '/filemanager', '/migration', '/packages', '/users', '/reseller'].some((path) => can(path)),
)
const canWebAppsGroup = computed(() =>
  ['/cloudflare', '/wordpress', '/plugins', '/app-runtime', '/php', '/terminal'].some((path) => can(path)),
)
const canDataAccessGroup = computed(() =>
  ['/databases', '/emails', '/minio', '/ftp', '/sftp', '/backups', '/db-backup'].some((path) => can(path)),
)
const canTuningGroup = computed(() =>
  ['/ols-tuning', '/databases', '/ftp', '/php', '/mail-tuning'].some((path) => can(path)),
)
const canSecurityGroup = computed(() =>
  ['/ssl', '/security', '/activity-log', '/log-viewer'].some((path) => can(path)),
)
const canDevopsGroup = computed(() =>
  ['/docker/images', '/docker/containers', '/docker/create', '/docker/apps', '/cron-jobs', '/federated', '/ops-center'].some((path) => can(path)),
)
const canSystemGroup = computed(() =>
  ['/server-status', '/cloudlinux', '/panel-control', '/panel-update', '/ai-tools'].some((path) => can(path)),
)

const commandItems = computed(() => [
  { label: t('routes.Dashboard'), path: '/' },
  { label: t('routes.Websites'), path: '/websites' },
  { label: t('routes.MigrationWizard'), path: '/migration' },
  { label: t('routes.Users'), path: '/users' },
  { label: t('routes.Packages'), path: '/packages' },
  { label: t('routes.Databases'), path: '/databases' },
  { label: t('routes.Emails'), path: '/emails' },
  { label: t('routes.FTP'), path: '/ftp' },
  { label: t('routes.SFTP'), path: '/sftp' },
  { label: t('routes.Backups'), path: '/backups' },
  { label: t('routes.DNS'), path: '/dns' },
  { label: t('layout.links.ssl_manage'), path: '/ssl?tab=manage' },
  { label: t('layout.links.ssl_hostname'), path: '/ssl?tab=hostname' },
  { label: t('layout.links.ssl_mail'), path: '/ssl?tab=mail' },
  { label: t('routes.Security'), path: '/security' },
  { label: t('routes.AppRuntime'), path: '/app-runtime' },
  { label: t('routes.WordPressManager'), path: '/wordpress' },
  { label: t('routes.PluginSDK'), path: '/plugins' },
  { label: t('routes.MinIO'), path: '/minio' },
  { label: t('routes.CronJobs'), path: '/cron-jobs' },
  { label: t('routes.LogViewer'), path: '/log-viewer' },
  { label: t('routes.Federated'), path: '/federated' },
  { label: t('routes.FileManager'), path: '/filemanager' },
  { label: t('routes.OpsCenter'), path: '/ops-center' },
  { label: t('routes.PHP'), path: '/php' },
  { label: t('routes.OlsTuning'), path: '/ols-tuning' },
  { label: t('layout.links.mariadb_tuning'), path: '/databases?tab=tuning&tuning_engine=mariadb' },
  { label: t('layout.links.postgresql_tuning'), path: '/databases?tab=tuning&tuning_engine=postgresql' },
  { label: t('layout.links.ftp_tuning'), path: '/ftp?tab=tuning' },
  { label: t('layout.links.php_config'), path: '/php?tab=config' },
  { label: t('routes.MailTuning'), path: '/mail-tuning' },
  { label: t('layout.links.reseller_acl'), path: '/reseller' },
  { label: t('routes.ServerStatus'), path: '/server-status' },
  { label: t('routes.CloudLinux'), path: '/cloudlinux' },
  { label: t('layout.links.panel_port'), path: '/panel-control' },
  { label: t('panel_update.title'), path: '/panel-update' },
  { label: t('routes.AITools'), path: '/ai-tools' },
  { label: t('routes.ApiSettings'), path: '/api-settings' },
  { label: t('routes.Docker Images'), path: '/docker/images' },
  { label: t('routes.Docker Containers'), path: '/docker/containers' },
  { label: t('routes.Docker App Store'), path: '/docker/apps' },
  { label: t('routes.ActivityLog'), path: '/activity-log' },
  { label: t('routes.DbBackup'), path: '/db-backup' },
])
const roleFilteredCommandItems = computed(() => commandItems.value.filter((item) => can(item.path)))

const isDatabaseTuningRoute = computed(() => route.path === '/databases' && route.query.tab === 'tuning')
const isFtpTuningRoute = computed(() => route.path === '/ftp' && route.query.tab === 'tuning')
const isPhpTuningRoute = computed(() => route.path === '/php' && route.query.tab === 'config')
const isDockerRoute = computed(() => route.path.startsWith('/docker'))
const isSecurityRoute = computed(() => route.path.startsWith('/security'))
const isSslRoute = computed(() => route.path.startsWith('/ssl'))
const isFtpRoute = computed(() => route.path === '/sftp' || (route.path === '/ftp' && !isFtpTuningRoute.value))
const isBackupsRoute = computed(() => route.path === '/backups' || route.path === '/db-backup')
const isLogsRoute = computed(() => route.path === '/activity-log' || route.path === '/log-viewer')
const isHostingRoute = computed(() =>
  ['/websites', '/dns', '/filemanager', '/migration', '/packages', '/users', '/reseller'].some(prefix => route.path.startsWith(prefix)),
)
const isWebAppsRoute = computed(() =>
  ['/cloudflare', '/wordpress', '/plugins', '/app-runtime', '/php', '/terminal'].some(prefix => route.path.startsWith(prefix))
    && !isPhpTuningRoute.value
)
const isDataAccessRoute = computed(() =>
  ['/databases', '/emails', '/ftp', '/sftp', '/backups', '/db-backup', '/minio'].some(prefix => route.path.startsWith(prefix))
    && !isDatabaseTuningRoute.value
    && !isFtpTuningRoute.value
)
const isTuningRoute = computed(() =>
  route.path === '/ols-tuning' ||
  route.path === '/mail-tuning' ||
  isDatabaseTuningRoute.value ||
  isFtpTuningRoute.value ||
  isPhpTuningRoute.value
)
const isSecurityLogsRoute = computed(() =>
  ['/ssl', '/security', '/activity-log', '/log-viewer'].some(prefix => route.path.startsWith(prefix))
)
const isDevopsRoute = computed(() =>
  ['/docker', '/cron-jobs', '/federated', '/ops-center'].some(prefix => route.path.startsWith(prefix))
)
const isSystemRoute = computed(() =>
  ['/server-status', '/cloudlinux', '/panel-control', '/panel-port', '/panel-update', '/ai-tools'].some(prefix => route.path.startsWith(prefix))
)
const topLevelMenus = {
  hosting: hostingMenuOpen,
  webApps: webAppsMenuOpen,
  dataAccess: dataAccessMenuOpen,
  tuning: tuningMenuOpen,
  securityLogs: securityLogsMenuOpen,
  devops: devopsMenuOpen,
  system: systemMenuOpen,
}
const activeTopLevelMenu = computed(() => {
  if (isHostingRoute.value) return 'hosting'
  if (isWebAppsRoute.value) return 'webApps'
  if (isDataAccessRoute.value) return 'dataAccess'
  if (isTuningRoute.value) return 'tuning'
  if (isSecurityLogsRoute.value) return 'securityLogs'
  if (isDevopsRoute.value) return 'devops'
  if (isSystemRoute.value) return 'system'
  return ''
})
const allMenusExpanded = computed(() =>
  hostingMenuOpen.value &&
  webAppsMenuOpen.value &&
  dataAccessMenuOpen.value &&
  tuningMenuOpen.value &&
  securityLogsMenuOpen.value &&
  devopsMenuOpen.value &&
  systemMenuOpen.value &&
  dockerMenuOpen.value &&
  securityMenuOpen.value &&
  sslMenuOpen.value &&
  ftpMenuOpen.value &&
  backupsMenuOpen.value &&
  logsMenuOpen.value
)
const isSslTabActive = (tab) => {
  if (!isSslRoute.value) return false
  const selectedTab = typeof route.query.tab === 'string' ? route.query.tab : 'manage'
  return selectedTab === tab
}
const isDatabaseTuningActive = (value) => {
  if (!isDatabaseTuningRoute.value) return false
  const selected = typeof route.query.tuning_engine === 'string' ? route.query.tuning_engine : 'mariadb'
  return selected === value
}
const filteredCommandItems = computed(() => {
  const q = commandQuery.value.trim().toLowerCase()
  if (!q) return roleFilteredCommandItems.value
  return roleFilteredCommandItems.value.filter(i => i.label.toLowerCase().includes(q) || i.path.toLowerCase().includes(q))
})

const toggleTopLevelMenu = (targetKey) => {
  const targetMenu = topLevelMenus[targetKey]
  if (!targetMenu) return

  const nextState = !targetMenu.value
  Object.values(topLevelMenus).forEach((menuRef) => {
    menuRef.value = false
  })
  targetMenu.value = nextState
}

const syncMenuState = () => {
  const activeMenu = activeTopLevelMenu.value
  if (activeMenu) {
    Object.entries(topLevelMenus).forEach(([key, menuRef]) => {
      menuRef.value = key === activeMenu
    })
  }
  if (isDockerRoute.value) {
    dockerMenuOpen.value = true
    devopsMenuOpen.value = true
  }
  if (isSecurityRoute.value) {
    securityMenuOpen.value = true
    securityLogsMenuOpen.value = true
  }
  if (isSslRoute.value) {
    sslMenuOpen.value = true
    webAppsMenuOpen.value = true
  }
  if (isFtpRoute.value) {
    ftpMenuOpen.value = true
    dataAccessMenuOpen.value = true
  }
  if (isBackupsRoute.value) {
    backupsMenuOpen.value = true
    dataAccessMenuOpen.value = true
  }
  if (isTuningRoute.value) {
    tuningMenuOpen.value = true
  }
  if (isLogsRoute.value) {
    logsMenuOpen.value = true
    securityLogsMenuOpen.value = true
  }
}

syncMenuState()

const setAllMenus = (value) => {
  hostingMenuOpen.value = value
  webAppsMenuOpen.value = value
  dataAccessMenuOpen.value = value
  tuningMenuOpen.value = value
  securityLogsMenuOpen.value = value
  devopsMenuOpen.value = value
  systemMenuOpen.value = value
  dockerMenuOpen.value = value
  securityMenuOpen.value = value
  sslMenuOpen.value = value
  ftpMenuOpen.value = value
  backupsMenuOpen.value = value
  logsMenuOpen.value = value
}

const toggleAllMenus = () => {
  setAllMenus(!allMenusExpanded.value)
}

const handleLogout = async () => {
  await authStore.secureLogout()
  router.push('/login')
}

const notificationDotClass = (type) => {
  if (type === 'success') return 'bg-green-400'
  if (type === 'warning') return 'bg-yellow-400'
  if (type === 'error') return 'bg-red-400'
  return 'bg-blue-400'
}

const formatNotificationTime = (timestamp) => {
  const value = Number(timestamp || 0)
  if (!value) return '-'
  const diffMs = Date.now() - value
  const diffMin = Math.floor(diffMs / 60000)
  const formatter = new Intl.RelativeTimeFormat(locale.value, { numeric: 'auto' })
  if (diffMin < 1) return formatter.format(0, 'minute')
  if (diffMin < 60) return formatter.format(-diffMin, 'minute')
  const diffHour = Math.floor(diffMin / 60)
  if (diffHour < 24) return formatter.format(-diffHour, 'hour')
  const diffDay = Math.floor(diffHour / 24)
  if (diffDay < 7) return formatter.format(-diffDay, 'day')
  return new Date(value).toLocaleString(locale.value)
}

const openNotification = (id) => {
  notificationStore.markRead(id)
}

const markAllNotificationsRead = () => {
  notificationStore.markAllRead()
}

const clearNotifications = () => {
  notificationStore.clearAll()
  notificationOpen.value = false
}

const fetchSidebarServerIp = async () => {
  sidebarIpLoading.value = true
  try {
    const silentLoadingConfig = { headers: { 'X-Aura-Silent-Loading': '1' } }
    const res = await api.get('/status/metrics', silentLoadingConfig)
    let rawIp = String(res.data?.data?.server_ip || '').trim()
    if (!rawIp) {
      const sec = await api.get('/security/status', silentLoadingConfig)
      rawIp = String(sec.data?.data?.server_ip || '').trim()
    }
    sidebarServerIp.value = rawIp
  } catch (err) {
    // Sidebar server IP is best-effort and should not block layout rendering.
    sidebarServerIp.value = ''
  } finally {
    sidebarIpLoading.value = false
  }
}

const checkHeaderUpdateStatus = async ({ silent = false, force = true } = {}) => {
  if (!canPanelUpdateAccess.value) return
  if (!silent) releaseCheckLoading.value = true
  try {
    const baseHeaders = silent ? { 'X-Aura-Silent-Loading': '1', 'X-Aura-Silent-Error': '1' } : {}
    const config = {
      params: {
        _ts: Date.now(),
        ...(force ? { force: 1 } : {}),
      },
      ...(Object.keys(baseHeaders).length ? { headers: baseHeaders } : {}),
    }
    const res = await api.get('/status/update', config)
    if (res.data?.status === 'success' && res.data?.data) {
      const next = normalizeHeaderUpdateStatus(res.data.data)
      headerUpdateStatus.value = {
        ...headerUpdateStatus.value,
        ...next,
      }
    }
  } catch (err) {
    // Header notice is best-effort; Panel Update page shows detailed errors.
  } finally {
    if (!silent) releaseCheckLoading.value = false
  }
}

const onHeaderUpdateStatusEvent = (event) => {
  const next = normalizeHeaderUpdateStatus(event?.detail || {})
  headerUpdateStatus.value = {
    ...headerUpdateStatus.value,
    ...next,
  }
}

const openPanelUpdatePage = () => {
  router.push('/panel-update')
}

const onKeydown = (e) => {
  if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'k') {
    e.preventDefault()
    commandOpen.value = !commandOpen.value
    return
  }
  if (e.key === 'Escape' && commandOpen.value) {
    commandOpen.value = false
    return
  }
  if (e.key === 'Escape' && notificationOpen.value) {
    notificationOpen.value = false
  }
}

const openCommandRoute = (path) => {
  commandOpen.value = false
  router.push(path)
}

watch(() => route.fullPath, () => {
  toggleMenu.value = false
  notificationOpen.value = false
  syncMenuState()
})

onMounted(() => {
  window.addEventListener('keydown', onKeydown)
  window.addEventListener(headerUpdateStatusEventName, onHeaderUpdateStatusEvent)
  fetchSidebarServerIp()
  if (canPanelUpdateAccess.value) {
    checkHeaderUpdateStatus({ silent: true, force: true })
    releaseStatusInterval = window.setInterval(() => {
      checkHeaderUpdateStatus({ silent: true, force: true })
    }, 5 * 60 * 1000)
  }
})

onBeforeUnmount(() => {
  window.removeEventListener('keydown', onKeydown)
  window.removeEventListener(headerUpdateStatusEventName, onHeaderUpdateStatusEvent)
  if (releaseStatusInterval) {
    window.clearInterval(releaseStatusInterval)
    releaseStatusInterval = null
  }
})
</script>

<style scoped>
.sidebar-link {
  @apply flex items-center px-3 py-2.5 text-sm font-medium rounded-lg text-[#e7e0ce] hover:text-[#fff7e8] hover:bg-panel-card transition-all duration-200;
}

.sidebar-link-active {
  @apply bg-brand-500/10 text-brand-400 hover:bg-brand-500/10 hover:text-brand-400 border border-brand-500/20 shadow-[inset_0_0_12px_rgba(16,185,129,0.1)];
}

.sidebar-link-section-active {
  @apply text-[#fff4dd] bg-white/[0.04];
}

.sidebar-sub-link {
  @apply flex items-center px-2 py-1.5 text-[13px] leading-5 font-medium rounded-md text-[#d9d1bf] hover:text-[#fff7e8] hover:bg-white/5 transition-all duration-150;
}

.sidebar-sub-link-active {
  @apply text-[#fff4dd] bg-white/[0.06] hover:text-[#fff4dd];
}

.accordion-enter-active,
.accordion-leave-active {
  transition: max-height 0.2s ease, opacity 0.2s ease;
  max-height: 300px;
  overflow: hidden;
}

.accordion-enter-from,
.accordion-leave-to {
  max-height: 0;
  opacity: 0;
}

.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.2s ease, transform 0.2s ease;
}

.fade-enter-from,
.fade-leave-to {
  opacity: 0;
  transform: translateY(10px);
}
</style>
