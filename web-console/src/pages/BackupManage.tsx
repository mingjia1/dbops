import React, { useEffect, useState } from 'react'
import { Card, Table, Button, Space, Tag, Progress, Select } from 'antd'
import { PlusOutlined } from '@ant-design/icons'
import type { ColumnsType } from 'antd/es/table'
import { backupApi, instanceApi } from '../services/api'

interface Backup {
  id: string
  instance_name: string
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

const BackupManage: React.FC = () => {
  const [data, setData] = useState<Backup[]>([])
  const [instances, setInstances] = useState<any[]>([])
  const [selectedInstance, setSelectedInstance] = useState<string | undefined>(undefined)
  const [loading, setLoading] = useState(false)

  useEffect(() => {
    instanceApi.list(100, 0).then((res: any) => {
      if (res?.data) setInstances(res.data)
    }).catch(() => {})
  }, [])

  useEffect(() => {
    if (!selectedInstance) return
    setLoading(true)
    backupApi.listBackups(selectedInstance).then((res: any) => {
      const list: Backup[] = (res?.data || []).map((r: BackupRecord) => ({
        id: r.id,
        instance_name: r.instance_id,
        backup_type: r.backup_type,
        size: r.size || '-',
        status: r.status,
        progress: r.status === 'running' ? 50 : r.status === 'completed' ? 100 : 0,
        created_at: r.created_at,
      }))
      setData(list)
    }).catch(() => {}).finally(() => setLoading(false))
  }, [selectedInstance])

  const columns: ColumnsType<Backup> = [
    {
      title: '实例',
      dataIndex: 'instance_name',
      key: 'instance_name',
    },
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
    {
      title: '大小',
      dataIndex: 'size',
      key: 'size',
    },
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
    {
      title: '创建时间',
      dataIndex: 'created_at',
      key: 'created_at',
    },
    {
      title: '操作',
      key: 'action',
      render: () => (
        <Space>
          <Button type="link" size="small">下载</Button>
          <Button type="link" size="small">恢复</Button>
          <Button type="link" size="small" danger>删除</Button>
        </Space>
      ),
    },
  ]

  return (
    <div style={{ padding: '24px' }}>
      <Card
        title="备份管理"
        extra={
          <Space>
            <Select
              placeholder="选择实例"
              style={{ width: 200 }}
              allowClear
              value={selectedInstance}
              onChange={setSelectedInstance}
              options={instances.map((i: any) => ({ label: i.name, value: i.id }))}
            />
            <Button type="primary" icon={<PlusOutlined />}>
              创建备份
            </Button>
          </Space>
        }
      >
        <Table columns={columns} dataSource={data} rowKey="id" loading={loading} />
      </Card>
    </div>
  )
}

export default BackupManage