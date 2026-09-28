import { useEffect, useRef, useState, useCallback } from 'react'

export interface TaskEvent {
  task_id: string
  event_type: 'progress' | 'log' | 'status' | 'step'
  progress: number
  stage: string
  log_line: string
  status: string
  metadata?: Record<string, string>
}

export interface UseTaskSSEOptions {
  taskID: string
  baseURL?: string
  enabled?: boolean
  onProgress?: (event: TaskEvent) => void
  onLog?: (event: TaskEvent) => void
  onStatus?: (event: TaskEvent) => void
  onStep?: (event: TaskEvent) => void
  onComplete?: (event: TaskEvent) => void
  onError?: (error: Error) => void
}

function normalizeTaskEvent(value: unknown): TaskEvent {
  if (!value || typeof value !== 'object') {
    throw new Error('invalid task event')
  }
  const maybeWrapped = value as { type?: string; data?: unknown }
  const payload = maybeWrapped.data && typeof maybeWrapped.data === 'object'
    ? maybeWrapped.data as Partial<TaskEvent>
    : value as Partial<TaskEvent>
  const eventType = payload.event_type || maybeWrapped.type
  if (!eventType) {
    throw new Error('invalid task event')
  }
  return {
    task_id: payload.task_id || '',
    event_type: eventType as TaskEvent['event_type'],
    progress: typeof payload.progress === 'number' ? payload.progress : 0,
    stage: payload.stage || '',
    log_line: payload.log_line || '',
    status: payload.status || '',
    metadata: payload.metadata,
  }
}

export function useTaskSSE(options: UseTaskSSEOptions) {
  const {
    taskID,
    baseURL = '/api/v1',
    enabled = true,
    onProgress,
    onLog,
    onStatus,
    onStep,
    onComplete,
    onError,
  } = options

  const [events, setEvents] = useState<TaskEvent[]>([])
  const [progress, setProgress] = useState(0)
  const [stage, setStage] = useState('')
  const [status, setStatus] = useState('pending')
  const [logs, setLogs] = useState<string[]>([])
  const [connected, setConnected] = useState(false)
  const sourceRef = useRef<EventSource | null>(null)

  const addEvent = useCallback((event: TaskEvent) => {
    setEvents(prev => [...prev, event])
    if (event.event_type === 'progress') {
      setProgress(event.progress)
      setStage(event.stage)
      if (onProgress) onProgress(event)
    }
    if (event.event_type === 'log') {
      setLogs(prev => [...prev, event.log_line])
      if (onLog) onLog(event)
    }
    if (event.event_type === 'status') {
      setStatus(event.status)
      if (onStatus) onStatus(event)
      if (event.status === 'completed' || event.status === 'failed') {
        if (onComplete) onComplete(event)
      }
    }
    if (event.event_type === 'step') {
      if (onStep) onStep(event)
    }
  }, [onProgress, onLog, onStatus, onStep, onComplete])

  useEffect(() => {
    if (!enabled || !taskID) return
    let cancelled = false

    // P0-6: 不再把长效 JWT 拼进 URL (?token= 会泄漏到历史/日志/Referer)。
    // 优先用 POST /auth/sse-ticket 换 60s 一次性 ticket；拿不到 (离线/测试环境) 则
    // 降级为裸 URL —— 后端同源场景下 HttpOnly cookie 会自动带上。
    const connect = async () => {
      let url = `${baseURL}/tasks/stream/${taskID}`
      try {
        const res = await fetch('/api/v1/auth/sse-ticket', {
          method: 'POST',
          credentials: 'include',
          headers: {
            ...(typeof localStorage !== 'undefined' && localStorage.getItem('token')
              ? { Authorization: `Bearer ${localStorage.getItem('token')}` }
              : {}),
          },
        })
        if (res.ok) {
          const data = await res.json()
          const ticket = data?.data?.ticket || data?.ticket
          if (ticket) url = `${url}?ticket=${encodeURIComponent(ticket)}`
        }
      } catch {
        // ticket 端点不可达 → 降级裸 URL (cookie 鉴权)
      }
      if (cancelled) return
      const source = new EventSource(url, { withCredentials: true })
      sourceRef.current = source

      source.onopen = () => setConnected(true)

      source.onmessage = (msg) => {
        try {
          const event = normalizeTaskEvent(JSON.parse(msg.data))
          addEvent(event)
        } catch {
          // raw log line
          setLogs(prev => [...prev, msg.data])
        }
      }

      source.onerror = (err) => {
        setConnected(false)
        if (onError) onError(new Error('SSE connection lost'))
        source.close()
      }
    }

    void connect()

    return () => {
      cancelled = true
      if (sourceRef.current) {
        sourceRef.current.close()
        sourceRef.current = null
      }
    }
  }, [taskID, baseURL, enabled, addEvent, onError])

  const disconnect = useCallback(() => {
    if (sourceRef.current) {
      sourceRef.current.close()
      sourceRef.current = null
      setConnected(false)
    }
  }, [])

  return {
    events,
    progress,
    stage,
    status,
    logs,
    connected,
    disconnect,
  }
}
