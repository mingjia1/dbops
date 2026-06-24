import React, { useState, useEffect } from 'react'
import { Button, Card, Divider, Form, Input, InputNumber, message, Space, Switch, Tabs, Tag, Typography } from 'antd'
import { CloudOutlined, DatabaseOutlined, LockOutlined, SettingOutlined, ToolOutlined } from '@ant-design/icons'

const { Text } = Typography
const STORAGE_KEY = 'dbops_credential_password'

const SecuritySettings: React.FC = () => {
  const [enabled, setEnabled] = useState(false)
  const [form] = Form.useForm()

  useEffect(() => {
    const stored = localStorage.getItem(STORAGE_KEY)
    if (stored) {
      setEnabled(true)
      form.setFieldsValue({ password: stored, confirm_password: stored })
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
      <Card title={<Space><SettingOutlined /><span>系统设置</span></Space>}>
        <Tabs
          defaultActiveKey="security"
          items={[
            {
              key: 'security',
              label: <Space><LockOutlined />安全设置</Space>,
              children: (
                <Card type="inner" title="实例密码查看保护">
                  <div style={{ marginBottom: 16 }}>
                    <Space align="center">
                      <Switch
                        checked={enabled}
                        onChange={(checked) => {
                          if (checked) {
                            handleSave().catch(() => {
                              message.warning('请先填写并确认二级密码后再开启')
                              setEnabled(false)
                            })
                          } else {
                            handleDisable()
                          }
                        }}
                      />
                      <Text>{enabled ? '已开启' : '未开启'}</Text>
                      {enabled && <Tag color="success">保护中</Tag>}
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
                          {enabled ? '更新密码' : '保存密码'}
                        </Button>
                        {enabled && (
                          <Button danger onClick={handleDisable}>关闭保护</Button>
                        )}
                      </Space>
                    </Form.Item>
                  </Form>
                </Card>
              ),
            },
            {
              key: 'mysql-credential',
              label: <Space><DatabaseOutlined />MySQL 账号</Space>,
              children: <MySQLCredentialConfig />,
            },
            {
              key: 'password-policy',
              label: <Space><LockOutlined />密码策略</Space>,
              children: <PasswordPolicyConfig />,
            },
            {
              key: 'relay',
              label: <Space><CloudOutlined />中继服务器</Space>,
              children: <RelayServerConfig />,
            },
            {
              key: 'params',
              label: <Space><ToolOutlined />平台参数</Space>,
              children: <PlatformParams />,
            },
            {
              key: 'metrics',
              label: <Space><DatabaseOutlined />监控指标</Space>,
              children: <MetricsConfig />,
            },
          ]}
        />
      </Card>
    </div>
  )
}

// --- 中继服务器配置 ---
const RELAY_STORAGE_KEY = 'dbops_relay_server'

const RelayServerConfig: React.FC = () => {
  const [form] = Form.useForm()
  const [packages, setPackages] = useState<Array<{ name: string; size: string }>>([])
  const [loading, setLoading] = useState(false)

  useEffect(() => {
    const stored = localStorage.getItem(RELAY_STORAGE_KEY)
    if (stored) {
      try {
        const cfg = JSON.parse(stored)
        form.setFieldsValue(cfg)
      } catch { /* ignore */ }
    }
  }, [form])

  const handleSave = async () => {
    const values = await form.validateFields()
    localStorage.setItem(RELAY_STORAGE_KEY, JSON.stringify(values))
    message.success('中继服务器配置已保存')
  }

  const handleTest = async () => {
    const values = await form.validateFields()
    if (!values.relay_url) {
      message.warning('请先填写中继服务器地址')
      return
    }
    setLoading(true)
    try {
      const res = await fetch(values.relay_url, { method: 'HEAD', signal: AbortSignal.timeout(5000) })
      if (res.ok) {
        message.success('中继服务器连接正常')
      } else {
        message.warning(`中继服务器响应: HTTP ${res.status}`)
      }
    } catch {
      message.error('无法连接到中继服务器')
    } finally {
      setLoading(false)
    }
  }

  const handleScanPackages = async () => {
    const values = form.getFieldsValue()
    if (!values.relay_url) {
      message.warning('请先填写中继服务器地址')
      return
    }
    setLoading(true)
    try {
      const res = await fetch(values.relay_url)
      const text = await res.text()
      const items: Array<{ name: string; size: string }> = []
      const regex = /href="([^"]+\.(?:tar\.gz|tar\.xz|tgz))"[^>]*<\/a>\s+(\S+)/g
      let match
      while ((match = regex.exec(text)) !== null) {
        items.push({ name: match[1], size: match[2] || '-' })
      }
      if (items.length === 0) {
        // Try to find any file links
        const allLinks = text.match(/href="([^"]+)"/g) || []
        const fileLinks = allLinks.filter((l: string) => !l.includes('?') && !l.endsWith('/"'))
        fileLinks.forEach((l: string) => {
          const name = l.replace('href="', '').replace('"', '')
          if (name && !name.startsWith('..')) items.push({ name, size: '-' })
        })
      }
      setPackages(items)
      if (items.length === 0) {
        message.info('未在中继服务器上发现安装包')
      }
    } catch {
      message.error('扫描中继服务器失败')
    } finally {
      setLoading(false)
    }
  }

  return (
    <Card type="inner" title="中继服务器配置">
      <Text type="secondary" style={{ display: 'block', marginBottom: 16 }}>
        中继服务器用于分发 MySQL 安装包。集群部署时 agent 会从中继服务器下载安装包。
        地址格式示例：http://10.3.67.52:8888
      </Text>
      <Form form={form} layout="vertical" style={{ maxWidth: 500 }}>
        <Form.Item name="relay_url" label="中继服务器地址" rules={[{ required: true, message: '请输入中继服务器 URL' }]}>
          <Input placeholder="http://10.3.67.52:8888" />
        </Form.Item>
        <Form.Item>
          <Space>
            <Button type="primary" onClick={handleSave}>保存配置</Button>
            <Button onClick={handleTest} loading={loading}>测试连接</Button>
            <Button onClick={handleScanPackages} loading={loading}>扫描安装包</Button>
          </Space>
        </Form.Item>
      </Form>
      {packages.length > 0 && (
        <div style={{ marginTop: 16 }}>
          <strong>发现 {packages.length} 个安装包：</strong>
          <div style={{ marginTop: 8, maxHeight: 200, overflow: 'auto', background: '#f5f5f5', padding: 12, borderRadius: 4, fontFamily: 'monospace', fontSize: 12 }}>
            {packages.map((p, i) => (
              <div key={i}>{p.name} <span style={{ color: '#888' }}>{p.size}</span></div>
            ))}
          </div>
        </div>
      )}
    </Card>
  )
}

