import React, { useState, useEffect } from 'react'
import { Card, Table, Button, Modal, Form, Input, Select, Space, Tag, message, Popconfirm, Switch } from 'antd'
import { EyeInvisibleOutlined, PlusOutlined, EditOutlined, DeleteOutlined } from '@ant-design/icons'
import api from '../services/api'

interface MaskingRule {
  id: string
  name: string
  description?: string
  field_path: string
  pattern?: string
  algorithm: string
  replacement?: string
  roles: string
  enabled: boolean
}

export default function MaskingPage() {
  const [rules, setRules] = useState<MaskingRule[]>([])
  const [loading, setLoading] = useState(false)
  const [editOpen, setEditOpen] = useState(false)
  const [editing, setEditing] = useState<MaskingRule | null>(null)
  const [form] = Form.useForm()

  const fetchRules = async () => {
    setLoading(true)
    try { const res = await api.get('/masking'); setRules(res.data || []) }
    catch { message.error('获取脱敏规则失败') }
    finally { setLoading(false) }
  }
  useEffect(() => { fetchRules() }, [])

  const handleSave = async (values: any) => {
    try {
      if (editing) { await api.put(`/masking/${editing.id}`, values); message.success('更新成功') }
      else { await api.post('/masking', values); message.success('创建成功') }
      setEditOpen(false); form.resetFields(); setEditing(null); fetchRules()
    } catch (err: any) { message.error(err.message) }
  }

  const handleDelete = async (id: string) => {
    try { await api.delete(`/masking/${id}`); message.success('已删除'); fetchRules() }
    catch (err: any) { message.error(err.message) }
  }

  const columns = [
    { title: '名称', dataIndex: 'name', key: 'name' },
    { title: '字段路径', dataIndex: 'field_path', key: 'field_path', ellipsis: true },
    { title: '算法', dataIndex: 'algorithm', key: 'algorithm', render: (v: string) => <Tag>{v}</Tag> },
    { title: '适用角色', dataIndex: 'roles', key: 'roles', render: (v: string) => <Tag color="blue">{v}</Tag> },
    { title: '状态', dataIndex: 'enabled', key: 'enabled', render: (v: boolean) => v ? <Tag color="green">启用</Tag> : <Tag>禁用</Tag> },
    { title: '操作', key: 'action', render: (_: any, r: MaskingRule) => (
      <Space>
        <Button size="small" icon={<EditOutlined />} onClick={() => { setEditing(r); form.setFieldsValue(r); setEditOpen(true) }}>编辑</Button>
        <Popconfirm title="确认删除？" onConfirm={() => handleDelete(r.id)}>
          <Button size="small" danger icon={<DeleteOutlined />}>删除</Button>
        </Popconfirm>
      </Space>
    )},
  ]

  return (
    <div style={{ padding: 24 }}>
      <Card title={<><EyeInvisibleOutlined /> 数据脱敏规则</>}
        extra={<Button type="primary" icon={<PlusOutlined />} onClick={() => { setEditing(null); form.resetFields(); setEditOpen(true) }}>新建规则</Button>}>
        <Table columns={columns} dataSource={rules} rowKey="id" loading={loading} size="small" />
      </Card>
      <Modal title={editing ? '编辑规则' : '新建规则'} open={editOpen} onCancel={() => setEditOpen(false)}
        onOk={() => form.validateFields().then(handleSave)}>
        <Form form={form} layout="vertical">
          <Form.Item name="name" label="名称" rules={[{ required: true }]}><Input /></Form.Item>
          <Form.Item name="field_path" label="字段路径" rules={[{ required: true }]}><Input placeholder="如 user.email 或 order.card_number" /></Form.Item>
          <Form.Item name="algorithm" label="脱敏算法" rules={[{ required: true }]}>
            <Select options={[
              { label: '掩码 (mask)', value: 'mask' },
              { label: '哈希 (hash)', value: 'hash' },
              { label: '替换 (replace)', value: 'replace' },
              { label: '截断 (truncate)', value: 'truncate' },
            ]} />
          </Form.Item>
          <Form.Item name="replacement" label="替换值"><Input placeholder="仅 replace 算法需要" /></Form.Item>
          <Form.Item name="roles" label="适用角色" initialValue="*"><Input placeholder="* 表示所有角色" /></Form.Item>
          <Form.Item name="description" label="描述"><Input.TextArea rows={2} /></Form.Item>
        </Form>
      </Modal>
    </div>
  )
}
