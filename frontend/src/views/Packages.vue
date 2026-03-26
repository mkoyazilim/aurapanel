<template>
  <div class="space-y-6">
    <div class="flex items-center justify-between">
      <div>
        <h1 class="text-2xl font-bold text-white">Paket Yönetimi</h1>
        <p class="text-gray-400 mt-1">Hosting, Reseller ve diğer satış/müşteri paketlerinizin limitlerini belirleyin.</p>
      </div>
      <button class="btn-primary">
        <Plus class="w-5 h-5" />
        Yeni Paket Oluştur
      </button>
    </div>

    <div class="grid grid-cols-1 md:grid-cols-3 gap-6">
      <div v-for="pkg in packages" :key="pkg.name" class="aura-card">
        <div class="flex items-center justify-between mb-4">
          <h3 class="text-lg font-bold text-white">{{ pkg.name }}</h3>
          <span class="px-2 py-1 text-xs font-semibold bg-panel-dark rounded border border-panel-border whitespace-nowrap">{{ pkg.type }}</span>
        </div>
        
        <ul class="space-y-3 mb-6">
          <li class="flex items-center text-sm text-gray-300">
            <HardDrive class="w-4 h-4 text-brand-500 mr-3" />
            <span class="font-medium text-white w-24">Disk:</span> {{ pkg.disk }}
          </li>
          <li class="flex items-center text-sm text-gray-300">
            <Activity class="w-4 h-4 text-blue-500 mr-3" />
            <span class="font-medium text-white w-24">Trafik:</span> {{ pkg.bw }}
          </li>
          <li class="flex items-center text-sm text-gray-300">
            <Globe class="w-4 h-4 text-purple-500 mr-3" />
            <span class="font-medium text-white w-24">Domain:</span> {{ pkg.domain }}
          </li>
          <li class="flex items-center text-sm text-gray-300">
            <Database class="w-4 h-4 text-orange-500 mr-3" />
            <span class="font-medium text-white w-24">Veritabanı:</span> {{ pkg.db }}
          </li>
          <li class="flex items-center text-sm text-gray-300">
            <Mail class="w-4 h-4 text-green-500 mr-3" />
            <span class="font-medium text-white w-24">E-Posta:</span> {{ pkg.email }}
          </li>
        </ul>
        
        <div class="flex items-center gap-3">
          <button class="btn-secondary flex-1 justify-center text-sm">Düzenle</button>
          <button class="btn-danger p-2" title="Sil"><Trash2 class="w-4 h-4" /></button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { Plus, Check, HardDrive, Activity, Globe, Database, Mail, Trash2 } from 'lucide-vue-next'

const packages = [
  { name: 'Başlangıç Hosting', type: 'Hosting Planı', disk: '10 GB NVMe', bw: 'Sınırsız', domain: '1 Adet', db: '3 Adet', email: '10 Adet' },
  { name: 'Kurumsal Hosting', type: 'Hosting Planı', disk: '50 GB NVMe', bw: 'Sınırsız', domain: 'Sınırsız', db: 'Sınırsız', email: 'Sınırsız' },
  { name: 'Bayi (Reseller) V1', type: 'Reseller Planı', disk: '100 GB NVMe', bw: 'Sınırsız', domain: 'Limitsiz Alt Hesap', db: 'Limitsiz', email: 'Limitsiz' },
]
</script>
