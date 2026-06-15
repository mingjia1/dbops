import React, { useEffect, useRef, useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { Button, Card, Dropdown, Empty, Form, Input, message, Modal, Popconfirm, Space, Table, Tag, Tooltip } from 'antd'
import { DatabaseOutlined, DesktopOutlined, DownOutlined, PlusOutlined, ReloadOutlined, RocketOutlined, ScanOutlined, ThunderboltOutlined } from '@ant-design/icons'
import type { ColumnsType } from 'antd/es/table'
import type { MenuProps } from 'antd'
import { hostApi, instanceApi, type Host, type HostScanResult } from '../services/api'

const longRunningAgentActions = new Set(['install', 'add', 'update', 'modify', 'restart'])
const isFailedAgentStatus = (status?: string) => {
  const normalized = (status || '').toLowerCase()
  return ['failed', 'error', 'timeout', 'cancelled', 'canceled'].includes(normalized)
}

const summarizeAgentRows = (rows: any[]) =>
  rows.map((row: any) => `${row?.host_name || row?.host_id || row?.address || '-'}: ${row?.message || row?.status || 'unknown'}`).join('\n')

const parseBatchHosts = (text: string) => {
  const trimmed = text.trim()
  if (!trimmed) return []
  if (trimmed.startsWith('[')) return JSON.parse(trimmed)
  return trimmed.split(/\r?\n/).map((line, index) => {
    const [name, address, sshPort, sshUser, credential, agentPort, tags] = line.split(',').map((v) => v?.trim())
    return {
      name: name || `host-${index + 1}`,
      address,
      ssh_port: sshPort ? Number(sshPort) : 22,
      ssh_user: sshUser || 'root',
      ssh_auth_method: 'password',
      ssh_credential: credential || '',
      agent_port: agentPort ? Number(agentPort) : 9090,
      os_type: 'linux',
      tags,
    }
  }).filter((item) => item.address)
}

const HostList: React.FC = () => {
  const navigate = useNavigate()
  const [hosts, setHosts] = useState<Host[]>([])
  const [loading, setLoading] = useState(false)
  const [instanceCount, setInstanceCount] = useState<Record<string, number>>({})
  const [scanningHosts, setScanningHosts] = useState<Record<string, boolean>>({})
  const [selectedRowKeys, setSelectedRowKeys] = useState<React.Key[]>([])
  const [batchOpen, setBatchOpen] = useState(false)
  const [batchSubmitting, setBatchSubmitting] = useState(false)
  const [batchForm] = Form.useForm()
  const pollRef = useRef<Record<string, number>>({})

  const fetchHosts = async () => {
    setLoading(true)
    try {
      const res: any = await hostApi.list(1000, 0)
      setHosts(res.data || [])
    } catch {
      setHosts([])
    } finally {
      setLoading(false)
    }
  }

  const fetchInstanceCount = async (hostId: string) => {
    try {
      const r: any = await instanceApi.listByHost(hostId, 1000, 0)
      setInstanceCount((p) => ({ ...p, [hostId]: Array.isArray(r?.data) ? r.data.length : 0 }))
    } catch {
      setInstanceCount((p) => ({ ...p, [hostId]: 0 }))
    }
  }

  useEffect(() => {
    fetchHosts()
  }, [])

  useEffect(() => {
    hosts.forEach((h) => fetchInstanceCount(h.id))
  }, [hosts.length])

  useEffect(() => () => {
    Object.values(pollRef.current).forEach((timer) => window.clearInterval(timer))
  }, [])

  const stopScanPolling = (hostId: string) => {
    const timer = pollRef.current[hostId]
    if (timer) {
      window.clearInterval(timer)
      delete pollRef.current[hostId]
    }
    setScanningHosts((p) => ({ ...p, [hostId]: false }))
  }

  const startScanPolling = (host: Host, taskId: string) => {
    pollRef.current[host.id] = window.setInterval(async () => {
      try {
        const r: any = await hostApi.getScanResult(host.id, taskId)
        const data: HostScanResult = r?.data
        if (!data) return
        if (data.status === 'success') {
          stopScanPolling(host.id)
          const pending = (data.instances || []).filter((i) => !i.already_managed)
          if (pending.length > 0) {
            message.success(`${host.name} 发现 ${pending.length} 个待纳管实例`)
            navigate(`/dashboard/hosts/${host.id}?tab=instances&scan_task=${taskId}`)
          } else {
            message.success(`${host.name} 扫描完成，无待纳管实例`)
            fetchInstanceCount(host.id)
          }
        }
        if (data.status === 'failed') {
          stopScanPolling(host.id)
          message.error(`${host.name} 扫描失败: ${data.error || data.message || '未知错误'}`)
        }
      } catch {
        // keep polling; transient API errors should not cancel a running task
      }
    }, 2000)
  }

  const handleScan = async (host: Host) => {
    try {
      setScanningHosts((p) => ({ ...p, [host.id]: true }))
      const r: any = await hostApi.scanInstances(host.id, { probe_mysql: true })
      const taskId = r?.data?.task_id
      if (!taskId) {
        setScanningHosts((p) => ({ ...p, [host.id]: false }))
        message.warning('扫描任务未返回 task_id')
        return
      }
      message.info(`${host.name} 扫描任务已提交`)
      startScanPolling(host, taskId)
    } catch {
      setScanningHosts((p) => ({ ...p, [host.id]: false }))
      message.error('扫描发起失败')
    }
  }

  const handleBatchTest = async () => {
    const selected = hosts.filter((h) => selectedRowKeys.includes(h.id))
    if (selected.length === 0) {
      message.warning('请先选择主机')
      return
    }
    let submitted = 0
    for (const host of selected) {
      try {
        await hostApi.testConnection(host.id)
        submitted += 1
      } catch {
        // interceptor shows error
      }
    }
    message.success(`已提交 ${submitted} 个主机连通性检测任务`)
  }

  const handleBatchAgent = async (action: string) => {
    const hostIds = selectedRowKeys.map(String)
    if (hostIds.length === 0) {
      message.warning('\u8bf7\u5148\u9009\u62e9\u4e3b\u673a')
      return
    }
    const asyncAction = longRunningAgentActions.has(action)
    const res: any = await hostApi.batchAgentAction(hostIds, action, asyncAction)
    const rows = res?.data?.rows || []
    const failedRows = rows.filter((row: any) => isFailedAgentStatus(row?.status))
    if (failedRows.length > 0 || (res?.data?.failed ?? 0) > 0) {
      Modal.error({
        title: `Agent ${action} \u64cd\u4f5c\u5931\u8d25`,
        content: <div style={{ maxHeight: 260, overflow: 'auto', whiteSpace: 'pre-wrap' }}>{summarizeAgentRows(failedRows.length > 0 ? failedRows : rows)}</div>,
      })
    } else if (asyncAction || res?.data?.async) {
      Modal.info({
        title: `Agent ${action} \u4efb\u52a1\u5df2\u63d0\u4ea4`,
        content: `\u5df2\u63d0\u4ea4 ${rows.length || hostIds.length} \u53f0\u4e3b\u673a\uff0c\u5e73\u53f0\u4f1a\u5728\u540e\u53f0\u6267\u884c\u3002\u8bf7\u7a0d\u540e\u5237\u65b0\u4e3b\u673a\u6216 Agent \u72b6\u6001\u67e5\u770b\u6700\u7ec8\u7ed3\u679c\u3002`,
      })
    } else {
      message.success(`Agent ${action} \u6210\u529f\uff1a${res?.data?.success ?? 0} \u4e2a`)
    }
    fetchHosts()
  }
  const submitBatchCreate = async () => {
    const values = await batchForm.validateFields()
    const parsed = parseBatchHosts(values.hosts)
    if (parsed.length === 0) {
      message.warning('没有可添加的主机')
      return
    }
    setBatchSubmitting(true)
    try {
      const res: any = await hostApi.batchCreate(parsed)
      const created = res?.data?.created ?? 0
      const failedRows = (res?.data?.rows || []).filter((row: any) => row.status === 'failed')
      if (failedRows.length > 0) {
        Modal.warning({
          title: `批量添加主机部分失败：成功 ${created} 个，失败 ${failedRows.length} 个`,
          content: (
            <div style={{ maxHeight: 260, overflow: 'auto', whiteSpace: 'pre-wrap' }}>
              {failedRows.map((row: any) => `${row.index || '-'} ${row.name || row.address || '-'}: ${row.message || 'failed'}`).join('\n')}
            </div>
          ),
        })
      } else {
        message.success(`批量添加主机完成：成功 ${created}/${parsed.length}`)
      }
      setBatchOpen(false)
      batchForm.resetFields()
      fetchHosts()
    } finally {
      setBatchSubmitting(false)
    }
  }

  const handleBatchDeployFromHosts = async () => {
    const selected = hosts.filter((h) => selectedRowKeys.includes(h.id))
    if (selected.length === 0) {
      message.warning('请先选择主机')
      return
    }
    let deployed = 0
    for (const host of selected) {
      try {
        const res: any = await instanceApi.listByHost(host.id, 1000, 0)
        const instances = res.data || []
        for (const inst of instances) {
          try {
            await instanceApi.deploy(inst.id)
            deployed += 1
          } catch { /* skip */ }
        }
      } catch { /* skip */ }
    }
    if (deployed > 0) message.success(`已提交 ${deployed} 个 MySQL 实例部署任务`)
    else message.warning('所选主机无实例可部署')
  }

  const agentMenuItems: MenuProps['items'] = [
    { key: 'install', label: '批量安装Agent' },
    { key: 'update', label: '批量更新Agent' },
    { key: 'stop', label: '批量停止Agent' },
    { key: 'start', label: '批量启动Agent' },
    { key: 'status', label: '批量检查状态' },
  ]

  const handleDelete = async (id: string) => {
    try {
      await hostApi.delete(id)
      message.success('主机删除成功')
      fetchHosts()
    } catch {
      // interceptor already showed error
    }
  }

  const columns: ColumnsType<Host> = [
    { title: '主机名称', key: 'name', render: (_, r) => <Link to={`/dashboard/hosts/${r.id}`}>{r.name}</Link> },
    { title: '地址', key: 'address', render: (_, r) => `${r.address}:${r.ssh_port}` },
    { title: 'SSH 用户', dataIndex: 'ssh_user', key: 'ssh_user' },
    { title: '操作系统', dataIndex: 'os_type', key: 'os_type', render: (os) => os?.toUpperCase() || '-' },
    {
      title: '实例数',
      key: 'instances',
      render: (_, r) => {
        const n = instanceCount[r.id]
        if (n === undefined) return <Tag>加载中</Tag>
        if (n === 0) {
          return (
            <Tooltip title="该主机暂无已纳管实例，可扫描后纳管">
              <Tag color="warning" icon={<DatabaseOutlined />}>0</Tag>
            </Tooltip>
          )
        }
        return <Tag color="processing" icon={<DatabaseOutlined />}>{n}</Tag>
      },
    },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      render: (status) => {
        const colorMap: Record<string, string> = { success: 'success', failed: 'error', unknown: 'default', pending: 'processing' }
        const textMap: Record<string, string> = { success: '可用', failed: '不可用', unknown: '未检测', pending: '检测中' }
        return <Tag color={colorMap[status] || 'default'}>{textMap[status] || status}</Tag>
      },
    },
    { title: '最后检测', dataIndex: 'last_check_at', key: 'last_check_at', render: (t) => (t ? new Date(t).toLocaleString() : '-') },
    {
      title: '操作',
      key: 'action',
      width: 300,
      render: (_, r) => (
        <Space>
          <Button type="link" size="small" icon={<ScanOutlined />} loading={!!scanningHosts[r.id]} onClick={() => handleScan(r)}>扫描实例</Button>
          <Button type="link" size="small" onClick={() => navigate(`/dashboard/instances?host_id=${r.id}`)}>管理实例</Button>
          <Button type="link" size="small" onClick={() => navigate(`/dashboard/hosts/${r.id}/edit`)}>编辑</Button>
          <Popconfirm title="确定删除该主机？" onConfirm={() => handleDelete(r.id)} okText="确定" cancelText="取消">
            <Button type="link" size="small" danger>删除</Button>
          </Popconfirm>
        </Space>
      ),
    },
  ]

  return (
    <div style={{ padding: 24 }}>
      <Card
        title={<Space><DesktopOutlined /><span>主机管理</span></Space>}
        extra={
          <Space>
            <Button icon={<ThunderboltOutlined />} disabled={selectedRowKeys.length === 0} onClick={handleBatchTest}>批量检测</Button>
            <Button icon={<RocketOutlined />} disabled={selectedRowKeys.length === 0} onClick={handleBatchDeployFromHosts}>部署MySQL</Button>
            <Dropdown menu={{ items: agentMenuItems, onClick: ({ key }) => handleBatchAgent(key) }} disabled={selectedRowKeys.length === 0}>
              <Button disabled={selectedRowKeys.length === 0}>Agent 操作 <DownOutlined /></Button>
            </Dropdown>
            <Button icon={<ReloadOutlined />} onClick={fetchHosts}>刷新</Button>
            <Button icon={<PlusOutlined />} onClick={() => setBatchOpen(true)}>批量添加</Button>
            <Button type="primary" icon={<PlusOutlined />} onClick={() => navigate('/dashboard/hosts/new')}>添加主机</Button>
          </Space>
        }
      >
        <Table
          rowSelection={{ selectedRowKeys, onChange: setSelectedRowKeys }}
          columns={columns}
          dataSource={hosts}
          rowKey="id"
          loading={loading}
          pagination={{ pageSize: 10 }}
          locale={{
            emptyText: (
              <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无主机">
                <Space>
                  <Button type="primary" icon={<PlusOutlined />} onClick={() => navigate('/dashboard/hosts/new')}>添加主机</Button>
                  <Button icon={<PlusOutlined />} onClick={() => setBatchOpen(true)}>批量添加</Button>
                </Space>
              </Empty>
            ),
          }}
        />
      </Card>

      <Modal
        title="批量添加主机"
        open={batchOpen}
        onCancel={() => setBatchOpen(false)}
        onOk={submitBatchCreate}
        confirmLoading={batchSubmitting}
        okText="批量添加"
        cancelText="取消"
        width={760}
      >
        <Form form={batchForm} layout="vertical">
          <Form.Item
            name="hosts"
            label="主机清单"
            extra="支持 CSV：name,address,ssh_port,ssh_user,ssh_password,agent_port,tags；也支持 JSON 数组。"
            rules={[{ required: true, message: '请输入主机清单' }]}
          >
            <Input.TextArea
              rows={10}
              placeholder={'db-host-01,10.1.81.41,22,root,ssh-password,9090,test\n备用格式也可粘贴 JSON 数组'}
            />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}

export default HostList
