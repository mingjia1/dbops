import React, { useMemo } from 'react'
import { Progress, Steps, Card, Typography, Tag, Space } from 'antd'
import { useTaskSSE, type TaskEvent } from '../services/useTaskSSE'
import {
  CheckCircleOutlined,
  SyncOutlined,
  ClockCircleOutlined,
  CloseCircleOutlined,
} from '@ant-design/icons'

const { Text, Paragraph } = Typography

interface DeployProgressProps {
  taskID: string
  clusterName?: string
  baseURL?: string
  onComplete?: (event: TaskEvent) => void
}

const statusColor: Record<string, string> = {
  pending: 'default',
  running: 'processing',
  completed: 'success',
  failed: 'error',
  skipped: 'warning',
}

const statusIcon: Record<string, React.ReactNode> = {
  pending: <ClockCircleOutlined />,
  running: <SyncOutlined spin />,
  completed: <CheckCircleOutlined />,
  failed: <CloseCircleOutlined />,
}

export const DeployProgress: React.FC<DeployProgressProps> = ({
  taskID,
  clusterName,
  baseURL,
  onComplete,
}) => {
  const { progress, stage, status, logs, connected, events } = useTaskSSE({
    taskID,
    baseURL,
    enabled: !!taskID,
    onComplete,
  })

  const phaseEvents = useMemo(() => {
    return events.filter(e => e.event_type === 'progress' || e.event_type === 'status')
  }, [events])

  const currentStep = useMemo(() => {
    const phases = ['template_build', 'replicate', 'assemble']
    const idx = phases.indexOf(stage)
    return idx >= 0 ? idx : 0
  }, [stage])

  const stepItems = [
    {
      title: '模板构建',
      description: '部署首节点',
      status: getStepStatus(0, currentStep, status),
    },
    {
      title: '参数复刻',
      description: '并发部署副本',
      status: getStepStatus(1, currentStep, status),
    },
    {
      title: '集群装配',
      description: '建立复制关系',
      status: getStepStatus(2, currentStep, status),
    },
  ]

  return (
    <Card
      title={clusterName ? `部署 ${clusterName}` : '部署进度'}
      extra={
        <Space>
          <Tag color={connected ? 'green' : 'red'}>
            {connected ? '实时连接' : '断开'}
          </Tag>
          <Tag color={statusColor[status] || 'default'}>{status}</Tag>
        </Space>
      }
    >
      <Progress
        percent={progress}
        status={status === 'failed' ? 'exception' : status === 'completed' ? 'success' : 'active'}
        strokeColor={{ '0%': '#108ee9', '100%': '#87d068' }}
      />

      <Steps
        current={currentStep}
        items={stepItems}
        style={{ marginTop: 24, marginBottom: 24 }}
      />

      {stage && (
        <Text type="secondary" style={{ display: 'block', marginBottom: 8 }}>
          当前阶段: {stage}
        </Text>
      )}

      <Card
        type="inner"
        title="执行日志"
        bodyStyle={{ maxHeight: 300, overflow: 'auto', background: '#1e1e1e', padding: 12 }}
      >
        <pre style={{ color: '#d4d4d4', margin: 0, fontSize: 12, lineHeight: 1.5 }}>
          {logs.length === 0
            ? '<等待日志...>'
            : logs.map((line, i) => (
                <div key={i}>
                  <Text style={{ color: '#6A9955', fontFamily: 'monospace' }}>{line}</Text>
                </div>
              ))}
        </pre>
      </Card>

      {phaseEvents.length > 0 && (
        <Card type="inner" title="阶段详情" style={{ marginTop: 12 }}>
          {phaseEvents.map((event, i) => (
            <div key={i} style={{ marginBottom: 4 }}>
              <Space>
                {statusIcon[event.status] || <ClockCircleOutlined />}
                <Text strong>{event.stage || event.event_type}</Text>
                {event.progress > 0 && (
                  <Text type="secondary">{event.progress}%</Text>
                )}
                <Tag color={statusColor[event.status]}>{event.status}</Tag>
              </Space>
            </div>
          ))}
        </Card>
      )}
    </Card>
  )
}

function getStepStatus(stepIdx: number, currentStep: number, overallStatus: string): 'wait' | 'process' | 'finish' | 'error' {
  if (overallStatus === 'failed' && stepIdx === currentStep) return 'error'
  if (stepIdx < currentStep) return 'finish'
  if (stepIdx === currentStep) return 'process'
  return 'wait'
}

export default DeployProgress
