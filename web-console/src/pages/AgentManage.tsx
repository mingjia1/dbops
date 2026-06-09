import React, { useEffect, useState } from 'react'
import { Button, Card, Form, InputNumber, message, Modal, Select, Space, Table, Tag } from 'antd'
import { CloudServerOutlined, ReloadOutlined, ThunderboltOutlined } from '@ant-design/icons'
import type { ColumnsType } from 'antd/es/table'
import { hostApi, type Host } from '../services/api'

const actionOptions = [
  { value: 'install', label: '添加/安装 Agent' },
  { value: 'modify', label: '修改配置并重启' },
  { value: 'update', label: '更新 Agent' },
  { value: 'stop', label: '停止 Agent' },
  { value: 'start', label: '启动 Agent' },
  { value: 'delete', label: '删除 Agent' },
  { value: 'status', label: '检查状态' },
]

const longRunningAgentActions = new Set(['install', 'add', 'update', 'modify', 'restart'])
const isFailedAgentStatus = (status?: string) => {
  const normalized = (status || '').toLowerCase()
  return ['failed', 'error', 'timeout', 'cancelled', 'canceled'].includes(normalized)
}

const isSuccessfulAgentStatus = (status?: string) => {
  const normalized = (status || '').toLowerCase()
  return ['success', 'succeeded', 'completed', 'ok'].includes(normalized)
}

const summarizeAgentRows = (rows: any[]) =>
  rows.map((row: any) => `${row?.host_name || row?.host_id || row?.address || '-'}: ${row?.message || row?.status || 'unknown'}`).join('\n')

const AgentManage: React.FC = () => {
  const [hosts, setHosts] = useState<Host[]>([])
  const [loading, setLoading] = useState(false)
  const [running, setRunning] = useState(false)
  const [selectedRowKeys, setSelectedRowKeys] = useState<React.Key[]>([])
  const [rows, setRows] = useState<any[]>([])
  const [form] = Form.useForm()

  const fetchHosts = async () => {
    setLoading(true)
    try {
      const res: any = await hostApi.list(1000, 0)
      setHosts(res?.data || [])
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    fetchHosts()
  }, [])

  const execute = async () => {
    const values = await form.validateFields()
    const hostIds = selectedRowKeys.map(String)
    if (hostIds.length === 0) {
      message.warning('\u8bf7\u5148\u9009\u62e9\u4e3b\u673a')
      return
    }
    setRunning(true)
    try {
      const asyncAction = longRunningAgentActions.has(values.action)
      if (values.agent_port && !asyncAction) {
        const resultRows = []
        for (const hostId of hostIds) {
          const res: any = await hostApi.agentAction(hostId, values.action, values.agent_port)
          resultRows.push(res?.data)
        }
        setRows(resultRows)
        const failedRows = resultRows.filter((row: any) => !isSuccessfulAgentStatus(row?.status))
        if (failedRows.length > 0) {
          Modal.error({
            title: `Agent ${values.action} \u64cd\u4f5c\u5931\u8d25`,
            content: <div style={{ maxHeight: 260, overflow: 'auto', whiteSpace: 'pre-wrap' }}>{summarizeAgentRows(failedRows)}</div>,
          })
        } else {
          message.success(`Agent ${values.action} \u64cd\u4f5c\u5b8c\u6210`)
        }
      } else {
        const res: any = await hostApi.batchAgentAction(hostIds, values.action, asyncAction, values.agent_port)
        const resultRows = res?.data?.rows || []
        setRows(resultRows)
        const failedRows = resultRows.filter((row: any) => isFailedAgentStatus(row?.status))
        if (failedRows.length > 0 || (res?.data?.failed ?? 0) > 0) {
          Modal.error({
            title: `Agent ${values.action} \u64cd\u4f5c\u5931\u8d25`,
            content: <div style={{ maxHeight: 260, overflow: 'auto', whiteSpace: 'pre-wrap' }}>{summarizeAgentRows(failedRows.length > 0 ? failedRows : resultRows)}</div>,
          })
        } else if (asyncAction || res?.data?.async) {
          Modal.info({
            title: `Agent ${values.action} \u4efb\u52a1\u5df2\u63d0\u4ea4`,
            content: `\u5df2\u63d0\u4ea4 ${resultRows.length || hostIds.length} \u53f0\u4e3b\u673a\uff0c\u5e73\u53f0\u4f1a\u5728\u540e\u53f0\u6267\u884c\u3002\u8bf7\u7a0d\u540e\u5237\u65b0\u4e3b\u673a\u6216 Agent \u72b6\u6001\u67e5\u770b\u6700\u7ec8\u7ed3\u679c\u3002`,
          })
        } else {
          message.success(`Agent ${values.action} \u6210\u529f\uff1a${res?.data?.success ?? 0} \u4e2a`)
        }
      }
      fetchHosts()
    } finally {
      setRunning(false)
    }
  }
  const columns: ColumnsType<Host> = [
    { title: '主机名称', dataIndex: 'name', key: 'name' },
    { title: '地址', key: 'address', render: (_, r) => `${r.address}:${r.ssh_port}` },
    { title: 'SSH 用户', dataIndex: 'ssh_user', key: 'ssh_user' },
    { title: 'Agent 端口', dataIndex: 'agent_port', key: 'agent_port', render: (v) => v || 9090 },
    { title: '主机状态', dataIndex: 'status', key: 'status', render: (v) => <Tag color={v === 'success' ? 'success' : v === 'failed' ? 'error' : 'default'}>{v || 'unknown'}</Tag> },
  ]

  const resultColumns: ColumnsType<any> = [
    { title: '主机', dataIndex: 'host_name', key: 'host_name' },
    { title: '地址', dataIndex: 'address', key: 'address' },
    { title: '动作', dataIndex: 'action', key: 'action' },
    { title: '结果', dataIndex: 'status', key: 'status', render: (v) => <Tag color={v === 'success' ? 'success' : 'error'}>{v}</Tag> },
    { title: '信息', dataIndex: 'message', key: 'message' },
  ]

  return (
    <div style={{ padding: 24 }}>
      <Card
        title={<Space><CloudServerOutlined /><span>Agent 管理</span></Space>}
        extra={<Button icon={<ReloadOutlined />} onClick={fetchHosts}>刷新</Button>}
      >
        <Form form={form} layout="inline" initialValues={{ action: 'install' }} style={{ marginBottom: 16 }}>
          <Form.Item name="action" rules={[{ required: true }]}>
            <Select style={{ width: 180 }} options={actionOptions} />
          </Form.Item>
          <Form.Item name="agent_port">
            <InputNumber min={1} max={65535} placeholder="Agent 端口(可选)" style={{ width: 150 }} />
          </Form.Item>
          <Button type="primary" icon={<ThunderboltOutlined />} onClick={execute} loading={running} disabled={selectedRowKeys.length === 0}>
            执行选中主机
          </Button>
        </Form>
        <Table
          rowSelection={{ selectedRowKeys, onChange: setSelectedRowKeys }}
          columns={columns}
          dataSource={hosts}
          rowKey="id"
          loading={loading}
          pagination={{ pageSize: 20 }}
        />
      </Card>

      {rows.length > 0 && (
        <Card title="执行结果" style={{ marginTop: 16 }}>
          <Table columns={resultColumns} dataSource={rows.map((row, index) => ({ ...row, key: `${row?.host_id || index}-${index}` }))} pagination={false} />
        </Card>
      )}
    </div>
  )
}

export default AgentManage
