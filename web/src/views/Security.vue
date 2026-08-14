<template>
  <Layout>
    <div class="page">
      <div style="margin-bottom: 20px">
        <h1 style="margin: 0; font-size: 22px; display: flex; align-items: center; gap: 8px">
          🛡️ {{ $t('security.title') }}
        </h1>
        <p class="muted text-sm" style="margin: 4px 0 0 0">{{ $t('security.subtitle') }}</p>
      </div>

      <div v-if="error" class="alert error">{{ error }}</div>
      <div v-if="notice" class="alert ok">{{ notice }}</div>

      <!-- Site Seçim Kartı -->
      <div class="card">
        <label style="margin: 0 0 6px 0; font-weight: 600">🌐 Site Seçin</label>
        <select v-model="selectedSite" @change="loadSecurity" style="width: 100%; max-width: 400px">
          <option value="" disabled>{{ $t('security.select_site') }}</option>
          <option v-for="s in sites" :key="s.id" :value="s.id">{{ s.name }} ({{ s.id }})</option>
        </select>
      </div>

      <!-- Site Seçilmemiş Durumu -->
      <div v-if="!selectedSite" class="card empty-state">
        <div style="font-size: 40px; margin-bottom: 12px">🛡️</div>
        <p class="muted">{{ $t('security.select_site_prompt') }}</p>
      </div>

      <!-- Yükleniyor Durumu -->
      <div v-else-if="loading" class="card empty-state">
        <p class="muted">{{ $t('common.loading') }}</p>
      </div>

      <!-- Güvenlik Profilleri Izgarası -->
      <div v-else class="security-grid">
        <div
          v-for="p in profiles"
          :key="p.id"
          class="card profile-card"
          :class="{ 'active-profile': currentProfile === p.id }"
        >
          <div class="profile-header">
            <div>
              <h3 style="margin: 0 0 4px 0; font-size: 17px">
                {{ getProfileIcon(p.id) }} {{ p.label }}
              </h3>
              <p class="muted text-sm" style="margin: 0">{{ p.description }}</p>
            </div>
            <span v-if="currentProfile === p.id" class="badge ok">
              ✓ {{ $t('security.active_badge') }}
            </span>
          </div>

          <div class="profile-features">
            <h4 style="font-size: 13px; font-weight: 600; margin: 16px 0 8px 0; color: var(--muted)">
              {{ $t('security.rules_applied') }}:
            </h4>
            <ul class="rule-list">
              <li v-for="(rule, idx) in p.settings" :key="idx">
                <code class="rule-code mono">{{ rule }}</code>
              </li>
            </ul>
          </div>

          <div class="profile-action" style="margin-top: 20px">
            <button
              class="btn"
              :class="currentProfile === p.id ? '' : 'primary'"
              style="width: 100%"
              :disabled="currentProfile === p.id || saving"
              @click="applyProfile(p.id)"
            >
              {{ currentProfile === p.id ? '✓ ' + $t('security.active_badge') : '⚡ ' + $t('security.apply_button') }}
            </button>
          </div>
        </div>
      </div>
    </div>
  </Layout>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import Layout from '../components/Layout.vue'
import { api } from '../api'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()
const sites = ref([])
const selectedSite = ref('')
const currentProfile = ref('minimal')
const profiles = ref([])
const loading = ref(false)
const saving = ref(false)
const error = ref('')
const notice = ref('')

function getProfileIcon(id) {
  if (id === 'hardened') return '🔒'
  if (id === 'balanced') return '⚖️'
  return '🌱'
}

async function fetchSites() {
  try {
    sites.value = await api('/sites')
    if (sites.value.length > 0) {
      selectedSite.value = sites.value[0].id
      await loadSecurity()
    }
  } catch (err) {
    console.error('Siteler alınamadı:', err)
  }
}

async function loadSecurity() {
  if (!selectedSite.value) return
  loading.value = true
  error.value = ''
  try {
    const data = await api(`/sites/${selectedSite.value}/security`)
    currentProfile.value = data.profile
    profiles.value = data.profiles || []
  } catch (err) {
    error.value = err.message
  } finally {
    loading.value = false
  }
}

async function applyProfile(profileID) {
  if (!confirm(t('security.apply_confirm'))) return
  saving.value = true
  error.value = ''
  notice.value = ''
  try {
    await api(`/sites/${selectedSite.value}/security`, {
      method: 'PUT',
      body: { profile: profileID }
    })
    currentProfile.value = profileID
    notice.value = t('security.apply_success')
  } catch (err) {
    error.value = err.message
  } finally {
    saving.value = false
  }
}

onMounted(fetchSites)
</script>

<style scoped>
.empty-state {
  text-align: center;
  padding: 48px 20px;
}

.security-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(320px, 1fr));
  gap: 20px;
}

.profile-card {
  display: flex;
  flex-direction: column;
  justify-content: space-between;
  border: 2px solid var(--border, #e2e8f0);
  border-radius: 12px;
  transition: all 0.2s ease;
}

.profile-card.active-profile {
  border-color: var(--primary, #4f46e5);
  box-shadow: 0 0 0 1px var(--primary, #4f46e5);
  background: rgba(79, 70, 229, 0.02);
}

.profile-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 8px;
}

.rule-list {
  list-style: none;
  padding: 0;
  margin: 0;
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.rule-code {
  font-size: 11.5px;
  background: #f8fafc;
  border: 1px solid #e2e8f0;
  padding: 3px 8px;
  border-radius: 6px;
  display: inline-block;
  color: #334155;
  word-break: break-all;
}
</style>
