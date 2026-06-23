import React, { useEffect, useState } from 'react'
import { Card, Form, Select, Button, Space, Table, message, Tag, Descriptions, Input, InputNumber, Progress, Steps, Divider, Tabs, Modal, Row, Col } from 'antd'
import { PlayCircleOutlined, CheckCircleOutlined, SwapOutlined, SyncOutlined, StopOutlined } from '@ant-design/icons'
import { migrationApi, instanceApi } from '../services/api'

interface MigrationTask {
  id: string
  migration_type?: 'physical' | 'replication' | 'gtid'
  strategy?: 'physical' | 'replication' | 'gtid'
  source_instance?: string
  target_instance?: string
  source_instance_id?: string
  target_instance_id?: string
  status: 'pending' | 'preparing' | 'migrating' | 'running' | 'verifying' | 'switching' | 'completed' | 'failed' | 'cancelled'
  progress: number
  started_at: string
  completed_at?: string
  error?: string
  error_message?: string
  steps?: Array<{ name: string; status: string; message?: string; started_at?: string; completed_at?: string }>
  logs?: string[]
}

const MIGRATION_SUBSTEPS: Record<string, string[]> = {
  '数据导出': ['锁定源表', '执行 mysqldump/xtrabackup', '生成校验和', '记录导出位置'],
  '数据传输': ['建立目标连接', '传输数据文件', '验证传输完整性'],
  '数据导入': ['准备目标实例', '导入数据文件', '重建索引', '更新系统表'],
  '一致性校验': ['表行数对比', 'CRC32 校验', 'GTID 一致性检查'],
  '切换': ['停止源实例写入', '等待目标追上', '切换业务连接', '验证新主可用'],
}

const activeMigrationStatuses = new Set(['pending', 'preparing', 'migrating', 'running', 'verifying', 'switching'])

const isActiveMigrationStatus = (status?: string) => activeMigrationStatuses.has((status || '').toLowerCase())

const isFailedMigrationStatus = (status?: string) => {
  const normalized = (status || '').toLowerCase()
  return ['failed', 'error', 'timeout', 'cancelled', 'canceled'].includes(normalized)
}

const isCompletedMigrationStatus = (status?: string) => {
  const normalized = (status || '').toLowerCase()
  return ['completed', 'success', 'succeeded', 'ok'].includes(normalized)
}

const migrationProgressStatus = (status?: string) => {
  if (isCompletedMigrationStatus(status)) return 'success'
  if (isFailedMigrationStatus(status)) return 'exception'
  if (isActiveMigrationStatus(status)) return 'active'
  return 'normal'
}

const migrationStatusColor = (status?: string) => {
  const normalized = (status || '').toLowerCase()
  if (isCompletedMigrationStatus(normalized)) return 'success'
  if (isFailedMigrationStatus(normalized)) return 'error'
  if (normalized === 'verifying' || normalized === 'switching') return 'warning'
  if (isActiveMigrationStatus(normalized)) return 'processing'
  return 'default'
}

const showMigrationStartResult = (label: string, task: MigrationTask) => {
  const detail = task.error || task.error_message || task.status
  if (isFailedMigrationStatus(task.status)) {
    message.error(`${label}任务启动失败: ${detail}`)
  } else if (isCompletedMigrationStatus(task.status)) {
    message.success(`${label}任务已完成`)
  } else {
    message.success(`${label}任务已启动`)
  }
}
interface MigrationProgressStep {
  stage: string
  progress: number
  details: string
}

interface MigrationProgressResponse {
  task_id: string
  status: string
  progress: number
  current_step?: string
  total_steps?: number
  completed_steps?: number
  data_transferred?: number
  estimated_time?: number
  updated_at?: string
  steps?: Array<{ name: string; status: string; message?: string; started_at?: string; completed_at?: string }>
  logs?: string[]
}

