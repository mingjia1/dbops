import React, { useEffect, useRef, useState } from 'react'
import { useNavigate, useParams, useSearchParams } from 'react-router-dom'
import {
  Card, Descriptions, Button, Space, Tag, Spin, message, Alert, Table, Popconfirm,
  Tabs, Modal, Form, Input, Result, Statistic, Row, Col, Badge, Radio, Switch, Tooltip, Select,
} from 'antd'
import {
  ArrowLeftOutlined, ThunderboltOutlined, DatabaseOutlined, PlusOutlined,
  ScanOutlined, ImportOutlined, CheckCircleOutlined, CloseCircleOutlined,
} from '@ant-design/icons'
import type { ColumnsType } from 'antd/es/table'
import {
  hostApi, instanceApi, clusterDeployApi,
  type Host, type HostTestResult, type Instance,
  type HostScanResult, type ScannedInstance,
} from '../services/api'

const HostDetail: React.FC = () => {
  const navigate = useNavigate()
  const { id } = useParams<{ id: string }>()
  const [searchParams, setSearchParams] = useSearchParams()
  const initialTab = searchParams.get('tab') || 'basic'

  const [host, setHost] = useState<Host | null>(null)
  const [loading, setLoading] = useState(true)
  const [testing, setTesting] = useState(false)
  const [testResult, setTestResult] = useState<HostTestResult | null>(null)
  const [instances, setInstances] = useState<Instance[]>([])
  const [instLoading, setInstLoading] = useState(false)
  const [tab, setTab] = useState(initialTab)

  const [scanning, setScanning] = useState(false)
  const [scanResult, setScanResult] = useState<HostScanResult | null>(null)
  const [scanError, setScanError] = useState<string | null>(null)
  const scanPollRef = useRef<number | null>(null)
  const initialScanTask = searchParams.get('scan_task')

  const [registerOpen, setRegisterOpen] = useState(false)
  const [registering, setRegistering] = useState(false)
  const [registerTarget, setRegisterTarget] = useState<ScannedInstance | null>(null)
  const [registerForm] = Form.useForm()
  const [batchRegisterOpen, setBatchRegisterOpen] = useState(false)
  const [batchRegistering, setBatchRegistering] = useState(false)
  const [batchRegisterForm] = Form.useForm()

  const [scanConfigOpen, setScanConfigOpen] = useState(false)
  const [scanMode, setScanMode] = useState<'default' | 'custom' | 'range'>('default')
  const [scanPorts, setScanPorts] = useState<number[]>([3306, 33060, 3307])
  const [scanRange, setScanRange] = useState<string>('3306-3310')
  const [discoverProcess, setDiscoverProcess] = useState(true)
  const [clusters, setClusters] = useState<any[]>([])
  const [scanForm] = Form.useForm()

  useEffect(() => {
    setTab(searchParams.get('tab') || 'basic')
  }, [searchParams])

  const fetchHost = async () => {
    if (!id) return
    setLoading(true)
    try {
      const res: any = await hostApi.get(id)
      setHost(res.data)
    } catch {
      message.error('主机不存在')
      navigate('/dashboard/hosts')
    } finally {
      setLoading(false)
    }
  }

  const fetchInstances = async () => {
    if (!id) return
    setInstLoading(true)
    try {
      const res: any = await instanceApi.listByHost(id)
      setInstances(res.data || [])
    } catch {
      setInstances([])
    } finally {
      setInstLoading(false)
    }
  }

  useEffect(() => {
    fetchHost()
    fetchClusters()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [id])

  useEffect(() => {
    if (id) fetchInstances()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [id])

  useEffect(() => () => {
    if (scanPollRef.current) window.clearInterval(scanPollRef.current)
  }, [])

  useEffect(() => {
    if (initialScanTask && id) {
      pollScanResult(initialScanTask)
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [initialScanTask, id])

  const stopScanPolling = () => {
    if (scanPollRef.current) {
      window.clearInterval(scanPollRef.current)
      scanPollRef.current = null
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

  const pollScanResult = (taskId: string) => {
    stopScanPolling()
    setScanning(true)
    setTab('instances')
    let attempts = 0
    scanPollRef.current = window.setInterval(async () => {
      attempts += 1
      try {
        const r: any = await hostApi.getScanResult(id!, taskId)
        const data: HostScanResult = r?.data
        if (data) setScanResult(data)
        if (data?.status === 'success' || data?.status === 'failed') {
          stopScanPolling()
          setScanning(false)
          if (data.status === 'success') {
            const newOnes = (data.instances || []).filter((i) => !i.already_managed)
            message.success(data.message || `扫描完成, 发现 ${data.instances.length} 个实例`)
            if (newOnes.length === 0) {
              const sp = new URLSearchParams(searchParams)
              sp.delete('scan_task')
              setSearchParams(sp, { replace: true })
            }
          } else {
            setScanError(data.error || data.message || '扫描失败')
            message.error('扫描失败')
          }
        } else if (attempts > 60) {
          stopScanPolling()
          setScanning(false)
          setScanError('扫描超时, 请稍后重试')
        }
      } catch (err: any) {
        stopScanPolling()
        setScanning(false)
        setScanError(err?.response?.data?.message || err?.message || '扫描结果查询失败')
      }
    }, 2000)
  }

  const openScanConfig = () => {
    scanForm.setFieldsValue({
      mode: scanMode,
      ports: scanPorts,
      port_range: scanRange,
    })
    setScanConfigOpen(true)
  }

  const submitScan = async () => {
    if (!id) return
    let payload: { ports?: number[]; port_range?: string; probe_mysql?: boolean; discover_process?: boolean } = { probe_mysql: true, discover_process: discoverProcess }
    if (scanMode === 'default') {
      payload = { probe_mysql: true, discover_process: discoverProcess, port_range: '3300-3400,33061,33060' }
    } else if (scanMode === 'custom') {
      const v = scanForm.getFieldValue('ports') as number[] | undefined
      if (!v || v.length === 0) {
        message.warning('请至少输入一个端口')
        return
      }
      payload.ports = v
    } else if (scanMode === 'range') {
      const v = (scanForm.getFieldValue('port_range') as string | undefined)?.trim()
      if (!v) {
        message.warning('请输入端口范围')
        return
      }
      payload.port_range = v
    }
    setScanConfigOpen(false)
    setScanning(true)
    setScanResult(null)
    setScanError(null)
    setTab('instances')
    try {
      const r: any = await hostApi.scanInstances(id, payload)
      const taskId = r?.data?.task_id
      if (!taskId) {
        message.warning('后端未返回 task_id, 请手动添加实例')
        setScanning(false)
        return
      }
      pollScanResult(taskId)
    } catch (err: any) {
      setScanning(false)
      message.error(err?.response?.data?.message || err?.message || '扫描发起失败')
      setScanError(err?.response?.data?.message || err?.message || '扫描发起失败')
    }
  }

  const handleStartScan = openScanConfig

  const openRegister = (s: ScannedInstance) => {
    setRegisterTarget(s)
    registerForm.resetFields()
    registerForm.setFieldsValue({
      name: s.recommended_name || `${host?.name || 'host'}-${s.port}`,
      username: 'root',
      port: s.port,
    })
    setRegisterOpen(true)
  }

  const submitRegister = async () => {
    if (!id || !registerTarget) return
    try {
      const values = await registerForm.validateFields()
      setRegistering(true)
      try {
        await hostApi.registerScannedInstance(id, {
          port: registerTarget.port,
          name: values.name,
          username: values.username,
          password: values.password,
          cluster_id: values.cluster_id || undefined,
        })
        message.success(`实例 ${values.name} 已纳管`)
        setRegisterOpen(false)
        fetchInstances()
        if (scanResult) {
          setScanResult({
            ...scanResult,
            instances: scanResult.instances.map((i) =>
              i.port === registerTarget.port ? { ...i, already_managed: true } : i,
            ),
          })
        }
      } catch (err: any) {
        message.error(err?.response?.data?.message || err?.message || '纳管失败')
      }
    } catch {
      // validate
    } finally {
      setRegistering(false)
    }
  }

  const openBatchRegister = () => {
    batchRegisterForm.resetFields()
    batchRegisterForm.setFieldsValue({ username: 'root' })
    setBatchRegisterOpen(true)
  }

  const submitBatchRegister = async () => {
    if (!id || !scanResult) return
    const targets = (scanResult.instances || []).filter((item) => !item.already_managed)
    if (targets.length === 0) {
      message.info('没有待纳管实例')
      return
    }
    const values = await batchRegisterForm.validateFields()
    setBatchRegistering(true)
    try {
      const payload = targets.map((item) => ({
        port: item.port,
        name: item.recommended_name || `${host?.name || 'host'}-${item.port}`,
        username: values.username,
        password: values.password,
        cluster_id: values.cluster_id || undefined,
      }))
      const res: any = await hostApi.registerScannedInstances(id, payload)
      const rows = res?.data?.rows || []
      const registeredPorts = new Set(rows.filter((row: any) => row.status === 'registered' || row.status === 'skipped').map((row: any) => row.port))
      message.success(`一键纳管完成，成功 ${res?.data?.registered ?? 0} 个，跳过 ${res?.data?.skipped ?? 0} 个`)
      setBatchRegisterOpen(false)
      fetchInstances()
      setScanResult({
        ...scanResult,
        instances: scanResult.instances.map((item) =>
          registeredPorts.has(item.port) ? { ...item, already_managed: true } : item,
        ),
      })
    } finally {
      setBatchRegistering(false)
    }
  }

  const handleDeleteInstance = async (iid: string) => {
    try {
      await instanceApi.delete(iid)
      message.success('实例删除成功')
      fetchInstances()
    } catch {
      // interceptor handled
    }
  }

  const instanceColumns: ColumnsType<Instance> = [
    { title: '实例名称', dataIndex: 'name', key: 'name' },
    { title: '集群 ID', dataIndex: 'cluster_id', key: 'cluster_id', render: (v) => v || '-' },
    {
      title: '状态', key: 'status',
      render: (_, r) => {
        const role = r.status?.role
        const health = r.status?.health_status
        if (health === 'healthy' || health === 'ok') return <Tag color="success">健康{role ? ` (${role})` : ''}</Tag>
        if (health === 'unhealthy' || health === 'failed') return <Tag color="error">异常</Tag>
        return <Tag>未检测</Tag>
      },
    },
    {
      title: '创建时间', dataIndex: 'created_at', key: 'created_at',
      render: (t) => (t ? new Date(t).toLocaleString() : '-'),
    },
    {
      title: '操作', key: 'action',
      render: (_, r) => (
        <Space>
          <Button type="link" size="small" onClick={() => navigate(`/dashboard/instances/${r.id}`)}>
            详情
          </Button>
          <Popconfirm title="确定删除?" onConfirm={() => handleDeleteInstance(r.id)} okText="确定" cancelText="取消">
            <Button type="link" size="small" danger>删除</Button>
          </Popconfirm>
        </Space>
      ),
    },
  ]

  const scannedColumns: ColumnsType<ScannedInstance> = [
    {
      title: '端口', dataIndex: 'port', key: 'port',
      render: (p) => <Tag color="blue">{p}</Tag>,
    },
    {
      title: '类型', dataIndex: 'flavor', key: 'flavor',
      render: (f: string) => {
        if (f === 'mysql') return <Tag color="success">MySQL</Tag>
        if (f === 'mariadb') return <Tag color="purple">MariaDB</Tag>
        if (f === 'tidb') return <Tag color="cyan">TiDB</Tag>
        if (f === 'unknown') return <Tag>未知</Tag>
        return f || '-'
      },
    },
    { title: '版本', dataIndex: 'version', key: 'version', render: (v) => v || '-' },
    {
      title: '角色', dataIndex: 'role', key: 'role',
      render: (r) => r ? <Tag color="purple">{r}</Tag> : '-',
    },
    {
      title: '运行状态', dataIndex: 'running', key: 'running',
      render: (running: boolean) => running
        ? <Badge status="success" text="运行中" />
        : <Badge status="default" text="已停止" />,
    },
    {
      title: '发现方式', dataIndex: 'source', key: 'source',
      render: (s: string) => {
        if (s === 'tcp+process') return <Tooltip title="TCP 探测 + 进程发现"><Tag color="cyan">TCP+进程</Tag></Tooltip>
        if (s === 'process') return <Tooltip title="通过 ps 命令发现的 mysqld 进程"><Tag color="geekblue">进程</Tag></Tooltip>
        if (s === 'tcp') return <Tooltip title="通过 TCP 端口探测发现"><Tag color="blue">TCP</Tag></Tooltip>
        return s || '-'
      },
    },
    {
      title: 'PID', dataIndex: 'pid', key: 'pid',
      render: (pid: number) => pid || '-',
    },
    {
      title: '内存(MB)', dataIndex: 'memory_mb', key: 'memory_mb',
      render: (mb: number) => mb ? `${mb}` : '-',
    },
    {
      title: '数据目录', dataIndex: 'datadir', key: 'datadir',
      ellipsis: true,
      width: 160,
      render: (d: string) => d || '-',
    },
    {
      title: '纳管', dataIndex: 'already_managed', key: 'already_managed',
      render: (managed: boolean) => managed
        ? <Tag color="success" icon={<CheckCircleOutlined />}>已纳管</Tag>
        : <Tag color="warning" icon={<CloseCircleOutlined />}>未纳管</Tag>,
    },
    {
      title: '操作', key: 'action',
      render: (_, r) => r.already_managed ? (
        <Button type="link" size="small" disabled>已纳管</Button>
      ) : (
        <Button type="link" size="small" icon={<ImportOutlined />} onClick={() => openRegister(r)}>
          一键纳管
        </Button>
      ),
    },
  ]

  const handleTest = async () => {
    if (!id) return
    setTesting(true)
    setTestResult(null)
    try {
      const res: any = await hostApi.testConnection(id)
      const initial: HostTestResult = res.data
      setTestResult(initial)

      const taskId = initial.task_id
      let attempts = 0
      const interval = setInterval(async () => {
        attempts++
        try {
          const r: any = await hostApi.getTestResult(taskId)
          setTestResult(r.data)
          if (r.data.status === 'success' || r.data.status === 'failed' || attempts >= 10) {
            clearInterval(interval)
            setTesting(false)
            if (r.data.status === 'success' || r.data.status === 'failed') {
              fetchHost()
            }
          }
        } catch {
          clearInterval(interval)
          setTesting(false)
        }
      }, 1000)
    } catch {
      setTesting(false)
    }
  }

  const onTabChange = (k: string) => {
    setTab(k)
    const sp = new URLSearchParams(searchParams)
    if (k === 'basic') sp.delete('tab')
    else sp.set('tab', k)
    setSearchParams(sp, { replace: true })
  }

  if (loading) {
    return (
      <div style={{ padding: '24px', textAlign: 'center' }}>
        <Spin />
      </div>
    )
  }

  if (!host) return null

  const newInstances = scanResult?.instances?.filter((i) => !i.already_managed) || []
  const managedInScan = scanResult?.instances?.filter((i) => i.already_managed) || []

  return (
    <div style={{ padding: '24px' }}>
      <Card
        title={
          <Space>
            <Button type="text" icon={<ArrowLeftOutlined />} onClick={() => navigate('/dashboard/hosts')} />
            <span>主机详情 - {host.name}</span>
          </Space>
        }
        extra={
          <Space>
            <Button icon={<ThunderboltOutlined />} onClick={handleTest} loading={testing}>
              测试连接
            </Button>
            <Button
              type="primary"
              icon={<ScanOutlined />}
              onClick={handleStartScan}
              loading={scanning}
            >
              {scanning ? '扫描中...' : '自动扫描实例'}
            </Button>
            <Button onClick={() => navigate(`/dashboard/hosts/${host.id}/edit`)}>
              编辑
            </Button>
          </Space>
        }
      >
        {testResult && (
          <Alert
            style={{ marginBottom: 16 }}
            type={
              testResult.status === 'success'
                ? 'success'
                : testResult.status === 'failed'
                ? 'error'
                : 'info'
            }
            message={
              testResult.status === 'success'
                ? '连接成功'
                : testResult.status === 'failed'
                ? '连接失败'
                : '正在检测...'
            }
            description={
              <div>
                <div>{testResult.message}</div>
                {testResult.latency_ms > 0 && (
                  <div style={{ marginTop: 4 }}>延迟: {testResult.latency_ms} ms</div>
                )}
              </div>
            }
            showIcon
          />
        )}

        <Tabs
          activeKey={tab}
          onChange={onTabChange}
          items={[
            {
              key: 'basic',
              label: '基础信息',
              children: (
                <Descriptions bordered column={2}>
                  <Descriptions.Item label="主机ID">{host.id}</Descriptions.Item>
                  <Descriptions.Item label="主机名称">{host.name}</Descriptions.Item>
                  <Descriptions.Item label="地址">{host.address}</Descriptions.Item>
                  <Descriptions.Item label="SSH 端口">{host.ssh_port}</Descriptions.Item>
                  <Descriptions.Item label="SSH 用户">{host.ssh_user}</Descriptions.Item>
                  <Descriptions.Item label="认证方式">
                    <Tag>{host.ssh_auth_method === 'password' ? '密码' : '密钥'}</Tag>
                  </Descriptions.Item>
                  <Descriptions.Item label="操作系统">{host.os_type?.toUpperCase()}</Descriptions.Item>
                  <Descriptions.Item label="状态">
                    <Tag color={host.status === 'success' ? 'success' : host.status === 'failed' ? 'error' : 'default'}>
                      {host.status === 'success' ? '可用' : host.status === 'failed' ? '不可用' : '未检测'}
                    </Tag>
                  </Descriptions.Item>
                  <Descriptions.Item label="标签">{host.tags || '-'}</Descriptions.Item>
                  <Descriptions.Item label="最后检测">
                    {host.last_check_at ? new Date(host.last_check_at).toLocaleString() : '-'}
                  </Descriptions.Item>
                  <Descriptions.Item label="描述" span={2}>
                    {host.description || '-'}
                  </Descriptions.Item>
                  <Descriptions.Item label="创建时间">
                    {new Date(host.created_at).toLocaleString()}
                  </Descriptions.Item>
                  <Descriptions.Item label="更新时间">
                    {new Date(host.updated_at).toLocaleString()}
                  </Descriptions.Item>
                </Descriptions>
              ),
            },
            {
              key: 'instances',
              label: `本机实例 (${instances.length})`,
              children: (
                <div>
                  <Row gutter={16} style={{ marginBottom: 16 }}>
                    <Col span={6}>
                      <Card size="small">
                        <Statistic
                          title="已纳管实例"
                          value={instances.length}
                          valueStyle={{ color: instances.length > 0 ? '#3f8600' : '#999' }}
                          prefix={<DatabaseOutlined />}
                        />
                      </Card>
                    </Col>
                    <Col span={6}>
                      <Card size="small">
                        <Statistic
                          title="扫描发现"
                          value={scanResult?.instances?.length || 0}
                          prefix={<ScanOutlined />}
                        />
                      </Card>
                    </Col>
                    <Col span={6}>
                      <Card size="small">
                        <Statistic
                          title="待纳管"
                          value={newInstances.length}
                          valueStyle={{ color: newInstances.length > 0 ? '#fa8c16' : '#999' }}
                        />
                      </Card>
                    </Col>
                    <Col span={6}>
                      <Card size="small">
                        <Statistic
                          title="已纳管(扫描中)"
                          value={managedInScan.length}
                          valueStyle={{ color: '#1890ff' }}
                        />
                      </Card>
                    </Col>
                  </Row>

                  {scanError && (
                    <Alert
                      type="warning"
                      showIcon
                      style={{ marginBottom: 16 }}
                      message="扫描功能不可用"
                      description={`${scanError}。您仍可使用下方"添加实例"按钮手动录入。`}
                    />
                  )}

                  {scanning && (
                    <Alert
                      type="info"
                      showIcon
                      style={{ marginBottom: 16 }}
                      message="正在通过 SSH 扫描该主机上的 MySQL 实例..."
                      description="扫描过程会检查监听端口、读取 my.cnf、获取版本信息。"
                    />
                  )}

                  {scanResult && scanResult.instances && scanResult.instances.length > 0 && (
                    <Card
                      type="inner"
                      title={
                        <Space>
                          <ScanOutlined />
                          <span>扫描结果 ({scanResult.instances.length})</span>
                        </Space>
                      }
                      style={{ marginBottom: 16 }}
                      extra={
                        <Button
                          size="small"
                          type="primary"
                          icon={<ImportOutlined />}
                          disabled={newInstances.length === 0}
                          onClick={openBatchRegister}
                        >
                          一键纳管下一个
                        </Button>
                      }
                    >
                      <Table
                        columns={scannedColumns}
                        dataSource={scanResult.instances}
                        rowKey="port"
                        size="small"
                        pagination={false}
                      />
                    </Card>
                  )}

                  <Card
                    type="inner"
                    title={
                      <Space>
                        <DatabaseOutlined />
                        <span>已纳管实例</span>
                      </Space>
                    }
                    extra={
                      <Button
                        type="primary"
                        icon={<PlusOutlined />}
                        onClick={() => navigate(`/dashboard/instances?preset_host=${host.id}`)}
                      >
                        添加实例
                      </Button>
                    }
                  >
                    {instances.length === 0 ? (
                      <Result
                        icon={<DatabaseOutlined />}
                        title="该主机暂无已纳管实例"
                        subTitle="可以点击下方按钮自动扫描该主机, 或手动添加实例"
                        extra={
                          <Space>
                            <Button
                              type="primary"
                              icon={<ScanOutlined />}
                              onClick={handleStartScan}
                              loading={scanning}
                            >
                              自动扫描该主机
                            </Button>
                            <Button
                              icon={<PlusOutlined />}
                              onClick={() => navigate(`/dashboard/instances?preset_host=${host.id}`)}
                            >
                              手动添加实例
                            </Button>
                          </Space>
                        }
                      />
                    ) : (
                      <Table
                        columns={instanceColumns}
                        dataSource={instances}
                        rowKey="id"
                        loading={instLoading}
                        pagination={{ pageSize: 10 }}
                      />
                    )}
                  </Card>
                </div>
              ),
            },
          ]}
        />
      </Card>

      <Modal
        title="一键纳管全部待纳管实例"
        open={batchRegisterOpen}
        onCancel={() => setBatchRegisterOpen(false)}
        onOk={submitBatchRegister}
        confirmLoading={batchRegistering}
        okText="全部纳管"
        cancelText="取消"
        width={560}
      >
        <Alert
          type="info"
          showIcon
          style={{ marginBottom: 12 }}
          message={`将纳管 ${newInstances.length} 个扫描发现的实例`}
          description="这些实例会使用同一组 MySQL 连接账号和密码登记到平台。已纳管端口会自动跳过。"
        />
        <Form form={batchRegisterForm} layout="vertical">
          <Form.Item name="username" label="连接用户名" rules={[{ required: true, message: '请输入连接用户名' }]}>
            <Input placeholder="例如: root" />
          </Form.Item>
          <Form.Item name="password" label="连接密码" rules={[{ required: true, message: '请输入密码' }]}>
            <Input.Password placeholder="MySQL 密码" autoComplete="new-password" />
          </Form.Item>
          <Form.Item name="cluster_id" label="所属集群">
            <Select
              allowClear
              placeholder="选择集群（可选）"
              options={clusters.map((c: any) => ({ value: c.cluster_id, label: `${c.cluster_id} (${c.arch || '未知架构'}) - ${c.node_count || 0}节点` }))}
            />
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        title={`配置扫描: ${host?.name || ''}`}
        open={scanConfigOpen}
        onCancel={() => setScanConfigOpen(false)}
        onOk={submitScan}
        okText="开始扫描"
        cancelText="取消"
        width={560}
      >
        <Alert
          type="info"
          showIcon
          style={{ marginBottom: 12 }}
          message="扫描说明"
          description="平台会并发 TCP 探测你指定的端口, 尝试读取 MySQL 握手包以获取版本/类型。开启进程发现后，还会通过 SSH 查询 mysqld 进程信息（需要主机已配置 SSH 凭据）。"
        />
        <Form form={scanForm} layout="vertical">
          <Form.Item label="扫描方式" name="mode">
            <Radio.Group
              value={scanMode}
              onChange={(e) => setScanMode(e.target.value)}
            >
              <Radio.Button value="default">常用端口</Radio.Button>
              <Radio.Button value="custom">自定义端口</Radio.Button>
              <Radio.Button value="range">端口范围</Radio.Button>
            </Radio.Group>
          </Form.Item>

          {scanMode === 'default' && (
            <div style={{ marginBottom: 12, padding: 8, background: '#f0f0f0', borderRadius: 4, fontSize: 13 }}>
              将扫描 3300-3400, 33060, 33061 等常见 MySQL 端口
            </div>
          )}

          {scanMode === 'custom' && (
            <Form.Item label="自定义端口" extra="例如: 3306, 3307, 3308 (用英文逗号分隔)">
              <Input
                placeholder="3306, 3307, 3308"
                value={scanPorts.join(', ')}
                onChange={(e) => {
                  const arr = e.target.value
                    .split(',')
                    .map((s) => parseInt(s.trim(), 10))
                    .filter((n) => Number.isFinite(n) && n > 0 && n <= 65535)
                  setScanPorts(arr)
                }}
              />
            </Form.Item>
          )}

          {scanMode === 'range' && (
            <Form.Item
              label="端口范围"
              name="port_range"
              extra="支持单范围如 3306-3310, 也可混用逗号如 3306, 13306-13308"
            >
              <Input
                placeholder="3306-3310"
                value={scanRange}
                onChange={(e) => setScanRange(e.target.value)}
              />
            </Form.Item>
          )}

          <Form.Item label="进程发现" extra="通过 SSH 查询主机上的 mysqld 进程, 可发现非标准端口的实例并获取 PID/内存/数据目录等信息">
            <Switch checked={discoverProcess} onChange={setDiscoverProcess} checkedChildren="开启" unCheckedChildren="关闭" />
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        title={`纳管扫描到的实例: ${registerTarget?.port || ''}`}
        open={registerOpen}
        onCancel={() => setRegisterOpen(false)}
        onOk={submitRegister}
        confirmLoading={registering}
        okText="纳管"
        cancelText="取消"
        width={560}
      >
        <Form form={registerForm} layout="vertical">
          <Form.Item name="name" label="实例名称" rules={[{ required: true, message: '请输入实例名称' }]}>
            <Input />
          </Form.Item>
          <Form.Item
            name="username"
            label="连接用户名"
            rules={[{ required: true, message: '请输入连接用户名' }]}
            extra="默认 root, 也可使用具有 SUPER/REPLICATION 权限的运维账号"
          >
            <Input placeholder="例如: root" />
          </Form.Item>
          <Form.Item
            name="password"
            label="连接密码"
            rules={[{ required: true, message: '请输入密码' }]}
          >
            <Input.Password placeholder="MySQL 密码" autoComplete="new-password" />
          </Form.Item>
          <Form.Item name="cluster_id" label="所属集群">
            <Select
              allowClear
              placeholder="选择集群（可选）"
              options={clusters.map((c: any) => ({ value: c.cluster_id, label: `${c.cluster_id} (${c.arch || '未知架构'}) - ${c.node_count || 0}节点` }))}
            />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}

export default HostDetail
