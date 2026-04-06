<template>
  <div class="space-y-6">
    <div class="flex items-center justify-between">
      <div>
        <h1 class="text-2xl font-bold text-white">{{ t('cron_jobs.title') }}</h1>
        <p class="text-gray-400 mt-1">{{ t('cron_jobs.subtitle') }}</p>
      </div>
      <button class="btn-secondary" @click="loadJobs">{{ t('cron_jobs.refresh') }}</button>
    </div>

    <div class="aura-card space-y-3">
      <h2 class="text-lg font-bold text-white">{{ t('cron_jobs.create_title') }}</h2>
      <div class="grid grid-cols-1 md:grid-cols-3 gap-3">
        <input v-model="form.user" class="aura-input" :placeholder="t('cron_jobs.user_placeholder')" />
        <input v-model="form.schedule" class="aura-input" :placeholder="t('cron_jobs.schedule_placeholder')" />
        <input v-model="form.command" class="aura-input" :placeholder="t('cron_jobs.command_placeholder')" />
      </div>
      <button class="btn-primary" @click="createJob">{{ t('cron_jobs.add') }}</button>
    </div>

    <div class="aura-card">
      <h2 class="text-lg font-bold text-white mb-3">{{ t('cron_jobs.list_title') }}</h2>
      <div class="space-y-2">
        <div v-for="job in jobs" :key="job.id" class="bg-panel-dark border border-panel-border rounded-lg p-3 flex items-center justify-between">
          <div>
            <p class="text-white">#{{ job.id }} - {{ job.user }}</p>
            <p class="text-xs text-gray-400">{{ job.schedule }} -> {{ job.command }}</p>
          </div>
          <button class="btn-danger px-3 py-1 text-xs" @click="deleteJob(job.id)">{{ t('cron_jobs.delete') }}</button>
        </div>
        <div v-if="jobs.length === 0" class="text-gray-400 text-sm">{{ t('cron_jobs.empty') }}</div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import api from '../services/api'

const { t } = useI18n({ useScope: 'global' })

const jobs = ref([])
const form = ref({
  user: 'root',
  schedule: '*/5 * * * *',
  command: ''
})

async function loadJobs() {
  const res = await api.get('/monitor/cron/jobs')
  jobs.value = res.data.data || []
}

async function createJob() {
  if (!form.value.command) return
  await api.post('/monitor/cron/jobs', form.value)
  form.value.command = ''
  await loadJobs()
}

async function deleteJob(id) {
  await api.delete('/monitor/cron/jobs', { params: { id } })
  await loadJobs()
}

onMounted(loadJobs)
</script>
