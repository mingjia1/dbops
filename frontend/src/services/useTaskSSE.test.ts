import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest'
import { renderHook, act, waitFor } from '@testing-library/react'
import { useTaskSSE, type TaskEvent, type UseTaskSSEOptions } from './useTaskSSE'

// Mock EventSource
class MockEventSource {
  static instances: MockEventSource[] = []
  onopen: (() => void) | null = null
  onmessage: ((msg: { data: string }) => void) | null = null
  onerror: ((err: Event) => void) | null = null
  readyState = 0
  url: string
  closed = false

  constructor(url: string) {
    this.url = url
    MockEventSource.instances.push(this)
    // Simulate connection open
    setTimeout(() => {
      if (!this.closed && this.onopen) this.onopen()
    }, 0)
  }

  close() {
    this.closed = true
  }

  // Helper to simulate receiving a message
  receive(data: string) {
    if (!this.closed && this.onmessage) {
      this.onmessage({ data })
    }
  }

  // Helper to simulate an error
  fail() {
    if (!this.closed && this.onerror) {
      this.onerror(new Event('error'))
    }
  }

  static lastInstance(): MockEventSource | undefined {
    return MockEventSource.instances[MockEventSource.instances.length - 1]
  }

  static reset() {
    MockEventSource.instances = []
  }
}

// Mock global EventSource
const OriginalEventSource = globalThis.EventSource
beforeEach(() => {
  MockEventSource.reset()
  ;(globalThis as any).EventSource = MockEventSource as any
  // P0-6: 连接流程先 fetch 一次性 ticket。测试环境让 fetch 直接拒绝,
  // hook 走降级分支 (裸 URL + cookie), 行为确定且无真实网络请求。
  vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new Error('no ticket endpoint in tests')))
})

afterEach(() => {
  globalThis.EventSource = OriginalEventSource
  vi.unstubAllGlobals()
})

// P0-6: EventSource 现在在 ticket fetch 之后异步创建, 用 waitFor 等它出现。
async function getLastSource(): Promise<MockEventSource> {
  let instance: MockEventSource | undefined
  await waitFor(() => {
    instance = MockEventSource.lastInstance()
    expect(instance).toBeDefined()
  })
  return instance!
}

