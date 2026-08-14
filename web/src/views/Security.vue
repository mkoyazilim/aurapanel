<template>
  <Layout>
    <div class="page-header">
      <div class="page-title-row">
        <div>
          <h2>{{ $t('security.title') }}</h2>
          <p class="muted">{{ $t('security.subtitle') }}</p>
        </div>
        <div class="header-actions">
          <select v-model="selectedSite" @change="loadSecurity" class="select-site">
            <option value="" disabled>{{ $t('security.select_site') }}</option>
            <option v-for="s in sites" :key="s.id" :value="s.id">{{ s.name }} ({{ s.id }})</option>
          </select>
        </div>
      </div>
    </div>

    <div v-if="!selectedSite" class="empty-state card">
      <p class="muted">{{ $t('security.select_site_prompt') }}</p>
    </div>

    <div v-else-if="loading" class="loading-state">
      <p>{{ $t('common.loading') }}</p>
    </div>

    <div v-else class="security-grid">
      <div
        v-for="p in profiles"
        :key="p.id"
        class="card profile-card"
        :class="{ 'active-profile': currentProfile === p.id }"
      >
        <div class="profile-header">
          <div>
            <h3>{{ p.label }}</h3>
            <p class="muted text-sm">{{ p.description }}</p>
          </div>
          <span v-if="currentProfile === p.id" class="badge badge-success">
            {{ $t('security.current') }}
          </span>
        </div>

        <div class="profile-features">
          <h4>{{ $t('security.rules_applied') }}:</h4>
          <ul>
            <li v-for="(rule, idx) in p.settings" :key="idx">
              <code>{{ rule }}</code>
            </li>
          </ul>
        </div>

        <div class="profile-action">
          <button
            class="btn w-full"
            :class="currentProfile === p.id ? 'btn-secondary' : 'btn-primary'"
            :disabled="currentProfile === p.id || saving"
            @click="applyProfile(p.id)"
          >
            {{ currentProfile === p.id ? $t('security.active_badge') : $t('security.apply_button') }}
          </button>
        </div>
      </div>
    </div>
  </Layout>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import Layout from '../components/Layout.vue'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()
const sites = ref([])
const selectedSite = ref('')
const currentProfile = ref('minimal')
const profiles = ref([])
const loading = ref(false)
const saving = ref(false)

async function fetchSites() {
  try {
    const res = await fetch('/api/v1/sites')
    if (res.ok) {
      sites.value = await res.json()
      if (sites.value.length > 0) {
        selectedSite.value = sites.value[0].id
        loadSecurity()
      }
    }
  } catch (err) {
    console.error('Siteler alınamadı:', err)
  }
}

async function loadSecurity() {
  if (!selectedSite.value) return
  loading.value = true
  try {
    const res = await fetch(`/api/v1/sites/${selectedSite.value}/security`)
    if (res.ok) {
      const data = await res.json()
      currentProfile.value = data.profile
      profiles.value = data.profiles || []
    }
  } catch (err) {
    console.error('Güvenlik profili yüklenemedi:', err)
  } finally {
    loading.value = false
  }
}

async function applyProfile(profileID) {
  if (!confirm(t('security.apply_confirm'))) return
  saving.value = true
  try {
    const res = await fetch(`/api/v1/sites/${selectedSite.value}/security`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ profile: profileID })
    })
    if (res.ok) {
      currentProfile.value = profileID
      alert(t('security.apply_success'))
    } else {
      const data = await res.json()
      alert(data.error || t('security.apply_failed'))
    }
  } catch (err) {
    alert(err.message)
  } finally {
    saving.value = false
  }
}

onMounted(fetchSites)
</script>

<style scoped>
.page-header { margin-bottom: 24px; }
.page-title-row { display: flex; justify-content: space-between; align-items: center; }
.select-site { padding: 8px 12px; border-radius: 6px; border: 1px solid var(--border-color); background: var(--bg-card); color: var(--text-color); font-size: 14px; }
.empty-state { padding: 48px; text-align: center; }
.security-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(300px, 1fr)); gap: 20px; }
.profile-card { display: flex; flex-direction: column; justify-content: space-between; border: 2px solid transparent; transition: border-color 0.2s; }
.profile-card.active-profile { border-color: #3b82f6; background: rgba(59, 130, 246, 0.02); }
.profile-header { display: flex; justify-content: space-between; align-items: flex-start; margin-bottom: 16px; }
.profile-header h3 { margin: 0 0 4px 0; }
.profile-features { margin-bottom: 24px; }
.profile-features h4 { font-size: 13px; font-weight: 600; margin-bottom: 8px; color: var(--text-muted); }
.profile-features ul { list-style: none; padding: 0; margin: 0; }
.profile-features li { margin-bottom: 6px; }
.profile-features code { font-size: 12px; background: rgba(0,0,0,0.05); padding: 2px 6px; border-radius: 4px; display: inline-block; }
.profile-action { margin-top: auto; }
.badge { padding: 4px 8px; border-radius: 4px; font-size: 12px; font-weight: 500; }
.badge-success { background: #dcfce7; color: #166534; }
.w-full { width: 100%; }
</style>
