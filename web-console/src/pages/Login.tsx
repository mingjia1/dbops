import React, { useState } from 'react'
import { Form, Input, Button, Tabs, message } from 'antd'
import { UserOutlined, LockOutlined, MailOutlined, TeamOutlined } from '@ant-design/icons'
import { useNavigate } from 'react-router-dom'
import axios from 'axios'
import './Login.css'

const Login: React.FC = () => {
  const [loading, setLoading] = useState(false)
  const navigate = useNavigate()

  const onLogin = async (values: any) => {
    setLoading(true)
    try {
      const response = await axios.post('/api/v1/auth/login', {
        username: values.username,
        password: values.password,
      })
      if (response.data && response.data.code === 200 && response.data.data) {
        const { token, user } = response.data.data
        localStorage.setItem('token', token)
        localStorage.setItem('user', JSON.stringify(user))
        message.success('登录成功')
        setTimeout(() => navigate('/dashboard', { replace: true }), 200)
      } else {
        message.error(response.data?.message || '登录失败')
      }
    } catch (error: any) {
      message.error(error.response?.data?.message || '登录请求失败')
    } finally {
      setLoading(false)
    }
  }

  const onRegister = async (values: any) => {
    setLoading(true)
    try {
      const response = await axios.post('/api/v1/auth/register', {
        username: values.username,
        password: values.password,
        email: values.email,
        role: 'admin',
      })
      if (response.data && response.data.code === 200) {
        message.success('注册成功，请登录')
      } else {
        message.error(response.data?.message || '注册失败')
      }
    } catch (error: any) {
      message.error(error.response?.data?.message || '注册请求失败')
    } finally {
      setLoading(false)
    }
  }

  const tabItems = [
    {
      key: 'login',
      label: '登录',
      children: (
        <Form name="login" onFinish={onLogin} layout="vertical" size="large" autoComplete="off">
          <Form.Item name="username" rules={[{ required: true, message: '请输入用户名' }]}>
            <Input prefix={<UserOutlined />} placeholder="用户名" />
          </Form.Item>
          <Form.Item name="password" rules={[{ required: true, message: '请输入密码' }]}>
            <Input.Password prefix={<LockOutlined />} placeholder="密码" />
          </Form.Item>
          <Form.Item>
            <Button type="primary" htmlType="submit" block loading={loading} className="login-btn">
              登 录
            </Button>
          </Form.Item>
        </Form>
      ),
    },
    {
      key: 'register',
      label: '注册',
      children: (
        <Form name="register" onFinish={onRegister} layout="vertical" size="large" autoComplete="off">
          <Form.Item name="username" rules={[{ required: true, message: '请输入用户名' }]}>
            <Input prefix={<UserOutlined />} placeholder="用户名" />
          </Form.Item>
          <Form.Item name="email" rules={[{ required: true, type: 'email', message: '请输入有效邮箱' }]}>
            <Input prefix={<MailOutlined />} placeholder="邮箱" />
          </Form.Item>
          <Form.Item name="password" rules={[{ required: true, message: '请输入密码' }]}>
            <Input.Password prefix={<LockOutlined />} placeholder="密码" />
          </Form.Item>
          <Form.Item name="confirm" dependencies={['password']} rules={[{ required: true, message: '请确认密码' }, ({ getFieldValue }) => ({ validator(_, value) { if (!value || getFieldValue('password') === value) return Promise.resolve(); return Promise.reject(new Error('两次密码不一致')); } })]}>
            <Input.Password prefix={<LockOutlined />} placeholder="确认密码" />
          </Form.Item>
          <Form.Item>
            <Button type="primary" htmlType="submit" block loading={loading} className="login-btn">
              注 册
            </Button>
          </Form.Item>
        </Form>
      ),
    },
  ]

  return (
    <div className="login-page">
      <div className="login-brand">
        <div className="brand-content">
          <div className="brand-logo">
            <TeamOutlined />
          </div>
          <h1 className="brand-title">MySQL 运维平台</h1>
          <p className="brand-desc">企业级数据库全生命周期管理</p>
          <div className="brand-features">
            <div className="feature-item">
              <span className="feature-dot" />
              <span>主机管理 · 实例部署 · 版本升级</span>
            </div>
            <div className="feature-item">
              <span className="feature-dot" />
              <span>备份恢复 · 监控告警 · 数据迁移</span>
            </div>
            <div className="feature-item">
              <span className="feature-dot" />
              <span>参数模板 · 审批流程 · 审计日志</span>
            </div>
          </div>
          <div className="brand-footer">
            <span>Standalone 模式 · 任意账号即可登录</span>
          </div>
        </div>
      </div>
      <div className="login-form-wrap">
        <div className="login-card">
          <Tabs items={tabItems} centered size="large" />
        </div>
      </div>
    </div>
  )
}

export default Login
