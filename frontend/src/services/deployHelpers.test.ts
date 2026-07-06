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
  deploymentPayloadFingerprint,
  normalizeDeployment,
  buildDeployPayload,
  type DeployStepView,
  type ArchType,
} from './deployHelpers'

describe('deploymentPayloadFingerprint', () => {
  const basePayload = {
    cluster_id: 'cluster-1',
    cluster_type: 'ha',
    mysql: { version: '8.0.36', password: 'secret' },
    nodes: [
      { host_id: 'host-1', role: 'master', mysql_port: 3306 },
      { host_id: 'host-2', role: 'replica', mysql_port: 3306 },
    ],
    custom: {
      middleware: { proxysql: { enabled: false } },
      tools: { health_check: { enabled: true } },
    },
  }

  it('ignores secret-only changes', () => {
    expect(deploymentPayloadFingerprint(basePayload)).toBe(deploymentPayloadFingerprint({
      ...basePayload,
      mysql: { ...basePayload.mysql, password: 'changed' },
    }))
  })

  it('changes when topology changes', () => {
    expect(deploymentPayloadFingerprint(basePayload)).not.toBe(deploymentPayloadFingerprint({
      ...basePayload,
      nodes: [...basePayload.nodes, { host_id: 'host-3', role: 'replica', mysql_port: 3307 }],
    }))
  })
})

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
    expect(getStatusCategory('interrupted')).toBe('failed')
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
  it('returns true for interrupted', () => { expect(isFailedDeployStatus('interrupted')).toBe(true) })
  it('returns false for success', () => { expect(isFailedDeployStatus('success')).toBe(false) })
})

