<template>
  <div class="space-y-6">
    <section class="relative overflow-hidden rounded-[32px] border border-panel-border/70 bg-[linear-gradient(135deg,rgba(15,23,42,0.98),rgba(30,41,59,0.94))] p-6 shadow-[0_30px_80px_-45px_rgba(2,6,23,0.95)] sm:p-8">
      <div class="absolute inset-0">
        <div class="absolute -right-16 -top-16 h-56 w-56 rounded-full bg-cyan-500/12 blur-3xl"></div>
        <div class="absolute -bottom-20 left-12 h-56 w-56 rounded-full bg-brand-500/14 blur-3xl"></div>
        <div class="absolute inset-0 bg-[linear-gradient(120deg,rgba(16,185,129,0.14),transparent_38%,rgba(6,182,212,0.14))]"></div>
      </div>

      <div class="relative space-y-8">
        <div class="flex flex-wrap items-start justify-between gap-4">
          <div class="space-y-2">
            <div class="flex flex-wrap items-center gap-2 text-xs uppercase tracking-[0.2em] text-gray-500">
              <span>{{ t('website_manage.title') }}</span>
              <ChevronRight class="h-3.5 w-3.5" />
              <span class="text-brand-300">{{ domain }}</span>
            </div>
            <h1 class="text-3xl font-bold tracking-tight text-white sm:text-4xl">{{ domain }}</h1>
            <p class="max-w-3xl text-sm leading-6 text-gray-300 sm:text-base">
              {{ t('website_manage.launcher_subtitle') }}
            </p>
          </div>

          <div class="flex flex-wrap gap-2">
            <button class="btn-secondary" @click="goBack">
              <ArrowLeft class="h-4 w-4" />
              {{ t('website_manage.back') }}
            </button>
            <button class="btn-secondary" @click="refreshAll">
              <RefreshCw class="h-4 w-4" />
              {{ t('website_manage.refresh') }}
            </button>
          </div>
        </div>

        <div class="flex flex-wrap gap-2">
          <span
            class="inline-flex items-center gap-2 rounded-full border px-3 py-1.5 text-sm font-medium"
            :class="isSuspended ? 'border-amber-400/25 bg-amber-400/10 text-amber-200' : 'border-brand-500/25 bg-brand-500/10 text-brand-100'"
          >
            <span class="h-2 w-2 rounded-full" :class="isSuspended ? 'bg-amber-300' : 'bg-brand-300'"></span>
            {{ t('website_manage.status') }}: {{ isSuspended ? t('website_manage.suspended') : t('website_manage.active') }}
          </span>
          <span
            class="inline-flex items-center gap-2 rounded-full border px-3 py-1.5 text-sm font-medium"
            :class="site.ssl ? 'border-cyan-400/25 bg-cyan-400/10 text-cyan-100' : 'border-amber-400/25 bg-amber-400/10 text-amber-200'"
          >
            <ShieldCheck class="h-4 w-4" />
            {{ t('website_manage.ssl') }}: {{ site.ssl ? t('website_manage.ssl_active') : t('website_manage.ssl_missing') }}
          </span>
          <span class="inline-flex items-center gap-2 rounded-full border border-white/10 bg-white/[0.04] px-3 py-1.5 text-sm font-medium text-gray-200">
            <Server class="h-4 w-4 text-brand-300" />
            PHP {{ form.php_version }}
          </span>
          <span class="inline-flex items-center gap-2 rounded-full border border-white/10 bg-white/[0.04] px-3 py-1.5 text-sm font-medium text-gray-200">
            <Link2 class="h-4 w-4 text-cyan-300" />
            {{ aliases.length }} {{ t('website_manage.summary.aliases') }}
          </span>
        </div>

        <div class="space-y-6">
          <div v-for="group in launcherGroups" :key="group.key" class="space-y-4">
            <div class="flex flex-wrap items-center justify-between gap-3 border-b border-white/10 pb-3">
              <div>
                <p class="text-xs font-semibold uppercase tracking-[0.22em] text-gray-500">{{ group.label }}</p>
                <p class="mt-1 text-sm text-gray-400">{{ group.description }}</p>
              </div>
              <span class="rounded-full border border-white/10 bg-white/[0.04] px-3 py-1 text-xs text-gray-400">
                {{ group.items.length }} {{ t('website_manage.launcher_items') }}
              </span>
            </div>

            <div class="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
              <component
                :is="item.href ? 'a' : 'button'"
                v-for="item in group.items"
                :key="item.key"
                :href="item.href"
                class="group flex min-h-[110px] items-center gap-4 rounded-[24px] border border-white/10 bg-slate-950/35 px-4 py-4 text-left transition-all duration-300 hover:-translate-y-0.5 hover:border-brand-500/35 hover:bg-slate-950/55"
                @click="item.action ? item.action() : null"
              >
                <div class="flex h-14 w-14 shrink-0 items-center justify-center rounded-2xl border border-white/10 bg-white/[0.06]">
                  <component :is="item.icon" class="h-7 w-7" :class="item.iconClass || 'text-brand-300'" />
                </div>
                <div class="min-w-0 flex-1">
                  <p class="truncate text-base font-semibold text-white">{{ item.label }}</p>
                  <p class="mt-1 text-sm leading-5 text-gray-400">{{ item.description }}</p>
                  <p v-if="item.badge" class="mt-2 text-xs font-medium text-brand-200">{{ item.badge }}</p>
                </div>
              </component>
            </div>
          </div>
        </div>
      </div>
    </section>

    <div v-if="error" class="rounded-2xl border border-red-500/25 bg-red-500/10 px-5 py-4 text-sm text-red-200 shadow-[0_20px_50px_-35px_rgba(239,68,68,0.6)]">
      {{ error }}
    </div>

    <div class="grid gap-6 xl:grid-cols-[minmax(0,1.2fr)_minmax(340px,0.8fr)]">
      <div class="space-y-6">
        <section id="profile" class="scroll-mt-24 rounded-[28px] border border-panel-border/70 bg-[linear-gradient(180deg,rgba(30,41,59,0.98),rgba(15,23,42,0.92))] p-6 shadow-[0_28px_80px_-50px_rgba(2,6,23,0.9)]">
          <div class="flex flex-wrap items-start justify-between gap-4">
            <div class="flex items-start gap-4">
              <div class="flex h-12 w-12 items-center justify-center rounded-2xl border border-brand-500/20 bg-brand-500/10 text-brand-200">
                <Settings2 class="h-5 w-5" />
              </div>
              <div>
                <h2 class="text-xl font-semibold text-white">{{ t('website_manage.sections.profile_title') }}</h2>
                <p class="mt-1 text-sm text-gray-400">{{ t('website_manage.sections.profile_body') }}</p>
              </div>
            </div>
            <button class="btn-primary" @click="saveWebsite">
              <Save class="h-4 w-4" />
              {{ t('website_manage.save') }}
            </button>
          </div>

          <div class="mt-6 grid gap-4 md:grid-cols-2">
            <label class="block">
              <span class="mb-2 block text-[11px] font-semibold uppercase tracking-[0.18em] text-gray-500">{{ t('website_manage.owner') }}</span>
              <input v-model="form.owner" class="aura-input" :placeholder="t('website_manage.owner')" />
            </label>

            <label class="block">
              <span class="mb-2 block text-[11px] font-semibold uppercase tracking-[0.18em] text-gray-500">PHP Runtime</span>
              <select v-model="form.php_version" class="aura-input">
                <option v-for="version in phpVersions" :key="version" :value="version">PHP {{ version }}</option>
              </select>
            </label>

            <label class="block">
              <span class="mb-2 block text-[11px] font-semibold uppercase tracking-[0.18em] text-gray-500">{{ t('website_manage.package') }}</span>
              <select v-model="form.package" class="aura-input">
                <option v-for="pkg in packageOptions" :key="`manage-package-${pkg}`" :value="pkg">{{ pkg }}</option>
              </select>
            </label>

            <label class="block">
              <span class="mb-2 block text-[11px] font-semibold uppercase tracking-[0.18em] text-gray-500">{{ t('website_manage.admin_email') }}</span>
              <input v-model="form.email" class="aura-input" :placeholder="t('website_manage.admin_email')" />
            </label>
          </div>
        </section>

        <section id="server" class="scroll-mt-24 rounded-[28px] border border-panel-border/70 bg-[linear-gradient(180deg,rgba(30,41,59,0.98),rgba(15,23,42,0.92))] p-6 shadow-[0_28px_80px_-50px_rgba(2,6,23,0.9)]">
          <div class="flex items-start gap-4">
            <div class="flex h-12 w-12 items-center justify-center rounded-2xl border border-cyan-500/20 bg-cyan-500/10 text-cyan-100">
              <Server class="h-5 w-5" />
            </div>
            <div>
              <h2 class="text-xl font-semibold text-white">{{ t('website_manage.sections.server_title') }}</h2>
              <p class="mt-1 text-sm text-gray-400">{{ t('website_manage.sections.server_body') }}</p>
            </div>
          </div>

          <div class="mt-6 grid gap-4 xl:grid-cols-2">
            <div class="rounded-2xl border border-panel-border/70 bg-panel-dark/70 p-4">
              <div class="flex flex-wrap items-center justify-between gap-3">
                <h3 class="text-base font-semibold text-white">{{ t('website_manage.rewrite') }}</h3>
                <button class="btn-primary" @click="saveRewrite">
                  <Save class="h-4 w-4" />
                  {{ t('website_manage.save') }}
                </button>
              </div>
              <textarea v-model="advanced.rewrite_rules" rows="10" class="aura-input mt-4 w-full font-mono text-xs"></textarea>
            </div>

            <div class="rounded-2xl border border-panel-border/70 bg-panel-dark/70 p-4">
              <div class="flex flex-wrap items-center justify-between gap-3">
                <h3 class="text-base font-semibold text-white">{{ t('website_manage.vhost_config') }}</h3>
                <button class="btn-primary" @click="saveVhost">
                  <Save class="h-4 w-4" />
                  {{ t('website_manage.save') }}
                </button>
              </div>
              <textarea v-model="advanced.vhost_config" rows="10" class="aura-input mt-4 w-full font-mono text-xs"></textarea>
            </div>
          </div>
        </section>

        <section id="observability" class="scroll-mt-24 rounded-[28px] border border-panel-border/70 bg-[linear-gradient(180deg,rgba(30,41,59,0.98),rgba(15,23,42,0.92))] p-6 shadow-[0_28px_80px_-50px_rgba(2,6,23,0.9)]">
          <div class="flex flex-wrap items-start justify-between gap-4">
            <div class="flex items-start gap-4">
              <div class="flex h-12 w-12 items-center justify-center rounded-2xl border border-brand-500/20 bg-brand-500/10 text-brand-200">
                <Activity class="h-5 w-5" />
              </div>
              <div>
                <h2 class="text-xl font-semibold text-white">{{ t('website_manage.sections.observability_title') }}</h2>
                <p class="mt-1 text-sm text-gray-400">{{ t('website_manage.sections.observability_body') }}</p>
              </div>
            </div>
            <button class="btn-secondary" @click="refreshInsights">
              <RefreshCw class="h-4 w-4" />
              {{ t('website_manage.refresh') }}
            </button>
          </div>

          <div class="mt-6 space-y-5">
            <div class="flex flex-wrap gap-2">
              <button
                v-for="tab in insightTabs"
                :key="tab.key"
                class="inline-flex items-center gap-2 rounded-xl border px-3 py-2 text-sm font-medium transition"
                :class="insightTab === tab.key ? 'border-brand-500/40 bg-brand-500/10 text-brand-100' : 'border-panel-border bg-panel-dark/80 text-gray-300 hover:border-brand-500/30 hover:text-white'"
                @click="insightTab = tab.key"
              >
                <component :is="tab.icon" class="h-4 w-4" />
                {{ tab.label }}
              </button>
            </div>

            <div v-if="insightError" class="rounded-2xl border border-amber-500/30 bg-amber-500/10 px-4 py-3 text-sm text-amber-100">
              {{ insightError }}
            </div>

            <div v-if="insightTab === 'traffic'" class="space-y-5">
              <div class="flex flex-wrap items-center gap-3">
                <label class="text-[11px] font-semibold uppercase tracking-[0.18em] text-gray-500">{{ t('website_manage.insights.range') }}</label>
                <select v-model.number="trafficHours" class="aura-input w-48" @change="loadTraffic">
                  <option v-for="option in trafficRanges" :key="option.value" :value="option.value">{{ option.label }}</option>
                </select>
                <span v-if="traffic?.source_log" class="rounded-full border border-white/10 bg-white/[0.04] px-3 py-1 text-xs text-gray-400">
                  {{ t('website_manage.insights.source') }}: {{ traffic.source_log }}
                </span>
              </div>

              <div class="grid gap-3 md:grid-cols-3">
                <div class="rounded-2xl border border-panel-border/70 bg-panel-dark/70 p-4">
                  <p class="text-[11px] font-semibold uppercase tracking-[0.18em] text-gray-500">{{ t('website_manage.insights.total_hits') }}</p>
                  <p class="mt-3 text-2xl font-semibold text-white">{{ traffic.totals?.hits || 0 }}</p>
                </div>
                <div class="rounded-2xl border border-panel-border/70 bg-panel-dark/70 p-4">
                  <p class="text-[11px] font-semibold uppercase tracking-[0.18em] text-gray-500">{{ t('website_manage.insights.unique_visitors') }}</p>
                  <p class="mt-3 text-2xl font-semibold text-white">{{ traffic.totals?.visitors || 0 }}</p>
                </div>
                <div class="rounded-2xl border border-panel-border/70 bg-panel-dark/70 p-4">
                  <p class="text-[11px] font-semibold uppercase tracking-[0.18em] text-gray-500">{{ t('website_manage.insights.bandwidth') }}</p>
                  <p class="mt-3 text-2xl font-semibold text-white">{{ formatBytes(traffic.totals?.bandwidth_bytes || 0) }}</p>
                </div>
              </div>

              <div class="grid gap-4 xl:grid-cols-[minmax(0,1.15fr)_minmax(320px,0.85fr)]">
                <div class="rounded-2xl border border-panel-border/70 bg-panel-dark/70 p-4">
                  <p class="text-base font-semibold text-white">{{ t('website_manage.insights.hourly_traffic') }}</p>
                  <div v-if="trafficLoading" class="mt-4 text-sm text-gray-400">{{ t('website_manage.insights.loading') }}</div>
                  <div v-else-if="!traffic.series?.length" class="mt-4 text-sm text-gray-500">{{ t('website_manage.insights.no_traffic') }}</div>
                  <div v-else class="mt-4 space-y-4">
                    <div class="flex items-center justify-between gap-3 text-xs text-gray-500">
                      <p>{{ t('website_manage.insights.latest_12_hours') }}</p>
                      <p>{{ recentTrafficSeries.length }}/{{ traffic.series.length }}</p>
                    </div>
                    <div class="grid h-[260px] grid-cols-12 items-end gap-2 rounded-2xl border border-white/10 bg-slate-950/40 p-4">
                      <div
                        v-for="item in recentTrafficSeries"
                        :key="item.bucket"
                        class="flex h-full min-w-0 flex-col justify-end gap-2"
                        :title="`${item.bucket} - ${item.hits} hit`"
                      >
                        <div class="relative flex-1 overflow-hidden rounded-xl bg-slate-950/80">
                          <div
                            class="absolute bottom-0 left-0 right-0 rounded-xl bg-gradient-to-t from-brand-500 to-cyan-400"
                            :style="{ height: `${Math.max(12, Math.round((item.hits / Math.max(1, recentMaxTrafficHit)) * 100))}%` }"
                          ></div>
                        </div>
                        <div class="space-y-1 text-center">
                          <p class="truncate text-[10px] uppercase tracking-[0.12em] text-gray-500">{{ trafficBucketLabel(item.bucket) }}</p>
                          <p class="text-[10px] font-semibold text-gray-200">{{ item.hits }}</p>
                        </div>
                      </div>
                    </div>
                  </div>
                </div>

                <div class="rounded-2xl border border-panel-border/70 bg-panel-dark/70 p-4">
                  <p class="text-base font-semibold text-white">{{ t('website_manage.insights.top_urls') }}</p>
                  <div v-if="!traffic.top_paths?.length" class="mt-4 text-sm text-gray-500">{{ t('website_manage.insights.no_data') }}</div>
                  <div v-else class="mt-4 max-h-[360px] space-y-3 overflow-auto pr-1">
                    <div v-for="row in traffic.top_paths" :key="row.path" class="rounded-2xl border border-white/10 bg-white/[0.03] p-3">
                      <p class="break-all font-mono text-xs text-gray-200">{{ row.path }}</p>
                      <p class="mt-2 text-xs text-gray-500">{{ row.hits }} hit | {{ formatBytes(row.bandwidth_bytes) }}</p>
                    </div>
                  </div>
                </div>
              </div>
            </div>

            <div v-else class="space-y-4">
              <div class="flex flex-wrap gap-2">
                <button
                  class="inline-flex items-center gap-2 rounded-xl border px-3 py-2 text-sm font-medium transition"
                  :class="logKind === 'access' ? 'border-brand-500/40 bg-brand-500/10 text-brand-100' : 'border-panel-border bg-panel-dark/80 text-gray-300 hover:border-brand-500/30 hover:text-white'"
                  @click="changeLogKind('access')"
                >
                  <Globe class="h-4 w-4" />
                  {{ t('website_manage.logs_access') }}
                </button>
                <button
                  class="inline-flex items-center gap-2 rounded-xl border px-3 py-2 text-sm font-medium transition"
                  :class="logKind === 'error' ? 'border-brand-500/40 bg-brand-500/10 text-brand-100' : 'border-panel-border bg-panel-dark/80 text-gray-300 hover:border-brand-500/30 hover:text-white'"
                  @click="changeLogKind('error')"
                >
                  <ScrollText class="h-4 w-4" />
                  {{ t('website_manage.logs_error') }}
                </button>
              </div>

              <pre class="max-h-[420px] overflow-auto whitespace-pre-wrap rounded-2xl border border-panel-border/70 bg-panel-dark/80 p-4 text-xs text-gray-200">{{ logs.join('\n') || t('website_manage.no_logs') }}</pre>
            </div>
          </div>
        </section>
      </div>

      <div class="space-y-6">
        <section id="domain" class="scroll-mt-24 rounded-[28px] border border-panel-border/70 bg-[linear-gradient(180deg,rgba(30,41,59,0.98),rgba(15,23,42,0.92))] p-6 shadow-[0_28px_80px_-50px_rgba(2,6,23,0.9)]">
          <div class="flex items-start gap-4">
            <div class="flex h-12 w-12 items-center justify-center rounded-2xl border border-cyan-500/20 bg-cyan-500/10 text-cyan-100">
              <Globe class="h-5 w-5" />
            </div>
            <div>
              <h2 class="text-xl font-semibold text-white">{{ t('website_manage.sections.domain_title') }}</h2>
              <p class="mt-1 text-sm text-gray-400">{{ t('website_manage.sections.domain_body') }}</p>
            </div>
          </div>

          <div class="mt-6 rounded-2xl border border-panel-border/70 bg-panel-dark/70 p-4">
            <div class="flex flex-wrap items-center justify-between gap-3">
              <h3 class="text-base font-semibold text-white">{{ t('website_manage.alias_title') }}</h3>
              <span class="rounded-full border border-white/10 bg-white/[0.04] px-3 py-1 text-xs text-gray-400">{{ aliases.length }} {{ t('website_manage.summary.aliases') }}</span>
            </div>

            <div class="mt-4 flex flex-col gap-3 sm:flex-row">
              <input v-model="aliasInput" class="aura-input flex-1" :placeholder="t('website_manage.alias_placeholder')" />
              <button class="btn-primary shrink-0" @click="addAlias">
                <Link2 class="h-4 w-4" />
                {{ t('website_manage.add_alias') }}
              </button>
            </div>

            <div class="mt-4 max-h-[300px] space-y-3 overflow-auto pr-1">
              <div
                v-for="alias in aliases"
                :key="alias.alias"
                class="flex flex-wrap items-center justify-between gap-3 rounded-2xl border border-white/10 bg-white/[0.03] px-4 py-3"
              >
                <div class="min-w-0">
                  <p class="truncate text-sm font-semibold text-white">{{ alias.alias }}</p>
                  <p class="mt-1 text-xs text-gray-500">{{ domain }}</p>
                </div>
                <button class="btn-danger px-3 py-2" @click="deleteAlias(alias.alias)">
                  {{ t('common.delete') }}
                </button>
              </div>

              <div v-if="aliases.length === 0" class="rounded-2xl border border-dashed border-white/10 bg-white/[0.02] px-4 py-5 text-sm text-gray-500">
                {{ t('website_manage.insights.no_data') }}
              </div>
            </div>
          </div>
        </section>

        <section id="security" class="scroll-mt-24 rounded-[28px] border border-panel-border/70 bg-[linear-gradient(180deg,rgba(30,41,59,0.98),rgba(15,23,42,0.92))] p-6 shadow-[0_28px_80px_-50px_rgba(2,6,23,0.9)]">
          <div class="flex items-start gap-4">
            <div class="flex h-12 w-12 items-center justify-center rounded-2xl border border-emerald-500/20 bg-emerald-500/10 text-emerald-100">
              <Lock class="h-5 w-5" />
            </div>
            <div>
              <h2 class="text-xl font-semibold text-white">{{ t('website_manage.sections.security_title') }}</h2>
              <p class="mt-1 text-sm text-gray-400">{{ t('website_manage.sections.security_body') }}</p>
            </div>
          </div>

          <div class="mt-6 space-y-4">
            <div class="rounded-2xl border border-panel-border/70 bg-panel-dark/70 p-4">
              <div class="flex flex-wrap items-start justify-between gap-3">
                <div>
                  <h3 class="text-base font-semibold text-white">{{ t('website_manage.open_basedir') }}</h3>
                  <p class="mt-1 text-sm text-gray-400">{{ t('website_manage.enabled') }} filesystem izolasyonu</p>
                </div>
                <span
                  class="rounded-full border px-3 py-1 text-xs font-semibold uppercase tracking-[0.16em]"
                  :class="advanced.open_basedir ? 'border-brand-500/30 bg-brand-500/10 text-brand-100' : 'border-white/10 bg-white/[0.04] text-gray-400'"
                >
                  {{ advanced.open_basedir ? t('website_manage.enabled') : 'Off' }}
                </span>
              </div>

              <label class="mt-4 flex items-center gap-3 rounded-2xl border border-white/10 bg-white/[0.03] px-4 py-3 text-sm text-gray-200">
                <input v-model="advanced.open_basedir" type="checkbox" class="h-4 w-4 rounded border-panel-border bg-panel-dark text-brand-500 focus:ring-brand-500/50" />
                {{ t('website_manage.enabled') }}
              </label>

              <button class="btn-primary mt-4" @click="saveOpenBasedir">
                <Save class="h-4 w-4" />
                {{ t('website_manage.save') }}
              </button>
            </div>

            <div class="rounded-2xl border border-panel-border/70 bg-panel-dark/70 p-4">
              <div class="flex flex-wrap items-center justify-between gap-3">
                <div>
                  <h3 class="text-base font-semibold text-white">{{ t('website_manage.custom_ssl') }}</h3>
                  <p class="mt-1 text-sm text-gray-400">PEM formatinda sertifika ve private key girin.</p>
                </div>
                <button class="btn-primary" @click="saveCustomSsl">
                  <ShieldCheck class="h-4 w-4" />
                  {{ t('website_manage.save') }}
                </button>
              </div>

              <label class="mt-4 block">
                <span class="mb-2 block text-[11px] font-semibold uppercase tracking-[0.18em] text-gray-500">Certificate</span>
                <textarea v-model="customSsl.cert_pem" rows="6" class="aura-input w-full font-mono text-xs" placeholder="-----BEGIN CERTIFICATE-----"></textarea>
              </label>

              <label class="mt-4 block">
                <span class="mb-2 block text-[11px] font-semibold uppercase tracking-[0.18em] text-gray-500">Private Key</span>
                <textarea v-model="customSsl.key_pem" rows="6" class="aura-input w-full font-mono text-xs" placeholder="-----BEGIN PRIVATE KEY-----"></textarea>
              </label>
            </div>
          </div>
        </section>
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import {
  Activity,
  ArrowLeft,
  Box,
  ChevronRight,
  Globe,
  Link2,
  Lock,
  PauseCircle,
  PlayCircle,
  RefreshCw,
  Save,
  ScrollText,
  Server,
  Settings2,
  ShieldCheck,
  User,
} from 'lucide-vue-next'
import api from '../services/api'

