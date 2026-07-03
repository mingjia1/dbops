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

const isFailed = (s?: string) => ['failed', 'error', 'timeout', 'cancelled', 'canceled', 'inactive'].includes((s || '').toLowerCase())
const isOk = (s?: string) => {
  const n = (s || '').toLowerCase()
  return ['success', 'succeeded', 'completed', 'ok', 'active'].includes(n) || n.startsWith('agent healthy')
}

const tagColor = (s?: string) => {
  if (isOk(s)) return 'success'
  if (isFailed(s)) return 'error'
  if (['submitted', 'pending', 'running', 'installing'].includes((s || '').toLowerCase())) return 'processing'
  return 'default'
}

const summarize = (rows: any[]) =>
  rows.map(r => `${r?.host_name || r?.address || '-'}: ${r?.message || r?.status || '-'}`).join('\n')

interface LiveStatus { status: string; message?: string; action?: string }

const AgentManage: React.FC = () => {
  const [hosts, setHosts] = useState<Host[]>([])
  const [loading, setLoading] = useState(false)
  const [running, setRunning] = useState(false)
  const [selectedRowKeys, setSelectedRowKeys] = useState<React.Key[]>([])
  const [resultRows, setResultRows] = useState<any[]>([])
  const [liveStatus, setLiveStatus] = useState<Record<string, LiveStatus>>({})
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

  const clearLive = (hostId: string) => {
    setLiveStatus(prev => {
      const next = { ...prev }
      delete next[hostId]
      return next
    })
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
    setLiveStatus({})

    try {
      const action = values.action as string
      const results: any[] = []

      for (const hid of hostIds) {
        setLiveStatus(prev => ({ ...prev, [hid]: { status: 'running', message: `${action}中...`, action } }))

        try {
          const res: any = await hostApi.agentAction(hid, action, values.agent_port)
          const d = res?.data
          results.push(d)
          setLiveStatus(prev => ({ ...prev, [hid]: { status: d?.status || 'unknown', message: d?.message, action } }))
        } catch (err: any) {
          const msg = err?.response?.data?.message || err?.message || '请求失败'
          results.push({ host_id: hid, action, status: 'failed', message: msg })
          setLiveStatus(prev => ({ ...prev, [hid]: { status: 'failed', message: msg, action } }))
        }
      }

      setResultRows(results)

      const failed = results.filter(r => !isOk(r?.status))
      if (failed.length > 0) {
        message.error(`${failed.length} 台主机 ${action} 失败，详见执行结果`)
      } else {
        message.success(`${action} 成功：${results.length} 台`)
      }

      await fetchHosts()
    } finally {
      setRunning(false)
    }
  }

  const getStatusDisplay = (host: Host) => {
    const live = liveStatus[host.id]
    if (live) {
      const s = live.status
      let label = s || 'unknown'
      if (s === 'running') label = live.message || '执行中...'
      else if (isOk(s)) label = '可用'
      else if (isFailed(s)) label = '不可用'
      return { status: s, label, tip: live.message }
    }
    const dbStatus = host.status
    const tip = host.agent_last_message || undefined
    if (dbStatus === 'active') return { status: 'success', label: '可用', tip }
    if (dbStatus === 'failed' || dbStatus === 'inactive') return { status: 'failed', label: '不可用', tip }
    return { status: dbStatus, label: dbStatus || 'unknown', tip }
  }

  const columns: ColumnsType<Host> = [
    { title: '主机名称', dataIndex: 'name', key: 'name' },
    { title: '地址', key: 'address', render: (_, r) => `${r.address}:${r.ssh_port}` },
    { title: 'SSH 用户', dataIndex: 'ssh_user', key: 'ssh_user' },
    { title: 'Agent 端口', dataIndex: 'agent_port', key: 'agent_port', render: (v) => v || 9090 },
    { title: 'Agent 版本', dataIndex: 'agent_version', key: 'agent_version', render: (v) => v || '-' },
    {
      title: '最近操作',
      key: 'agent_last_action',
      render: (_, r) => {
        if (!r.agent_last_action && !r.agent_last_result) return '-'
        const time = r.agent_last_at ? new Date(r.agent_last_at).toLocaleString() : '-'
        return (
          <Space size={4} direction="vertical">
            <span>{r.agent_last_action || '-'}</span>
            <Tag color={tagColor(r.agent_last_result)} title={`${r.agent_last_message || '-'}\n${time}`}>
              {r.agent_last_result || 'unknown'}
            </Tag>
          </Space>
        )
      },
    },
    {
      title: '主机状态',
      key: 'status',
      render: (_, r) => {
        const d = getStatusDisplay(r)
        return <Tag color={tagColor(d.status)} title={d.tip}>{d.label}</Tag>
      },
    },
  ]

  const resultColumns: ColumnsType<any> = [
    { title: '主机', dataIndex: 'host_name', key: 'host_name' },
    { title: '地址', dataIndex: 'address', key: 'address' },
    { title: '动作', dataIndex: 'action', key: 'action' },
    { title: '结果', dataIndex: 'status', key: 'status', render: (v) => <Tag color={tagColor(v)}>{v || '-'}</Tag> },
    { title: '信息', dataIndex: 'message', key: 'message', ellipsis: true },
  ]

  return (
    <div style={{ padding: 24 }}>
      <Card
        title={<Space><CloudServerOutlined /><span>Agent 管理</span></Space>}
        extra={<Button icon={<ReloadOutlined />} onClick={() => { setLiveStatus({}); fetchHosts() }} loading={loading}>刷新</Button>}
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
