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
import { DeleteOutlined, FileSearchOutlined, PlusOutlined, ReloadOutlined, RollbackOutlined, ScheduleOutlined, ScanOutlined } from '@ant-design/icons'
import type { ColumnsType } from 'antd/es/table'
import { backupApi, instanceApi, type Instance } from '../services/api'

interface BackupRecord {
  id: string
  task_id: string
  instance_id: string
  backup_type: string
  status: string
  message?: string
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
const isFailedBackupStatus = (status?: string) => {
  const normalized = (status || '').toLowerCase()
  return ['failed', 'error', 'timeout', 'cancelled', 'canceled'].includes(normalized)
}

const isCompletedBackupStatus = (status?: string) => {
  const normalized = (status || '').toLowerCase()
  return ['completed', 'success', 'succeeded', 'ok'].includes(normalized)
}

const isActiveBackupStatus = (status?: string) => {
  const normalized = (status || '').toLowerCase()
  return ['pending', 'running', 'submitted', 'accepted', 'queued'].includes(normalized)
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
  const [editingPolicy, setEditingPolicy] = useState<BackupPolicy | null>(null)
  const [submitting, setSubmitting] = useState(false)
  const [form] = Form.useForm()
  const [policyForm] = Form.useForm()

  useEffect(() => {
    Promise.all([
      instanceApi.list(100, 0).then((res: any) => setInstances(res?.data || [])).catch(() => setInstances([])),
      fetchPolicies()
    ])
  }, [])

  useEffect(() => {
    fetchRecords()
  }, [selectedInstance])

  useEffect(() => {
    if (!selectedInstance || !records.some((record) => isActiveBackupStatus(record.status))) return
    const timer = window.setInterval(() => fetchRecordsFor(selectedInstance), 5000)
    return () => window.clearInterval(timer)
  }, [selectedInstance, records])

  const fetchRecords = async () => {
    if (!selectedInstance) {
      setRecords([])
      return
    }
    await fetchRecordsFor(selectedInstance)
  }

  const fetchRecordsFor = async (instanceId: string) => {
    setLoading(true)
    try {
      const res: any = await backupApi.listBackups(instanceId)
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
      const res: any = await backupApi.executeBackup(selectedInstance, values.backup_type)
      const data = res?.data || {}
      if (isFailedBackupStatus(data.status)) {
        throw new Error(data.message || '备份任务执行失败')
      }
      if (isCompletedBackupStatus(data.status)) {
        message.success('备份执行完成')
      } else {
        message.success('备份任务已提交，请刷新记录查看状态')
      }
      setCreateOpen(false)
      setTab('records')
      await fetchRecordsFor(selectedInstance)
    } catch (err: any) {
      if (!err?.response?.data?.message && err?.message) {
        message.error(err.message)
        return
      }
      message.error(err?.response?.data?.message || '备份执行失败')
    } finally {
      setSubmitting(false)
    }
  }

  const executePolicy = async (policy: BackupPolicy) => {
    setSubmitting(true)
    setSelectedInstance(policy.instance_id)
    setTab('records')
    try {
      const res: any = await backupApi.executeBackup(policy.instance_id, policy.backup_type, policy.id)
      const data = res?.data || {}
      if (isFailedBackupStatus(data.status)) {
        throw new Error(data.message || '\u5907\u4efd\u4efb\u52a1\u6267\u884c\u5931\u8d25')
      }
      if (isCompletedBackupStatus(data.status)) {
        message.success('\u5907\u4efd\u6267\u884c\u5b8c\u6210')
      } else {
        message.success('\u5907\u4efd\u4efb\u52a1\u5df2\u63d0\u4ea4\uff0c\u8bf7\u5237\u65b0\u8bb0\u5f55\u67e5\u770b\u72b6\u6001')
      }
    } catch (err: any) {
      message.error(err?.response?.data?.message || err?.message || '\u5907\u4efd\u6267\u884c\u5931\u8d25')
    } finally {
      await fetchRecordsFor(policy.instance_id)
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
      await fetchRecordsFor(selectedInstance)
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
    setEditingPolicy(null)
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

  const openEditPolicy = (policy: BackupPolicy) => {
    setEditingPolicy(policy)
    policyForm.setFieldsValue({
      instance_id: policy.instance_id,
      backup_type: policy.backup_type,
      schedule: policy.schedule,
      retention_days: policy.retention_days,
      storage_type: policy.storage_type || 'local',
      storage_path: policy.storage_path,
      enabled: policy.enabled,
    })
    setPolicyOpen(true)
  }

  const submitPolicy = async () => {
    try {
      const values = await policyForm.validateFields()
      setSubmitting(true)
      if (editingPolicy) {
        await backupApi.updatePolicy(editingPolicy.id, values)
        message.success('备份策略已更新')
      } else {
        await backupApi.createPolicy(values)
        message.success('备份策略已创建')
      }
      setPolicyOpen(false)
      setEditingPolicy(null)
      fetchPolicies()
    } catch (err: any) {
      message.error(err?.response?.data?.message || (editingPolicy ? '更新备份策略失败' : '创建备份策略失败'))
    } finally {
      setSubmitting(false)
    }
  }

  const deletePolicy = (policy: BackupPolicy) => {
    Modal.confirm({
      title: '\u786e\u8ba4\u5220\u9664\u5907\u4efd\u7b56\u7565',
      content: '\u4ec5\u5141\u8bb8\u5220\u9664\u5c1a\u672a\u4ea7\u751f\u5907\u4efd\u8bb0\u5f55\u7684\u7b56\u7565\uff1b\u5df2\u6709\u5907\u4efd\u8bb0\u5f55\u7684\u7b56\u7565\u4f1a\u88ab\u540e\u7aef\u62d2\u7edd\u5220\u9664\u3002',
      okText: '\u5220\u9664\u7b56\u7565',
      cancelText: '\u53d6\u6d88',
      okButtonProps: { danger: true },
      onOk: async () => {
        setSubmitting(true)
        try {
          await backupApi.deletePolicy(policy.id)
          message.success('\u5907\u4efd\u7b56\u7565\u5df2\u5220\u9664')
          await fetchPolicies()
        } catch (err: any) {
          message.error(err?.response?.data?.message || err?.message || '\u5220\u9664\u5907\u4efd\u7b56\u7565\u5931\u8d25')
        } finally {
          setSubmitting(false)
        }
      },
    })
  }

  const instanceName = (id: string) => instances.find((item) => item.id === id)?.name || id

  const restoreRecord = (record: BackupRecord) => {
    Modal.confirm({
      title: '\u786e\u8ba4\u6062\u590d\u5907\u4efd',
      content: `\u5c06\u5907\u4efd\u6062\u590d\u5230\u5b9e\u4f8b ${instanceName(record.instance_id)}\uff0c\u8be5\u64cd\u4f5c\u4f1a\u8986\u76d6\u76ee\u6807\u6570\u636e\u3002`,
      okText: '\u786e\u8ba4\u6062\u590d',
      cancelText: '\u53d6\u6d88',
      okButtonProps: { danger: true },
      onOk: async () => {
        setSubmitting(true)
        try {
          const res: any = await backupApi.restore({
            backup_id: record.id,
            target_instance_id: record.instance_id,
            target_type: 'in-place',
            confirm_overwrite: true,
          })
          const data = res?.data || {}
          if (isFailedBackupStatus(data.status)) {
            throw new Error(data.message || '\u6062\u590d\u5931\u8d25')
          }
          message.success(isCompletedBackupStatus(data.status) ? '\u6062\u590d\u5b8c\u6210' : '\u6062\u590d\u4efb\u52a1\u5df2\u63d0\u4ea4')
          await fetchRecordsFor(record.instance_id)
        } catch (err: any) {
          message.error(err?.response?.data?.message || err?.message || '\u6062\u590d\u5931\u8d25')
        } finally {
          setSubmitting(false)
        }
      },
    })
  }

  const deleteRecord = (record: BackupRecord) => {
    Modal.confirm({
      title: '\u786e\u8ba4\u5220\u9664\u5907\u4efd\u8bb0\u5f55',
      content: '\u4ec5\u5220\u9664\u5e73\u53f0\u7eb3\u7ba1\u8bb0\u5f55\uff0c\u4e0d\u5220\u9664\u8fdc\u7a0b\u5907\u4efd\u6587\u4ef6\u6216\u76ee\u5f55\u3002',
      okText: '\u5220\u9664\u8bb0\u5f55',
      cancelText: '\u53d6\u6d88',
      okButtonProps: { danger: true },
      onOk: async () => {
        setSubmitting(true)
        try {
          await backupApi.delete(record.id)
          message.success('\u5907\u4efd\u8bb0\u5f55\u5df2\u5220\u9664')
          await fetchRecordsFor(record.instance_id)
        } catch (err: any) {
          message.error(err?.response?.data?.message || err?.message || '\u5220\u9664\u5907\u4efd\u8bb0\u5f55\u5931\u8d25')
        } finally {
          setSubmitting(false)
        }
      },
    })
  }

  const recordColumns: ColumnsType<BackupRecord> = [
    { title: '实例', dataIndex: 'instance_id', key: 'instance_id', width: 120, ellipsis: true, render: instanceName },
    {
      title: '类型',
      dataIndex: 'backup_type',
      key: 'backup_type',
      width: 80,
      render: (type) => <BackupTypeTag type={type} />,
    },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      width: 90,
      render: (status) => (
        <Tag color={isCompletedBackupStatus(status) ? 'success' : isActiveBackupStatus(status) ? 'processing' : isFailedBackupStatus(status) ? 'error' : 'default'}>
          {formatStatus(status)}
        </Tag>
      ),
    },
    {
      title: '信息',
      dataIndex: 'message',
      key: 'message',
      ellipsis: true,
      render: (text) => text ? <Tooltip title={text}>{text}</Tooltip> : '-',
    },
    { title: '大小', dataIndex: 'size', key: 'size', width: 100 },
    {
      title: '文件',
      dataIndex: 'file_path',
      key: 'file_path',
      width: 260,
      ellipsis: true,
      render: (path) => path ? <Tooltip title={path}><span style={{ fontFamily: 'monospace', fontSize: 12, display: 'block', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{path.split('/').pop() || path}</span></Tooltip> : '-',
    },
    { title: '创建时间', dataIndex: 'created_at', key: 'created_at', width: 160, render: (v) => v ? new Date(v).toLocaleString() : '-' },
    {
      title: '\u64cd\u4f5c',
      key: 'actions',
      width: 160,
      render: (_, record) => (
        <Space size="small">
          <Button
            size="small"
            type="link"
            icon={<RollbackOutlined />}
            disabled={!isCompletedBackupStatus(record.status)}
            loading={submitting}
            onClick={() => restoreRecord(record)}
          >
            {'\u6062\u590d'}
          </Button>
          <Button
            size="small"
            type="link"
            danger
            icon={<DeleteOutlined />}
            loading={submitting}
            onClick={() => deleteRecord(record)}
          >
            {'\u5220\u9664'}
          </Button>
        </Space>
      ),
    },
  ]

  const policyColumns: ColumnsType<BackupPolicy> = [
    { title: '实例', dataIndex: 'instance_id', key: 'instance_id', width: 120, ellipsis: true, render: instanceName },
    { title: '类型', dataIndex: 'backup_type', key: 'backup_type', render: (type) => <BackupTypeTag type={type} /> },
    { title: 'Cron', dataIndex: 'schedule', key: 'schedule' },
    {
      title: '操作',
      key: 'action',
      render: (_, record) => (
        <Space size="small">
          <Button size="small" type="link" loading={submitting} onClick={() => openEditPolicy(record)}>
            {'\u7f16\u8f91'}
          </Button>
          <Button size="small" type="link" loading={submitting} onClick={() => executePolicy(record)}>
            执行
          </Button>
          <Button size="small" type="link" danger icon={<DeleteOutlined />} loading={submitting} onClick={() => deletePolicy(record)}>
            {'\u5220\u9664'}
          </Button>
        </Space>
      ),
    },
    { title: '保留天数', dataIndex: 'retention_days', key: 'retention_days' },
    { title: '存储路径', dataIndex: 'storage_path', key: 'storage_path', width: 180, ellipsis: true, render: (v: string) => v ? <Tooltip title={v}><span style={{ fontSize: 12 }}>{v.split('/').pop() || v}</span></Tooltip> : '-' },
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
            创建备份任务
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
                    <Col span={6}><Card size="small"><Statistic title="已完成" value={records.filter((r) => isCompletedBackupStatus(r.status)).length} valueStyle={{ color: '#3f8600' }} /></Card></Col>
                    <Col span={6}><Card size="small"><Statistic title="运行中" value={records.filter((r) => isActiveBackupStatus(r.status)).length} valueStyle={{ color: '#1677ff' }} /></Card></Col>
                    <Col span={6}><Card size="small"><Statistic title="失败" value={records.filter((r) => isFailedBackupStatus(r.status)).length} /></Card></Col>
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
        title="创建备份任务"
        open={createOpen}
        onCancel={() => setCreateOpen(false)}
        onOk={submitBackup}
        confirmLoading={submitting}
        okText="创建任务"
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
        title={editingPolicy ? '编辑备份策略' : '新建备份策略'}
        open={policyOpen}
        onCancel={() => {
          setPolicyOpen(false)
          setEditingPolicy(null)
        }}
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

    </div>
  )
}

const BackupTypeTag: React.FC<{ type: string }> = ({ type }) => {
  const text = type === 'full' ? '全量' : type === 'incremental' ? '增量' : type === 'logical' ? '逻辑' : type
  const color = type === 'full' ? 'blue' : type === 'incremental' ? 'green' : 'orange'
  return <Tag color={color}>{text}</Tag>
}

function formatStatus(status: string): string {
  if (status === 'completed') return '已完成'
  if (status === 'running') return '运行中'
  if (status === 'failed') return '失败'
  return status || '-'
}

function formatSize(bytes: number): string {
  if (!bytes) return '-'
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  if (bytes < 1024 * 1024 * 1024) return `${(bytes / 1024 / 1024).toFixed(1)} MB`
  return `${(bytes / 1024 / 1024 / 1024).toFixed(2)} GB`
}

export default BackupManage