const { t } = useI18n({ useScope: 'global' })
const route = useRoute()
const router = useRouter()

const domain = computed(() => String(route.params.domain || '').toLowerCase())
const phpVersions = ref([])
const hostingPackages = ref([])

const error = ref('')
const site = ref({})
const form = ref({ owner: 'aura', php_version: '8.3', package: 'default', email: '' })
const aliases = ref([])
const aliasInput = ref('')
const advanced = ref({ open_basedir: false, rewrite_rules: '', vhost_config: '' })
const customSsl = ref({ cert_pem: '', key_pem: '' })
const logKind = ref('access')
const logs = ref([])
const insightError = ref('')
const insightTab = ref('traffic')
const trafficHours = ref(24)
const trafficLoading = ref(false)
const traffic = ref({
  totals: { hits: 0, visitors: 0, bandwidth_bytes: 0 },
  series: [],
  top_paths: [],
  source_log: '',
})

const adminEmail = computed(() => form.value.email || `webmaster@${domain.value}`)
const isSuspended = computed(() => String(site.value?.status || 'active').toLowerCase() === 'suspended')
const recentTrafficSeries = computed(() => (traffic.value.series || []).slice(-12))
const recentMaxTrafficHit = computed(() => Math.max(1, ...recentTrafficSeries.value.map(item => Number(item.hits || 0))))
const packageOptions = computed(() => {
  const names = new Set(['default'])

  for (const pkg of hostingPackages.value || []) {
    const name = String(pkg?.name || '').trim()
    if (name) names.add(name)
  }

  const currentPackage = String(form.value.package || site.value?.package || '').trim()
  if (currentPackage) names.add(currentPackage)

  const ordered = Array.from(names).filter(Boolean)
  const tail = ordered.filter(name => name !== 'default').sort((a, b) => a.localeCompare(b))
  return ['default', ...tail]
})

