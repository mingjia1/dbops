import React, { useState, useEffect } from 'react'
import { Card, Table, Button, Modal, Form, Input, Select, Space, Tag, message, Popconfirm, Switch } from 'antd'
import { UserOutlined, PlusOutlined, EditOutlined, DeleteOutlined } from '@ant-design/icons'
import api from '../services/api'

interface User {
  id: string
  username: string
  email: string
  role: string
  status: string
  created_at: string
}

export default function UserManagePage() {
  const [users, setUsers] = useState<User[]>([])
  const [loading, setLoading] = useState(false)
  const [editOpen, setEditOpen] = useState(false)
  const [editing, setEditing] = useState<User | null>(null)
  const [form] = Form.useForm()

  const fetchUsers = async () => {
    setLoading(true)
    try { const res = await api.get('/users'); setUsers(res.data?.data || []) }
    catch { message.error('获取用户列表失败') }
    finally { setLoading(false) }
  }
  useEffect(() => { fetchUsers() }, [])

  const handleSave = async (values: any) => {
    try {
      if (editing) { await api.put(`/users/${editing.id}`, values); message.success('更新成功') }
      else { await api.post('/users', values); message.success('创建成功') }
      setEditOpen(false); form.resetFields(); setEditing(null); fetchUsers()
    } catch (err: any) { message.error(err.message) }
  }
  const handleDelete = async (id: string) => {
    try { await api.delete(`/users/${id}`); message.success('已删除'); fetchUsers() }
    catch (err: any) { message.error(err.message) }
  }

  const columns = [
    { title: '用户名', dataIndex: 'username', key: 'username' },
    { title: '邮箱', dataIndex: 'email', key: 'email', ellipsis: true },
    { title: '角色', dataIndex: 'role', key: 'role', render: (v: string) => <Tag color={v === 'admin' ? 'red' : 'blue'}>{v}</Tag> },
    { title: '状态', dataIndex: 'status', key: 'status', render: (v: string) => <Tag color={v === 'active' ? 'green' : 'default'}>{v}</Tag> },
    { title: '创建时间', dataIndex: 'created_at', key: 'created_at', width: 160, render: (v: string) => new Date(v).toLocaleString() },
    { title: '操作', key: 'action', render: (_: any, r: User) => (
      <Space>
        <Button size="small" icon={<EditOutlined />} onClick={() => { setEditing(r); form.setFieldsValue(r); setEditOpen(true) }}>编辑</Button>
        <Popconfirm title="确认删除此用户？" onConfirm={() => handleDelete(r.id)}>
          <Button size="small" danger icon={<DeleteOutlined />}>删除</Button>
        </Popconfirm>
      </Space>
    )},
  ]

  return (
    <div style={{ padding: 24 }}>
      <Card title={<><UserOutlined /> 用户管理</>}
        extra={<Button type="primary" icon={<PlusOutlined />} onClick={() => { setEditing(null); form.resetFields(); setEditOpen(true) }}>新建用户</Button>}>
        <Table columns={columns} dataSource={users} rowKey="id" loading={loading} size="small" />
      </Card>
      <Modal title={editing ? '编辑用户' : '新建用户'} open={editOpen} onCancel={() => setEditOpen(false)}
        onOk={() => form.validateFields().then(handleSave)}>
        <Form form={form} layout="vertical">
          <Form.Item name="username" label="用户名" rules={[{ required: true }]}><Input /></Form.Item>
          <Form.Item name="email" label="邮箱" rules={[{ type: 'email' }]}><Input /></Form.Item>
          {!editing && <Form.Item name="password" label="密码" rules={[{ required: true }]}><Input.Password /></Form.Item>}
          <Form.Item name="role" label="角色" rules={[{ required: true }]}>
            <Select options={[{ label: '管理员', value: 'admin' }, { label: '操作员', value: 'operator' }, { label: '只读', value: 'viewer' }]} />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}
