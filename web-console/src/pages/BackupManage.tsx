import React, { useEffect, useState } from 'react'
import {
  Button,
  Card,
  Col,
  Empty,
  Form,
  Input,
  InputNumber,
  Modal,
  Radio,
  Row,
  Select,
  Space,
  Statistic,
  Switch,
  Table,
  Tabs,
  Tag,
  Tooltip,
  message,
} from 'antd'
import { FileSearchOutlined, PlusOutlined, ReloadOutlined, ScheduleOutlined, ScanOutlined } from '@ant-design/icons'
import type { ColumnsType } from 'antd/es/table'
import { backupApi, instanceApi, type DiscoveredBackup, type Instance } from '../services/api'

interface BackupRecord {
  id: string
  task_id: string
  instance_id: string
  backup_type: string
  status: string
  size: string
  file_path: string
  created_at: string
}

interface BackupPolicy {
  id: string
  instance_id: string
  backup_type: string
  schedule: string
  retention_days: number
  storage_type: string
  storage_path: string
  enabled: boolean
  created_at: string
}

const BackupManage: React.FC = () => {
  const [tab, setTab] = useState('records')
  const [instances, setInstances] = useState<Instance[]>([])
  const [selectedInstance, setSelectedInstance] = useState<string>()
  const [records, setRecords] = useState<BackupRecord[]>([])
  const [policies, setPolicies] = useState<BackupPolicy[]>([])
  const [loading, setLoading] = useState(false)
  const [policyLoading, setPolicyLoading] = useState(false)
  const [scanLoading, setScanLoading] = useState(false)
  const [createOpen, setCreateOpen] = useState(false)
  const [policyOpen, setPolicyOpen] = useState(false)
  const [scanOpen, setScanOpen] = useState(false)
  const [discovered, setDiscovered] = useState<DiscoveredBackup[]>([])
  const [scannedAt, setScannedAt] = useState<string>()
  const [submitting, setSubmitting] = useState(false)
  const [form] = Form.useForm()
  const [policyForm] = Form.useForm()

  useEffect(() => {
    instanceApi.list(100, 0).then((res: any) => setInstances(res?.data || [])).catch(() => setInstances([]))
    fetchPolicies()
  }, [])

  useEffect(() => {
    fetchRecords()
  }, [selectedInstance])

  const fetchRecords = async () => {
    if (!selectedInstance) {
      setRecords([])
      return
    }
    setLoading(true)
    try {
      const res: any = await backupApi.listBackups(selectedInstance)
      setRecords(res?.data || [])
    } catch {
      setRecords([])
    } finally {
      setLoading(false)
    }
  }

  const fetchPolicies = async () => {
    setPolicyLoading(true)
    try {
      const res: any = await backupApi.listPolicies()
      setPolicies(res?.data || [])
    } catch {
      setPolicies([])
    } finally {
      setPolicyLoading(false)
    }
  }

  const submitBackup = async () => {
    if (!selectedInstance) {
      message.warning('请先选择实例')
      return
    }
    try {
      const values = await form.validateFields()
      setSubmitting(true)
      await backupApi.executeBackup(selectedInstance, values.backup_type)
      message.success('备份任务已提交')
      setCreateOpen(false)
      fetchRecords()
    } catch (err: any) {
      message.error(err?.response?.data?.message || '提交备份任务失败')
    } finally {
      setSubmitting(false)
    }
  }

  const scanBackups = async () => {
    if (!selectedInstance) {
      message.warning('请先选择实例')
      return
    }
    setScanLoading(true)
    try {
      const res: any = await backupApi.scan(selectedInstance)
      const data = res?.data || {}
      const list = data.backups || []
      setDiscovered(list)
      setScannedAt(data.scanned_at)
      setScanOpen(true)
      message.success(`扫描完成，发现 ${list.length} 个备份文件`)
    } catch (err: any) {
      message.error(err?.response?.data?.message || '扫描备份失败')
    } finally {
      setScanLoading(false)
    }
  }

  const openCreate = () => {
    if (!selectedInstance) {
      message.warning('请先选择实例')
      return
    }
    form.setFieldsValue({ backup_type: 'full' })
    setCreateOpen(true)
  }

  const openPolicy = () => {
    policyForm.setFieldsValue({
      instance_id: selectedInstance,
      backup_type: 'full',
      schedule: '0 2 * * *',
      retention_days: 7,
      storage_type: 'local',
      storage_path: '/backup/mysql',
      enabled: true,
    })
    setPolicyOpen(true)
  }

  const submitPolicy = async () => {
    try {
      const values = await policyForm.validateFields()
      setSubmitting(true)
      await backupApi.createPolicy(values)
      message.success('备份策略已创建')
      setPolicyOpen(false)
      fetchPolicies()
    } catch (err: any) {
      message.error(err?.response?.data?.message || '创建备份策略失败')
    } finally {
      setSubmitting(false)
    }
  }

  const instanceName = (id: string) => instances.find((item) => item.id === id)?.name || id

  const recordColumns: ColumnsType<BackupRecord> = [
    { title: '实例', dataIndex: 'instance_id', key: 'instance_id', render: instanceName },
    {
      title: '类型',
      dataIndex: 'backup_type',
      key: 'backup_type',
      render: (type) => <BackupTypeTag type={type} />,
    },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      render: (status) => (
        <Tag color={status === 'completed' ? 'success' : status === 'running' ? 'processing' : 'error'}>
          {status === 'completed' ? '已完成' : status === 'running' ? '运行中' : '失败'}
        </Tag>
      ),
    },
    { title: '大小', dataIndex: 'size', key: 'size' },
    {
      title: '路径',
      dataIndex: 'file_path',
      key: 'file_path',
      render: (path) => path ? <Tooltip title={path}><span style={{ fontFamily: 'monospace' }}>{path}</span></Tooltip> : '-',
    },
    { title: '创建时间', dataIndex: 'created_at', key: 'created_at', render: (v) => v ? new Date(v).toLocaleString() : '-' },
  ]

  const policyColumns: ColumnsType<BackupPolicy> = [
    { title: '实例', dataIndex: 'instance_id', key: 'instance_id', render: instanceName },
    { title: '类型', dataIndex: 'backup_type', key: 'backup_type', render: (type) => <BackupTypeTag type={type} /> },
    { title: 'Cron', dataIndex: 'schedule', key: 'schedule' },
    { title: '保留天数', dataIndex: 'retention_days', key: 'retention_days' },
    { title: '存储', dataIndex: 'storage_path', key: 'storage_path' },
    { title: '状态', dataIndex: 'enabled', key: 'enabled', render: (enabled) => <Tag color={enabled ? 'success' : 'default'}>{enabled ? '启用' : '禁用'}</Tag> },
  ]

  return (
    <div style={{ padding: 24 }}>
      <Card>
        <Space style={{ marginBottom: 16 }}>
          <Select
            allowClear
            showSearch
            optionFilterProp="label"
            placeholder="选择实例"
            style={{ width: 260 }}
            value={selectedInstance}
            onChange={setSelectedInstance}
            options={instances.map((item) => ({ value: item.id, label: item.name }))}
          />
          <Button icon={<ReloadOutlined />} onClick={fetchRecords} disabled={!selectedInstance}>
            刷新记录
          </Button>
          <Button icon={<FileSearchOutlined />} onClick={scanBackups} loading={scanLoading} disabled={!selectedInstance}>
            扫描已有备份
          </Button>
          <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>
            创建备份
          </Button>
        </Space>

        <Tabs
          activeKey={tab}
          onChange={setTab}
          items={[
            {
              key: 'records',
              label: '备份记录',
              children: (
                <>
                  <Row gutter={16} style={{ marginBottom: 16 }}>
                    <Col span={6}><Card size="small"><Statistic title="记录数" value={records.length} /></Card></Col>
                    <Col span={6}><Card size="small"><Statistic title="已完成" value={records.filter((r) => r.status === 'completed').length} valueStyle={{ color: '#3f8600' }} /></Card></Col>
                    <Col span={6}><Card size="small"><Statistic title="运行中" value={records.filter((r) => r.status === 'running').length} valueStyle={{ color: '#1677ff' }} /></Card></Col>
                    <Col span={6}><Card size="small"><Statistic title="失败" value={records.filter((r) => r.status === 'failed').length} /></Card></Col>
                  </Row>
                  <Table columns={recordColumns} dataSource={records} rowKey="id" loading={loading} locale={{ emptyText: <Empty description="暂无备份记录" /> }} />
                </>
              ),
            },
            {
              key: 'policies',
              label: '备份策略',
              children: (
                <>
                  <Space style={{ marginBottom: 16 }}>
                    <Button type="primary" icon={<PlusOutlined />} onClick={openPolicy}>
                      新建策略
                    </Button>
                    <Button icon={<ReloadOutlined />} onClick={fetchPolicies}>
                      刷新
                    </Button>
                  </Space>
                  <Table columns={policyColumns} dataSource={policies} rowKey="id" loading={policyLoading} />
                </>
              ),
            },
          ]}
        />
      </Card>

      <Modal
        title="创建备份"
        open={createOpen}
        onCancel={() => setCreateOpen(false)}
        onOk={submitBackup}
        confirmLoading={submitting}
        okText="启动"
        cancelText="取消"
      >
        <Form form={form} layout="vertical">
          <Form.Item label="备份类型" name="backup_type" rules={[{ required: true }]}>
            <Radio.Group>
              <Radio.Button value="full">全量</Radio.Button>
              <Radio.Button value="incremental">增量</Radio.Button>
              <Radio.Button value="logical">逻辑</Radio.Button>
            </Radio.Group>
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        title="新建备份策略"
        open={policyOpen}
        onCancel={() => setPolicyOpen(false)}
        onOk={submitPolicy}
        confirmLoading={submitting}
        width={620}
      >
        <Form form={policyForm} layout="vertical">
          <Form.Item name="instance_id" label="目标实例" rules={[{ required: true }]}>
            <Select options={instances.map((item) => ({ value: item.id, label: item.name }))} />
          </Form.Item>
          <Form.Item name="backup_type" label="备份类型" rules={[{ required: true }]}>
            <Radio.Group>
              <Radio.Button value="full">全量</Radio.Button>
              <Radio.Button value="incremental">增量</Radio.Button>
              <Radio.Button value="logical">逻辑</Radio.Button>
            </Radio.Group>
          </Form.Item>
          <Form.Item name="schedule" label={<span><ScheduleOutlined /> Cron 表达式</span>} rules={[{ required: true }]}>
            <Input placeholder="0 2 * * *" />
          </Form.Item>
          <Form.Item name="retention_days" label="保留天数" rules={[{ required: true }]}>
            <InputNumber min={1} max={3650} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="storage_type" label="存储类型">
            <Radio.Group>
              <Radio.Button value="local">本地</Radio.Button>
              <Radio.Button value="nfs">NFS</Radio.Button>
              <Radio.Button value="s3">S3</Radio.Button>
            </Radio.Group>
          </Form.Item>
          <Form.Item name="storage_path" label="存储路径">
            <Input placeholder="/backup/mysql" />
          </Form.Item>
          <Form.Item name="enabled" label="启用" valuePropName="checked">
            <Switch checkedChildren="启用" unCheckedChildren="禁用" />
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        title="扫描已有备份"
        open={scanOpen}
        onCancel={() => setScanOpen(false)}
        footer={[
          <Button key="close" onClick={() => setScanOpen(false)}>关闭</Button>,
          <Button key="scan" icon={<ScanOutlined />} onClick={scanBackups} loading={scanLoading}>重新扫描</Button>,
        ]}
        width={860}
      >
        {scannedAt && <div style={{ marginBottom: 8, color: '#8c8c8c' }}>扫描时间：{new Date(scannedAt).toLocaleString()}</div>}
        <Table
          rowKey="file_path"
          size="small"
          pagination={{ pageSize: 8 }}
          dataSource={discovered}
          columns={[
            { title: '文件', dataIndex: 'file_name', key: 'file_name', render: (name) => <Tag color="blue">{name}</Tag> },
            { title: '类型', dataIndex: 'backup_type', key: 'backup_type', render: (type) => <BackupTypeTag type={type} /> },
            { title: '大小', dataIndex: 'size_bytes', key: 'size_bytes', render: formatSize },
            { title: '路径', dataIndex: 'file_path', key: 'file_path', render: (path) => <Tooltip title={path}><span style={{ fontFamily: 'monospace' }}>{path}</span></Tooltip> },
            { title: '纳管状态', dataIndex: 'already_managed', key: 'already_managed', render: (managed) => <Tag color={managed ? 'success' : 'warning'}>{managed ? '已纳管' : '未纳管'}</Tag> },
          ]}
        />
      </Modal>
    </div>
  )
}

const BackupTypeTag: React.FC<{ type: string }> = ({ type }) => {
  const text = type === 'full' ? '全量' : type === 'incremental' ? '增量' : type === 'logical' ? '逻辑' : type
  const color = type === 'full' ? 'blue' : type === 'incremental' ? 'green' : 'orange'
  return <Tag color={color}>{text}</Tag>
}

function formatSize(bytes: number): string {
  if (!bytes) return '-'
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  if (bytes < 1024 * 1024 * 1024) return `${(bytes / 1024 / 1024).toFixed(1)} MB`
  return `${(bytes / 1024 / 1024 / 1024).toFixed(2)} GB`
}

export default BackupManage