const insightRequestConfig = {
  timeout: 30000,
  headers: { 'X-Aura-Silent-Error': '1' },
}

const summaryCards = computed(() => [
  { label: t('website_manage.summary.owner'), value: form.value.owner || 'aura', icon: User, iconClass: 'text-cyan-300' },
  { label: t('website_manage.summary.package'), value: form.value.package || 'default', icon: Box, iconClass: 'text-amber-300' },
  { label: t('website_manage.summary.php'), value: `PHP ${form.value.php_version || '8.3'}`, icon: Server, iconClass: 'text-brand-300' },
  { label: t('website_manage.summary.aliases'), value: String(aliases.value.length), icon: Link2, iconClass: 'text-cyan-300' },
])

const quickActions = computed(() => [
  {
    key: 'save',
    label: t('website_manage.save'),
    description: t('website_manage.quick_action_save'),
    icon: Save,
    handler: saveWebsite,
  },
  {
    key: 'ssl',
    label: t('website_manage.issue_ssl'),
    description: t('website_manage.quick_action_ssl'),
    icon: ShieldCheck,
    handler: issueSsl,
  },
  {
    key: isSuspended.value ? 'resume' : 'suspend',
    label: isSuspended.value ? t('website_manage.unsuspend') : t('website_manage.suspend'),
    description: isSuspended.value ? t('website_manage.quick_action_unsuspend') : t('website_manage.quick_action_suspend'),
    icon: isSuspended.value ? PlayCircle : PauseCircle,
    handler: toggleSuspend,
  },
  {
    key: 'refresh',
    label: t('website_manage.refresh'),
    description: t('website_manage.quick_action_refresh'),
    icon: RefreshCw,
    handler: refreshAll,
  },
])

