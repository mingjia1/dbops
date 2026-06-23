import React, { useEffect, useState, useRef, useCallback } from 'react'
import { Button, Card, Form, InputNumber, message, Modal, Select, Space, Table, Tag } from 'antd'
import { CloudServerOutlined, ReloadOutlined, ThunderboltOutlined } from '@ant-design/icons'
import type { ColumnsType } from 'antd/es/table'
import { hostApi, type Host } from '../services/api'

const actionOptions = [
  { value: 'install', label: '安装 Agent' },
  { value: 'start', label: '启动 Agent' },
  { value: 'stop', label: '停止 Agent' },
  { value: 'restart', label: '重启 Agent' },
  { value: 'status', label: '检查状态' },
  { value: 'delete', label: '删除 Agent' },
]

const longRunningAgentActions = new Set(['install', 'add', 'update', 'modify', 'restart'])

const isFailedAgentStatus = (status?: string) => {
  const s = (status || '').toLowerCase()
  return ['failed', 'error', 'timeout', 'cancelled', 'canceled'].includes(s)
}

const isSuccessfulAgentStatus = (status?: string) => {
  const s = (status || '').toLowerCase()
  return ['success', 'succeeded', 'completed', 'ok', 'agent healthy'].includes(s) || s.startsWith('agent healthy')
}

const agentStatusColor = (status?: string) => {
  const s = (status || '').toLowerCase()
  if (isSuccessfulAgentStatus(s)) return 'success'
  if (isFailedAgentStatus(s)) return 'error'
  if (['submitted', 'pending', 'running', 'installing'].includes(s)) return 'processing'
  return 'default'
}

const hostStatusFromAgent = (actionResult?: any): string => {
  if (!actionResult) return ''
  if (isSuccessfulAgentStatus(actionResult.status)) return 'active'
  if (isFailedAgentStatus(actionResult.status)) return 'failed'
  return actionResult.status || ''
}

const summarizeRows = (rows: any[]) =>
  rows.map((r: any) => `${r?.host_name || r?.address || '-'}: ${r?.message || r?.status || 'unknown'}`).join('\n')

