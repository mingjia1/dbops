import React, { useState } from 'react'
import { Card, Form, Select, Button, Space, Table, message, Tag, Descriptions, Input, InputNumber } from 'antd'
import { PlayCircleOutlined, ClockCircleOutlined } from '@ant-design/icons'
import { backupApi } from '@/services/api'

interface BackupRecord {
  task_id: string
  status: string
  started_at: string
  completed_at: string
  file_path: string
  file_size: number
  checksum: string
}

const BackupManage: React.FC = () => {
  const [form] = Form.useForm()
  const [backupRecords, setBackupRecords] = useState<BackupRecord[]>([])
  const [loading, setLoading] = useState(false)
  const [policyLoading, setPolicyLoading] = useState(false)

  const handleCreatePolicy = async (values: any) => {
    setPolicyLoading(true)
    try {
      await backupApi.createPolicy(values)
      message.success('备份策略创建成功')
    } catch (err) {
      message.error('创建失败')
    } finally {
      setPolicyLoading(false)
    }
  }

  const handleExecuteBackup = async () => {
    const values = form.getFieldsValue()
    if (!values.instance_id) {
      message.error('请选择实例')
      return
    }

    setLoading(true)
    try {
      const response = await backupApi.executeBackup(values.instance_id, values.backup_type)
      message.success('备份任务已启动')
      setBackupRecords([response.data])
    } catch (err) {
      message.error('备份失败')
    } finally {
      setLoading(false)
    }
  }

  const handleListBackups = async (instanceId: string) => {
    setLoading(true)
    try {
      const response = await backupApi.listBackups(instanceId)
      setBackupRecords(response.data)
    } catch (err) {
      message.error('获取备份列表失败')
    } finally {
      setLoading(false)
    }
  }

  const columns = [
    {
      title: '任务ID',
      dataIndex: 'task_id',
      key: 'task_id',
    },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      render: (status: string) => {
        const color = status === 'completed' ? 'success' : status === 'running' ? 'processing' : 'error'
        return <Tag color={color}>{status}</Tag>
      },
    },
    {
      title: '开始时间',
      dataIndex: 'started_at',
      key: 'started_at',
    },
    {
      title: '完成时间',
      dataIndex: 'completed_at',
      key: 'completed_at',
    },
    {
      title: '文件路径',
      dataIndex: 'file_path',
      key: 'file_path',
    },
    {
      title: '文件大小',
      dataIndex: 'file_size',
      key: 'file_size',
      render: (size: number) => `${(size / 1024 / 1024).toFixed(2)} MB`,
    },
  ]

  return (
    <Card title="备份管理">
      <div style={{ marginBottom: 24 }}>
        <Descriptions title="创建备份策略" bordered column={2}>
          <Descriptions.Item label="说明">
            配置定时备份策略，系统将自动执行备份任务
          </Descriptions.Item>
        </Descriptions>
        
        <Form form={form} layout="inline" style={{ marginTop: 16 }}>
          <Form.Item name="instance_id" label="实例" rules={[{ required: true }]}>
            <Select placeholder="选择实例" style={{ width: 200 }}>
              <Select.Option value="instance-001">instance-001</Select.Option>
              <Select.Option value="instance-002">instance-002</Select.Option>
            </Select>
          </Form.Item>
          <Form.Item name="backup_type" label="类型" initialValue="full">
            <Select style={{ width: 120 }}>
              <Select.Option value="full">全量备份</Select.Option>
              <Select.Option value="incremental">增量备份</Select.Option>
            </Select>
          </Form.Item>
          <Form.Item name="schedule" label="调度">
            <Input placeholder="0 2 * * *" style={{ width: 150 }} />
          </Form.Item>
          <Form.Item name="retention_days" label="保留天数" initialValue={7}>
            <InputNumber min={1} max={365} style={{ width: 100 }} />
          </Form.Item>
          <Form.Item>
            <Space>
              <Button loading={policyLoading} onClick={() => handleCreatePolicy(form.getFieldsValue())}>
                创建策略
              </Button>
              <Button
                type="primary"
                icon={<PlayCircleOutlined />}
                loading={loading}
                onClick={handleExecuteBackup}
              >
                执行备份
              </Button>
              <Button icon={<ClockCircleOutlined />} onClick={() => handleListBackups(form.getFieldValue('instance_id'))}>
                查看历史
              </Button>
            </Space>
          </Form.Item>
        </Form>
      </div>

      <Table
        columns={columns}
        dataSource={backupRecords}
        rowKey="task_id"
        loading={loading}
      />
    </Card>
  )
}

export default BackupManage