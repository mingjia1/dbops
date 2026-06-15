import React, { useEffect, useRef, useState } from 'react'
import {
  Button, Card, Checkbox, Col, Descriptions, Empty, Form, Input, InputNumber, message, Modal, Popover, Progress, Row, Select, Space, Steps, Table, Tabs, Tag,
} from 'antd'
import { CheckCircleOutlined, CloseCircleOutlined, ClusterOutlined, DeleteOutlined, EyeOutlined, KeyOutlined, PlayCircleOutlined, ReloadOutlined } from '@ant-design/icons'
import type { ColumnsType } from 'antd/es/table'
import { clusterDeployApi, hostApi, instanceApi, type Host, type Instance } from '../services/api'

type ArchType = 'ha' | 'mha' | 'mgr' | 'pxc'

const DEFAULT_MYSQL_CREDENTIAL = {
  username: 'root',
  password: 'Hcfc@DboOps#2024_80',
}

const DEFAULT_CREDENTIAL_ACK_KEY = 'dbops.clusterDeploy.defaultMysqlCredentialAck'
const STAGE_ORDER = ['环境检查', '安装二进制', '配置集群', '启动节点', '集群验证']

const DEPLOY_SUBSTEPS: Record<string, string[]> = {
  '环境检查': ['检查主机连通性', '验证端口可用性', '检查磁盘空间', '检查系统依赖'],
  '安装二进制': ['下载 MySQL 安装包', '解压安装包', '创建数据目录', '设置文件权限'],
  '配置集群': ['生成 my.cnf', '配置复制用户', '配置复制参数', '写入集群拓扑'],
  '启动节点': ['启动 MySQL 服务', '等待端口就绪', '执行健康检查', '验证服务状态'],
  '集群验证': ['检查复制延迟', '验证 GTID 一致性', '执行数据校验', '生成部署报告'],
}

interface DeployNodeProgress {
  instance_id?: string
  name?: string
  host?: string
  port?: number
  role?: string
  status?: string
  current_step?: string
  progress?: number
  message?: string
}

interface DeployResult {
  deployment_id: string
  cluster_id: string
  cluster_type: ArchType
  status: 'pending' | 'running' | 'success' | 'completed' | 'succeeded' | 'ok' | 'failed' | 'error' | 'timeout' | 'cancelled' | 'canceled' | 'partial' | 'partial_success' | 'destroyed' | string
  stage?: string
  progress: number
  message: string
  started_at: string
  finished_at?: string
  nodes?: DeployNodeProgress[]
  steps?: Array<{ name: string; status: string; message?: string; started_at?: string; completed_at?: string }>
  logs?: string[]
}

const normalizeStatus = (status?: string) => (status || '').trim().toLowerCase()

const getStatusCategory = (status?: string) => {
  const norm = normalizeStatus(status)
  if (['success', 'completed', 'succeeded', 'ok'].includes(norm)) return 'success'
  if (['failed', 'error', 'timeout', 'cancelled', 'canceled'].includes(norm)) return 'failed'
  if (['partial', 'partial_success'].includes(norm)) return 'partial'
  if (norm === 'destroyed') return 'destroyed'
  if (norm === 'running') return 'running'
  if (norm === 'pending') return 'pending'
  return norm || 'unknown'
}

const isCompletedDeployStatus = (status?: string) => getStatusCategory(status) === 'success'

const isFailedDeployStatus = (status?: string) => getStatusCategory(status) === 'failed'

const isPartialDeployStatus = (status?: string) => getStatusCategory(status) === 'partial'

const isDestroyedDeployStatus = (status?: string) => getStatusCategory(status) === 'destroyed'

const isTerminalDeployStatus = (status?: string) =>
  isCompletedDeployStatus(status) || isFailedDeployStatus(status) || isPartialDeployStatus(status) || isDestroyedDeployStatus(status)

const deploymentProgress = (status?: string, progress?: number) => {
  if (typeof progress === 'number') return progress
  return isTerminalDeployStatus(status) ? 100 : 0
}

const deploymentProgressStatus = (status?: string) => {
  if (isFailedDeployStatus(status)) return 'exception'
  if (isCompletedDeployStatus(status) || isDestroyedDeployStatus(status)) return 'success'
  if (isPartialDeployStatus(status)) return 'normal'
  return 'active'
}