describe('isTerminalDeployStatus', () => {
  it('returns true for success/failed/partial/destroyed', () => {
    expect(isTerminalDeployStatus('success')).toBe(true)
    expect(isTerminalDeployStatus('failed')).toBe(true)
    expect(isTerminalDeployStatus('interrupted')).toBe(true)
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

// ---- deploymentProgressStatus ----

describe('deploymentProgressStatus', () => {
  it('returns exception for failed', () => {
    expect(deploymentProgressStatus('failed')).toBe('exception')
    expect(deploymentProgressStatus('error')).toBe('exception')
    expect(deploymentProgressStatus('timeout')).toBe('exception')
  })

  it('returns success for completed/destroyed', () => {
    expect(deploymentProgressStatus('success')).toBe('success')
    expect(deploymentProgressStatus('completed')).toBe('success')
    expect(deploymentProgressStatus('destroyed')).toBe('success')
  })

  it('returns normal for partial', () => {
    expect(deploymentProgressStatus('partial')).toBe('normal')
  })

  it('returns active for running/pending', () => {
    expect(deploymentProgressStatus('running')).toBe('active')
    expect(deploymentProgressStatus('pending')).toBe('active')
  })

  it('returns active for undefined', () => {
    expect(deploymentProgressStatus()).toBe('active')
  })
})

// ---- deploymentStepStatus ----

describe('deploymentStepStatus', () => {
  it('returns error for failed', () => {
    expect(deploymentStepStatus('failed')).toBe('error')
    expect(deploymentStepStatus('error')).toBe('error')
  })

  it('returns finish for completed', () => {
    expect(deploymentStepStatus('success')).toBe('finish')
    expect(deploymentStepStatus('destroyed')).toBe('finish')
  })

  it('returns error for partial', () => {
    expect(deploymentStepStatus('partial')).toBe('error')
  })

  it('returns process for running', () => {
    expect(deploymentStepStatus('running')).toBe('process')
  })
})

// ---- isPartialDeployStatus / isDestroyedDeployStatus ----

describe('isPartialDeployStatus', () => {
  it('returns true for partial', () => { expect(isPartialDeployStatus('partial')).toBe(true) })
  it('returns true for partial_success', () => { expect(isPartialDeployStatus('partial_success')).toBe(true) })
  it('returns false for success', () => { expect(isPartialDeployStatus('success')).toBe(false) })
})

describe('isDestroyedDeployStatus', () => {
  it('returns true for destroyed', () => { expect(isDestroyedDeployStatus('destroyed')).toBe(true) })
  it('returns false for success', () => { expect(isDestroyedDeployStatus('success')).toBe(false) })
})

// ---- buildDeployPayload ----

describe('buildDeployPayload', () => {
  const credential = { username: 'root', password: 'Root#1234' }

  it('builds HA payload', () => {
    const result = buildDeployPayload('ha', {
      cluster_id: 'ha-cluster-01',
      mysql_version: '8.0.36',
      mysql_port: 3306,
      master_host_id: 'host-1',
      replica_host_ids: ['host-2'],
      replica_port: 3307,
      repl_user: 'repl',
      repl_password: 'Repl#2024',
      enable_keepalived: true,
    }, credential)
    expect(result.cluster_id).toBe('ha-cluster-01')
    expect(result.cluster_type).toBe('ha')
    expect(result.mysql.version).toBe('8.0.36')
    expect(result.nodes).toHaveLength(2)
    expect(result.nodes[0].role).toBe('master')
    expect(result.nodes[1].role).toBe('replica')
    expect(result.replication.mode).toBe('async')
  })

  it('builds MHA payload', () => {
    const result = buildDeployPayload('mha', {
      cluster_id: 'mha-cluster-01',
      mysql_version: '8.0',
      master_host_id: 'host-1',
      replica_host_ids: ['host-2'],
      manager_host_id: 'host-3',
      vip: '192.168.1.100',
      ssh_user: 'root',
    }, credential)
    expect(result.cluster_type).toBe('mha')
    expect(result.nodes).toHaveLength(3)
    expect(result.nodes[0].role).toBe('manager')
    expect(result.nodes[1].role).toBe('master')
    expect(result.nodes[2].role).toBe('replica')
    expect(result.custom.vip).toBe('192.168.1.100')
    expect(result.custom.ssh_user).toBe('root')
  })

  it('builds MGR payload', () => {
    const result = buildDeployPayload('mgr', {
      cluster_id: 'mgr-cluster-01',
      mysql_version: '8.0.36',
      master_host_id: 'host-1',
      replica_host_ids: ['host-2', 'host-3'],
      group_name: 'aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee',
      local_port: 33061,
    }, credential)
    expect(result.cluster_type).toBe('mgr')
    expect(result.nodes).toHaveLength(3)
    expect(result.nodes[0].role).toBe('primary')
    expect(result.nodes[1].role).toBe('secondary')
    expect(result.replication.mode).toBe('single-primary')
    expect(result.custom.group_name).toBe('aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee')
  })

  it('builds PXC payload', () => {
    const result = buildDeployPayload('pxc', {
      cluster_id: 'pxc-cluster-01',
      mysql_version: '8.0',
      master_host_id: 'host-1',
      replica_host_ids: ['host-2'],
      wsrep_port: 4567,
      sst_method: 'xtrabackup-v2',
    }, credential)
    expect(result.cluster_type).toBe('pxc')
    expect(result.nodes).toHaveLength(2)
    expect(result.nodes[0].role).toBe('bootstrap')
    expect(result.nodes[1].role).toBe('secondary')
    expect(result.replication.mode).toBe('galera')
    expect(result.custom.wsrep_port).toBe(4567)
    expect(result.custom.sst_method).toBe('xtrabackup-v2')
  })

  it('handles single replica_host_id', () => {
    const result = buildDeployPayload('ha', {
      cluster_id: 'test',
      mysql_version: '8.0',
      master_host_id: 'host-1',
      replica_host_id: 'host-2',
    }, credential)
    expect(result.nodes).toHaveLength(2)
    expect(result.nodes[1].role).toBe('replica')
  })

  it('includes mysql config from text input', () => {
    const result = buildDeployPayload('ha', {
      cluster_id: 'test',
      mysql_version: '8.0',
      master_host_id: 'host-1',
      replica_host_ids: ['host-2'],
      mysql_config_text: 'max_connections=500\ninnodb_buffer_pool_size=2G',
    }, credential)
    expect(result.mysql.config.max_connections).toBe('500')
    expect(result.mysql.config.innodb_buffer_pool_size).toBe('2G')
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
