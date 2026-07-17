import { describe, expect, it } from 'vitest'
import { isTaskTerminalStatus } from './api'
import { isCompletedDeployStatus, isFailedDeployStatus, isTerminalDeployStatus } from './deployHelpers'

describe('live deploy progress helpers', () => {
  it('task terminal statuses', () => {
    expect(isTaskTerminalStatus('running')).toBe(false)
    expect(isTaskTerminalStatus('completed')).toBe(true)
    expect(isTaskTerminalStatus('FAILED')).toBe(true)
  })

  it('deployment terminal helpers still work', () => {
    expect(isTerminalDeployStatus('running')).toBe(false)
    expect(isCompletedDeployStatus('completed')).toBe(true)
    expect(isFailedDeployStatus('failed')).toBe(true)
  })
})
