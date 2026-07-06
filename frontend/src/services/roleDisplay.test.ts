import { describe, expect, it } from 'vitest'
import { formatClusterRole, inferArchFromReplicationMode } from './roleDisplay'

describe('roleDisplay', () => {
  it('keeps MHA manager distinct from HA database roles', () => {
    expect(formatClusterRole('mha', 'manager')).toBe('manager')
    expect(formatClusterRole('mha', 'master')).toBe('master')
    expect(formatClusterRole('mha', 'replica')).toBe('slave')
  })

  it('maps MGR roles independently from replication roles', () => {
    expect(formatClusterRole('mgr', 'master')).toBe('primary')
    expect(formatClusterRole('mgr', 'bootstrap')).toBe('primary')
    expect(formatClusterRole('mgr', 'replica')).toBe('secondary')
    expect(formatClusterRole('mgr', 'secondary')).toBe('secondary')
  })

  it('maps PXC roles independently from MGR and HA', () => {
    expect(formatClusterRole('pxc', 'primary')).toBe('bootstrap')
    expect(formatClusterRole('pxc', 'master')).toBe('bootstrap')
    expect(formatClusterRole('pxc', 'replica')).toBe('secondary')
    expect(formatClusterRole('pxc', 'secondary')).toBe('secondary')
  })

  it('infers cluster architecture from replication mode', () => {
    expect(inferArchFromReplicationMode('group_replication')).toBe('mgr')
    expect(inferArchFromReplicationMode('galera')).toBe('pxc')
    expect(inferArchFromReplicationMode('mha')).toBe('mha')
    expect(inferArchFromReplicationMode('async')).toBe('ha')
  })
})
