const ROLE_ADMIN = 'admin'
const ROLE_RESELLER = 'reseller'
const ROLE_USER = 'user'

const PATH_RULES = [
  { prefix: '/users', roles: [ROLE_ADMIN] },
  { prefix: '/packages', roles: [ROLE_ADMIN] },
  { prefix: '/reseller', roles: [ROLE_ADMIN] },
  { prefix: '/activity-log', roles: [ROLE_ADMIN] },
  { prefix: '/panel-port', roles: [ROLE_ADMIN] },
  { prefix: '/ols-tuning', roles: [ROLE_ADMIN] },
  { prefix: '/docker', roles: [ROLE_ADMIN] },
  { prefix: '/federated', roles: [ROLE_ADMIN] },
  { prefix: '/ops-center', roles: [ROLE_ADMIN] },
  { prefix: '/cloudflare', roles: [ROLE_ADMIN] },
  { prefix: '/migration', roles: [ROLE_ADMIN, ROLE_RESELLER] },
  { prefix: '/cron-jobs', roles: [ROLE_ADMIN, ROLE_RESELLER] },
  { prefix: '/db-backup', roles: [ROLE_ADMIN, ROLE_RESELLER] },
  { prefix: '/backups', roles: [ROLE_ADMIN, ROLE_RESELLER] },
  { prefix: '/databases', roles: [ROLE_ADMIN, ROLE_RESELLER] },
  { prefix: '/emails', roles: [ROLE_ADMIN, ROLE_RESELLER] },
  { prefix: '/ftp', roles: [ROLE_ADMIN, ROLE_RESELLER] },
  { prefix: '/sftp', roles: [ROLE_ADMIN, ROLE_RESELLER] },
  { prefix: '/minio', roles: [ROLE_ADMIN, ROLE_RESELLER] },
  { prefix: '/wordpress', roles: [ROLE_ADMIN, ROLE_RESELLER] },
  { prefix: '/app-runtime', roles: [ROLE_ADMIN, ROLE_RESELLER] },
  { prefix: '/php', roles: [ROLE_ADMIN, ROLE_RESELLER] },
  { prefix: '/ssl', roles: [ROLE_ADMIN, ROLE_RESELLER, ROLE_USER] },
  { prefix: '/dns', roles: [ROLE_ADMIN, ROLE_RESELLER, ROLE_USER] },
  { prefix: '/websites', roles: [ROLE_ADMIN, ROLE_RESELLER, ROLE_USER] },
  { prefix: '/filemanager', roles: [ROLE_ADMIN, ROLE_RESELLER, ROLE_USER] },
  { prefix: '/terminal', roles: [ROLE_ADMIN, ROLE_RESELLER, ROLE_USER] },
  { prefix: '/security', roles: [ROLE_ADMIN, ROLE_RESELLER, ROLE_USER] },
  { prefix: '/log-viewer', roles: [ROLE_ADMIN, ROLE_RESELLER, ROLE_USER] },
  { prefix: '/server-status', roles: [ROLE_ADMIN, ROLE_RESELLER, ROLE_USER] },
  { prefix: '/', roles: [ROLE_ADMIN, ROLE_RESELLER, ROLE_USER] },
]

const normalizedRules = [...PATH_RULES].sort((a, b) => b.prefix.length - a.prefix.length)

export function normalizeRole(role) {
  const value = String(role || '').trim().toLowerCase()
  if (value === ROLE_ADMIN || value === ROLE_RESELLER || value === ROLE_USER) {
    return value
  }
  return ROLE_USER
}

export function canAccessPath(path, role) {
  const rawPath = String(path || '/').trim() || '/'
  const normalizedPath = rawPath.split('?')[0].split('#')[0] || '/'
  const normalizedRole = normalizeRole(role)
  if (normalizedRole === ROLE_ADMIN) return true

  const rule = normalizedRules.find((item) =>
    normalizedPath === item.prefix || normalizedPath.startsWith(`${item.prefix}/`),
  )
  if (!rule) return false
  return rule.roles.includes(normalizedRole)
}

export const RBAC_ROLES = Object.freeze({
  ADMIN: ROLE_ADMIN,
  RESELLER: ROLE_RESELLER,
  USER: ROLE_USER,
})