// --- 平台参数配置 ---
const PARAMS_STORAGE_KEY = 'dbops_platform_params'

const defaultParams = {
  agent_port: 9090,
  default_mysql_port: 3306,
  default_os_user: 'mysql',
  default_datadir_prefix: '/data/mysql',
  health_check_interval_sec: 30,
  deploy_timeout_min: 30,
  backup_retention_days: 7,
}

const PlatformParams: React.FC = () => {
  const [form] = Form.useForm()

  useEffect(() => {
    const stored = localStorage.getItem(PARAMS_STORAGE_KEY)
    if (stored) {
      try { form.setFieldsValue(JSON.parse(stored)) } catch { /* ignore */ }
    } else {
      form.setFieldsValue(defaultParams)
    }
  }, [form])

  const handleSave = () => {
    const values = form.getFieldsValue()
    localStorage.setItem(PARAMS_STORAGE_KEY, JSON.stringify(values))
    message.success('平台参数已保存')
  }

  return (
    <Card type="inner" title="平台默认参数">
      <Text type="secondary" style={{ display: 'block', marginBottom: 16 }}>
        这些参数在创建实例和集群部署时作为默认值使用。
      </Text>
      <Form form={form} layout="horizontal" labelCol={{ span: 8 }} wrapperCol={{ span: 16 }} style={{ maxWidth: 500 }}>
        <Form.Item name="agent_port" label="Agent 端口">
          <InputNumber min={1} max={65535} style={{ width: '100%' }} />
        </Form.Item>
        <Form.Item name="default_mysql_port" label="默认 MySQL 端口">
          <InputNumber min={1} max={65535} style={{ width: '100%' }} />
        </Form.Item>
        <Form.Item name="default_os_user" label="默认 OS 用户">
          <Input placeholder="mysql" />
        </Form.Item>
        <Form.Item name="default_datadir_prefix" label="数据目录前缀">
          <Input placeholder="/data/mysql" />
        </Form.Item>
        <Form.Item name="health_check_interval_sec" label="健康检查间隔(秒)">
          <InputNumber min={10} max={3600} style={{ width: '100%' }} />
        </Form.Item>
        <Form.Item name="deploy_timeout_min" label="部署超时(分钟)">
          <InputNumber min={5} max={120} style={{ width: '100%' }} />
        </Form.Item>
        <Form.Item name="backup_retention_days" label="备份保留天数">
          <InputNumber min={1} max={365} style={{ width: '100%' }} />
        </Form.Item>
        <Form.Item>
          <Button type="primary" onClick={handleSave}>保存参数</Button>
        </Form.Item>
      </Form>
    </Card>
  )
}