const formatBytes = (value?: number) => {
  if (!value || value <= 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let size = value
  let index = 0
  while (size >= 1024 && index < units.length - 1) {
    size /= 1024
    index += 1
  }
  return `${size.toFixed(index === 0 ? 0 : 1)} ${units[index]}`
}

const buildProgressDetails = (progress: MigrationProgressResponse): MigrationProgressStep[] => {
  const totalSteps = progress.total_steps || 0
  const completedSteps = progress.completed_steps || 0
  const stepPercent = totalSteps > 0 ? Math.round((completedSteps / totalSteps) * 100) : progress.progress || 0
  return [
    {
      stage: progress.current_step || '迁移执行',
      progress: progress.progress || 0,
      details: `状态: ${progress.status || 'unknown'}`,
    },
    {
      stage: '阶段完成度',
      progress: stepPercent,
      details: totalSteps > 0 ? `${completedSteps}/${totalSteps} 个阶段已完成` : '等待后端返回阶段数量',
    },
    {
      stage: '数据传输',
      progress: progress.progress || 0,
      details: `已传输 ${formatBytes(progress.data_transferred)}${progress.estimated_time ? `, 预计剩余 ${progress.estimated_time}s` : ''}`,
    },
  ]
}

const PhysicalFormSection: React.FC<{
  instances: any[]
  loading: boolean
  onSubmit: (values: any) => void
}> = ({ instances, loading, onSubmit }) => {
  const [form] = Form.useForm()
  return (
    <Form form={form} layout="vertical" onFinish={onSubmit}>
      <Row gutter={16}>
        <Col xs={24} md={8}>
          <Form.Item name="source_instance" label="源实例" rules={[{ required: true, message: '请选择源实例' }]}>
            <Select placeholder="选择源实例" options={instances.map((i: any) => ({ label: i.name, value: i.id }))} />
          </Form.Item>
        </Col>
        <Col xs={24} md={8}>
          <Form.Item name="target_instance" label="目标实例" rules={[{ required: true, message: '请选择目标实例' }]}>
            <Select placeholder="选择目标实例" options={instances.map((i: any) => ({ label: i.name, value: i.id }))} />
          </Form.Item>
        </Col>
        <Col xs={24} md={8}>
          <Form.Item name="compress" label="压缩方式" initialValue="gzip">
            <Select>
              <Select.Option value="gzip">gzip</Select.Option>
              <Select.Option value="lz4">lz4</Select.Option>
              <Select.Option value="none">不压缩</Select.Option>
            </Select>
          </Form.Item>
        </Col>
        <Col xs={24} md={8}>
          <Form.Item name="parallel_threads" label="并行线程数" initialValue={4}>
            <InputNumber min={1} max={16} style={{ width: '100%' }} />
          </Form.Item>
        </Col>
        <Col xs={24} md={8}>
          <Form.Item label=" ">
            <Button type="primary" icon={<PlayCircleOutlined />} htmlType="submit" loading={loading}>启动物理迁移</Button>
          </Form.Item>
        </Col>
      </Row>
    </Form>
  )
}

const ReplicationFormSection: React.FC<{
  instances: any[]
  loading: boolean
  onSubmit: (values: any) => void
}> = ({ instances, loading, onSubmit }) => {
  const [form] = Form.useForm()
  return (
    <Form form={form} layout="vertical" onFinish={onSubmit}>
      <Row gutter={16}>
        <Col xs={24} md={8}>
          <Form.Item name="source_instance" label="源实例" rules={[{ required: true, message: '请选择源实例' }]}>
            <Select placeholder="选择源实例" options={instances.map((i: any) => ({ label: i.name, value: i.id }))} />
          </Form.Item>
        </Col>
        <Col xs={24} md={8}>
          <Form.Item name="target_instance" label="目标实例" rules={[{ required: true, message: '请选择目标实例' }]}>
            <Select placeholder="选择目标实例" options={instances.map((i: any) => ({ label: i.name, value: i.id }))} />
          </Form.Item>
        </Col>
        <Col xs={24} md={8}>
          <Form.Item name="replication_user" label="复制用户" rules={[{ required: true }]}>
            <Input placeholder="repl_user" />
          </Form.Item>
        </Col>
        <Col xs={24} md={8}>
          <Form.Item name="replication_password" label="复制密码" rules={[{ required: true }]}>
            <Input.Password placeholder="输入密码" />
          </Form.Item>
        </Col>
        <Col xs={24} md={8}>
          <Form.Item name="sync_delay_threshold" label="延迟阈值" initialValue={10}>
            <InputNumber min={0} max={3600} style={{ width: '100%' }} />
          </Form.Item>
        </Col>
        <Col xs={24} md={8}>
          <Form.Item label=" ">
            <Button type="primary" icon={<SyncOutlined />} htmlType="submit" loading={loading}>启动复制迁移</Button>
          </Form.Item>
        </Col>
      </Row>
    </Form>
  )
}

const GTIDFormSection: React.FC<{
  instances: any[]
  loading: boolean
  onSubmit: (values: any) => void
}> = ({ instances, loading, onSubmit }) => {
  const [form] = Form.useForm()
  return (
    <Form form={form} layout="vertical" onFinish={onSubmit}>
      <Row gutter={16}>
        <Col xs={24} md={8}>
          <Form.Item name="source_instance" label="源实例" rules={[{ required: true, message: '请选择源实例' }]}>
            <Select placeholder="选择源实例" options={instances.map((i: any) => ({ label: i.name, value: i.id }))} />
          </Form.Item>
        </Col>
        <Col xs={24} md={8}>
          <Form.Item name="target_instance" label="目标实例" rules={[{ required: true, message: '请选择目标实例' }]}>
            <Select placeholder="选择目标实例" options={instances.map((i: any) => ({ label: i.name, value: i.id }))} />
          </Form.Item>
        </Col>
        <Col xs={24} md={8}>
          <Form.Item name="gtid_purged" label="清除GTID">
            <Input placeholder="GTID集合(可选)" />
          </Form.Item>
        </Col>
        <Col xs={24} md={8}>
          <Form.Item name="gtid_executed" label="执行GTID">
            <Input placeholder="GTID集合(可选)" />
          </Form.Item>
        </Col>
        <Col xs={24} md={8}>
          <Form.Item name="transaction_batch_size" label="事务批次" initialValue={100}>
            <InputNumber min={10} max={10000} style={{ width: '100%' }} />
          </Form.Item>
        </Col>
        <Col xs={24} md={8}>
          <Form.Item label=" ">
            <Button type="primary" icon={<PlayCircleOutlined />} htmlType="submit" loading={loading}>启动GTID迁移</Button>
          </Form.Item>
        </Col>
      </Row>
    </Form>
  )
}

const MigrationManage: React.FC = () => {
  const [migrationTasks, setMigrationTasks] = useState<MigrationTask[]>([])
  const [instances, setInstances] = useState<any[]>([])
  const [loading, setLoading] = useState(false)
  const [currentTab, setCurrentTab] = useState('physical')
  const [activeMigration, setActiveMigration] = useState<MigrationTask | null>(null)
  const [progressDetails, setProgressDetails] = useState<MigrationProgressStep[]>([])

  const loadData = async () => {
    try {
      const [instanceRes, migrationRes]: any[] = await Promise.all([
        instanceApi.list(100, 0),
        migrationApi.list(),
      ])
      setInstances(instanceRes?.data || [])
      setMigrationTasks(migrationRes?.data || [])
    } catch (err: any) {
      message.error(err?.response?.data?.message || err?.message || '加载迁移数据失败')
    }
  }

  useEffect(() => {
    loadData()
  }, [])

  useEffect(() => {
    if (!activeMigration) return
    if (!isActiveMigrationStatus(activeMigration.status)) return
    const interval = setInterval(() => {
      Promise.allSettled([
        migrationApi.get(activeMigration.id),
        migrationApi.getProgress(activeMigration.id),
      ]).then(([taskResult, progressResult]) => {
        const task = taskResult.status === 'fulfilled' ? taskResult.value?.data : null
        const progress: MigrationProgressResponse | null = progressResult.status === 'fulfilled' ? progressResult.value?.data : null
        const merged = {
          ...(task || {}),
          ...(progress ? { status: progress.status, progress: progress.progress } : {}),
          id: task?.id || progress?.task_id || activeMigration.id,
          steps: Array.isArray(task?.steps) ? task.steps : (Array.isArray(progress?.steps) ? progress.steps : undefined),
          logs: Array.isArray(task?.logs) ? task.logs : (Array.isArray(progress?.logs) ? progress.logs : undefined),
        }
        if (progress) {
          setProgressDetails(buildProgressDetails(progress))
        }
        setActiveMigration((prev) => (prev ? { ...prev, ...merged } : prev))
        setMigrationTasks((tasks) => tasks.map((t) => (t.id === merged.id ? { ...t, ...merged } : t)))
        if (!isActiveMigrationStatus(merged.status)) {
          clearInterval(interval)
        }
      }).catch(() => clearInterval(interval))
    }, 2000)
    return () => clearInterval(interval)
  }, [activeMigration?.id, activeMigration?.status])

  useEffect(() => {
    if (!migrationTasks.some((task) => isActiveMigrationStatus(task.status))) return
    const interval = setInterval(loadData, 5000)
    return () => clearInterval(interval)
  }, [migrationTasks])

  const buildCreatePayload = (values: any, strategy: 'physical' | 'replication' | 'gtid') => ({
    name: `${strategy}-${Date.now()}`,
    source_instance_id: values.source_instance,
    target_instance_id: values.target_instance,
    strategy,
    config: JSON.stringify(values),
  })

  const taskFromResult = (values: any, strategy: 'physical' | 'replication' | 'gtid', res: any): MigrationTask | null => {
    const taskId = res?.data?.task_id || res?.data?.id
    if (!taskId) return null
    return {
      id: taskId,
      migration_type: strategy,
      strategy,
      source_instance: values.source_instance,
      target_instance: values.target_instance,
      source_instance_id: values.source_instance,
      target_instance_id: values.target_instance,
      status: res?.data?.status || 'migrating',
      progress: typeof res?.data?.progress === 'number' ? res.data.progress : 0,
      started_at: res?.data?.started_at || new Date().toISOString(),
    }
  }

  const handlePhysicalMigration = async (values: any) => {
    setLoading(true)
    try {
      // F2: 后端失败时直接 message.error + return, 不再塞假 task 进列表
      const res: any = await migrationApi.createPhysical(buildCreatePayload(values, 'physical'))
      const task = taskFromResult(values, 'physical', res)
      if (!task) throw new Error('migration API did not return task_id')
      await loadData()
      setActiveMigration(task)
      showMigrationStartResult('物理迁移', task)
      setProgressDetails([
        { stage: '数据导出', progress: 0, details: '准备中...' },
        { stage: '数据传输', progress: 0, details: '等待中...' },
        { stage: '数据导入', progress: 0, details: '等待中...' },
      ])
    } catch (err: any) {
      message.error('启动物理迁移失败: ' + (err?.response?.data?.message || err?.message || '未知错误'))
    } finally {
      setLoading(false)
    }
  }

  const handleReplicationMigration = async (values: any) => {
    setLoading(true)
    try {
      // F2: 同上, 不再吞错
      const res: any = await migrationApi.createReplication(buildCreatePayload(values, 'replication'))
      const task = taskFromResult(values, 'replication', res)
      if (!task) throw new Error('migration API did not return task_id')
      await loadData()
      setActiveMigration(task)
      showMigrationStartResult('复制迁移', task)
      setProgressDetails([
        { stage: '建立复制', progress: 0, details: '准备中...' },
        { stage: '数据同步', progress: 0, details: '等待中...' },
        { stage: '一致性校验', progress: 0, details: '等待中...' },
      ])
    } catch (err: any) {
      message.error('启动复制迁移失败: ' + (err?.response?.data?.message || err?.message || '未知错误'))
    } finally {
      setLoading(false)
    }
  }

  const handleGTIDMigration = async (values: any) => {
    setLoading(true)
    try {
      // F2: 同上
      const res: any = await migrationApi.createGTID(buildCreatePayload(values, 'gtid'))
      const task = taskFromResult(values, 'gtid', res)
      if (!task) throw new Error('migration API did not return task_id')
      await loadData()
      setActiveMigration(task)
      showMigrationStartResult('GTID迁移', task)
      setProgressDetails([
        { stage: 'GTID解析', progress: 0, details: '准备中...' },
        { stage: '事务应用', progress: 0, details: '等待中...' },
        { stage: '数据校验', progress: 0, details: '等待中...' },
      ])
    } catch (err: any) {
      message.error('启动GTID迁移失败: ' + (err?.response?.data?.message || err?.message || '未知错误'))
    } finally {
      setLoading(false)
    }
  }

  const handleVerify = async (taskId: string) => {
    message.info(`开始验证迁移任务: ${taskId}`)
    try {
      const res: any = await migrationApi.verify(taskId)
      const errors = res?.data?.errors || []
      message[errors.length > 0 ? 'warning' : 'success'](errors.length > 0 ? '迁移验证完成，但存在错误' : '迁移验证通过')
      await loadData()
    } catch (err: any) {
      message.error(err?.response?.data?.message || err?.message || '迁移验证失败')
    }
  }

  const handleSwitch = async (taskId: string) => {
    Modal.confirm({
      title: '\u786e\u8ba4\u5207\u6362',
      content: '\u5207\u6362\u64cd\u4f5c\u4f1a\u628a\u4e1a\u52a1\u6d41\u91cf\u5207\u5230\u76ee\u6807\u5b9e\u4f8b\uff0c\u53ef\u80fd\u5bfc\u81f4\u77ed\u6682\u4e0d\u53ef\u7528\uff0c\u8bf7\u786e\u8ba4\u5df2\u901a\u77e5\u4e1a\u52a1\u65b9\u3002',
      okText: '\u786e\u8ba4\u5207\u6362',
      okButtonProps: { danger: true },
      onOk: async () => {
        try {
          const res: any = await migrationApi.switchover(taskId)
          const data = res?.data || res
          await loadData()
          if (isFailedMigrationStatus(data?.status)) {
            message.error(data?.message || '\u8fc1\u79fb\u5207\u6362\u5931\u8d25')
          } else if (isCompletedMigrationStatus(data?.status)) {
            message.success(data?.message || '\u8fc1\u79fb\u5207\u6362\u5b8c\u6210')
          } else {
            message.info(data?.message || '\u8fc1\u79fb\u5207\u6362\u5df2\u63d0\u4ea4\uff0c\u8bf7\u5237\u65b0\u4efb\u52a1\u72b6\u6001')
          }
        } catch (err: any) {
          if (err?.response?.status === 404) {
            message.error('\u8fc1\u79fb\u5207\u6362\u63a5\u53e3\u4e0d\u5b58\u5728\u6216\u4efb\u52a1\u4e0d\u5b58\u5728')
            return
          }
          message.error(err?.response?.data?.message || '\u5207\u6362\u5931\u8d25')
        }
      },
    })
  }
  const handleCancel = (taskId: string) => {
    Modal.confirm({
      title: '确认取消迁移',
      content: '取消后, 已传输的数据不会自动回滚, 需手动清理。继续吗?',
      okText: '确认取消',
      okButtonProps: { danger: true },
      onOk: async () => {
        try {
          await migrationApi.cancel(taskId)
          await loadData()
          message.success('已取消迁移')
          return;
        } catch (err: any) {
          if (err?.response?.status === 404) {
            message.error('迁移取消接口不存在或任务不存在')
            return;
          } else {
            message.error(err?.response?.data?.message || '取消失败')
          }
        }
      },
    })
  }

  const renderProgressMonitor = () => (
    activeMigration && (
      <Card title="迁移进度监控" style={{ marginTop: 16 }}>
        <Descriptions column={2} bordered>
          <Descriptions.Item label="任务ID">{activeMigration.id}</Descriptions.Item>
          <Descriptions.Item label="迁移类型">
            <Tag color="blue">{activeMigration.migration_type || activeMigration.strategy}</Tag>
          </Descriptions.Item>
          <Descriptions.Item label="源实例">{activeMigration.source_instance || activeMigration.source_instance_id}</Descriptions.Item>
          <Descriptions.Item label="目标实例">{activeMigration.target_instance || activeMigration.target_instance_id}</Descriptions.Item>
          <Descriptions.Item label="状态">
            <Tag color={migrationStatusColor(activeMigration.status)}>
              {activeMigration.status}
            </Tag>
          </Descriptions.Item>
          <Descriptions.Item label="开始时间">{activeMigration.started_at}</Descriptions.Item>
          {(activeMigration.error || activeMigration.error_message) && (
            <Descriptions.Item label="错误信息" span={2}>{activeMigration.error || activeMigration.error_message}</Descriptions.Item>
          )}
        </Descriptions>
        
        <Divider />
        
        <div style={{ marginBottom: 8 }}>
          <strong>总体进度</strong>
        </div>
        <Progress percent={activeMigration.progress} status={migrationProgressStatus(activeMigration.status)} />
        
        <Divider />

        {activeMigration.steps && activeMigration.steps.length > 0 ? (
          <div>
            <strong>详细步骤</strong>
            <Steps direction="vertical" size="small" style={{ marginTop: 8 }} current={activeMigration.steps.findIndex((s) => s.status === 'running')}>
              {activeMigration.steps.map((step, idx) => (
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
        ) : (
          <div>
            <strong>迁移阶段</strong>
            <Steps current={-1} direction="vertical" style={{ marginTop: 8 }}>
              {progressDetails.map((item, index) => (
                <Steps.Step
                  key={index}
                  title={item.stage}
                  description={
                    <div>
                      <Progress percent={item.progress} size="small" />
                      <span>{item.details}</span>
                    </div>
                  }
                />
              ))}
            </Steps>
          </div>
        )}

        <Divider />
        <strong>当前阶段子步骤</strong>
        <div style={{ marginTop: 8 }}>
          {(MIGRATION_SUBSTEPS[activeMigration.status === 'verifying' ? '一致性校验' : activeMigration.status === 'switching' ? '切换' : '数据导出'] || MIGRATION_SUBSTEPS['数据导出']).map((substep, idx) => (
            <div key={idx} style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 4 }}>
              {idx < ((MIGRATION_SUBSTEPS[activeMigration.status === 'verifying' ? '一致性校验' : activeMigration.status === 'switching' ? '切换' : '数据导出'] || MIGRATION_SUBSTEPS['数据导出']).length) - 1 ?
                <span style={{ color: '#52c41a' }}>&#10003;</span> :
                <span style={{ color: '#1677ff' }}>&#9679;</span>
              }
              <span>{substep}</span>
            </div>
          ))}
        </div>

        {activeMigration.logs && activeMigration.logs.length > 0 && (
          <>
            <Divider />
            <strong>迁移日志</strong>
            <div style={{ background: '#1e1e1e', color: '#d4d4d4', padding: 12, borderRadius: 6, maxHeight: 200, overflow: 'auto', fontFamily: 'monospace', fontSize: 12, marginTop: 8 }}>
              {activeMigration.logs.map((log, idx) => (
                <div key={idx}>{log}</div>
              ))}
            </div>
          </>
        )}
      </Card>
    )
  )

  const columns = [
    {
      title: '任务ID',
      dataIndex: 'id',
      key: 'id',
    },
    {
      title: '错误信息',
      key: 'error',
      width: 260,
      ellipsis: true,
      render: (_: any, record: MigrationTask) => record.error || record.error_message || '-',
    },
    {
      title: '迁移类型',
      key: 'migration_type',
      render: (_: any, record: MigrationTask) => {
        const type = record.migration_type || record.strategy || ''
        const typeMap: Record<string, { color: string; text: string }> = {
          physical: { color: 'blue', text: '物理迁移' },
          replication: { color: 'green', text: '复制迁移' },
          gtid: { color: 'orange', text: 'GTID迁移' },
        }
        return <Tag color={typeMap[type]?.color}>{typeMap[type]?.text}</Tag>
      },
    },
    {
      title: '源实例',
      key: 'source_instance',
      render: (_: any, record: MigrationTask) => record.source_instance || record.source_instance_id,
    },
    {
      title: '目标实例',
      key: 'target_instance',
      render: (_: any, record: MigrationTask) => record.target_instance || record.target_instance_id,
    },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      render: (status: string) => {
        return <Tag color={migrationStatusColor(status)}>{status}</Tag>
      },
    },
    {
      title: '进度',
      dataIndex: 'progress',
      key: 'progress',
      render: (progress: number) => <Progress percent={progress} size="small" />,
    },
    {
      title: '开始时间',
      dataIndex: 'started_at',
      key: 'started_at',
    },
    {
      title: '操作',
      key: 'action',
      render: (_: any, record: MigrationTask) => (
        <Space>
          <Button
            size="small"
            icon={<CheckCircleOutlined />}
            onClick={() => handleVerify(record.id)}
            disabled={!['running', 'migrating'].includes((record.status || '').toLowerCase())}
          >
            验证
          </Button>
          <Button
            size="small"
            type="primary"
            icon={<SwapOutlined />}
            onClick={() => handleSwitch(record.id)}
            disabled={(record.status || '').toLowerCase() !== 'verifying'}
          >
            切换
          </Button>
          {isActiveMigrationStatus(record.status) && (
            <Button
              size="small"
              danger
              icon={<StopOutlined />}
              onClick={() => handleCancel(record.id)}
            >
              取消
            </Button>
          )}
        </Space>
      ),
    },
  ]

  return (
    <div style={{ padding: '24px' }}>
      <Card title="数据迁移管理">
      <Tabs
        activeKey={currentTab}
        onChange={setCurrentTab}
        items={[
          { key: 'physical', label: '物理迁移', children: <PhysicalFormSection instances={instances} loading={loading} onSubmit={handlePhysicalMigration} /> },
          { key: 'replication', label: '复制迁移', children: <ReplicationFormSection instances={instances} loading={loading} onSubmit={handleReplicationMigration} /> },
          { key: 'gtid', label: 'GTID迁移', children: <GTIDFormSection instances={instances} loading={loading} onSubmit={handleGTIDMigration} /> },
        ]}
      />

      {renderProgressMonitor()}

      <Divider />

      <Table
        columns={columns}
        dataSource={migrationTasks}
        rowKey="id"
        loading={loading}
        style={{ marginTop: 16 }}
      />
    </Card>
    </div>
  )
}

export default MigrationManage
