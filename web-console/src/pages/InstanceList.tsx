import React, { useEffect, useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { Alert, Button, Card, Divider, Empty, Form, Input, InputNumber, message, Modal, Popconfirm, Select, Space, Table, Tag } from 'antd'
import { CheckCircleOutlined, DatabaseOutlined, PlusOutlined, ReloadOutlined, RocketOutlined, ScanOutlined } from '@ant-design/icons'
import type { ColumnsType } from 'antd/es/table'
import { extractTaskPayload, hostApi, instanceApi, versionApi, type Host, type Instance, type VersionEntry } from '../services/api'

const parseBatchInstances = (text: string) => {
  const trimmed = text.trim()
  if (!trimmed) return []
  if (trimmed.startsWith('[')) return JSON.parse(trimmed)
  return trimmed.split(/\r?\n/).map((line, index) => {
    const [name, host, port, username, password, hostId, clusterId] = line.split(',').map((v) => v?.trim())
    return {
      name: name || `mysql-${index + 1}`,
      host,
      port: port ? Number(port) : 3306,
      username: username || 'root',
      password: password || '',
      host_id: hostId || undefined,
      cluster_id: clusterId || undefined,
    }
  }).filter((item) => item.host && item.port)
}

const isFailedTaskStatus = (status?: string) => {
  const normalized = (status || '').toLowerCase()
  return ['failed', 'error', 'unhealthy', 'timeout', 'cancelled', 'canceled'].includes(normalized)
}

const isSuccessfulTaskStatus = (status?: string) => {
  const normalized = (status || '').toLowerCase()
  return ['success', 'succeeded', 'healthy', 'ok', 'completed'].includes(normalized)
}

const isHealthCheckSuccess = (task: any) => {
  if (!task || isFailedTaskStatus(task.status)) return false
  return isSuccessfulTaskStatus(task.status)
}

const getHealthCheckTask = (res: any) => extractTaskPayload(res)

const getHealthCheckErrorTask = (err: any) => extractTaskPayload(err?.response?.data || err?.data || null)

const formatHealthCheckFailure = (instanceName: string, task: any, fallback: string) => {
  const parts = [
    `${instanceName}: ${task?.message || fallback}`,
    task?.status ? `status=${task.status}` : '',
    task?.task_id ? `task_id=${task.task_id}` : '',
  ].filter(Boolean)
  return parts.join(' | ')
}

const formatHealthCheckSummary = (ok: number, failed: number) =>
  `一键检测失败：成功 ${ok} 个，失败 ${failed} 个。Agent 端口拒绝连接、Agent 返回 failed/error/timeout、或后端 code 非 200 都按失败处理。`

const InstanceList: React.FC = () => {
  const [searchParams] = useSearchParams()
  const navigate = useNavigate()
  const presetHost = searchParams.get('preset_host') || searchParams.get('host_id') || undefined

  const [instances, setInstances] = useState<Instance[]>([])
  const [hosts, setHosts] = useState<Host[]>([])
  const [versions, setVersions] = useState<VersionEntry[]>([])
  const [loading, setLoading] = useState(false)
  const [hostFilter, setHostFilter] = useState<string | undefined>(presetHost)
  const [modalOpen, setModalOpen] = useState(false)
  const [batchOpen, setBatchOpen] = useState(false)
  const [submitting, setSubmitting] = useState(false)
  const [batchSubmitting, setBatchSubmitting] = useState(false)
  const [selectedRowKeys, setSelectedRowKeys] = useState<React.Key[]>([])
  const [form] = Form.useForm()
  const [batchForm] = Form.useForm()

  const fetchInstances = async () => {
    setLoading(true)
    try {
      const filterId = hostFilter || presetHost || undefined
      const res: any = filterId ? await instanceApi.listByHost(filterId, 1000, 0) : await instanceApi.list(1000, 0)
      setInstances(res.data || [])
    } catch {
      setInstances([])
    } finally {
      setLoading(false)
    }
  }

  const fetchHosts = async () => {
    try {
      const res: any = await hostApi.list(1000, 0)
      setHosts(res.data || [])
    } catch {
      setHosts([])
    }
  }

  useEffect(() => {
    fetchInstances()
    fetchHosts()
    versionApi.list().then((res: any) => setVersions(res?.data || [])).catch(() => setVersions([]))
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  useEffect(() => {
    fetchInstances()
    if (presetHost) form.setFieldsValue({ host_id: presetHost })
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [hostFilter, presetHost])

  const hostNameById = (id: string | null | undefined) => {
    if (!id) return '-'
    const host = hosts.find((item) => item.id === id)
    return host ? host.name : id.substring(0, 8)
  }

  const selectedHostAddress = (hostId?: string) => hosts.find((item) => item.id === hostId)?.address

  const openCreate = () => {
    form.resetFields()
    if (presetHost) {
      form.setFieldsValue({ host_id: presetHost, host: selectedHostAddress(presetHost) })
    }
    setModalOpen(true)
  }

  const handleCreate = async () => {
    try {
      const values = await form.validateFields()
      setSubmitting(true)
      await instanceApi.create(values)
      message.success('实例创建成功')
      setModalOpen(false)
      form.resetFields()
      fetchInstances()
    } finally {
      setSubmitting(false)
    }
  }

  const submitBatchCreate = async () => {
    const values = await batchForm.validateFields()
    const parsed = parseBatchInstances(values.instances)
    if (parsed.length === 0) {
      message.warning('没有可添加的实例')
      return
    }
    setBatchSubmitting(true)
    try {
      const res: any = await instanceApi.batchCreate(parsed)
      message.success(`批量添加完成，成功 ${res?.data?.created ?? 0}/${parsed.length}`)
      setBatchOpen(false)
      batchForm.resetFields()
      fetchInstances()
    } finally {
      setBatchSubmitting(false)
    }
  }

  const handleDelete = async (id: string) => {
    try {
      await instanceApi.delete(id)
      message.success('实例删除成功')
      fetchInstances()
    } catch {
      // interceptor already showed error
    }
  }

  const handleBatchDeploy = async () => {
    const selected = instances.filter((item) => selectedRowKeys.includes(item.id))
    if (selected.length === 0) {
      message.warning('请先选择实例')
      return
    }
    let submitted = 0
    for (const instance of selected) {
      try {
        await instanceApi.deploy(instance.id)
        submitted += 1
      } catch {
        // interceptor already showed error
      }
    }
    if (submitted === selected.length) message.success(`已提交 ${submitted} 个 MySQL 实例部署任务`)
    else message.warning(`部署任务提交完成，成功 ${submitted} 个，失败 ${selected.length - submitted} 个`)
  }

  const handleBatchHealthCheck = async () => {
    const selected = instances.filter((item) => selectedRowKeys.includes(item.id))
    if (selected.length === 0) {
      message.warning('\u8bf7\u5148\u9009\u62e9\u5b9e\u4f8b')
      return
    }
    let ok = 0
    let failed = 0
    const failedRows: string[] = []
    for (const instance of selected) {
      try {
        const res: any = await instanceApi.healthCheck(instance.id)
        const task = getHealthCheckTask(res)
        if (!isHealthCheckSuccess(task)) {
          failed += 1
          failedRows.push(formatHealthCheckFailure(instance.name, task, 'health check failed'))
        } else {
          ok += 1
        }
      } catch (err: any) {
        failed += 1
        const task = getHealthCheckErrorTask(err)
        failedRows.push(formatHealthCheckFailure(instance.name, task, err?.response?.data?.message || err?.message || '\u8bf7\u6c42\u5931\u8d25'))
      }
    }
    if (failed > 0) {
      Modal.error({
        title: `一键检测失败：${failed} 个实例未通过`,
        content: (
          <div style={{ maxHeight: 260, overflow: 'auto', whiteSpace: 'pre-wrap' }}>
            <div>{formatHealthCheckSummary(ok, failed)}</div>
            <Divider style={{ margin: '12px 0' }} />
            {failedRows.join('\n')}
          </div>
        ),
      })
    } else {
      message.success(`\u68c0\u6d4b\u6210\u529f\uff1a${ok} \u4e2a`)
    }
    fetchInstances()
  }
  const handleScanHost = async () => {
    if (!hostFilter) {
      message.warning('请先选择主机')
      return
    }
    try {
      const r: any = await hostApi.scanInstances(hostFilter, { probe_mysql: true })
      const taskId = r?.data?.task_id
      if (!taskId) {
        message.warning('扫描任务未返回 task_id')
        return
      }
      message.info('扫描任务已提交，正在打开主机详情')
      navigate(`/dashboard/hosts/${hostFilter}?tab=instances&scan_task=${taskId}`)
    } catch {
      message.error('扫描发起失败')
    }
  }

  const columns: ColumnsType<Instance> = [
    { title: '实例名称', dataIndex: 'name', key: 'name' },
    { title: '所属主机', dataIndex: 'host_id', key: 'host_id', render: (id) => hostNameById(id) },
    { title: '连接地址', key: 'endpoint', render: (_, r) => `${r.connection?.host || r.host || '-'}:${r.connection?.port || r.port || '-'}` },
    { title: '集群 ID', dataIndex: 'cluster_id', key: 'cluster_id', render: (v) => v || '-' },
    {
      title: '状态',
      key: 'status',
      render: (_, r) => {
        const role = r.status?.role
        const health = r.status?.health_status
        const run = r.status?.run_status
        if (health === 'healthy' || health === 'ok') return <Tag color="success">健康{role ? ` (${role})` : ''}</Tag>
        if (health === 'unhealthy' || health === 'failed') return <Tag color="error">异常</Tag>
        if (run === 'running') return <Tag color="processing">运行中{role ? ` (${role})` : ''}</Tag>
        if (run === 'stopped') return <Tag>已停止</Tag>
        return <Tag>未检测</Tag>
      },
    },
    { title: '创建时间', dataIndex: 'created_at', key: 'created_at', render: (t) => (t ? new Date(t).toLocaleString() : '-') },
    {
      title: '操作',
      key: 'action',
      render: (_, r) => (
        <Space>
          <Button type="link" size="small" onClick={() => navigate(`/dashboard/instances/${r.id}`)}>详情</Button>
          <Popconfirm title="确定删除该实例？" onConfirm={() => handleDelete(r.id)} okText="确定" cancelText="取消">
            <Button type="link" size="small" danger>删除</Button>
          </Popconfirm>
        </Space>
      ),
    },
  ]

  const presetHostObj = hosts.find((h) => h.id === presetHost)

  return (
    <div style={{ padding: 24 }}>
      {presetHostObj && (
        <Alert
          type="info"
          showIcon
          style={{ marginBottom: 16 }}
          message={`已按主机筛选: ${presetHostObj.name}`}
          description={<Space><span>主机地址: {presetHostObj.address}:{presetHostObj.ssh_port}</span><Button size="small" type="link" onClick={() => navigate(`/dashboard/hosts/${presetHostObj.id}`)}>打开主机详情</Button></Space>}
          closable
        />
      )}
      <Card
        title={<Space><DatabaseOutlined /><span>实例管理</span></Space>}
        extra={
          <Space>
            <Select
              placeholder="按主机筛选"
              allowClear
              style={{ width: 220 }}
              value={hostFilter}
              onChange={setHostFilter}
              options={hosts.map((h) => ({ value: h.id, label: `${h.name} (${h.address})` }))}
            />
            <Button icon={<ScanOutlined />} onClick={handleScanHost} disabled={!hostFilter}>扫描该主机</Button>
            <Button icon={<CheckCircleOutlined />} disabled={selectedRowKeys.length === 0} onClick={handleBatchHealthCheck}>一键检测选中</Button>
            <Button icon={<RocketOutlined />} disabled={selectedRowKeys.length === 0} onClick={handleBatchDeploy}>部署 MySQL 实例</Button>
            <Button icon={<ReloadOutlined />} onClick={fetchInstances}>刷新</Button>
            <Button icon={<PlusOutlined />} onClick={() => setBatchOpen(true)}>批量添加</Button>
            <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>添加实例</Button>
          </Space>
        }
      >
        <Table
          rowSelection={{ selectedRowKeys, onChange: setSelectedRowKeys }}
          columns={columns}
          dataSource={instances}
          rowKey="id"
          loading={loading}
          pagination={{ pageSize: 20 }}
          locale={{
            emptyText: (
              <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无实例">
                <Space>
                  {hostFilter && <Button type="primary" icon={<ScanOutlined />} onClick={handleScanHost}>扫描该主机并纳管</Button>}
                  <Button icon={<PlusOutlined />} onClick={openCreate}>添加实例</Button>
                  <Button icon={<PlusOutlined />} onClick={() => setBatchOpen(true)}>批量添加</Button>
                </Space>
              </Empty>
            ),
          }}
        />
      </Card>

      <Modal title="添加实例" open={modalOpen} onCancel={() => setModalOpen(false)} onOk={handleCreate} confirmLoading={submitting} okText="创建" cancelText="取消" width={640}>
        <Form form={form} layout="vertical" autoComplete="off">
          <Form.Item name="name" label="实例名称" rules={[{ required: true, message: '请输入实例名称' }]}>
            <Input placeholder="例如: order-db-01" />
          </Form.Item>
          <Form.Item name="host_id" label="所属主机" rules={[{ required: true, message: '请选择所属主机' }]}>
            <Select
              placeholder="选择主机"
              options={hosts.map((h) => ({ value: h.id, label: `${h.name} (${h.address}:${h.ssh_port})` }))}
              onChange={(value) => form.setFieldsValue({ host: selectedHostAddress(value) })}
            />
          </Form.Item>
          <Form.Item name="host" label="连接地址" rules={[{ required: true, message: '请输入连接地址' }]}>
            <Input placeholder="例如: 192.168.1.100" />
          </Form.Item>
          <Form.Item name="port" label="端口" rules={[{ required: true, message: '请输入端口' }]} initialValue={3306}>
            <InputNumber min={1} max={65535} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="username" label="用户名" rules={[{ required: true, message: '请输入用户名' }]}>
            <Input placeholder="例如: root" />
          </Form.Item>
          <Form.Item name="password" label="密码" rules={[{ required: true, message: '请输入密码' }]}>
            <Input.Password placeholder="MySQL 密码" autoComplete="new-password" />
          </Form.Item>
          <Form.Item name="cluster_id" label="集群 ID">
            <Input placeholder="例如: mgr-cluster-01" />
          </Form.Item>
          <Divider plain>部署参数</Divider>
          <Form.Item name="version_id" label="目标版本">
            <Select
              allowClear
              showSearch
              placeholder="可留空，后续部署或升级时再选择"
              options={versions.map((v) => ({ value: v.id, label: `${v.flavor} ${v.version}${v.is_lts ? ' [LTS]' : ''}${v.status === 'eol' ? ' [EOL]' : ''}` }))}
            />
          </Form.Item>
          <Form.Item name="basedir" label="basedir"><Input placeholder="/opt/mysql-8.0.36" /></Form.Item>
          <Form.Item name="datadir" label="datadir"><Input placeholder="/data/mysql/3307" /></Form.Item>
          <Form.Item name="os_user" label="OS 用户" initialValue="mysql"><Input placeholder="mysql" /></Form.Item>
          <Form.Item name="package_url" label="package_url"><Input placeholder="可留空，使用版本目录默认包地址" /></Form.Item>
        </Form>
      </Modal>

      <Modal title="批量添加实例" open={batchOpen} onCancel={() => setBatchOpen(false)} onOk={submitBatchCreate} confirmLoading={batchSubmitting} okText="批量添加" cancelText="取消" width={760}>
        <Form form={batchForm} layout="vertical">
          <Form.Item
            name="instances"
            label="实例清单"
            extra="支持 CSV：name,host,port,username,password,host_id,cluster_id；也支持 JSON 数组。host_id 可从主机列表复制，留空也可先按连接地址纳管。"
            rules={[{ required: true, message: '请输入实例清单' }]}
          >
            <Input.TextArea rows={10} placeholder={'mysql-3306,10.1.81.41,3306,root,123456,host-id,cluster-a\n备用格式也可粘贴 JSON 数组'} />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}

export default InstanceList
