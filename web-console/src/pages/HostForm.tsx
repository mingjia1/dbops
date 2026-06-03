import React, { useEffect, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { Card, Form, Input, InputNumber, Select, Button, Space, message, Spin } from 'antd'
import { ArrowLeftOutlined } from '@ant-design/icons'
import { hostApi, Host } from '../services/api'

interface FormValues {
  name: string
  address: string
  ssh_port: number
  ssh_user: string
  ssh_auth_method: string
  ssh_credential: string
  os_type: string
  description?: string
  tags?: string
}

const HostForm: React.FC = () => {
  const navigate = useNavigate()
  const { id } = useParams<{ id: string }>()
  const isEdit = !!id
  const [form] = Form.useForm<FormValues>()
  const [loading, setLoading] = useState(false)
  const [submitting, setSubmitting] = useState(false)
  const [host, setHost] = useState<Host | null>(null)

  useEffect(() => {
    if (isEdit && id) {
      setLoading(true)
      hostApi
        .get(id)
        .then((res: any) => {
          const h: Host = res.data
          setHost(h)
          form.setFieldsValue({
            name: h.name,
            address: h.address,
            ssh_port: h.ssh_port,
            ssh_user: h.ssh_user,
            ssh_auth_method: h.ssh_auth_method,
            os_type: h.os_type,
            description: h.description,
            tags: h.tags,
          })
        })
        .catch(() => {
          message.error('主机不存在')
          navigate('/dashboard/hosts')
        })
        .finally(() => setLoading(false))
    } else {
      form.setFieldsValue({
        ssh_port: 22,
        ssh_auth_method: 'password',
        os_type: 'linux',
      })
    }
  }, [id, isEdit, form, navigate])

  const onFinish = async (values: FormValues) => {
    setSubmitting(true)
    try {
      if (isEdit && id) {
        await hostApi.update(id, values)
        message.success('主机更新成功')
      } else {
        await hostApi.create(values)
        message.success('主机创建成功')
      }
      navigate('/dashboard/hosts')
    } catch {
      // interceptor already showed error
    } finally {
      setSubmitting(false)
    }
  }

  if (loading) {
    return (
      <div style={{ padding: '24px', textAlign: 'center' }}>
        <Spin />
      </div>
    )
  }

  return (
    <div style={{ padding: '24px' }}>
      <Card
        title={
          <Space>
            <Button type="text" icon={<ArrowLeftOutlined />} onClick={() => navigate('/dashboard/hosts')} />
            <span>{isEdit ? `编辑主机 - ${host?.name}` : '添加主机'}</span>
          </Space>
        }
      >
        <Form
          form={form}
          layout="vertical"
          onFinish={onFinish}
          style={{ maxWidth: 800 }}
          autoComplete="off"
        >
          <Form.Item
            name="name"
            label="主机名称"
            rules={[{ required: true, message: '请输入主机名称' }]}
          >
            <Input placeholder="例如: db-prod-01" disabled={isEdit} />
          </Form.Item>

          <Form.Item
            name="address"
            label="主机地址 (IP 或域名)"
            rules={[{ required: true, message: '请输入主机地址' }]}
          >
            <Input placeholder="例如: 192.168.1.100" />
          </Form.Item>

          <Form.Item
            name="ssh_port"
            label="SSH 端口"
            rules={[{ required: true, message: '请输入 SSH 端口' }]}
          >
            <InputNumber min={1} max={65535} style={{ width: 200 }} />
          </Form.Item>

          <Form.Item
            name="ssh_user"
            label="SSH 用户名"
            rules={[{ required: true, message: '请输入 SSH 用户名' }]}
          >
            <Input placeholder="例如: root" />
          </Form.Item>

          <Form.Item
            name="ssh_auth_method"
            label="认证方式"
            rules={[{ required: true, message: '请选择认证方式' }]}
          >
            <Select
              options={[
                { value: 'password', label: '密码' },
                { value: 'private_key', label: '私钥' },
              ]}
            />
          </Form.Item>

          <Form.Item
            name="ssh_credential"
            label={isEdit ? 'SSH 凭据 (留空则不修改)' : 'SSH 凭据 (密码或私钥)'}
            rules={isEdit ? [] : [{ required: true, message: '请输入 SSH 凭据' }]}
          >
            <Input.Password
              placeholder={isEdit ? '留空表示不修改现有凭据' : '密码或 PEM 格式私钥'}
              autoComplete="new-password"
            />
          </Form.Item>

          <Form.Item name="os_type" label="操作系统类型">
            <Select
              options={[
                { value: 'linux', label: 'Linux' },
                { value: 'centos', label: 'CentOS' },
                { value: 'ubuntu', label: 'Ubuntu' },
                { value: 'debian', label: 'Debian' },
                { value: 'rhel', label: 'RHEL' },
                { value: 'rocky', label: 'Rocky' },
              ]}
            />
          </Form.Item>

          <Form.Item name="description" label="描述">
            <Input.TextArea rows={3} placeholder="可选: 主机用途、所在机房等" />
          </Form.Item>

          <Form.Item name="tags" label="标签">
            <Input placeholder="可选: 用逗号分隔,例如: prod,mysql,master" />
          </Form.Item>

          <Form.Item>
            <Space>
              <Button type="primary" htmlType="submit" loading={submitting}>
                {isEdit ? '保存' : '创建'}
              </Button>
              <Button onClick={() => navigate('/dashboard/hosts')}>取消</Button>
            </Space>
          </Form.Item>
        </Form>
      </Card>
    </div>
  )
}

export default HostForm
