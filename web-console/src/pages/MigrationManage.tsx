import React, { useEffect, useState } from 'react'
import { Card, Form, Select, Button, Space, Table, message, Tag, Descriptions, Input, InputNumber, Progress, Steps, Divider, Tabs, Alert, Modal } from 'antd'
import { PlayCircleOutlined, CheckCircleOutlined, SwapOutlined, SyncOutlined, StopOutlined, ExclamationCircleOutlined } from '@ant-design/icons'
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
  error_message?: string
}

interface MigrationProgress {
  stage: string
  progress: number
  details: string
}

const PhysicalFormSection: React.FC<{
  instances: any[]
  loading: boolean
  onSubmit: (values: any) => void
}> = ({ instances, loading, onSubmit }) => {
  const [form] = Form.useForm()
  return (
    <Form form={form} layout="vertical" onFinish={onSubmit}>
      <Alert message="物理迁移说明" description="通过物理文件拷贝方式迁移数据，适用于大数据量、快速迁移场景。" type="info" showIcon style={{ marginBottom: 16 }} />
      <Form.Item name="source_instance" label="源实例" rules={[{ required: true, message: '请选择源实例' }]}>
        <Select placeholder="选择源实例" options={instances.map((i: any) => ({ label: i.name, value: i.id }))} />
      </Form.Item>
      <Form.Item name="target_instance" label="目标实例" rules={[{ required: true, message: '请选择目标实例' }]}>
        <Select placeholder="选择目标实例" options={instances.map((i: any) => ({ label: i.name, value: i.id }))} />
      </Form.Item>
      <Form.Item name="compress" label="压缩方式" initialValue="gzip">
        <Select>
          <Select.Option value="gzip">gzip</Select.Option>
          <Select.Option value="lz4">lz4</Select.Option>
          <Select.Option value="none">不压缩</Select.Option>
        </Select>
      </Form.Item>
      <Form.Item name="parallel_threads" label="并行线程数" initialValue={4}>
        <InputNumber min={1} max={16} />
      </Form.Item>
      <Form.Item>
        <Button type="primary" icon={<PlayCircleOutlined />} htmlType="submit" loading={loading}>启动物理迁移</Button>
      </Form.Item>
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
      <Alert message="复制迁移说明" description="通过主从复制方式迁移数据，支持在线迁移、增量同步。" type="info" showIcon style={{ marginBottom: 16 }} />
      <Form.Item name="source_instance" label="源实例" rules={[{ required: true, message: '请选择源实例' }]}>
        <Select placeholder="选择源实例" options={instances.map((i: any) => ({ label: i.name, value: i.id }))} />
      </Form.Item>
      <Form.Item name="target_instance" label="目标实例" rules={[{ required: true, message: '请选择目标实例' }]}>
        <Select placeholder="选择目标实例" options={instances.map((i: any) => ({ label: i.name, value: i.id }))} />
      </Form.Item>
      <Form.Item name="replication_user" label="复制用户" rules={[{ required: true }]}>
        <Input placeholder="repl_user" />
      </Form.Item>
      <Form.Item name="replication_password" label="复制密码" rules={[{ required: true }]}>
        <Input.Password placeholder="输入密码" />
      </Form.Item>
      <Form.Item name="sync_delay_threshold" label="同步延迟阈值(秒)" initialValue={10}>
        <InputNumber min={0} max={3600} />
      </Form.Item>
      <Form.Item>
        <Button type="primary" icon={<SyncOutlined />} htmlType="submit" loading={loading}>启动复制迁移</Button>
      </Form.Item>
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
      <Alert message="GTID迁移说明" description="基于GTID的事务级迁移，支持断点续传、精确一致性。" type="info" showIcon style={{ marginBottom: 16 }} />
      <Form.Item name="source_instance" label="源实例" rules={[{ required: true, message: '请选择源实例' }]}>
        <Select placeholder="选择源实例" options={instances.map((i: any) => ({ label: i.name, value: i.id }))} />
      </Form.Item>
      <Form.Item name="target_instance" label="目标实例" rules={[{ required: true, message: '请选择目标实例' }]}>
        <Select placeholder="选择目标实例" options={instances.map((i: any) => ({ label: i.name, value: i.id }))} />
      </Form.Item>
      <Form.Item name="gtid_purged" label="已清除GTID">
        <Input placeholder="GTID集合(可选)" />
      </Form.Item>
      <Form.Item name="gtid_executed" label="已执行GTID">
        <Input placeholder="GTID集合(可选)" />
      </Form.Item>
      <Form.Item name="transaction_batch_size" label="事务批次大小" initialValue={100}>
        <InputNumber min={10} max={10000} />
      </Form.Item>
      <Form.Item>
        <Button type="primary" icon={<PlayCircleOutlined />} htmlType="submit" loading={loading}>启动GTID迁移</Button>
      </Form.Item>
    </Form>
  )
}

