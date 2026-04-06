<template>
  <div class="space-y-6">
    <div class="flex items-center justify-between">
      <div>
        <h1 class="text-2xl font-bold text-white">{{ t('security_center.title') }}</h1>
        <p class="mt-1 text-gray-400">{{ t('security_center.subtitle') }}</p>
      </div>
      <button class="btn-secondary" @click="loadAll">{{ t('security_center.refresh') }}</button>
    </div>

    <div class="aura-card">
      <div class="flex flex-wrap gap-2">
        <button
          v-for="tabItem in tabs"
          :key="tabItem.id"
          class="rounded-lg px-3 py-2 text-sm transition"
          :class="activeTab === tabItem.id ? 'border border-brand-500/30 bg-brand-500/20 text-brand-300' : 'border border-panel-border bg-panel-dark text-gray-300 hover:bg-panel-hover'"
          @click="setTab(tabItem.id)"
        >
          {{ tabItem.label }}
        </button>
      </div>
    </div>

    <div v-if="activeTab === 'overview'" class="grid grid-cols-1 gap-4 md:grid-cols-2 lg:grid-cols-3">
      <div v-for="item in statusCards" :key="item.key" class="aura-card">
        <div class="flex items-center justify-between">
          <h3 class="text-sm font-semibold text-gray-300">{{ item.label }}</h3>
          <span :class="item.value ? 'text-green-400' : 'text-yellow-400'">
            {{ item.value ? t('security_center.overview.active') : t('security_center.overview.passive') }}
          </span>
        </div>
      </div>
    </div>

    <div v-if="activeTab === 'firewall'" class="aura-card space-y-4">
      <h2 class="text-lg font-bold text-white">{{ t('security_center.firewall.title') }}</h2>
      <div class="rounded-xl border border-panel-border bg-panel-dark p-4">
        <div class="flex flex-wrap items-center gap-3 text-sm">
          <span class="font-semibold text-white">{{ t('security_center.status_label') }}</span>
          <span :class="status.firewall_active ? 'text-emerald-400' : 'text-yellow-400'">
            {{ status.firewall_active ? t('security_center.overview.active') : t('security_center.passive_or_unknown') }}
          </span>
          <span v-if="status.firewall_manager" class="text-gray-400">{{ t('security_center.manager_label', { manager: status.firewall_manager }) }}</span>
          <span v-if="status.server_ip" class="text-gray-400">{{ t('security_center.server_ip_label', { ip: status.server_ip }) }}</span>
        </div>
        <p v-if="(status.firewall_open_ports || []).length" class="mt-3 text-xs text-gray-400">
          {{ t('security_center.open_ports_label', { ports: status.firewall_open_ports.join(', ') }) }}
        </p>
      </div>
      <div class="grid grid-cols-1 gap-3 md:grid-cols-4">
        <input
          v-model="firewallForm.ip_address"
          class="aura-input"
          :placeholder="t('security_center.firewall.ip_cidr_placeholder')"
        />
        <input v-model="firewallForm.reason" class="aura-input" :placeholder="t('security_center.firewall.reason')" />
        <select v-model="firewallForm.block" class="aura-input">
          <option :value="true">{{ t('security_center.firewall.block_all') }}</option>
          <option :value="false">{{ t('security_center.firewall.allow_all') }}</option>
        </select>
        <button class="btn-primary" @click="addFirewallRule">{{ t('security_center.firewall.add_rule') }}</button>
      </div>
      <p class="text-xs text-gray-400">
        {{ t('security_center.firewall.ip_cidr_hint') }}
      </p>
      <div v-if="firewallError" class="rounded-lg border border-red-500/40 bg-red-500/10 p-3 text-sm text-red-300">
        {{ firewallError }}
      </div>
      <div class="overflow-x-auto">
        <table class="w-full text-sm">
          <thead>
            <tr class="border-b border-panel-border text-gray-400">
              <th class="py-2 text-left">{{ t('security_center.firewall.ip') }}</th>
              <th class="py-2 text-left">{{ t('security_center.firewall.action') }}</th>
              <th class="py-2 text-left">{{ t('security_center.firewall.reason') }}</th>
              <th class="py-2 text-right">{{ t('security_center.firewall.action') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-if="firewallRules.length === 0">
              <td colspan="4" class="py-4 text-center text-gray-400">{{ t('security_center.firewall.no_rules') }}</td>
            </tr>
            <tr v-for="rule in firewallRules" :key="rule.ip_address" class="border-b border-panel-border/60">
              <td class="py-2 font-mono">{{ rule.ip_address }}</td>
              <td class="py-2">{{ rule.block ? t('security_center.firewall.block') : t('security_center.firewall.allow') }}</td>
              <td class="py-2 text-gray-300">{{ rule.reason }}</td>
              <td class="py-2 text-right">
                <button class="btn-danger px-3 py-1 text-xs" @click="deleteFirewallRule(rule.ip_address)">{{ t('security_center.firewall.delete') }}</button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>

      <div class="mt-4 rounded-xl border border-panel-border bg-panel-dark p-4 space-y-3">
        <h3 class="text-base font-semibold text-white">{{ t('security_center.firewall.port_rules_title') }}</h3>
        <div class="grid grid-cols-1 gap-3 md:grid-cols-5">
          <input v-model="firewallPortForm.port" type="number" min="1" max="65535" class="aura-input" :placeholder="t('security_center.firewall.port_placeholder')" />
          <select v-model="firewallPortForm.protocol" class="aura-input">
            <option value="tcp">{{ t('security_center.firewall.protocol_tcp') }}</option>
            <option value="udp">{{ t('security_center.firewall.protocol_udp') }}</option>
          </select>
          <select v-model="firewallPortForm.block" class="aura-input">
            <option :value="false">{{ t('security_center.firewall.allow') }}</option>
            <option :value="true">{{ t('security_center.firewall.block') }}</option>
          </select>
          <input v-model="firewallPortForm.reason" class="aura-input" :placeholder="t('security_center.firewall.reason_optional')" />
          <button class="btn-primary" @click="addFirewallPortRule">{{ t('security_center.firewall.add_port_rule') }}</button>
        </div>
        <p class="text-xs text-gray-400">
          {{ t('security_center.firewall.port_hint') }}
        </p>
        <div v-if="firewallPortError" class="rounded-lg border border-red-500/40 bg-red-500/10 p-3 text-sm text-red-300">
          {{ firewallPortError }}
        </div>
        <div class="overflow-x-auto">
          <table class="w-full text-sm">
            <thead>
              <tr class="border-b border-panel-border text-gray-400">
                <th class="py-2 text-left">{{ t('security_center.firewall.port') }}</th>
                <th class="py-2 text-left">{{ t('security_center.firewall.protocol') }}</th>
                <th class="py-2 text-left">{{ t('security_center.firewall.action') }}</th>
                <th class="py-2 text-left">{{ t('security_center.firewall.reason') }}</th>
                <th class="py-2 text-right">{{ t('common.actions') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-if="firewallPortRules.length === 0">
                <td colspan="5" class="py-4 text-center text-gray-400">{{ t('security_center.firewall.no_port_rules') }}</td>
              </tr>
              <tr
                v-for="rule in firewallPortRules"
                :key="`${rule.port}-${rule.protocol}-${rule.block}`"
                class="border-b border-panel-border/60"
              >
                <td class="py-2 font-mono">{{ rule.port }}</td>
                <td class="py-2 uppercase">{{ rule.protocol }}</td>
                <td class="py-2">{{ rule.block ? t('security_center.firewall.block') : t('security_center.firewall.allow') }}</td>
                <td class="py-2 text-gray-300">{{ rule.reason || '-' }}</td>
                <td class="py-2 text-right">
                  <button class="btn-danger px-3 py-1 text-xs" @click="deleteFirewallPortRule(rule)">{{ t('common.delete') }}</button>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </div>

    <div v-if="activeTab === 'waf'" class="space-y-4">
      <div class="aura-card">
        <h2 class="text-lg font-bold text-white mb-4">{{ t('security_center.waf.manage_title') }}</h2>
        <div class="flex items-center justify-between p-4 rounded-xl border border-panel-border bg-panel-dark">
          <div>
            <h3 class="font-semibold text-white">{{ t('security_center.waf.global_status') }}</h3>
            <p class="text-sm text-gray-400">{{ t('security_center.waf.global_desc') }}</p>
            <p class="mt-2 text-sm" :class="status.ml_waf ? 'text-emerald-400' : 'text-yellow-400'">
              {{ status.ml_waf ? t('common.active') : t('common.inactive') }}
            </p>
          </div>
          <div class="flex gap-2">
            <button class="btn-primary" :disabled="wafStateSaving || !!status.ml_waf" @click="setWafState(true)">
              {{ t('security_center.waf.open') }}
            </button>
            <button class="btn-secondary" :disabled="wafStateSaving || !status.ml_waf" @click="setWafState(false)">
              {{ t('security_center.waf.close') }}
            </button>
          </div>
        </div>
        <div v-if="wafActionMessage" class="mt-3 rounded-lg border border-emerald-500/30 bg-emerald-500/10 p-3 text-sm text-emerald-200">
          {{ wafActionMessage }}
        </div>
        <div v-if="wafActionError" class="mt-3 rounded-lg border border-red-500/40 bg-red-500/10 p-3 text-sm text-red-300">
          {{ wafActionError }}
        </div>
      </div>
      
      <div class="aura-card space-y-4">
        <h2 class="text-lg font-bold text-white">{{ t('security_center.waf.title') }}</h2>
        <div class="grid grid-cols-1 gap-3 md:grid-cols-2">
        <input v-model="wafInput.path" class="aura-input" :placeholder="t('security_center.waf.path')" />
        <input v-model="wafInput.query" class="aura-input" :placeholder="t('security_center.waf.query')" />
        <input v-model="wafInput.user_agent" class="aura-input" :placeholder="t('security_center.waf.user_agent')" />
        <input v-model="wafInput.ip" class="aura-input" :placeholder="t('security_center.waf.ip')" />
      </div>
      <textarea v-model="wafInput.body" class="aura-input min-h-24 w-full" :placeholder="t('security_center.waf.body')"></textarea>
      <button class="btn-primary" @click="runWafTest">{{ t('security_center.waf.analyze') }}</button>
      <div v-if="wafResult" class="rounded-lg border border-panel-border bg-panel-dark p-4 text-sm">
        <p><strong>{{ t('security_center.waf.allowed') }}:</strong> {{ wafResult.allowed }}</p>
        <p><strong>{{ t('security_center.waf.score') }}:</strong> {{ wafResult.score }}</p>
        <p><strong>{{ t('security_center.waf.reason') }}:</strong> {{ wafResult.reason }}</p>
      </div>
      </div>
    </div>

    <div v-if="activeTab === 'fail2ban'" class="space-y-4">
      <div class="aura-card">
        <div class="flex items-center justify-between mb-4">
          <div>
            <h2 class="text-lg font-bold text-white">{{ t('security_center.fail2ban.title') }}</h2>
            <p class="text-sm text-gray-400">{{ t('security_center.fail2ban.desc') }}</p>
          </div>
          <button class="btn-secondary" @click="loadFail2ban" :disabled="fail2banLoading">
            {{ fail2banLoading ? t('common.loading') : t('common.refresh') }}
          </button>
        </div>

        <div class="rounded-xl border border-panel-border bg-panel-dark p-4">
          <div class="flex items-center gap-3 mb-4">
            <div class="w-3 h-3 rounded-full" :class="fail2banStatus.status === 'active' ? 'bg-green-500' : 'bg-red-500'"></div>
            <span class="font-semibold text-white">{{ t('security_center.fail2ban.status_label') }}: {{ fail2banStatus.status === 'active' ? t('common.active') : t('common.inactive') }}</span>
          </div>
          
          <div class="mt-4">
            <h3 class="text-sm font-semibold text-gray-300 mb-2">{{ t('security_center.fail2ban.logs_title') }}</h3>
            <pre class="bg-black/50 p-4 rounded-lg text-xs font-mono text-gray-300 overflow-x-auto whitespace-pre-wrap">{{ fail2banStatus.raw || t('common.no_data') }}</pre>
          </div>
          
          <div class="mt-6 border-t border-panel-border pt-4">
            <h3 class="text-sm font-semibold text-gray-300 mb-3">{{ t('security_center.fail2ban.unban_title') }}</h3>
            <div class="flex gap-3 max-w-md">
              <input type="text" id="unbanIpInput" :placeholder="t('security_center.fail2ban.unban_placeholder')" class="aura-input flex-1" />
              <button class="btn-primary" @click="() => { const el = document.getElementById('unbanIpInput'); if(el.value) unbanIp(el.value); }">{{ t('security_center.fail2ban.unban_btn') }}</button>
            </div>
          </div>
        </div>
      </div>
    </div>

    <div v-if="activeTab === 'ddos'" class="aura-card space-y-4">
      <div class="rounded-xl border border-panel-border bg-panel-dark p-4">
        <div class="flex flex-wrap items-center gap-3 text-sm">
          <span class="font-semibold text-white">{{ t('security_center.ddos.title') }}</span>
          <span :class="ddos.enabled ? 'text-emerald-400' : 'text-yellow-400'">
            {{ ddos.enabled ? t('common.active') : t('common.inactive') }}
          </span>
          <span class="text-gray-400">{{ t('security_center.ddos.profile_label', { profile: ddos.profile }) }}</span>
        </div>
        <p class="mt-3 text-xs text-gray-400">{{ t('security_center.ddos.note') }}</p>
      </div>

      <div class="grid grid-cols-1 gap-3 md:grid-cols-3">
        <label class="rounded-lg border border-panel-border bg-panel-dark p-3 text-sm text-gray-300">
          <span class="mb-2 block text-xs text-gray-400">{{ t('security_center.ddos.enabled') }}</span>
          <select v-model="ddos.enabled" class="aura-input w-full">
            <option :value="true">{{ t('common.active') }}</option>
            <option :value="false">{{ t('common.inactive') }}</option>
          </select>
        </label>
        <label class="rounded-lg border border-panel-border bg-panel-dark p-3 text-sm text-gray-300">
          <span class="mb-2 block text-xs text-gray-400">{{ t('security_center.ddos.profile') }}</span>
          <select v-model="ddos.profile" class="aura-input w-full" :disabled="!ddos.enabled">
            <option value="standard">{{ t('security_center.ddos.profile_standard') }}</option>
            <option value="strict">{{ t('security_center.ddos.profile_strict') }}</option>
          </select>
        </label>
        <div class="flex items-end">
          <button class="btn-primary w-full" :disabled="ddosSaving" @click="saveDdos">
            {{ ddosSaving ? t('common.loading') : t('security_center.ddos.apply') }}
          </button>
        </div>
      </div>

      <div class="grid grid-cols-1 gap-3 md:grid-cols-2">
        <label class="rounded-lg border border-panel-border bg-panel-dark p-3 text-sm text-gray-300">
          <span class="mb-2 block text-xs text-gray-400">{{ t('security_center.ddos.global_rps') }}</span>
          <input v-model.number="ddos.global_rps" min="5" type="number" class="aura-input w-full" />
        </label>
        <label class="rounded-lg border border-panel-border bg-panel-dark p-3 text-sm text-gray-300">
          <span class="mb-2 block text-xs text-gray-400">{{ t('security_center.ddos.global_burst') }}</span>
          <input v-model.number="ddos.global_burst" min="5" type="number" class="aura-input w-full" />
        </label>
        <label class="rounded-lg border border-panel-border bg-panel-dark p-3 text-sm text-gray-300">
          <span class="mb-2 block text-xs text-gray-400">{{ t('security_center.ddos.auth_rps') }}</span>
          <input v-model.number="ddos.auth_rps" min="2" type="number" class="aura-input w-full" />
        </label>
        <label class="rounded-lg border border-panel-border bg-panel-dark p-3 text-sm text-gray-300">
          <span class="mb-2 block text-xs text-gray-400">{{ t('security_center.ddos.auth_burst') }}</span>
          <input v-model.number="ddos.auth_burst" min="2" type="number" class="aura-input w-full" />
        </label>
      </div>

      <div v-if="ddosWarnings.length" class="rounded-lg border border-yellow-500/30 bg-yellow-500/10 p-3 text-sm text-yellow-200">
        <p v-for="(warn, idx) in ddosWarnings" :key="`ddos-warn-${idx}`">{{ warn }}</p>
      </div>

      <div class="grid grid-cols-1 gap-3 md:grid-cols-2">
        <div class="rounded-lg border border-panel-border bg-panel-dark p-3 text-sm">
          <h3 class="mb-2 font-semibold text-white">{{ t('security_center.ddos.compatibility') }}</h3>
          <p v-if="ddosLoading" class="text-gray-400">{{ t('common.loading') }}</p>
          <ul v-else class="space-y-1 text-gray-300">
            <li v-for="(item, idx) in ddosCompatibility" :key="`ddos-compat-${idx}`">{{ item }}</li>
            <li v-if="ddosCompatibility.length === 0" class="text-gray-500">{{ t('common.no_data') }}</li>
          </ul>
        </div>
        <div class="rounded-lg border border-panel-border bg-panel-dark p-3 text-sm">
          <h3 class="mb-2 font-semibold text-white">{{ t('security_center.ddos.recommendations') }}</h3>
          <ul class="space-y-1 text-gray-300">
            <li v-for="(item, idx) in ddosRecommendations" :key="`ddos-reco-${idx}`">{{ item }}</li>
            <li v-if="ddosRecommendations.length === 0" class="text-gray-500">{{ t('common.no_data') }}</li>
          </ul>
        </div>
      </div>
    </div>

    <div v-if="activeTab === '2fa'" class="aura-card space-y-4">
      <h2 class="text-lg font-bold text-white">{{ t('security_center.twofa.title') }}</h2>
      <div class="grid grid-cols-1 gap-3 md:grid-cols-3">
        <input v-model="totp.account_name" class="aura-input md:col-span-2" :placeholder="t('security_center.twofa.account_name')" />
        <button class="btn-primary" @click="setup2fa">{{ t('security_center.twofa.setup') }}</button>
      </div>
      <div v-if="totp.secret" class="space-y-3 rounded-lg border border-panel-border bg-panel-dark p-4">
        <p class="text-sm"><strong>{{ t('security_center.twofa.secret') }}:</strong> <span class="font-mono">{{ totp.secret }}</span></p>
        <img v-if="totp.qr_base64" :src="`data:image/png;base64,${totp.qr_base64}`" :alt="t('security_center.twofa.qr_alt')" class="h-40 w-40 rounded border border-panel-border" />
        <div class="flex gap-3">
          <input v-model="totp.token" class="aura-input" :placeholder="t('security_center.twofa.token')" />
          <button class="btn-secondary" @click="verify2fa">{{ t('security_center.twofa.verify') }}</button>
        </div>
        <p v-if="totp.verifyResult !== null" :class="totp.verifyResult ? 'text-green-400' : 'text-red-400'">
          {{ totp.verifyResult ? t('security_center.twofa.verified') : t('security_center.twofa.invalid') }}
        </p>
      </div>
    </div>

    <div v-if="activeTab === 'ssh_settings'" class="aura-card space-y-4">
      <h2 class="text-lg font-bold text-white">{{ t('security_center.ssh_settings.title') }}</h2>
      <div class="grid grid-cols-1 gap-3 md:grid-cols-4">
        <input v-model="ssh.user" class="aura-input" :placeholder="t('security_center.ssh.user')" />
        <input v-model="ssh.title" class="aura-input" :placeholder="t('security_center.ssh.title_label')" />
        <input v-model="ssh.public_key" class="aura-input md:col-span-2" :placeholder="t('security_center.ssh.public_key')" />
      </div>
      <div class="flex gap-3">
        <button class="btn-secondary" @click="goToSftpUserAdd">{{ t('security_center.ssh_settings.add_user') }}</button>
        <button class="btn-primary" @click="addSshKey">{{ t('security_center.ssh.add_key') }}</button>
        <button class="btn-secondary" @click="loadSshKeys">{{ t('security_center.ssh.list') }}</button>
      </div>
      <p class="text-xs text-gray-400">{{ t('security_center.ssh_settings.sftp_note') }}</p>
      <div class="space-y-2">
        <div v-for="key in sshKeys" :key="key.id" class="flex items-center justify-between gap-3 rounded-lg border border-panel-border bg-panel-dark p-3">
          <div>
            <p class="text-sm text-white">{{ key.user }} - {{ key.title }}</p>
            <p class="break-all font-mono text-xs text-gray-400">{{ key.public_key }}</p>
          </div>
          <button class="btn-danger px-3 py-1 text-xs" @click="deleteSshKey(key)">{{ t('security_center.ssh.delete') }}</button>
        </div>
      </div>
    </div>

    <div v-if="activeTab === 'ssh_settings'" class="aura-card space-y-4">
      <div class="flex items-center justify-between mb-4">
        <div>
          <h2 class="text-lg font-bold text-white">{{ t('security_center.ssh_config.title') }}</h2>
          <p class="text-sm text-gray-400">{{ t('security_center.ssh_config.desc') }}</p>
        </div>
        <button class="btn-secondary" @click="loadSshConfig" :disabled="sshConfigLoading">
          {{ sshConfigLoading ? t('common.loading') : t('common.refresh') }}
        </button>
      </div>

      <div class="grid grid-cols-1 md:grid-cols-2 gap-6">
        <div>
          <label class="block text-sm text-gray-400 mb-1">{{ t('security_center.ssh_config.port_label') }}</label>
          <input v-model="sshConfig.port" type="number" class="aura-input w-full" :placeholder="t('security_center.ssh_config.port_placeholder')" />
          <p class="text-xs text-gray-500 mt-1">{{ t('security_center.ssh_config.port_desc') }}</p>
        </div>
        <div>
          <label class="block text-sm text-gray-400 mb-1">{{ t('security_center.ssh_config.root_login_label') }}</label>
          <select v-model="sshConfig.permit_root_login" class="aura-input w-full">
            <option value="yes">{{ t('security_center.ssh_config.root_yes') }}</option>
            <option value="prohibit-password">{{ t('security_center.ssh_config.root_prohibit_password') }}</option>
            <option value="no">{{ t('security_center.ssh_config.root_no') }}</option>
          </select>
          <p class="text-xs text-gray-500 mt-1">{{ t('security_center.ssh_config.root_desc') }}</p>
        </div>
      </div>

      <div class="mt-6 flex justify-end">
        <button class="btn-primary" @click="saveSshConfig" :disabled="sshConfigSaving">
          {{ sshConfigSaving ? t('common.loading') : t('security_center.ssh_config.save') }}
        </button>
      </div>
    </div>

    <div v-if="activeTab === 'malware'" class="space-y-4">
      <div class="aura-card space-y-4">
        <h2 class="text-lg font-bold text-white">{{ t('security_center.malware.title') }}</h2>
        <div class="grid grid-cols-1 gap-3 md:grid-cols-4">
          <input v-model="malwareForm.path" class="aura-input md:col-span-2" :placeholder="t('security_center.malware.path_placeholder')" />
          <select v-model="malwareForm.engine" class="aura-input">
            <option value="auto">{{ t('security_center.malware.engine_auto') }}</option>
            <option value="clamav">{{ t('security_center.malware.engine_clamav') }}</option>
            <option value="yara">{{ t('security_center.malware.engine_yara') }}</option>
            <option value="signature">{{ t('security_center.malware.engine_signature') }}</option>
          </select>
          <button class="btn-primary" :disabled="malwareStarting" @click="startMalwareScan">
            {{ malwareStarting ? t('security_center.malware.starting') : t('security_center.malware.start_scan') }}
          </button>
        </div>
      </div>

      <div class="aura-card space-y-4">
        <div class="flex items-center justify-between">
          <h3 class="text-base font-semibold text-white">{{ t('security_center.malware.scan_jobs') }}</h3>
          <button class="btn-secondary" @click="loadMalwareJobs">{{ t('common.refresh') }}</button>
        </div>
        <div v-if="malwareJobs.length === 0" class="text-sm text-gray-400">{{ t('security_center.malware.scan_jobs_empty') }}</div>
        <div v-for="job in malwareJobs" :key="job.id" class="rounded-lg border border-panel-border bg-panel-dark p-4 space-y-3">
          <div class="flex flex-wrap items-center gap-2 text-sm">
            <span class="font-mono text-gray-300">{{ job.id }}</span>
            <span class="text-gray-500">|</span>
            <span :class="scanStatusClass(job.status)">{{ job.status }}</span>
            <span class="text-gray-500">|</span>
            <span class="text-gray-300">%{{ job.progress || 0 }}</span>
            <span class="text-gray-500">|</span>
            <span class="text-gray-300">{{ t('security_center.malware.findings_count', { count: job.infected_files || 0 }) }}</span>
            <button class="btn-secondary ml-auto text-xs px-2 py-1" @click="loadMalwareStatus(job.id)">{{ t('security_center.malware.details') }}</button>
          </div>
          <p class="text-xs text-gray-400 break-all">{{ t('security_center.malware.target_label', { path: job.target_path }) }}</p>
          <div class="h-2 rounded bg-[#0f172a] overflow-hidden">
            <div class="h-full bg-gradient-to-r from-emerald-500 to-cyan-500" :style="{ width: `${job.progress || 0}%` }"></div>
          </div>
          <div v-if="job.findings?.length" class="space-y-2">
            <p class="text-sm text-white font-semibold">{{ t('security_center.malware.detected_files') }}</p>
            <div v-for="finding in job.findings" :key="finding.id" class="rounded border border-panel-border p-2 text-xs">
              <p class="text-gray-200 break-all font-mono">{{ finding.file_path }}</p>
              <p class="text-yellow-300 mt-1">{{ finding.signature }} ({{ finding.engine }})</p>
              <div class="mt-2 flex gap-2">
                <button
                  class="btn-danger text-xs px-2 py-1"
                  :disabled="finding.quarantined"
                  @click="quarantineMalwareFinding(job.id, finding.id)"
                >
                  {{ finding.quarantined ? t('security_center.malware.quarantined') : t('security_center.malware.quarantine') }}
                </button>
              </div>
            </div>
          </div>
          <div v-if="job.logs?.length" class="rounded border border-panel-border p-2">
            <p class="text-xs text-gray-400 mb-1">{{ t('security_center.malware.scan_log') }}</p>
            <pre class="max-h-32 overflow-auto text-[11px] text-gray-300 whitespace-pre-wrap">{{ job.logs.join('\n') }}</pre>
          </div>
        </div>
      </div>

      <div class="aura-card space-y-3">
        <div class="flex items-center justify-between">
          <h3 class="text-base font-semibold text-white">{{ t('security_center.malware.quarantine_manager') }}</h3>
          <button class="btn-secondary" @click="loadQuarantineRecords">{{ t('common.refresh') }}</button>
        </div>
        <div v-if="quarantineRecords.length === 0" class="text-sm text-gray-400">{{ t('security_center.malware.quarantine_empty') }}</div>
        <div v-for="item in quarantineRecords" :key="item.id" class="rounded-lg border border-panel-border bg-panel-dark p-3 text-xs space-y-1">
          <p class="text-gray-200 font-mono break-all">{{ t('security_center.malware.original_label', { path: item.original_path }) }}</p>
          <p class="text-gray-400 font-mono break-all">{{ t('security_center.malware.quarantine_label', { path: item.quarantine_path }) }}</p>
          <p class="text-gray-500">{{ t('security_center.malware.job_finding_label', { job: item.job_id, finding: item.finding_id }) }}</p>
          <button
            class="btn-secondary text-xs px-2 py-1 mt-2"
            :disabled="!!item.restored_at"
            @click="restoreQuarantineRecord(item.id)"
          >
            {{ item.restored_at ? t('security_center.malware.restored') : t('security_center.malware.restore') }}
          </button>
        </div>
      </div>
    </div>

    <div v-if="activeTab === 'hardening'" class="aura-card space-y-4">
      <h2 class="text-lg font-bold text-white">{{ t('security_center.hardening.title') }}</h2>
      <div class="grid grid-cols-1 gap-3 md:grid-cols-3">
        <select v-model="hardening.stack" class="aura-input">
          <option value="wordpress">{{ t('security_center.hardening.stacks.wordpress') }}</option>
          <option value="laravel">{{ t('security_center.hardening.stacks.laravel') }}</option>
          <option value="generic">{{ t('security_center.hardening.stacks.generic') }}</option>
        </select>
        <input v-model="hardening.domain" class="aura-input md:col-span-2" :placeholder="t('security_center.hardening.domain')" />
      </div>
      <button class="btn-primary" @click="applyHardening">{{ t('security_center.hardening.apply') }}</button>
      <div v-if="hardeningResult" class="rounded-lg border border-panel-border bg-panel-dark p-4">
        <p class="mb-2 text-sm text-white"><strong>{{ t('security_center.hardening.rules_for', { domain: hardeningResult.domain }) }}</strong></p>
        <ul class="list-disc pl-5 text-sm text-gray-300">
          <li v-for="rule in hardeningResult.applied_rules" :key="rule">{{ rule }}</li>
        </ul>
      </div>
    </div>

    <div v-if="activeTab === 'kernel'" class="aura-card space-y-4">
      <h2 class="text-lg font-bold text-white">{{ t('security_center.kernel.title') }}</h2>
      <div class="flex gap-3">
        <button class="btn-secondary" @click="loadImmutableStatus">{{ t('security_center.kernel.immutable_status') }}</button>
        <button class="btn-secondary" @click="loadEbpfEvents">{{ t('security_center.kernel.ebpf_events') }}</button>
      </div>
      <div class="flex gap-3">
        <input v-model="livePatchTarget" class="aura-input" :placeholder="t('security_center.kernel.live_patch_target')" />
        <button class="btn-primary" @click="runLivePatch">{{ t('security_center.kernel.live_patch') }}</button>
      </div>
      <pre v-if="immutableStatus" class="overflow-auto rounded-lg border border-panel-border bg-panel-dark p-3 text-xs text-gray-300">{{ JSON.stringify(immutableStatus, null, 2) }}</pre>
      <div class="space-y-2">
        <div v-for="(eventItem, index) in ebpfEvents" :key="index" class="rounded-lg border border-panel-border bg-panel-dark p-3 text-sm text-gray-300">
          {{ eventItem }}
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import api from '../services/api'

const { t } = useI18n({ useScope: 'global' })
const route = useRoute()
const router = useRouter()
const authStore = useAuthStore()

const tabs = [
  { id: 'overview', label: t('security_center.tabs.overview') },
  { id: 'firewall', label: t('security_center.tabs.firewall') },
  { id: 'fail2ban', label: t('security_center.tabs.fail2ban') },
  { id: 'waf', label: t('security_center.tabs.waf') },
  { id: 'ddos', label: t('security_center.tabs.ddos') },
  { id: '2fa', label: t('security_center.tabs.twofa') },
  { id: 'ssh_settings', label: t('security_center.tabs.ssh_settings') },
  { id: 'malware', label: t('security_center.tabs.malware') },
  { id: 'hardening', label: t('security_center.tabs.hardening') },
  { id: 'kernel', label: t('security_center.tabs.kernel') },
]

function normalizeSecurityTab(tab) {
  const value = String(tab || '').trim()
  if (value === 'ssh' || value === 'ssh_config') return 'ssh_settings'
  return value || 'overview'
}

const activeTab = ref(normalizeSecurityTab(typeof route.query.tab === 'string' ? route.query.tab : 'overview'))
const fail2banStatus = ref({ status: 'loading', raw: '' })
const fail2banLoading = ref(false)
const ddosLoading = ref(false)
const ddosSaving = ref(false)
const ddosWarnings = ref([])
const ddosCompatibility = ref([])
const ddosRecommendations = ref([])
const ddos = ref({
  enabled: false,
  profile: 'standard',
  global_rps: 120,
  global_burst: 240,
  auth_rps: 20,
  auth_burst: 40,
})

async function loadFail2ban() {
  fail2banLoading.value = true
  try {
    const res = await api.get('/security/fail2ban/list')
    fail2banStatus.value = res.data.data || { status: 'inactive', raw: '' }
  } catch (err) {
    console.error(err)
  } finally {
    fail2banLoading.value = false
  }
}

async function unbanIp(ip) {
  if (!confirm(t('security_center.fail2ban.unban_confirm', { ip }))) return
  try {
    await api.post(`/security/fail2ban/unban?ip=${ip}`)
    await loadFail2ban()
  } catch (err) {
    alert(`${t('common.error')}: ${err.response?.data?.message || err.message}`)
  }
}

function normalizeDdosProfile(profile) {
  const value = String(profile || '').toLowerCase()
  return value === 'strict' ? 'strict' : 'standard'
}

function toPositiveInt(value, fallback) {
  const num = Number(value)
  return Number.isFinite(num) && num > 0 ? Math.round(num) : fallback
}

async function loadDdos() {
  ddosLoading.value = true
  try {
    const res = await api.get('/security/ddos')
    const data = res.data?.data || {}
    ddos.value.enabled = !!data.enabled
    ddos.value.profile = normalizeDdosProfile(data.profile)
    ddos.value.global_rps = toPositiveInt(data.global_rps, 120)
    ddos.value.global_burst = toPositiveInt(data.global_burst, 240)
    ddos.value.auth_rps = toPositiveInt(data.auth_rps, 20)
    ddos.value.auth_burst = toPositiveInt(data.auth_burst, 40)
    ddosCompatibility.value = Array.isArray(data.compatibility) ? data.compatibility : []
    ddosRecommendations.value = Array.isArray(data.recommendations) ? data.recommendations : []
    ddosWarnings.value = []
  } catch (err) {
    ddosWarnings.value = [err.response?.data?.message || err.message || t('security_center.ddos.load_failed')]
  } finally {
    ddosLoading.value = false
  }
}

async function saveDdos() {
  ddosSaving.value = true
  try {
    const payload = {
      enabled: !!ddos.value.enabled,
      profile: normalizeDdosProfile(ddos.value.profile),
      global_rps: toPositiveInt(ddos.value.global_rps, 120),
      global_burst: toPositiveInt(ddos.value.global_burst, 240),
      auth_rps: toPositiveInt(ddos.value.auth_rps, 20),
      auth_burst: toPositiveInt(ddos.value.auth_burst, 40),
    }
    const res = await api.post('/security/ddos', payload)
    const data = res.data?.data || {}
    ddosWarnings.value = Array.isArray(data.warnings) ? data.warnings : []
    ddosCompatibility.value = Array.isArray(data.compatibility) ? data.compatibility : []
    ddosRecommendations.value = Array.isArray(data.recommendations) ? data.recommendations : []
    await loadDdos()
    await loadStatus()
  } catch (err) {
    ddosWarnings.value = [err.response?.data?.message || err.message || t('security_center.ddos.save_failed')]
  } finally {
    ddosSaving.value = false
  }
}

const status = ref({})
const firewallRules = ref([])
const firewallError = ref('')
const firewallPortRules = ref([])
const firewallPortError = ref('')
const sshKeys = ref([])
const wafResult = ref(null)
const wafActionMessage = ref('')
const wafActionError = ref('')
const wafStateSaving = ref(false)
const hardeningResult = ref(null)
const immutableStatus = ref(null)
const ebpfEvents = ref([])
const livePatchTarget = ref('kernel')
const silentEbpfConfig = {
  headers: {
    'X-Aura-Silent-Error': '1',
    'X-Aura-Silent-Loading': '1',
  },
}
const malwareForm = ref({ path: '/home', engine: 'auto' })
const malwareJobs = ref([])
const quarantineRecords = ref([])
const malwareStarting = ref(false)
let malwarePollTimer = null

const firewallForm = ref({ ip_address: '', block: true, reason: '' })
const firewallPortForm = ref({ port: '', protocol: 'tcp', block: false, reason: '' })
const wafInput = ref({
  method: 'GET',
  path: '/',
  query: '',
  body: '',
  user_agent: 'AuraPanel-Security-Test',
  ip: '127.0.0.1',
})
const totp = ref({
  account_name: authStore.user?.email || authStore.user?.username || 'admin@aurapanel',
  secret: '',
  qr_base64: '',
  token: '',
  verifyResult: null,
})
const ssh = ref({ user: 'root', title: '', public_key: '' })
const hardening = ref({ stack: 'wordpress', domain: '' })

const sshConfig = ref({ port: '22', permit_root_login: 'yes' })
const sshConfigLoading = ref(false)
const sshConfigSaving = ref(false)

const statusCards = computed(() => [
  { key: 'ebpf', label: t('security_center.cards.ebpf'), value: status.value.ebpf_monitoring },
  { key: 'waf', label: t('security_center.cards.waf'), value: status.value.ml_waf },
  { key: 'ddos', label: t('security_center.cards.ddos'), value: status.value.ddos_guard },
  { key: 'totp', label: t('security_center.cards.totp'), value: status.value.totp_2fa },
  { key: 'wg', label: t('security_center.cards.wireguard'), value: status.value.wireguard_federation },
  { key: 'immutable', label: t('security_center.cards.immutable'), value: status.value.immutable_os_support },
  { key: 'livepatch', label: t('security_center.cards.livepatch'), value: status.value.live_patching },
  { key: 'hardening', label: t('security_center.cards.hardening'), value: status.value.one_click_hardening },
  { key: 'fw', label: t('security_center.cards.nft_firewall'), value: status.value.nft_firewall },
  { key: 'ssh', label: t('security_center.cards.ssh_key_manager'), value: status.value.ssh_key_manager },
])

function setTab(tab) {
  const normalized = normalizeSecurityTab(tab)
  activeTab.value = normalized
  router.replace({ query: { ...route.query, tab: normalized } })
  if (normalized === 'fail2ban') loadFail2ban()
  if (normalized === 'ddos') loadDdos()
  if (normalized === 'ssh_settings') {
    loadSshConfig()
    loadSshKeys()
  }
}

watch(
  () => route.query.tab,
  tab => {
    const normalized = normalizeSecurityTab(typeof tab === 'string' ? tab : 'overview')
    activeTab.value = normalized
    if (normalized === 'ddos') loadDdos()
    if (normalized === 'ssh_settings') {
      loadSshConfig()
      loadSshKeys()
    }
  },
)

function goToSftpUserAdd() {
  router.push({ path: '/sftp', query: { action: 'create' } })
}

async function loadStatus() {
  const res = await api.get('/security/status')
  status.value = res.data.data || {}
}

async function loadFirewallRules() {
  try {
    const res = await api.get('/security/firewall/rules')
    firewallRules.value = res.data.data || []
  } catch (err) {
    firewallError.value = err.response?.data?.message || err.message || t('security_center.firewall.load_failed')
  }
}

async function loadFirewallPortRules() {
  try {
    const res = await api.get('/security/firewall/ports')
    firewallPortRules.value = res.data.data || []
  } catch (err) {
    firewallPortError.value = err.response?.data?.message || err.message || t('security_center.firewall.port_load_failed')
  }
}

function isValidIPv4(input) {
  const parts = String(input || '').split('.')
  if (parts.length !== 4) return false
  return parts.every(part => {
    if (!/^\d+$/.test(part)) return false
    const value = Number(part)
    return value >= 0 && value <= 255
  })
}

function isValidIPv4OrCIDR(input) {
  const value = String(input || '').trim()
  if (!value) return false
  const segments = value.split('/')
  if (segments.length > 2) return false
  const ipPart = segments[0]
  if (!isValidIPv4(ipPart)) return false
  if (segments.length === 1) return true
  const cidrPart = segments[1]
  if (!/^\d{1,2}$/.test(cidrPart)) return false
  const cidr = Number(cidrPart)
  return cidr >= 0 && cidr <= 32
}

function isValidPort(input) {
  if (!/^\d+$/.test(String(input || ''))) return false
  const value = Number(input)
  return value >= 1 && value <= 65535
}

async function addFirewallRule() {
  const ipAddress = String(firewallForm.value.ip_address || '').trim()
  if (!isValidIPv4OrCIDR(ipAddress)) {
    firewallError.value = t('security_center.firewall.invalid_ip_cidr')
    return
  }

  firewallError.value = ''
  try {
    await api.post('/security/firewall', {
      ...firewallForm.value,
      ip_address: ipAddress,
      reason: String(firewallForm.value.reason || '').trim(),
    })
    firewallForm.value.ip_address = ''
    firewallForm.value.reason = ''
    await loadFirewallRules()
  } catch (err) {
    firewallError.value = err.response?.data?.message || err.message || t('security_center.firewall.add_failed')
  }
}

async function deleteFirewallRule(ip) {
  firewallError.value = ''
  try {
    await api.post('/security/firewall/rules/delete', { ip_address: ip })
    await loadFirewallRules()
  } catch (err) {
    firewallError.value = err.response?.data?.message || err.message || t('security_center.firewall.delete_failed')
  }
}

async function addFirewallPortRule() {
  const portText = String(firewallPortForm.value.port || '').trim()
  if (!isValidPort(portText)) {
    firewallPortError.value = t('security_center.firewall.invalid_port')
    return
  }

  firewallPortError.value = ''
  try {
    await api.post('/security/firewall/ports', {
      ...firewallPortForm.value,
      port: Number(portText),
      protocol: String(firewallPortForm.value.protocol || 'tcp').toLowerCase(),
      reason: String(firewallPortForm.value.reason || '').trim(),
    })
    firewallPortForm.value.port = ''
    firewallPortForm.value.reason = ''
    await loadFirewallPortRules()
  } catch (err) {
    firewallPortError.value = err.response?.data?.message || err.message || t('security_center.firewall.add_port_failed')
  }
}

async function deleteFirewallPortRule(rule) {
  firewallPortError.value = ''
  try {
    await api.post('/security/firewall/ports/delete', {
      port: Number(rule.port),
      protocol: String(rule.protocol || 'tcp').toLowerCase(),
      block: !!rule.block,
    })
    await loadFirewallPortRules()
  } catch (err) {
    firewallPortError.value = err.response?.data?.message || err.message || t('security_center.firewall.delete_port_failed')
  }
}

async function runWafTest() {
  wafActionError.value = ''
  try {
    const res = await api.post('/security/waf', wafInput.value)
    wafResult.value = res.data?.data || res.data
  } catch (err) {
    wafActionError.value = err.response?.data?.message || err.message || t('common.error')
  }
}

async function setWafState(enabled) {
  wafActionError.value = ''
  wafActionMessage.value = ''
  wafStateSaving.value = true
  try {
    const res = await api.post('/security/waf', { action: enabled ? 'enable' : 'disable' })
    const data = res.data?.data || {}
    const nextState = typeof data.enabled === 'boolean' ? data.enabled : enabled
    status.value = { ...status.value, ml_waf: nextState }
    wafActionMessage.value = res.data?.message || (nextState ? 'WAF enabled successfully.' : 'WAF disabled successfully.')
    await loadStatus()
  } catch (err) {
    wafActionError.value = err.response?.data?.message || err.message || t('common.error')
  } finally {
    wafStateSaving.value = false
  }
}

async function setup2fa() {
  const res = await api.post('/security/2fa/setup', { account_name: totp.value.account_name })
  totp.value.secret = res.data.data.secret
  totp.value.qr_base64 = res.data.data.qr_base64
  totp.value.verifyResult = null
}

async function verify2fa() {
  const res = await api.post('/security/2fa/verify', { token: totp.value.token })
  totp.value.verifyResult = !!res.data.valid
  if (totp.value.verifyResult) {
    authStore.updateUser({ two_fa_enabled: true })
    await loadStatus()
  }
}

async function loadSshConfig() {
  sshConfigLoading.value = true
  try {
    const res = await api.get('/security/ssh/config')
    if (res.data?.data) {
      sshConfig.value = res.data.data
    }
  } catch (err) {
    alert(`${t('security_center.ssh_config.load_failed')}: ${err.message}`)
  } finally {
    sshConfigLoading.value = false
  }
}

async function saveSshConfig() {
  sshConfigSaving.value = true
  try {
    const port = Number(sshConfig.value.port)
    await api.post('/security/ssh/config', {
      port: Number.isFinite(port) ? port : String(sshConfig.value.port || '').trim(),
      permit_root_login: String(sshConfig.value.permit_root_login || '').trim().toLowerCase(),
    })
    alert(t('security_center.ssh_config.save_success'))
  } catch (err) {
    alert(`${t('common.error')}: ${err.response?.data?.message || err.message}`)
  } finally {
    sshConfigSaving.value = false
  }
}

async function addSshKey() {
  await api.post('/security/ssh-keys', ssh.value)
  ssh.value.title = ''
  ssh.value.public_key = ''
  await loadSshKeys()
}

async function loadSshKeys() {
  const params = ssh.value.user ? { user: ssh.value.user } : {}
  const res = await api.get('/security/ssh-keys', { params })
  sshKeys.value = res.data.data || []
}

async function deleteSshKey(key) {
  await api.delete('/security/ssh-keys', {
    params: { user: key.user, key_id: key.id },
  })
  await loadSshKeys()
}

async function applyHardening() {
  const res = await api.post('/security/hardening/apply', hardening.value)
  hardeningResult.value = res.data.data
}

async function loadImmutableStatus() {
  const res = await api.get('/security/immutable/status')
  immutableStatus.value = res.data.data
}

async function loadEbpfEvents() {
  try {
    await api.post('/security/ebpf/collect', { limit: 100 }, silentEbpfConfig)
    const res = await api.get('/security/ebpf/events', silentEbpfConfig)
    ebpfEvents.value = res.data.data || []
  } catch {
    ebpfEvents.value = []
  }
}

async function runLivePatch() {
  await api.post('/security/live-patch', { target: livePatchTarget.value })
}

function scanStatusClass(status) {
  const value = String(status || '').toLowerCase()
  if (value === 'completed') return 'text-green-400'
  if (value === 'failed') return 'text-red-400'
  if (value === 'running') return 'text-blue-400'
  return 'text-yellow-400'
}

function stopMalwarePolling() {
  if (malwarePollTimer) {
    clearInterval(malwarePollTimer)
    malwarePollTimer = null
  }
}

function startMalwarePolling() {
  stopMalwarePolling()
  malwarePollTimer = setInterval(async () => {
    await loadMalwareJobs()
  }, 2500)
}

async function loadMalwareJobs() {
  const res = await api.get('/security/malware/scan/jobs', { params: { limit: 15 } })
  malwareJobs.value = res.data.data || []
  const hasActive = malwareJobs.value.some(job => {
    const state = String(job.status || '').toLowerCase()
    return state === 'queued' || state === 'running'
  })
  if (hasActive) {
    if (!malwarePollTimer) {
      startMalwarePolling()
    }
  } else {
    stopMalwarePolling()
  }
}

async function loadMalwareStatus(jobId) {
  const res = await api.get('/security/malware/scan/status', { params: { id: jobId } })
  const latest = res.data?.data
  if (!latest) return
  const idx = malwareJobs.value.findIndex(job => job.id === latest.id)
  if (idx >= 0) {
    malwareJobs.value[idx] = latest
  } else {
    malwareJobs.value.unshift(latest)
  }
}

async function startMalwareScan() {
  malwareStarting.value = true
  try {
    await api.post('/security/malware/scan/start', malwareForm.value)
    await loadMalwareJobs()
    await loadQuarantineRecords()
  } finally {
    malwareStarting.value = false
  }
}

async function quarantineMalwareFinding(jobId, findingId) {
  await api.post('/security/malware/quarantine', { job_id: jobId, finding_id: findingId })
  await loadMalwareStatus(jobId)
  await loadQuarantineRecords()
}

async function loadQuarantineRecords() {
  const res = await api.get('/security/malware/quarantine')
  quarantineRecords.value = res.data.data || []
}

async function restoreQuarantineRecord(quarantineId) {
  await api.post('/security/malware/quarantine/restore', { quarantine_id: quarantineId })
  await loadQuarantineRecords()
  await loadMalwareJobs()
}

async function loadAll() {
  await Promise.all([
    loadStatus(),
    loadDdos(),
    loadFirewallRules(),
    loadFirewallPortRules(),
    loadSshKeys(),
    loadMalwareJobs(),
    loadQuarantineRecords(),
  ])
  if (activeTab.value === 'ssh_settings') {
    await loadSshConfig()
  }
}

onMounted(loadAll)
onBeforeUnmount(() => {
  stopMalwarePolling()
})
</script>

