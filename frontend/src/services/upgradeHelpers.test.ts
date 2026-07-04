import { describe, expect, it } from 'vitest'
import {
  UPGRADE_STAGES, ROLLING_UPGRADE_STAGES, UPGRADE_SUBSTEPS, strategyOptions,
  activeUpgradeStatuses, terminalUpgradeStatuses,
  isCompletedUpgradeStatus, isFailedUpgradeStatus,
  clampProgress, upgradeStagesFor, inferStepIndex,
  buildUpgradeSteps, currentUpgradeStage, getUpgradeSubsteps,
  type ActiveUpgrade,
} from './upgradeHelpers'

// ---- Constants ----

describe('UPGRADE_STAGES', () => {
  it('has expected stages', () => {
    expect(UPGRADE_STAGES).toEqual(['预检查', '备份数据', '停止服务', '安装新版本', '数据升级', '启动服务', '验证'])
  })
})

describe('ROLLING_UPGRADE_STAGES', () => {
  it('has expected stages', () => {
    expect(ROLLING_UPGRADE_STAGES).toEqual(['集群检查', '升级从库', '从库验证', '角色切换', '升级原主库', '重建拓扑', '最终验证'])
  })
})

describe('UPGRADE_SUBSTEPS', () => {
  it('contains entries for all upgrade stages', () => {
    for (const stage of UPGRADE_STAGES) {
      expect(UPGRADE_SUBSTEPS[stage]).toBeDefined()
      expect(UPGRADE_SUBSTEPS[stage].length).toBeGreaterThan(0)
    }
  })
})

describe('strategyOptions', () => {
  it('has 3 options', () => {
    expect(strategyOptions).toHaveLength(3)
    expect(strategyOptions[0].value).toBe('inplace')
    expect(strategyOptions[1].value).toBe('logical')
    expect(strategyOptions[2].value).toBe('rolling')
  })
})

describe('activeUpgradeStatuses', () => {
  it('includes running/pending/queued/executing', () => {
    expect(activeUpgradeStatuses.has('running')).toBe(true)
    expect(activeUpgradeStatuses.has('pending')).toBe(true)
    expect(activeUpgradeStatuses.has('queued')).toBe(true)
    expect(activeUpgradeStatuses.has('executing')).toBe(true)
  })
})

describe('terminalUpgradeStatuses', () => {
  it('includes completed, failed, error, cancelled, timeout', () => {
    expect(terminalUpgradeStatuses.has('success')).toBe(true)
    expect(terminalUpgradeStatuses.has('completed')).toBe(true)
    expect(terminalUpgradeStatuses.has('failed')).toBe(true)
    expect(terminalUpgradeStatuses.has('error')).toBe(true)
    expect(terminalUpgradeStatuses.has('cancelled')).toBe(true)
    expect(terminalUpgradeStatuses.has('canceled')).toBe(true)
    expect(terminalUpgradeStatuses.has('timeout')).toBe(true)
  })
})

// ---- isCompletedUpgradeStatus ----

describe('isCompletedUpgradeStatus', () => {
  it('returns true for success', () => { expect(isCompletedUpgradeStatus('success')).toBe(true) })
  it('returns true for completed', () => { expect(isCompletedUpgradeStatus('completed')).toBe(true) })
  it('returns true for uppercase', () => { expect(isCompletedUpgradeStatus('SUCCESS')).toBe(true) })
  it('returns false for failed', () => { expect(isCompletedUpgradeStatus('failed')).toBe(false) })
  it('returns false for running', () => { expect(isCompletedUpgradeStatus('running')).toBe(false) })
  it('returns false for undefined', () => { expect(isCompletedUpgradeStatus()).toBe(false) })
})

// ---- isFailedUpgradeStatus ----

describe('isFailedUpgradeStatus', () => {
  it('returns true for failed', () => { expect(isFailedUpgradeStatus('failed')).toBe(true) })
  it('returns true for error', () => { expect(isFailedUpgradeStatus('error')).toBe(true) })
  it('returns true for cancelled', () => { expect(isFailedUpgradeStatus('cancelled')).toBe(true) })
  it('returns true for canceled', () => { expect(isFailedUpgradeStatus('canceled')).toBe(true) })
  it('returns true for timeout', () => { expect(isFailedUpgradeStatus('timeout')).toBe(true) })
  it('returns false for success', () => { expect(isFailedUpgradeStatus('success')).toBe(false) })
  it('returns false for running', () => { expect(isFailedUpgradeStatus('running')).toBe(false) })
  it('returns false for undefined', () => { expect(isFailedUpgradeStatus()).toBe(false) })
})

// ---- clampProgress ----

describe('clampProgress', () => {
  it('returns valid percentage', () => { expect(clampProgress(50)).toBe(50) })
  it('clamps below 0', () => { expect(clampProgress(-10)).toBe(0) })
  it('clamps above 100', () => { expect(clampProgress(150)).toBe(100) })
  it('handles float', () => { expect(clampProgress(50.6)).toBe(50.6) })
  it('returns 0 for NaN', () => { expect(clampProgress(NaN)).toBe(0) })
  it('returns 0 for undefined', () => { expect(clampProgress()).toBe(0) })
  it('returns 0 for null', () => { expect(clampProgress(null as any)).toBe(0) })
})

