import React, { useEffect, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { Card, Descriptions, Button, Space, Tag, Spin, message, Alert, Table, Popconfirm } from 'antd'
import { ArrowLeftOutlined, ThunderboltOutlined, DatabaseOutlined, PlusOutlined } from '@ant-design/icons'
import type { ColumnsType } from 'antd/es/table'
import { hostApi, instanceApi, Host, HostTestResult, Instance } from '../services/api'

const HostDetail: React.FC = () => {
  const navigate = useNavigate()
  const { id } = useParams<{ id: string }>()
  const [host, setHost] = useState<Host | null>(null)
  const [loading, setLoading] = useState(true)
  const [testing, setTesting] = useState(false)
  const [testResult, setTestResult] = useState<HostTestResult | null>(null)
  const [instances, setInstances] = useState<Instance[]>([])
  const [instLoading, setInstLoading] = useState(false)

  const fetchHost = async () => {
    if (!id) return
    setLoading(true)
    try {
      const res: any = await hostApi.get(id)
      setHost(res.data)
    } catch {
      message.error('主机不存在')
      navigate('/dashboard/hosts')
    } finally {
      setLoading(false)
    }
  }

  const fetchInstances = async () => {
    if (!id) return
    setInstLoading(true)
    try {
      const res: any = await instanceApi.listByHost(id)
      setInstances(res.data || [])
    } catch {
      setInstances([])
    } finally {
      setInstLoading(false)
    }
  }

  useEffect(() => {
    fetchHost()
  }, [id])

  useEffect(() => {
    if (id) fetchInstances()
  }, [id])

  const handleDeleteInstance = async (iid: string) => {
    try {
      await instanceApi.delete(iid)
      message.success('实例删除成功')
      fetchInstances()
    } catch {
      // interceptor handled
    }
  }

  const instanceColumns: ColumnsType<Instance> = [
    { title: '实例名称', dataIndex: 'name', key: 'name' },
    { title: '集群 ID', dataIndex: 'cluster_id', key: 'cluster_id', render: (v) => v || '-' },
    {
      title: '创建时间', dataIndex: 'created_at', key: 'created_at',
      render: (t) => (t ? new Date(t).toLocaleString() : '-'),
    },
    {
      title: '操作', key: 'action',
      render: (_, r) => (
        <Space>
          <Button type="link" size="small" onClick={() => navigate(`/dashboard/instances/${r.id}`)}>
            详情
          </Button>
          <Popconfirm title="确定删除?" onConfirm={() => handleDeleteInstance(r.id)} okText="确定" cancelText="取消">
            <Button type="link" size="small" danger>删除</Button>
          </Popconfirm>
        </Space>
      ),
    },
  ]

  const handleTest = async () => {
    if (!id) return
    setTesting(true)
    setTestResult(null)
    try {
      const res: any = await hostApi.testConnection(id)
      const initial: HostTestResult = res.data
      setTestResult(initial)

      const taskId = initial.task_id
      let attempts = 0
      const interval = setInterval(async () => {
        attempts++
        try {
          const r: any = await hostApi.getTestResult(taskId)
          setTestResult(r.data)
          if (r.data.status === 'success' || r.data.status === 'failed' || attempts >= 10) {
            clearInterval(interval)
            setTesting(false)
            if (r.data.status === 'success' || r.data.status === 'failed') {
              fetchHost()
            }
          }
        } catch {
          clearInterval(interval)
          setTesting(false)
        }
      }, 1000)
    } catch {
      setTesting(false)
    }
  }

  if (loading) {
    return (
      <div style={{ padding: '24px', textAlign: 'center' }}>
        <Spin />
      </div>
    )
  }

  if (!host) return null

  return (
    <div style={{ padding: '24px' }}>
      <Card
        title={
          <Space>
            <Button type="text" icon={<ArrowLeftOutlined />} onClick={() => navigate('/dashboard/hosts')} />
            <span>主机详情 - {host.name}</span>
          </Space>
        }
        extra={
          <Space>
            <Button icon={<ThunderboltOutlined />} onClick={handleTest} loading={testing}>
              测试连接
            </Button>
            <Button type="primary" onClick={() => navigate(`/dashboard/hosts/${host.id}/edit`)}>
              编辑
            </Button>
          </Space>
        }
      >
        {testResult && (
          <Alert
            style={{ marginBottom: 16 }}
            type={
              testResult.status === 'success'
                ? 'success'
                : testResult.status === 'failed'
                ? 'error'
                : 'info'
            }
            message={
              testResult.status === 'success'
                ? '连接成功'
                : testResult.status === 'failed'
                ? '连接失败'
                : '正在检测...'
            }
            description={
              <div>
                <div>{testResult.message}</div>
                {testResult.latency_ms > 0 && (
                  <div style={{ marginTop: 4 }}>延迟: {testResult.latency_ms} ms</div>
                )}
              </div>
            }
            showIcon
          />
        )}

        <Descriptions bordered column={2}>
          <Descriptions.Item label="主机ID">{host.id}</Descriptions.Item>
          <Descriptions.Item label="主机名称">{host.name}</Descriptions.Item>
          <Descriptions.Item label="地址">{host.address}</Descriptions.Item>
          <Descriptions.Item label="SSH 端口">{host.ssh_port}</Descriptions.Item>
          <Descriptions.Item label="SSH 用户">{host.ssh_user}</Descriptions.Item>
          <Descriptions.Item label="认证方式">
            <Tag>{host.ssh_auth_method === 'password' ? '密码' : '密钥'}</Tag>
          </Descriptions.Item>
          <Descriptions.Item label="操作系统">{host.os_type?.toUpperCase()}</Descriptions.Item>
          <Descriptions.Item label="状态">
            <Tag color={host.status === 'success' ? 'success' : host.status === 'failed' ? 'error' : 'default'}>
              {host.status === 'success' ? '可用' : host.status === 'failed' ? '不可用' : '未检测'}
            </Tag>
          </Descriptions.Item>
          <Descriptions.Item label="标签">{host.tags || '-'}</Descriptions.Item>
          <Descriptions.Item label="最后检测">
            {host.last_check_at ? new Date(host.last_check_at).toLocaleString() : '-'}
          </Descriptions.Item>
          <Descriptions.Item label="描述" span={2}>
            {host.description || '-'}
          </Descriptions.Item>
          <Descriptions.Item label="创建时间">
            {new Date(host.created_at).toLocaleString()}
          </Descriptions.Item>
          <Descriptions.Item label="更新时间">
            {new Date(host.updated_at).toLocaleString()}
          </Descriptions.Item>
        </Descriptions>
      </Card>

      <Card
        style={{ marginTop: 16 }}
        title={
          <Space>
            <DatabaseOutlined />
            <span>本主机实例 ({instances.length})</span>
          </Space>
        }
        extra={
          <Button type="primary" icon={<PlusOutlined />} onClick={() => navigate(`/dashboard/instances?preset_host=${host.id}`)}>
            创建实例
          </Button>
        }
      >
        <Table
          columns={instanceColumns}
          dataSource={instances}
          rowKey="id"
          loading={instLoading}
          pagination={{ pageSize: 10 }}
          locale={{ emptyText: '暂无实例, 点击"创建实例"添加' }}
        />
      </Card>
    </div>
  )
}

export default HostDetail
