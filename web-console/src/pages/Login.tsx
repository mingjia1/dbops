import React, { useState } from 'react'
import { Form, Input, Button, Card, message } from 'antd'
import { UserOutlined, LockOutlined } from '@ant-design/icons'
import { useNavigate } from 'react-router-dom'
import axios from 'axios'

const Login: React.FC = () => {
  const [loading, setLoading] = useState(false)
  const navigate = useNavigate()

  const onFinish = async (values: any) => {
    setLoading(true)
    try {
      const response = await axios.post('/api/v1/auth/login', {
        username: values.username,
        password: values.password,
      })

      if (response.data.code === 200) {
        const { token, user } = response.data.data
        localStorage.setItem('token', token)
        localStorage.setItem('user', JSON.stringify(user))
        
        message.success('登录成功')
        navigate('/dashboard')
      } else {
        message.error(response.data.message || '登录失败')
      }
    } catch (error: any) {
      console.error('Login error:', error)
      message.error(error.response?.data?.message || '登录失败，请检查网络连接')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div style={{ 
      display: 'flex', 
      justifyContent: 'center', 
      alignItems: 'center', 
      minHeight: '100vh',
      background: '#f0f2f5'
    }}>
      <Card title="MySQL 运维平台" style={{ width: 400 }}>
        <Form
          name="login"
          onFinish={onFinish}
          autoComplete="off"
        >
          <Form.Item
            name="username"
            rules={[{ required: true, message: '请输入用户名' }]}
          >
            <Input prefix={<UserOutlined />} placeholder="用户名" />
          </Form.Item>

          <Form.Item
            name="password"
            rules={[{ required: true, message: '请输入密码' }]}
          >
            <Input.Password prefix={<LockOutlined />} placeholder="密码" />
          </Form.Item>

          <Form.Item>
            <Button type="primary" htmlType="submit" block loading={loading}>
              登录
            </Button>
          </Form.Item>
        </Form>
        
        <div style={{ marginTop: 16, textAlign: 'center', color: '#999', fontSize: 12 }}>
          <p>Standalone 模式：任意用户名密码均可登录</p>
        </div>
      </Card>
    </div>
  )
}

export default Login