import React, { useEffect, useState } from 'react'
import { Alert, Button, Descriptions, Modal, Space, Statistic, Steps, Tag, Card } from 'antd'
import { CheckCircleOutlined, CloseCircleOutlined, ReloadOutlined } from '@ant-design/icons'
import { taskApi, isTaskTerminalStatus } from '../services/api'

interface TaskErrorModalProps {
  task: Record<string, any>
  open: boolean
  onClose: () => void
  onRetry?: () => void
}

const TaskErrorModal: React.FC<TaskErrorModalProps> = ({ task, open, onClose, onRetry }) => {
  const [logs, setLogs] = useState<string[]>([])
  const [loadingLogs, setLoadingLogs] = useState(false)

  useEffect(() => {
    if (!open || !task?.id) return
    let cancelled = false
    setLoadingLogs(true)
    taskApi.getLogs(task.id)
      .then((res: any) => {
        if (!cancelled) {
          const raw = res?.data || []
          const lines = Array.isArray(raw) ? raw.map((row: any) => row?.message || row?.log || String(row)).filter(Boolean) : []
          setLogs(lines.length > 0 ? lines : ['(no logs)'])
        }
      })
      .catch(() => {
        if (!cancelled) setLogs(['(failed to load logs)'])
      })
      .finally(() => {
        if (!cancelled) setLoadingLogs(false)
      })
    return () => {
      cancelled = true
    }
  }, [open, task?.id])

  const terminal = isTaskTerminalStatus(task?.status)
  const failed = ['failed', 'error', 'timeout', 'cancelled', 'canceled'].includes(String(task?.status || '').toLowerCase())

  return (
    <Modal
      title={`任务详情${task?.id ? ` #${task.id}` : ''}`}
      open={open}
      onCancel={onClose}
      width={720}
      footer={
        <Space>
          <Button onClick={onClose}>关闭</Button>
          {failed && onRetry && (
            <Button type="primary" icon={<ReloadOutlined />} onClick={onRetry}>
              重试
            </Button>
          )}
        </Space>
      }
    >
      <Steps
        size="small"
        current={terminal ? (failed ? 3 : 2) : 1}
        status={failed ? 'error' : terminal ? 'finish' : 'process'}
        style={{ marginBottom: 16 }}
        items={[
          { title: '提交', status: 'finish' as const },
          { title: '执行中', status: terminal ? 'finish' as const : 'process' as const },
          { title: '完成', status: failed ? 'error' : terminal ? 'finish' : 'wait' as const },
        ]}
      />
      <Descriptions bordered size="small" column={2}>
        <Descriptions.Item label="任务 ID">{task?.id || '-'}</Descriptions.Item>
        <Descriptions.Item label="状态">
          <Tag color={failed ? 'error' : terminal ? 'success' : 'processing'}>
            {task?.status || '-'}
          </Tag>
        </Descriptions.Item>
        <Descriptions.Item label="实例 ID">{task?.instance_id || '-'}</Descriptions.Item>
        <Descriptions.Item label="当前阶段">{task?.current_stage || task?.stage || '-'}</Descriptions.Item>
        <Descriptions.Item label="进度" span={2}>
          <Statistic value={typeof task?.progress === 'number' ? `${task.progress}%` : '-'} />
        </Descriptions.Item>
        <Descriptions.Item label="错误信息" span={2}>
          {task?.error_message || task?.error || task?.message || '-'}
        </Descriptions.Item>
      </Descriptions>
      <Card title="日志" size="small" style={{ marginTop: 12 }}>
        {loadingLogs ? (
          <div style={{ padding: 12 }}>加载中...</div>
        ) : (
          <pre style={{ maxHeight: 240, overflow: 'auto', whiteSpace: 'pre-wrap', fontSize: 12 }}>
            {logs.join('\n')}
          </pre>
        )}
      </Card>
    </Modal>
  )
}

export default TaskErrorModal
