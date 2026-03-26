<template>
  <div class="space-y-6">
    <div class="flex items-center justify-between">
      <div>
        <h1 class="text-2xl font-bold text-white">E-Posta Hesapları (Stalwart)</h1>
        <p class="text-gray-400 mt-1">Domainleriniz için profesyonel e-posta hesapları oluşturun ve yönetin.</p>
      </div>
      <button class="btn-primary">
        <MailPlus class="w-5 h-5" />
        E-Posta Oluştur
      </button>
    </div>

    <div class="aura-card text-center py-16">
      <Mail class="w-16 h-16 text-gray-600 mx-auto mb-4" />
      <h2 class="text-xl font-semibold text-white mb-2">E-Posta Servisi Beklemede</h2>
      <p class="text-gray-400 mb-6">Stalwart Mail Server bağlantısı bekleniyor. Buradan Limitsiz E-Posta hesabı açabilirsiniz.</p>
      <button class="btn-secondary mx-auto">Mail Sunucusunu Doğrula</button>
    </div>
  </div>
</template>

<script setup>
import { Mail, MailPlus } from 'lucide-vue-next'
</script>
