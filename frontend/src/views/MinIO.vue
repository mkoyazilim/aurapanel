<template>
  <div class="space-y-6">
    <div class="flex items-center justify-between">
      <div>
        <h1 class="text-2xl font-bold text-white">{{ t('minio.title') }}</h1>
        <p class="text-gray-400 mt-1">{{ t('minio.subtitle') }}</p>
      </div>
      <button class="btn-secondary" @click="refreshAll">{{ t('minio.refresh') }}</button>
    </div>

    <div class="aura-card space-y-3">
      <h2 class="text-lg font-bold text-white">{{ t('minio.create_bucket') }}</h2>
      <div class="flex gap-3">
        <input v-model="bucketName" class="aura-input" :placeholder="t('minio.bucket_placeholder')" />
        <button class="btn-primary" @click="createBucket">{{ t('minio.create') }}</button>
      </div>
    </div>

    <div class="aura-card space-y-3">
      <h2 class="text-lg font-bold text-white">{{ t('minio.credentials') }}</h2>
      <div class="flex gap-3">
        <input v-model="credUser" class="aura-input" :placeholder="t('minio.user_placeholder')" />
        <button class="btn-primary" @click="createCredentials">{{ t('minio.generate') }}</button>
      </div>
      <div v-if="creds" class="bg-panel-dark border border-panel-border rounded-lg p-3 text-sm">
        <p><strong>{{ t('minio.access_key') }}:</strong> {{ creds.access_key }}</p>
        <p><strong>{{ t('minio.secret_key') }}:</strong> {{ creds.secret_key }}</p>
      </div>
    </div>

    <div class="aura-card">
      <h2 class="text-lg font-bold text-white mb-3">{{ t('minio.bucket_list') }}</h2>
      <div class="space-y-2">
        <div v-for="bucket in buckets" :key="bucket" class="bg-panel-dark border border-panel-border rounded-lg p-3 text-white">
          {{ bucket }}
        </div>
        <div v-if="buckets.length === 0" class="text-gray-400 text-sm">{{ t('minio.empty') }}</div>
      </div>
    </div>

    <div class="aura-card space-y-4">
      <div>
        <h2 class="text-lg font-bold text-white">{{ t('minio.aws_title') }}</h2>
        <p class="text-gray-400 text-sm mt-1">{{ t('minio.aws_subtitle') }}</p>
      </div>

      <div class="grid grid-cols-1 md:grid-cols-2 gap-3">
        <input v-model="s3.region" class="aura-input" :placeholder="t('minio.aws_region_placeholder')" />
        <input v-model="s3.bucket" class="aura-input" :placeholder="t('minio.aws_bucket_placeholder')" />
        <input v-model="s3.access_key" class="aura-input" :placeholder="t('minio.access_key')" />
        <input v-model="s3.secret_key" type="password" class="aura-input" :placeholder="t('minio.aws_secret_placeholder')" />
        <input v-model="s3.endpoint" class="aura-input md:col-span-2" :placeholder="t('minio.aws_endpoint_placeholder')" />
      </div>

      <label class="inline-flex items-center gap-2 text-sm text-gray-300">
        <input v-model="s3.use_path_style" type="checkbox" class="rounded border-panel-border bg-panel-dark" />
        <span>{{ t('minio.aws_use_path_style') }}</span>
      </label>

      <p v-if="s3.has_secret && !s3.secret_key" class="text-xs text-gray-400">
        {{ t('minio.aws_secret_kept') }}
      </p>

      <div class="flex gap-3">
        <button class="btn-secondary" :disabled="s3Busy" @click="saveS3Config">{{ t('minio.aws_save') }}</button>
        <button class="btn-primary" :disabled="s3Busy" @click="testS3Config">{{ t('minio.aws_test') }}</button>
      </div>

      <p v-if="s3Message" class="text-sm text-green-400">{{ s3Message }}</p>
      <p v-if="s3Error" class="text-sm text-red-400">{{ s3Error }}</p>
    </div>
  </div>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import api from '../services/api'

const { t } = useI18n({ useScope: 'global' })

const bucketName = ref('')
const credUser = ref('admin')
const buckets = ref([])
const creds = ref(null)
const s3 = ref({
  provider: 'aws',
  region: 'us-east-1',
  bucket: '',
  endpoint: '',
  access_key: '',
  secret_key: '',
  has_secret: false,
  use_path_style: false
})
const s3Busy = ref(false)
const s3Message = ref('')
const s3Error = ref('')

async function loadBuckets() {
  const res = await api.get('/storage/minio/buckets')
  buckets.value = res.data.data || []
}

async function createBucket() {
  if (!bucketName.value) return
  await api.post('/storage/minio/buckets', { bucket_name: bucketName.value })
  bucketName.value = ''
  await loadBuckets()
}

async function createCredentials() {
  const res = await api.post('/storage/minio/credentials', { user: credUser.value })
  creds.value = res.data.data
}

async function loadS3Config() {
  const res = await api.get('/storage/minio/s3-config')
  const data = res.data?.data || {}
  s3.value = {
    provider: data.provider || 'aws',
    region: data.region || 'us-east-1',
    bucket: data.bucket || '',
    endpoint: data.endpoint || '',
    access_key: data.access_key || '',
    secret_key: '',
    has_secret: Boolean(data.has_secret),
    use_path_style: Boolean(data.use_path_style)
  }
}

async function saveS3Config() {
  s3Busy.value = true
  s3Message.value = ''
  s3Error.value = ''
  try {
    const payload = {
      provider: s3.value.provider || 'aws',
      region: s3.value.region,
      bucket: s3.value.bucket,
      endpoint: s3.value.endpoint,
      access_key: s3.value.access_key,
      secret_key: s3.value.secret_key,
      use_path_style: s3.value.use_path_style
    }
    const res = await api.post('/storage/minio/s3-config', payload)
    const data = res.data?.data || {}
    s3.value.has_secret = Boolean(data.has_secret)
    s3.value.secret_key = ''
    s3Message.value = t('minio.aws_saved')
  } catch (error) {
    s3Error.value = error?.response?.data?.message || t('minio.aws_save_failed')
  } finally {
    s3Busy.value = false
  }
}

async function testS3Config() {
  s3Busy.value = true
  s3Message.value = ''
  s3Error.value = ''
  try {
    const payload = {
      provider: s3.value.provider || 'aws',
      region: s3.value.region,
      bucket: s3.value.bucket,
      endpoint: s3.value.endpoint,
      access_key: s3.value.access_key,
      secret_key: s3.value.secret_key,
      use_path_style: s3.value.use_path_style
    }
    await api.post('/storage/minio/s3-test', payload)
    s3Message.value = t('minio.aws_test_ok')
  } catch (error) {
    s3Error.value = error?.response?.data?.message || t('minio.aws_test_failed')
  } finally {
    s3Busy.value = false
  }
}

async function refreshAll() {
  await Promise.all([loadBuckets(), loadS3Config()])
}

onMounted(refreshAll)
</script>
