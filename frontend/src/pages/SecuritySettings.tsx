import React, { useState, useEffect } from 'react'
import { Button, Card, Form, Input, message, Space, Switch, Typography } from 'antd'
import { LockOutlined, SafetyOutlined } from '@ant-design/icons'

const { Text } = Typography
const STORAGE_KEY = 'dbops_credential_password'

const SecuritySettings: React.FC = () => {
  const [enabled, setEnabled] = useState(false)
  const [form] = Form.useForm()

  useEffect(() => {
    const stored = localStorage.getItem(STORAGE_KEY)
    if (stored) {
      setEnabled(true)
      form.setFieldsValue({ password: stored })
    }
  }, [form])

  const handleSave = async () => {
    const values = await form.validateFields()
    if (values.password !== values.confirm_password) {
      message.error('两次输入的密码不一致')
      return
    }
    localStorage.setItem(STORAGE_KEY, values.password)
    setEnabled(true)
    message.success('二级密码已设置')
  }

  const handleDisable = () => {
    localStorage.removeItem(STORAGE_KEY)
    sessionStorage.removeItem('dbops_credential_verified')
    setEnabled(false)
    form.resetFields()
    message.success('二级密码已关闭')
  }

  return (
    <div style={{ padding: 24 }}>
      <Card
        title={<Space><SafetyOutlined /><span>安全设置</span></Space>}
      >
        <Card type="inner" title="实例密码查看保护">
          <div style={{ marginBottom: 16 }}>
            <Space align="center">
              <Switch checked={enabled} onChange={(checked) => { if (!checked) handleDisable() }} />
              <Text>{enabled ? '已开启' : '未开启'}</Text>
            </Space>
            <Text type="secondary" style={{ display: 'block', marginTop: 8 }}>
              开启后，在实例管理页面查看实例密码时需要输入二级密码验证。验证在当前浏览器会话内有效，关闭浏览器后需重新验证。
            </Text>
          </div>
          <Form form={form} layout="vertical" style={{ maxWidth: 400 }}>
            <Form.Item name="password" label="二级密码" rules={[{ required: true, message: '请输入二级密码' }]}>
              <Input.Password placeholder="设置二级密码" autoComplete="new-password" />
            </Form.Item>
            <Form.Item name="confirm_password" label="确认二级密码" rules={[{ required: true, message: '请确认二级密码' }]}>
              <Input.Password placeholder="再次输入" autoComplete="new-password" />
            </Form.Item>
            <Form.Item>
              <Space>
                <Button type="primary" icon={<LockOutlined />} onClick={handleSave}>
                  {enabled ? '更新密码' : '开启保护'}
                </Button>
                {enabled && (
                  <Button danger onClick={handleDisable}>关闭保护</Button>
                )}
              </Space>
            </Form.Item>
          </Form>
        </Card>
      </Card>
    </div>
  )
}

export default SecuritySettings
