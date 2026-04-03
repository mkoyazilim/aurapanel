import { beforeAll, describe, expect, it, vi } from 'vitest'

let router

beforeAll(async () => {
  vi.doMock('../layouts/DashboardLayout.vue', () => ({ default: { template: '<div />' } }))
  vi.doMock('../views/Dashboard.vue', () => ({ default: { template: '<div />' } }))
  vi.doMock('../views/Websites.vue', () => ({ default: { template: '<div />' } }))
  vi.doMock('../views/Login.vue', () => ({ default: { template: '<div />' } }))

  router = (await import('./index')).default
})

describe('router smoke', () => {
  it('contains critical authenticated routes', () => {
    const names = router.getRoutes().map((r) => String(r.name || ''))

    expect(names).toContain('Login')
    expect(names).toContain('Websites')
    expect(names).toContain('Databases')
    expect(names).toContain('DNS')
    expect(names).toContain('Emails')
    expect(names).toContain('FTP')
    expect(names).toContain('SSL')
    expect(names).toContain('MinIO')
    expect(names).toContain('PanelPort')
    expect(names).toContain('PanelUpdate')
  })
})