describe('useTaskSSE', () => {
  const defaultOptions: UseTaskSSEOptions = {
    taskID: 'test-deploy-001',
    enabled: true,
  }

  it('connects to SSE endpoint with correct URL', async () => {
    renderHook(() => useTaskSSE(defaultOptions))

    const instance = await getLastSource()
    expect(instance.url).toBe('/api/v1/tasks/stream/test-deploy-001')
  })

  it('does not connect when disabled', () => {
    renderHook(() => useTaskSSE({ ...defaultOptions, enabled: false }))

    expect(MockEventSource.instances.length).toBe(0)
  })

  it('does not connect when taskID is empty', () => {
    renderHook(() => useTaskSSE({ ...defaultOptions, taskID: '' }))

    expect(MockEventSource.instances.length).toBe(0)
  })

  it('calls onProgress when receiving progress event', async () => {
    const onProgress = vi.fn()
    renderHook(() => useTaskSSE({ ...defaultOptions, onProgress }))

    const instance = await getLastSource()

    const event: TaskEvent = {
      task_id: 'test-deploy-001',
      event_type: 'progress',
      progress: 50,
      stage: '安装二进制',
      log_line: '',
      status: 'running',
    }

    act(() => {
      instance!.receive(JSON.stringify(event))
    })

    expect(onProgress).toHaveBeenCalledTimes(1)
    expect(onProgress).toHaveBeenCalledWith(event)
  })

  it('unwraps backend SSE messages before dispatching events', async () => {
    const onProgress = vi.fn()
    renderHook(() => useTaskSSE({ ...defaultOptions, onProgress }))

    const instance = await getLastSource()

    const event: TaskEvent = {
      task_id: 'test-deploy-001',
      event_type: 'progress',
      progress: 60,
      stage: 'deploying',
      log_line: '',
      status: 'running',
    }

    act(() => {
      instance!.receive(JSON.stringify({ type: 'progress', data: event }))
    })

    expect(onProgress).toHaveBeenCalledTimes(1)
    expect(onProgress).toHaveBeenCalledWith(event)
  })

  it('calls onLog when receiving log event', async () => {
    const onLog = vi.fn()
    renderHook(() => useTaskSSE({ ...defaultOptions, onLog }))

    const instance = await getLastSource()

    const event: TaskEvent = {
      task_id: 'test-deploy-001',
      event_type: 'log',
      progress: 0,
      stage: '',
      log_line: '[10:30:15] 正在部署主节点 10.0.0.1:3306',
      status: '',
    }

    act(() => {
      instance!.receive(JSON.stringify(event))
    })

    expect(onLog).toHaveBeenCalledTimes(1)
    expect(onLog).toHaveBeenCalledWith(event)
  })

  it('calls onStep when receiving step event', async () => {
    const onStep = vi.fn()
    renderHook(() => useTaskSSE({ ...defaultOptions, onStep }))

    const instance = await getLastSource()

    const event: TaskEvent = {
      task_id: 'test-deploy-001',
      event_type: 'step',
      progress: 0,
      stage: '',
      log_line: '',
      status: '',
      metadata: {
        step_name: 'Deploy node 10.0.0.1',
        step_status: 'running',
        step_message: '正在执行部署...',
      },
    }

    act(() => {
      instance!.receive(JSON.stringify(event))
    })

    expect(onStep).toHaveBeenCalledTimes(1)
    expect(onStep).toHaveBeenCalledWith(event)
  })

  it('calls onStep multiple times for different steps', async () => {
    const onStep = vi.fn()
    renderHook(() => useTaskSSE({ ...defaultOptions, onStep }))

    const instance = await getLastSource()

    const step1: TaskEvent = {
      task_id: 'test-deploy-001',
      event_type: 'step',
      progress: 0,
      stage: '',
      log_line: '',
      status: '',
      metadata: { step_name: 'Step 1', step_status: 'running', step_message: '' },
    }

    const step2: TaskEvent = {
      task_id: 'test-deploy-001',
      event_type: 'step',
      progress: 0,
      stage: '',
      log_line: '',
      status: '',
      metadata: { step_name: 'Step 2', step_status: 'completed', step_message: 'Done' },
    }

    act(() => { instance.receive(JSON.stringify(step1)) })
    act(() => { instance.receive(JSON.stringify(step2)) })

    expect(onStep).toHaveBeenCalledTimes(2)
    expect(onStep).toHaveBeenNthCalledWith(1, step1)
    expect(onStep).toHaveBeenNthCalledWith(2, step2)
  })

  it('calls onStatus when receiving status event', async () => {
    const onStatus = vi.fn()
    renderHook(() => useTaskSSE({ ...defaultOptions, onStatus }))

    const instance = await getLastSource()

    const event: TaskEvent = {
      task_id: 'test-deploy-001',
      event_type: 'status',
      progress: 0,
      stage: '',
      log_line: '',
      status: 'completed',
    }

    act(() => {
      instance!.receive(JSON.stringify(event))
    })

    expect(onStatus).toHaveBeenCalledTimes(1)
    expect(onStatus).toHaveBeenCalledWith(event)
  })

  it('calls onComplete when receiving completed status', async () => {
    const onComplete = vi.fn()
    renderHook(() => useTaskSSE({ ...defaultOptions, onComplete }))

    const instance = await getLastSource()

    const event: TaskEvent = {
      task_id: 'test-deploy-001',
      event_type: 'status',
      progress: 0,
      stage: '',
      log_line: '',
      status: 'completed',
    }

    act(() => {
      instance!.receive(JSON.stringify(event))
    })

    expect(onComplete).toHaveBeenCalledTimes(1)
  })

  it('updates returned state values on progress events', async () => {
    const { result } = renderHook(() => useTaskSSE(defaultOptions))

    const instance = await getLastSource()

    expect(result.current.progress).toBe(0)
    expect(result.current.stage).toBe('')
    expect(result.current.status).toBe('pending')

    // Wait for connection
    await waitFor(() => {
      expect(result.current.connected).toBe(true)
    })

    const event: TaskEvent = {
      task_id: 'test-deploy-001',
      event_type: 'progress',
      progress: 75,
      stage: '启动节点',
      log_line: '',
      status: 'running',
    }

    act(() => { instance.receive(JSON.stringify(event)) })

    expect(result.current.progress).toBe(75)
    expect(result.current.stage).toBe('启动节点')
  })

  it('handles raw (non-JSON) messages as log lines', async () => {
    const { result } = renderHook(() => useTaskSSE(defaultOptions))

    const instance = await getLastSource()

    act(() => {
      instance!.receive('This is a raw log line')
    })

    expect(result.current.logs).toContain('This is a raw log line')
  })

  it('disconnects and cleans up EventSource on unmount', async () => {
    const { unmount } = renderHook(() => useTaskSSE(defaultOptions))

    const instance = await getLastSource()
    expect(instance.closed).toBe(false)

    unmount()

    expect(instance.closed).toBe(true)
  })

  it('disconnect function closes EventSource', async () => {
    const { result } = renderHook(() => useTaskSSE(defaultOptions))

    const instance = await getLastSource()
    expect(instance.closed).toBe(false)

    act(() => {
      result.current.disconnect()
    })

    expect(instance.closed).toBe(true)
  })

  it('calls onError when SSE connection fails', async () => {
    const onError = vi.fn()
    renderHook(() => useTaskSSE({ ...defaultOptions, onError }))

    const instance = await getLastSource()

    act(() => {
      instance!.fail()
    })

    expect(onError).toHaveBeenCalledTimes(1)
  })

  it('calls onStep even when metadata is empty', async () => {
    const onStep = vi.fn()
    renderHook(() => useTaskSSE({ ...defaultOptions, onStep }))

    const instance = await getLastSource()

    const event: TaskEvent = {
      task_id: 'test-deploy-001',
      event_type: 'step',
      progress: 0,
      stage: '',
      log_line: '',
      status: '',
    }

    act(() => {
      instance!.receive(JSON.stringify(event))
    })

    // onStep should still be called — the callback filters by metadata internally
    expect(onStep).toHaveBeenCalledTimes(1)
  })
})
