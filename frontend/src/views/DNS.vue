<template>
  <div class="space-y-6">
    <div class="flex items-center justify-between">
      <div>
        <h1 class="text-2xl font-bold text-white">{{ t('dns.title') }}</h1>
        <p class="text-gray-400 mt-1">{{ t('dns.subtitle') }}</p>
      </div>
      <div class="flex items-center gap-3">
        <button class="btn-secondary" @click="openDefaultNsModal">
          <Settings class="w-5 h-5 mr-1 inline" />
          {{ t('dns.default_ns') }}
        </button>
        <button class="btn-primary" @click="showAddZoneModal = true">
          <Plus class="w-5 h-5 mr-1 inline" />
          {{ t('dns.add_zone') }}
        </button>
      </div>
    </div>

    <div v-if="loading" class="aura-card text-center py-12">
      <Loader2 class="w-8 h-8 text-brand-500 animate-spin mx-auto mb-3" />
      <p class="text-gray-400">{{ t('common.loading') }}</p>
    </div>

    <div v-else-if="zones.length === 0" class="aura-card text-center py-16">
      <Network class="w-16 h-16 text-gray-600 mx-auto mb-4" />
      <h2 class="text-xl font-semibold text-white mb-2">{{ t('common.no_data') }}</h2>
      <p class="text-gray-400 mb-6">{{ t('dns.subtitle') }}</p>
      <button class="btn-primary mx-auto" @click="showAddZoneModal = true">{{ t('dns.add_zone') }}</button>
    </div>

    <div v-else class="space-y-4">
      <div v-for="zone in zones" :key="zone.id"
        class="aura-card flex flex-col sm:flex-row gap-4 justify-between items-start sm:items-center">
        <div class="flex items-center gap-4">
          <div class="w-10 h-10 rounded-lg bg-panel-dark flex items-center justify-center border border-panel-border">
            <Network class="w-5 h-5 text-brand-500" />
          </div>
          <div>
            <h3 class="text-lg font-bold text-white">{{ zone.name }}</h3>
            <p class="text-sm text-gray-400">{{ zone.kind }} - {{ zone.records }} {{ t('dns.add_record') }}</p>
            <p class="text-xs mt-1" :class="zone.dnssec_enabled ? 'text-green-400' : 'text-yellow-400'">
              {{ t('dns.dnssec') }}: {{ zone.dnssec_enabled ? t('dns.active') : t('dns.passive') }}
            </p>
          </div>
        </div>
        <div class="flex items-center gap-2">
          <button class="btn-secondary px-3 py-1.5 text-sm" :disabled="dnssecDomain === zone.name" @click="toggleDnssec(zone)">
            <Loader2 v-if="dnssecDomain === zone.name" class="w-4 h-4 mr-1 inline animate-spin" />
            {{ t('dns.dnssec') }} {{ zone.dnssec_enabled ? t('dns.disable') : t('dns.enable') }}
          </button>
          <button class="btn-secondary px-3 py-1.5 text-sm" :disabled="reconcilingDomain === zone.name" @click="reconcileZone(zone)">
            <Loader2 v-if="reconcilingDomain === zone.name" class="w-4 h-4 mr-1 inline animate-spin" />
            {{ t('dns.reconcile') }}
          </button>
          <button class="btn-secondary px-3 py-1.5 text-sm" @click="selectZone(zone)">
            {{ t('dns.manage_records') }}
          </button>
          <button class="btn-danger px-2 py-1.5" @click="confirmDeleteZone(zone)">
            <Trash2 class="w-4 h-4" />
          </button>
        </div>
      </div>
    </div>

    <!-- DNS Records Panel -->
    <div v-if="selectedZone" class="aura-card">
      <div class="flex justify-between items-center mb-4">
        <h2 class="text-lg font-bold text-white">{{ selectedZone.name }} - {{ t('dns.manage_records') }}</h2>
        <button class="btn-primary text-sm" @click="openAddRecordModal">
          <Plus class="w-4 h-4 mr-1 inline" />{{ t('dns.add_record') }}
        </button>
      </div>
      <div v-if="recordsLoading" class="py-6 text-center">
        <Loader2 class="w-6 h-6 text-brand-500 animate-spin mx-auto" />
      </div>
      <div v-else class="overflow-x-auto">
        <table class="w-full text-sm">
          <thead>
            <tr class="border-b border-panel-border text-gray-400">
              <th class="text-left py-2 pr-4">{{ t('dns.record_type') }}</th>
              <th class="text-left py-2 pr-4">{{ t('dns.record_name') }}</th>
              <th class="text-left py-2 pr-4">{{ t('dns.record_value') }}</th>
              <th class="text-left py-2">{{ t('dns.record_ttl') }}</th>
              <th class="text-right py-2"></th>
            </tr>
          </thead>
          <tbody>
            <tr v-if="records.length === 0">
              <td colspan="5" class="py-4 text-center text-gray-400">{{ t('common.no_data') }}</td>
            </tr>
            <tr v-for="(r, i) in records" :key="i" class="border-b border-panel-border/50 text-white">
              <td class="py-2 pr-4"><span class="px-2 py-0.5 rounded bg-brand-500/10 text-brand-400 text-xs font-bold">{{ r.record_type }}</span></td>
              <td class="py-2 pr-4 font-mono text-sm">{{ r.name }}</td>
              <td class="py-2 pr-4 font-mono text-sm text-gray-300">{{ r.content }}</td>
              <td class="py-2 text-gray-400">{{ r.ttl }}s</td>
              <td class="py-2 text-right">
                <button class="p-1.5 text-red-400 hover:text-red-300 rounded hover:bg-black/20" @click="deleteRecord(r)">
                  <Trash2 class="w-4 h-4" />
                </button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <!-- Add Zone Modal -->
    <Teleport to="body">
      <div v-if="showAddZoneModal" class="fixed inset-0 bg-black/60 flex items-center justify-center z-50 p-4">
        <div class="bg-panel-card border border-panel-border rounded-2xl p-8 w-full max-w-md shadow-2xl">
          <h2 class="text-xl font-bold text-white mb-6">{{ t('dns.add_zone') }}</h2>
          <div>
            <label class="block text-sm text-gray-400 mb-1">{{ t('dns.domain_label') }}</label>
            <input v-model="newZone" list="dns-domain-options" type="text" class="aura-input w-full" :placeholder="t('dns.placeholders.zone_domain')" />
            <datalist id="dns-domain-options">
              <option v-for="domainName in suggestedDomains" :key="`dns-zone-${domainName}`" :value="domainName" />
            </datalist>
          </div>
          <div class="flex gap-3 mt-8">
            <button class="btn-secondary flex-1" @click="showAddZoneModal = false">{{ t('common.cancel') }}</button>
            <button class="btn-primary flex-1" @click="addZone" :disabled="creatingZone">
              <Loader2 v-if="creatingZone" class="w-4 h-4 mr-2 animate-spin inline" />
              {{ t('common.create') }}
            </button>
          </div>
        </div>
      </div>
    </Teleport>

    <!-- Add Record Modal -->
    <Teleport to="body">
      <div v-if="showAddRecordModal" class="fixed inset-0 bg-black/60 flex items-center justify-center z-50 p-4">
        <div class="bg-panel-card border border-panel-border rounded-2xl p-8 w-full max-w-md shadow-2xl">
          <h2 class="text-xl font-bold text-white mb-6">{{ t('dns.add_record') }}</h2>
          
          <div class="space-y-4">
            <div>
              <label class="block text-sm text-gray-400 mb-1">{{ t('dns.record_type') }}</label>
              <select v-model="newRecord.record_type" class="aura-input w-full">
                <option value="A">A</option>
                <option value="CNAME">CNAME</option>
                <option value="TXT">TXT</option>
                <option value="MX">MX</option>
              </select>
            </div>
            <div>
              <label class="block text-sm text-gray-400 mb-1">{{ t('dns.record_name') }}</label>
              <input v-model="newRecord.name" type="text" class="aura-input w-full" :placeholder="t('dns.record_name_placeholder')" />
            </div>
            <div>
              <label class="block text-sm text-gray-400 mb-1">{{ t('dns.record_value') }}</label>
              <input v-model="newRecord.content" type="text" class="aura-input w-full" :placeholder="t('dns.placeholders.record_value')" />
            </div>
            <div>
              <label class="block text-sm text-gray-400 mb-1">{{ t('dns.record_ttl') }}</label>
              <input v-model="newRecord.ttl" type="number" class="aura-input w-full" />
            </div>
          </div>

          <div class="flex gap-3 mt-8">
            <button class="btn-secondary flex-1" @click="showAddRecordModal = false">{{ t('common.cancel') }}</button>
            <button class="btn-primary flex-1" @click="addRecord" :disabled="addingRecord">
              <Loader2 v-if="addingRecord" class="w-4 h-4 mr-2 animate-spin inline" />
              {{ t('common.create') }}
            </button>
          </div>
        </div>
      </div>
    </Teleport>

    <!-- Default NS Modal -->
    <Teleport to="body">
      <div v-if="showNsModal" class="fixed inset-0 bg-black/60 flex items-center justify-center z-50 p-4">
        <div class="bg-panel-card border border-panel-border rounded-2xl p-8 w-full max-w-md shadow-2xl">
          <h2 class="text-xl font-bold text-white mb-6">{{ t('dns.default_ns') }}</h2>
          <div class="space-y-4">
            <div>
              <label class="block text-sm text-gray-400 mb-1">{{ t('dns.wizard_base_domain') }}</label>
              <div class="flex gap-2">
                <input v-model="wizardBaseDomain" list="dns-domain-options" type="text" class="aura-input w-full" :placeholder="t('dns.placeholders.zone_domain')" />
                <button class="btn-secondary whitespace-nowrap" :disabled="nsWizardLoading" @click="fillNsByWizard">
                  <Loader2 v-if="nsWizardLoading" class="w-4 h-4 mr-1 inline animate-spin" />
                  {{ t('dns.wizard_fill') }}
                </button>
              </div>
            </div>
            <div>
              <label class="block text-sm text-gray-400 mb-1">{{ t('dns.nameserver_1') }}</label>
              <input v-model="nsConfig.ns1" type="text" class="aura-input w-full" :placeholder="t('dns.placeholders.nameserver_1')" />
            </div>
            <div>
              <label class="block text-sm text-gray-400 mb-1">{{ t('dns.nameserver_2') }}</label>
              <input v-model="nsConfig.ns2" type="text" class="aura-input w-full" :placeholder="t('dns.placeholders.nameserver_2')" />
            </div>
          </div>
          <div class="flex gap-3 mt-8">
            <button class="btn-secondary" :disabled="nsResetLoading" @click="resetDefaultNs">
              <Loader2 v-if="nsResetLoading" class="w-4 h-4 mr-1 animate-spin inline" />
              {{ t('dns.reset') }}
            </button>
            <button class="btn-secondary flex-1" @click="showNsModal = false">{{ t('common.cancel') }}</button>
            <button class="btn-primary flex-1" @click="saveDefaultNs" :disabled="savingNs">
              <Loader2 v-if="savingNs" class="w-4 h-4 mr-2 animate-spin inline" />
              {{ t('common.save') }}
            </button>
          </div>
        </div>
      </div>
    </Teleport>
  </div>
