import { defineStore } from 'pinia'

function normalizeAction(action) {
  const value = String(action || '').trim().toLowerCase()
  switch (value) {
    case 'loading':
    case 'processing':
    case 'saving':
    case 'updating':
    case 'deleting':
    case 'adding':
    case 'uploading':
      return value
    default:
      return 'processing'
  }
}

export const useRequestStateStore = defineStore('requestState', {
  state: () => ({
    counter: 0,
    order: [],
    pending: {},
    lastAction: 'processing',
  }),
  getters: {
    pendingCount: (state) => state.order.length,
    isBusy: (state) => state.order.length > 0,
    currentKey: (state) => {
      if (state.order.length === 0) {
        return state.lastAction || 'processing'
      }
      const latestId = state.order[state.order.length - 1]
      return state.pending[latestId]?.action || state.lastAction || 'processing'
    },
  },
  actions: {
    start(action) {
      const normalized = normalizeAction(action)
      this.counter += 1
      const id = `req_${Date.now()}_${this.counter}`
      this.pending[id] = {
        action: normalized,
        startedAt: Date.now(),
      }
      this.order.push(id)
      this.lastAction = normalized
      return id
    },
    finish(id) {
      if (!id) {
        return
      }
      const item = this.pending[id]
      if (item) {
        this.lastAction = item.action || this.lastAction
      }
      delete this.pending[id]
      const idx = this.order.indexOf(id)
      if (idx >= 0) {
        this.order.splice(idx, 1)
      }
      if (this.order.length === 0) {
        this.pending = {}
      }
    },
    reset() {
      this.order = []
      this.pending = {}
      this.lastAction = 'processing'
    },
  },
})

