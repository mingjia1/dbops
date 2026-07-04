import React, { useEffect, useState } from 'react'
import {
  Button, Card, Empty, Form, Modal, Select, Space, Table, Tabs, Tag, Tooltip, message,
} from 'antd'
import { DeleteOutlined, FileSearchOutlined, PlusOutlined, ReloadOutlined, RollbackOutlined } from '@ant-design/icons'
import type { ColumnsType } from 'antd/es/table'
import { backupApi, instanceApi, type Instance } from '../services/api'
import {
  isFailedBackupStatus, isCompletedBackupStatus, isActiveBackupStatus, formatBackupStatus,
  type BackupRecord, type BackupPolicy, BACKUP_TYPE_LABELS, BACKUP_TYPE_COLORS,
} from '../services/backupHelpers'
import BackupCreateModal from '../components/BackupCreateModal'
import BackupPolicyModal from '../components/BackupPolicyModal'
import BackupStatCards from '../components/BackupStatCards'

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
          {formatBackupStatus(status)}
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
                          <BackupStatCards records={records} />
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

      <BackupCreateModal
        open={createOpen}
        submitting={submitting}
        form={form}
        onOk={submitBackup}
        onCancel={() => setCreateOpen(false)}
      />

      <BackupPolicyModal
        open={policyOpen}
        submitting={submitting}
        editingPolicy={editingPolicy}
        form={policyForm}
        instanceOptions={instances.map((item) => ({ value: item.id, label: item.name }))}
        onOk={submitPolicy}
        onCancel={() => {
          setPolicyOpen(false)
          setEditingPolicy(null)
        }}
      />

    </div>
  )
}

const BackupTypeTag: React.FC<{ type: string }> = ({ type }) => {
  const text = BACKUP_TYPE_LABELS[type] || type
  const color = BACKUP_TYPE_COLORS[type] || 'default'
  return <Tag color={color}>{text}</Tag>
}

export default BackupManage
