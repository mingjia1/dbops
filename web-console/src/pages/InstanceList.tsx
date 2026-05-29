import React, { useEffect, useState } from 'react'
import { Table, Button, Space, Modal, Form, Input, InputNumber, Switch, message, Tag } from 'antd'
import { PlusOutlined, DeleteOutlined, ReloadOutlined } from '@ant-design/icons'
import { useDispatch, useSelector } from 'react-redux'
import { fetchInstances, createInstance, deleteInstance, detectVersion } from '@/store/instanceSlice'
import type { RootState } from '@/store'

const InstanceList: React.FC = () => {
  const dispatch = useDispatch()
  const { instances, loading } = useSelector((state: RootState) => state.instances as any)
  const [modalVisible, setModalVisible] = useState(false)
  const [form] = Form.useForm()

  useEffect(() => {
    dispatch(fetchInstances() as any)
  }, [dispatch])

  const handleCreate = async (values: any) => {
    try {
      await dispatch(createInstance(values) as any)
      message.success('实例创建成功')
      setModalVisible(false)
      form.resetFields()
    } catch (err) {
      message.error('创建失败')
    }
  }

  const handleDelete = (id: string) => {
    Modal.confirm({
      title: '确认删除',
      content: '确定要删除此实例吗？',
      onOk: async () => {
        await dispatch(deleteInstance(id) as any)
        message.success('删除成功')
      },
    })
  }

  const handleDetectVersion = async (id: string) => {
    try {
      await dispatch(detectVersion(id) as any)
      message.success('版本识别完成')
    } catch (err) {
      message.error('版本识别失败')
    }
  }

  const columns = [
    {
      title: 'ID',
      dataIndex: 'id',
      key: 'id',
      width: 150,
    },
    {
      title: '名称',
      dataIndex: 'name',
      key: 'name',
    },
    {
      title: '集群',
      dataIndex: 'cluster_id',
      key: 'cluster_id',
      render: (text: string) => text || <Tag color="default">单点</Tag>,
    },
    {
      title: '创建时间',
      dataIndex: 'created_at',
      key: 'created_at',
      width: 180,
    },
    {
      title: '操作',
      key: 'action',
      width: 200,
      render: (_: any, record: any) => (
        <Space>
          <Button size="small" icon={<ReloadOutlined />} onClick={() => handleDetectVersion(record.id)}>
            识别版本
          </Button>
          <Button size="small" danger icon={<DeleteOutlined />} onClick={() => handleDelete(record.id)}>
            删除
          </Button>
        </Space>
      ),
    },
  ]

  return (
    <div>
      <div style={{ marginBottom: 16 }}>
        <Button type="primary" icon={<PlusOutlined />} onClick={() => setModalVisible(true)}>
          新建实例
        </Button>
      </div>

      <Table
        columns={columns}
        dataSource={instances}
        rowKey="id"
        loading={loading}
      />

      <Modal
        title="新建实例"
        open={modalVisible}
        onCancel={() => setModalVisible(false)}
        onOk={() => form.submit()}
      >
        <Form form={form} layout="vertical" onFinish={handleCreate}>
          <Form.Item name="name" label="实例名称" rules={[{ required: true }]}>
            <Input />
          </Form.Item>
          <Form.Item name="host" label="主机地址" rules={[{ required: true }]}>
            <Input placeholder="192.168.1.100" />
          </Form.Item>
          <Form.Item name="port" label="端口" rules={[{ required: true }]} initialValue={3306}>
            <InputNumber min={1} max={65535} />
          </Form.Item>
          <Form.Item name="username" label="用户名" rules={[{ required: true }]} initialValue="root">
            <Input />
          </Form.Item>
          <Form.Item name="password" label="密码" rules={[{ required: true }]}>
            <Input.Password />
          </Form.Item>
          <Form.Item name="cluster_id" label="集群ID">
            <Input placeholder="可选" />
          </Form.Item>
          <Form.Item name="ssl_enabled" label="SSL" valuePropName="checked">
            <Switch />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}

export default InstanceList