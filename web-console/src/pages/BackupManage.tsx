import React, { useEffect, useState } from 'react'
import { Card, Table, Button, Space, Tag, Progress, Select, Modal, Form, Radio, Input, InputNumber, Switch, message, Alert, Tabs, Statistic, Row, Col } from 'antd'
import { PlusOutlined, ReloadOutlined, ScheduleOutlined } from '@ant-design/icons'
import type { ColumnsType } from 'antd/es/table'
import { backupApi, instanceApi, type Instance } from '../services/api'

interface Backup {
  id: string
  instance_name: string
  instance_id: string
  backup_type: string
  size: string
  status: string
  progress: number
  created_at: string
}

interface BackupRecord {
  id: string
  instance_id: string
  backup_type: string
  status: string
  size: string | null
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
  last_run_at?: string
  created_at: string
}

const BackupManage: React.FC = () => {
  const [tab, setTab] = useState('records')
  const [data, setData] = useState<Backup[]>([])
  const [policies, setPolicies] = useState<BackupPolicy[]>([])
  const [instances, setInstances] = useState<Instance[]>([])
  const [selectedInstance, setSelectedInstance] = useState<string | undefined>(undefined)
  const [loading, setLoading] = useState(false)
  const [policyLoading, setPolicyLoading] = useState(false)
  const [createOpen, setCreateOpen] = useState(false)
  const [restoreOpen, setRestoreOpen] = useState(false)
  const [policyOpen, setPolicyOpen] = useState(false)
  const [editingPolicy, setEditingPolicy] = useState<BackupPolicy | null>(null)
  const [restoringBackup, setRestoringBackup] = useState<Backup | null>(null)
  const [submitting, setSubmitting] = useState(false)
  const [form] = Form.useForm()
  const [restoreForm] = Form.useForm()
  const [policyForm] = Form.useForm()

  useEffect(() => {
    instanceApi.list(100, 0).then((res: any) => {
      if (res?.data) setInstances(res.data)
    }).catch(() => {})
    fetchPolicies()
  }, [])

  const fetchBackups = () => {
    if (!selectedInstance) { setData([]); return }
    setLoading(true)
    backupApi.listBackups(selectedInstance).then((res: any) => {
      const list: Backup[] = (res?.data || []).map((r: BackupRecord) => ({
        id: r.id,
        instance_name: instances.find((i) => i.id === r.instance_id)?.name || r.instance_id,
        instance_id: r.instance_id,
        backup_type: r.backup_type,
        size: r.size || '-',
        status: r.status,
        progress: r.status === 'running' ? 50 : r.status === 'completed' ? 100 : 0,
        created_at: r.created_at,
      }))
      setData(list)
    }).catch(() => setData([])).finally(() => setLoading(false))
  }

  const fetchPolicies = () => {
    setPolicyLoading(true)
    backupApi.listPolicies().then((res: any) => {
      setPolicies(res?.data || [])
    }).catch(() => setPolicies([])).finally(() => setPolicyLoading(false))
  }

  useEffect(() => {
    fetchBackups()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [selectedInstance, instances])

  const openCreate = () => {
    if (!selectedInstance) {
      message.warning('请先选择实例')
      return
    }
    form.setFieldsValue({ backup_type: 'full' })
    setCreateOpen(true)
  }

  const submitCreate = async () => {
    try {
      const values = await form.validateFields()
      setSubmitting(true)
      await backupApi.executeBackup(selectedInstance!, values.backup_type)
      message.success('备份任务已提交')
      setCreateOpen(false)
      fetchBackups()
    } catch (err: any) {
      message.error(err?.response?.data?.message || '提交备份任务失败')
    } finally {
      setSubmitting(false)
    }
  }

  const openRestore = (r: Backup) => {
    setRestoringBackup(r)
    restoreForm.resetFields()
    setRestoreOpen(true)
  }

  const submitRestore = async () => {
    if (!restoringBackup) return
    try {
      const values = await restoreForm.validateFields()
      setSubmitting(true)
      try {
        await backupApi.restore({
          backup_id: restoringBackup.id,
          target_instance_id: values.target_instance_id,
          target_type: values.target_type,
          confirm_overwrite: values.confirm_overwrite,
        })
        message.success('恢复任务已提交')
        setRestoreOpen(false)
      } catch (err: any) {
        if (err?.response?.status === 404) {
          message.warning('后端未实现 restore 接口, 请确认 API 服务')
        } else {
          throw err
        }
      }
    } catch (err: any) {
      message.error(err?.response?.data?.message || '提交恢复任务失败')
    } finally {
      setSubmitting(false)
    }
  }

  const openPolicy = (p?: BackupPolicy) => {
    setEditingPolicy(p || null)
    if (p) {
      policyForm.setFieldsValue(p)
    } else {
      policyForm.resetFields()
      policyForm.setFieldsValue({
        instance_id: selectedInstance,
        backup_type: 'full',
        schedule: '0 2 * * *',
        retention_days: 7,
        enabled: true,
        storage_type: 'local',
      })
    }
    setPolicyOpen(true)
  }

  const submitPolicy = async () => {
    try {
      const values = await policyForm.validateFields()
      setSubmitting(true)
      if (editingPolicy) {
        await backupApi.updatePolicy(editingPolicy.id, values)
        message.success('策略已更新')
      } else {
        await backupApi.createPolicy(values)
        message.success('策略已创建')
      }
      setPolicyOpen(false)
      fetchPolicies()
    } catch (err: any) {
      if (err?.response?.status === 404) {
        message.warning('后端未实现 policy 接口, 请确认 API 服务')
      } else {
        message.error(err?.response?.data?.message || '操作失败')
      }
    } finally {
      setSubmitting(false)
    }
  }

  const deletePolicy = (p: BackupPolicy) => {
    Modal.confirm({
      title: '确认删除',
      content: `确定要删除备份策略 ${p.id} 吗?`,
      okText: '删除',
      okButtonProps: { danger: true },
      onOk: async () => {
        try {
          await backupApi.deletePolicy(p.id)
          message.success('已删除')
          fetchPolicies()
        } catch (err: any) {
          if (err?.response?.status === 404) {
            message.warning('后端未实现 policy 接口')
          } else {
            message.error('删除失败')
          }
        }
      },
    })
  }

  const handleDownload = (r: Backup) => {
    const content = `MySQL Backup\n============\nID: ${r.id}\n实例: ${r.instance_name}\n类型: ${r.backup_type}\n大小: ${r.size}\n状态: ${r.status}\n创建时间: ${r.created_at}\n`
    const blob = new Blob([content], { type: 'text/plain' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `${r.id}.txt`
    a.click()
    URL.revokeObjectURL(url)
    message.success('备份元数据已下载')
  }

  const handleDelete = (r: Backup) => {
    Modal.confirm({
      title: '确认删除',
      content: `确定要删除备份 ${r.id} 吗?\n此操作将删除备份文件, 不可恢复。`,
      okText: '删除',
      cancelText: '取消',
      okButtonProps: { danger: true },
      onOk: async () => {
        try {
          await backupApi.delete(r.id)
          message.success('已删除')
          fetchBackups()
        } catch (err: any) {
          if (err?.response?.status === 404) {
            setData((ds) => ds.filter((d) => d.id !== r.id))
            message.warning('后端未实现, 已从本地列表移除')
          } else {
            message.error('删除失败')
          }
        }
      },
    })
  }

  const recordColumns: ColumnsType<Backup> = [
    { title: '实例', dataIndex: 'instance_name', key: 'instance_name' },
    {
      title: '备份类型',
      dataIndex: 'backup_type',
      key: 'backup_type',
      render: (type) => (
        <Tag color={type === 'full' ? 'blue' : type === 'incremental' ? 'green' : 'orange'}>
          {type === 'full' ? '全量备份' : type === 'incremental' ? '增量备份' : type === 'logical' ? '逻辑备份' : type}
        </Tag>
      ),
    },
    { title: '大小', dataIndex: 'size', key: 'size' },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      render: (status) => (
        <Tag color={status === 'running' ? 'processing' : status === 'completed' ? 'success' : 'error'}>
          {status === 'running' ? '进行中' : status === 'completed' ? '已完成' : '失败'}
        </Tag>
      ),
    },
    {
      title: '进度',
      dataIndex: 'progress',
      key: 'progress',
      render: (progress) => <Progress percent={progress} size="small" />,
    },
    { title: '创建时间', dataIndex: 'created_at', key: 'created_at' },
    {
      title: '操作',
      key: 'action',
      render: (_, r: Backup) => (
        <Space>
          <Button type="link" size="small" onClick={() => handleDownload(r)}>下载</Button>
          <Button type="link" size="small" onClick={() => openRestore(r)} disabled={r.status !== 'completed'}>
            恢复
          </Button>
          <Button type="link" size="small" danger onClick={() => handleDelete(r)}>删除</Button>
        </Space>
      ),
    },
  ]

  const policyColumns: ColumnsType<BackupPolicy> = [
    { title: '实例', dataIndex: 'instance_id', key: 'instance_id', render: (id) => instances.find((i) => i.id === id)?.name || id },
    {
      title: '备份类型',
      dataIndex: 'backup_type',
      key: 'backup_type',
      render: (t) => <Tag color={t === 'full' ? 'blue' : t === 'incremental' ? 'green' : 'orange'}>{t}</Tag>,
    },
    { title: 'Cron 表达式', dataIndex: 'schedule', key: 'schedule' },
    { title: '保留天数', dataIndex: 'retention_days', key: 'retention_days' },
    {
      title: '状态',
      dataIndex: 'enabled',
      key: 'enabled',
      render: (e) => <Tag color={e ? 'success' : 'default'}>{e ? '启用' : '禁用'}</Tag>,
    },
    {
      title: '操作',
      key: 'action',
      render: (_, p) => (
        <Space>
          <Button type="link" size="small" onClick={() => openPolicy(p)}>编辑</Button>
          <Button type="link" size="small" danger onClick={() => deletePolicy(p)}>删除</Button>
        </Space>
      ),
    },
  ]

  return (
    <div style={{ padding: '24px' }}>
      <Alert
        type="info"
        showIcon
        style={{ marginBottom: 16 }}
        message="操作说明"
        description={
          <div>
            <div>• <b>备份记录</b>: 列出已执行的备份, 支持下载元数据、恢复到指定实例、删除。</div>
            <div>• <b>备份策略</b>: 配置定时备份 (Cron), 自动执行, 到期自动清理。</div>
          </div>
        }
      />
      <Card>
        <Tabs
          activeKey={tab}
          onChange={setTab}
          items={[
            {
              key: 'records',
              label: '备份记录',
              children: (
                <>
                  <Space style={{ marginBottom: 16 }}>
                    <Select
                      placeholder="选择实例"
                      style={{ width: 220 }}
                      allowClear
                      value={selectedInstance}
                      onChange={setSelectedInstance}
                      options={instances.map((i) => ({ label: i.name, value: i.id }))}
                    />
                    <Button icon={<ReloadOutlined />} onClick={fetchBackups} disabled={!selectedInstance}>
                      刷新
                    </Button>
                    <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>
                      创建备份
                    </Button>
                  </Space>
                  {data.length > 0 && (
                    <Row gutter={16} style={{ marginBottom: 16 }}>
                      <Col span={6}><Card size="small"><Statistic title="备份总数" value={data.length} /></Card></Col>
                      <Col span={6}><Card size="small"><Statistic title="已完成" value={data.filter((d) => d.status === 'completed').length} valueStyle={{ color: '#3f8600' }} /></Card></Col>
                      <Col span={6}><Card size="small"><Statistic title="进行中" value={data.filter((d) => d.status === 'running').length} valueStyle={{ color: '#1890ff' }} /></Card></Col>
                      <Col span={6}><Card size="small"><Statistic title="失败" value={data.filter((d) => d.status === 'failed').length} valueStyle={{ color: '#cf1322' }} /></Card></Col>
                    </Row>
                  )}
                  <Table columns={recordColumns} dataSource={data} rowKey="id" loading={loading} />
                </>
              ),
            },
            {
              key: 'policies',
              label: '备份策略',
              children: (
                <>
                  <Space style={{ marginBottom: 16 }}>
                    <Button type="primary" icon={<PlusOutlined />} onClick={() => openPolicy()}>
                      新建策略
                    </Button>
                    <Button icon={<ReloadOutlined />} onClick={fetchPolicies}>刷新</Button>
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
        onOk={submitCreate}
        confirmLoading={submitting}
        okText="启动"
        cancelText="取消"
      >
        <Form form={form} layout="vertical">
          <Form.Item label="备份类型" name="backup_type" rules={[{ required: true }]}>
            <Radio.Group>
              <Radio.Button value="full">全量备份</Radio.Button>
              <Radio.Button value="incremental">增量备份</Radio.Button>
              <Radio.Button value="logical">逻辑备份</Radio.Button>
            </Radio.Group>
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        title={`恢复备份: ${restoringBackup?.id || ''}`}
        open={restoreOpen}
        onCancel={() => setRestoreOpen(false)}
        onOk={submitRestore}
        confirmLoading={submitting}
        okText="启动恢复"
        cancelText="取消"
        okButtonProps={{ danger: true }}
        width={600}
      >
        <Alert
          type="error"
          showIcon
          message="危险操作"
          description="恢复操作将覆盖目标实例的现有数据, 不可逆。请确认目标实例已停止业务或与业务方沟通确认。"
          style={{ marginBottom: 12 }}
        />
        <Form form={restoreForm} layout="vertical">
          <Form.Item name="target_instance_id" label="目标实例" rules={[{ required: true }]}>
            <Select
              showSearch
              optionFilterProp="label"
              options={instances.map((i) => ({ value: i.id, label: i.name }))}
              placeholder="选择恢复到的目标实例"
            />
          </Form.Item>
          <Form.Item name="target_type" label="恢复方式" initialValue="full">
            <Radio.Group>
              <Radio.Button value="full">全量恢复</Radio.Button>
              <Radio.Button value="point_in_time">时间点恢复</Radio.Button>
            </Radio.Group>
          </Form.Item>
          <Form.Item name="confirm_overwrite" label="确认覆盖目标实例" valuePropName="checked" rules={[{ required: true, message: '请确认' }]}>
            <Switch checkedChildren="已确认" unCheckedChildren="未确认" />
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        title={editingPolicy ? '编辑备份策略' : '新建备份策略'}
        open={policyOpen}
        onCancel={() => setPolicyOpen(false)}
        onOk={submitPolicy}
        confirmLoading={submitting}
        width={600}
      >
        <Form form={policyForm} layout="vertical">
          <Form.Item name="instance_id" label="目标实例" rules={[{ required: true }]}>
            <Select
              showSearch
              optionFilterProp="label"
              options={instances.map((i) => ({ value: i.id, label: i.name }))}
              placeholder="选择要备份的实例"
            />
          </Form.Item>
          <Form.Item name="backup_type" label="备份类型" rules={[{ required: true }]}>
            <Radio.Group>
              <Radio.Button value="full">全量</Radio.Button>
              <Radio.Button value="incremental">增量</Radio.Button>
              <Radio.Button value="logical">逻辑</Radio.Button>
            </Radio.Group>
          </Form.Item>
          <Form.Item
            name="schedule"
            label={
              <span>
                <ScheduleOutlined /> Cron 表达式
              </span>
            }
            rules={[{ required: true, message: '请输入 cron 表达式' }]}
            extra="例如: 0 2 * * * 表示每天凌晨 2 点执行"
          >
            <Input placeholder="0 2 * * *" />
          </Form.Item>
          <Form.Item name="retention_days" label="保留天数" rules={[{ required: true }]}>
            <InputNumber min={1} max={365} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="storage_type" label="存储类型">
            <Radio.Group>
              <Radio.Button value="local">本地</Radio.Button>
              <Radio.Button value="s3">S3</Radio.Button>
              <Radio.Button value="nfs">NFS</Radio.Button>
            </Radio.Group>
          </Form.Item>
          <Form.Item name="storage_path" label="存储路径">
            <Input placeholder="/data/backup/mysql" />
          </Form.Item>
          <Form.Item name="enabled" label="启用" valuePropName="checked">
            <Switch checkedChildren="启用" unCheckedChildren="禁用" />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}

export default BackupManage
