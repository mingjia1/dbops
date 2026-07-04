import { describe, expect, it } from 'vitest'
import {
  normalizeStatus,
  getStatusCategory,
  isCompletedDeployStatus,
  isFailedDeployStatus,
  isPartialDeployStatus,
  isDestroyedDeployStatus,
  isTerminalDeployStatus,
  deploymentProgress,
  deploymentProgressStatus,
  deploymentStepStatus,
  clampProgress,
  stepStatusToAntd,
  stepProgressPercent,
  majorVersion,
  versionSupportsArch,
  createMgrGroupName,
  parseMySQLConfig,
  normalizeDeployment,
  type DeployStepView,
  type ArchType,
} from './deployHelpers'

// ---- normalizeStatus ----

describe('normalizeStatus', () => {
  it('trims and lowercases', () => {
    expect(normalizeStatus('  SUCCESS  ')).toBe('success')
  })

  it('returns empty string for undefined', () => {
    expect(normalizeStatus()).toBe('')
  })

  it('returns empty string for null-ish', () => {
    expect(normalizeStatus(null as any)).toBe('')
  })
})

// ---- getStatusCategory ----

describe('getStatusCategory', () => {
  it('maps success variants to "success"', () => {
    expect(getStatusCategory('success')).toBe('success')
    expect(getStatusCategory('completed')).toBe('success')
    expect(getStatusCategory('succeeded')).toBe('success')
    expect(getStatusCategory('ok')).toBe('success')
  })

  it('maps failure variants to "failed"', () => {
    expect(getStatusCategory('failed')).toBe('failed')
    expect(getStatusCategory('error')).toBe('failed')
    expect(getStatusCategory('timeout')).toBe('failed')
    expect(getStatusCategory('cancelled')).toBe('failed')
    expect(getStatusCategory('canceled')).toBe('failed')
  })

  it('maps partial to "partial"', () => {
    expect(getStatusCategory('partial')).toBe('partial')
    expect(getStatusCategory('partial_success')).toBe('partial')
  })

  it('maps destroyed', () => {
    expect(getStatusCategory('destroyed')).toBe('destroyed')
  })

  it('maps pending', () => {
    expect(getStatusCategory('pending')).toBe('pending')
  })

  it('returns unknown for empty', () => {
    expect(getStatusCategory('')).toBe('unknown')
  })
})

// ---- Status checkers ----

describe('isCompletedDeployStatus', () => {
  it('returns true for success', () => { expect(isCompletedDeployStatus('success')).toBe(true) })
  it('returns false for failed', () => { expect(isCompletedDeployStatus('failed')).toBe(false) })
})

describe('isFailedDeployStatus', () => {
  it('returns true for failed', () => { expect(isFailedDeployStatus('failed')).toBe(true) })
  it('returns false for success', () => { expect(isFailedDeployStatus('success')).toBe(false) })
})

describe('isTerminalDeployStatus', () => {
  it('returns true for success/failed/partial/destroyed', () => {
    expect(isTerminalDeployStatus('success')).toBe(true)
    expect(isTerminalDeployStatus('failed')).toBe(true)
    expect(isTerminalDeployStatus('partial')).toBe(true)
    expect(isTerminalDeployStatus('destroyed')).toBe(true)
  })

  it('returns false for running/pending', () => {
    expect(isTerminalDeployStatus('running')).toBe(false)
    expect(isTerminalDeployStatus('pending')).toBe(false)
  })
})

// ---- deploymentProgress ----

describe('deploymentProgress', () => {
  it('uses progress when provided', () => { expect(deploymentProgress('running', 75)).toBe(75) })

  it('returns 100 for terminal status without progress', () => {
    expect(deploymentProgress('success')).toBe(100)
    expect(deploymentProgress('failed')).toBe(100)
  })

  it('returns 0 for non-terminal without progress', () => {
    expect(deploymentProgress('running')).toBe(0)
    expect(deploymentProgress('pending')).toBe(0)
  })
})

// ---- clampProgress ----

describe('clampProgress', () => {
  it('returns valid percentage', () => { expect(clampProgress(50)).toBe(50) })
  it('clamps below 0', () => { expect(clampProgress(-10)).toBe(0) })
  it('clamps above 100', () => { expect(clampProgress(150)).toBe(100) })
  it('rounds', () => { expect(clampProgress(50.6)).toBe(51) })
  it('returns 0 for NaN', () => { expect(clampProgress(NaN)).toBe(0) })
  it('returns 0 for undefined', () => { expect(clampProgress()).toBe(0) })
})

// ---- stepStatusToAntd ----

describe('stepStatusToAntd', () => {
  it('maps completed/finish', () => { expect(stepStatusToAntd('completed')).toBe('finish') })
  it('maps running/process', () => { expect(stepStatusToAntd('running')).toBe('process') })
  it('maps failed/error', () => { expect(stepStatusToAntd('failed')).toBe('error') })
  it('defaults to wait', () => { expect(stepStatusToAntd('pending')).toBe('wait') })
  it('defaults to wait for undefined', () => { expect(stepStatusToAntd()).toBe('wait') })
})

// ---- stepProgressPercent ----

