import React, { useEffect, useMemo, useState } from 'react'
import {
  Alert,
  Button,
  Card,
  DatePicker,
  Descriptions,
  Empty,
  Form,
  Input,
  InputNumber,
  Modal,
  Progress,
  Select,
  Space,
  Spin,
  Steps,
  Switch,
  Table,
  Tag,
  Typography,
  message,
} from 'antd'
import {
  CheckCircleOutlined,
  ExclamationCircleOutlined,
  FileTextOutlined,
  PlayCircleOutlined,
  ReloadOutlined,
  RollbackOutlined,
} from '@ant-design/icons'
import { instanceApi, upgradeApi, versionApi, type Instance, type VersionEntry } from '../services/api'

const { Title, Paragraph } = Typography

const UPGRADE_STAGES = ['预检查', '备份数据', '停止服务', '安装新版本', '数据升级', '启动服务', '验证']
const ROLLING_UPGRADE_STAGES = ['集群检查', '升级从库', '从库验证', '角色切换', '升级原主库', '重建拓扑', '最终验证']

const UPGRADE_SUBSTEPS: Record<string, string[]> = {
  '预检查': ['检查磁盘空间', '验证版本兼容性', '检查实例状态', '验证备份状态'],
  '备份数据': ['执行全量备份', '验证备份完整性', '记录备份路径'],
  '停止服务': ['通知应用断开连接', '等待事务完成', '停止 MySQL 服务'],
  '安装新版本': ['下载安装包', '解压新版本', '替换二进制文件', '验证安装路径'],
  '数据升级': ['执行 mysql_upgrade', '应用系统表变更', '验证数据字典'],
  '启动服务': ['启动 MySQL 服务', '等待端口就绪', '执行健康检查'],
  '验证': ['连接测试', '查询系统变量', '执行数据校验', '生成升级报告'],
}

interface UpgradeStep {
  name: string
  status: string
  message?: string
  started_at?: string
  completed_at?: string
}

interface ActiveUpgrade {
  task_id: string
  instance_id: string
  cluster_id?: string
  strategy?: string
  task_type?: string
  status: string
  progress: number
  stage?: string
  message?: string
  steps?: UpgradeStep[]
  logs?: string[]
  started_at: string
  finished_at?: string
}

interface UpgradeHistory {
  id: string
  instance_id: string
  instance_name?: string
  upgrade_type?: string
  task_type?: string
  plan_id?: string
  source_version?: string
  target_version?: string
  status: string
  progress?: number
  stage?: string
  message?: string
  start_time?: string
  created_at?: string
}

const strategyOptions = [
  { value: 'inplace', label: '原地升级' },
  { value: 'logical', label: '逻辑迁移' },
  { value: 'rolling', label: '滚动升级' },
]

const activeUpgradeStatuses = new Set(['pending', 'running', 'queued', 'executing'])
const terminalUpgradeStatuses = new Set(['success', 'completed', 'failed', 'error', 'cancelled', 'canceled', 'timeout'])
const isCompletedUpgradeStatus = (status?: string) => ['success', 'completed'].includes((status || '').toLowerCase())
const isFailedUpgradeStatus = (status?: string) => ['failed', 'error', 'cancelled', 'canceled', 'timeout'].includes((status || '').toLowerCase())

const clampProgress = (progress?: number) => {
  if (typeof progress !== 'number' || Number.isNaN(progress)) return 0
  return Math.max(0, Math.min(100, progress))
}

const upgradeStagesFor = (upgrade?: Pick<ActiveUpgrade, 'strategy' | 'task_type'> | Pick<UpgradeHistory, 'upgrade_type' | 'task_type'>) => {
  const rawType = `${(upgrade as any)?.strategy || ''} ${(upgrade as any)?.upgrade_type || ''} ${(upgrade as any)?.task_type || ''}`.toLowerCase()
  return rawType.includes('rolling') ? ROLLING_UPGRADE_STAGES : UPGRADE_STAGES
}

