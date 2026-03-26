<template>
  <div class="space-y-6">
    <div class="flex items-center justify-between">
      <div>
        <h1 class="text-2xl font-bold text-white">Kullanıcı & Limit Yönetimi</h1>
        <p class="text-gray-400 mt-1">Sunucunuzdaki müşterileriniz veya kendi projeleriniz için kaynak limitlerini (Disk, Trafik) yönetin.</p>
      </div>
      <button class="btn-primary">
        <UserPlus class="w-5 h-5" />
        Yeni Kullanıcı
      </button>
    </div>

    <!-- Active Packages -->
    <div class="grid grid-cols-1 md:grid-cols-3 gap-6">
      <div v-for="pkg in packages" :key="pkg.name" class="aura-card">
        <div class="flex items-center justify-between mb-4">
          <h3 class="text-lg font-bold text-white">{{ pkg.name }}</h3>
          <span class="px-2 py-1 text-xs font-semibold bg-panel-dark rounded border border-panel-border whitespace-nowrap">{{ pkg.users }} Kullanıcı</span>
        </div>
        
        <ul class="space-y-2 mb-6">
          <li v-for="feature in pkg.features" :key="feature" class="flex items-center text-sm text-gray-300">
            <Check class="w-4 h-4 text-brand-500 mr-2" />
            {{ feature }}
          </li>
        </ul>
        
        <button class="btn-secondary w-full justify-center">Edit Package</button>
      </div>
    </div>
  </div>
</template>

<script setup>
import { UserPlus, Check } from 'lucide-vue-next'

const packages = [
  { 
    name: 'Giriş Limiti', users: 15, 
    features: ['5 GB NVMe Alanı', 'Sınırsız Trafik', '1 Alan Adı', 'PHP 8.x + Redis'] 
  },
  { 
    name: 'Gelişmiş Limit', users: 8, 
    features: ['20 GB NVMe Alanı', 'Sınırsız Trafik', '10 Alan Adı', 'Gelişmiş WAF Güvenliği'] 
  },
  { 
    name: 'Limitsiz VIP', users: 3, 
    features: ['100 GB NVMe Alanı', 'Sınırsız Trafik', 'Sınırsız Alan Adı', 'Özel Kaynak İzolasyonu'] 
  },
]
</script>
