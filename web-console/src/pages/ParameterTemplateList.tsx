import React, { useEffect, useState } from 'react'
import { Table, Button, Space, Modal, Form, Input, Select, message, Tag } from 'antd'
import { PlusOutlined, EditOutlined, DeleteOutlined } from '@ant-design/icons'
import { parameterTemplateApi, ParameterTemplate } from '@/services/api'

const ParameterTemplateList: React.FC = () => {
  const [templates, setTemplates] = useState<ParameterTemplate[]>([])
  const [loading, setLoading] = useState(false)
  const [modalVisible, setModalVisible] = useState(false)
  const [editingTemplate, setEditingTemplate] = useState<ParameterTemplate | null>(null)
  const [form] = Form.useForm()

  useEffect(() => {
    loadTemplates()
  }, [])

  const loadTemplates = async () => {
    setLoading(true)
    try {
      const data = await parameterTemplateApi.list() as any
      setTemplates(data)
    } catch (err) {
      message.error('加载参数模板失败')
    } finally {
      setLoading(false)
    }
  }

  const handleCreate = () => {
    setEditingTemplate(null)
    form.resetFields()
    setModalVisible(true)
  }

  const handleEdit = (template: ParameterTemplate) => {
    setEditingTemplate(template)
    form.setFieldsValue(template)
    setModalVisible(true)
  }

  const handleDelete = (id: string) => {
    Modal.confirm({
      title: '确认删除',
      content: '确定要删除此参数模板吗？',
      onOk: async () => {
        await parameterTemplateApi.delete(id)
        message.success('删除成功')
        loadTemplates()
      },
    })
  }

  const handleSubmit = async (values: any) => {
    try {
      if (editingTemplate) {
        await parameterTemplateApi.update(editingTemplate.id, values)
        message.success('更新成功')
      } else {
        await parameterTemplateApi.create(values)
        message.success('创建成功')
      }
      setModalVisible(false)
      loadTemplates()
    } catch (err) {
      message.error('操作失败')
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
      title: '模板名称',
      dataIndex: 'name',
      key: 'name',
    },
    {
      title: '分类',
      dataIndex: 'category',
      key: 'category',
      render: (text: string) => <Tag color="blue">{text}</Tag>,
    },
    {
      title: '描述',
      dataIndex: 'description',
      key: 'description',
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
      width: 180,
      render: (_: any, record: ParameterTemplate) => (
        <Space>
          <Button size="small" icon={<EditOutlined />} onClick={() => handleEdit(record)}>
            编辑
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
        <Button type="primary" icon={<PlusOutlined />} onClick={handleCreate}>
          新建模板
        </Button>
      </div>

      <Table
        columns={columns}
        dataSource={templates}
        rowKey="id"
        loading={loading}
      />

      <Modal
        title={editingTemplate ? '编辑模板' : '新建模板'}
        open={modalVisible}
        onCancel={() => setModalVisible(false)}
        onOk={() => form.submit()}
      >
        <Form form={form} layout="vertical" onFinish={handleSubmit}>
          <Form.Item name="name" label="模板名称" rules={[{ required: true }]}>
            <Input />
          </Form.Item>
          <Form.Item name="category" label="分类" rules={[{ required: true }]}>
            <Select>
              <Select.Option value="performance">性能优化</Select.Option>
              <Select.Option value="security">安全配置</Select.Option>
              <Select.Option value="ha">高可用</Select.Option>
              <Select.Option value="general">通用配置</Select.Option>
            </Select>
          </Form.Item>
          <Form.Item name="description" label="描述">
            <Input.TextArea rows={3} />
          </Form.Item>
          <Form.Item name="parameters" label="参数配置" rules={[{ required: true }]}>
            <Input.TextArea rows={5} placeholder="innodb_buffer_pool_size=1G&#10;max_connections=1000" />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}

export default ParameterTemplateList