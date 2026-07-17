import React, { useEffect, useRef, useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { Button, Card, Dropdown, Empty, Form, Input, InputNumber, message, Modal, Popconfirm, Space, Table, Tag, Tooltip } from 'antd'
import { DatabaseOutlined, DesktopOutlined, DownOutlined, PlusOutlined, ReloadOutlined, RocketOutlined, ScanOutlined, ThunderboltOutlined } from '@ant-design/icons'
import type { ColumnsType } from 'antd/es/table'
import type { MenuProps } from 'antd'
import { hostApi, instanceApi, type Host, type HostScanResult, type HostTestResult } from '../services/api'
import LiveDeployTracker from '../components/LiveDeployTracker'

const longRunningAgentActions = new Set(['install', 'add', 'update', 'modify', 'restart', 'delete', 'remove'])
const isFailedAgentStatus = (status?: string) => {
  const normalized = (status || '').toLowerCase()
  return ['failed', 'error', 'timeout', 'cancelled', 'canceled'].includes(normalized)
}
const isOkAgentStatus = (status?: string) => {
  const normalized = (status || '').toLowerCase()
  return ['success', 'succeeded', 'completed', 'ok'].includes(normalized)
}
const agentResultColor = (status?: string) => {
  const normalized = (status || '').toLowerCase()
  if (isOkAgentStatus(normalized)) return 'success'
  if (isFailedAgentStatus(normalized)) return 'error'
  if (['submitted', 'pending', 'running'].includes(normalized)) return 'processing'
  return 'default'
}

const summarizeAgentRows = (rows: any[]) =>
  rows.map((row: any) => `${row?.host_name || row?.host_id || row?.address || '-'}: ${row?.message || row?.status || 'unknown'}`).join('\n')

const getErrorMessage = (err: any) => err?.response?.data?.message || err?.message || '未知错误'

const HostList: React.FC = () => {
  const navigate = useNavigate()
  const [hosts, setHosts] = useState<Host[]>([])
  const [loading, setLoading] = useState(false)
  const [scanningHosts, setScanningHosts] = useState<Record<string, boolean>>({})
  const [selectedRowKeys, setSelectedRowKeys] = useState<React.Key[]>([])
  const [batchOpen, setBatchOpen] = useState(false)
  const [batchSubmitting, setBatchSubmitting] = useState(false)
  const [testRows, setTestRows] = useState<Record<string, HostTestResult & { host_name?: string; address?: string }>>({})
  const [batchForm] = Form.useForm()
  const [liveTaskIds, setLiveTaskIds] = useState<string[]>([])
  const pollRef = useRef<Record<string, number>>({})
  const testPollRef = useRef<Record<string, number>>({})

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

  useEffect(() => {
    fetchHosts()
  }, [])

  useEffect(() => () => {
    Object.values(pollRef.current).forEach((timer) => window.clearInterval(timer))
    Object.values(testPollRef.current).forEach((timer) => window.clearInterval(timer))
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
            fetchHosts()
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
    setTestRows((prev) => {
      const next = { ...prev }
      selected.forEach((host) => {
        next[host.id] = {
          task_id: '',
          host_id: host.id,
          host_name: host.name,
          address: host.address,
          status: 'pending',
          message: '检测任务提交中',
          latency_ms: 0,
          started_at: new Date().toISOString(),
          ended_at: '',
        }
      })
      return next
    })
    let submitted = 0
    for (const host of selected) {
      try {
        const res: any = await hostApi.testConnection(host.id)
        const initial: HostTestResult = res?.data
        if (initial) {
          setTestRows((prev) => ({
            ...prev,
            [host.id]: { ...initial, host_name: host.name, address: host.address },
          }))
          if (initial.task_id && initial.status === 'pending') pollHostTestResult(host, initial.task_id)
        }
        submitted += 1
      } catch (err: any) {
        setTestRows((prev) => ({
          ...prev,
          [host.id]: {
            task_id: '',
            host_id: host.id,
            host_name: host.name,
            address: host.address,
            status: 'failed',
            message: err?.response?.data?.message || err?.message || '检测提交失败',
            latency_ms: 0,
            started_at: new Date().toISOString(),
            ended_at: new Date().toISOString(),
          },
        }))
      }
    }
    if (submitted > 0) {
      message.success(`已提交 ${submitted} 个主机连通性检测任务`)
    } else {
      message.error('主机连通性检测提交失败')
    }
  }

  const pollHostTestResult = (host: Host, taskId: string) => {
    const existingTimer = testPollRef.current[host.id]
    if (existingTimer) window.clearInterval(existingTimer)
    testPollRef.current[host.id] = window.setInterval(async () => {
      try {
        const res: any = await hostApi.getTestResult(taskId)
        const row: HostTestResult = res?.data
        if (!row) return
        setTestRows((prev) => ({
          ...prev,
          [host.id]: { ...row, host_name: host.name, address: host.address },
        }))
        if (row.status === 'success' || row.status === 'failed') {
          window.clearInterval(testPollRef.current[host.id])
          delete testPollRef.current[host.id]
          fetchHosts()
        }
      } catch {
        // keep polling; the task may not be visible immediately
      }
    }, 1500)
  }

  const handleBatchAgent = async (action: string) => {
    const hostIds = selectedRowKeys.map(String)
    if (hostIds.length === 0) {
      message.warning('\u8bf7\u5148\u9009\u62e9\u4e3b\u673a')
      return
    }
    try {
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
    } catch (err: any) {
      Modal.error({
        title: `Agent ${action} \u64cd\u4f5c\u63d0\u4ea4\u5931\u8d25`,
        content: getErrorMessage(err),
      })
    } finally {
      fetchHosts()
    }
  }
  const submitBatchCreate = async () => {
    const values = await batchForm.validateFields()
    const hosts = (values.hosts || []).filter((h: any) => h.address && h.address.trim())
    if (hosts.length === 0) {
      message.warning('请至少填写一台主机的地址')
      return
    }
    const parsed = hosts.map((h: any) => ({
      name: h.name || h.address,
      address: h.address,
      ssh_port: h.ssh_port || 22,
      ssh_user: h.ssh_user || 'root',
      ssh_credential: h.ssh_password || '',
      agent_port: h.agent_port || 9090,
    }))
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
    } catch (err: any) {
      message.error('批量添加主机失败: ' + getErrorMessage(err))
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
    let totalInstances = 0
    const noInstanceHosts: string[] = []
    const occupiedPorts: string[] = []

    for (const host of selected) {
      try {
        const res: any = await instanceApi.listByHost(host.id, 1000, 0)
        const instances = res.data || []
        if (instances.length === 0) {
          noInstanceHosts.push(host.name || host.address)
        } else {
          totalInstances += instances.length
          for (const inst of instances) {
            const portInfo = inst.connection?.port || inst.port || 3306
            const version = inst.version?.version || inst.version?.full_version || '未知版本'
            occupiedPorts.push(`${host.name || host.address}:${portInfo} (${version})`)
            try {
              const dr: any = await instanceApi.deploy(inst.id)
              const tid = dr?.data?.task_id || dr?.task_id
              if (tid) setLiveTaskIds((prev) => [...prev, tid])
              deployed += 1
            } catch { /* skip */ }
          }
        }
      } catch { /* skip */ }
    }

    if (deployed > 0) {
      Modal.info({
        title: `已提交 ${deployed} 个实例部署任务（下方可看实时进度）`,
        width: 640,
        content: (
          <div>
            <p>部署中的实例：</p>
            <ul style={{ maxHeight: 120, overflow: 'auto', paddingLeft: 20 }}>
              {occupiedPorts.map((p, i) => <li key={i}>{p}</li>)}
            </ul>
            <p style={{ marginTop: 8 }}>请关闭后查看列表下方进度卡片；任务在后台继续执行。</p>
          </div>
        ),
      })
    } else if (noInstanceHosts.length === selected.length) {
      Modal.confirm({
        title: '所选主机暂无已注册实例',
        content: (
          <div>
            <p>以下主机暂无实例记录，需要先创建实例才能部署 MySQL：</p>
            <ul style={{ paddingLeft: 20 }}>
              {noInstanceHosts.map((n, i) => <li key={i}>{n}</li>)}
            </ul>
            <p>是否跳转到实例管理页面创建实例？</p>
          </div>
        ),
        okText: '去创建实例',
        cancelText: '取消',
        onOk: () => navigate('/dashboard/instances'),
      })
    } else {
      message.info(`${noInstanceHosts.length} 台主机暂无实例，已跳过`)
    }
  }

  const agentMenuItems: MenuProps['items'] = [
    { key: 'install', label: '批量安装Agent' },
    { key: 'update', label: '批量更新Agent' },
    { key: 'stop', label: '批量停止Agent' },
    { key: 'start', label: '批量启动Agent' },
    { key: 'status', label: '批量检查状态' },
    { key: 'delete', label: '批量删除Agent' },
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
        const n = r.instance_count
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
        const colorMap: Record<string, string> = { success: 'success', active: 'success', failed: 'error', unhealthy: 'error', unknown: 'default', pending: 'processing' }
        const textMap: Record<string, string> = { success: '可用', active: '可用', failed: '不可用', unhealthy: '不可用', unknown: '未检测', pending: '检测中' }
        return <Tag color={colorMap[status] || 'default'}>{textMap[status] || '未知'}</Tag>
      },
    },
    {
      title: 'Agent 版本',
      dataIndex: 'agent_version',
      key: 'agent_version',
      render: (v) => v || '-',
    },
    {
      title: 'Agent 状态',
      dataIndex: 'agent_status',
      key: 'agent_status',
      render: (s) => {
        if (!s) return <Tag>未安装</Tag>
        const colorMap: Record<string, string> = { running: 'success', stopped: 'error', removed: 'default', unknown: 'default' }
        const textMap: Record<string, string> = { running: '运行中', stopped: '已停止', removed: '已移除', unknown: '未知' }
        return <Tag color={colorMap[s] || 'default'}>{textMap[s] || s}</Tag>
      },
    },
    {
      title: '最近 Agent 操作',
      key: 'agent_last_action',
      width: 220,
      render: (_, r) => {
        if (!r.agent_last_action && !r.agent_last_result) return '-'
        const time = r.agent_last_at ? new Date(r.agent_last_at).toLocaleString() : '-'
        return (
          <Tooltip title={`${r.agent_last_message || '-'}\n${time}`}>
            <Space size={4} direction="vertical">
              <span>{r.agent_last_action || '-'}</span>
              <Tag color={agentResultColor(r.agent_last_result)}>{r.agent_last_result || 'unknown'}</Tag>
            </Space>
          </Tooltip>
        )
      },
    },
    { title: '最后检测', dataIndex: 'last_check_at', key: 'last_check_at', render: (t) => (t ? new Date(t).toLocaleString() : '-') },
    {
      title: '操作',
      key: 'action',
      width: 300,
      render: (_, r) => (
        <Space>
          <Button type="link" size="small" onClick={() => navigate(`/dashboard/instances?host_id=${r.id}`)}>管理实例</Button>
          <Button type="link" size="small" onClick={() => navigate(`/dashboard/hosts/${r.id}/edit`)}>编辑</Button>
          <Popconfirm title="确定删除该主机？" onConfirm={() => handleDelete(r.id)} okText="确定" cancelText="取消">
            <Button type="link" size="small" danger>删除</Button>
          </Popconfirm>
        </Space>
      ),
    },
  ]

  const testResultColumns: ColumnsType<HostTestResult & { host_name?: string; address?: string }> = [
    { title: '主机', key: 'host', render: (_, r) => `${r.host_name || r.host_id} (${r.address || '-'})` },
    { title: '检测结果', dataIndex: 'status', key: 'status', render: (v) => <Tag color={v === 'success' ? 'success' : v === 'failed' ? 'error' : 'processing'}>{v || 'pending'}</Tag> },
    { title: '说明', dataIndex: 'message', key: 'message', render: (v) => v || '-' },
    { title: '延迟', dataIndex: 'latency_ms', key: 'latency_ms', render: (v) => (v ? `${v} ms` : '-') },
  ]

  return (
    <div style={{ padding: 24 }}>
      <Card
        title={<Space><DesktopOutlined /><span>主机管理</span></Space>}
        extra={
          <Space>
            <Button icon={<ThunderboltOutlined />} disabled={selectedRowKeys.length === 0} onClick={handleBatchTest}>批量检测</Button>
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

      {Object.keys(testRows).length > 0 && (
        <Card title="批量检测结果" style={{ marginTop: 16 }} size="small">
          <Table
            size="small"
            columns={testResultColumns}
            dataSource={Object.values(testRows).filter((row) => selectedRowKeys.length === 0 || selectedRowKeys.includes(row.host_id))}
            rowKey={(row) => row.host_id || row.task_id}
            pagination={false}
          />
        </Card>
      )}

      <Modal
        title="批量添加主机"
        open={batchOpen}
        onCancel={() => setBatchOpen(false)}
        onOk={submitBatchCreate}
        confirmLoading={batchSubmitting}
        okText="批量添加"
        cancelText="取消"
        width={900}
      >
        <Form form={batchForm} layout="vertical">
          <Form.List name="hosts" initialValue={[{ name: '', address: '', ssh_port: 22, ssh_user: 'root', ssh_password: '', agent_port: 9090 }]}>
            {(fields, { add, remove }) => (
              <>
                <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr 60px 60px 1fr 60px 40px', gap: 8, marginBottom: 4, fontSize: 12, fontWeight: 600, color: '#666' }}>
                  <span>主机名</span><span>地址</span><span>端口</span><span>用户</span><span>密码</span><span>Agent</span><span></span>
                </div>
                {fields.map(({ key, name }) => (
                  <div key={key} style={{ display: 'grid', gridTemplateColumns: '1fr 1fr 60px 60px 1fr 60px 40px', gap: 8, marginBottom: 4, alignItems: 'center' }}>
                    <Form.Item name={[name, 'name']} style={{ margin: 0 }}><Input size="small" placeholder="db-host-01" /></Form.Item>
                    <Form.Item name={[name, 'address']} style={{ margin: 0 }} rules={[{ required: true }]}><Input size="small" placeholder="192.0.2.41" /></Form.Item>
                    <Form.Item name={[name, 'ssh_port']} style={{ margin: 0 }} initialValue={22}><InputNumber size="small" min={1} max={65535} style={{ width: '100%' }} /></Form.Item>
                    <Form.Item name={[name, 'ssh_user']} style={{ margin: 0 }} initialValue="root"><Input size="small" /></Form.Item>
                    <Form.Item name={[name, 'ssh_password']} style={{ margin: 0 }}><Input.Password size="small" placeholder="SSH密码" autoComplete="new-password" /></Form.Item>
                    <Form.Item name={[name, 'agent_port']} style={{ margin: 0 }} initialValue={9090}><InputNumber size="small" min={1} max={65535} style={{ width: '100%' }} /></Form.Item>
                    <Button size="small" danger type="text" disabled={fields.length <= 1} onClick={() => remove(name)} style={{ padding: 0, fontSize: 16 }}>×</Button>
                  </div>
                ))}
                <Button size="small" type="dashed" onClick={() => add({ name: '', address: '', ssh_port: 22, ssh_user: 'root', ssh_password: '', agent_port: 9090 })} style={{ marginTop: 4 }}>+ 添加一行</Button>
              </>
            )}
          </Form.List>
        </Form>
      </Modal>

      {liveTaskIds.length > 0 && (
        <Card title="实时安装进度" style={{ marginTop: 16 }}>
          <Space direction="vertical" style={{ width: '100%' }} size={12}>
            {liveTaskIds.map((tid) => (
              <LiveDeployTracker key={tid} taskId={tid} title={`任务 ${tid}`} />
            ))}
            <Button size="small" onClick={() => setLiveTaskIds([])}>清空进度卡片</Button>
          </Space>
        </Card>
      )}
    </div>
  )
}

export default HostList
