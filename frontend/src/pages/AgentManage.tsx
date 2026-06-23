import React, { useEffect, useState, useCallback } from 'react'
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

const isFailedStatus = (s?: string) => ['failed', 'error', 'timeout', 'cancelled', 'canceled'].includes((s || '').toLowerCase())
const isSuccessStatus = (s?: string) => {
  const n = (s || '').toLowerCase()
  return ['success', 'succeeded', 'completed', 'ok'].includes(n) || n.startsWith('agent healthy')
}

const statusColor = (s?: string) => {
  if (isSuccessStatus(s)) return 'success'
  if (isFailedStatus(s)) return 'error'
  if (['submitted', 'pending', 'running', 'installing'].includes((s || '').toLowerCase())) return 'processing'
  return 'default'
}

const summarize = (rows: any[]) => rows.map(r => `${r?.host_name || r?.address || '-'}: ${r?.message || r?.status || '-'}`).join('\n')

const AgentManage: React.FC = () => {
  const [hosts, setHosts] = useState<Host[]>([])
  const [loading, setLoading] = useState(false)
  const [running, setRunning] = useState(false)
  const [selectedRowKeys, setSelectedRowKeys] = useState<React.Key[]>([])
  const [resultRows, setResultRows] = useState<any[]>([])
  const [localStatus, setLocalStatus] = useState<Record<string, { status: string; message?: string }>>({})
  const [form] = Form.useForm()

  const fetchHosts = useCallback(async () => {
    setLoading(true)
    try {
      const res: any = await hostApi.list(1000, 0)
      setHosts(res?.data || [])
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => { fetchHosts() }, [fetchHosts])

  const runSync = async (hostIds: string[], action: string, agentPort?: number) => {
    const results: any[] = []
    setLocalStatus({})
    for (const hid of hostIds) {
      setLocalStatus(prev => ({ ...prev, [hid]: { status: 'running', message: `${action}中...` } }))
      try {
        const res: any = await hostApi.agentAction(hid, action, agentPort)
        const d = res?.data
        results.push(d)
        setLocalStatus(prev => ({ ...prev, [hid]: { status: d?.status || 'unknown', message: d?.message } }))
      } catch (err: any) {
        const msg = err?.response?.data?.message || err?.message || '请求失败'
        results.push({ host_id: hid, action, status: 'failed', message: msg })
        setLocalStatus(prev => ({ ...prev, [hid]: { status: 'failed', message: msg } }))
      }
    }
    setResultRows(results)
    return results
  }

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
      const results = await runSync(hostIds, action, values.agent_port)
      const failed = results.filter(r => !isSuccessStatus(r?.status))
      if (failed.length > 0) {
        message.error(`${failed.length} 台主机 ${action} 失败，详见执行结果`)
      } else {
        message.success(`${action} 成功：${results.length} 台`)
      }
      fetchHosts()
    } finally {
      setRunning(false)
    }
  }

  const getCellStatus = (host: Host) => {
    const local = localStatus[host.id]
    if (local) return local
    return { status: host.status, message: undefined }
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
        const cs = getCellStatus(r)
        const s = cs.status
        const label = cs.message && cs.message.length < 40 ? cs.message : (s || 'unknown')
        return <Tag color={statusColor(s)} title={cs.message}>{label}</Tag>
      },
    },
  ]

  const resultColumns: ColumnsType<any> = [
    { title: '主机', dataIndex: 'host_name', key: 'host_name' },
    { title: '地址', dataIndex: 'address', key: 'address' },
    { title: '动作', dataIndex: 'action', key: 'action' },
    { title: '结果', dataIndex: 'status', key: 'status', render: (v) => <Tag color={statusColor(v)}>{v || '-'}</Tag> },
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
