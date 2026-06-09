import React, { useEffect, useRef, useState } from 'react'
import {
  Button, Card, Checkbox, Collapse, Descriptions, Empty, Form, Input, InputNumber, message, Modal, Progress, Select, Space, Steps, Table, Tabs, Tag,
} from 'antd'
import { CheckCircleOutlined, CloseCircleOutlined, ClusterOutlined, DeleteOutlined, PlayCircleOutlined, ReloadOutlined } from '@ant-design/icons'
import type { ColumnsType } from 'antd/es/table'
import { clusterDeployApi, hostApi, instanceApi, type Host, type Instance } from '../services/api'

type ArchType = 'ha' | 'mha' | 'mgr' | 'pxc'

const DEFAULT_MYSQL_CREDENTIAL = {
  username: 'root',
  password: 'Hcfc@DboOps#2024_80',
}

const DEFAULT_CREDENTIAL_ACK_KEY = 'dbops.clusterDeploy.defaultMysqlCredentialAck'
const STAGE_ORDER = ['环境检查', '安装二进制', '配置集群', '启动节点', '集群验证']

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
  nodes?: Array<{ instance_id?: string; name?: string; host?: string; port?: number; role?: string }>
}

const normalizeStatus = (status?: string) => (status || '').trim().toLowerCase()

const isCompletedDeployStatus = (status?: string) =>
  ['success', 'completed', 'succeeded', 'ok'].includes(normalizeStatus(status))

const isFailedDeployStatus = (status?: string) =>
  ['failed', 'error', 'timeout', 'cancelled', 'canceled'].includes(normalizeStatus(status))

const isPartialDeployStatus = (status?: string) =>
  ['partial', 'partial_success'].includes(normalizeStatus(status))

