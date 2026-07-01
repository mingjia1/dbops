import { useEffect, useMemo, useState } from 'react'
import { Button, Card, Form, Input, Modal, Popconfirm, Select, Space, Switch, Table, Tag, message } from 'antd'
import { DeleteOutlined, EditOutlined, KeyOutlined, PlusOutlined, UserOutlined } from '@ant-design/icons'
import { PlatformRole, roleApi, userApi } from '../services/api'

interface UserRole {
  name: string
  display_name?: string
}

interface User {
  id: string
  username: string
  email: string
  role: string
  roles?: UserRole[]
  status: string
  display_name?: string
  phone?: string
  source?: string
  last_login_at?: string
  created_at: string
}

export default function UserManagePage() {
  const [users, setUsers] = useState<User[]>([])
  const [roles, setRoles] = useState<PlatformRole[]>([])
  const [loading, setLoading] = useState(false)
  const [editOpen, setEditOpen] = useState(false)
  const [resetOpen, setResetOpen] = useState(false)
  const [editing, setEditing] = useState<User | null>(null)
  const [resetting, setResetting] = useState<User | null>(null)
  const [form] = Form.useForm()
  const [resetForm] = Form.useForm()

  const roleOptions = useMemo(() => roles.map(role => ({
    label: role.display_name || role.name,
    value: role.name,
  })), [roles])

  const fetchData = async () => {
    setLoading(true)
    try {
      const [userRes, roleRes]: any[] = await Promise.all([userApi.list(), roleApi.list()])
      setUsers(userRes.data || [])
      setRoles(roleRes.data || [])
    } catch {
      message.error('获取用户数据失败')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => { fetchData() }, [])

  const openCreate = () => {
    setEditing(null)
    form.resetFields()
    form.setFieldsValue({ status: 'active', roles: ['operator'], role: 'operator' })
    setEditOpen(true)
  }

  const openEdit = (user: User) => {
    const userRoles = user.roles?.map(role => role.name) || [user.role]
    setEditing(user)
    form.setFieldsValue({ ...user, roles: userRoles, role: userRoles[0] || user.role })
    setEditOpen(true)
  }

  const handleSave = async (values: any) => {
    const roles = values.roles?.length ? values.roles : [values.role || 'operator']
    const payload = { ...values, roles, role: roles[0] }
    try {
      if (editing) {
        await userApi.update(editing.id, payload)
        await userApi.updateRoles(editing.id, roles)
        message.success('更新成功')
      } else {
        await userApi.create(payload)
        message.success('创建成功')
      }
      setEditOpen(false)
      fetchData()
    } catch (err: any) {
      message.error(err.response?.data?.message || err.message || '保存失败')
    }
  }

  const handleStatus = async (user: User, active: boolean) => {
    try {
      if (active) await userApi.enable(user.id)
      else await userApi.disable(user.id)
      message.success(active ? '已启用' : '已禁用')
      fetchData()
    } catch (err: any) {
      message.error(err.response?.data?.message || err.message || '状态更新失败')
    }
  }

  const handleResetPassword = async (values: any) => {
    if (!resetting) return
    try {
      await userApi.resetPassword(resetting.id, values.new_password)
      message.success('密码已重置')
      setResetOpen(false)
      resetForm.resetFields()
    } catch (err: any) {
      message.error(err.response?.data?.message || err.message || '密码重置失败')
    }
  }

  const handleDelete = async (id: string) => {
    try {
      await userApi.delete(id)
      message.success('已删除')
      fetchData()
    } catch (err: any) {
      message.error(err.response?.data?.message || err.message || '删除失败')
    }
  }

  const columns = [
    { title: '用户名', dataIndex: 'username', key: 'username', render: (_: string, r: User) => (
      <Space direction="vertical" size={0}>
        <span>{r.display_name || r.username}</span>
        <span style={{ color: '#8c8c8c', fontSize: 12 }}>{r.username}</span>
      </Space>
    ) },
    { title: '邮箱', dataIndex: 'email', key: 'email', ellipsis: true },
    { title: '角色', key: 'roles', render: (_: any, r: User) => (
      <Space wrap>
        {(r.roles?.length ? r.roles : [{ name: r.role }]).map(role => (
          <Tag key={role.name} color={role.name === 'admin' ? 'red' : 'blue'}>{role.display_name || role.name}</Tag>
        ))}
      </Space>
    ) },
    { title: '来源', dataIndex: 'source', key: 'source', width: 90, render: (v: string) => <Tag>{v || 'local'}</Tag> },
    { title: '状态', dataIndex: 'status', key: 'status', width: 100, render: (_: string, r: User) => (
      <Switch checked={r.status === 'active'} checkedChildren="启用" unCheckedChildren="禁用" onChange={v => handleStatus(r, v)} />
    ) },
    { title: '最后登录', dataIndex: 'last_login_at', key: 'last_login_at', width: 170, render: (v: string) => v ? new Date(v).toLocaleString() : '-' },
    { title: '操作', key: 'action', width: 240, render: (_: any, r: User) => (
      <Space>
        <Button size="small" icon={<EditOutlined />} onClick={() => openEdit(r)}>编辑</Button>
        <Button size="small" icon={<KeyOutlined />} onClick={() => { setResetting(r); resetForm.resetFields(); setResetOpen(true) }}>重置密码</Button>
        <Popconfirm title="确认删除此用户？" onConfirm={() => handleDelete(r.id)}>
          <Button size="small" danger icon={<DeleteOutlined />} />
        </Popconfirm>
      </Space>
    ) },
  ]

  return (
    <div style={{ padding: 24 }}>
      <Card
        title={<Space><UserOutlined /><span>用户与认证管理</span></Space>}
        extra={<Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>新建用户</Button>}
      >
        <Table columns={columns} dataSource={users} rowKey="id" loading={loading} size="small" />
      </Card>

      <Modal title={editing ? '编辑用户' : '新建用户'} open={editOpen} onCancel={() => setEditOpen(false)} onOk={() => form.validateFields().then(handleSave)}>
        <Form form={form} layout="vertical">
          <Form.Item name="username" label="用户名" rules={[{ required: true }]}><Input /></Form.Item>
          <Form.Item name="display_name" label="显示名称"><Input /></Form.Item>
          <Form.Item name="email" label="邮箱" rules={[{ required: true, type: 'email' }]}><Input /></Form.Item>
          <Form.Item name="phone" label="手机号"><Input /></Form.Item>
          {!editing && <Form.Item name="password" label="密码" rules={[{ required: true, min: 6 }]}><Input.Password /></Form.Item>}
          <Form.Item name="roles" label="角色" rules={[{ required: true }]}>
            <Select mode="multiple" options={roleOptions} />
          </Form.Item>
          <Form.Item name="status" label="状态" rules={[{ required: true }]}>
            <Select options={[{ label: '启用', value: 'active' }, { label: '禁用', value: 'disabled' }]} />
          </Form.Item>
        </Form>
      </Modal>

      <Modal title={`重置密码 - ${resetting?.username || ''}`} open={resetOpen} onCancel={() => setResetOpen(false)} onOk={() => resetForm.validateFields().then(handleResetPassword)}>
        <Form form={resetForm} layout="vertical">
          <Form.Item name="new_password" label="新密码" rules={[{ required: true, min: 6 }]}>
            <Input.Password />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}
