import React, { useEffect, useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { Button, Card, Divider, Empty, Form, Input, InputNumber, message, Modal, Popconfirm, Select, Space, Table, Tag, Tooltip } from 'antd'
import { CheckCircleOutlined, CopyOutlined, DatabaseOutlined, EyeOutlined, EyeInvisibleOutlined, PlusOutlined, ReloadOutlined, ScanOutlined } from '@ant-design/icons'
import type { ColumnsType } from 'antd/es/table'
import { extractTaskPayload, hostApi, instanceApi, versionApi, clusterDeployApi, type Host, type Instance, type VersionEntry } from '../services/api'
import { isSecondaryPasswordEnabled, isSecondaryPasswordVerified, verifySecondaryPassword } from '../services/sessionSecrets'
import { formatClusterRole } from '../services/roleDisplay'

const isSuccessfulTaskStatus = (status?: string) => {
  const normalized = (status || '').toLowerCase()
  return ['success', 'succeeded', 'healthy', 'ok', 'completed'].includes(normalized)
}

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
  `一键检测完成：成功 ${ok} 个，异常 ${failed} 个。后端 HTTP 200 只表示批量请求完成，逐实例按 is_healthy/status/error_message 判断。`

const healthStatusFromBatchRow = (row: any) => {
  if (!row) return 'unhealthy'
  if (row.is_healthy === true) return 'healthy'
  if (isSuccessfulTaskStatus(row.status)) return 'healthy'
  return 'unhealthy'
}

const formatBatchHealthFailure = (instanceName: string, row: any) => {
  const connection = row?.connection || [row?.connection_host, row?.connection_port].filter(Boolean).join(':')
  const agent = row?.agent_endpoint || [row?.agent_host, row?.agent_port].filter(Boolean).join(':')
  const target = row?.target_endpoint || [row?.target_host, row?.target_port].filter(Boolean).join(':')
  const context = [
    connection ? `连接=${connection}` : '',
    agent ? `Agent=${agent}` : '',
    target ? `实际检测=${target}` : '',
    row?.target_user ? `用户=${row.target_user}` : '',
  ].filter(Boolean).join(' | ')
  const parts = [
    `${instanceName}${context ? ` [${context}]` : ''}: ${row?.error_message || row?.message || row?.status || 'health check failed'}`,
    row?.status ? `status=${row.status}` : '',
    row?.task_id ? `task_id=${row.task_id}` : '',
  ].filter(Boolean)
  return parts.join(' | ')
}

const InstanceList: React.FC = () => {
  const [searchParams] = useSearchParams()
  const navigate = useNavigate()
  const presetHost = searchParams.get('preset_host') || searchParams.get('host_id') || undefined

  const [instances, setInstances] = useState<Instance[]>([])
  const [hosts, setHosts] = useState<Host[]>([])
  const [versions, setVersions] = useState<VersionEntry[]>([])
  const [clusters, setClusters] = useState<any[]>([])
  const [loading, setLoading] = useState(false)
  const [hostFilter, setHostFilter] = useState<string | undefined>(presetHost)
  const [modalOpen, setModalOpen] = useState(false)
  const [batchOpen, setBatchOpen] = useState(false)
  const [submitting, setSubmitting] = useState(false)
  const [batchSubmitting, setBatchSubmitting] = useState(false)
  const [selectedRowKeys, setSelectedRowKeys] = useState<React.Key[]>([])
  const [form] = Form.useForm()
  const [batchForm] = Form.useForm()
  const [credentialsCache, setCredentialsCache] = useState<Record<string, { username: string; password: string }>>({})
  const [visiblePasswords, setVisiblePasswords] = useState<Set<string>>(new Set())
  const [secondaryAuthOpen, setSecondaryAuthOpen] = useState(false)
  const [secondaryAuthForm] = Form.useForm()
  const [pendingCredentialId, setPendingCredentialId] = useState<string | null>(null)

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

  const fetchClusters = async () => {
    try {
      const res: any = await clusterDeployApi.listClusters()
      setClusters(res.data || [])
    } catch {
      setClusters([])
    }
  }

  useEffect(() => {
    fetchHosts()
    fetchClusters()
    versionApi.list().then((res: any) => setVersions(res?.data || [])).catch(() => setVersions([]))
  }, [])

  useEffect(() => {
    fetchInstances()
    if (presetHost) form.setFieldsValue({ host_id: presetHost })
  }, [hostFilter, presetHost])

  const hostNameById = (id: string | null | undefined) => {
    if (!id) return '-'
    const host = hosts.find((item) => item.id === id)
    return host ? host.name : id.substring(0, 8)
  }

  const hostAddressById = (id: string | null | undefined) => hosts.find((item) => item.id === id)?.address

  const selectedHostAddress = (hostId?: string) => hosts.find((item) => item.id === hostId)?.address

  const archByClusterId = (clusterId?: string) => {
    if (!clusterId) return undefined
    const cluster = clusters.find((item: any) => item.cluster_id === clusterId || item.deployment_id === clusterId)
    return cluster?.arch || cluster?.cluster_type
  }

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
    const instances = (values.instances || []).filter((i: any) => i.host && i.host.trim())
    if (instances.length === 0) {
      message.warning('请至少填写一个实例的连接地址')
      return
    }
    const parsed = instances.map((i: any) => ({
      name: i.name || `${i.host}-${i.port || 3306}`,
      host: i.host,
      port: i.port || 3306,
      username: i.username || 'root',
      password: i.password || '',
      host_id: i.host_id || undefined,
      cluster_id: i.cluster_id || undefined,
      version_id: i.version_id || undefined,
      basedir: i.basedir || undefined,
      datadir: i.datadir || undefined,
      os_user: i.os_user || undefined,
      package_url: i.package_url || undefined,
    }))
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

  const handleBatchHealthCheck = async () => {
    const selected = instances.filter((item) => selectedRowKeys.includes(item.id))
    if (selected.length === 0) {
      message.warning('\u8bf7\u5148\u9009\u62e9\u5b9e\u4f8b')
      return
    }

    const selectedIds = selected.map((item) => item.id)
    const selectedById = new Map(selected.map((item) => [item.id, item]))
    let ok = 0
    let failed = 0
    const failedRows: string[] = []

    try {
      const res: any = await instanceApi.batchHealthCheck(selectedIds)
      const rows = Array.isArray(res?.data) ? res.data : []
      const statusById = new Map<string, string>()

      rows.forEach((row: any) => {
        const instanceId = row?.instance_id
        const instance = selectedById.get(instanceId)
        const status = healthStatusFromBatchRow(row)
        if (instanceId) statusById.set(instanceId, status)
        if (status === 'healthy') {
          ok += 1
        } else {
          failed += 1
          failedRows.push(formatBatchHealthFailure(instance?.name || instanceId || '-', row))
        }
      })

      selectedIds.forEach((id) => {
        if (statusById.has(id)) return
        failed += 1
        failedRows.push(formatBatchHealthFailure(selectedById.get(id)?.name || id, {
          status: 'error',
          error_message: 'health check did not return a result',
        }))
      })

      setInstances((prev) => prev.map((item) => {
        const status = statusById.get(item.id)
        if (!status) return item
        return {
          ...item,
          status: {
            ...(item.status || {}),
            health_status: status,
            run_status: status === 'healthy' ? 'running' : (item.status?.run_status || 'unknown'),
            role: item.status?.role || '',
          },
        }
      }))
    } catch (err: any) {
      failed = selected.length
      const task = getHealthCheckErrorTask(err)
      failedRows.push(formatHealthCheckFailure('batch', task, err?.response?.data?.message || err?.message || '\u8bf7\u6c42\u5931\u8d25'))
    }

    if (failed > 0) {
      Modal.warning({
        title: `一键检测完成：${failed} 个实例异常`,
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

  const fetchCredential = async (instanceId: string) => {
    if (credentialsCache[instanceId]) return credentialsCache[instanceId]
    try {
      const res: any = await instanceApi.getCredentials(instanceId)
      const cred = { username: res?.data?.username || 'root', password: res?.data?.password || '' }
      setCredentialsCache((prev) => ({ ...prev, [instanceId]: cred }))
      return cred
    } catch {
      return { username: 'root', password: '(获取失败)' }
    }
  }

  const togglePasswordVisible = async (instanceId: string) => {
    if (visiblePasswords.has(instanceId)) {
      setVisiblePasswords((prev) => { const next = new Set(prev); next.delete(instanceId); return next })
    } else {
      // Check if secondary password is enabled and not yet verified in this session
      if (isSecondaryPasswordEnabled() && !isSecondaryPasswordVerified()) {
        setPendingCredentialId(instanceId)
        secondaryAuthForm.resetFields()
        setSecondaryAuthOpen(true)
        return
      }
      await fetchCredential(instanceId)
      setVisiblePasswords((prev) => new Set(prev).add(instanceId))
    }
  }

  const handleSecondaryAuth = async () => {
    const values = await secondaryAuthForm.validateFields()
    if (!(await verifySecondaryPassword(values.secondary_password))) {
      message.error('二级密码错误')
      return
    }
    setSecondaryAuthOpen(false)
    if (pendingCredentialId) {
      await fetchCredential(pendingCredentialId)
      setVisiblePasswords((prev) => new Set(prev).add(pendingCredentialId))
      setPendingCredentialId(null)
    }
  }

  const copyToClipboard = (text: string) => {
    navigator.clipboard.writeText(text).then(() => message.success('已复制'))
  }

  const columns: ColumnsType<Instance> = [
    { title: '实例名称', dataIndex: 'name', key: 'name' },
    { title: '所属主机', dataIndex: 'host_id', key: 'host_id', render: (id) => hostNameById(id) },
    {
      title: '连接地址',
      key: 'endpoint',
      render: (_, r) => {
        const host = r.connection?.host || r.host || hostAddressById(r.host_id)
        const port = r.connection?.port || r.port || 3306
        return host ? `${host}:${port}` : '-'
      },
    },
    { title: '用户名', key: 'username', render: (_, r) => r.connection?.username || 'root' },
    {
      title: '密码',
      key: 'password',
      width: 180,
      render: (_, r) => {
        const visible = visiblePasswords.has(r.id)
        const cred = credentialsCache[r.id]
        const pw = visible ? (cred?.password || '...') : '••••••••'
        return (
          <Space size={4}>
            <span style={{ fontFamily: 'monospace', fontSize: 12, maxWidth: 100, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
              {pw}
            </span>
            <Tooltip title={visible ? '隐藏' : '查看密码'}>
              <Button type="text" size="small" icon={visible ? <EyeInvisibleOutlined /> : <EyeOutlined />} onClick={() => togglePasswordVisible(r.id)} />
            </Tooltip>
            {visible && cred?.password && (
              <Tooltip title="复制密码">
                <Button type="text" size="small" icon={<CopyOutlined />} onClick={() => copyToClipboard(cred.password)} />
              </Tooltip>
            )}
          </Space>
        )
      },
    },
    { title: '集群 ID', dataIndex: 'cluster_id', key: 'cluster_id', render: (v) => v || '-' },
    {
      title: '状态',
      key: 'status',
      render: (_, r) => {
        const role = r.status?.role
        const displayRole = formatClusterRole(archByClusterId(r.cluster_id), role)
        const health = r.status?.health_status
        const run = r.status?.run_status
        if (run === 'stopped') return <Tag color="error">已停止</Tag>
        if (health === 'healthy' || health === 'ok') return <Tag color="success">运行中{role ? ` (${displayRole})` : ''}</Tag>
        if (health === 'unhealthy' || health === 'failed') return <Tag color="error">异常</Tag>
        if (run === 'running') return <Tag color="success">运行中{role ? ` (${displayRole})` : ''}</Tag>
        return <Tag color="default">未检测</Tag>
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

  return (
    <div style={{ padding: 24 }}>
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
            <Button icon={<CheckCircleOutlined />} disabled={selectedRowKeys.length === 0} onClick={handleBatchHealthCheck}>一键检测选中</Button>
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
          pagination={{ pageSize: 10 }}
          locale={{
            emptyText: (
              <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无实例">
                <Space>
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
          <Form.Item name="cluster_id" label="所属集群">
            <Select
              allowClear
              placeholder="选择集群（可选）"
              options={clusters.map((c: any) => ({ value: c.cluster_id, label: `${c.cluster_id} (${c.arch || '未知架构'}) - ${c.node_count || 0}节点` }))}
            />
          </Form.Item>
          <Divider plain>部署参数</Divider>
          <Form.Item name="version_id" label="目标版本">
            <Select
              allowClear
              showSearch
              placeholder="可留空，后续部署或升级时再选择"
              options={versions.map((v) => ({ value: v.id, label: `${v.flavor} ${v.version}${v.is_lts ? ' [LTS]' : ''}${v.status === 'eol' ? ' [EOL]' : ''}${v.local_available ? ' [存在]' : ' [下载]'}` }))}
            />
          </Form.Item>
          <Form.Item name="basedir" label="basedir"><Input placeholder="/opt/mysql-8.0.36" /></Form.Item>
          <Form.Item name="datadir" label="datadir"><Input placeholder="/data/mysql/3307" /></Form.Item>
          <Form.Item name="os_user" label="OS 用户" initialValue="mysql"><Input placeholder="mysql" /></Form.Item>
          <Form.Item name="package_url" label="package_url"><Input placeholder="可留空，使用版本目录默认包地址" /></Form.Item>
        </Form>
      </Modal>

      <Modal title="批量添加实例" open={batchOpen} onCancel={() => setBatchOpen(false)} onOk={submitBatchCreate} confirmLoading={batchSubmitting} okText="批量添加" cancelText="取消" width={900}>
        <Form form={batchForm} layout="vertical">
          <Form.List name="instances" initialValue={[{ name: '', host: '', port: 3306, username: 'root', password: '', os_user: 'mysql' }]}>
            {(fields, { add, remove }) => (
              <>
                <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr 70px 80px 1fr 40px', gap: 8, marginBottom: 4, fontSize: 12, fontWeight: 600, color: '#666' }}>
                  <span>实例名</span><span>地址</span><span>端口</span><span>用户</span><span>密码</span><span></span>
                </div>
                {fields.map(({ key, name }) => (
                  <div key={key} style={{ marginBottom: 8 }}>
                    <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr 70px 80px 1fr 40px', gap: 8, marginBottom: 4, alignItems: 'center' }}>
                      <Form.Item name={[name, 'name']} style={{ margin: 0 }}><Input size="small" placeholder="mysql-3306" /></Form.Item>
                      <Form.Item name={[name, 'host']} style={{ margin: 0 }} rules={[{ required: true }]}><Input size="small" placeholder="10.1.81.41" /></Form.Item>
                      <Form.Item name={[name, 'port']} style={{ margin: 0 }} initialValue={3306}><InputNumber size="small" min={1} max={65535} style={{ width: '100%' }} /></Form.Item>
                      <Form.Item name={[name, 'username']} style={{ margin: 0 }} initialValue="root"><Input size="small" /></Form.Item>
                      <Form.Item name={[name, 'password']} style={{ margin: 0 }}><Input.Password size="small" placeholder="MySQL密码" autoComplete="new-password" /></Form.Item>
                      <Button size="small" danger type="text" disabled={fields.length <= 1} onClick={() => remove(name)} style={{ padding: 0, fontSize: 16 }}>×</Button>
                    </div>
                    <div style={{ display: 'grid', gridTemplateColumns: '1.2fr 1.2fr 1fr 1fr 80px 1.2fr', gap: 8 }}>
                      <Form.Item name={[name, 'host_id']} style={{ margin: 0 }}>
                        <Select
                          size="small"
                          allowClear
                          placeholder="绑定主机"
                          options={hosts.map((h) => ({ value: h.id, label: `${h.name} (${h.address})` }))}
                          onChange={(value) => {
                            const host = hosts.find((h) => h.id === value)
                            if (host) {
                              const rows = batchForm.getFieldValue('instances') || []
                              rows[name] = { ...rows[name], host: host.address }
                              batchForm.setFieldsValue({ instances: rows })
                            }
                          }}
                        />
                      </Form.Item>
                      <Form.Item name={[name, 'version_id']} style={{ margin: 0 }}>
                        <Select
                          size="small"
                          allowClear
                          showSearch
                          optionFilterProp="label"
                          placeholder="目标版本"
                          options={versions.map((v) => ({ value: v.id, label: `${v.flavor} ${v.version}` }))}
                        />
                      </Form.Item>
                      <Form.Item name={[name, 'basedir']} style={{ margin: 0 }}><Input size="small" placeholder="basedir" /></Form.Item>
                      <Form.Item name={[name, 'datadir']} style={{ margin: 0 }}><Input size="small" placeholder="datadir" /></Form.Item>
                      <Form.Item name={[name, 'os_user']} style={{ margin: 0 }} initialValue="mysql"><Input size="small" placeholder="OS用户" /></Form.Item>
                      <Form.Item name={[name, 'package_url']} style={{ margin: 0 }}><Input size="small" placeholder="package_url" /></Form.Item>
                    </div>
                  </div>
                ))}
                <Button size="small" type="dashed" onClick={() => add({ name: '', host: '', port: 3306, username: 'root', password: '', os_user: 'mysql' })} style={{ marginTop: 4 }}>+ 添加一行</Button>
              </>
            )}
          </Form.List>
        </Form>
      </Modal>
      <Modal title="安全验证" open={secondaryAuthOpen} onCancel={() => { setSecondaryAuthOpen(false); setPendingCredentialId(null) }} onOk={handleSecondaryAuth} okText="验证" cancelText="取消" destroyOnClose>
        <p>查看密码需要输入二级密码验证。</p>
        <Form form={secondaryAuthForm} layout="vertical">
          <Form.Item name="secondary_password" label="二级密码" rules={[{ required: true, message: '请输入二级密码' }]}>
            <Input.Password placeholder="请输入二级密码" autoComplete="off" />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}

export default InstanceList