const isDestroyedDeployStatus = (status?: string) => normalizeStatus(status) === 'destroyed'

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
  })

  const loadDeployments = async () => {
    setHistoryLoading(true)
    try {
      const res: any = await clusterDeployApi.list(100, 0)
      setDeployments((Array.isArray(res?.data) ? res.data : []).map(normalizeDeployment))
    } catch {
      setDeployments([])
    } finally {
      setHistoryLoading(false)
    }
  }

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

  const showDeployNotes = () => {
    Modal.info({
      title: '集群部署操作说明',
      content: (
        <div>
          <p>真实部署会在目标主机上安装或配置 MySQL 实例，并可能修改复制、服务和数据目录配置。</p>
          <p>伪集群演练模式只把已有实例写入平台集群拓扑，用于验证管理、拓扑、角色切换和销毁流程，不会停止或删除数据库服务。</p>
        </div>
      ),
      okText: '知道了',
    })
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
      content: '销毁只清理平台纳管关系、拓扑和角色状态，不会停止或删除数据库服务。',
      okText: '确认销毁',
      cancelText: '取消',
      onOk: async () => {
        const res: any = await clusterDeployApi.destroy(record.deployment_id)
        const next: DeployResult = {
          ...record,
          status: 'destroyed',
          progress: 100,
          message: res?.data?.message || '集群已销毁',
          finished_at: new Date().toISOString(),
        }
        patchDeployment(next)
        loadDeployments()
        message.success('集群已销毁')
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
    { title: '部署ID', dataIndex: 'deployment_id', key: 'deployment_id', width: 200 },
    { title: '集群ID', dataIndex: 'cluster_id', key: 'cluster_id' },
    {
      title: '架构',
      dataIndex: 'cluster_type',
      key: 'cluster_type',
      width: 100,
      render: (type: ArchType) => <Tag color={type === 'ha' ? 'cyan' : type === 'mha' ? 'blue' : type === 'mgr' ? 'green' : 'orange'}>{type.toUpperCase()}</Tag>,
    },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      width: 100,
      render: (status: string) => {
        if (isCompletedDeployStatus(status)) return <Tag color="success" icon={<CheckCircleOutlined />}>成功</Tag>
        if (isDestroyedDeployStatus(status)) return <Tag color="default">已销毁</Tag>
        if (isPartialDeployStatus(status)) return <Tag color="warning" icon={<CloseCircleOutlined />}>部分完成</Tag>
        if (isFailedDeployStatus(status)) return <Tag color="error" icon={<CloseCircleOutlined />}>失败</Tag>
        if (normalizeStatus(status) === 'pending') return <Tag color="default">待开始</Tag>
        return <Tag color="processing" icon={<ReloadOutlined spin />}>进行中</Tag>
      },
    },
    { title: '当前阶段', dataIndex: 'stage', key: 'stage', render: (stage: string) => stage || '-' },
    {
      title: '进度',
      dataIndex: 'progress',
      key: 'progress',
      width: 180,
      render: (progress: number, record) => <Progress percent={deploymentProgress(record.status, progress)} size="small" status={deploymentProgressStatus(record.status)} />,
    },
    { title: '信息', dataIndex: 'message', key: 'message' },
    {
      title: '节点信息',
      key: 'nodes',
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
    { title: '开始时间', dataIndex: 'started_at', key: 'started_at', render: (time: string) => (time ? new Date(time).toLocaleString() : '-') },
    {
      title: '操作',
      key: 'action',
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
    <Form form={form} layout="vertical" onFinish={onFinish}>
      <Form.Item name="cluster_id" label="集群ID" rules={[{ required: true, message: '请输入集群ID' }]}>
        <Input placeholder={`例如: ${arch}-cluster-01`} />
      </Form.Item>
      <Form.Item name="pseudo_mode" valuePropName="checked" initialValue>
        <Checkbox>伪集群演练模式</Checkbox>
      </Form.Item>
      <Form.Item name="master_host_id" label="主节点主机" rules={[{ required: true, message: '请选择主节点' }]}>
        <Select options={hostOptions} placeholder="选择主节点主机" />
      </Form.Item>
      {!options?.simpleReplica && (
        <Form.Item
          name="replica_host_ids"
          label="从节点主机"
          rules={[
            { required: true, message: '请选择从节点' },
            { validator: (_, value) => (Array.isArray(value) && value.length >= 1 ? Promise.resolve() : Promise.reject(new Error('至少选择 1 个从节点'))) },
          ]}
        >
          <Select mode="multiple" options={hostOptions} placeholder="至少选择 1 个从节点" maxTagCount={5} />
        </Form.Item>
      )}
      <Form.Item name="repl_user" label="复制用户" rules={[{ required: true }]} initialValue="repl_user">
        <Input />
      </Form.Item>
      <Form.Item name="repl_password" label="复制密码" rules={[{ required: true }]} initialValue="ReplPass#2026">
        <Input.Password />
      </Form.Item>
      <Form.Item name="mysql_port" label="主节点端口" initialValue={3309}>
        <InputNumber min={1} max={65535} style={{ width: '100%' }} />
      </Form.Item>
      <Form.Item name="vip" label="VIP (可选)">
        <Input placeholder="例如: 192.168.1.100" />
      </Form.Item>
      {extraFields}
      <Form.Item>
        <Button type="primary" icon={<PlayCircleOutlined />} htmlType="submit" loading={submitting}>
          启动 {arch.toUpperCase()} 部署
        </Button>
      </Form.Item>
    </Form>
  )

  return (
    <div style={{ padding: '24px' }}>
      <Card title="部署历史" style={{ marginBottom: 16 }}>
        {deployments.length === 0 ? (
          <Empty description="暂无部署记录" />
        ) : (
          <Table columns={columns} dataSource={deployments} rowKey="deployment_id" loading={historyLoading} />
        )}
      </Card>

      <Collapse
        defaultActiveKey={[]}
        items={[{
          key: 'deploy',
          label: <Space><ClusterOutlined /><span>集群部署</span></Space>,
          extra: <Button size="small" onClick={(e) => { e.stopPropagation(); showDeployNotes() }}>操作说明</Button>,
          children: (
            <>
        <Card size="small" title="默认创建的 MySQL 实例账号" extra={<Button size="small" onClick={openCredentialModal}>修改</Button>} style={{ marginBottom: 16 }}>
          {showDefaultCredential && (
            <Space direction="vertical" size={8}>
              <Descriptions size="small" column={1} bordered>
                <Descriptions.Item label="用户名">{credential.username}</Descriptions.Item>
                <Descriptions.Item label="密码">{credential.password}</Descriptions.Item>
              </Descriptions>
              <Button size="small" onClick={acknowledgeDefaultCredential}>我已保存，隐藏默认密码</Button>
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
            <Descriptions size="small" column={2}>
              <Descriptions.Item label="用户名">{credential.username}</Descriptions.Item>
              <Descriptions.Item label="密码">已隐藏</Descriptions.Item>
            </Descriptions>
          )}
        </Card>

        <Tabs
          activeKey={tab}
          onChange={(key) => setTab(key as ArchType)}
          items={[
            {
              key: 'ha',
              label: 'HA 主从',
              children: renderForm('ha', haForm,
                <>
                  <Form.Item name="replica_host_id" label="从节点主机" rules={[{ required: true, message: '请选择从节点' }]}>
                    <Select options={hostOptions} placeholder="选择从节点主机" />
                  </Form.Item>
                  <Form.Item name="replica_port" label="从节点端口" initialValue={3310}>
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
                <>
                  <Form.Item name="manager_host_id" label="MHA Manager 主机" rules={[{ required: true }]}>
                    <Select options={hostOptions} placeholder="选择 Manager 主机" />
                  </Form.Item>
                  <Form.Item name="replica_port" label="从节点端口" initialValue={3310}>
                    <InputNumber min={1} max={65535} style={{ width: '100%' }} />
                  </Form.Item>
                </>,
                (values) => runDeploy('mha', values, clusterDeployApi.deployMHA),
              ),
            },
            {
              key: 'mgr',
              label: 'MGR 部署',
              children: renderForm('mgr', mgrForm,
                <>
                  <Form.Item name="group_name" label="Group Name" initialValue="aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa">
                    <Input />
                  </Form.Item>
                  <Form.Item name="replica_port" label="从节点端口" initialValue={3310}>
                    <InputNumber min={1} max={65535} style={{ width: '100%' }} />
                  </Form.Item>
                </>,
                (values) => runDeploy('mgr', values, clusterDeployApi.deployMGR),
              ),
            },
            {
              key: 'pxc',
              label: 'PXC 部署',
              children: renderForm('pxc', pxcForm,
                <>
                  <Form.Item name="replica_port" label="从节点端口" initialValue={3310}>
                    <InputNumber min={1} max={65535} style={{ width: '100%' }} />
                  </Form.Item>
                  <Form.Item name="wsrep_port" label="wsrep 端口" initialValue={4567}>
                    <InputNumber min={1} max={65535} style={{ width: '100%' }} />
                  </Form.Item>
                </>,
                (values) => runDeploy('pxc', values, clusterDeployApi.deployPXC),
              ),
            },
          ]}
        />
            </>
          ),
        }]}
      />

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
        </Card>
      )}
    </div>
  )
}

export default ClusterDeploy
