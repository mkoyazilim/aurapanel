<template>
  <div class="space-y-6">
    <div class="flex items-center justify-between">
      <div>
        <h1 class="text-2xl font-bold text-white">DNS Yönetimi (PowerDNS)</h1>
        <p class="text-gray-400 mt-1">Alan adlarınız için A, CNAME, TXT, MX kayıtlarını DNS bölgelerinden ayarlayın.</p>
      </div>
      <button class="btn-primary">
        <Plus class="w-5 h-5" />
        Yeni DNS Zone
      </button>
    </div>

    <div class="aura-card text-center py-16">
      <Network class="w-16 h-16 text-gray-600 mx-auto mb-4" />
      <h2 class="text-xl font-semibold text-white mb-2">DNS Kaydı Bulunamadı</h2>
      <p class="text-gray-400 mb-6">Alan adlarınızı AuraPanel NameServer'larına yönlendirip buradan yönetin.</p>
      <button class="btn-secondary mx-auto">PowerDNS Senkronizasyonu</button>
    </div>
  </div>
</template>

<script setup>
import { Network, Plus } from 'lucide-vue-next'
</script>