const deploymentStepStatus = (status?: string) => {
  if (isFailedDeployStatus(status)) return 'error'
  if (isCompletedDeployStatus(status) || isDestroyedDeployStatus(status)) return 'finish'
  if (isPartialDeployStatus(status)) return 'error'
  return 'process'
}

const showDeploymentResultMessage = (dep: DeployResult) => {
  const arch = dep.cluster_type?.toUpperCase?.() || 'Cluster'
  if (isCompletedDeployStatus(dep.status)) message.success(`${arch} 集群部署完成`)
  else if (isPartialDeployStatus(dep.status)) message.warning(`${arch} 集群部署部分完成: ${dep.message || dep.status}`)
  else if (isFailedDeployStatus(dep.status)) message.error(`${arch} 集群部署失败: ${dep.message || dep.status}`)
}

const ClusterDeploy: React.FC = () => {
  const [hosts, setHosts] = useState<Host[]>([])
  const [instances, setInstances] = useState<Instance[]>([])
  const [tab, setTab] = useState<ArchType>('ha')
  const [submitting, setSubmitting] = useState(false)
  const [historyLoading, setHistoryLoading] = useState(false)
  const [deployments, setDeployments] = useState<DeployResult[]>([])
  const [statusFilter, setStatusFilter] = useState<string[]>(['success'])
  const [archFilter, setArchFilter] = useState<ArchType | 'all'>('all')
  const [showHistory, setShowHistory] = useState(false)
  const [activeDeployment, setActiveDeployment] = useState<DeployResult | null>(null)
  const [currentStep, setCurrentStep] = useState(0)
  const [credential, setCredential] = useState(DEFAULT_MYSQL_CREDENTIAL)
  const [credentialModalOpen, setCredentialModalOpen] = useState(false)
  const [showDefaultCredential, setShowDefaultCredential] = useState(false)
  const [oneTimeCredential, setOneTimeCredential] = useState<typeof DEFAULT_MYSQL_CREDENTIAL | null>(null)
  const pollRef = useRef<number | null>(null)

  const [haForm] = Form.useForm()
  const [mhaForm] = Form.useForm()
  const [mgrForm] = Form.useForm()
  const [pxcForm] = Form.useForm()
  const [credentialForm] = Form.useForm()

  useEffect(() => {
    hostApi.list(100, 0).then((res: any) => setHosts(res?.data || [])).catch(() => {})
    instanceApi.list(1000, 0).then((res: any) => setInstances(res?.data || [])).catch(() => {})
    loadDeployments()
    setShowDefaultCredential(localStorage.getItem(DEFAULT_CREDENTIAL_ACK_KEY) !== '1')
  }, [])

  useEffect(() => () => {
    if (pollRef.current) window.clearInterval(pollRef.current)
  }, [])

  const hostOptions = hosts.map((h) => ({ value: h.id, label: `${h.name} (${h.address})` }))

  const normalizeDeployment = (data: any): DeployResult => ({
    deployment_id: data.deployment_id || data.id,
    cluster_id: data.cluster_id || data.deployment_id || data.id,
    cluster_type: data.cluster_type,
    status: data.status || 'pending',
    progress: deploymentProgress(data.status, data.progress),
    stage: data.stage,
    message: data.message || '',
    started_at: data.started_at || data.created_at,
    finished_at: data.finished_at || data.updated_at,
    nodes: Array.isArray(data.nodes) ? data.nodes : [],
    steps: Array.isArray(data.steps) ? data.steps : [],
    logs: Array.isArray(data.logs) ? data.logs : [],
  })

  const loadDeployments = async () => {
    setHistoryLoading(true)
    try {
      const res: any = await clusterDeployApi.list(1000, 0)
      const allData = (Array.isArray(res?.data) ? res.data : []).map(normalizeDeployment)
      setDeployments(allData)
    } catch {
      setDeployments([])
    } finally {
      setHistoryLoading(false)
    }
  }

  useEffect(() => {
    loadDeployments()
  }, [])

  const deploymentNodes = (record: DeployResult) => {
    if (record.nodes?.length) {
      return record.nodes.map((node) => {
        const endpoint = `${node.host || '-'}:${node.port || '-'}`
        return `${node.name || node.instance_id || '-'} (${endpoint}, ${node.role || '-'})`
      })
    }
    const clusterID = record.cluster_id || record.deployment_id
    return instances
      .filter((inst) => inst.cluster_id === clusterID)
      .map((inst) => {
        const endpoint = `${inst.connection?.host || inst.host || '-'}:${inst.connection?.port || inst.port || '-'}`
        const role = inst.status?.role || '-'
        return `${inst.name} (${endpoint}, ${role})`
      })
  }

  const openCredentialModal = () => {
    credentialForm.setFieldsValue({ username: credential.username, password: '', confirm_password: '' })
    setCredentialModalOpen(true)
  }

  const acknowledgeDefaultCredential = () => {
    localStorage.setItem(DEFAULT_CREDENTIAL_ACK_KEY, '1')
    setShowDefaultCredential(false)
  }

  const submitCredentialChange = async () => {
    const values = await credentialForm.validateFields()
    Modal.confirm({
      title: '确认修改默认 MySQL 实例凭据?',
      content: (
        <div>
          <p>修改后新密码只会在页面上显示一次，请确认已准备好记录。</p>
          <Descriptions size="small" column={1} bordered>
            <Descriptions.Item label="用户名">{values.username}</Descriptions.Item>
            <Descriptions.Item label="新密码">{values.password}</Descriptions.Item>
          </Descriptions>
        </div>
      ),
      okText: '确认修改',
      cancelText: '取消',
      onOk: () => {
        const next = { username: values.username, password: values.password }
        setCredential(next)
        setOneTimeCredential(next)
        setShowDefaultCredential(false)
        localStorage.setItem(DEFAULT_CREDENTIAL_ACK_KEY, '1')
        setCredentialModalOpen(false)
        message.success('默认 MySQL 实例凭据已更新')
      },
    })
  }

  const stopPolling = () => {
    if (pollRef.current) {
      window.clearInterval(pollRef.current)
      pollRef.current = null
    }
  }

  const patchDeployment = (dep: DeployResult) => {
    setDeployments((items) => items.map((item) => (item.deployment_id === dep.deployment_id ? dep : item)))
    setActiveDeployment((cur) => (cur && cur.deployment_id === dep.deployment_id ? dep : cur))
  }

  const startPolling = (dep: DeployResult) => {
    if (isTerminalDeployStatus(dep.status)) return
    stopPolling()
    let attempts = 0
    pollRef.current = window.setInterval(async () => {
      attempts += 1
      try {
        const res: any = await clusterDeployApi.getStatus(dep.deployment_id)
        const data = res?.data
        if (!data) return
        const next: DeployResult = {
          ...dep,
          status: data.status || dep.status,
          stage: data.stage,
          progress: typeof data.progress === 'number' ? data.progress : dep.progress,
          message: data.message || dep.message,
          finished_at: data.finished_at,
          nodes: Array.isArray(data.nodes) ? data.nodes : dep.nodes,
          steps: Array.isArray(data.steps) ? data.steps : dep.steps,
          logs: Array.isArray(data.logs) ? data.logs : dep.logs,
        }
        patchDeployment(next)
        if (isTerminalDeployStatus(next.status)) loadDeployments()
        const stepIdx = next.stage ? STAGE_ORDER.indexOf(next.stage) : -1
        if (stepIdx >= 0) setCurrentStep(stepIdx)
        if (isTerminalDeployStatus(next.status) || attempts > 600) {
          stopPolling()
          showDeploymentResultMessage(next)
        }
      } catch {
        // Polling is retried until deployment reaches a terminal state.
      }
    }, 2000)
  }

  const runDeploy = (arch: ArchType, values: any, apiCall: (data: any) => Promise<any>) => {
    Modal.confirm({
      title: `确认启动 ${arch.toUpperCase()} 集群部署?`,
      content: values.pseudo_mode
        ? '伪集群演练只写入平台纳管关系和拓扑，不会停止或删除目标主机上的数据库服务。'
        : '真实部署会修改目标主机上的 MySQL 实例、复制配置和服务状态。请确认已完成环境检查并具备回滚方案。',
      okText: '确认部署',
      cancelText: '取消',
      onOk: () => doDeploy(arch, values, apiCall),
    })
  }

  const doDeploy = async (arch: ArchType, values: any, apiCall: (data: any) => Promise<any>) => {
    setSubmitting(true)
    setCurrentStep(0)
    setActiveDeployment(null)
    try {
      const res: any = await apiCall(buildDeployPayload(arch, values))
      if (!res?.data?.deployment_id && !res?.data?.id) {
        throw new Error('backend did not return deployment_id')
      }
      const status = res?.data?.status || 'running'
      const dep: DeployResult = {
        deployment_id: res?.data?.deployment_id || res?.data?.id,
        cluster_id: res?.data?.cluster_id || values.cluster_id || res?.data?.deployment_id || res?.data?.id,
        cluster_type: res?.data?.cluster_type || arch,
        status,
        progress: deploymentProgress(status),
        stage: isCompletedDeployStatus(status) ? STAGE_ORDER[4] : STAGE_ORDER[0],
        message: res?.data?.message || '部署已提交，等待后端开始执行',
        started_at: new Date().toISOString(),
      }
      setActiveDeployment(dep)
      await loadDeployments()
      if (isTerminalDeployStatus(dep.status)) {
        showDeploymentResultMessage(dep)
      } else {
        message.success(`${arch.toUpperCase()} 集群部署任务已提交`)
      }
      startPolling(dep)
    } catch (err: any) {
      message.error(`提交部署失败: ${err?.response?.data?.message || err?.message}`)
    } finally {
      setSubmitting(false)
    }
  }

  const destroyDeployment = (record: DeployResult) => {
    Modal.confirm({
      title: `销毁集群 ${record.deployment_id}?`,
      content: (
        <div>
          <p><strong>销毁流程：</strong></p>
          <ol style={{ paddingLeft: 20 }}>
            <li>对所有实例执行完整备份</li>
            <li>验证备份文件（路径、大小、校验和）</li>
            <li>清理远程主机上的数据库目录和服务</li>
            <li>删除平台纳管关系和拓扑</li>
          </ol>
          <p style={{ color: '#ff4d4f', marginTop: 8 }}>⚠️ 此操作会删除数据库服务和数据目录，无法撤销</p>
          <p style={{ color: '#ff4d4f' }}>⚠️ 如果备份失败，销毁操作将被拒绝</p>
        </div>
      ),
      okText: '确认销毁',
      okButtonProps: { danger: true },
      cancelText: '取消',
      onOk: async () => {
        try {
          const res: any = await clusterDeployApi.destroy(record.deployment_id)
          const next: DeployResult = {
            ...record,
            status: 'destroyed',
            progress: 100,
            message: res?.data?.message || '集群已销毁',
            finished_at: new Date().toISOString(),
          }
          patchDeployment(next)
          await loadDeployments()
          message.success('集群销毁成功：已完成备份验证和远程清理')
        } catch (err: any) {
          await loadDeployments()
          message.error(`销毁失败: ${err?.response?.data?.message || err?.message || '未知错误'}`)
        }
      },
    })
  }

  const buildDeployPayload = (arch: ArchType, values: any) => {
    const base = {
      cluster_id: values.cluster_id,
      name: values.cluster_id,
      repl_user: values.repl_user,
      repl_password: values.repl_password,
      mysql_user: credential.username,
      mysql_password: credential.password,
      pseudo_mode: !!values.pseudo_mode,
    }
    if (arch === 'ha') {
      return {
        ...base,
        master_host_id: values.master_host_id,
        replica_host_id: values.replica_host_id,
        replica_host_ids: values.replica_host_ids || (values.replica_host_id ? [values.replica_host_id] : []),
        master_port: values.mysql_port || 3306,
        replica_port: values.replica_port || 3307,
      }
    }
    if (arch === 'mha') {
      return {
        ...base,
        master_host_id: values.master_host_id,
        manager_host_id: values.manager_host_id,
        replica_host_ids: values.replica_host_ids || [],
        master_port: values.mysql_port || 3306,
        replica_port: values.replica_port || values.mysql_port || 3306,
        vip: values.vip || '',
      }
    }
    if (arch === 'mgr') {
      return {
        ...base,
        name: values.group_name || values.cluster_id,
        master_host_id: values.master_host_id,
        replica_host_ids: values.replica_host_ids || [],
        primary_port: values.mysql_port || 3306,
        replica_port: values.replica_port || values.mysql_port || 3306,
        group_mode: 'single-primary',
      }
    }
    return {
      ...base,
      master_host_id: values.master_host_id,
      replica_host_ids: values.replica_host_ids || [],
      bootstrap_node: { host: '', port: values.mysql_port || 3306 },
      other_nodes: values.replica_host_ids?.length ? [{ host: '', port: values.replica_port || values.mysql_port || 3306 }] : [],
      wsrep_port: values.wsrep_port || 4567,
    }
  }

  const columns: ColumnsType<DeployResult> = [
    { title: '部署ID', dataIndex: 'deployment_id', key: 'deployment_id', width: 180, ellipsis: true },
    { title: '集群ID', dataIndex: 'cluster_id', key: 'cluster_id', width: 150, ellipsis: true },
    {
      title: '架构',
      dataIndex: 'cluster_type',
      key: 'cluster_type',
      width: 80,
      render: (type: ArchType) => <Tag color={type === 'ha' ? 'cyan' : type === 'mha' ? 'blue' : type === 'mgr' ? 'green' : 'orange'}>{type.toUpperCase()}</Tag>,
    },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      width: 110,
      render: (status: string) => {
        if (isCompletedDeployStatus(status)) return <Tag color="success" icon={<CheckCircleOutlined />}>成功</Tag>
        if (isDestroyedDeployStatus(status)) return <Tag color="default">已销毁</Tag>
        if (isPartialDeployStatus(status)) return <Tag color="warning" icon={<CloseCircleOutlined />}>部分完成</Tag>
        if (isFailedDeployStatus(status)) return <Tag color="error" icon={<CloseCircleOutlined />}>失败</Tag>
        if (normalizeStatus(status) === 'pending') return <Tag color="default">待开始</Tag>
        return <Tag color="processing" icon={<ReloadOutlined spin />}>进行中</Tag>
      },
    },
    { title: '当前阶段', dataIndex: 'stage', key: 'stage', width: 100, ellipsis: true, render: (stage: string) => stage || '-' },
    {
      title: '进度',
      dataIndex: 'progress',
      key: 'progress',
      width: 140,
      render: (progress: number, record) => <Progress percent={deploymentProgress(record.status, progress)} size="small" status={deploymentProgressStatus(record.status)} />,
    },
    { title: '信息', dataIndex: 'message', key: 'message', ellipsis: true },
    {
      title: '节点信息',
      key: 'nodes',
      width: 200,
      ellipsis: true,
      render: (_, record) => {
        const nodes = deploymentNodes(record)
        if (nodes.length === 0) return '-'
        return (
          <Space direction="vertical" size={2}>
            {nodes.map((node) => <span key={node}>{node}</span>)}
          </Space>
        )
      },
    },
    { title: '开始时间', dataIndex: 'started_at', key: 'started_at', width: 160, render: (time: string) => (time ? new Date(time).toLocaleString() : '-') },
    {
      title: '操作',
      key: 'action',
      width: 80,
      render: (_, record) => (
        <Button size="small" danger icon={<DeleteOutlined />} disabled={isDestroyedDeployStatus(record.status)} onClick={() => destroyDeployment(record)}>
          销毁
        </Button>
      ),
    },
  ]

  const renderForm = (
    arch: ArchType,
    form: any,
    extraFields: React.ReactNode,
    onFinish: (values: any) => void,
    options?: { simpleReplica?: boolean },
  ) => (
    <Form form={form} layout="horizontal" labelCol={{ span: 4 }} wrapperCol={{ span: 20 }} onFinish={onFinish}>
      <Row gutter={16}>
        <Col span={12}>
          <Form.Item name="cluster_id" label="集群ID" rules={[{ required: true, message: '请输入集群ID' }]}>
            <Input placeholder={`${arch}-cluster-01`} />
          </Form.Item>
        </Col>
        <Col span={12}>
          <Form.Item name="pseudo_mode" label="演练模式" valuePropName="checked" initialValue={false}>
            <Checkbox>伪集群演练</Checkbox>
          </Form.Item>
        </Col>
      </Row>
      <Row gutter={16}>
        <Col span={12}>
          <Form.Item name="master_host_id" label="主节点" rules={[{ required: true }]}>
            <Select options={hostOptions} placeholder="选择主节点主机" />
          </Form.Item>
        </Col>
        <Col span={12}>
          <Form.Item name="mysql_port" label="主端口" initialValue={3309}>
            <InputNumber min={1} max={65535} style={{ width: '100%' }} />
          </Form.Item>
        </Col>
      </Row>
      {!options?.simpleReplica && (
        <Row gutter={16}>
          <Col span={12}>
            <Form.Item name="replica_host_ids" label="从节点" rules={[{ required: true }]}>
              <Select mode="multiple" options={hostOptions} placeholder="至少选择1个从节点" maxTagCount={2} />
            </Form.Item>
          </Col>
          <Col span={12}>{extraFields}</Col>
        </Row>
      )}
      {options?.simpleReplica && (
        <Row gutter={16}>
          <Col span={12}>{extraFields}</Col>
        </Row>
      )}
      <Row gutter={16}>
        <Col span={12}>
          <Form.Item name="repl_user" label="复制用户" rules={[{ required: true }]} initialValue="repl_user">
            <Input />
          </Form.Item>
        </Col>
        <Col span={12}>
          <Form.Item name="repl_password" label="复制密码" rules={[{ required: true }]} initialValue="ReplPass#2026">
            <Input.Password />
          </Form.Item>
        </Col>
      </Row>
      <Row gutter={16}>
        <Col span={12}>
          <Form.Item name="vip" label="VIP">
            <Input placeholder="可选" />
          </Form.Item>
        </Col>
      </Row>
      <Form.Item wrapperCol={{ offset: 4 }}>
        <Button type="primary" icon={<PlayCircleOutlined />} htmlType="submit" loading={submitting}>
          启动部署
        </Button>
      </Form.Item>
    </Form>
  )

  const filteredDeployments = deployments.filter((d) => {
    const statusMatch = statusFilter.length === 0 || statusFilter.includes(getStatusCategory(d.status))
    const archMatch = archFilter === 'all' || d.cluster_type === archFilter
    return statusMatch && archMatch
  })

  return (
    <div style={{ padding: '24px' }}>
      <Card
        title={
          <Space>
            <ClusterOutlined />
            <span>集群部署</span>
          </Space>
        }
        extra={
          <Space>
            <span style={{ fontSize: 13, color: '#666' }}>默认 MySQL 账号：</span>
            <Popover
              content={
                <div style={{ minWidth: 280 }}>
                  {showDefaultCredential && (
                    <Space direction="vertical" size={8}>
                      <Descriptions size="small" column={1} bordered>
                        <Descriptions.Item label="用户名">{credential.username}</Descriptions.Item>
                        <Descriptions.Item label="密码">{credential.password}</Descriptions.Item>
                      </Descriptions>
                      <Button size="small" type="primary" onClick={acknowledgeDefaultCredential}>我已保存，隐藏默认密码</Button>
                    </Space>
                  )}
                  {oneTimeCredential && (
                    <Space direction="vertical" size={8}>
                      <Descriptions size="small" column={1} bordered>
                        <Descriptions.Item label="用户名">{oneTimeCredential.username}</Descriptions.Item>
                        <Descriptions.Item label="密码">{oneTimeCredential.password}</Descriptions.Item>
                      </Descriptions>
                      <Button size="small" danger onClick={() => setOneTimeCredential(null)}>我已保存，立即隐藏</Button>
                    </Space>
                  )}
                  {!showDefaultCredential && !oneTimeCredential && (
                    <Descriptions size="small" column={1}>
                      <Descriptions.Item label="用户名">{credential.username}</Descriptions.Item>
                      <Descriptions.Item label="密码">••••••••</Descriptions.Item>
                    </Descriptions>
                  )}
                </div>
              }
              trigger="click"
            >
              <Button size="small" icon={<EyeOutlined />} type={showDefaultCredential ? 'primary' : 'default'}>查看</Button>
            </Popover>
            <Button size="small" icon={<KeyOutlined />} onClick={openCredentialModal}>修改</Button>
            {!showDefaultCredential && !oneTimeCredential && (
              <Tag icon={<CheckCircleOutlined />} color="success">已确认</Tag>
            )}
            <Button type="primary" icon={<ClusterOutlined />} onClick={() => setShowHistory(!showHistory)}>
              {showHistory ? '返回部署' : '部署历史'}
            </Button>
          </Space>
        }
      >
        {showHistory ? (
          <div>
            <Space style={{ marginBottom: 16 }}>
              <Select
                mode="multiple"
                placeholder="筛选状态"
                value={statusFilter}
                onChange={setStatusFilter}
                style={{ minWidth: 200 }}
                maxTagCount="responsive"
                options={[
                  { label: '成功', value: 'success' },
                  { label: '失败', value: 'failed' },
                  { label: '部分完成', value: 'partial' },
                  { label: '运行中', value: 'running' },
                  { label: '待开始', value: 'pending' },
                  { label: '已销毁', value: 'destroyed' },
                ]}
              />
              <Select
                placeholder="筛选架构"
                value={archFilter}
                onChange={setArchFilter}
                style={{ width: 120 }}
                options={[
                  { label: '全部架构', value: 'all' },
                  { label: 'HA', value: 'ha' },
                  { label: 'MHA', value: 'mha' },
                  { label: 'MGR', value: 'mgr' },
                  { label: 'PXC', value: 'pxc' },
                ]}
              />
            </Space>
            {filteredDeployments.length === 0 ? (
              <Empty description="暂无符合条件的部署记录" />
            ) : (
              <Table
                columns={columns}
                dataSource={filteredDeployments}
                rowKey="deployment_id"
                loading={historyLoading}
                scroll={{ x: 'max-content' }}
                pagination={{
                  defaultPageSize: 10,
                  showSizeChanger: true,
                  showQuickJumper: true,
                  showTotal: (t) => `共 ${t} 条记录`,
                }}
              />
            )}
          </div>
        ) : (
          <Tabs
            activeKey={tab}
            onChange={(key) => setTab(key as ArchType)}
            items={[
              {
                key: 'ha',
                label: 'HA 主从',
                children: renderForm('ha', haForm,
                  <>
                    <Form.Item name="replica_host_id" label="从节点" rules={[{ required: true }]}>
                      <Select options={hostOptions} placeholder="选择从节点主机" />
                    </Form.Item>
                    <Form.Item name="replica_port" label="从端口" initialValue={3310}>
                      <InputNumber min={1} max={65535} style={{ width: '100%' }} />
                    </Form.Item>
                  </>,
                  (values) => runDeploy('ha', values, clusterDeployApi.deployHA),
                  { simpleReplica: true },
                ),
              },
              {
                key: 'mha',
                label: 'MHA 部署',
                children: renderForm('mha', mhaForm,
                  <Form.Item name="manager_host_id" label="Manager" rules={[{ required: true }]}>
                    <Select options={hostOptions} placeholder="选择 Manager 主机" />
                  </Form.Item>,
                  (values) => runDeploy('mha', values, clusterDeployApi.deployMHA),
                ),
              },
              {
                key: 'mgr',
                label: 'MGR 部署',
                children: renderForm('mgr', mgrForm,
                  <Form.Item name="group_name" label="Group Name" initialValue="aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa">
                    <Input />
                  </Form.Item>,
                  (values) => runDeploy('mgr', values, clusterDeployApi.deployMGR),
                ),
              },
              {
                key: 'pxc',
                label: 'PXC 部署',
                children: renderForm('pxc', pxcForm,
                  <Form.Item name="wsrep_port" label="wsrep 端口" initialValue={4567}>
                    <InputNumber min={1} max={65535} style={{ width: '100%' }} />
                  </Form.Item>,
                  (values) => runDeploy('pxc', values, clusterDeployApi.deployPXC),
                ),
              },
            ]}
          />
        )}
      </Card>

      <Modal
        title="修改默认 MySQL 实例账号"
        open={credentialModalOpen}
        onCancel={() => setCredentialModalOpen(false)}
        onOk={submitCredentialChange}
        okText="下一步确认"
        cancelText="取消"
        destroyOnClose
      >
        <p>保存后部署任务会使用新凭据创建 MySQL 实例。新密码确认后只显示一次。</p>
        <Form form={credentialForm} layout="vertical">
          <Form.Item name="username" label="用户名" rules={[{ required: true, message: '请输入用户名' }]}>
            <Input placeholder="例如: root" autoComplete="off" />
          </Form.Item>
          <Form.Item name="password" label="新密码" rules={[{ required: true, min: 8, message: '请输入至少 8 位密码' }]}>
            <Input.Password autoComplete="new-password" />
          </Form.Item>
          <Form.Item
            name="confirm_password"
            label="确认新密码"
            dependencies={['password']}
            rules={[
              { required: true, message: '请再次输入新密码' },
              ({ getFieldValue }) => ({
                validator(_, value) {
                  if (!value || getFieldValue('password') === value) return Promise.resolve()
                  return Promise.reject(new Error('两次输入的密码不一致'))
                },
              }),
            ]}
          >
            <Input.Password autoComplete="new-password" />
          </Form.Item>
        </Form>
      </Modal>

      {activeDeployment && (
        <Card
          title="部署进度"
          style={{ marginTop: 16 }}
          extra={
            <Space>
              <Tag color={isCompletedDeployStatus(activeDeployment.status) ? 'success' : isPartialDeployStatus(activeDeployment.status) ? 'warning' : isFailedDeployStatus(activeDeployment.status) ? 'error' : 'processing'}>
                {activeDeployment.status}
              </Tag>
              {activeDeployment.finished_at && <span>完成于 {new Date(activeDeployment.finished_at).toLocaleString()}</span>}
            </Space>
          }
        >
          <Steps
            current={currentStep}
            size="small"
            items={STAGE_ORDER.map((title) => ({ title }))}
            status={deploymentStepStatus(activeDeployment.status)}
          />
          <Progress
            percent={deploymentProgress(activeDeployment.status, activeDeployment.progress)}
            status={deploymentProgressStatus(activeDeployment.status)}
            style={{ marginTop: 16 }}
          />
          <div style={{ marginTop: 8, color: '#666' }}>{activeDeployment.message}</div>

          {activeDeployment.steps && activeDeployment.steps.length > 0 && (
            <div style={{ marginTop: 16 }}>
              <strong>详细步骤</strong>
              <Steps direction="vertical" size="small" style={{ marginTop: 8 }} current={activeDeployment.steps.findIndex((s) => s.status === 'running')}>
                {activeDeployment.steps.map((step, idx) => (
                  <Steps.Step
                    key={idx}
                    title={step.name}
                    description={
                      <div>
                        <div style={{ color: '#888', fontSize: 12 }}>
                          {step.message || ''}
                          {step.started_at && ` (${new Date(step.started_at).toLocaleTimeString()})`}
                          {step.completed_at && ` -> ${new Date(step.completed_at).toLocaleTimeString()}`}
                        </div>
                      </div>
                    }
                    status={step.status === 'completed' ? 'finish' : step.status === 'running' ? 'process' : step.status === 'failed' ? 'error' : 'wait'}
                  />
                ))}
              </Steps>
            </div>
          )}

          {!activeDeployment.steps || activeDeployment.steps.length === 0 ? (
            <div style={{ marginTop: 16 }}>
              <strong>当前阶段子步骤</strong>
              <div style={{ marginTop: 8 }}>
                {(DEPLOY_SUBSTEPS[activeDeployment.stage || ''] || DEPLOY_SUBSTEPS['环境检查']).map((substep, idx) => (
                  <div key={idx} style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 4 }}>
                    {idx < (DEPLOY_SUBSTEPS[activeDeployment.stage || ''] || DEPLOY_SUBSTEPS['环境检查']).length - 1 ?
                      <span style={{ color: '#52c41a' }}>&#10003;</span> :
                      <span style={{ color: '#1677ff' }}>&#9679;</span>
                    }
                    <span>{substep}</span>
                  </div>
                ))}
              </div>
            </div>
          ) : null}

          {activeDeployment.nodes && activeDeployment.nodes.length > 0 && (
            <div style={{ marginTop: 16 }}>
              <strong>节点进度</strong>
              <div style={{ marginTop: 8 }}>
                {activeDeployment.nodes.map((node, idx) => (
                  <Card key={idx} size="small" style={{ marginBottom: 8 }}
                    title={`${node.name || node.instance_id || `节点 ${idx + 1}`} (${node.host || '-'}:${node.port || '-'})`}
                    extra={<Tag color={node.role === 'master' || node.role === 'primary' ? 'blue' : 'default'}>{node.role || '-'}</Tag>}
                  >
                    <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                      <span>状态: <Tag color={node.status === 'healthy' ? 'success' : node.status === 'deploying' ? 'processing' : 'warning'}>{node.status || 'unknown'}</Tag></span>
                      {node.current_step && <span>步骤: {node.current_step}</span>}
                    </div>
                    {typeof node.progress === 'number' && (
                      <Progress percent={node.progress} size="small" style={{ marginTop: 4 }} />
                    )}
                    {node.message && <div style={{ color: '#888', fontSize: 12, marginTop: 4 }}>{node.message}</div>}
                  </Card>
                ))}
              </div>
            </div>
          )}

          {activeDeployment.logs && activeDeployment.logs.length > 0 && (
            <div style={{ marginTop: 16 }}>
              <strong>部署日志</strong>
              <div style={{ background: '#1e1e1e', color: '#d4d4d4', padding: 12, borderRadius: 6, maxHeight: 200, overflow: 'auto', fontFamily: 'monospace', fontSize: 12, marginTop: 8 }}>
                {activeDeployment.logs.map((log, idx) => (
                  <div key={idx}>{log}</div>
                ))}
              </div>
            </div>
          )}
        </Card>
      )}
    </div>
  )
}

export default ClusterDeploy