const inferStepIndex = (progress: number, stages: string[], status?: string, stage?: string) => {
  const normalized = (status || '').toLowerCase()
  if (isCompletedUpgradeStatus(normalized)) return stages.length - 1
  if (isFailedUpgradeStatus(normalized)) return Math.min(stages.length - 1, Math.floor((clampProgress(progress) / 100) * stages.length))
  if (stage) {
    const idx = stages.indexOf(stage)
    if (idx >= 0) return idx
  }
  return Math.min(stages.length - 1, Math.floor((clampProgress(progress) / 100) * stages.length))
}

const buildUpgradeSteps = (upgrade: ActiveUpgrade): UpgradeStep[] => {
  if (upgrade.steps?.length) return upgrade.steps
  const stages = upgradeStagesFor(upgrade)
  const current = inferStepIndex(upgrade.progress, stages, upgrade.status, upgrade.stage)
  return stages.map((name, idx) => {
    let status = 'wait'
    if (idx < current || isCompletedUpgradeStatus(upgrade.status)) status = 'completed'
    else if (idx === current && !terminalUpgradeStatuses.has((upgrade.status || '').toLowerCase())) status = 'running'
    else if (idx === current && isFailedUpgradeStatus(upgrade.status)) status = 'failed'
    return {
      name,
      status,
      message: idx === current ? (upgrade.message || '正在执行') : undefined,
    }
  })
}

const currentUpgradeStage = (upgrade: ActiveUpgrade) => {
  const stages = upgradeStagesFor(upgrade)
  return stages[inferStepIndex(upgrade.progress, stages, upgrade.status, upgrade.stage)] || stages[0]
}

