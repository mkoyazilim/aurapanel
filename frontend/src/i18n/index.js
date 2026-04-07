import { createI18n } from 'vue-i18n'
import en from '../locales/en.json'
import tr from '../locales/tr.json'
import de from '../locales/de.json'
import es from '../locales/es.json'
import fr from '../locales/fr.json'
import pt from '../locales/pt.json'
import ru from '../locales/ru.json'
import bn from '../locales/bn.json'

const STORAGE_KEY = 'aura_locale'
const DEFAULT_LOCALE = 'en'
const supportedLocales = ['en', 'tr', 'de', 'es', 'fr', 'pt', 'ru', 'bn']
const rtlLocales = []

const normalizeLocale = (value) => {
  const locale = String(value || '').trim().toLowerCase()
  if (!locale) return DEFAULT_LOCALE
  if (supportedLocales.includes(locale)) return locale
  const short = locale.split('-')[0]
  return supportedLocales.includes(short) ? short : DEFAULT_LOCALE
}

const resolveInitialLocale = () => {
  if (typeof window === 'undefined') return DEFAULT_LOCALE
  const stored = window.localStorage.getItem(STORAGE_KEY)
  if (stored) return normalizeLocale(stored)
  return normalizeLocale(window.navigator.language || DEFAULT_LOCALE)
}

const applyDocumentLocale = (locale) => {
  if (typeof document === 'undefined') return
  document.documentElement.lang = locale
  document.documentElement.dir = rtlLocales.includes(locale) ? 'rtl' : 'ltr'
}

const initialLocale = resolveInitialLocale()

const i18n = createI18n({
  legacy: false,
  locale: initialLocale,
  fallbackLocale: 'en',
  messages: {
    en,
    tr,
    de,
    es,
    fr,
    pt,
    ru,
    bn,
  },
})

applyDocumentLocale(initialLocale)

export const setAppLocale = (locale) => {
  const normalized = normalizeLocale(locale)
  i18n.global.locale.value = normalized
  if (typeof window !== 'undefined') {
    window.localStorage.setItem(STORAGE_KEY, normalized)
  }
  applyDocumentLocale(normalized)
}

export const getAppLocale = () => normalizeLocale(i18n.global.locale.value)
export { supportedLocales, normalizeLocale }
export default i18n
