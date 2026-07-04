import { describe, expect, it } from 'vitest'
import { processStepEvent, type DeployState } from './deployStepHelper'

function makeState(overrides?: Partial<DeployState>): DeployState {
  return {
    deployment_id: 'deploy-001',
    steps: [],
    status: 'running',
    progress: 30,
    message: '',
    ...overrides,
  }
}

describe('processStepEvent', () => {
  it('returns the same state reference when current is null (React bail-out)', () => {
    expect(processStepEvent(null, 'deploy-001', 'Step 1')).toBeNull()
  })

  it('returns the same state reference when deployment_id does not match', () => {
    const state = makeState()
    const result = processStepEvent(state, 'other-deploy', 'Step 1')
    // Must return the same reference so React skips re-render
    expect(result).toBe(state)
  })

  it('returns the same state reference when stepName is empty', () => {
    const state = makeState()
    const result = processStepEvent(state, 'deploy-001', '')
    expect(result).toBe(state)
  })

  it('adds a new step with default status "pending"', () => {
    const state = makeState()
    const result = processStepEvent(state, 'deploy-001', '环境检查')

    expect(result).not.toBeNull()
    expect(result!.steps).toHaveLength(1)
    expect(result!.steps![0]).toMatchObject({
      name: '环境检查',
      status: 'pending',
    })
    expect(result!.steps![0].started_at).toBeDefined()
    expect(result!.steps![0].completed_at).toBeUndefined()
  })

  it('adds a new step with provided status and message', () => {
    const state = makeState()
    const result = processStepEvent(state, 'deploy-001', '安装二进制', 'running', '正在安装 MySQL 8.0')

    expect(result!.steps).toHaveLength(1)
    expect(result!.steps![0]).toMatchObject({
      name: '安装二进制',
      status: 'running',
      message: '正在安装 MySQL 8.0',
    })
  })

  it('updates existing step status and message', () => {
    const state = makeState({
      steps: [{ name: '环境检查', status: 'running', message: '进行中', started_at: '2024-01-01T00:00:00.000Z' }],
    })
    const result = processStepEvent(state, 'deploy-001', '环境检查', 'completed', '检查通过')

    expect(result!.steps).toHaveLength(1)
    expect(result!.steps![0]).toMatchObject({
      name: '环境检查',
      status: 'completed',
      message: '检查通过',
      started_at: '2024-01-01T00:00:00.000Z',
    })
    expect(result!.steps![0].completed_at).toBeDefined()
  })

  it('sets started_at when step transitions to running', () => {
    const state = makeState({
      steps: [{ name: '启动节点', status: 'pending' }],
    })
    const result = processStepEvent(state, 'deploy-001', '启动节点', 'running')

    expect(result!.steps![0].started_at).toBeDefined()
    expect(result!.steps![0].completed_at).toBeUndefined()
  })

  it('sets completed_at when step completes', () => {
    const state = makeState({
      steps: [{ name: '启动节点', status: 'running', started_at: '2024-01-01T00:00:00.000Z' }],
    })
    const result = processStepEvent(state, 'deploy-001', '启动节点', 'completed')

    expect(result!.steps![0].status).toBe('completed')
    expect(result!.steps![0].completed_at).toBeDefined()
    // Verify completed_at is after started_at
    expect(new Date(result!.steps![0].completed_at!).getTime())
      .toBeGreaterThanOrEqual(new Date(result!.steps![0].started_at!).getTime())
  })

  it('sets completed_at when step fails', () => {
    const state = makeState({
      steps: [{ name: '启动节点', status: 'running', started_at: '2024-01-01T00:00:00.000Z' }],
    })
    const result = processStepEvent(state, 'deploy-001', '启动节点', 'failed', '端口被占用')

    expect(result!.steps![0].status).toBe('failed')
    expect(result!.steps![0].message).toBe('端口被占用')
    expect(result!.steps![0].completed_at).toBeDefined()
  })

  it('keeps existing message when no new message provided', () => {
    const state = makeState({
      steps: [{ name: 'Step 1', status: 'running', message: '原始消息' }],
    })
    const result = processStepEvent(state, 'deploy-001', 'Step 1', 'completed')

    expect(result!.steps![0].message).toBe('原始消息')
  })

  it('overwrites message when provided', () => {
    const state = makeState({
      steps: [{ name: 'Step 1', status: 'running', message: '原始消息' }],
    })
    const result = processStepEvent(state, 'deploy-001', 'Step 1', 'completed', '新消息')

    expect(result!.steps![0].message).toBe('新消息')
  })

  it('preserves other deployment fields (status, progress, message)', () => {
    const state = makeState({ status: 'running', progress: 50, message: '部署中' })
    const result = processStepEvent(state, 'deploy-001', 'Step A', 'running')

    expect(result!.status).toBe('running')
    expect(result!.progress).toBe(50)
    expect(result!.message).toBe('部署中')
  })

  it('does not mutate the original state', () => {
    const state = makeState({
      steps: [{ name: 'Step 1', status: 'pending' }],
    })
    const originalSteps = state.steps
    processStepEvent(state, 'deploy-001', 'Step 1', 'running')

    // Original should be unchanged
    expect(state.steps![0].status).toBe('pending')
    // Should return a new object
    expect(state.steps).toBe(originalSteps)
  })

  it('handles multiple step events in sequence', () => {
    let state = makeState()

    // Step 1 starts
    state = processStepEvent(state, 'deploy-001', '环境检查', 'running', '开始检查')!
    expect(state.steps).toHaveLength(1)

    // New step while first is still running
    state = processStepEvent(state, 'deploy-001', '安装二进制', 'running', '开始安装')!
    expect(state.steps).toHaveLength(2)

    // First step completes
    state = processStepEvent(state, 'deploy-001', '环境检查', 'completed', '检查通过')!
    expect(state.steps).toHaveLength(2)
    expect(state.steps![0].status).toBe('completed')
    expect(state.steps![0].completed_at).toBeDefined()
    expect(state.steps![1].status).toBe('running')
    expect(state.steps![1].completed_at).toBeUndefined()
  })

  it('overwrites status when new status is provided for existing step', () => {
    const state = makeState({
      steps: [{ name: '配置集群', status: 'running' }],
    })
    const result = processStepEvent(state, 'deploy-001', '配置集群', 'failed', '配置失败')

    expect(result!.steps![0].status).toBe('failed')
  })
})