const UpgradeManage: React.FC = () => {
  const [history, setHistory] = useState<UpgradeHistory[]>([])
  const [instances, setInstances] = useState<Instance[]>([])
  const [versions, setVersions] = useState<VersionEntry[]>([])
  const [versionsLoading, setVersionsLoading] = useState(false)
  const [planOpen, setPlanOpen] = useState(false)
  const [compatOpen, setCompatOpen] = useState(false)
  const [inPlaceOpen, setInPlaceOpen] = useState(false)
  const [planResult, setPlanResult] = useState<any>(null)
  const [compatResult, setCompatResult] = useState<any>(null)
  const [reportOpen, setReportOpen] = useState(false)
  const [reportLoading, setReportLoading] = useState(false)
  const [reportResult, setReportResult] = useState<any>(null)
  const [submitting, setSubmitting] = useState(false)
  const [activeUpgrade, setActiveUpgrade] = useState<ActiveUpgrade | null>(null)
  const [upgradeStep, setUpgradeStep] = useState(0)
  const [planForm] = Form.useForm()
  const [compatForm] = Form.useForm()
  const [inPlaceForm] = Form.useForm()

  const planInstanceId = Form.useWatch('instance_id', planForm)
  const compatInstanceId = Form.useWatch('instance_id', compatForm)
  const inPlaceInstanceId = Form.useWatch('instance_id', inPlaceForm)
  const executeStrategy = Form.useWatch('strategy', inPlaceForm)

  const loadData = () => {
    upgradeApi.listHistory().then((res: any) => setHistory(res?.data || [])).catch(() => setHistory([]))
    instanceApi.list(1000, 0).then((res: any) => setInstances(res?.data || [])).catch(() => setInstances([]))
    setVersionsLoading(true)
    versionApi.list().then((res: any) => setVersions(res?.data || [])).catch(() => setVersions([])).finally(() => setVersionsLoading(false))
  }

  useEffect(() => {
    loadData()
  }, [])

  useEffect(() => {
    if (!history.some((item) => activeUpgradeStatuses.has((item.status || '').toLowerCase()))) return
    const timer = window.setInterval(loadData, 5000)
    return () => window.clearInterval(timer)
  }, [history])

  useEffect(() => {
    if (!activeUpgrade) return
    if (terminalUpgradeStatuses.has((activeUpgrade.status || '').toLowerCase())) return
    const timer = window.setInterval(async () => {
      try {
        const res: any = await upgradeApi.get(activeUpgrade.task_id)
        const data = res?.data
        if (data) {
          setActiveUpgrade((prev) => prev ? {
            ...prev,
            status: data.status || prev.status,
            progress: typeof data.progress === 'number' ? data.progress : prev.progress,
            stage: data.stage || currentUpgradeStage({ ...prev, progress: typeof data.progress === 'number' ? data.progress : prev.progress, status: data.status || prev.status }),
            message: data.message || data.error_message || prev.message,
            task_type: data.task_type || prev.task_type,
            steps: Array.isArray(data.steps) ? data.steps : undefined,
            logs: Array.isArray(data.logs) ? data.logs : prev.logs,
            finished_at: data.finished_at || data.completed_at,
          } : prev)
          const nextStages = upgradeStagesFor({ strategy: activeUpgrade.strategy, task_type: data.task_type || activeUpgrade.task_type })
          setUpgradeStep(inferStepIndex(typeof data.progress === 'number' ? data.progress : activeUpgrade.progress, nextStages, data.status || activeUpgrade.status, data.stage))
          if (terminalUpgradeStatuses.has((data.status || '').toLowerCase())) {
            loadData()
          }
        }
      } catch {}
    }, 3000)
    return () => window.clearInterval(timer)
  }, [activeUpgrade?.task_id, activeUpgrade?.status])

  const instanceOptions = useMemo(
    () => instances.map((i) => ({
      value: i.id,
      label: `${i.name} (${i.connection?.host || i.host || '-'}:${i.connection?.port || i.port || '-'})`,
    })),
    [instances],
  )

  const versionOptions = useMemo(
    () => versions
      .slice()
      .sort((a, b) => {
        if (a.flavor !== b.flavor) return a.flavor.localeCompare(b.flavor)
        return b.release_date.localeCompare(a.release_date)
      })
      .map((v) => ({
        value: v.id,
        label: `${v.flavor} ${v.version}${v.is_lts ? ' [LTS]' : ''}${v.status === 'eol' ? ' [EOL]' : ''}`,
      })),
    [versions],
  )

  const clusterOptions = useMemo(() => {
    const clusterIds = Array.from(new Set(instances.map((i) => i.cluster_id).filter(Boolean)))
    return clusterIds.map((clusterId) => ({
      value: clusterId,
      label: `${clusterId} (${instances.filter((i) => i.cluster_id === clusterId).length} 个实例)`,
    }))
  }, [instances])

  const findInstance = (id?: string) => instances.find((i) => i.id === id)
  const detectedVersion = (inst?: Instance) =>
    inst?.version?.full_version || inst?.version?.version || inst?.connection?.version_id || '未识别'

  const versionInfo = (id?: string) => (
    <Descriptions size="small" bordered column={1} style={{ marginBottom: 16 }}>
      <Descriptions.Item label="当前源版本">
        <Tag color={id ? 'blue' : 'default'}>{detectedVersion(findInstance(id))}</Tag>
      </Descriptions.Item>
    </Descriptions>
  )

  const planUpgrade = async (values: any) => {
    if (!values.backup_confirmed) {
      message.warning('请先确认数据已完成备份')
      return
    }
    setSubmitting(true)
    try {
      const res: any = await upgradeApi.planPath({
        instance_id: values.instance_id,
        target_version: values.target_version,
        strategy: values.strategy,
        check_data_exists: !!values.check_data_exists,
        backup_confirmed: !!values.backup_confirmed,
        backup_method: values.backup_method,
        scheduled_time: values.scheduled_time,
      })
      setPlanResult(res?.data)
      message.success('升级路径规划已生成')
    } catch (err: any) {
      message.error(err?.response?.data?.message || err?.message || '升级路径规划失败')
    } finally {
      setSubmitting(false)
    }
  }

  const checkCompatibility = async (values: any) => {
    setSubmitting(true)
    try {
      const res: any = await upgradeApi.checkCompat(values)
      setCompatResult(res?.data)
      message.success('兼容性检查完成')
    } catch (err: any) {
      setCompatResult(null)
      message.error(err?.response?.data?.message || err?.message || '兼容性检查失败')
    } finally {
      setSubmitting(false)
    }
  }

  const executeUpgrade = async (values: any) => {
    if (!values.backup_enabled) {
      message.warning('请确认数据已备份后再启动升级')
      return
    }
    setSubmitting(true)
    try {
      const strategy = values.strategy || 'inplace'
      let res: any
      if (strategy === 'logical') {
        res = await upgradeApi.executeLogical({
          instance_id: values.instance_id,
          plan_id: values.plan_id,
          target_version: values.target_version,
          backup_enabled: !!values.backup_enabled,
          parallelism: values.parallelism,
          batch_size: values.batch_size,
        })
      } else if (strategy === 'rolling') {
        res = await upgradeApi.executeRolling({
          cluster_id: values.cluster_id,
          plan_id: values.plan_id,
          target_version: values.target_version,
          max_in_parallel: values.max_in_parallel,
          health_check_interval: values.health_check_interval,
        })
      } else {
        res = await upgradeApi.executeInPlace({
          instance_id: values.instance_id,
          plan_id: values.plan_id,
          target_version: values.target_version,
          backup_enabled: !!values.backup_enabled,
        })
      }
      if (!res?.data?.task_id && !res?.data?.id) {
        throw new Error('upgrade API did not return task_id')
      }
      const taskId = res?.data?.task_id || res?.data?.id
      const activeUpgradeData: ActiveUpgrade = {
        task_id: taskId,
        instance_id: values.instance_id || '',
        cluster_id: values.cluster_id,
        strategy,
        task_type: strategy === 'rolling' ? 'upgrade_rolling' : strategy === 'logical' ? 'upgrade_logical' : 'upgrade_in_place',
        status: res?.data?.status || 'running',
        progress: typeof res?.data?.progress === 'number' ? res.data.progress : 0,
        stage: upgradeStagesFor({ strategy })[0],
        message: '升级任务已提交',
        started_at: new Date().toISOString(),
        steps: [],
        logs: [],
      }
      setActiveUpgrade(activeUpgradeData)
      setUpgradeStep(0)
      message.success('升级任务已提交')
      setInPlaceOpen(false)
      inPlaceForm.resetFields()
      loadData()
    } catch (err: any) {
      message.error(err?.response?.data?.message || err?.message || '升级任务提交失败')
    } finally {
      setSubmitting(false)
    }
  }

  const showReport = async (record: UpgradeHistory) => {
    setReportOpen(true)
    setReportLoading(true)
    setReportResult(null)
    try {
      const res: any = await upgradeApi.getReport(record.id)
      setReportResult(res?.data || null)
    } catch (err: any) {
      message.error(err?.response?.data?.message || err?.message || '加载升级报告失败')
      setReportOpen(false)
    } finally {
      setReportLoading(false)
    }
  }

  const canRollback = (record: UpgradeHistory) => {
    const type = (record.task_type || record.upgrade_type || '').toLowerCase()
    if (type.includes('rollback')) return false
    const status = (record.status || '').toLowerCase()
    return terminalUpgradeStatuses.has(status) && !!record.instance_id
  }

  const rollbackUpgrade = (record: UpgradeHistory) => {
    Modal.confirm({
      title: '\u786e\u8ba4\u56de\u6eda\u5347\u7ea7',
      content: '\u56de\u6eda\u4f1a\u505c\u6b62\u76ee\u6807\u5b9e\u4f8b\u3001\u6062\u590d\u6570\u636e\u548c\u914d\u7f6e\uff0c\u8bf7\u786e\u8ba4\u5df2\u8bc4\u4f30\u4e1a\u52a1\u5f71\u54cd\u3002',
      okText: '\u786e\u8ba4\u56de\u6eda',
      cancelText: '\u53d6\u6d88',
      okButtonProps: { danger: true },
      onOk: async () => {
        setSubmitting(true)
        try {
          const res: any = await upgradeApi.rollback({
            plan_id: record.plan_id || record.id,
            instance_id: record.instance_id,
            force: true,
          })
          const data = res?.data || {}
          message.success(data.rollback_id ? `\u56de\u6eda\u4efb\u52a1\u5df2\u63d0\u4ea4: ${data.rollback_id}` : '\u56de\u6eda\u4efb\u52a1\u5df2\u63d0\u4ea4')
          loadData()
        } catch (err: any) {
          message.error(err?.response?.data?.message || err?.message || '\u56de\u6eda\u5347\u7ea7\u5931\u8d25')
        } finally {
          setSubmitting(false)
        }
      },
    })
  }

  const columns = [
    { title: 'ID', dataIndex: 'id', key: 'id', width: 150, ellipsis: true },
    {
      title: '信息',
      dataIndex: 'message',
      key: 'message',
      width: 260,
      ellipsis: true,
      render: (v: string) => v || '-',
    },
    {
      title: '操作',
      key: 'action',
      fixed: 'right' as const,
      width: 190,
      render: (_: any, record: UpgradeHistory) => (
        <Space size="small">
          <Button size="small" icon={<FileTextOutlined />} onClick={() => showReport(record)}>
            报告
          </Button>
          {canRollback(record) && (
            <Button size="small" danger icon={<RollbackOutlined />} loading={submitting} onClick={() => rollbackUpgrade(record)}>
              {'\u56de\u6eda'}
            </Button>
          )}
        </Space>
      ),
    },
    { title: '实例', dataIndex: 'instance_name', key: 'instance_name', render: (v: string, r: UpgradeHistory) => v || r.instance_id },
    { title: '类型', dataIndex: 'upgrade_type', key: 'upgrade_type', render: (v: string) => <Tag>{v || '-'}</Tag> },
    {
      title: '版本变化',
      key: 'version',
      render: (_: any, r: UpgradeHistory) => `${r.source_version || '-'} -> ${r.target_version || '-'}`,
    },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      render: (s: string) => <Tag color={isCompletedUpgradeStatus(s) ? 'success' : isFailedUpgradeStatus(s) ? 'error' : terminalUpgradeStatuses.has((s || '').toLowerCase()) ? 'default' : 'processing'}>{s || '-'}</Tag>,
    },
    {
      title: '进度',
      dataIndex: 'progress',
      key: 'progress',
      width: 160,
      render: (p: number) => <Progress percent={p || 0} size="small" />,
    },
    { title: '阶段', dataIndex: 'stage', key: 'stage', render: (v: string) => v || '-' },
    { title: '时间', dataIndex: 'start_time', key: 'start_time', render: (v: string, r: UpgradeHistory) => v || r.created_at || '-' },
  ]

  const activeUpgradeStages = activeUpgrade ? upgradeStagesFor(activeUpgrade) : UPGRADE_STAGES
  const activeUpgradeSteps = activeUpgrade ? buildUpgradeSteps(activeUpgrade) : []
  const activeUpgradeCurrentStage = activeUpgrade ? currentUpgradeStage(activeUpgrade) : UPGRADE_STAGES[0]
  const activeUpgradeSubsteps = UPGRADE_SUBSTEPS[activeUpgradeCurrentStage] || UPGRADE_SUBSTEPS['预检查']

  return (
    <div>
      <Title level={4}>版本升级管理</Title>
      <Alert
        type="warning"
        showIcon
        style={{ marginBottom: 16 }}
        message="升级前必须确认目标实例版本、数据存在性和备份状态。源版本由选中实例自动识别。"
      />

      <Card style={{ marginBottom: 16 }}>
        <Space wrap>
          <Button type="primary" icon={<FileTextOutlined />} onClick={() => setPlanOpen(true)}>规划升级路径</Button>
          <Button icon={<CheckCircleOutlined />} onClick={() => setCompatOpen(true)}>兼容性检查</Button>
          <Button danger icon={<PlayCircleOutlined />} onClick={() => setInPlaceOpen(true)}>启动升级任务</Button>
          <Button icon={<ReloadOutlined />} onClick={loadData}>刷新</Button>
        </Space>
      </Card>

      <Table columns={columns} dataSource={history} rowKey="id" scroll={{ x: 1000 }} />

      {activeUpgrade && (
        <Card
          title="升级进度"
          style={{ marginTop: 16 }}
          extra={
            <Space>
              <Tag color={isCompletedUpgradeStatus(activeUpgrade.status) ? 'success' : isFailedUpgradeStatus(activeUpgrade.status) ? 'error' : 'processing'}>
                {activeUpgrade.status}
              </Tag>
              {activeUpgrade.finished_at && <span>完成于 {new Date(activeUpgrade.finished_at).toLocaleString()}</span>}
            </Space>
          }
        >
          <Steps
            current={upgradeStep}
            size="small"
            items={activeUpgradeStages.map((title) => ({ title }))}
            status={isFailedUpgradeStatus(activeUpgrade.status) ? 'error' : isCompletedUpgradeStatus(activeUpgrade.status) ? 'finish' : 'process'}
          />
          <Progress
            percent={activeUpgrade.progress}
            status={isCompletedUpgradeStatus(activeUpgrade.status) ? 'success' : isFailedUpgradeStatus(activeUpgrade.status) ? 'exception' : 'active'}
            style={{ marginTop: 16 }}
          />
          <div style={{ marginTop: 8, color: '#666' }}>{activeUpgrade.message}</div>

          <div style={{ marginTop: 16 }}>
            <strong>详细步骤</strong>
            <Steps direction="vertical" size="small" style={{ marginTop: 8 }} current={activeUpgradeSteps.findIndex((s) => s.status === 'running')}>
              {activeUpgradeSteps.map((step, idx) => (
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

          <div style={{ marginTop: 16 }}>
            <strong>当前阶段子步骤</strong>
            <div style={{ marginTop: 8 }}>
              {activeUpgradeSubsteps.map((substep, idx) => (
                <div key={idx} style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 4 }}>
                  {idx < activeUpgradeSubsteps.length - 1 ?
                    <span style={{ color: '#52c41a' }}>&#10003;</span> :
                    <span style={{ color: '#1677ff' }}>&#9679;</span>
                  }
                  <span>{substep}</span>
                </div>
              ))}
            </div>
          </div>

          {activeUpgrade.logs && activeUpgrade.logs.length > 0 && (
            <div style={{ marginTop: 16 }}>
              <strong>升级日志</strong>
              <div style={{ background: '#1e1e1e', color: '#d4d4d4', padding: 12, borderRadius: 6, maxHeight: 200, overflow: 'auto', fontFamily: 'monospace', fontSize: 12, marginTop: 8 }}>
                {activeUpgrade.logs.map((log, idx) => (
                  <div key={idx}>{log}</div>
                ))}
              </div>
            </div>
          )}
        </Card>
      )}

      <Modal
        title="规划升级路径"
        open={planOpen}
        onCancel={() => setPlanOpen(false)}
        onOk={() => planForm.submit()}
        confirmLoading={submitting}
        width={720}
      >
        <Form form={planForm} layout="vertical" onFinish={planUpgrade} initialValues={{ check_data_exists: true, backup_confirmed: false, strategy: 'inplace' }}>
          <Form.Item name="instance_id" label="目标实例" rules={[{ required: true, message: '请选择实例' }]}>
            <Select showSearch optionFilterProp="label" options={instanceOptions} placeholder="选择实例" />
          </Form.Item>
          {versionInfo(planInstanceId)}
          <Form.Item name="target_version" label="目标版本" rules={[{ required: true, message: '请选择目标版本' }]}>
            <Select
              showSearch
              optionFilterProp="label"
              loading={versionsLoading}
              notFoundContent={versionsLoading ? <Spin size="small" /> : <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} />}
              options={versionOptions}
              placeholder="选择目标版本"
            />
          </Form.Item>
          <Form.Item name="strategy" label="升级策略" rules={[{ required: true }]}>
            <Select options={strategyOptions} />
          </Form.Item>
          <Form.Item name="check_data_exists" label="检测是否存在数据" valuePropName="checked">
            <Switch checkedChildren="检测" unCheckedChildren="跳过" />
          </Form.Item>
          <Form.Item name="backup_confirmed" label="数据是否已备份" valuePropName="checked">
            <Switch checkedChildren="已备份" unCheckedChildren="未备份" />
          </Form.Item>
          <Form.Item name="backup_method" label="备份方式">
            <Select allowClear options={[
              { value: 'full', label: '全量备份' },
              { value: 'incremental', label: '增量备份' },
              { value: 'external', label: '外部备份已完成' },
            ]} />
          </Form.Item>
          <Form.Item name="scheduled_time" label="计划执行时间">
            <DatePicker showTime style={{ width: '100%' }} />
          </Form.Item>
        </Form>
        {planResult && (
          <Card size="small" title="规划结果">
            <Paragraph>源版本: {planResult.source_flavor} {planResult.source_version}</Paragraph>
            <Paragraph>目标版本: {planResult.target_flavor} {planResult.target_version}</Paragraph>
            <Paragraph>风险等级: <Tag>{planResult.risk_level}</Tag></Paragraph>
            <Paragraph>预计耗时: {planResult.estimated_time || '-'} 分钟</Paragraph>
          </Card>
        )}
      </Modal>

      <Modal
        title="兼容性检查"
        open={compatOpen}
        onCancel={() => { setCompatOpen(false); setCompatResult(null) }}
        onOk={() => compatForm.submit()}
        confirmLoading={submitting}
        width={760}
      >
        <Form form={compatForm} layout="vertical" onFinish={checkCompatibility}>
          <Form.Item name="instance_id" label="目标实例" rules={[{ required: true, message: '请选择实例' }]}>
            <Select showSearch optionFilterProp="label" options={instanceOptions} placeholder="选择实例" />
          </Form.Item>
          {versionInfo(compatInstanceId)}
          <Form.Item name="target_version" label="目标版本" rules={[{ required: true, message: '请选择目标版本' }]}>
            <Select showSearch optionFilterProp="label" loading={versionsLoading} options={versionOptions} placeholder="选择目标版本" />
          </Form.Item>
        </Form>
        {compatResult && (
          <Card size="small" title="检查结果">
            <Tag color={compatResult.is_compatible ? 'success' : 'error'}>
              {compatResult.is_compatible ? '兼容' : '不兼容'}
            </Tag>
            <Paragraph style={{ marginTop: 12 }}>错误: {compatResult.error_count || 0}，警告: {compatResult.warning_count || 0}</Paragraph>
            {(compatResult.incompatibilities || []).map((item: any, index: number) => (
              <Alert key={index} style={{ marginTop: 8 }} type={item.level === 'error' ? 'error' : 'warning'} message={item.description} description={item.solution} />
            ))}
          </Card>
        )}
      </Modal>

      <Modal
        title="启动升级任务"
        open={inPlaceOpen}
        onCancel={() => setInPlaceOpen(false)}
        onOk={() => inPlaceForm.submit()}
        confirmLoading={submitting}
        okButtonProps={{ danger: true }}
        width={720}
      >
        <Alert type="error" showIcon icon={<ExclamationCircleOutlined />} style={{ marginBottom: 16 }} message="升级任务会影响实例或集群可用性，必须先完成备份并有回滚方案。" />
        <Form form={inPlaceForm} layout="vertical" onFinish={executeUpgrade} initialValues={{ backup_enabled: false, strategy: 'inplace', parallelism: 4, batch_size: 1000, max_in_parallel: 1, health_check_interval: 30 }}>
          <Form.Item name="strategy" label="升级策略" rules={[{ required: true }]}>
            <Select options={strategyOptions} />
          </Form.Item>
          {executeStrategy === 'rolling' ? (
            <Form.Item name="cluster_id" label="目标集群" rules={[{ required: true, message: '请选择集群' }]}>
              <Select showSearch optionFilterProp="label" options={clusterOptions} placeholder="选择集群" />
            </Form.Item>
          ) : (
            <>
              <Form.Item name="instance_id" label="目标实例" rules={[{ required: true, message: '请选择实例' }]}>
                <Select showSearch optionFilterProp="label" options={instanceOptions} placeholder="选择实例" />
              </Form.Item>
              {versionInfo(inPlaceInstanceId)}
            </>
          )}
          <Form.Item name="plan_id" label="升级计划ID" rules={[{ required: true, message: '请输入规划后生成的计划ID' }]}>
            <Input placeholder="先规划升级路径，再填写计划ID" />
          </Form.Item>
          <Form.Item name="target_version" label="目标版本" rules={[{ required: true, message: '请选择目标版本' }]}>
            <Select showSearch optionFilterProp="label" loading={versionsLoading} options={versionOptions} placeholder="选择目标版本" />
          </Form.Item>
          <Form.Item name="backup_enabled" label="数据是否已备份" valuePropName="checked">
            <Switch checkedChildren="已备份" unCheckedChildren="未备份" />
          </Form.Item>
          {executeStrategy === 'logical' && (
            <>
              <Form.Item name="parallelism" label="并行度">
                <InputNumber min={1} max={32} style={{ width: '100%' }} />
              </Form.Item>
              <Form.Item name="batch_size" label="批次大小">
                <InputNumber min={100} max={100000} style={{ width: '100%' }} />
              </Form.Item>
            </>
          )}
          {executeStrategy === 'rolling' ? (
            <>
              <Form.Item name="max_in_parallel" label="最大并行实例数">
                <InputNumber min={1} max={8} style={{ width: '100%' }} />
              </Form.Item>
              <Form.Item name="health_check_interval" label="健康检查间隔(秒)">
                <InputNumber min={5} max={600} style={{ width: '100%' }} />
              </Form.Item>
            </>
          ) : (
            <Form.Item name="stop_app_timeout" label="停止应用超时(秒)" initialValue={300}>
              <InputNumber min={30} max={3600} style={{ width: '100%' }} />
            </Form.Item>
          )}
        </Form>
      </Modal>
      <Modal
        title="升级报告"
        open={reportOpen}
        onCancel={() => setReportOpen(false)}
        footer={<Button onClick={() => setReportOpen(false)}>关闭</Button>}
        width={760}
      >
        <Spin spinning={reportLoading}>
          {reportResult ? (
            <Space direction="vertical" style={{ width: '100%' }} size="middle">
              <Descriptions size="small" bordered column={1}>
                <Descriptions.Item label="报告 ID">{reportResult.report_id || '-'}</Descriptions.Item>
                <Descriptions.Item label="任务 / 计划 ID">{reportResult.plan_id || '-'}</Descriptions.Item>
                <Descriptions.Item label="生成时间">{reportResult.generated_at || '-'}</Descriptions.Item>
                <Descriptions.Item label="摘要">{reportResult.summary || '-'}</Descriptions.Item>
                <Descriptions.Item label="详情">{reportResult.details || '-'}</Descriptions.Item>
              </Descriptions>
              <Descriptions size="small" bordered column={2} title="指标">
                <Descriptions.Item label="耗时(秒)">{reportResult.metrics?.duration_seconds ?? 0}</Descriptions.Item>
                <Descriptions.Item label="错误">{reportResult.metrics?.errors_encountered ?? 0}</Descriptions.Item>
                <Descriptions.Item label="警告">{reportResult.metrics?.warnings_generated ?? 0}</Descriptions.Item>
                <Descriptions.Item label="表数量">{reportResult.metrics?.tables_processed ?? 0}</Descriptions.Item>
              </Descriptions>
              {(reportResult.issues || []).map((item: any, index: number) => (
                <Alert
                  key={index}
                  type={item.severity === 'error' ? 'error' : 'warning'}
                  message={`${item.type || '问题'}: ${item.description || '-'}`}
                  description={`是否解决=${item.resolved ? '是' : '否'} ${item.timestamp || ''}`}
                />
              ))}
              {(reportResult.recommendations || []).length > 0 && (
                <Card size="small" title="建议">
                  {(reportResult.recommendations || []).map((item: string, index: number) => (
                    <Paragraph key={index}>{item}</Paragraph>
                  ))}
                </Card>
              )}
            </Space>
          ) : (
            <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} />
          )}
        </Spin>
      </Modal>
    </div>
  )
}

export default UpgradeManage
