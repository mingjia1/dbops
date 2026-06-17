import React, { useEffect, useRef, useState } from 'react'
import {
  Alert, Button, Card, Checkbox, Col, Descriptions, Empty, Form, Input, InputNumber, message, Modal, Popover, Progress, Row, Select, Space, Steps, Table, Tabs, Tag,
} from 'antd'
import { CheckCircleOutlined, CloseCircleOutlined, ClusterOutlined, DeleteOutlined, EyeOutlined, KeyOutlined, PlayCircleOutlined, ReloadOutlined } from '@ant-design/icons'
import type { ColumnsType } from 'antd/es/table'
import { clusterDeployApi, hostApi, instanceApi, type Host, type Instance } from '../services/api'

type ArchType = 'ha' | 'mha' | 'mgr' | 'pxc'

const createMgrGroupName = () => {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
    return crypto.randomUUID()
  }
  const hex = `${Date.now().toString(16)}${Math.random().toString(16).slice(2)}00000000000000000000000000000000`.slice(0, 32)
  return `${hex.slice(0, 8)}-${hex.slice(8, 12)}-4${hex.slice(13, 16)}-8${hex.slice(17, 20)}-${hex.slice(20, 32)}`
}

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

  // Plan preview state
  const [planPreviewOpen, setPlanPreviewOpen] = useState(false)
  const [planPreviewData, setPlanPreviewData] = useState<any>(null)
  const [planPreviewArch, setPlanPreviewArch] = useState<ArchType>('ha')
  const [planPreviewLoading, setPlanPreviewLoading] = useState(false)
  const [pendingDeployPayload, setPendingDeployPayload] = useState<any>(null)
  const [pendingDeployArch, setPendingDeployArch] = useState<ArchType>('ha')
  const [pendingDeployValues, setPendingDeployValues] = useState<any>(null)

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

  const doPreview = (arch: ArchType, values: any) => {
    const payload = buildDeployPayload(arch, values)
    setPlanPreviewLoading(true)
    setPlanPreviewArch(arch)
    clusterDeployApi.validateCluster(payload).then((res: any) => {
      const plan = res?.data?.plan || res?.data
      setPlanPreviewData(plan)
      setPendingDeployPayload(payload)
      setPendingDeployArch(arch)
      setPendingDeployValues(values)
      setPlanPreviewOpen(true)
    }).catch((err: any) => {
      message.error(`计划验证失败: ${err?.response?.data?.message || err?.message}`)
    }).finally(() => {
      setPlanPreviewLoading(false)
    })
  }

  const doConfirmDeploy = () => {
    if (!pendingDeployPayload || !pendingDeployArch) return
    setPlanPreviewOpen(false)
    doDeploy(pendingDeployArch, pendingDeployValues)
  }

  const runDeploy = (arch: ArchType, values: any) => {
    // Show plan preview before deployment
    doPreview(arch, values)
  }

  const viewDeployPlan = async (record: DeployResult) => {
    try {
      const res: any = await clusterDeployApi.getDeployPlan(record.deployment_id)
      const plan = res?.data || res
      setPlanPreviewArch(record.cluster_type)
      setPlanPreviewData(plan)
      setPendingDeployPayload(null)
      setPlanPreviewOpen(true)
    } catch (err: any) {
      message.error(`获取部署计划失败: ${err?.response?.data?.message || err?.message}`)
    }
  }

  const doDeploy = async (arch: ArchType, values: any) => {
    setSubmitting(true)
    setCurrentStep(0)
    setActiveDeployment(null)
    try {
      const payload = buildDeployPayload(arch, values)
      await clusterDeployApi.validateCluster(payload)
      const res: any = await clusterDeployApi.deployCluster(payload)
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

  const parseMySQLConfig = (text?: string) => {
    const config: Record<string, string> = {}
    ;(text || '').split('\n').forEach((line) => {
      const item = line.trim()
      if (!item || item.startsWith('#')) return
      const idx = item.indexOf('=')
      if (idx <= 0) return
      config[item.slice(0, idx).trim()] = item.slice(idx + 1).trim()
    })
    return config
  }

  const buildDeployPayload = (arch: ArchType, values: any) => {
    const replicaHostIDs = values.replica_host_ids || (values.replica_host_id ? [values.replica_host_id] : [])
    const nodes: any[] = []
    const addNode = (hostID: string, role: string, port?: number, extra?: Record<string, any>) => {
      if (!hostID) return
      nodes.push({
        host_id: hostID,
        role,
        mysql_port: port || values.mysql_port || 3306,
        data_dir: extra?.data_dir,
        basedir: extra?.basedir,
        server_id: extra?.server_id,
        custom: extra?.custom,
      })
    }

    if (arch === 'ha') {
      addNode(values.master_host_id, 'master', values.mysql_port, { data_dir: values.master_data_dir, server_id: values.master_server_id })
      replicaHostIDs.forEach((hostID: string, index: number) => addNode(hostID, 'replica', values.replica_port || values.mysql_port || 3306, {
        data_dir: values.replica_data_dir,
        server_id: values.replica_server_id || (values.master_server_id ? Number(values.master_server_id) + index + 1 : undefined),
      }))
    } else if (arch === 'mha') {
      addNode(values.manager_host_id, 'manager', values.mysql_port)
      addNode(values.master_host_id, 'master', values.mysql_port)
      replicaHostIDs.forEach((hostID: string) => addNode(hostID, 'replica', values.replica_port || values.mysql_port || 3306))
    } else if (arch === 'mgr') {
      addNode(values.master_host_id, 'primary', values.mysql_port, { server_id: values.master_server_id, custom: { local_port: values.local_port } })
      replicaHostIDs.forEach((hostID: string, index: number) => addNode(hostID, 'secondary', values.replica_port || values.mysql_port || 3306, {
        server_id: values.replica_server_id || (values.master_server_id ? Number(values.master_server_id) + index + 1 : undefined),
        custom: values.local_port ? { local_port: Number(values.local_port) + index + 1 } : undefined,
      }))
    } else {
      addNode(values.master_host_id, 'bootstrap', values.mysql_port, { data_dir: values.master_data_dir })
      replicaHostIDs.forEach((hostID: string) => addNode(hostID, 'secondary', values.replica_port || values.mysql_port || 3306, { data_dir: values.replica_data_dir }))
    }

    const custom: Record<string, any> = {}
    if (arch === 'ha' && values.semi_sync_enabled !== undefined) custom.semi_sync_enabled = !!values.semi_sync_enabled
    if (arch === 'mha') {
      if (values.vip) custom.vip = values.vip
      if (values.vip_interface) custom.vip_interface = values.vip_interface
      if (values.ping_interval) custom.ping_interval = values.ping_interval
      if (values.ping_retry) custom.ping_retry = values.ping_retry
      if (values.ssh_user) custom.ssh_user = values.ssh_user
    }
    if (arch === 'mgr') {
      if (values.group_name) custom.group_name = values.group_name
      if (values.local_port) custom.local_port = values.local_port
    }
    if (arch === 'pxc') {
      if (values.cluster_name) custom.cluster_name = values.cluster_name
      if (values.sst_method) custom.sst_method = values.sst_method
      if (values.wsrep_sst_port) custom.wsrep_sst_port = values.wsrep_sst_port
      if (values.wsrep_ssl_enabled !== undefined) custom.wsrep_ssl_enabled = !!values.wsrep_ssl_enabled
    }

    return {
      cluster_id: values.cluster_id,
      name: values.cluster_id,
      cluster_type: arch,
      mode: values.pseudo_mode ? 'pseudo' : 'real',
      mysql: {
        version: values.mysql_version || '8.0',
        user: credential.username,
        password: credential.password,
        package_url: values.package_url,
        package_checksum: values.package_checksum,
        config: parseMySQLConfig(values.mysql_config_text),
      },
      replication: {
        user: values.repl_user,
        password: values.repl_password,
        mode: arch === 'mgr' ? 'single-primary' : 'async',
      },
      nodes,
      custom,
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
      width: 140,
      render: (_, record) => (
        <Space>
          <Button size="small" icon={<EyeOutlined />} onClick={() => viewDeployPlan(record)}>
            查看计划
          </Button>
          <Button size="small" danger icon={<DeleteOutlined />} disabled={isDestroyedDeployStatus(record.status)} onClick={() => destroyDeployment(record)}>
            销毁
          </Button>
        </Space>
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
    <Form form={form} layout="horizontal" labelCol={{ span: 6 }} wrapperCol={{ span: 18 }} onFinish={onFinish}>
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
      <Row gutter={16}>
        <Col span={12}>
          <Form.Item name="mysql_version" label="MySQL版本" initialValue="8.0">
            <Input placeholder="8.0" />
          </Form.Item>
        </Col>
        <Col span={12}>
          <Form.Item name="package_url" label="安装包">
            <Input placeholder="https://repo.example/mysql.tar.gz" />
          </Form.Item>
        </Col>
      </Row>
      <Row gutter={16}>
        <Col span={12}>
          <Form.Item name="package_checksum" label="校验值">
            <Input placeholder="64 位 SHA256" />
          </Form.Item>
        </Col>
        <Col span={12}>
          <Form.Item name="master_data_dir" label="主数据目录">
            <Input placeholder="/data/mysql/3306" />
          </Form.Item>
        </Col>
      </Row>
      <Row gutter={16}>
        <Col span={12}>
          <Form.Item name="replica_data_dir" label="从数据目录">
            <Input placeholder="/data/mysql/3307" />
          </Form.Item>
        </Col>
        {(arch === 'ha' || arch === 'mgr') && (
          <Col span={12}>
            <Form.Item name="master_server_id" label="主server_id">
              <InputNumber min={1} style={{ width: '100%' }} />
            </Form.Item>
          </Col>
        )}
      </Row>
      {(arch === 'ha' || arch === 'mgr') && (
        <Row gutter={16}>
          <Col span={12}>
            <Form.Item name="replica_server_id" label="从server_id">
              <InputNumber min={1} style={{ width: '100%' }} />
            </Form.Item>
          </Col>
          {arch === 'mgr' && (
            <Col span={12}>
              <Form.Item name="local_port" label="MGR端口">
                <InputNumber min={1} max={65535} style={{ width: '100%' }} />
              </Form.Item>
            </Col>
          )}
        </Row>
      )}
      {arch === 'ha' && (
        <Row gutter={16}>
          <Col span={12}>
            <Form.Item name="semi_sync_enabled" label="半同步复制" valuePropName="checked" initialValue={false}>
              <Checkbox>启用</Checkbox>
            </Form.Item>
          </Col>
        </Row>
      )}
      {arch === 'mha' && (
        <Row gutter={16}>
          <Col span={12}>
            <Form.Item name="vip_interface" label="VIP 网卡">
              <Input placeholder="eth0" />
            </Form.Item>
          </Col>
          <Col span={12}>
            <Form.Item name="ping_interval" label="探测间隔">
              <InputNumber min={1} style={{ width: '100%' }} />
            </Form.Item>
          </Col>
          <Col span={12}>
            <Form.Item name="ssh_user" label="SSH 用户">
              <Input placeholder="root" />
            </Form.Item>
          </Col>
        </Row>
      )}
      {arch === 'pxc' && (
        <Row gutter={16}>
          <Col span={12}>
            <Form.Item name="cluster_name" label="PXC集群">
              <Input placeholder="pxc-prod" />
            </Form.Item>
          </Col>
          <Col span={12}>
            <Form.Item name="sst_method" label="SST方法" initialValue="xtrabackup-v2">
              <Input />
            </Form.Item>
          </Col>
          <Col span={12}>
            <Form.Item name="wsrep_ssl_enabled" label="wsrepSSL" valuePropName="checked" initialValue={false}>
              <Checkbox>启用</Checkbox>
            </Form.Item>
          </Col>
        </Row>
      )}
      <Row gutter={16}>
        <Col span={12}>
          <Form.Item name="mysql_config_text" label="MySQL配置">
        <Input.TextArea rows={3} placeholder={'max_connections=512\ninnodb_buffer_pool_size=2G'} />
          </Form.Item>
        </Col>
      </Row>
      <Form.Item wrapperCol={{ offset: 6 }}>
        <Space>
          <Button type="primary" icon={<PlayCircleOutlined />} htmlType="submit" loading={submitting}>
            启动部署
          </Button>
          <Button icon={<EyeOutlined />} loading={planPreviewLoading} onClick={() => form.validateFields().then((values: any) => doPreview(arch, values)).catch(() => {})}>
            预览计划
          </Button>
        </Space>
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
                  (values) => runDeploy('ha', values),
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
                  (values) => runDeploy('mha', values),
                ),
              },
              {
                key: 'mgr',
                label: 'MGR 部署',
                children: renderForm('mgr', mgrForm,
                  <Form.Item name="group_name" label="MGR 组名" initialValue={createMgrGroupName()}>
                    <Input />
                  </Form.Item>,
                  (values) => runDeploy('mgr', values),
                ),
              },
              {
                key: 'pxc',
                label: 'PXC 部署',
                children: renderForm('pxc', pxcForm,
                  <Form.Item name="wsrep_port" label="wsrep 端口" initialValue={4567}>
                    <InputNumber min={1} max={65535} style={{ width: '100%' }} />
                  </Form.Item>,
                  (values) => runDeploy('pxc', values),
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

      {/* Plan Preview Modal */}
      <Modal
        title={
          <Space>
            <EyeOutlined />
            <span>部署计划预览 - {planPreviewArch?.toUpperCase()}</span>
          </Space>
        }
        open={planPreviewOpen}
        onCancel={() => {
          setPlanPreviewOpen(false)
          setPlanPreviewData(null)
        }}
        width={800}
        footer={
          pendingDeployPayload ? (
            <Space>
              <Button onClick={() => {
                setPlanPreviewOpen(false)
                setPlanPreviewData(null)
              }}>取消</Button>
              <Button type="primary" icon={<PlayCircleOutlined />} onClick={doConfirmDeploy}>
                确认部署
              </Button>
            </Space>
          ) : (
            <Button onClick={() => {
              setPlanPreviewOpen(false)
              setPlanPreviewData(null)
            }}>关闭</Button>
          )
        }
        destroyOnClose
      >
        {planPreviewData ? (
          <div>
            {/* Mode warning for new deployments */}
            {pendingDeployPayload && (
              <Alert
                type={pendingDeployValues?.pseudo_mode ? 'info' : 'warning'}
                message={pendingDeployValues?.pseudo_mode
                  ? '伪集群演练只写入平台纳管关系和拓扑，不会修改目标主机上的数据库服务。'
                  : '真实部署会修改目标主机上的 MySQL 实例、复制配置和服务状态。请确认已完成环境检查并具备回滚方案。'}
                style={{ marginBottom: 16 }}
                showIcon
              />
            )}
            {/* Plan Summary */}
            <Descriptions size="small" column={2} bordered style={{ marginBottom: 16 }}>
              <Descriptions.Item label="部署ID">{planPreviewData.deployment_id || planPreviewData.id || '-'}</Descriptions.Item>
              <Descriptions.Item label="架构类型">
                <Tag color={planPreviewData.cluster_type === 'ha' ? 'cyan' : planPreviewData.cluster_type === 'mha' ? 'blue' : planPreviewData.cluster_type === 'mgr' ? 'green' : 'orange'}>
                  {(planPreviewData.cluster_type || '').toUpperCase()}
                </Tag>
              </Descriptions.Item>
              <Descriptions.Item label="部署模式">
                <Tag>{planPreviewData.mode || 'real'}</Tag>
              </Descriptions.Item>
              <Descriptions.Item label="节点数量">{planPreviewData.nodes?.length || 0}</Descriptions.Item>
              <Descriptions.Item label="步骤数量">{planPreviewData.steps?.length || 0}</Descriptions.Item>
              {planPreviewData.parameters?.mysql_version && (
                <Descriptions.Item label="MySQL 版本">{planPreviewData.parameters.mysql_version}</Descriptions.Item>
              )}
            </Descriptions>

            {/* Nodes Table */}
            <strong style={{ display: 'block', marginBottom: 8 }}>节点列表</strong>
            <Table
              size="small"
              columns={[
                { title: 'Host', dataIndex: 'host', key: 'host', width: 140 },
                { title: '角色', dataIndex: 'role', key: 'role', width: 100, render: (role: string) => <Tag>{role}</Tag> },
                { title: 'MySQL 端口', dataIndex: 'mysql_port', key: 'mysql_port', width: 100 },
                { title: 'Agent 端口', dataIndex: 'agent_port', key: 'agent_port', width: 100 },
                { title: '数据目录', dataIndex: 'data_dir', key: 'data_dir', width: 140, render: (v: string) => v || '-' },
                { title: 'Server ID', dataIndex: 'server_id', key: 'server_id', width: 80, render: (v: number) => v || '-' },
              ]}
              dataSource={planPreviewData.nodes || []}
              rowKey={(row: any) => row.id || row.host || Math.random()}
              pagination={false}
              style={{ marginBottom: 16 }}
            />

            {/* Steps Timeline */}
            <strong style={{ display: 'block', marginBottom: 8 }}>执行步骤</strong>
            <Steps
              direction="vertical"
              size="small"
              current={-1}
              items={(planPreviewData.steps || []).map((step: any, idx: number) => ({
                title: (
                  <Space size={4}>
                    <span>{step.name || step.id || `Step ${idx + 1}`}</span>
                    {step.type && <Tag color="default" style={{ fontSize: 10 }}>{step.type}</Tag>}
                    {step.target_node && <span style={{ color: '#888', fontSize: 12 }}>@{step.target_node}</span>}
                  </Space>
                ),
                description: step.depends_on?.length > 0 ? (
                  <span style={{ fontSize: 12, color: '#888' }}>依赖: {step.depends_on.join(', ')}</span>
                ) : undefined,
                status: 'wait' as const,
              }))}
            />

            {/* Architecture-specific parameters */}
            {planPreviewData.parameters && Object.keys(planPreviewData.parameters).length > 0 && (
              <div style={{ marginTop: 16 }}>
                <strong style={{ display: 'block', marginBottom: 8 }}>部署参数</strong>
                <Descriptions size="small" column={2} bordered>
                  {Object.entries(planPreviewData.parameters).map(([key, value]: [string, any]) => (
                    <Descriptions.Item label={key} key={key}>
                      {typeof value === 'object' ? JSON.stringify(value) : String(value)}
                    </Descriptions.Item>
                  ))}
                </Descriptions>
              </div>
            )}
          </div>
        ) : (
          <Empty description="无法加载部署计划" />
        )}
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