describe('stepProgressPercent', () => {
  const finished = (name: string): DeployStepView => ({ name, status: 'completed' })
  const running = (name: string): DeployStepView => ({ name, status: 'running' })
  const waiting = (name: string): DeployStepView => ({ name, status: 'pending' })
  const failed = (name: string): DeployStepView => ({ name, status: 'failed' })

  it('returns 100 for finished step', () => {
    expect(stepProgressPercent(finished('A'), 0, [finished('A')])).toBe(100)
  })

  it('returns 100 for finished step even with low overallProgress', () => {
    expect(stepProgressPercent(finished('A'), 0, [finished('A')], 10)).toBe(100)
  })

  it('returns 0 for waiting step', () => {
    expect(stepProgressPercent(waiting('B'), 1, [finished('A'), waiting('B')])).toBe(0)
  })

  it('returns adjusted percent for running step', () => {
    const steps = [finished('A'), running('B'), waiting('C')]
    const percent = stepProgressPercent(running('B'), 1, steps, 50)
    // completedBefore = 1, stepSpan = 100/3 ≈ 33.3
    // (50 - 1*33.3) / 33.3 * 100 = (16.7/33.3)*100 ≈ 50
    expect(percent).toBeGreaterThan(0)
    expect(percent).toBeLessThan(100)
  })

  it('returns at least 5 for failed step with low progress', () => {
    expect(stepProgressPercent(failed('A'), 0, [failed('A')], 0)).toBe(5)
  })
})

// ---- majorVersion ----

describe('majorVersion', () => {
  it('extracts major version number', () => {
    expect(majorVersion('8.0.36')).toBe(8)
    expect(majorVersion('5.7.44')).toBe(5)
    expect(majorVersion('10.11.6')).toBe(10)
  })

  it('returns 0 for invalid input', () => {
    expect(majorVersion('')).toBe(0)
    expect(majorVersion()).toBe(0)
  })
})

// ---- versionSupportsArch ----

describe('versionSupportsArch', () => {
  it('returns true for non-MGR architectures', () => {
    expect(versionSupportsArch('ha', '5.7.44')).toBe(true)
    expect(versionSupportsArch('pxc', '5.7.44')).toBe(true)
  })

  it('requires MySQL 8+ for MGR', () => {
    expect(versionSupportsArch('mgr', '8.0.36')).toBe(true)
    expect(versionSupportsArch('mgr', '5.7.44')).toBe(false)
    expect(versionSupportsArch('mgr', '8.4.0')).toBe(true)
  })
})

// ---- createMgrGroupName ----

describe('createMgrGroupName', () => {
  it('returns a valid UUID-like string', () => {
    const name = createMgrGroupName()
    expect(name).toBeTruthy()
    expect(name.length).toBeGreaterThan(20)
    // Should contain dashes at expected positions (UUID format)
    expect(name).toMatch(/^[a-f0-9-]+$/)
  })
})

// ---- parseMySQLConfig ----

describe('parseMySQLConfig', () => {
  it('parses key=value pairs', () => {
    const result = parseMySQLConfig('max_connections=200\nport=3306')
    expect(result).toEqual({ max_connections: '200', port: '3306' })
  })

  it('skips comments and empty lines', () => {
    const result = parseMySQLConfig('# this is a comment\n\nmax_connections=200')
    expect(result).toEqual({ max_connections: '200' })
  })

  it('handles spaces around =', () => {
    const result = parseMySQLConfig('  max_connections = 200  ')
    expect(result).toEqual({ max_connections: '200' })
  })

  it('returns empty object for empty input', () => {
    expect(parseMySQLConfig('')).toEqual({})
    expect(parseMySQLConfig()).toEqual({})
  })

  it('handles values with = inside', () => {
    const result = parseMySQLConfig('my_cnf=/etc/my.cnf')
    expect(result).toEqual({ my_cnf: '/etc/my.cnf' })
  })
})

// ---- normalizeDeployment ----

describe('normalizeDeployment', () => {
  it('maps fields correctly', () => {
    const raw = {
      deployment_id: 'dep-1',
      cluster_id: 'cluster-1',
      cluster_type: 'ha',
      status: 'running',
      progress: 50,
      stage: '安装二进制',
      message: 'deploying...',
      started_at: '2024-01-01T00:00:00Z',
      nodes: [{ host: '10.0.0.1' }],
      steps: [{ name: 'Step 1' }],
      logs: ['log line'],
    }
    const result = normalizeDeployment(raw)
    expect(result.deployment_id).toBe('dep-1')
    expect(result.cluster_id).toBe('cluster-1')
    expect(result.cluster_type).toBe('ha')
    expect(result.status).toBe('running')
    expect(result.progress).toBe(50)
    expect(result.nodes).toHaveLength(1)
    expect(result.steps).toHaveLength(1)
    expect(result.logs).toHaveLength(1)
  })

  it('falls back to id when deployment_id missing', () => {
    expect(normalizeDeployment({ id: 'fallback-id' }).deployment_id).toBe('fallback-id')
  })

  it('defaults missing arrays to empty', () => {
    const result = normalizeDeployment({})
    expect(result.nodes).toEqual([])
    expect(result.steps).toEqual([])
    expect(result.logs).toEqual([])
  })

  it('defaults missing status to pending', () => {
    expect(normalizeDeployment({}).status).toBe('pending')
  })
})
