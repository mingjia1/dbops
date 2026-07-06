import React, { useEffect, useState, useCallback } from 'react'
import { Button, Card, Form, InputNumber, message, Modal, Select, Space, Table, Tag } from 'antd'
import { CloudServerOutlined, ReloadOutlined, ThunderboltOutlined } from '@ant-design/icons'
import type { ColumnsType } from 'antd/es/table'
import { hostApi, type Host } from '../services/api'

const actionOptions = [
  { value: 'install', label: '安装 Agent' },
  { value: 'update', label: '更新 Agent' },
  { value: 'start', label: '启动 Agent' },
  { value: 'stop', label: '停止 Agent' },
  { value: 'restart', label: '重启 Agent' },
  { value: 'status', label: '检查状态' },
  { value: 'delete', label: '删除 Agent' },
]

const asyncActions = ['install', 'add', 'update', 'modify', 'restart', 'delete', 'remove']
const pendingResults = ['submitted', 'pending', 'running', 'installing']
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

const getErrorMessage = (err: any) => err?.response?.data?.message || err?.message || '未知错误'

interface LiveStatus { status: string; message?: string; action?: string; agent_version?: string }

const sleep = (ms: number) => new Promise(resolve => window.setTimeout(resolve, ms))
const isPendingResult = (status?: string) => pendingResults.includes((status || '').toLowerCase())
const isAsyncAction = (action: string) => asyncActions.includes((action || '').toLowerCase())
const formatTime = (value?: string | null) => value ? new Date(value).toLocaleString() : '-'

