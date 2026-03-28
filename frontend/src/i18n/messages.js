import { extraEn, extraTr } from './panel-extra-messages'

const deepMerge = (base, override) => {
  if (Array.isArray(base) || Array.isArray(override)) {
    return override ?? base
  }
  if (base && typeof base === 'object' && override && typeof override === 'object') {
    const merged = { ...base }
    for (const key of Object.keys(override)) {
      merged[key] = key in base ? deepMerge(base[key], override[key]) : override[key]
    }
    return merged
  }
  return override ?? base
}

// Fetch localized json strings
import enJson from '../locales/en.json'
import trJson from '../locales/tr.json'

const baseEn = enJson
const baseTr = trJson

const en = deepMerge(baseEn, extraEn)
const tr = deepMerge(en, deepMerge(baseTr, extraTr))

const de = deepMerge(en, { common: { active: 'Aktiv', inactive: 'Inaktiv' }, locale: { label: 'Sprache', select: 'Sprache wahlen' }, login: { title: 'Anmelden', email_label: 'E-Mail / Benutzername', password_label: 'Passwort', remember_me: 'Dieses Gerat merken', submit: 'Anmelden', submitting: 'Anmeldung lauft...', error_default: 'Anmeldung fehlgeschlagen.' }, layout: { toggle_all_open: 'Alle Offnen', toggle_all_close: 'Alle Schliessen', toggle_all_label: 'Menugruppen', groups: { web_apps: 'Web & Apps', data_access: 'Daten & Zugriff', security_logs: 'Sicherheit & Protokolle', devops: 'Entwicklung' }, notifications: { title: 'Benachrichtigungen', unread: '{count} ungelesen', mark_all_read: 'Alle als gelesen markieren', clear: 'Leeren', empty: 'Noch keine Benachrichtigungen.', new: 'NEU' }, user_menu: { secure_logout: 'Sicher abmelden' }, footer: { zero_trust: 'Zero-Trust Aktiv', server_load: 'Serverlast' } } })
const es = deepMerge(en, { common: { active: 'Activo', inactive: 'Inactivo' }, locale: { label: 'Idioma', select: 'Seleccionar idioma' }, login: { title: 'Iniciar Sesion', email_label: 'Correo / Usuario', password_label: 'Contrasena', remember_me: 'Recordar este dispositivo', submit: 'Entrar', submitting: 'Iniciando sesion...', error_default: 'No se pudo iniciar sesion.' }, layout: { toggle_all_open: 'Abrir Todo', toggle_all_close: 'Cerrar Todo', toggle_all_label: 'Grupos del Menu', groups: { web_apps: 'Web y Apps', data_access: 'Datos y Acceso', security_logs: 'Seguridad y Registros', devops: 'Desarrollo' }, notifications: { title: 'Notificaciones', unread: '{count} sin leer', mark_all_read: 'Marcar todo como leido', clear: 'Limpiar', empty: 'Todavia no hay notificaciones.', new: 'NUEVO' }, user_menu: { secure_logout: 'Cerrar sesion seguro' }, footer: { zero_trust: 'Zero-Trust Activo', server_load: 'Carga del Servidor' } } })
const fr = deepMerge(en, { common: { active: 'Actif', inactive: 'Inactif' }, locale: { label: 'Langue', select: 'Choisir la langue' }, login: { title: 'Connexion', email_label: 'E-mail / Nom utilisateur', password_label: 'Mot de passe', remember_me: 'Se souvenir de cet appareil', submit: 'Se connecter', submitting: 'Connexion en cours...', error_default: 'Connexion impossible.' }, layout: { toggle_all_open: 'Tout Ouvrir', toggle_all_close: 'Tout Fermer', toggle_all_label: 'Groupes de Menu', groups: { web_apps: 'Web et Apps', data_access: 'Donnees et Acces', security_logs: 'Securite et Journaux', devops: 'Developpement' }, notifications: { title: 'Notifications', unread: '{count} non lues', mark_all_read: 'Tout marquer comme lu', clear: 'Effacer', empty: 'Aucune notification pour le moment.', new: 'NOUVEAU' }, user_menu: { secure_logout: 'Deconnexion securisee' }, footer: { zero_trust: 'Zero-Trust Activo', server_load: 'Charge Serveur' } } })

export const messages = { en, tr, de, es, fr }
export const supportedLocales = ['en', 'tr', 'de', 'es', 'fr']
export const rtlLocales = []
