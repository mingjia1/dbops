import { useEffect, useRef, useState, useCallback } from 'react'

export interface TaskEvent {
  task_id: string
  event_type: 'progress' | 'log' | 'status'
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
  onComplete?: (event: TaskEvent) => void
  onError?: (error: Error) => void
}

export function useTaskSSE(options: UseTaskSSEOptions) {
  const {
    taskID,
    baseURL = '/api/v1',
    enabled = true,
    onProgress,
    onLog,
    onStatus,
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
  }, [onProgress, onLog, onStatus, onComplete])

  useEffect(() => {
    if (!enabled || !taskID) return

    const url = `${baseURL}/tasks/${taskID}/stream`
    const source = new EventSource(url)
    sourceRef.current = source

    source.onopen = () => setConnected(true)

    source.onmessage = (msg) => {
      try {
        const event = JSON.parse(msg.data) as TaskEvent
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

    return () => {
      source.close()
      sourceRef.current = null
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