</template>

<script setup>
import { computed, ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { Network, Plus, Trash2, Loader2, Settings } from 'lucide-vue-next'
import api from '../services/api'

const { t } = useI18n()
const zones = ref([])
const websiteDomains = ref([])
const loading = ref(true)
const recordsLoading = ref(false)

const showAddZoneModal = ref(false)
const showAddRecordModal = ref(false)
const showNsModal = ref(false)

const selectedZone = ref(null)
const newZone = ref('')
const records = ref([])

const newRecord = ref({
  record_type: 'A',
  name: '',
  content: '',
  ttl: 3600
})

const nsConfig = ref({
  ns1: '',
  ns2: ''
})

const creatingZone = ref(false)
const addingRecord = ref(false)
const savingNs = ref(false)
const reconcilingDomain = ref('')
const dnssecDomain = ref('')
const wizardBaseDomain = ref('')
const nsWizardLoading = ref(false)
const nsResetLoading = ref(false)
const suggestedDomains = computed(() => {
  const names = new Set()

  for (const zone of zones.value || []) {
    const raw = String(zone?.name || '').replace(/\.$/, '').trim().toLowerCase()
    if (raw) names.add(raw)
  }

  for (const domain of websiteDomains.value || []) {
    const raw = String(domain || '').trim().toLowerCase()
    if (raw) names.add(raw)
  }

  return Array.from(names).sort((a, b) => a.localeCompare(b))
})

async function loadZones() {
  loading.value = true
  try {
    const res = await api.get('/dns/zones')
    zones.value = res.data.data || []
  } finally {
    loading.value = false
  }
}

async function loadWebsiteDomains() {
  try {
    const res = await api.get('/vhost/list')
    websiteDomains.value = (res.data?.data || []).map(item => item?.domain).filter(Boolean)
  } catch {
    websiteDomains.value = []
  }
}

async function loadRecords(domainName) {
  // PowerDNS stores domains ending in dot, we need to extract the raw domain
  const rawDomain = domainName.endsWith('.') ? domainName.slice(0, -1) : domainName
  recordsLoading.value = true
  try {
    const res = await api.get(`/dns/zones/${rawDomain}/records`)
    records.value = res.data.data || []
  } finally {
    recordsLoading.value = false
  }
}

async function selectZone(zone) {
  selectedZone.value = zone
  await loadRecords(zone.name)
}

async function addZone() {
  if (!newZone.value) return
  creatingZone.value = true
  try {
    await api.post('/dns/zone', { domain: newZone.value, server_ip: '127.0.0.1' })
    showAddZoneModal.value = false
    newZone.value = ''
    await loadZones()
  } finally {
    creatingZone.value = false
  }
}

async function confirmDeleteZone(zone) {
  if (!confirm(t('dns.confirm_delete_zone'))) return
  const rawDomain = zone.name.endsWith('.') ? zone.name.slice(0, -1) : zone.name
  try {
    await api.delete(`/dns/zones/${rawDomain}`)
    if (selectedZone.value && selectedZone.value.id === zone.id) {
      selectedZone.value = null
      records.value = []
    }
    await loadZones()
  } catch(e) {
    console.error(e)
  }
}

async function reconcileZone(zone) {
  const rawDomain = zone.name.endsWith('.') ? zone.name.slice(0, -1) : zone.name
  reconcilingDomain.value = zone.name
  try {
    await api.post('/dns/reconcile', { domain: rawDomain })
    await loadZones()
    if (selectedZone.value && selectedZone.value.name === zone.name) {
      await loadRecords(zone.name)
    }
  } catch (e) {
    console.error(e)
  } finally {
    reconcilingDomain.value = ''
  }
}

async function toggleDnssec(zone) {
  const rawDomain = zone.name.endsWith('.') ? zone.name.slice(0, -1) : zone.name
  dnssecDomain.value = zone.name
  try {
    await api.post(`/dns/zones/${rawDomain}/dnssec`, { enabled: !zone.dnssec_enabled })
    await loadZones()
    if (selectedZone.value && selectedZone.value.name === zone.name) {
      selectedZone.value = zones.value.find(x => x.name === zone.name) || selectedZone.value
    }
  } catch (e) {
    console.error(e)
  } finally {
    dnssecDomain.value = ''
  }
}

function openAddRecordModal() {
  newRecord.value = {
    record_type: 'A',
    name: selectedZone.value.name, // base name
    content: '',
    ttl: 3600
  }
  showAddRecordModal.value = true
}

async function addRecord() {
  if (!newRecord.value.name || !newRecord.value.content || !selectedZone.value) return
  
  const rawDomain = selectedZone.value.name.endsWith('.') ? selectedZone.value.name.slice(0, -1) : selectedZone.value.name
  addingRecord.value = true
  try {
    await api.post(`/dns/zones/${rawDomain}/records`, newRecord.value)
    showAddRecordModal.value = false
    await loadRecords(selectedZone.value.name)
    await loadZones() // Refresh zone count
  } finally {
    addingRecord.value = false
  }
}

async function deleteRecord(record) {
  if (!confirm(t('dns.confirm_delete_record'))) return
  const rawDomain = selectedZone.value.name.endsWith('.') ? selectedZone.value.name.slice(0, -1) : selectedZone.value.name
  try {
    await api.delete(`/dns/zones/${rawDomain}/records`, {
      params: {
        record_type: record.record_type,
        name: record.name
      }
    })
    await loadRecords(selectedZone.value.name)
    await loadZones() // Refresh zone count
  } catch(e) {
    console.error(e)
  }
}

async function openDefaultNsModal() {
  showNsModal.value = true
  wizardBaseDomain.value = selectedZone.value?.name?.replace(/\.$/, '') || ''
  try {
    const res = await api.get('/dns/default-nameservers')
    if (res.data.data) {
      nsConfig.value.ns1 = res.data.data.ns1
      nsConfig.value.ns2 = res.data.data.ns2
    }
  } catch(e) {
    console.error(e)
  }
}

async function saveDefaultNs() {
  savingNs.value = true
  try {
    await api.post('/dns/default-nameservers', nsConfig.value)
    showNsModal.value = false
  } catch(e) {
    console.error(e)
  } finally {
    savingNs.value = false
  }
}

async function fillNsByWizard() {
  if (!wizardBaseDomain.value) return
  nsWizardLoading.value = true
  try {
    const res = await api.post('/dns/default-nameservers/wizard', {
      base_domain: wizardBaseDomain.value,
    })
    const data = res.data?.data
    if (data) {
      nsConfig.value.ns1 = data.ns1 || nsConfig.value.ns1
      nsConfig.value.ns2 = data.ns2 || nsConfig.value.ns2
    }
  } catch (e) {
    console.error(e)
  } finally {
    nsWizardLoading.value = false
  }
}

async function resetDefaultNs() {
  nsResetLoading.value = true
  try {
    const res = await api.post('/dns/default-nameservers/reset')
    const data = res.data?.data
    if (data) {
      nsConfig.value.ns1 = data.ns1 || ''
      nsConfig.value.ns2 = data.ns2 || ''
    }
  } catch (e) {
    console.error(e)
  } finally {
    nsResetLoading.value = false
  }
}

onMounted(async () => {
  await Promise.all([loadZones(), loadWebsiteDomains()])
})
</script>