// --- 监控指标阈值配置 ---
const METRICS_STORAGE_KEY = 'dbops_metrics_thresholds'

const defaultMetrics = {
  replication_lag_warn_sec: 10,
  replication_lag_critical_sec: 60,
  connection_usage_warn_pct: 80,
  connection_usage_critical_pct: 95,
  disk_usage_warn_pct: 80,
  disk_usage_critical_pct: 90,
  memory_usage_warn_pct: 85,
  qps_threshold: 10000,
}

const MetricsConfig: React.FC = () => {
  const [form] = Form.useForm()

  useEffect(() => {
    const stored = localStorage.getItem(METRICS_STORAGE_KEY)
    if (stored) {
      try { form.setFieldsValue(JSON.parse(stored)) } catch { /* ignore */ }
    } else {
      form.setFieldsValue(defaultMetrics)
    }
  }, [form])

  const handleSave = () => {
    const values = form.getFieldsValue()
    localStorage.setItem(METRICS_STORAGE_KEY, JSON.stringify(values))
    message.success('监控阈值已保存')
  }

  return (
    <Card type="inner" title="监控指标阈值">
      <Text type="secondary" style={{ display: 'block', marginBottom: 16 }}>
        设置告警阈值，当指标超过阈值时在监控仪表盘中显示警告。
      </Text>
      <Form form={form} layout="horizontal" labelCol={{ span: 10 }} wrapperCol={{ span: 14 }} style={{ maxWidth: 520 }}>
        <Divider plain>复制延迟</Divider>
        <Form.Item name="replication_lag_warn_sec" label="警告阈值(秒)">
          <InputNumber min={1} style={{ width: '100%' }} />
        </Form.Item>
        <Form.Item name="replication_lag_critical_sec" label="严重阈值(秒)">
          <InputNumber min={1} style={{ width: '100%' }} />
        </Form.Item>
        <Divider plain>连接数</Divider>
        <Form.Item name="connection_usage_warn_pct" label="警告阈值(%)">
          <InputNumber min={1} max={100} style={{ width: '100%' }} />
        </Form.Item>
        <Form.Item name="connection_usage_critical_pct" label="严重阈值(%)">
          <InputNumber min={1} max={100} style={{ width: '100%' }} />
        </Form.Item>
        <Divider plain>资源使用</Divider>
        <Form.Item name="disk_usage_warn_pct" label="磁盘警告(%)">
          <InputNumber min={1} max={100} style={{ width: '100%' }} />
        </Form.Item>
        <Form.Item name="disk_usage_critical_pct" label="磁盘严重(%)">
          <InputNumber min={1} max={100} style={{ width: '100%' }} />
        </Form.Item>
        <Form.Item name="memory_usage_warn_pct" label="内存警告(%)">
          <InputNumber min={1} max={100} style={{ width: '100%' }} />
        </Form.Item>
        <Form.Item name="qps_threshold" label="QPS 阈值">
          <InputNumber min={100} style={{ width: '100%' }} />
        </Form.Item>
        <Form.Item>
          <Button type="primary" onClick={handleSave}>保存阈值</Button>
        </Form.Item>
      </Form>
    </Card>
  )
}

