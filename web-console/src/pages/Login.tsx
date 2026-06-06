import React, { useState } from 'react'
import { Form, Input, Button, Tabs, App as AntApp } from 'antd'
import { UserOutlined, LockOutlined, MailOutlined, AppleOutlined, CloudOutlined, SafetyCertificateOutlined, ThunderboltOutlined, ClusterOutlined } from '@ant-design/icons'
import { useNavigate } from 'react-router-dom'
import api from '../services/api'
import './Login.css'

const Login: React.FC = () => {
  const [loading, setLoading] = useState(false)
  const navigate = useNavigate()
  const { message } = AntApp.useApp()

  // P1: 之前 Login.tsx 用裸 axios.post 绕开了 services/api.ts 的统一拦截器
  // (401 触发 triggerLogout / 错误 message 统一格式化). 后端 5xx 字段若不是
  // {message: ...} 就会显示 "undefined". 改用共享 api 实例, 同时 401 路径
  // 自动 triggerLogout, 与其他页面一致.
  const onLogin = async (values: any) => {
    setLoading(true)
    try {
      // api.interceptors.response 已经 return response.data, 这里 res 就是后端 body
      const res: any = await api.post('/auth/login', {
        username: values.username,
        password: values.password,
      })
      if (res && res.code === 200 && res.data) {
        const { token, user } = res.data
        if (token) localStorage.setItem('token', token)
        if (user) localStorage.setItem('user', JSON.stringify(user))
        message.success('欢迎回来，' + (user?.username || ''))
        setTimeout(() => navigate('/dashboard', { replace: true }), 220)
      } else {
        message.error(res?.message || '登录失败')
      }
    } catch (error: any) {
      const status = error.response?.status
      if (status === 401) {
        message.error('用户名或密码错误')
      } else if (status === 429) {
        message.warning('登录尝试过于频繁，请稍后再试')
      } else {
        message.error(error.response?.data?.message || error.message || '登录请求失败')
      }
    } finally {
      setLoading(false)
    }
  }

  const onRegister = async (values: any) => {
    setLoading(true)
    try {
      const res: any = await api.post('/auth/register', {
        username: values.username,
        password: values.password,
        email: values.email,
        role: 'operator',
      })
      if (res && res.code === 200) {
        message.success('注册成功，请登录')
      } else {
        message.error(res?.message || '注册失败')
      }
    } catch (error: any) {
      message.error(error.response?.data?.message || error.message || '注册请求失败')
    } finally {
      setLoading(false)
    }
  }

  const tabItems = [
    {
      key: 'login',
      label: '登录',
      children: (
        <Form name="login" onFinish={onLogin} layout="vertical" size="large" autoComplete="off" requiredMark={false}>
          <Form.Item name="username" rules={[{ required: true, message: '请输入用户名' }]}>
            <Input prefix={<UserOutlined style={{ color: 'rgba(60,60,67,0.4)' }} />} placeholder="用户名" autoFocus />
          </Form.Item>
          <Form.Item name="password" rules={[{ required: true, message: '请输入密码' }]}>
            <Input.Password prefix={<LockOutlined style={{ color: 'rgba(60,60,67,0.4)' }} />} placeholder="密码" />
          </Form.Item>
          <Form.Item style={{ marginBottom: 0 }}>
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
        <Form name="register" onFinish={onRegister} layout="vertical" size="large" autoComplete="off" requiredMark={false}>
          <Form.Item name="username" rules={[{ required: true, message: '请输入用户名' }, { min: 3, message: '至少 3 个字符' }]}>
            <Input prefix={<UserOutlined style={{ color: 'rgba(60,60,67,0.4)' }} />} placeholder="用户名" />
          </Form.Item>
          <Form.Item name="email" rules={[{ required: true, type: 'email', message: '请输入有效邮箱' }]}>
            <Input prefix={<MailOutlined style={{ color: 'rgba(60,60,67,0.4)' }} />} placeholder="邮箱" />
          </Form.Item>
          <Form.Item name="password" rules={[{ required: true, min: 6, message: '密码至少 6 位' }]}>
            <Input.Password prefix={<LockOutlined style={{ color: 'rgba(60,60,67,0.4)' }} />} placeholder="密码" />
          </Form.Item>
          <Form.Item name="confirm" dependencies={['password']} rules={[{ required: true, message: '请确认密码' }, ({ getFieldValue }) => ({ validator(_, value) { if (!value || getFieldValue('password') === value) return Promise.resolve(); return Promise.reject(new Error('两次密码不一致')); } })]}>
            <Input.Password prefix={<LockOutlined style={{ color: 'rgba(60,60,67,0.4)' }} />} placeholder="确认密码" />
          </Form.Item>
          <Form.Item style={{ marginBottom: 0 }}>
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
        <div className="brand-orb brand-orb-1" />
        <div className="brand-orb brand-orb-2" />
        <div className="brand-orb brand-orb-3" />
        <div className="brand-content apple-fade-in">
          <div className="brand-logo">
            <AppleOutlined />
          </div>
          <h1 className="brand-title">MySQL 运维平台</h1>
          <p className="brand-desc">企业级数据库全生命周期管理</p>

          <div className="brand-feature-grid">
            <div className="brand-feature">
              <div className="brand-feature-icon" style={{ background: 'linear-gradient(135deg,#0071E3,#5AC8FA)' }}><CloudOutlined /></div>
              <div className="brand-feature-text">
                <div className="brand-feature-title">主机与实例</div>
                <div className="brand-feature-desc">资产盘点 · 自动发现</div>
              </div>
            </div>
            <div className="brand-feature">
              <div className="brand-feature-icon" style={{ background: 'linear-gradient(135deg,#34C759,#30D158)' }}><SafetyCertificateOutlined /></div>
              <div className="brand-feature-text">
                <div className="brand-feature-title">备份与恢复</div>
                <div className="brand-feature-desc">策略化 · 一键回滚</div>
              </div>
            </div>
            <div className="brand-feature">
              <div className="brand-feature-icon" style={{ background: 'linear-gradient(135deg,#FF9500,#FFCC00)' }}><ThunderboltOutlined /></div>
              <div className="brand-feature-text">
                <div className="brand-feature-title">监控告警</div>
                <div className="brand-feature-desc">指标聚合 · 阈值告警</div>
              </div>
            </div>
            <div className="brand-feature">
              <div className="brand-feature-icon" style={{ background: 'linear-gradient(135deg,#AF52DE,#FF2D55)' }}><ClusterOutlined /></div>
              <div className="brand-feature-text">
                <div className="brand-feature-title">高可用集群</div>
                <div className="brand-feature-desc">MHA · MGR · PXC</div>
              </div>
            </div>
          </div>

          <div className="brand-footer">
            <span>v1.0 · 统一管理 · 可观测 · 可控制</span>
          </div>
        </div>
      </div>
      <div className="login-form-wrap">
        <div className="login-card apple-fade-in">
          <div className="login-card-header">
            <div className="login-card-title">欢迎回来</div>
            <div className="login-card-sub">登录以继续管理工作台</div>
          </div>
          <Tabs items={tabItems} centered size="large" />
          <div className="login-card-foot">
            首次使用?请查看后端启动日志中的默认 admin 密码
          </div>
        </div>
      </div>
    </div>
  )
}

export default Login

