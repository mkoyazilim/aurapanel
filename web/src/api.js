// API istemcisi: oturum cookie + CSRF başlığı, hata normalizasyonu.
export async function api(path, opts = {}) {
  const method = opts.method || 'GET'
  const headers = { 'Content-Type': 'application/json', ...(opts.headers || {}) }
  const csrf = localStorage.getItem('ap_csrf')
  if (csrf && !['GET', 'HEAD'].includes(method)) headers['X-CSRF-Token'] = csrf

  const res = await fetch('/api/v1' + path, {
    method,
    headers,
    body: opts.body !== undefined ? JSON.stringify(opts.body) : undefined,
  })
  if (res.status === 401 && !path.startsWith('/auth/login')) {
    window.location = '/login'
    throw new Error('oturum yok')
  }
  const data = await res.json().catch(() => ({}))
  if (!res.ok) throw new Error(data.error || `istek hatası (${res.status})`)
  return data
}

export function b64encode(s) {
  const bytes = new TextEncoder().encode(s)
  let bin = ''
  bytes.forEach((b) => (bin += String.fromCharCode(b)))
  return btoa(bin)
}

export function b64decode(b) {
  const bin = atob(b)
  const bytes = Uint8Array.from(bin, (c) => c.charCodeAt(0))
  return new TextDecoder().decode(bytes)
}
