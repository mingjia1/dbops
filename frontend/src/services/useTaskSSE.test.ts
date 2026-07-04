import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest'
import { renderHook, act } from '@testing-library/react'
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
})

afterEach(() => {
  globalThis.EventSource = OriginalEventSource
})

describe('useTaskSSE', () => {
  const defaultOptions: UseTaskSSEOptions = {
    taskID: 'test-deploy-001',
    enabled: true,
  }

  it('connects to SSE endpoint with correct URL', () => {
    renderHook(() => useTaskSSE(defaultOptions))

    const instance = MockEventSource.lastInstance()
    expect(instance).toBeDefined()
    expect(instance!.url).toBe('/api/v1/tasks/stream/test-deploy-001')
  })

  it('does not connect when disabled', () => {
    renderHook(() => useTaskSSE({ ...defaultOptions, enabled: false }))

    expect(MockEventSource.instances.length).toBe(0)
  })

  it('does not connect when taskID is empty', () => {
    renderHook(() => useTaskSSE({ ...defaultOptions, taskID: '' }))

    expect(MockEventSource.instances.length).toBe(0)
  })

  it('calls onProgress when receiving progress event', () => {
    const onProgress = vi.fn()
    renderHook(() => useTaskSSE({ ...defaultOptions, onProgress }))

    const instance = MockEventSource.lastInstance()
    expect(instance).toBeDefined()

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

  it('calls onLog when receiving log event', () => {
    const onLog = vi.fn()
    renderHook(() => useTaskSSE({ ...defaultOptions, onLog }))

    const instance = MockEventSource.lastInstance()

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

  it('calls onStep when receiving step event', () => {
    const onStep = vi.fn()
    renderHook(() => useTaskSSE({ ...defaultOptions, onStep }))

    const instance = MockEventSource.lastInstance()

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

  it('calls onStep multiple times for different steps', () => {
    const onStep = vi.fn()
    renderHook(() => useTaskSSE({ ...defaultOptions, onStep }))

    const instance = MockEventSource.lastInstance()!

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

  it('calls onStatus when receiving status event', () => {
    const onStatus = vi.fn()
    renderHook(() => useTaskSSE({ ...defaultOptions, onStatus }))

    const instance = MockEventSource.lastInstance()

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

  it('calls onComplete when receiving completed status', () => {
    const onComplete = vi.fn()
    renderHook(() => useTaskSSE({ ...defaultOptions, onComplete }))

    const instance = MockEventSource.lastInstance()

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

  it('updates returned state values on progress events', () => {
    const { result } = renderHook(() => useTaskSSE(defaultOptions))

    const instance = MockEventSource.lastInstance()!

    expect(result.current.progress).toBe(0)
    expect(result.current.stage).toBe('')
    expect(result.current.status).toBe('pending')
    expect(result.current.connected).toBe(false)

    // Wait for connection
    return vi.waitFor(() => {
      expect(result.current.connected).toBe(true)
    }).then(() => {
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
  })

  it('handles raw (non-JSON) messages as log lines', () => {
    const { result } = renderHook(() => useTaskSSE(defaultOptions))

    const instance = MockEventSource.lastInstance()!

    act(() => {
      instance!.receive('This is a raw log line')
    })

    expect(result.current.logs).toContain('This is a raw log line')
  })

  it('disconnects and cleans up EventSource on unmount', () => {
    const { unmount } = renderHook(() => useTaskSSE(defaultOptions))

    const instance = MockEventSource.lastInstance()!
    expect(instance.closed).toBe(false)

    unmount()

    expect(instance.closed).toBe(true)
  })

  it('disconnect function closes EventSource', () => {
    const { result } = renderHook(() => useTaskSSE(defaultOptions))

    const instance = MockEventSource.lastInstance()!
    expect(instance.closed).toBe(false)

    act(() => {
      result.current.disconnect()
    })

    expect(instance.closed).toBe(true)
  })

  it('calls onError when SSE connection fails', () => {
    const onError = vi.fn()
    renderHook(() => useTaskSSE({ ...defaultOptions, onError }))

    const instance = MockEventSource.lastInstance()!

    act(() => {
      instance!.fail()
    })

    expect(onError).toHaveBeenCalledTimes(1)
  })

  it('calls onStep even when metadata is empty', () => {
    const onStep = vi.fn()
    renderHook(() => useTaskSSE({ ...defaultOptions, onStep }))

    const instance = MockEventSource.lastInstance()!

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