const AgentManage: React.FC = () => {
  const [hosts, setHosts] = useState<Host[]>([])
  const [loading, setLoading] = useState(false)
  const [running, setRunning] = useState(false)
  const [selectedRowKeys, setSelectedRowKeys] = useState<React.Key[]>([])
  const [resultRows, setResultRows] = useState<any[]>([])
  const [actionStatus, setActionStatus] = useState<Record<string, string>>({})
  const [form] = Form.useForm()
  const pollRef = useRef<number | null>(null)

  const fetchHosts = useCallback(async () => {
    setLoading(true)
    try {
      const res: any = await hostApi.list(1000, 0)
      setHosts(res?.data || [])
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    fetchHosts()
    return () => { if (pollRef.current) clearTimeout(pollRef.current) }
  }, [fetchHosts])

  const execute = async () => {
    const values = await form.validateFields()
    const hostIds = selectedRowKeys.map(String)
    if (hostIds.length === 0) {
      message.warning('请先选择主机')
      return
    }

    setRunning(true)
    setResultRows([])

    try {
      const action = values.action as string
      const isAsync = longRunningAgentActions.has(action)

      if (isAsync) {
        const res: any = await hostApi.batchAgentAction(hostIds, action, true, values.agent_port)
        const rows = res?.data?.rows || []
        setResultRows(rows)

        setActionStatus(prev => {
          const next = { ...prev }
          for (const r of rows) {
            if (r.host_id) next[r.host_id] = 'installing'
          }
          return next
        })

        message.info(`已提交 ${rows.length || hostIds.length} 台主机的 ${action} 任务，正在后台执行...`)

        const pollStatus = async (attempt: number) => {
          if (attempt > 6) return
          await new Promise(resolve => setTimeout(resolve, 5000))
          await fetchHosts()

          const checkResults: any[] = []
          for (const hostId of hostIds) {
            try {
              const r: any = await hostApi.agentAction(hostId, 'status')
              checkResults.push({ host_id: hostId, ...r?.data })
            } catch { /* ignore */ }
          }

          setResultRows(prev => {
            const updated = [...prev]
            for (const cr of checkResults) {
              const idx = updated.findIndex(r => r.host_id === cr.host_id)
              if (idx >= 0) {
                updated[idx] = { ...updated[idx], ...cr }
              } else {
                updated.push(cr)
              }
            }
            return updated
          })

          setActionStatus(prev => {
            const next = { ...prev }
            for (const cr of checkResults) {
              if (cr.host_id && isSuccessfulAgentStatus(cr.status)) {
                next[cr.host_id] = 'success'
              } else if (cr.host_id && isFailedAgentStatus(cr.status)) {
                next[cr.host_id] = 'failed'
              }
            }
            return next
          })

          const allDone = checkResults.every(r => isSuccessfulAgentStatus(r.status))
          const allFailed = checkResults.every(r => isFailedAgentStatus(r.status))
          if (!allDone && !allFailed && attempt < 6) {
            pollStatus(attempt + 1)
          } else {
            await fetchHosts()
          }
        }

        pollStatus(1)
      } else {
        const resultRowsList: any[] = []
        for (const hostId of hostIds) {
          try {
            const res: any = await hostApi.agentAction(hostId, action, values.agent_port)
            resultRowsList.push(res?.data)
          } catch (err: any) {
            resultRowsList.push({
              host_id: hostId,
              action,
              status: 'failed',
              message: err?.message || 'request failed',
            })
          }
        }

        setResultRows(resultRowsList)

        const failed = resultRowsList.filter(r => !isSuccessfulAgentStatus(r?.status))
        if (failed.length > 0) {
          Modal.error({
            title: `Agent ${action} 操作失败`,
            content: <div style={{ maxHeight: 260, overflow: 'auto', whiteSpace: 'pre-wrap' }}>{summarizeRows(failed)}</div>,
          })
        } else {
          message.success(`Agent ${action} 完成：${resultRowsList.length} 台`)
        }

        await fetchHosts()
      }
    } finally {
      setRunning(false)
    }
  }

  const getDisplayStatus = (host: Host): string => {
    const override = actionStatus[host.id]
    if (override === 'installing') return 'installing'
    if (override === 'success') return 'active'
    if (override === 'failed') return 'failed'
    return host.status
  }

  const getDisplayStatusText = (host: Host): string => {
    const override = actionStatus[host.id]
    if (override === 'installing') return '安装中...'
    if (override === 'success') return '可用'
    if (override === 'failed') return '不可用'
    return host.status || 'unknown'
  }

  const columns: ColumnsType<Host> = [
    { title: '主机名称', dataIndex: 'name', key: 'name' },
    { title: '地址', key: 'address', render: (_, r) => `${r.address}:${r.ssh_port}` },
    { title: 'SSH 用户', dataIndex: 'ssh_user', key: 'ssh_user' },
    { title: 'Agent 端口', dataIndex: 'agent_port', key: 'agent_port', render: (v) => v || 9090 },
    {
      title: '主机状态',
      key: 'status',
      render: (_, r) => {
        const s = getDisplayStatus(r)
        const text = getDisplayStatusText(r)
        return <Tag color={agentStatusColor(s)}>{text}</Tag>
      },
    },
  ]

  const resultColumns: ColumnsType<any> = [
    { title: '主机', dataIndex: 'host_name', key: 'host_name' },
    { title: '地址', dataIndex: 'address', key: 'address' },
    { title: '动作', dataIndex: 'action', key: 'action' },
    { title: '结果', dataIndex: 'status', key: 'status', render: (v) => <Tag color={agentStatusColor(v)}>{v || 'unknown'}</Tag> },
    { title: '信息', dataIndex: 'message', key: 'message', ellipsis: true },
  ]

  return (
    <div style={{ padding: 24 }}>
      <Card
        title={<Space><CloudServerOutlined /><span>Agent 管理</span></Space>}
        extra={<Button icon={<ReloadOutlined />} onClick={fetchHosts} loading={loading}>刷新</Button>}
      >
        <Form form={form} layout="inline" initialValues={{ action: 'status' }} style={{ marginBottom: 16 }}>
          <Form.Item name="action" rules={[{ required: true }]}>
            <Select style={{ width: 180 }} options={actionOptions} />
          </Form.Item>
          <Form.Item name="agent_port">
            <InputNumber min={1} max={65535} placeholder="Agent 端口(可选)" style={{ width: 150 }} />
          </Form.Item>
          <Button
            type="primary"
            icon={<ThunderboltOutlined />}
            onClick={execute}
            loading={running}
            disabled={selectedRowKeys.length === 0}
          >
            执行选中主机
          </Button>
        </Form>
        <Table
          rowSelection={{ selectedRowKeys, onChange: setSelectedRowKeys }}
          columns={columns}
          dataSource={hosts}
          rowKey="id"
          loading={loading}
          pagination={{ pageSize: 10 }}
        />
      </Card>

      {resultRows.length > 0 && (
        <Card title="执行结果" style={{ marginTop: 16 }}>
          <Table
            columns={resultColumns}
            dataSource={resultRows.map((row, i) => ({ ...row, key: `${row?.host_id || i}-${i}` }))}
            pagination={false}
            size="small"
          />
        </Card>
      )}
    </div>
  )
}

export default AgentManage
