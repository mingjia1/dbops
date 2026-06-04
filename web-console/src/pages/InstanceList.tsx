import React, { useEffect, useState } from 'react'
import { useSearchParams, useNavigate } from 'react-router-dom'
import { Card, Table, Button, Space, Tag, Modal, Form, Input, InputNumber, Select, message, Popconfirm, Alert, Empty } from 'antd'
import { PlusOutlined, ReloadOutlined, ScanOutlined, DesktopOutlined } from '@ant-design/icons'
import type { ColumnsType } from 'antd/es/table'
import { instanceApi, hostApi, Instance, Host } from '../services/api'

const InstanceList: React.FC = () => {
  const [searchParams] = useSearchParams()
  const navigate = useNavigate()
  const presetHost = searchParams.get('preset_host') || searchParams.get('host_id') || undefined

  const [instances, setInstances] = useState<Instance[]>([])
  const [hosts, setHosts] = useState<Host[]>([])
  const [loading, setLoading] = useState(false)
  const [hostFilter, setHostFilter] = useState<string | undefined>(presetHost)
  const [modalOpen, setModalOpen] = useState(false)
  const [submitting, setSubmitting] = useState(false)
  const [form] = Form.useForm()

  const fetchInstances = async () => {
    setLoading(true)
    try {
      const filterId = hostFilter || presetHost || undefined
      const res: any = filterId
        ? await instanceApi.listByHost(filterId)
        : await instanceApi.list()
      setInstances(res.data || [])
    } catch {
      setInstances([])
    } finally {
      setLoading(false)
    }
  }

  const fetchHosts = async () => {
    try {
      const res: any = await hostApi.list(100, 0)
      setHosts(res.data || [])
    } catch {
      setHosts([])
    }
  }

  useEffect(() => {
    fetchInstances()
    fetchHosts()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  useEffect(() => {
    if (hostFilter || presetHost) {
      fetchInstances()
    }
    if (presetHost) {
      form.setFieldsValue({ host_id: presetHost })
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [hostFilter, presetHost, hosts])

  const handleDelete = async (id: string) => {
    try {
      await instanceApi.delete(id)
      message.success('实例删除成功')
      fetchInstances()
    } catch {
      // interceptor already showed error
    }
  }

  const handleCreate = async () => {
    try {
      const values = await form.validateFields()
      setSubmitting(true)
      await instanceApi.create(values)
      message.success('实例创建成功')
      setModalOpen(false)
      form.resetFields()
      fetchInstances()
    } catch {
      // interceptor already showed error
    } finally {
      setSubmitting(false)
    }
  }

  const hostNameById = (id: string | null | undefined) => {
    if (!id) return '-'
    const h = hosts.find((x) => x.id === id)
    return h ? h.name : id.substring(0, 8)
  }

  const columns: ColumnsType<Instance> = [
    { title: '实例名称', dataIndex: 'name', key: 'name' },
    {
      title: '所属主机',
      dataIndex: 'host_id',
      key: 'host_id',
      render: (id) => hostNameById(id),
    },
    { title: '集群 ID', dataIndex: 'cluster_id', key: 'cluster_id', render: (v) => v || '-' },
    {
      title: '状态',
      key: 'status',
      render: (_, r) => {
        const role = r.status?.role
        const health = r.status?.health_status
        const run = r.status?.run_status
        if (health === 'healthy' || health === 'ok') {
          return <Tag color="success">健康{role ? ` (${role})` : ''}</Tag>
        }
        if (health === 'unhealthy' || health === 'failed') {
          return <Tag color="error">异常</Tag>
        }
        if (run === 'running') {
          return <Tag color="processing">运行中{role ? ` (${role})` : ''}</Tag>
        }
        if (run === 'stopped') {
          return <Tag color="default">已停止</Tag>
        }
        return <Tag>未检测</Tag>
      },
    },
    {
      title: '复制延迟',
      key: 'lag',
      render: (_, r) => {
        const lag = r.status?.seconds_behind_master
        if (lag === undefined || lag === null) return '-'
        if (lag > 30) return <Tag color="error">{lag}s</Tag>
        if (lag > 5) return <Tag color="warning">{lag}s</Tag>
        return <Tag color="success">{lag}s</Tag>
      },
    },
    {
      title: '创建时间',
      dataIndex: 'created_at',
      key: 'created_at',
      render: (t) => (t ? new Date(t).toLocaleString() : '-'),
    },
    {
      title: '操作',
      key: 'action',
      render: (_, r) => (
        <Space>
          <Button type="link" size="small" onClick={() => navigate(`/dashboard/instances/${r.id}`)}>
            详情
          </Button>
          <Button type="link" size="small" onClick={() => instanceApi.detectVersion(r.id).then(() => message.success('已触发版本检测')).catch(() => {})}>
            检测版本
          </Button>
          <Popconfirm
            title="确定删除该实例?"
            onConfirm={() => handleDelete(r.id)}
            okText="确定"
            cancelText="取消"
          >
            <Button type="link" size="small" danger>
              删除
            </Button>
          </Popconfirm>
        </Space>
      ),
    },
  ]

  const handleScanHost = async () => {
    if (!hostFilter) {
      message.warning('请先选择一台主机')
      return
    }
    try {
      const r: any = await hostApi.scanInstances(hostFilter)
      const taskId = r?.data?.task_id
      if (!taskId) {
        message.warning('后端未实现 scan-instances 接口, 请手动添加实例')
        return
      }
      message.info('已发起扫描, 正在跳转到主机详情查看结果')
      navigate(`/dashboard/hosts/${hostFilter}?tab=instances&scan_task=${taskId}`)
    } catch (err: any) {
      if (err?.response?.status === 404) {
        message.warning('后端未实现 scan-instances 接口, 请手动添加实例')
      } else {
        message.error('扫描发起失败')
      }
    }
  }

  const presetHostObj = hosts.find((h) => h.id === presetHost)

  return (
    <div style={{ padding: '24px' }}>
      {presetHostObj && (
        <Alert
          type="info"
          showIcon
          style={{ marginBottom: 16 }}
          message={`已按主机筛选: ${presetHostObj.name}`}
          description={
            <Space>
              <span>主机地址: {presetHostObj.address}:{presetHostObj.ssh_port}</span>
              <Button
                size="small"
                type="link"
                onClick={() => navigate(`/dashboard/hosts/${presetHostObj.id}`)}
              >
                打开主机详情
              </Button>
            </Space>
          }
          closable
        />
      )}
      <Card
        title={
          <Space>
            <DesktopOutlined />
            <span>实例管理</span>
          </Space>
        }
        extra={
          <Space>
            <Select
              placeholder="按主机筛选"
              allowClear
              style={{ width: 200 }}
              value={hostFilter}
              onChange={setHostFilter}
              options={hosts.map((h) => ({ value: h.id, label: h.name }))}
            />
            <Button
              icon={<ScanOutlined />}
              onClick={handleScanHost}
              disabled={!hostFilter}
            >
              扫描该主机
            </Button>
            <Button icon={<ReloadOutlined />} onClick={fetchInstances}>
              刷新
            </Button>
            <Button
              type="primary"
              icon={<PlusOutlined />}
              onClick={() => {
                form.resetFields()
                setModalOpen(true)
              }}
            >
              添加实例
            </Button>
          </Space>
        }
      >
        <Table
          columns={columns}
          dataSource={instances}
          rowKey="id"
          loading={loading}
          pagination={{ pageSize: 20 }}
          locale={{
            emptyText: (
              <Empty
                image={Empty.PRESENTED_IMAGE_SIMPLE}
                description={
                  <div>
                    <div style={{ marginBottom: 8 }}>暂无实例</div>
                    {hostFilter ? (
                      <Space>
                        <Button
                          type="primary"
                          icon={<ScanOutlined />}
                          onClick={handleScanHost}
                        >
                          自动扫描该主机
                        </Button>
                        <Button
                          icon={<PlusOutlined />}
                          onClick={() => {
                            form.resetFields()
                            setModalOpen(true)
                          }}
                        >
                          手动添加实例
                        </Button>
                      </Space>
                    ) : (
                      <Space>
                        <Button
                          type="primary"
                          icon={<DesktopOutlined />}
                          onClick={() => navigate('/dashboard/hosts/new')}
                        >
                          添加主机
                        </Button>
                        <Button
                          icon={<PlusOutlined />}
                          onClick={() => {
                            form.resetFields()
                            setModalOpen(true)
                          }}
                        >
                          手动添加实例
                        </Button>
                      </Space>
                    )}
                  </div>
                }
              />
            ),
          }}
        />
      </Card>

      <Modal
        title="添加实例"
        open={modalOpen}
        onCancel={() => setModalOpen(false)}
        onOk={handleCreate}
        confirmLoading={submitting}
        okText="创建"
        cancelText="取消"
        width={600}
      >
        <Form form={form} layout="vertical" autoComplete="off">
          <Form.Item
            name="name"
            label="实例名称"
            rules={[{ required: true, message: '请输入实例名称' }]}
          >
            <Input placeholder="例如: order-db-01" />
          </Form.Item>

          <Form.Item
            name="host_id"
            label="所属主机"
            rules={[{ required: true, message: '请选择所属主机' }]}
          >
            <Select
              placeholder="选择主机"
              options={hosts.map((h) => ({ value: h.id, label: `${h.name} (${h.address}:${h.ssh_port})` }))}
            />
          </Form.Item>

          <Form.Item
            name="host"
            label="连接地址"
            rules={[{ required: true, message: '请输入连接地址' }]}
          >
            <Input placeholder="例如: 192.168.1.100" />
          </Form.Item>

          <Form.Item
            name="port"
            label="端口"
            rules={[{ required: true, message: '请输入端口' }]}
            initialValue={3306}
          >
            <InputNumber min={1} max={65535} style={{ width: '100%' }} />
          </Form.Item>

          <Form.Item
            name="username"
            label="用户名"
            rules={[{ required: true, message: '请输入用户名' }]}
          >
            <Input placeholder="例如: root" />
          </Form.Item>

          <Form.Item
            name="password"
            label="密码"
            rules={[{ required: true, message: '请输入密码' }]}
          >
            <Input.Password placeholder="MySQL 密码" autoComplete="new-password" />
          </Form.Item>

          <Form.Item name="cluster_id" label="集群 ID (可选)">
            <Input placeholder="例如: mgr-cluster-01" />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}

export default InstanceList
