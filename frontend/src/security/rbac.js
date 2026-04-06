const ROLE_ADMIN = 'admin'
const ROLE_RESELLER = 'reseller'
const ROLE_USER = 'user'

const PATH_RULES = [
  { prefix: '/users', roles: [ROLE_ADMIN, ROLE_RESELLER] },
  { prefix: '/packages', roles: [ROLE_ADMIN] },
  { prefix: '/reseller', roles: [ROLE_ADMIN] },
  { prefix: '/activity-log', roles: [ROLE_ADMIN] },
  { prefix: '/panel-control', roles: [ROLE_ADMIN] },
  { prefix: '/api-settings', roles: [ROLE_ADMIN] },
  { prefix: '/panel-update', roles: [ROLE_ADMIN] },
  { prefix: '/panel-port', roles: [ROLE_ADMIN] },
  { prefix: '/ai-tools', roles: [ROLE_ADMIN] },
  { prefix: '/cloudlinux', roles: [ROLE_ADMIN] },
  { prefix: '/ols-tuning', roles: [ROLE_ADMIN] },
  { prefix: '/docker', roles: [ROLE_ADMIN] },
  { prefix: '/federated', roles: [ROLE_ADMIN] },
  { prefix: '/ops-center', roles: [ROLE_ADMIN] },
  { prefix: '/cloudflare', roles: [ROLE_ADMIN] },
  { prefix: '/plugins', roles: [ROLE_ADMIN] },
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
  { prefix: '/mail-tuning', roles: [ROLE_ADMIN, ROLE_RESELLER] },
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

export const ROLE_PERMISSION_CATALOG = Object.freeze([
  { key: 'users.manage', labelKey: 'rbac_permissions.users.manage', label: 'User Management' },
  { key: 'packages.manage', labelKey: 'rbac_permissions.packages.manage', label: 'Package Management' },
  { key: 'reseller.manage', labelKey: 'rbac_permissions.reseller.manage', label: 'Reseller/ACL Management' },
  { key: 'websites.manage', labelKey: 'rbac_permissions.websites.manage', label: 'Website Management' },
  { key: 'dns.manage', labelKey: 'rbac_permissions.dns.manage', label: 'DNS Management' },
  { key: 'databases.manage', labelKey: 'rbac_permissions.databases.manage', label: 'Database Management' },
  { key: 'mail.manage', labelKey: 'rbac_permissions.mail.manage', label: 'Email Management' },
  { key: 'ftp.manage', labelKey: 'rbac_permissions.ftp.manage', label: 'FTP Management' },
  { key: 'sftp.manage', labelKey: 'rbac_permissions.sftp.manage', label: 'SFTP Management' },
  { key: 'backups.manage', labelKey: 'rbac_permissions.backups.manage', label: 'Backup Management' },
  { key: 'apps.manage', labelKey: 'rbac_permissions.apps.manage', label: 'App/WordPress/Plugin' },
  { key: 'runtime.manage', labelKey: 'rbac_permissions.runtime.manage', label: 'Docker/Runtime Management' },
  { key: 'files.manage', labelKey: 'rbac_permissions.files.manage', label: 'File Manager' },
  { key: 'terminal.manage', labelKey: 'rbac_permissions.terminal.manage', label: 'Web Terminal' },
  { key: 'php.manage', labelKey: 'rbac_permissions.php.manage', label: 'PHP Management' },
  { key: 'ssl.manage', labelKey: 'rbac_permissions.ssl.manage', label: 'SSL Management' },
  { key: 'security.manage', labelKey: 'rbac_permissions.security.manage', label: 'Security Management' },
  { key: 'monitoring.read', labelKey: 'rbac_permissions.monitoring.read', label: 'Server Status' },
  { key: 'logs.read', labelKey: 'rbac_permissions.logs.read', label: 'Log Viewer' },
  { key: 'cron.manage', labelKey: 'rbac_permissions.cron.manage', label: 'Cron Management' },
  { key: 'cloudflare.manage', labelKey: 'rbac_permissions.cloudflare.manage', label: 'Cloudflare Management' },
  { key: 'migration.manage', labelKey: 'rbac_permissions.migration.manage', label: 'Migration Wizard' },
  { key: 'cloudlinux.manage', labelKey: 'rbac_permissions.cloudlinux.manage', label: 'CloudLinux Management' },
  { key: 'minio.manage', labelKey: 'rbac_permissions.minio.manage', label: 'MinIO Management' },
  { key: 'activity.read', labelKey: 'rbac_permissions.activity.read', label: 'Activity Logs' },
  { key: 'ops.manage', labelKey: 'rbac_permissions.ops.manage', label: 'Ops/Federated Management' },
  { key: 'panel.manage', labelKey: 'rbac_permissions.panel.manage', label: 'Panel Management' },
  { key: 'ai.manage', labelKey: 'rbac_permissions.ai.manage', label: 'AI Tools' },
])

const PERMISSION_PATH_RULES = [
  { prefix: '/users', permissions: ['users.manage'] },
  { prefix: '/packages', permissions: ['packages.manage', 'users.manage'] },
  { prefix: '/reseller', permissions: ['reseller.manage'] },
  { prefix: '/activity-log', permissions: ['activity.read'] },
  { prefix: '/panel-control', permissions: ['panel.manage'] },
  { prefix: '/api-settings', permissions: ['panel.manage'] },
  { prefix: '/panel-update', permissions: ['panel.manage'] },
  { prefix: '/panel-port', permissions: ['panel.manage'] },
  { prefix: '/ai-tools', permissions: ['ai.manage'] },
  { prefix: '/cloudlinux', permissions: ['cloudlinux.manage'] },
  { prefix: '/ols-tuning', permissions: ['panel.manage'] },
  { prefix: '/mail-tuning', permissions: ['mail.manage'] },
  { prefix: '/docker', permissions: ['runtime.manage'] },
  { prefix: '/federated', permissions: ['ops.manage'] },
  { prefix: '/ops-center', permissions: ['ops.manage'] },
  { prefix: '/cloudflare', permissions: ['cloudflare.manage'] },
  { prefix: '/plugins', permissions: ['apps.manage'] },
  { prefix: '/migration', permissions: ['migration.manage'] },
  { prefix: '/cron-jobs', permissions: ['cron.manage'] },
  { prefix: '/db-backup', permissions: ['backups.manage'] },
  { prefix: '/backups', permissions: ['backups.manage'] },
  { prefix: '/databases', permissions: ['databases.manage'] },
  { prefix: '/emails', permissions: ['mail.manage'] },
  { prefix: '/ftp', permissions: ['ftp.manage'] },
  { prefix: '/sftp', permissions: ['sftp.manage'] },
  { prefix: '/minio', permissions: ['minio.manage'] },
  { prefix: '/wordpress', permissions: ['apps.manage'] },
  { prefix: '/app-runtime', permissions: ['runtime.manage', 'apps.manage'] },
  { prefix: '/php', permissions: ['php.manage'] },
  { prefix: '/ssl', permissions: ['ssl.manage'] },
  { prefix: '/dns', permissions: ['dns.manage'] },
  { prefix: '/websites', permissions: ['websites.manage'] },
  { prefix: '/filemanager', permissions: ['files.manage'] },
  { prefix: '/terminal', permissions: ['terminal.manage'] },
  { prefix: '/security', permissions: ['security.manage'] },
  { prefix: '/log-viewer', permissions: ['logs.read'] },
  { prefix: '/server-status', permissions: ['monitoring.read'] },
]

const normalizedRules = [...PATH_RULES].sort((a, b) => b.prefix.length - a.prefix.length)
const normalizedPermissionRules = [...PERMISSION_PATH_RULES].sort((a, b) => b.prefix.length - a.prefix.length)

export function normalizeRole(role) {
  const value = String(role || '').trim().toLowerCase()
  if (value === ROLE_ADMIN || value === ROLE_RESELLER || value === ROLE_USER) {
    return value
  }
  return ROLE_USER
}

export function normalizePermissions(permissions) {
  if (!Array.isArray(permissions)) return []
  const seen = new Set()
  const out = []
  for (const item of permissions) {
    const key = String(item || '').trim()
    if (!key || seen.has(key)) continue
    seen.add(key)
    out.push(key)
  }
  return out
}

function hasAnyPermission(permissionSet, requiredPermissions) {
  if (permissionSet.has('*')) return true
  for (const required of requiredPermissions) {
    if (permissionSet.has(required)) return true
  }
  return false
}

export function canAccessPath(path, role, permissions = []) {
  const rawPath = String(path || '/').trim() || '/'
  const normalizedPath = rawPath.split('?')[0].split('#')[0] || '/'
  const normalizedRole = normalizeRole(role)
  if (normalizedRole === ROLE_ADMIN) return true

  const normalizedPermissionList = normalizePermissions(permissions)
  if (normalizedPermissionList.length > 0) {
    if (normalizedPath === '/') {
      return true
    }
    const permissionRule = normalizedPermissionRules.find((item) =>
      normalizedPath === item.prefix || normalizedPath.startsWith(`${item.prefix}/`),
    )
    if (!permissionRule) {
      return false
    }
    return hasAnyPermission(new Set(normalizedPermissionList), permissionRule.permissions)
  }

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
