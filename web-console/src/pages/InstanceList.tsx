import React, { useEffect, useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { Card, Table, Button, Space, Tag, Modal, Form, Input, InputNumber, Select, message, Popconfirm } from 'antd'
import { PlusOutlined, ReloadOutlined } from '@ant-design/icons'
import type { ColumnsType } from 'antd/es/table'
import { instanceApi, hostApi, Instance, Host } from '../services/api'

const InstanceList: React.FC = () => {
  const [searchParams] = useSearchParams()
  const presetHost = searchParams.get('preset_host')

  const [instances, setInstances] = useState<Instance[]>([])
  const [hosts, setHosts] = useState<Host[]>([])
  const [loading, setLoading] = useState(false)
  const [hostFilter, setHostFilter] = useState<string | undefined>(presetHost || undefined)
  const [modalOpen, setModalOpen] = useState(false)
  const [submitting, setSubmitting] = useState(false)
  const [form] = Form.useForm()

  const fetchInstances = async () => {
    setLoading(true)
    try {
      const res: any = await instanceApi.list()
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
  }, [])

  useEffect(() => {
    if (presetHost) {
      form.setFieldsValue({ host_id: presetHost })
    }
  }, [presetHost, hosts])

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

  const filtered = hostFilter ? instances.filter((i) => i.host_id === hostFilter) : instances

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
      render: () => <Tag>未检测</Tag>,
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
          <Button type="link" size="small" onClick={() => message.info('详情页待接入')}>
            详情
          </Button>
          <Button type="link" size="small" onClick={() => instanceApi.detectVersion(r.id).then(() => message.success('已触发版本检测'))}>
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

  return (
    <div style={{ padding: '24px' }}>
      <Card
        title="实例管理"
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
          dataSource={filtered}
          rowKey="id"
          loading={loading}
          pagination={{ pageSize: 20 }}
          locale={{ emptyText: '暂无实例,请先添加主机,然后再添加实例' }}
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