const sectionLinks = computed(() => [
  { id: 'profile', label: t('website_manage.categories.profile'), icon: Settings2 },
  { id: 'domain', label: t('website_manage.categories.domain'), icon: Globe },
  { id: 'security', label: t('website_manage.categories.security'), icon: Lock },
  { id: 'server', label: t('website_manage.categories.server'), icon: Server },
  { id: 'observability', label: t('website_manage.categories.observability'), icon: Activity },
])

const insightTabs = computed(() => [
  { key: 'traffic', label: t('website_manage.insights.traffic'), icon: Activity },
  { key: 'logs', label: t('website_manage.insights.logs'), icon: ScrollText },
])

const trafficRanges = computed(() => [
  { value: 6, label: t('website_manage.insights.last_6_hours') },
  { value: 24, label: t('website_manage.insights.last_24_hours') },
  { value: 72, label: t('website_manage.insights.last_3_days') },
  { value: 168, label: t('website_manage.insights.last_7_days') },
])

const launcherGroups = computed(() => [
  {
    key: 'site',
    label: t('website_manage.launcher.site_title'),
    description: t('website_manage.launcher.site_body'),
    items: [
      {
        key: 'profile',
        label: t('website_manage.sections.profile_title'),
        description: t('website_manage.launcher.profile_desc'),
        href: '#profile',
        icon: Settings2,
        iconClass: 'text-cyan-300',
      },
      {
        key: 'aliases',
        label: t('website_manage.alias_title'),
        description: t('website_manage.launcher.alias_desc'),
        href: '#domain',
        icon: Link2,
        iconClass: 'text-brand-300',
        badge: `${aliases.value.length} ${t('website_manage.summary.aliases')}`,
      },
      {
        key: 'traffic',
        label: t('website_manage.insights.hourly_traffic'),
        description: t('website_manage.launcher.traffic_desc'),
        href: '#observability',
        icon: Activity,
        iconClass: 'text-cyan-300',
      },
      {
        key: 'logs',
        label: t('website_manage.insights.logs'),
        description: t('website_manage.launcher.logs_desc'),
        href: '#observability',
        icon: ScrollText,
        iconClass: 'text-amber-300',
      },
    ],
  },
  {
    key: 'security',
    label: t('website_manage.launcher.security_title'),
    description: t('website_manage.launcher.security_body'),
    items: [
      {
        key: 'ssl',
        label: t('website_manage.issue_ssl'),
        description: t('website_manage.launcher.ssl_desc'),
        href: '#security',
        icon: ShieldCheck,
      },
      {
        key: 'open_basedir',
        label: t('website_manage.open_basedir'),
        description: t('website_manage.launcher.open_basedir_desc'),
        href: '#security',
        icon: Lock,
        iconClass: 'text-cyan-300',
        badge: advanced.value.open_basedir ? t('website_manage.enabled') : 'Off',
      },
      {
        key: 'custom_ssl',
        label: t('website_manage.custom_ssl'),
        description: t('website_manage.launcher.custom_ssl_desc'),
        href: '#security',
        icon: ShieldCheck,
        iconClass: 'text-brand-300',
      },
    ],
  },
  {
    key: 'server',
    label: t('website_manage.launcher.server_title'),
    description: t('website_manage.launcher.server_body'),
    items: [
      {
        key: 'rewrite',
        label: t('website_manage.rewrite'),
        description: t('website_manage.launcher.rewrite_desc'),
        href: '#server',
        icon: ScrollText,
      },
      {
        key: 'vhost',
        label: t('website_manage.vhost_config'),
        description: t('website_manage.launcher.vhost_desc'),
        href: '#server',
        icon: Server,
        iconClass: 'text-cyan-300',
      },
      {
        key: 'save',
        label: t('website_manage.save'),
        description: t('website_manage.quick_action_save'),
        action: saveWebsite,
        icon: Save,
        iconClass: 'text-brand-300',
      },
      {
        key: 'suspend',
        label: isSuspended.value ? t('website_manage.unsuspend') : t('website_manage.suspend'),
        description: isSuspended.value ? t('website_manage.quick_action_unsuspend') : t('website_manage.quick_action_suspend'),
        action: toggleSuspend,
        icon: isSuspended.value ? PlayCircle : PauseCircle,
        iconClass: 'text-amber-300',
      },
    ],
  },
])

