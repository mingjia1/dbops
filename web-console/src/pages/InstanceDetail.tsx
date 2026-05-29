import React, { useEffect, useState } from 'react'
import { Card, Descriptions, Tag, Button, Space, message, Spin, Tabs, Table } from 'antd'
import { useParams, useNavigate } from 'react-router-dom'
import { ArrowLeftOutlined, ReloadOutlined, EditOutlined } from '@ant-design/icons'
import { instanceApi } from '@/services/api'

interface InstanceDetail {
  id: string
  name: string
  host: string
  port: number
  username: string
  cluster_id: string
  environment: string
  ssl_enabled: boolean
  created_at: string
  updated_at: string
}

interface VersionInfo {
  flavor: string
  version: string
  full_version: string
  is_lts: boolean
  eol_date: string
  features: string[]
}

const InstanceDetail: React.FC = () => {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const [instance, setInstance] = useState<InstanceDetail | null>(null)
  const [versionInfo, setVersionInfo] = useState<VersionInfo | null>(null)
  const [loading, setLoading] = useState(false)
  const [versionLoading, setVersionLoading] = useState(false)

  useEffect(() => {
    fetchInstance()
  }, [id])

  const fetchInstance = async () => {
    if (!id) return
    setLoading(true)
    try {
      const response = await instanceApi.get(id)
      setInstance(response.data)
    } catch (err) {
      message.error('获取实例信息失败')
    } finally {
      setLoading(false)
    }
  }

  const handleDetectVersion = async () => {
    if (!id) return
    setVersionLoading(true)
    try {
      const response = await instanceApi.detectVersion(id)
      setVersionInfo(response.data)
      message.success('版本识别完成')
    } catch (err) {
      message.error('版本识别失败')
    } finally {
      setVersionLoading(false)
    }
  }

  const getEolStatus = (eolDate: string) => {
    if (!eolDate) return <Tag color="default">未知</Tag>
    const eol = new Date(eolDate)
    const now = new Date()
    if (eol < now) return <Tag color="error">已过期</Tag>
    if (eol < new Date(now.getTime() + 365 * 24 * 60 * 60 * 1000)) return <Tag color="warning">即将过期</Tag>
    return <Tag color="success">正常</Tag>
  }

  const featureColumns = [
    {
      title: '特性',
      dataIndex: 'name',
      key: 'name',
    },
    {
      title: '支持状态',
      dataIndex: 'supported',
      key: 'supported',
      render: (supported: boolean) => (
        <Tag color={supported ? 'success' : 'default'}>{supported ? '支持' : '不支持'}</Tag>
      ),
    },
  ]

  if (loading) {
    return <Spin style={{ display: 'block', margin: '100px auto' }} />
  }

  if (!instance) {
    return <Card>实例不存在</Card>
  }

  return (
    <Card
      title={
        <Space>
          <Button icon={<ArrowLeftOutlined />} onClick={() => navigate('/instances')}>
            返回列表
          </Button>
          <span>实例详情: {instance.name}</span>
        </Space>
      }
      extra={
        <Space>
          <Button icon={<ReloadOutlined />} loading={versionLoading} onClick={handleDetectVersion}>
            识别版本
          </Button>
          <Button icon={<EditOutlined />}>编辑</Button>
        </Space>
      }
    >
      <Tabs defaultActiveKey="basic">
        <Tabs.TabPane tab="基本信息" key="basic">
          <Descriptions bordered column={2}>
            <Descriptions.Item label="实例ID">{instance.id}</Descriptions.Item>
            <Descriptions.Item label="实例名称">{instance.name}</Descriptions.Item>
            <Descriptions.Item label="主机地址">{instance.host}</Descriptions.Item>
            <Descriptions.Item label="端口">{instance.port}</Descriptions.Item>
            <Descriptions.Item label="用户名">{instance.username}</Descriptions.Item>
            <Descriptions.Item label="环境">{instance.environment || 'default'}</Descriptions.Item>
            <Descriptions.Item label="SSL">
              <Tag color={instance.ssl_enabled ? 'success' : 'default'}>
                {instance.ssl_enabled ? '已启用' : '未启用'}
              </Tag>
            </Descriptions.Item>
            <Descriptions.Item label="集群ID">
              {instance.cluster_id || <Tag color="default">单点</Tag>}
            </Descriptions.Item>
            <Descriptions.Item label="创建时间">
              {new Date(instance.created_at).toLocaleString()}
            </Descriptions.Item>
            <Descriptions.Item label="更新时间">
              {new Date(instance.updated_at).toLocaleString()}
            </Descriptions.Item>
          </Descriptions>
        </Tabs.TabPane>

        <Tabs.TabPane tab="版本信息" key="version">
          {versionInfo ? (
            <div>
              <Descriptions bordered column={2} style={{ marginBottom: 16 }}>
                <Descriptions.Item label="发行版">
                  <Tag color="blue">{versionInfo.flavor}</Tag>
                </Descriptions.Item>
                <Descriptions.Item label="版本号">{versionInfo.version}</Descriptions.Item>
                <Descriptions.Item label="完整版本">{versionInfo.full_version}</Descriptions.Item>
                <Descriptions.Item label="LTS">
                  <Tag color={versionInfo.is_lts ? 'success' : 'default'}>
                    {versionInfo.is_lts ? 'LTS' : '非LTS'}
                  </Tag>
                </Descriptions.Item>
                <Descriptions.Item label="EOL日期">
                  {versionInfo.eol_date || '未知'}
                </Descriptions.Item>
                <Descriptions.Item label="EOL状态">
                  {getEolStatus(versionInfo.eol_date)}
                </Descriptions.Item>
              </Descriptions>

              <Card title="特性支持" size="small">
                <Table
                  columns={featureColumns}
                  dataSource={versionInfo.features?.map((f) => ({
                    name: f,
                    supported: true,
                  })) || []}
                  rowKey="name"
                  pagination={false}
                  size="small"
                />
              </Card>
            </div>
          ) : (
            <div style={{ textAlign: 'center', padding: 40 }}>
              <p>暂无版本信息，请点击"识别版本"按钮获取</p>
              <Button type="primary" icon={<ReloadOutlined />} loading={versionLoading} onClick={handleDetectVersion}>
                识别版本
              </Button>
            </div>
          )}
        </Tabs.TabPane>
      </Tabs>
    </Card>
  )
}

export default InstanceDetail