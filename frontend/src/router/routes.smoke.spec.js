import { describe, expect, it } from 'vitest'
import router from './index'

describe('router smoke', () => {
  it('contains critical authenticated routes', () => {
    const names = router.getRoutes().map((r) => String(r.name || ''))

    expect(names).toContain('Login')
    expect(names).toContain('Websites')
    expect(names).toContain('Databases')
    expect(names).toContain('DNS')
    expect(names).toContain('Emails')
    expect(names).toContain('MinIO')
    expect(names).toContain('PanelPort')
  })
})
