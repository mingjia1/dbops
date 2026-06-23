import { useEffect, useRef, useState, useCallback } from 'react'

export interface TaskEvent {
  task_id: string
  event_type: 'progress' | 'log' | 'status' | 'topology_change'
  progress: number
  stage: string
  log_line: string
  status: string
  metadata?: Record<string, string>
}

export interface UseTaskWebSocketOptions {
  taskID: string
  wsURL?: string
  enabled?: boolean
  onProgress?: (event: TaskEvent) => void
  onLog?: (event: TaskEvent) => void
  onStatus?: (event: TaskEvent) => void
  onComplete?: (event: TaskEvent) => void
  onError?: (error: Error) => void
}

export function useTaskWebSocket(options: UseTaskWebSocketOptions) {
  const {
    taskID,
    wsURL = `ws://${window.location.host}/ws`,
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
  const wsRef = useRef<WebSocket | null>(null)
  const reconnectRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  const addEvent = useCallback((event: TaskEvent) => {
    setEvents(prev => [...prev, event])
    if (event.event_type === 'progress') {
      setProgress(event.progress)
      setStage(event.stage)
      onProgress?.(event)
    }
    if (event.event_type === 'log') {
      setLogs(prev => [...prev, event.log_line])
      onLog?.(event)
    }
    if (event.event_type === 'status') {
      setStatus(event.status)
      onStatus?.(event)
      if (event.status === 'completed' || event.status === 'failed') {
        onComplete?.(event)
      }
    }
  }, [onProgress, onLog, onStatus, onComplete])

  const connect = useCallback(() => {
    if (!enabled || !taskID) return

    const url = `${wsURL}/tasks/${taskID}/stream`
    const ws = new WebSocket(url)
    wsRef.current = ws

    ws.onopen = () => {
      setConnected(true)
    }

    ws.onmessage = (msg) => {
      try {
        const event = JSON.parse(msg.data) as TaskEvent
        addEvent(event)
      } catch {
        setLogs(prev => [...prev, msg.data])
      }
    }

    ws.onerror = () => {
      setConnected(false)
      onError?.(new Error('WebSocket connection error'))
    }

    ws.onclose = () => {
      setConnected(false)
      if (enabled && taskID) {
        reconnectRef.current = setTimeout(connect, 3000)
      }
    }
  }, [taskID, wsURL, enabled, addEvent, onError])

  useEffect(() => {
    connect()
    return () => {
      if (reconnectRef.current) clearTimeout(reconnectRef.current)
      wsRef.current?.close()
    }
  }, [connect])

  const disconnect = useCallback(() => {
    if (reconnectRef.current) clearTimeout(reconnectRef.current)
    wsRef.current?.close()
    wsRef.current = null
    setConnected(false)
  }, [])

  const sendMessage = useCallback((data: Record<string, unknown>) => {
    if (wsRef.current?.readyState === WebSocket.OPEN) {
      wsRef.current.send(JSON.stringify(data))
    }
  }, [])

  return {
    events, progress, stage, status, logs,
    connected, disconnect, sendMessage,
  }
}
