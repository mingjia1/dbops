import React, { useEffect, useRef, useState } from 'react'
import { Card, Descriptions, Tag, Button, Space, message, Spin, Tabs, Table, Modal, Form, Input, Popconfirm, Progress, Alert } from 'antd'
import { useParams, useNavigate } from 'react-router-dom'
import { ArrowLeftOutlined, ReloadOutlined, EditOutlined, DeleteOutlined, ThunderboltOutlined } from '@ant-design/icons'
import { instanceApi, type Instance as InstanceModel, type InstanceVersion } from '../services/api'

const InstanceDetail: React.FC = () => {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const [instance, setInstance] = useState<InstanceModel | null>(null)
  const [versionInfo, setVersionInfo] = useState<InstanceVersion | null>(null)
  const [loading, setLoading] = useState(false)
  const [versionLoading, setVersionLoading] = useState(false)
  const [editOpen, setEditOpen] = useState(false)
  const [submitting, setSubmitting] = useState(false)
  const [deploying, setDeploying] = useState(false)
  const [deployOpen, setDeployOpen] = useState(false)
  const [deployProgress, setDeployProgress] = useState(0)
  const [deployStage, setDeployStage] = useState('已提交')
  const [deployStatus, setDeployStatus] = useState<'running' | 'success' | 'failed' | null>(null)
  const [deployMessage, setDeployMessage] = useState<string>('')
  const pollRef = useRef<number | null>(null)
  const [form] = Form.useForm()

  useEffect(() => {
    fetchInstance()
  }, [id])

  useEffect(() => () => {
    if (pollRef.current) window.clearInterval(pollRef.current)
  }, [])

  const fetchInstance = async () => {
    if (!id) return
    setLoading(true)
    try {
      const response: any = await instanceApi.get(id)
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
      const response: any = await instanceApi.detectVersion(id)
      setVersionInfo(response.data)
      message.success('版本识别完成')
    } catch (err) {
      message.error('版本识别失败')
    } finally {
      setVersionLoading(false)
    }
  }

  const openEdit = () => {
    if (!instance) return
    form.setFieldsValue({
      name: instance.name,
      cluster_id: instance.cluster_id,
    })
    setEditOpen(true)
  }

  const submitEdit = async () => {
    if (!id) return
    try {
      const values = await form.validateFields()
      setSubmitting(true)
      await instanceApi.update(id, values)
      message.success('更新成功')
      setEditOpen(false)
      fetchInstance()
    } catch {
      // interceptor handled
    } finally {
      setSubmitting(false)
    }
  }

  const stopDeployPolling = () => {
    if (pollRef.current) {
      window.clearInterval(pollRef.current)
      pollRef.current = null
    }
  }

  const startDeployPolling = () => {
    stopDeployPolling()
    let attempts = 0
    pollRef.current = window.setInterval(async () => {
      attempts += 1
      try {
        const res: any = await instanceApi.get(id!)
        const data = res?.data
        const status = data?.status
        if (!status) return
        const prog = status.deploy_progress
        if (typeof prog === 'number') setDeployProgress(prog)
        if (status.stage) setDeployStage(status.stage)
        if (status.deploy_status) setDeployStatus(status.deploy_status)
        if (status.deploy_message) setDeployMessage(status.deploy_message)
        // F4: 之前 health==='healthy' || role 任一为真就判定 "部署完成",
        // 对于已运行实例 role 永远有值, 点 "部署" 立即假完成. 现在只信 deploy_status.
        if (status.deploy_status === 'success') {
          setDeployStatus('success')
          setDeployProgress(100)
          setDeployStage('部署完成')
          stopDeployPolling()
          message.success('部署完成')
          fetchInstance()
        } else if (status.deploy_status === 'failed') {
          setDeployStatus('failed')
          stopDeployPolling()
          message.error('部署失败')
        } else if (attempts > 600) {
          stopDeployPolling()
        }
      } catch {
        // ignore
      }
    }, 2000)
  }

  const handleDeploy = async () => {
    if (!id) return
    setDeploying(true)
    setDeployOpen(true)
    setDeployProgress(0)
    setDeployStage('已提交, 等待后端开始执行')
    setDeployStatus('running')
    setDeployMessage('')
    try {
      await instanceApi.deploy(id)
      message.success('部署任务已提交')
      startDeployPolling()
    } catch {
      setDeployStatus('failed')
      setDeployMessage('提交失败')
      // interceptor handled
    } finally {
      setDeploying(false)
    }
  }

  const handleDelete = async () => {
    if (!id) return
    try {
      await instanceApi.delete(id)
      message.success('删除成功')
      navigate('/dashboard/instances')
    } catch {
      // interceptor handled
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

  const featureList = versionInfo?.features
    ? versionInfo.features.split(',').map((f) => f.trim()).filter(Boolean)
    : []

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
          <Button icon={<ArrowLeftOutlined />} onClick={() => navigate('/dashboard/instances')}>
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
          <Button icon={<ThunderboltOutlined />} loading={deploying} onClick={handleDeploy}>
            部署
          </Button>
          <Button icon={<EditOutlined />} onClick={openEdit}>编辑</Button>
          <Popconfirm
            title="确定删除此实例?"
            onConfirm={handleDelete}
            okText="确定"
            cancelText="取消"
          >
            <Button danger icon={<DeleteOutlined />}>删除</Button>
          </Popconfirm>
        </Space>
      }
    >
      <Tabs
        defaultActiveKey="basic"
        items={[
          {
            key: 'basic',
            label: '基本信息',
            children: (
              <Descriptions bordered column={2}>
                <Descriptions.Item label="实例ID">{instance.id}</Descriptions.Item>
                <Descriptions.Item label="实例名称">{instance.name}</Descriptions.Item>
                <Descriptions.Item label="主机地址">{instance.host || instance.connection?.host || '-'}</Descriptions.Item>
                <Descriptions.Item label="端口">{instance.port || instance.connection?.port || '-'}</Descriptions.Item>
                <Descriptions.Item label="用户名">{instance.username || instance.connection?.username || '-'}</Descriptions.Item>
                <Descriptions.Item label="SSL">
                  <Tag color={instance.ssl_enabled || instance.connection?.ssl_enabled ? 'success' : 'default'}>
                    {instance.ssl_enabled || instance.connection?.ssl_enabled ? '已启用' : '未启用'}
                  </Tag>
                </Descriptions.Item>
                <Descriptions.Item label="集群ID" span={2}>
                  {instance.cluster_id || <Tag color="default">单点</Tag>}
                </Descriptions.Item>
                <Descriptions.Item label="角色">
                  {instance.status?.role ? <Tag color="blue">{instance.status.role}</Tag> : '-'}
                </Descriptions.Item>
                <Descriptions.Item label="健康状态">
                  {instance.status?.health_status ? <Tag color="green">{instance.status.health_status}</Tag> : '-'}
                </Descriptions.Item>
                <Descriptions.Item label="复制延迟(秒)">
                  {instance.status?.seconds_behind_master ?? '-'}
                </Descriptions.Item>
                <Descriptions.Item label="主节点ID">
                  {instance.topology?.master_id || '-'}
                </Descriptions.Item>
                <Descriptions.Item label="复制模式">
                  {instance.topology?.replication_mode || '-'}
                </Descriptions.Item>
                <Descriptions.Item label="创建时间">
                  {new Date(instance.created_at).toLocaleString()}
                </Descriptions.Item>
                <Descriptions.Item label="更新时间">
                  {new Date(instance.updated_at).toLocaleString()}
                </Descriptions.Item>
              </Descriptions>
            ),
          },
          {
            key: 'version',
            label: '版本信息',
            children: versionInfo ? (
              <div>
                <Descriptions bordered column={2} style={{ marginBottom: 16 }}>
                  <Descriptions.Item label="发行版"><Tag color="blue">{versionInfo.flavor}</Tag></Descriptions.Item>
                  <Descriptions.Item label="版本号">{versionInfo.version}</Descriptions.Item>
                  <Descriptions.Item label="完整版本">{versionInfo.full_version}</Descriptions.Item>
                  <Descriptions.Item label="LTS">
                    <Tag color={versionInfo.is_lts ? 'success' : 'default'}>{versionInfo.is_lts ? 'LTS' : '非LTS'}</Tag>
                  </Descriptions.Item>
                  <Descriptions.Item label="EOL日期">{versionInfo.eol_date || '未知'}</Descriptions.Item>
                  <Descriptions.Item label="EOL状态">{getEolStatus(versionInfo.eol_date)}</Descriptions.Item>
                </Descriptions>
                <Card title="特性支持" size="small">
                  <Table
                    columns={featureColumns}
                    dataSource={featureList.map((f) => ({ name: f, supported: true }))}
                    rowKey="name"
                    pagination={false}
                    size="small"
                  />
                </Card>
              </div>
            ) : (
              <div style={{ textAlign: 'center', padding: 40 }}>
                <p>暂无版本信息，请点击"识别版本"按钮获取</p>
                <Button type="primary" icon={<ReloadOutlined />} loading={versionLoading} onClick={handleDetectVersion}>识别版本</Button>
              </div>
            ),
          },
        ]}
      />

      <Modal
        title="编辑实例"
        open={editOpen}
        onCancel={() => setEditOpen(false)}
        onOk={submitEdit}
        confirmLoading={submitting}
        okText="保存"
        cancelText="取消"
      >
        <Form form={form} layout="vertical">
          <Form.Item name="name" label="实例名称" rules={[{ required: true, message: '请输入实例名称' }]}>
            <Input />
          </Form.Item>
          <Form.Item name="cluster_id" label="集群ID (可选)">
            <Input placeholder="例如: mgr-cluster-01" />
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        title={`部署实例: ${instance.name}`}
        open={deployOpen}
        onCancel={() => { stopDeployPolling(); setDeployOpen(false) }}
        footer={[<Button key="close" onClick={() => { stopDeployPolling(); setDeployOpen(false) }}>关闭</Button>]}
        width={600}
      >
        <Alert
          type="info"
          showIcon
          message="部署过程不可中断, 请耐心等待"
          description="部署会安装/初始化 MySQL 二进制, 修改系统配置。期间实例可能短暂不可用。"
          style={{ marginBottom: 16 }}
        />
        <div style={{ marginBottom: 8 }}>当前阶段: <b>{deployStage}</b></div>
        <Progress
          percent={deployProgress}
          status={deployStatus === 'failed' ? 'exception' : deployStatus === 'success' ? 'success' : 'active'}
        />
        {deployMessage && (
          <Alert
            style={{ marginTop: 16 }}
            type={deployStatus === 'failed' ? 'error' : 'info'}
            showIcon
            message={deployMessage}
          />
        )}
      </Modal>
    </Card>
  )
}

export default InstanceDetail