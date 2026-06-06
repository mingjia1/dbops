import React, { useEffect, useRef, useState } from 'react'
import {
  Card, Tabs, Form, Input, InputNumber, Select, Button, Space, message, Steps, Progress, Tag, Table, Empty, Alert,
} from 'antd'
import { ClusterOutlined, PlayCircleOutlined, CheckCircleOutlined, CloseCircleOutlined, ReloadOutlined } from '@ant-design/icons'
import type { ColumnsType } from 'antd/es/table'
import { clusterDeployApi, hostApi, type Host } from '../services/api'

type ArchType = 'mha' | 'mgr' | 'pxc'

interface DeployResult {
  deployment_id: string
  cluster_id: string
  cluster_type: ArchType
  status: 'pending' | 'running' | 'success' | 'failed'
  stage?: string
  progress: number
  message: string
  started_at: string
  finished_at?: string
}

const STAGE_ORDER = ['环境检查', '安装二进制', '配置集群', '启动节点', '集群验证']

const ClusterDeploy: React.FC = () => {
  const [hosts, setHosts] = useState<Host[]>([])
  const [tab, setTab] = useState<ArchType>('mha')
  const [submitting, setSubmitting] = useState(false)
  const [deployments, setDeployments] = useState<DeployResult[]>([])
  const [activeDeployment, setActiveDeployment] = useState<DeployResult | null>(null)
  const [currentStep, setCurrentStep] = useState(0)
  const pollRef = useRef<number | null>(null)

  const [mhaForm] = Form.useForm()
  const [mgrForm] = Form.useForm()
  const [pxcForm] = Form.useForm()

  useEffect(() => {
    hostApi.list(100, 0).then((res: any) => setHosts(res?.data || [])).catch(() => {})
  }, [])

  useEffect(() => () => {
    if (pollRef.current) window.clearInterval(pollRef.current)
  }, [])

  const hostOptions = hosts.map((h) => ({ value: h.id, label: `${h.name} (${h.address})` }))

  const stopPolling = () => {
    if (pollRef.current) {
      window.clearInterval(pollRef.current)
      pollRef.current = null
    }
  }

  const patchDeployment = (dep: DeployResult) => {
    setDeployments((ds) => ds.map((d) => (d.deployment_id === dep.deployment_id ? dep : d)))
    setActiveDeployment((cur) => (cur && cur.deployment_id === dep.deployment_id ? dep : cur))
  }

  const startPolling = (dep: DeployResult) => {
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
        }
        patchDeployment(next)
        const stepIdx = next.stage ? STAGE_ORDER.indexOf(next.stage) : -1
        // P2: 之前 +1 让"环境检查"(index 0) → current=1 → Steps 高亮"安装二进制",
        // 部署开始第一步就跳到第二步. 修: antd Steps.current 是 0-based, 直接用 stepIdx.
        if (stepIdx >= 0) setCurrentStep(stepIdx)
        if (next.status === 'success' || next.status === 'failed' || attempts > 600) {
          stopPolling()
          if (next.status === 'success') message.success(`${dep.cluster_type.toUpperCase()} 集群部署完成`)
          else if (next.status === 'failed') message.error(`部署失败: ${next.message}`)
        }
      } catch {
        // 静默失败, 下次重试
      }
    }, 2000)
  }

  const runDeploy = async (
    arch: ArchType,
    values: any,
    apiCall: (data: any) => Promise<any>,
  ) => {
    setSubmitting(true)
    setCurrentStep(0)
    setActiveDeployment(null)
    try {
      const res: any = await apiCall({ cluster_id: values.cluster_id, ...values })
      const dep: DeployResult = {
        deployment_id: res?.data?.deployment_id || `dep-${Date.now()}`,
        cluster_id: values.cluster_id,
        cluster_type: arch,
        status: 'running',
        progress: 0,
        stage: STAGE_ORDER[0],
        message: res?.data?.message || '部署已提交, 等待后端开始执行',
        started_at: new Date().toISOString(),
      }
      setActiveDeployment(dep)
      setDeployments((ds) => [dep, ...ds])
      message.success(`${arch.toUpperCase()} 集群部署任务已提交`)
      startPolling(dep)
    } catch (err: any) {
      message.error(`提交部署失败: ${err?.response?.data?.message || err?.message}`)
    } finally {
      setSubmitting(false)
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
      render: (t: ArchType) => <Tag color={t === 'mha' ? 'blue' : t === 'mgr' ? 'green' : 'orange'}>{t.toUpperCase()}</Tag>,
    },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      width: 100,
      render: (s: string) => {
        if (s === 'success') return <Tag color="success" icon={<CheckCircleOutlined />}>成功</Tag>
        if (s === 'failed') return <Tag color="error" icon={<CloseCircleOutlined />}>失败</Tag>
        if (s === 'pending') return <Tag color="default">待开始</Tag>
        return <Tag color="processing" icon={<ReloadOutlined spin />}>进行中</Tag>
      },
    },
    {
      title: '当前阶段',
      dataIndex: 'stage',
      key: 'stage',
      render: (s: string) => s || '-',
    },
    {
      title: '进度',
      dataIndex: 'progress',
      key: 'progress',
      width: 200,
      render: (p: number) => <Progress percent={p} size="small" status={p === 100 ? 'success' : 'active'} />,
    },
    { title: '信息', dataIndex: 'message', key: 'message' },
    {
      title: '开始时间',
      dataIndex: 'started_at',
      key: 'started_at',
      render: (t: string) => (t ? new Date(t).toLocaleString() : '-'),
    },
  ]

  const renderForm = (
    arch: ArchType,
    form: any,
    extraFields: React.ReactNode,
    onFinish: (v: any) => void,
  ) => (
    <Form form={form} layout="vertical" onFinish={onFinish}>
      <Form.Item name="cluster_id" label="集群ID" rules={[{ required: true, message: '请输入集群ID' }]}>
        <Input placeholder={`例如: ${arch}-cluster-01`} />
      </Form.Item>
      <Form.Item name="master_host_id" label="主节点主机" rules={[{ required: true, message: '请选择主节点' }]}>
        <Select options={hostOptions} placeholder="选择主节点主机" />
      </Form.Item>
      <Form.Item name="replica_host_ids" label="从节点主机" rules={[{ required: true, message: '请选择从节点' }]}>
        <Select mode="multiple" options={hostOptions} placeholder="可多选" />
      </Form.Item>
      <Form.Item name="repl_user" label="复制用户" rules={[{ required: true }]} initialValue="repl_user">
        <Input />
      </Form.Item>
      <Form.Item name="repl_password" label="复制密码" rules={[{ required: true }]}>
        <Input.Password />
      </Form.Item>
      <Form.Item name="mysql_port" label="MySQL 端口" initialValue={3306}>
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
      <Alert
        type="warning"
        showIcon
        style={{ marginBottom: 16 }}
        message="操作提示"
        description="集群部署是不可逆的破坏性操作: 会在目标主机上安装 MySQL 二进制、初始化 datadir、修改系统配置。请确认已在测试环境验证, 并具备回滚方案 (快照/备份)。"
      />
      <Card title={<Space><ClusterOutlined /><span>集群部署</span></Space>}>
        <Tabs
          activeKey={tab}
          onChange={(k) => setTab(k as ArchType)}
          items={[
            {
              key: 'mha',
              label: 'MHA 部署',
              children: renderForm('mha', mhaForm,
                <Form.Item name="manager_host_id" label="MHA Manager 主机" rules={[{ required: true }]}>
                  <Select options={hostOptions} placeholder="选择 Manager 主机" />
                </Form.Item>,
                (v) => runDeploy('mha', v, clusterDeployApi.deployMHA),
              ),
            },
            {
              key: 'mgr',
              label: 'MGR 部署',
              children: renderForm('mgr', mgrForm,
                <Form.Item name="group_name" label="Group Name" initialValue="aaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa">
                  <Input />
                </Form.Item>,
                (v) => runDeploy('mgr', v, clusterDeployApi.deployMGR),
              ),
            },
            {
              key: 'pxc',
              label: 'PXC 部署',
              children: renderForm('pxc', pxcForm,
                <Form.Item name="wsrep_port" label="wsrep 端口" initialValue={4567}>
                  <InputNumber min={1} max={65535} style={{ width: '100%' }} />
                </Form.Item>,
                (v) => runDeploy('pxc', v, clusterDeployApi.deployPXC),
              ),
            },
          ]}
        />
      </Card>

      {activeDeployment && (
        <Card
          title="部署进度"
          style={{ marginTop: 16 }}
          extra={
            <Space>
              <Tag color={activeDeployment.status === 'success' ? 'success' : activeDeployment.status === 'failed' ? 'error' : 'processing'}>
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
            status={activeDeployment.status === 'failed' ? 'error' : activeDeployment.status === 'success' ? 'finish' : 'process'}
          />
          <Progress
            percent={activeDeployment.progress}
            status={activeDeployment.status === 'failed' ? 'exception' : activeDeployment.status === 'success' ? 'success' : 'active'}
            style={{ marginTop: 16 }}
          />
          <div style={{ marginTop: 8, color: '#666' }}>{activeDeployment.message}</div>
        </Card>
      )}

      <Card title="部署历史" style={{ marginTop: 16 }}>
        {deployments.length === 0 ? (
          <Empty description="暂无部署记录" />
        ) : (
          <Table columns={columns} dataSource={deployments} rowKey="deployment_id" />
        )}
      </Card>
    </div>
  )
}

export default ClusterDeploy
