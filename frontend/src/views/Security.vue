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
          <span class="font-semibold text-white">Durum:</span>
          <span :class="status.firewall_active ? 'text-emerald-400' : 'text-yellow-400'">
            {{ status.firewall_active ? 'Aktif' : 'Pasif / Tespit edilemedi' }}
          </span>
          <span v-if="status.firewall_manager" class="text-gray-400">Yonetici: {{ status.firewall_manager }}</span>
          <span v-if="status.server_ip" class="text-gray-400">Sunucu IP: {{ status.server_ip }}</span>
        </div>
        <p v-if="(status.firewall_open_ports || []).length" class="mt-3 text-xs text-gray-400">
          Acik portlar: {{ status.firewall_open_ports.join(', ') }}
        </p>
      </div>
      <div class="grid grid-cols-1 gap-3 md:grid-cols-4">
        <input
          v-model="firewallForm.ip_address"
          class="aura-input"
          placeholder="IP/CIDR (orn: 185.190.140.62 veya 185.190.140.0/24)"
        />
        <input v-model="firewallForm.reason" class="aura-input" :placeholder="t('security_center.firewall.reason')" />
        <select v-model="firewallForm.block" class="aura-input">
          <option :value="true">{{ t('security_center.firewall.block') }} (Tum trafik)</option>
          <option :value="false">{{ t('security_center.firewall.allow') }} (Tum trafik)</option>
        </select>
        <button class="btn-primary" @click="addFirewallRule">{{ t('security_center.firewall.add_rule') }}</button>
      </div>
      <p class="text-xs text-gray-400">
        Bu alan port degil, IP/CIDR tabanli kural icindir. Port/TCP/UDP yonetimi icin Panel Port veya servis bazli firewall ekranini kullanin.
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
    </div>

    <div v-if="activeTab === 'waf'" class="space-y-4">
      <div class="aura-card">
        <h2 class="text-lg font-bold text-white mb-4">ModSecurity & ML-WAF Yönetimi</h2>
        <div class="flex items-center justify-between p-4 rounded-xl border border-panel-border bg-panel-dark">
          <div>
            <h3 class="font-semibold text-white">Global WAF Durumu</h3>
            <p class="text-sm text-gray-400">Sunucu genelinde tüm web siteleri için ModSecurity kural motorunu açar veya kapatır.</p>
          </div>
          <div class="flex gap-2">
            <button class="btn-primary" disabled>WAF Açık</button>
            <button class="btn-secondary">Kapat</button>
          </div>
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
            {{ fail2banLoading ? t('common.loading') : t('common.refresh') || 'Yenile' }}
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
              <input type="text" id="unbanIpInput" placeholder="Örn: 192.168.1.1" class="aura-input flex-1" />
              <button class="btn-primary" @click="() => { const el = document.getElementById('unbanIpInput'); if(el.value) unbanIp(el.value); }">{{ t('security_center.fail2ban.unban_btn') }}</button>
            </div>
          </div>
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

    <div v-if="activeTab === 'ssh'" class="aura-card space-y-4">
      <h2 class="text-lg font-bold text-white">{{ t('security_center.ssh.title') }}</h2>
      <div class="grid grid-cols-1 gap-3 md:grid-cols-4">
        <input v-model="ssh.user" class="aura-input" :placeholder="t('security_center.ssh.user')" />
        <input v-model="ssh.title" class="aura-input" :placeholder="t('security_center.ssh.title_label')" />
        <input v-model="ssh.public_key" class="aura-input md:col-span-2" :placeholder="t('security_center.ssh.public_key')" />
      </div>
      <div class="flex gap-3">
        <button class="btn-primary" @click="addSshKey">{{ t('security_center.ssh.add_key') }}</button>
        <button class="btn-secondary" @click="loadSshKeys">{{ t('security_center.ssh.list') }}</button>
      </div>
      <div class="space-y-2">
        <div v-for="key in sshKeys" :key="key.id" class="flex items-center justify-between gap-3 rounded-lg border border-panel-border bg-panel-dark p-3">
          <div>
            <p class="text-sm text-white">{{ key.user }} · {{ key.title }}</p>
            <p class="break-all font-mono text-xs text-gray-400">{{ key.public_key }}</p>
          </div>
          <button class="btn-danger px-3 py-1 text-xs" @click="deleteSshKey(key)">{{ t('security_center.ssh.delete') }}</button>
        </div>
      </div>
    </div>

    <div v-if="activeTab === 'ssh_config'" class="aura-card space-y-4">
      <div class="flex items-center justify-between mb-4">
        <div>
          <h2 class="text-lg font-bold text-white">{{ t('security_center.ssh_config.title') || 'SSH Configuration' }}</h2>
          <p class="text-sm text-gray-400">{{ t('security_center.ssh_config.desc') || 'Manage SSH port and Root login access.' }}</p>
        </div>
        <button class="btn-secondary" @click="loadSshConfig" :disabled="sshConfigLoading">
          {{ sshConfigLoading ? t('common.loading') : (t('common.refresh') || 'Yenile') }}
        </button>
      </div>

      <div class="grid grid-cols-1 md:grid-cols-2 gap-6">
        <div>
          <label class="block text-sm text-gray-400 mb-1">SSH Port</label>
          <input v-model="sshConfig.port" type="number" class="aura-input w-full" placeholder="22" />
          <p class="text-xs text-gray-500 mt-1">{{ t('security_center.ssh_config.port_desc') || 'Default is 22. Make sure you open the new port in your firewall first.' }}</p>
        </div>
        <div>
          <label class="block text-sm text-gray-400 mb-1">Root Login (PermitRootLogin)</label>
          <select v-model="sshConfig.permit_root_login" class="aura-input w-full">
            <option value="yes">Yes (Erişime Açık)</option>
            <option value="prohibit-password">Prohibit Password (Sadece SSH Key)</option>
            <option value="no">No (Tamamen Kapalı)</option>
          </select>
          <p class="text-xs text-gray-500 mt-1">{{ t('security_center.ssh_config.root_desc') || 'Disabling root login enhances security.' }}</p>
        </div>
      </div>

      <div class="mt-6 flex justify-end">
        <button class="btn-primary" @click="saveSshConfig" :disabled="sshConfigSaving">
          {{ sshConfigSaving ? t('common.loading') : (t('security_center.ssh_config.save') || 'Save & Restart SSH') }}
        </button>
      </div>
    </div>

    <div v-if="activeTab === 'malware'" class="space-y-4">
      <div class="aura-card space-y-4">
        <h2 class="text-lg font-bold text-white">Malware Scanner</h2>
        <div class="grid grid-cols-1 gap-3 md:grid-cols-4">
          <input v-model="malwareForm.path" class="aura-input md:col-span-2" placeholder="/home/site/public_html" />
          <select v-model="malwareForm.engine" class="aura-input">
            <option value="auto">Auto (ClamAV/Yara/Fallback)</option>
            <option value="clamav">ClamAV</option>
            <option value="yara">Yara</option>
            <option value="signature">Signature Fallback</option>
          </select>
          <button class="btn-primary" :disabled="malwareStarting" @click="startMalwareScan">
            {{ malwareStarting ? 'Tarama baslatiliyor...' : 'Taramayi Baslat' }}
          </button>
        </div>
      </div>

      <div class="aura-card space-y-4">
        <div class="flex items-center justify-between">
          <h3 class="text-base font-semibold text-white">Scan Jobs</h3>
          <button class="btn-secondary" @click="loadMalwareJobs">Yenile</button>
        </div>
        <div v-if="malwareJobs.length === 0" class="text-sm text-gray-400">Tarama kaydi bulunmuyor.</div>
        <div v-for="job in malwareJobs" :key="job.id" class="rounded-lg border border-panel-border bg-panel-dark p-4 space-y-3">
          <div class="flex flex-wrap items-center gap-2 text-sm">
            <span class="font-mono text-gray-300">{{ job.id }}</span>
            <span class="text-gray-500">|</span>
            <span :class="scanStatusClass(job.status)">{{ job.status }}</span>
            <span class="text-gray-500">|</span>
            <span class="text-gray-300">%{{ job.progress || 0 }}</span>
            <span class="text-gray-500">|</span>
            <span class="text-gray-300">{{ job.infected_files || 0 }} bulgu</span>
            <button class="btn-secondary ml-auto text-xs px-2 py-1" @click="loadMalwareStatus(job.id)">Detay</button>
          </div>
          <p class="text-xs text-gray-400 break-all">Hedef: {{ job.target_path }}</p>
          <div class="h-2 rounded bg-[#0f172a] overflow-hidden">
            <div class="h-full bg-gradient-to-r from-emerald-500 to-cyan-500" :style="{ width: `${job.progress || 0}%` }"></div>
          </div>
          <div v-if="job.findings?.length" class="space-y-2">
            <p class="text-sm text-white font-semibold">Tespit Edilen Dosyalar</p>
            <div v-for="finding in job.findings" :key="finding.id" class="rounded border border-panel-border p-2 text-xs">
              <p class="text-gray-200 break-all font-mono">{{ finding.file_path }}</p>
              <p class="text-yellow-300 mt-1">{{ finding.signature }} ({{ finding.engine }})</p>
              <div class="mt-2 flex gap-2">
                <button
                  class="btn-danger text-xs px-2 py-1"
                  :disabled="finding.quarantined"
                  @click="quarantineMalwareFinding(job.id, finding.id)"
                >
                  {{ finding.quarantined ? 'Karantinada' : 'Karantinaya Al' }}
                </button>
              </div>
            </div>
          </div>
          <div v-if="job.logs?.length" class="rounded border border-panel-border p-2">
            <p class="text-xs text-gray-400 mb-1">Scan Log</p>
            <pre class="max-h-32 overflow-auto text-[11px] text-gray-300 whitespace-pre-wrap">{{ job.logs.join('\n') }}</pre>
          </div>
        </div>
      </div>

      <div class="aura-card space-y-3">
        <div class="flex items-center justify-between">
          <h3 class="text-base font-semibold text-white">Karantina Yoneticisi</h3>
          <button class="btn-secondary" @click="loadQuarantineRecords">Yenile</button>
        </div>
        <div v-if="quarantineRecords.length === 0" class="text-sm text-gray-400">Karantina kaydi bulunmuyor.</div>
        <div v-for="item in quarantineRecords" :key="item.id" class="rounded-lg border border-panel-border bg-panel-dark p-3 text-xs space-y-1">
          <p class="text-gray-200 font-mono break-all">Orijinal: {{ item.original_path }}</p>
          <p class="text-gray-400 font-mono break-all">Karantina: {{ item.quarantine_path }}</p>
          <p class="text-gray-500">Job: {{ item.job_id }} • Finding: {{ item.finding_id }}</p>
          <button
            class="btn-secondary text-xs px-2 py-1 mt-2"
            :disabled="!!item.restored_at"
            @click="restoreQuarantineRecord(item.id)"
          >
            {{ item.restored_at ? 'Geri Yuklendi' : 'Geri Yukle' }}
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
  { id: 'fail2ban', label: 'Fail2Ban' },
  { id: 'waf', label: t('security_center.tabs.waf') },
  { id: '2fa', label: t('security_center.tabs.twofa') },
  { id: 'ssh', label: t('security_center.tabs.ssh') },
  { id: 'ssh_config', label: t('security_center.tabs.ssh_config') || 'SSH Config' },
  { id: 'malware', label: 'Malware Scanner' },
  { id: 'hardening', label: t('security_center.tabs.hardening') },
  { id: 'kernel', label: t('security_center.tabs.kernel') },
]

