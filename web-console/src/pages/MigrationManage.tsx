import React, { useState } from 'react'
import { Card, Form, Select, Button, Space, Table, message, Tag, Descriptions, Input, InputNumber, Progress, Steps, Divider, Tabs, Alert } from 'antd'
import { PlayCircleOutlined, CheckCircleOutlined, SwapOutlined, SyncOutlined } from '@ant-design/icons'

interface MigrationTask {
  task_id: string
  migration_type: 'physical' | 'replication' | 'gtid'
  source_instance: string
  target_instance: string
  status: 'pending' | 'running' | 'verifying' | 'completed' | 'failed'
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

const MigrationManage: React.FC = () => {
  const [form] = Form.useForm()
  const [gtidForm] = Form.useForm()
  const [replicationForm] = Form.useForm()
  const [migrationTasks, setMigrationTasks] = useState<MigrationTask[]>([])
  const [loading, setLoading] = useState(false)
  const [currentTab, setCurrentTab] = useState('physical')
  const [activeMigration, setActiveMigration] = useState<MigrationTask | null>(null)
  const [progressDetails, setProgressDetails] = useState<MigrationProgress[]>([])

  const handlePhysicalMigration = async (values: any) => {
    setLoading(true)
    try {
      const task: MigrationTask = {
        task_id: `mig-${Date.now()}`,
        migration_type: 'physical',
        source_instance: values.source_instance,
        target_instance: values.target_instance,
        status: 'running',
        progress: 0,
        started_at: new Date().toISOString(),
      }
      setMigrationTasks([task, ...migrationTasks])
      setActiveMigration(task)
      message.success('物理迁移任务已启动')
      
      setProgressDetails([
        { stage: '数据导出', progress: 0, details: '准备中...' },
        { stage: '数据传输', progress: 0, details: '等待中...' },
        { stage: '数据导入', progress: 0, details: '等待中...' },
      ])
    } catch (err) {
      message.error('启动物理迁移失败')
    } finally {
      setLoading(false)
    }
  }

  const handleReplicationMigration = async (values: any) => {
    setLoading(true)
    try {
      const task: MigrationTask = {
        task_id: `mig-${Date.now()}`,
        migration_type: 'replication',
        source_instance: values.source_instance,
        target_instance: values.target_instance,
        status: 'running',
        progress: 0,
        started_at: new Date().toISOString(),
      }
      setMigrationTasks([task, ...migrationTasks])
      setActiveMigration(task)
      message.success('复制迁移任务已启动')
      
      setProgressDetails([
        { stage: '建立复制', progress: 0, details: '准备中...' },
        { stage: '数据同步', progress: 0, details: '等待中...' },
        { stage: '一致性校验', progress: 0, details: '等待中...' },
      ])
    } catch (err) {
      message.error('启动复制迁移失败')
    } finally {
      setLoading(false)
    }
  }

  const handleGTIDMigration = async (values: any) => {
    setLoading(true)
    try {
      const task: MigrationTask = {
        task_id: `mig-${Date.now()}`,
        migration_type: 'gtid',
        source_instance: values.source_instance,
        target_instance: values.target_instance,
        status: 'running',
        progress: 0,
        started_at: new Date().toISOString(),
      }
      setMigrationTasks([task, ...migrationTasks])
      setActiveMigration(task)
      message.success('GTID迁移任务已启动')
      
      setProgressDetails([
        { stage: 'GTID解析', progress: 0, details: '准备中...' },
        { stage: '事务应用', progress: 0, details: '等待中...' },
        { stage: '数据校验', progress: 0, details: '等待中...' },
      ])
    } catch (err) {
      message.error('启动GTID迁移失败')
    } finally {
      setLoading(false)
    }
  }

  const handleVerify = async (taskId: string) => {
    message.info(`开始验证迁移任务: ${taskId}`)
    setMigrationTasks(tasks => 
      tasks.map(t => t.task_id === taskId ? { ...t, status: 'verifying' } : t)
    )
  }

  const handleSwitch = async (taskId: string) => {
    message.info(`开始切换: ${taskId}`)
    setMigrationTasks(tasks => 
      tasks.map(t => t.task_id === taskId ? { ...t, status: 'completed', progress: 100, completed_at: new Date().toISOString() } : t)
    )
    message.success('切换完成')
  }

  const renderPhysicalForm = () => (
    <Form form={form} layout="vertical">
      <Alert
        message="物理迁移说明"
        description="通过物理文件拷贝方式迁移数据，适用于大数据量、快速迁移场景。"
        type="info"
        showIcon
        style={{ marginBottom: 16 }}
      />
      <Form.Item name="source_instance" label="源实例" rules={[{ required: true, message: '请选择源实例' }]}>
        <Select placeholder="选择源实例">
          <Select.Option value="instance-001">instance-001</Select.Option>
          <Select.Option value="instance-002">instance-002</Select.Option>
        </Select>
      </Form.Item>
      <Form.Item name="target_instance" label="目标实例" rules={[{ required: true, message: '请选择目标实例' }]}>
        <Select placeholder="选择目标实例">
          <Select.Option value="instance-003">instance-003</Select.Option>
          <Select.Option value="instance-004">instance-004</Select.Option>
        </Select>
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
        <Button type="primary" icon={<PlayCircleOutlined />} loading={loading} onClick={() => handlePhysicalMigration(form.getFieldsValue())}>
          启动物理迁移
        </Button>
      </Form.Item>
    </Form>
  )

  const renderReplicationForm = () => (
    <Form form={replicationForm} layout="vertical">
      <Alert
        message="复制迁移说明"
        description="通过主从复制方式迁移数据，支持在线迁移、增量同步。"
        type="info"
        showIcon
        style={{ marginBottom: 16 }}
      />
      <Form.Item name="source_instance" label="源实例" rules={[{ required: true, message: '请选择源实例' }]}>
        <Select placeholder="选择源实例">
          <Select.Option value="instance-001">instance-001</Select.Option>
          <Select.Option value="instance-002">instance-002</Select.Option>
        </Select>
      </Form.Item>
      <Form.Item name="target_instance" label="目标实例" rules={[{ required: true, message: '请选择目标实例' }]}>
        <Select placeholder="选择目标实例">
          <Select.Option value="instance-003">instance-003</Select.Option>
          <Select.Option value="instance-004">instance-004</Select.Option>
        </Select>
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
        <Button type="primary" icon={<SyncOutlined />} loading={loading} onClick={() => handleReplicationMigration(replicationForm.getFieldsValue())}>
          启动复制迁移
        </Button>
      </Form.Item>
    </Form>
  )

  const renderGTIDForm = () => (
    <Form form={gtidForm} layout="vertical">
      <Alert
        message="GTID迁移说明"
        description="基于GTID的事务级迁移，支持断点续传、精确一致性。"
        type="info"
        showIcon
        style={{ marginBottom: 16 }}
      />
      <Form.Item name="source_instance" label="源实例" rules={[{ required: true, message: '请选择源实例' }]}>
        <Select placeholder="选择源实例">
          <Select.Option value="instance-001">instance-001</Select.Option>
          <Select.Option value="instance-002">instance-002</Select.Option>
        </Select>
      </Form.Item>
      <Form.Item name="target_instance" label="目标实例" rules={[{ required: true, message: '请选择目标实例' }]}>
        <Select placeholder="选择目标实例">
          <Select.Option value="instance-003">instance-003</Select.Option>
          <Select.Option value="instance-004">instance-004</Select.Option>
        </Select>
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
        <Button type="primary" icon={<PlayCircleOutlined />} loading={loading} onClick={() => handleGTIDMigration(gtidForm.getFieldsValue())}>
          启动GTID迁移
        </Button>
      </Form.Item>
    </Form>
  )

  const renderProgressMonitor = () => (
    activeMigration && (
      <Card title="迁移进度监控" style={{ marginTop: 16 }}>
        <Descriptions column={2} bordered>
          <Descriptions.Item label="任务ID">{activeMigration.task_id}</Descriptions.Item>
          <Descriptions.Item label="迁移类型">
            <Tag color="blue">{activeMigration.migration_type}</Tag>
          </Descriptions.Item>
          <Descriptions.Item label="源实例">{activeMigration.source_instance}</Descriptions.Item>
          <Descriptions.Item label="目标实例">{activeMigration.target_instance}</Descriptions.Item>
          <Descriptions.Item label="状态">
            <Tag color={activeMigration.status === 'running' ? 'processing' : activeMigration.status === 'completed' ? 'success' : 'error'}>
              {activeMigration.status}
            </Tag>
          </Descriptions.Item>
          <Descriptions.Item label="开始时间">{activeMigration.started_at}</Descriptions.Item>
        </Descriptions>
        
        <Divider />
        
        <div style={{ marginBottom: 8 }}>
          <strong>总体进度</strong>
        </div>
        <Progress percent={activeMigration.progress} status={activeMigration.status === 'running' ? 'active' : activeMigration.status === 'completed' ? 'success' : 'exception'} />
        
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
      dataIndex: 'task_id',
      key: 'task_id',
    },
    {
      title: '迁移类型',
      dataIndex: 'migration_type',
      key: 'migration_type',
      render: (type: string) => {
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
      dataIndex: 'source_instance',
      key: 'source_instance',
    },
    {
      title: '目标实例',
      dataIndex: 'target_instance',
      key: 'target_instance',
    },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      render: (status: string) => {
        const colorMap: Record<string, string> = {
          pending: 'default',
          running: 'processing',
          verifying: 'warning',
          completed: 'success',
          failed: 'error',
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
            onClick={() => handleVerify(record.task_id)}
            disabled={record.status !== 'running'}
          >
            Verify
          </Button>
          <Button 
            size="small" 
            type="primary"
            icon={<SwapOutlined />}
            onClick={() => handleSwitch(record.task_id)}
            disabled={record.status !== 'verifying'}
          >
            Switch
          </Button>
        </Space>
      ),
    },
  ]

  return (
    <Card title="数据迁移管理">
      <Tabs activeKey={currentTab} onChange={setCurrentTab}>
        <Tabs.TabPane tab="物理迁移" key="physical">
          {renderPhysicalForm()}
        </Tabs.TabPane>
        <Tabs.TabPane tab="复制迁移" key="replication">
          {renderReplicationForm()}
        </Tabs.TabPane>
        <Tabs.TabPane tab="GTID迁移" key="gtid">
          {renderGTIDForm()}
        </Tabs.TabPane>
      </Tabs>

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
        rowKey="task_id"
        loading={loading}
        style={{ marginTop: 16 }}
      />
    </Card>
  )
}

export default MigrationManage