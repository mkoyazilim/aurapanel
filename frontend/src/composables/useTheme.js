import { computed, ref } from 'vue'

const STORAGE_KEY = 'aura_theme'
const DEFAULT_THEME = 'dark'
const SUPPORTED_THEMES = ['dark', 'light']

const themeState = ref(DEFAULT_THEME)
let initialized = false

const normalizeTheme = (value) => {
  const candidate = String(value || '').trim().toLowerCase()
  return SUPPORTED_THEMES.includes(candidate) ? candidate : DEFAULT_THEME
}

const applyThemeToDocument = (theme) => {
  if (typeof document === 'undefined') return
  const root = document.documentElement
  root.setAttribute('data-theme', theme)
  root.style.colorScheme = theme === 'dark' ? 'dark' : 'light'
}

export const initializeTheme = () => {
  if (initialized) return themeState.value
  initialized = true

  const storedTheme = typeof window !== 'undefined' ? window.localStorage.getItem(STORAGE_KEY) : ''
  themeState.value = normalizeTheme(storedTheme)
  applyThemeToDocument(themeState.value)
  return themeState.value
}

export const setTheme = (theme) => {
  const normalized = normalizeTheme(theme)
  themeState.value = normalized
  applyThemeToDocument(normalized)
  if (typeof window !== 'undefined') {
    window.localStorage.setItem(STORAGE_KEY, normalized)
  }
  return normalized
}

export const toggleTheme = () => setTheme(themeState.value === 'dark' ? 'light' : 'dark')

export const useTheme = () => {
  initializeTheme()
  return {
    theme: computed(() => themeState.value),
    isDark: computed(() => themeState.value === 'dark'),
    isLight: computed(() => themeState.value === 'light'),
    setTheme,
    toggleTheme,
  }
}