const AgentManage: React.FC = () => {
  const [hosts, setHosts] = useState<Host[]>([])
  const [loading, setLoading] = useState(false)
  const [running, setRunning] = useState(false)
  const [selectedRowKeys, setSelectedRowKeys] = useState<React.Key[]>([])
  const [resultRows, setResultRows] = useState<any[]>([])
  const [liveStatus, setLiveStatus] = useState<Record<string, LiveStatus>>({})
  const [form] = Form.useForm()

  const fetchHosts = useCallback(async (notify = true, clearOnError = true, showLoading = true) => {
    if (showLoading) setLoading(true)
    try {
      const res: any = await hostApi.list(1000, 0)
      const rows = res?.data || []
      setHosts(rows)
      return rows as Host[]
    } catch (err: any) {
      if (clearOnError) setHosts([])
      if (notify) message.error('加载主机列表失败: ' + getErrorMessage(err))
      return [] as Host[]
    } finally {
      if (showLoading) setLoading(false)
    }
  }, [])

  useEffect(() => { fetchHosts() }, [fetchHosts])

  useEffect(() => {
    const timer = window.setInterval(() => {
      if (!running) void fetchHosts(false, false, false)
    }, 5000)
    return () => window.clearInterval(timer)
  }, [fetchHosts, running])

  const clearLive = (hostId: string) => {
    setLiveStatus(prev => {
      const next = { ...prev }
      delete next[hostId]
      return next
    })
  }

  const buildHostResult = (host: Host, action: string) => ({
    host_id: host.id,
    host_name: host.name,
    address: host.address,
    agent_port: host.agent_port || 9090,
    action,
    status: host.agent_last_action === action ? host.agent_last_result || host.status : host.status,
    message: host.agent_last_action === action ? host.agent_last_message : '',
    agent_version: host.agent_version || '',
  })

  const pollAsyncResults = async (hostIds: string[], pendingHostIds: string[], action: string, initialRows: any[]) => {
    const resultByHost = new Map(initialRows.map(row => [row.host_id, row]))
    const pending = new Set(pendingHostIds)

    for (let attempt = 0; attempt < 90 && pending.size > 0; attempt += 1) {
      await sleep(2000)
      const latestHosts = await fetchHosts(false, false, false)

      latestHosts
        .filter(host => pending.has(host.id) && host.agent_last_action === action)
        .forEach(host => {
          const row = buildHostResult(host, action)
          resultByHost.set(host.id, row)
          setLiveStatus(prev => ({
            ...prev,
            [host.id]: { status: row.status || 'unknown', message: row.message, action, agent_version: row.agent_version },
          }))
          if (!isPendingResult(row.status)) {
            pending.delete(host.id)
          }
        })

      setResultRows(hostIds.map(id => resultByHost.get(id) || { host_id: id, action, status: 'pending', message: '等待执行结果...' }))
    }

    if (pending.size > 0) {
      pending.forEach(id => {
        const row = resultByHost.get(id) || { host_id: id, action }
        resultByHost.set(id, { ...row, status: 'timeout', message: '等待 agent 操作结果超时，请稍后刷新查看最新状态' })
        setLiveStatus(prev => ({
          ...prev,
          [id]: { status: 'timeout', message: '等待 agent 操作结果超时，请稍后刷新查看最新状态', action },
        }))
      })
    }

    const finalRows = hostIds.map(id => resultByHost.get(id) || { host_id: id, action, status: 'unknown' })
    setResultRows(finalRows)
    return finalRows
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
      const hostByID = new Map(hosts.map(host => [host.id, host]))
      setLiveStatus(prev => {
        const next = { ...prev }
        hostIds.forEach(hid => {
          next[hid] = { status: 'running', message: `${action}中...`, action }
        })
        return next
      })

      const res: any = await hostApi.batchAgentAction(hostIds, action, isAsyncAction(action), values.agent_port)
      const payload = res?.data || {}
      const rows: any[] = Array.isArray(payload.rows) ? payload.rows : []
      const rowsByHost = new Map<string, any>(rows.map((row: any) => [String(row?.host_id), row]))
      const results: any[] = hostIds.map((hid) => {
        const host = hostByID.get(hid)
        return rowsByHost.get(String(hid)) || {
          host_id: hid,
          host_name: host?.name,
          address: host?.address,
          agent_port: host?.agent_port || values.agent_port || 9090,
          action,
          status: 'failed',
          message: 'batch response missing host result',
        }
      })
      const pendingIds = results
        .filter(row => isAsyncAction(action) && isPendingResult(row?.status))
        .map(row => String(row.host_id))

      setLiveStatus(prev => {
        const next = { ...prev }
        results.forEach(row => {
          next[row.host_id] = {
            status: row?.status || 'unknown',
            message: row?.message,
            action,
            agent_version: row?.agent_version,
          }
        })
        return next
      })

      setResultRows(results)
      const finalResults = pendingIds.length > 0 ? await pollAsyncResults(hostIds, pendingIds, action, results) : results

      const failed = finalResults.filter(r => !isOk(r?.status))
      if (failed.length > 0) {
        message.error(`${failed.length} 台主机 ${action} 失败，详见执行结果`)
      } else {
        message.success(`${action} 成功：${finalResults.length} 台`)
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
    const dbStatus = host.agent_status || host.status
    const tip = [
      host.agent_last_message || undefined,
      host.agent_last_heartbeat ? `最后心跳: ${formatTime(host.agent_last_heartbeat)}` : undefined,
    ].filter(Boolean).join('\n') || undefined
    if (['running', 'active', 'success'].includes(dbStatus)) return { status: 'success', label: '可用', tip }
    if (['failed', 'inactive', 'stopped', 'removed'].includes(dbStatus)) return { status: 'failed', label: '不可用', tip }
    return { status: dbStatus, label: dbStatus || 'unknown', tip }
  }

  const columns: ColumnsType<Host> = [
    { title: '主机名称', dataIndex: 'name', key: 'name' },
    { title: '地址', key: 'address', render: (_, r) => `${r.address}:${r.ssh_port}` },
    { title: 'SSH 用户', dataIndex: 'ssh_user', key: 'ssh_user' },
    { title: 'Agent 端口', dataIndex: 'agent_port', key: 'agent_port', render: (v) => v || 9090 },
    { title: 'Agent 版本', dataIndex: 'agent_version', key: 'agent_version', render: (v, r) => liveStatus[r.id]?.agent_version || v || '-' },
    { title: '最后心跳', dataIndex: 'agent_last_heartbeat', key: 'agent_last_heartbeat', render: (v) => formatTime(v) },
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
      title: 'Agent 状态',
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
    { title: 'Agent 版本', dataIndex: 'agent_version', key: 'agent_version', render: (v) => v || '-' },
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