function quickActionClass(key) {
  if (key === 'save') {
    return 'border-brand-400/30 bg-[linear-gradient(135deg,rgba(16,185,129,0.24),rgba(6,182,212,0.12))] shadow-[0_22px_40px_-30px_rgba(16,185,129,0.75)] hover:border-brand-300/60'
  }

  if (key === 'ssl') {
    return 'border-cyan-400/25 bg-[linear-gradient(135deg,rgba(8,145,178,0.22),rgba(15,23,42,0.55))] hover:border-cyan-300/55'
  }

  if (key === 'suspend') {
    return 'border-amber-400/25 bg-[linear-gradient(135deg,rgba(245,158,11,0.22),rgba(15,23,42,0.55))] hover:border-amber-300/55'
  }

  if (key === 'resume') {
    return 'border-brand-400/25 bg-[linear-gradient(135deg,rgba(16,185,129,0.2),rgba(15,23,42,0.55))] hover:border-brand-300/55'
  }

  return 'border-white/10 bg-slate-950/40 hover:border-brand-500/40 hover:bg-slate-950/65'
}

function trafficBucketLabel(bucket) {
  const parts = String(bucket || '').trim().split(/\s+/)
  return parts[parts.length - 1] || bucket
}

function msg(err, fallbackKey) {
  return err?.response?.data?.message || err?.message || t(fallbackKey)
}

