<template>
  <div class="space-y-6">
    <div class="flex items-center justify-between">
      <div>
        <h1 class="text-2xl font-bold text-white">Veritabanları (MySQL & MariaDB)</h1>
        <p class="text-gray-400 mt-1">Veritabanlarını yönetin, kullanıcı bağlayın ve phpMyAdmin/AuraDB arayüzüne erişin.</p>
      </div>
      <button class="btn-primary">
        <Plus class="w-5 h-5" />
        Veritabanı Oluştur
      </button>
    </div>

    <!-- Empty State / DB List -->
    <div class="aura-card text-center py-16">
      <Database class="w-16 h-16 text-gray-600 mx-auto mb-4" />
      <h2 class="text-xl font-semibold text-white mb-2">Henüz Bir Veritabanı Yok</h2>
      <p class="text-gray-400 mb-6">Uygulamalarınız için hızlı ve güvenli MariaDB veritabanları oluşturun.</p>
      <button class="btn-secondary mx-auto">Yeni Veritabanı Ekle</button>
    </div>
  </div>
</template>

<script setup>
import { Database, Plus } from 'lucide-vue-next'
</script>