const MigrationManage: React.FC = () => {
  const [migrationTasks, setMigrationTasks] = useState<MigrationTask[]>([])
  const [instances, setInstances] = useState<any[]>([])
  const [loading, setLoading] = useState(false)
  const [currentTab, setCurrentTab] = useState('physical')
  const [activeMigration, setActiveMigration] = useState<MigrationTask | null>(null)
  const [progressDetails, setProgressDetails] = useState<MigrationProgress[]>([])

  useEffect(() => {
    instanceApi.list(100, 0).then((res: any) => {
      setInstances(res?.data || [])
    }).catch(() => {})
    migrationApi.list().then((res: any) => {
      setMigrationTasks(res?.data || [])
    }).catch(() => {})
  }, [])

  useEffect(() => {
    if (!activeMigration) return
    if (!['running', 'migrating', 'preparing'].includes(activeMigration.status)) return
    const interval = setInterval(() => {
      migrationApi.get(activeMigration.id).then((res: any) => {
        const task = res?.data
        if (!task) return
        setActiveMigration((prev) => (prev ? { ...prev, ...task } : prev))
        setMigrationTasks((tasks) => tasks.map((t) => (t.id === task.id ? { ...t, ...task } : t)))
        if (!['running', 'migrating', 'preparing'].includes(task.status)) {
          clearInterval(interval)
        }
      }).catch(() => clearInterval(interval))
    }, 2000)
    return () => clearInterval(interval)
  }, [activeMigration?.id, activeMigration?.status])

  const buildCreatePayload = (values: any, strategy: 'physical' | 'replication' | 'gtid') => ({
    name: `${strategy}-${Date.now()}`,
    source_instance_id: values.source_instance,
    target_instance_id: values.target_instance,
    strategy,
    config: JSON.stringify(values),
  })

  const taskFromResult = (values: any, strategy: 'physical' | 'replication' | 'gtid', res: any): MigrationTask => ({
    id: res?.data?.task_id || res?.data?.id || `mig-${Date.now()}`,
    migration_type: strategy,
    strategy,
    source_instance: values.source_instance,
    target_instance: values.target_instance,
    source_instance_id: values.source_instance,
    target_instance_id: values.target_instance,
    status: res?.data?.status || 'migrating',
    progress: typeof res?.data?.progress === 'number' ? res.data.progress : 0,
    started_at: res?.data?.started_at || new Date().toISOString(),
  })

  const handlePhysicalMigration = async (values: any) => {
    setLoading(true)
    try {
      // F2: 后端失败时直接 message.error + return, 不再塞假 task 进列表
      const res: any = await migrationApi.createPhysical(buildCreatePayload(values, 'physical'))
      const task = taskFromResult(values, 'physical', res)
      setMigrationTasks([task, ...migrationTasks])
      setActiveMigration(task)
      message.success('物理迁移任务已启动')
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
      setMigrationTasks([task, ...migrationTasks])
      setActiveMigration(task)
      message.success('复制迁移任务已启动')
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
      setMigrationTasks([task, ...migrationTasks])
      setActiveMigration(task)
      message.success('GTID迁移任务已启动')
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
    try { await migrationApi.verify(taskId) } catch { /* fallback */ }
    setMigrationTasks(tasks =>
      tasks.map(t => t.id === taskId ? { ...t, status: 'verifying' } : t)
    )
  }

  const handleSwitch = async (taskId: string) => {
    Modal.confirm({
      title: '确认切换',
      content: '切换操作将把业务流量切到目标实例, 会导致短暂不可用, 请确认已通知业务方。',
      okText: '确认切换',
      okButtonProps: { danger: true },
      onOk: async () => {
        try {
          await migrationApi.switchover(taskId)
          setMigrationTasks(tasks =>
            tasks.map(t => t.id === taskId ? { ...t, status: 'completed', progress: 100, completed_at: new Date().toISOString() } : t)
          )
          message.success('切换完成')
        } catch (err: any) {
          if (err?.response?.status === 404) {
            setMigrationTasks(tasks =>
              tasks.map(t => t.id === taskId ? { ...t, status: 'completed', progress: 100, completed_at: new Date().toISOString() } : t)
            )
            message.warning('后端未实现, 已记录本地状态')
          } else {
            message.error(err?.response?.data?.message || '切换失败')
          }
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
          setMigrationTasks(tasks =>
            tasks.map(t => t.id === taskId ? { ...t, status: 'failed', error_message: '已取消' } : t)
          )
          message.success('已取消迁移')
        } catch (err: any) {
          if (err?.response?.status === 404) {
            setMigrationTasks(tasks =>
              tasks.map(t => t.id === taskId ? { ...t, status: 'failed', error_message: '已取消(本地)' } : t)
            )
            message.warning('后端未实现 cancel, 已记录本地状态')
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
            <Tag color={activeMigration.status === 'running' || activeMigration.status === 'migrating' ? 'processing' : activeMigration.status === 'completed' ? 'success' : 'error'}>
              {activeMigration.status}
            </Tag>
          </Descriptions.Item>
          <Descriptions.Item label="开始时间">{activeMigration.started_at}</Descriptions.Item>
        </Descriptions>
        
        <Divider />
        
        <div style={{ marginBottom: 8 }}>
          <strong>总体进度</strong>
        </div>
        <Progress percent={activeMigration.progress} status={activeMigration.status === 'running' || activeMigration.status === 'migrating' ? 'active' : activeMigration.status === 'completed' ? 'success' : 'exception'} />
        
        <Divider />
        
        <Steps current={-1} direction="vertical">
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
        const colorMap: Record<string, string> = {
          pending: 'default',
          running: 'processing',
          preparing: 'processing',
          migrating: 'processing',
          verifying: 'warning',
          switching: 'warning',
          completed: 'success',
          failed: 'error',
          cancelled: 'default',
        }
        return <Tag color={colorMap[status]}>{status}</Tag>
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
            disabled={record.status !== 'running' && record.status !== 'migrating'}
          >
            Verify
          </Button>
          <Button
            size="small"
            type="primary"
            icon={<SwapOutlined />}
            onClick={() => handleSwitch(record.id)}
            disabled={record.status !== 'verifying'}
          >
            Switch
          </Button>
          {(record.status === 'running' || record.status === 'migrating' || record.status === 'pending' || record.status === 'verifying') && (
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
      <Alert
        type="warning"
        showIcon
        icon={<ExclamationCircleOutlined />}
        style={{ marginBottom: 16 }}
        message="迁移注意事项"
        description={
          <ul style={{ marginBottom: 0, paddingLeft: 18 }}>
            <li>迁移会占用源实例 IO, 建议在业务低峰期执行</li>
            <li>Switch 操作将切换业务流量, 不可逆, 需提前通知业务方</li>
            <li>迁移出错时可使用"取消"按钮中止, 但已传输数据需手动清理</li>
          </ul>
        }
      />
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

      <Descriptions title="迁移任务列表" bordered column={1}>
        <Descriptions.Item label="说明">
          查看所有迁移任务的状态和进度，支持验证和切换操作
        </Descriptions.Item>
      </Descriptions>

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
