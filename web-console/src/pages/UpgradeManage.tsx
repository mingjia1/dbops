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
} from '@ant-design/icons'
import { instanceApi, upgradeApi, versionApi, type Instance, type VersionEntry } from '../services/api'

const { Title, Paragraph } = Typography

interface UpgradeHistory {
  id: string
  instance_id: string
  instance_name?: string
  upgrade_type?: string
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
  const [submitting, setSubmitting] = useState(false)
  const [planForm] = Form.useForm()
  const [compatForm] = Form.useForm()
  const [inPlaceForm] = Form.useForm()

  const planInstanceId = Form.useWatch('instance_id', planForm)
  const compatInstanceId = Form.useWatch('instance_id', compatForm)
  const inPlaceInstanceId = Form.useWatch('instance_id', inPlaceForm)

  const loadData = () => {
    upgradeApi.listHistory().then((res: any) => setHistory(res?.data || [])).catch(() => setHistory([]))
    instanceApi.list(1000, 0).then((res: any) => setInstances(res?.data || [])).catch(() => setInstances([]))
    setVersionsLoading(true)
    versionApi.list().then((res: any) => setVersions(res?.data || [])).catch(() => setVersions([])).finally(() => setVersionsLoading(false))
  }

  useEffect(() => {
    loadData()
  }, [])

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

  const executeInPlace = async (values: any) => {
    if (!values.backup_enabled) {
      message.warning('请确认数据已备份后再启动升级')
      return
    }
    setSubmitting(true)
    try {
      const res: any = await upgradeApi.executeInPlace({
        instance_id: values.instance_id,
        plan_id: values.plan_id,
        target_version: values.target_version,
        backup_enabled: !!values.backup_enabled,
      })
      message.success('原地升级任务已提交')
      setInPlaceOpen(false)
      inPlaceForm.resetFields()
      setHistory((items) => [{
        id: res?.data?.task_id || `upgrade-${Date.now()}`,
        instance_id: values.instance_id,
        instance_name: findInstance(values.instance_id)?.name || values.instance_id,
        upgrade_type: 'in_place',
        source_version: detectedVersion(findInstance(values.instance_id)),
        target_version: values.target_version,
        status: res?.data?.status || 'running',
        progress: res?.data?.progress || 0,
        stage: res?.data?.current_step,
        start_time: new Date().toISOString(),
      }, ...items])
    } catch (err: any) {
      message.error(err?.response?.data?.message || err?.message || '升级任务提交失败')
    } finally {
      setSubmitting(false)
    }
  }

  const columns = [
    { title: 'ID', dataIndex: 'id', key: 'id', width: 150, ellipsis: true },
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
      render: (s: string) => <Tag color={s === 'success' || s === 'completed' ? 'success' : s === 'failed' ? 'error' : 'processing'}>{s || '-'}</Tag>,
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
          <Button danger icon={<PlayCircleOutlined />} onClick={() => setInPlaceOpen(true)}>启动原地升级</Button>
          <Button icon={<ReloadOutlined />} onClick={loadData}>刷新</Button>
        </Space>
      </Card>

      <Table columns={columns} dataSource={history} rowKey="id" scroll={{ x: 1000 }} />

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
        title="启动原地升级"
        open={inPlaceOpen}
        onCancel={() => setInPlaceOpen(false)}
        onOk={() => inPlaceForm.submit()}
        confirmLoading={submitting}
        okButtonProps={{ danger: true }}
        width={720}
      >
        <Alert type="error" showIcon icon={<ExclamationCircleOutlined />} style={{ marginBottom: 16 }} message="原地升级会影响实例可用性，必须先完成备份并有回滚方案。" />
        <Form form={inPlaceForm} layout="vertical" onFinish={executeInPlace} initialValues={{ backup_enabled: false }}>
          <Form.Item name="instance_id" label="目标实例" rules={[{ required: true, message: '请选择实例' }]}>
            <Select showSearch optionFilterProp="label" options={instanceOptions} placeholder="选择实例" />
          </Form.Item>
          {versionInfo(inPlaceInstanceId)}
          <Form.Item name="plan_id" label="升级计划ID" rules={[{ required: true, message: '请输入规划后生成的计划ID' }]}>
            <Input placeholder="先规划升级路径，再填写计划ID" />
          </Form.Item>
          <Form.Item name="target_version" label="目标版本" rules={[{ required: true, message: '请选择目标版本' }]}>
            <Select showSearch optionFilterProp="label" loading={versionsLoading} options={versionOptions} placeholder="选择目标版本" />
          </Form.Item>
          <Form.Item name="backup_enabled" label="数据是否已备份" valuePropName="checked">
            <Switch checkedChildren="已备份" unCheckedChildren="未备份" />
          </Form.Item>
          <Form.Item name="stop_app_timeout" label="停止应用超时(秒)" initialValue={300}>
            <InputNumber min={30} max={3600} style={{ width: '100%' }} />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}

export default UpgradeManage