// ---- upgradeStagesFor ----

describe('upgradeStagesFor', () => {
  it('returns UPGRADE_STAGES for undefined', () => {
    expect(upgradeStagesFor()).toEqual(UPGRADE_STAGES)
  })

  it('returns ROLLING_UPGRADE_STAGES for rolling strategy', () => {
    expect(upgradeStagesFor({ strategy: 'rolling' })).toEqual(ROLLING_UPGRADE_STAGES)
  })

  it('returns ROLLING_UPGRADE_STAGES for rolling upgrade_type', () => {
    expect(upgradeStagesFor({ upgrade_type: 'rolling' })).toEqual(ROLLING_UPGRADE_STAGES)
  })

  it('returns ROLLING_UPGRADE_STAGES for rolling task_type', () => {
    expect(upgradeStagesFor({ task_type: 'rolling' })).toEqual(ROLLING_UPGRADE_STAGES)
  })

  it('returns UPGRADE_STAGES for inplace strategy', () => {
    expect(upgradeStagesFor({ strategy: 'inplace' })).toEqual(UPGRADE_STAGES)
  })
})

// ---- inferStepIndex ----

describe('inferStepIndex', () => {
  it('returns last index for completed status', () => {
    expect(inferStepIndex(0, UPGRADE_STAGES, 'success')).toBe(UPGRADE_STAGES.length - 1)
  })

  it('returns clamped index for failed status with progress', () => {
    const idx = inferStepIndex(50, UPGRADE_STAGES, 'failed')
    expect(idx).toBeGreaterThanOrEqual(0)
    expect(idx).toBeLessThan(UPGRADE_STAGES.length)
  })

  it('uses stage name to find index', () => {
    expect(inferStepIndex(50, UPGRADE_STAGES, 'running', '备份数据')).toBe(1)
  })

  it('defaults to progress-based index', () => {
    const idx = inferStepIndex(50, UPGRADE_STAGES, 'running')
    expect(idx).toBe(Math.floor((50 / 100) * UPGRADE_STAGES.length))
  })
})

// ---- buildUpgradeSteps ----

describe('buildUpgradeSteps', () => {
  const runningUpgrade: ActiveUpgrade = {
    task_id: 't1', instance_id: 'i1',
    status: 'running', progress: 30,
    started_at: '2024-01-01T00:00:00Z',
  }

  it('returns upgrade.steps if already set', () => {
    const result = buildUpgradeSteps({ ...runningUpgrade, steps: [{ name: '自定义', status: 'completed' }] })
    expect(result).toHaveLength(1)
  })

  it('builds steps from stages array', () => {
    const result = buildUpgradeSteps(runningUpgrade)
    expect(result).toHaveLength(UPGRADE_STAGES.length)
    expect(result[0].status).toBe('completed')
    expect(result[1].status).toBe('completed')
    expect(result[2].status).toBe('running')
    expect(result[3].status).toBe('wait')
  })

  it('marks all completed for success status', () => {
    const result = buildUpgradeSteps({ ...runningUpgrade, status: 'success', progress: 100 })
    result.forEach((step) => expect(step.status).toBe('completed'))
  })
})

// ---- currentUpgradeStage ----

describe('currentUpgradeStage', () => {
  it('returns first stage by default', () => {
    const upgrade: ActiveUpgrade = { task_id: 't1', instance_id: 'i1', status: 'pending', progress: 0, started_at: '' }
    expect(currentUpgradeStage(upgrade)).toBe(UPGRADE_STAGES[0])
  })

  it('returns correct stage based on progress', () => {
    const upgrade: ActiveUpgrade = { task_id: 't1', instance_id: 'i1', status: 'running', progress: 30, started_at: '' }
    const stage = currentUpgradeStage(upgrade)
    expect(UPGRADE_STAGES).toContain(stage)
  })

  it('returns last stage for completed', () => {
    const upgrade: ActiveUpgrade = { task_id: 't1', instance_id: 'i1', status: 'success', progress: 100, started_at: '' }
    expect(currentUpgradeStage(upgrade)).toBe(UPGRADE_STAGES[UPGRADE_STAGES.length - 1])
  })
})

// ---- getUpgradeSubsteps ----

describe('getUpgradeSubsteps', () => {
  it('returns substeps from planResult if available', () => {
    const planResult = { upgrade_path: [{ order: 1, name: '步骤A' }, { order: 2, name: '步骤B' }] }
    expect(getUpgradeSubsteps(planResult, '预检查')).toEqual(['步骤A', '步骤B'])
  })

  it('falls back to UPGRADE_SUBSTEPS for known stage', () => {
    expect(getUpgradeSubsteps(null, '预检查')).toEqual(UPGRADE_SUBSTEPS['预检查'])
  })

  it('falls back to 预检查 substeps for unknown stage', () => {
    expect(getUpgradeSubsteps(null, '未知阶段')).toEqual(UPGRADE_SUBSTEPS['预检查'])
  })
})
