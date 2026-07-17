import React, { useEffect, useRef, useState } from 'react'
import { Alert, Card, Progress, Space, Timeline, Typography } from 'antd'
import { LoadingOutlined } from '@ant-design/icons'
import { clusterDeployApi, isTaskTerminalStatus, pollTaskUntilDone } from '../services/api'
import {
  isCompletedDeployStatus,
  isFailedDeployStatus,
  isTerminalDeployStatus,
  normalizeDeployment,
} from '../services/deployHelpers'

const { Text } = Typography

type LogItem = { time: string; text: string; kind?: 'info' | 'error' | 'ok' }

function nowTime() {
  return new Date().toLocaleTimeString()
}

export interface LiveDeployTrackerProps {
  /** 单机任务 ID */
  taskId?: string | null
  /** 集群部署单 ID */
  deploymentId?: string | null
  title?: string
  onTerminal?: (ok: boolean, message?: string) => void
}

/**
 * 安装进度：任务与集群均走轮询（单一通道，终态只触发一次）。
 */
const LiveDeployTracker: React.FC<LiveDeployTrackerProps> = ({
  taskId,
  deploymentId,
  title = '安装进度',
  onTerminal,
}) => {
  const [progress, setProgress] = useState(0)
  const [stage, setStage] = useState('')
  const [statusMsg, setStatusMsg] = useState('等待…')
  const [logs, setLogs] = useState<LogItem[]>([])
  const [done, setDone] = useState(false)
  const [failed, setFailed] = useState(false)
  const terminalRef = useRef(false)

  const pushLog = (text: string, kind: LogItem['kind'] = 'info') => {
    if (!text) return
    setLogs((prev) => {
      const last = prev[prev.length - 1]
      if (last && last.text === text) return prev
      return [...prev, { time: nowTime(), text, kind }].slice(-100)
    })
  }

  const markTerminal = (ok: boolean, message?: string) => {
    if (terminalRef.current) return
    terminalRef.current = true
    setDone(true)
    setFailed(!ok)
    if (message) setStatusMsg(message)
    onTerminal?.(ok, message)
  }

  // 单机任务：只轮询
  useEffect(() => {
    if (!taskId || deploymentId) return
    let cancelled = false
    ;(async () => {
      const finalTask = await pollTaskUntilDone(taskId, (t) => {
        if (cancelled) return
        if (typeof t.progress === 'number') setProgress(t.progress)
        if (t.stage) setStage(t.stage)
        const msg = t.message || t.stage
        if (msg) {
          setStatusMsg(msg)
          pushLog(t.stage && t.message ? `${t.stage}: ${t.message}` : msg)
        }
      })
      if (cancelled) return
      const st = String(finalTask?.status || '').toLowerCase()
      const msg = finalTask?.message || finalTask?.error_message || finalTask?.status
      if (['completed', 'success'].includes(st)) {
        setProgress(100)
        markTerminal(true, msg || '完成')
      } else if (isTaskTerminalStatus(st)) {
        markTerminal(false, msg || '失败')
      }
    })()
    return () => { cancelled = true }
  }, [taskId, deploymentId])

  // 集群部署：只轮询
  useEffect(() => {
    if (!deploymentId) return
    let cancelled = false
    ;(async () => {
      for (let i = 0; i < 180 && !cancelled; i++) {
        try {
          const res: any = await clusterDeployApi.getStatus(deploymentId)
          const dep = normalizeDeployment(res?.data || {})
          if (typeof dep.progress === 'number') setProgress(dep.progress)
          const raw: any = res?.data || {}
          const msg = dep.message || dep.status || '部署中…'
          setStatusMsg(msg)
          setStage(raw.current_stage || raw.stage || msg)
          if (i === 0 || i % 2 === 0) pushLog(msg)
          const steps = raw.steps || raw.plan_steps || []
          if (Array.isArray(steps) && steps.length) {
            const running = steps.find((s: any) =>
              ['running', 'process', 'in_progress'].includes(String(s.status || '').toLowerCase()),
            )
            if (running) {
              const line = `${running.name || running.id || '步骤'}: ${running.message || running.status || ''}`
              setStage(running.name || running.id || '')
              pushLog(line)
            }
          }
          if (isTerminalDeployStatus(dep.status)) {
            if (isCompletedDeployStatus(dep.status)) {
              setProgress(100)
              markTerminal(true, msg)
            } else if (isFailedDeployStatus(dep.status)) {
              markTerminal(false, msg)
            } else {
              markTerminal(false, msg)
            }
            break
          }
        } catch {
          // keep polling
        }
        await new Promise((r) => setTimeout(r, 2000))
      }
    })()
    return () => { cancelled = true }
  }, [deploymentId])

  if (!taskId && !deploymentId) return null

  const progressStatus = failed ? 'exception' : done ? 'success' : 'active'

  return (
    <Card title={title} size="small">
      <Space direction="vertical" style={{ width: '100%' }} size={12}>
        <div>
          <Text type="secondary">当前阶段：</Text>
          <Text strong>{stage || statusMsg}</Text>
          {!done && <LoadingOutlined style={{ marginLeft: 8 }} />}
        </div>
        <Progress percent={progress} status={progressStatus as any} />
        <Text type="secondary">{statusMsg}</Text>
        {taskId && <Text type="secondary">任务 ID：{taskId}</Text>}
        {deploymentId && <Text type="secondary">部署单：{deploymentId}</Text>}
        {failed && <Alert type="error" showIcon message={statusMsg || '部署失败'} />}
        {done && !failed && <Alert type="success" showIcon message="部署完成" />}
        <div style={{ maxHeight: 280, overflow: 'auto' }}>
          <Timeline
            items={logs.map((l) => ({
              color: l.kind === 'error' ? 'red' : l.kind === 'ok' ? 'green' : 'blue',
              children: <Text><Text type="secondary">[{l.time}] </Text>{l.text}</Text>,
            }))}
          />
        </div>
      </Space>
    </Card>
  )
}

export default LiveDeployTracker