// --- 默认 MySQL 账号配置 ---
const CREDENTIAL_STORAGE_KEY = 'dbops_default_mysql_credential'

const MySQLCredentialConfig: React.FC = () => {
  const [form] = Form.useForm()

  useEffect(() => {
    const stored = localStorage.getItem(CREDENTIAL_STORAGE_KEY)
    if (stored) {
      try {
        const cred = JSON.parse(stored)
        form.setFieldsValue({ mysql_user: cred.username || 'root', mysql_password: cred.password || '' })
      } catch { /* ignore */ }
    }
  }, [form])

  const handleSave = async () => {
    const values = await form.validateFields()
    const cred = { username: values.mysql_user, password: values.mysql_password }
    localStorage.setItem(CREDENTIAL_STORAGE_KEY, JSON.stringify(cred))
    message.success('默认 MySQL 账号已保存')
  }

  return (
    <Card type="inner" title="集群部署默认 MySQL 账号">
      <Text type="secondary" style={{ display: 'block', marginBottom: 16 }}>
        集群部署时使用的默认 MySQL root 账号和密码。此设置会同步到集群部署页面。
      </Text>
      <Form form={form} layout="vertical" style={{ maxWidth: 400 }} initialValues={{ mysql_user: 'root' }}>
        <Form.Item name="mysql_user" label="用户名" rules={[{ required: true }]}>
          <Input placeholder="root" />
        </Form.Item>
        <Form.Item name="mysql_password" label="密码" rules={[{ required: true, min: 8, message: '密码至少 8 位' }]}>
          <Input.Password placeholder="MySQL root 密码" autoComplete="new-password" />
        </Form.Item>
        <Form.Item>
          <Button type="primary" onClick={handleSave}>保存账号</Button>
        </Form.Item>
      </Form>
    </Card>
  )
}

// --- 密码策略配置 ---
const POLICY_STORAGE_KEY = 'dbops_password_policy'

const defaultPolicy = {
  min_length: 8,
  require_uppercase: true,
  require_lowercase: true,
  require_digit: true,
  require_special: true,
}

const PasswordPolicyConfig: React.FC = () => {
  const [form] = Form.useForm()

  useEffect(() => {
    const stored = localStorage.getItem(POLICY_STORAGE_KEY)
    if (stored) {
      try { form.setFieldsValue(JSON.parse(stored)) } catch { form.setFieldsValue(defaultPolicy) }
    } else {
      form.setFieldsValue(defaultPolicy)
    }
  }, [form])

  const handleSave = () => {
    const values = form.getFieldsValue()
    localStorage.setItem(POLICY_STORAGE_KEY, JSON.stringify(values))
    message.success('密码策略已保存')
  }

  return (
    <Card type="inner" title="全平台密码复杂度要求">
      <Text type="secondary" style={{ display: 'block', marginBottom: 16 }}>
        设置平台上所有密码（实例密码、复制密码、用户密码）的最低要求。
      </Text>
      <Form form={form} layout="horizontal" labelCol={{ span: 8 }} wrapperCol={{ span: 16 }} style={{ maxWidth: 450 }}>
        <Form.Item name="min_length" label="最小长度">
          <InputNumber min={4} max={32} style={{ width: '100%' }} />
        </Form.Item>
        <Form.Item name="require_uppercase" label="需要大写字母" valuePropName="checked">
          <Switch />
        </Form.Item>
        <Form.Item name="require_lowercase" label="需要小写字母" valuePropName="checked">
          <Switch />
        </Form.Item>
        <Form.Item name="require_digit" label="需要数字" valuePropName="checked">
          <Switch />
        </Form.Item>
        <Form.Item name="require_special" label="需要特殊字符" valuePropName="checked">
          <Switch />
        </Form.Item>
        <Form.Item>
          <Button type="primary" onClick={handleSave}>保存策略</Button>
        </Form.Item>
      </Form>
    </Card>
  )
}

export default SecuritySettings
