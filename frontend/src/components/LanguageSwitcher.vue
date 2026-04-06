<template>
  <div v-if="hasMultipleLocales" class="relative">
    <button
      class="flex items-center gap-2 rounded-lg border border-panel-border bg-panel-card/80 px-3 py-2 text-sm text-gray-300 transition hover:border-brand-500/30 hover:text-white"
      @click="open = !open"
    >
      <Languages class="h-4 w-4 text-brand-400" />
      <span>{{ currentLabel }}</span>
      <ChevronDown class="h-4 w-4 text-gray-500 transition" :class="{ 'rotate-180': open }" />
    </button>

    <div
      v-if="open"
      class="absolute right-0 z-50 mt-2 min-w-[170px] overflow-hidden rounded-xl border border-panel-border bg-panel-card shadow-2xl"
    >
      <div class="border-b border-panel-border px-3 py-2 text-[11px] font-semibold uppercase tracking-[0.16em] text-gray-500">
        {{ t('locale.select') }}
      </div>
      <button
        v-for="code in supportedLocales"
        :key="code"
        class="flex w-full items-center justify-between px-3 py-2 text-left text-sm transition hover:bg-panel-dark"
        :class="code === locale ? 'text-white' : 'text-gray-300'"
        @click="selectLocale(code)"
      >
        <span>{{ t(`locale.options.${code}`) }}</span>
        <span v-if="code === locale" class="text-xs font-semibold text-brand-400">{{ t('common.active') }}</span>
      </button>
    </div>
  </div>
</template>

<script setup>
import { computed, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { ChevronDown, Languages } from 'lucide-vue-next'
import { setAppLocale, supportedLocales } from '../i18n'

const { locale, t } = useI18n({ useScope: 'global' })
const route = useRoute()
const open = ref(false)
const hasMultipleLocales = computed(() => supportedLocales.length > 1)

const currentLabel = computed(() => t(`locale.options.${locale.value}`))

const selectLocale = (code) => {
  setAppLocale(code)
  open.value = false
}

watch(() => route.fullPath, () => {
  open.value = false
})
</script>