async function loadSite() {
  const res = await api.get('/vhost/list', { params: { search: domain.value, page: 1, per_page: 100 } })
  const data = res.data?.data || []
  site.value = data.find(item => String(item.domain || '').toLowerCase() === domain.value) || {}
  form.value = {
    owner: site.value.owner || site.value.user || 'aura',
    php_version: site.value.php_version || site.value.php || '8.3',
    package: site.value.package || 'default',
    email: site.value.email || `webmaster@${domain.value}`,
  }
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

async function loadPackages() {
  try {
    const res = await api.get('/packages/list')
    hostingPackages.value = res.data?.data || []
  } catch {
    hostingPackages.value = []
  }
}

async function loadAliases() {
  const res = await api.get('/websites/aliases', { params: { domain: domain.value } })
  aliases.value = res.data?.data || []
}

async function loadAdvanced() {
  const [cfgRes, sslRes] = await Promise.all([
    api.get('/websites/advanced-config', { params: { domain: domain.value } }),
    api.get('/websites/custom-ssl', { params: { domain: domain.value } }),
  ])
  const cfg = cfgRes.data?.data || {}
  advanced.value = {
    open_basedir: !!cfg.open_basedir,
    rewrite_rules: cfg.rewrite_rules || '',
    vhost_config: cfg.vhost_config || '',
  }
  const ssl = sslRes.data?.data || {}
  customSsl.value = {
    cert_pem: ssl.cert_pem || '',
    key_pem: ssl.key_pem || '',
  }
}

async function loadLogs() {
  const res = await api.get('/monitor/logs/site', {
    ...insightRequestConfig,
    params: { domain: domain.value, kind: logKind.value, lines: 200 },
  })
  logs.value = res.data?.data || []
}

function formatBytes(bytes) {
  const value = Number(bytes || 0)
  if (value <= 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  const index = Math.min(units.length - 1, Math.floor(Math.log(value) / Math.log(1024)))
  const size = value / Math.pow(1024, index)
  return `${size.toFixed(size >= 10 ? 1 : 2)} ${units[index]}`
}

async function loadTraffic() {
  trafficLoading.value = true
  try {
    const res = await api.get('/analytics/website-traffic', {
      ...insightRequestConfig,
      params: { domain: domain.value, hours: trafficHours.value },
    })
    traffic.value = res.data?.data || {
      totals: { hits: 0, visitors: 0, bandwidth_bytes: 0 },
      series: [],
      top_paths: [],
      source_log: '',
    }
  } catch (err) {
    traffic.value = {
      totals: { hits: 0, visitors: 0, bandwidth_bytes: 0 },
      series: [],
      top_paths: [],
      source_log: '',
    }
    throw err
  } finally {
    trafficLoading.value = false
  }
}

async function refreshInsights() {
  insightError.value = ''
  try {
    if (insightTab.value === 'logs') {
      await loadLogs()
    } else {
      await loadTraffic()
    }
  } catch (err) {
    insightError.value = msg(err, 'website_manage.messages.load_failed')
  }
}

async function refreshAll() {
  error.value = ''
  insightError.value = ''
  try {
    await Promise.all([loadSite(), loadAliases(), loadAdvanced(), loadPackages()])
    await refreshInsights()
  } catch (err) {
    error.value = msg(err, 'website_manage.messages.load_failed')
  }
}

async function saveWebsite() {
  error.value = ''
  try {
    await api.post('/vhost/update', {
      domain: domain.value,
      owner: form.value.owner,
      php_version: form.value.php_version,
      package: form.value.package,
      email: form.value.email,
    })
    await loadSite()
  } catch (err) {
    error.value = msg(err, 'website_manage.messages.save_failed')
  }
}

async function toggleSuspend() {
  error.value = ''
  try {
    if (isSuspended.value) {
      await api.post('/vhost/unsuspend', { domain: domain.value })
    } else {
      await api.post('/vhost/suspend', { domain: domain.value })
    }
    await loadSite()
  } catch (err) {
    error.value = msg(err, 'website_manage.messages.status_failed')
  }
}

async function issueSsl() {
  error.value = ''
  try {
    await api.post('/ssl/issue', {
      domain: domain.value,
      email: form.value.email || `admin@${domain.value}`,
      provider: 'letsencrypt',
    })
    await loadSite()
  } catch (err) {
    error.value = msg(err, 'website_manage.messages.ssl_failed')
  }
}

async function addAlias() {
  if (!aliasInput.value) return
  error.value = ''
  try {
    await api.post('/websites/aliases', { domain: domain.value, alias: aliasInput.value })
    aliasInput.value = ''
    await loadAliases()
  } catch (err) {
    error.value = msg(err, 'website_manage.messages.alias_add_failed')
  }
}

async function deleteAlias(alias) {
  error.value = ''
  try {
    await api.delete('/websites/aliases', { params: { domain: domain.value, alias } })
    await loadAliases()
  } catch (err) {
    error.value = msg(err, 'website_manage.messages.alias_delete_failed')
  }
}

async function saveOpenBasedir() {
  error.value = ''
  try {
    await api.post('/websites/open-basedir', { domain: domain.value, enabled: !!advanced.value.open_basedir })
    await loadAdvanced()
  } catch (err) {
    error.value = msg(err, 'website_manage.messages.open_basedir_failed')
  }
}

async function saveRewrite() {
  error.value = ''
  try {
    await api.post('/websites/rewrite', { domain: domain.value, rules: advanced.value.rewrite_rules || '' })
    await loadAdvanced()
  } catch (err) {
    error.value = msg(err, 'website_manage.messages.rewrite_failed')
  }
}

async function saveVhost() {
  error.value = ''
  try {
    await api.post('/websites/vhost-config', { domain: domain.value, content: advanced.value.vhost_config || '' })
    await loadAdvanced()
  } catch (err) {
    error.value = msg(err, 'website_manage.messages.vhost_failed')
  }
}

async function saveCustomSsl() {
  error.value = ''
  try {
    await api.post('/websites/custom-ssl', {
      domain: domain.value,
      cert_pem: customSsl.value.cert_pem || '',
      key_pem: customSsl.value.key_pem || '',
    })
    await loadAdvanced()
  } catch (err) {
    error.value = msg(err, 'website_manage.messages.custom_ssl_failed')
  }
}

async function changeLogKind(kind) {
  logKind.value = kind
  await refreshInsights()
}

function goBack() {
  router.push('/websites')
}

watch(insightTab, async () => {
  await refreshInsights()
})

watch(packageOptions, (options) => {
  if (!options.length) return
  if (!options.includes(form.value.package)) {
    form.value.package = options[0]
  }
}, { immediate: true })

onMounted(() => {
  loadPhpVersions()
  refreshAll()
})
</script>