const activeTab = ref(route.query.tab || 'overview')
const fail2banStatus = ref({ status: 'loading', raw: '' })
const fail2banLoading = ref(false)

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
  if (!confirm(`${ip} adresinin engelini kaldırmak istediğinize emin misiniz?`)) return
  try {
    await api.post(`/security/fail2ban/unban?ip=${ip}`)
    await loadFail2ban()
  } catch (err) {
    alert('Hata: ' + (err.response?.data?.message || err.message))
  }
}

const status = ref({})
const firewallRules = ref([])
const firewallError = ref('')
const sshKeys = ref([])
const wafResult = ref(null)
const hardeningResult = ref(null)
const immutableStatus = ref(null)
const ebpfEvents = ref([])
const livePatchTarget = ref('kernel')
const malwareForm = ref({ path: '/home', engine: 'auto' })
const malwareJobs = ref([])
const quarantineRecords = ref([])
const malwareStarting = ref(false)
let malwarePollTimer = null

const firewallForm = ref({ ip_address: '', block: true, reason: '' })
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
  { key: 'ebpf', label: 'eBPF Monitoring', value: status.value.ebpf_monitoring },
  { key: 'waf', label: 'ML-WAF', value: status.value.ml_waf },
  { key: 'totp', label: '2FA (TOTP)', value: status.value.totp_2fa },
  { key: 'wg', label: 'WireGuard Federation', value: status.value.wireguard_federation },
  { key: 'immutable', label: 'Immutable OS', value: status.value.immutable_os_support },
  { key: 'livepatch', label: 'Live Patching', value: status.value.live_patching },
  { key: 'hardening', label: 'One-Click Hardening', value: status.value.one_click_hardening },
  { key: 'fw', label: 'nft Firewall', value: status.value.nft_firewall },
  { key: 'ssh', label: 'SSH Key Manager', value: status.value.ssh_key_manager },
])

