import { describe, expect, it, vi } from 'vitest'
import { onLogout, triggerLogout } from './authEvents'

describe('authEvents', () => {
  it('notifies registered logout handlers', () => {
    const handler = vi.fn()
    const dispose = onLogout(handler)

    triggerLogout()

    expect(handler).toHaveBeenCalledTimes(1)
    dispose()
  })

  it('stops notifying after unsubscribe', () => {
    const handler = vi.fn()
    const dispose = onLogout(handler)

    dispose()
    triggerLogout()

    expect(handler).not.toHaveBeenCalled()
  })
})
