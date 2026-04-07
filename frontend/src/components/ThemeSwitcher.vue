<template>
  <button
    class="inline-flex items-center gap-2 rounded-lg border border-panel-border bg-panel-card/80 px-3 py-2 text-sm text-gray-300 transition hover:border-brand-500/30 hover:text-white"
    :title="buttonTitle"
    @click="toggleTheme"
  >
    <Sun v-if="isDark" class="h-4 w-4 text-brand-400" />
    <Moon v-else class="h-4 w-4 text-brand-400" />
    <span>{{ activeThemeLabel }}</span>
  </button>
</template>

<script setup>
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { Moon, Sun } from 'lucide-vue-next'
import { useTheme } from '../composables/useTheme'

const { t } = useI18n({ useScope: 'global' })
const { isDark, theme, toggleTheme } = useTheme()

const activeThemeLabel = computed(() => (theme.value === 'dark' ? t('theme.dark') : t('theme.light')))
const buttonTitle = computed(() => (isDark.value ? t('theme.switch_to_light') : t('theme.switch_to_dark')))
</script>