function setTab(tab) {
  activeTab.value = tab
  router.replace({ query: { ...route.query, tab } })
  if (tab === 'fail2ban') loadFail2ban()
  if (tab === 'ssh_config') loadSshConfig()
}

watch(
  () => route.query.tab,
  tab => {
    activeTab.value = tab || 'overview'
    if (tab === 'ssh_config') loadSshConfig()
  },
)

async function loadStatus() {
  const res = await api.get('/security/status')
  status.value = res.data.data || {}
}

async function loadFirewallRules() {
  try {
    const res = await api.get('/security/firewall/rules')
    firewallRules.value = res.data.data || []
  } catch (err) {
    firewallError.value = err.response?.data?.message || err.message || 'Firewall kurallari alinamadi.'
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

async function addFirewallRule() {
  const ipAddress = String(firewallForm.value.ip_address || '').trim()
  if (!isValidIPv4OrCIDR(ipAddress)) {
    firewallError.value = 'Gecersiz IP/CIDR. Ornek: 185.190.140.62 veya 185.190.140.0/24'
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
    firewallError.value = err.response?.data?.message || err.message || 'Firewall kurali eklenemedi.'
  }
}

async function deleteFirewallRule(ip) {
  firewallError.value = ''
  try {
    await api.delete('/security/firewall/rules', { params: { ip_address: ip } })
    await loadFirewallRules()
  } catch (err) {
    firewallError.value = err.response?.data?.message || err.message || 'Firewall kurali silinemedi.'
  }
}

async function runWafTest() {
  const res = await api.post('/security/waf', wafInput.value)
  wafResult.value = res.data
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
    alert('SSH ayarları okunamadı: ' + err.message)
  } finally {
    sshConfigLoading.value = false
  }
}

async function saveSshConfig() {
  sshConfigSaving.value = true
  try {
    await api.post('/security/ssh/config', sshConfig.value)
    alert('SSH ayarları başarıyla kaydedildi ve servis yeniden başlatıldı.')
  } catch (err) {
    alert('Hata: ' + (err.response?.data?.message || err.message))
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
  await api.post('/security/ebpf/collect', { limit: 100 })
  const res = await api.get('/security/ebpf/events')
  ebpfEvents.value = res.data.data || []
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
    loadFirewallRules(),
    loadSshKeys(),
    loadMalwareJobs(),
    loadQuarantineRecords(),
  ])
}

onMounted(loadAll)
onBeforeUnmount(() => {
  stopMalwarePolling()
})
</script>
