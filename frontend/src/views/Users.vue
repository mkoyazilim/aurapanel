<template>
  <div class="space-y-6">
    <div class="flex items-center justify-between">
      <div>
        <h1 class="text-2xl font-bold text-white">Kullanıcı Yönetimi</h1>
        <p class="text-gray-400 mt-1">Sistemdeki tüm müşterileri ve bayileri (reseller) yönetin.</p>
      </div>
      <button class="btn-primary">
        <UserPlus class="w-5 h-5" />
        Yeni Kullanıcı
      </button>
    </div>

    <!-- User List -->
    <div class="space-y-4">
      <div v-for="user in users" :key="user.id" class="aura-card flex flex-col sm:flex-row gap-6 justify-between items-start sm:items-center">
        
        <div class="flex items-center gap-4">
          <div class="w-12 h-12 rounded-full bg-gradient-to-tr from-brand-600 to-panel-border flex items-center justify-center font-bold text-lg text-white">
            {{ user.name.charAt(0) }}
          </div>
          <div>
            <h3 class="text-lg font-bold text-white flex items-center gap-2">
              {{ user.name }}
              <span class="px-2 py-0.5 rounded text-xs font-semibold bg-panel-dark border border-panel-border">{{ user.package }}</span>
              <span v-if="user.role === 'Reseller'" class="px-2 py-0.5 rounded text-xs font-semibold bg-brand-500/10 text-brand-400 border border-brand-500/20">Reseller</span>
            </h3>
            <div class="text-sm text-gray-400 mt-1 flex items-center gap-4">
              <span>{{ user.email }}</span>
              <span class="flex items-center gap-1"><Globe class="w-4 h-4" /> {{ user.sites }} Site</span>
            </div>
          </div>
        </div>

        <div class="flex items-center gap-2 w-full sm:w-auto">
          <button class="btn-secondary px-3 py-1.5 text-sm">Giriş Yap</button>
          <button class="btn-secondary px-3 py-1.5 text-sm">Düzenle</button>
          <button class="btn-danger p-2" title="Kullanıcıyı Sil">
            <UserMinus class="w-4 h-4" />
          </button>
        </div>

      </div>
    </div>
  </div>
</template>

<script setup>
import { UserPlus, UserMinus, Globe } from 'lucide-vue-next'

const users = [
  { id: 1, name: 'Metin Bey', email: 'metin@example.com', package: 'Kurumsal Hosting', role: 'User', sites: 2 },
  { id: 2, name: 'Aura Studio', email: 'hello@aurastudio.com', package: 'Bayi (Reseller) V1', role: 'Reseller', sites: 15 },
  { id: 3, name: 'Test Müşteri', email: 'test@customer.com', package: 'Başlangıç Hosting', role: 'User', sites: 1 },
]
</script>
