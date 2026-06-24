import React, { useState, useEffect } from 'react'
import { Button, Card, Divider, Form, Input, InputNumber, message, Select, Space, Switch, Table, Tabs, Tag, Typography } from 'antd'
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

interface RelaySource {
  url: string
  label: string
  enabled: boolean
}

const RelayServerConfig: React.FC = () => {
  const [form] = Form.useForm()
  const [sources, setSources] = useState<RelaySource[]>([])
  const [packages, setPackages] = useState<Array<{ name: string; version: string; arch: string; flavor: string; path: string; source: string }>>([])
  const [loading, setLoading] = useState(false)

  useEffect(() => {
    const stored = localStorage.getItem(RELAY_STORAGE_KEY)
    if (stored) {
      try {
        const cfg = JSON.parse(stored)
        form.setFieldsValue({ relay_path: cfg.relay_path || '', download_priority: cfg.download_priority || 'relay_first' })
        if (cfg.sources && cfg.sources.length > 0) {
          setSources(cfg.sources)
        } else if (cfg.relay_url) {
          setSources([{ url: cfg.relay_url, label: '主中继服务器', enabled: true }])
        }
      } catch { /* ignore */ }
    }
    if (sources.length === 0) {
      setSources([{ url: '', label: '主中继服务器', enabled: true }])
    }
  }, [])

  const addSource = () => {
    setSources([...sources, { url: '', label: `备用源 ${sources.length}`, enabled: true }])
  }

  const removeSource = (index: number) => {
    setSources(sources.filter((_, i) => i !== index))
  }

  const updateSource = (index: number, field: keyof RelaySource, value: any) => {
    const next = [...sources]
    next[index] = { ...next[index], [field]: value }
    setSources(next)
  }

  const moveSource = (index: number, direction: 'up' | 'down') => {
    const next = [...sources]
    const target = direction === 'up' ? index - 1 : index + 1
    if (target < 0 || target >= next.length) return
    ;[next[index], next[target]] = [next[target], next[index]]
    setSources(next)
  }

  const handleSave = () => {
    const values = form.getFieldsValue()
    const cfg = {
      relay_url: sources.find(s => s.enabled && s.url)?.url || '',
      relay_path: values.relay_path || '',
      download_priority: values.download_priority || 'relay_first',
      sources,
    }
    localStorage.setItem(RELAY_STORAGE_KEY, JSON.stringify(cfg))
    message.success('中继服务器配置已保存')
  }

  const buildBaseUrl = (source: RelaySource) => {
    let url = (source.url || '').replace(/\/+$/, '')
    const path = form.getFieldValue('relay_path') || ''
    if (path) {
      url += '/' + path.replace(/^\/+/, '').replace(/\/+$/, '')
    }
    return url
  }

  const handleTest = async (source: RelaySource) => {
    if (!source.url) { message.warning('请先填写地址'); return }
    setLoading(true)
    try {
      const res = await fetch(buildBaseUrl(source), { method: 'HEAD', signal: AbortSignal.timeout(5000) })
      message[res.ok ? 'success' : 'warning'](`${source.label}: ${res.ok ? '连接正常' : 'HTTP ' + res.status}`)
    } catch {
      message.error(`${source.label}: 无法连接`)
    } finally {
      setLoading(false)
    }
  }

  const handleTestAll = async () => {
    setLoading(true)
    for (const src of sources.filter(s => s.enabled && s.url)) {
      try {
        const res = await fetch(buildBaseUrl(src), { method: 'HEAD', signal: AbortSignal.timeout(5000) })
        message[res.ok ? 'success' : 'warning'](`${src.label}: ${res.ok ? '✓' : 'HTTP ' + res.status}`)
      } catch {
        message.error(`${src.label}: 无法连接`)
      }
    }
    setLoading(false)
  }

  const handleScanAll = async () => {
    setLoading(true)
    const allPkgs: typeof packages = []
    for (const src of sources.filter(s => s.enabled && s.url)) {
      const url = buildBaseUrl(src)
      try {
        const res = await fetch(url)
        const text = await res.text()
        await scanHtml(text, url, src.label, allPkgs)
      } catch { /* skip */ }
    }
    setPackages(allPkgs)
    message[allPkgs.length > 0 ? 'success' : 'info'](allPkgs.length > 0 ? `发现 ${allPkgs.length} 个安装包` : '未发现安装包')
    setLoading(false)
  }

  const scanHtml = async (html: string, baseUrl: string, sourceLabel: string, result: typeof packages) => {
    const dirs: string[] = []
    const regex = /href="([^"]+)"/g
    let m
    while ((m = regex.exec(html)) !== null) {
      const href = m[1]
      if (href === '../' || href.startsWith('?')) continue
      if (href.endsWith('/')) dirs.push(href.replace(/\/$/, ''))
      else if (/\.(tar\.gz|tar\.xz|tgz|tar\.bz2)$/i.test(href)) {
        const parsed = parsePackageName(href)
        result.push({ ...parsed, path: href, source: sourceLabel })
      }
    }
    for (const dir of dirs) {
      try {
        const dirRes = await fetch(`${baseUrl}/${dir}`)
        const dirText = await dirRes.text()
        await scanHtml(dirText, `${baseUrl}/${dir}`, sourceLabel, result)
      } catch { /* skip */ }
    }
  }

  const parsePackageName = (fileName: string) => {
    const lower = fileName.toLowerCase()
    const mysqlMatch = lower.match(/^mysql-([\d.]+)-linux/)
    if (mysqlMatch) {
      return { name: fileName, version: mysqlMatch[1], arch: lower.includes('x86_64') ? 'x86_64' : lower.includes('aarch64') ? 'aarch64' : '通用', flavor: 'MySQL' }
    }
    const perconaMatch = lower.match(/^percona-server-([\d.]+-\d+)-linux/)
    if (perconaMatch) {
      return { name: fileName, version: perconaMatch[1], arch: lower.includes('x86_64') ? 'x86_64' : lower.includes('aarch64') ? 'aarch64' : '通用', flavor: 'Percona' }
    }
    const mariadbMatch = lower.match(/^mariadb-([\d.]+)-linux/)
    if (mariadbMatch) {
      return { name: fileName, version: mariadbMatch[1], arch: lower.includes('x86_64') ? 'x86_64' : lower.includes('aarch64') ? 'aarch64' : '通用', flavor: 'MariaDB' }
    }
    return { name: fileName, version: '-', arch: '通用', flavor: '未知' }
  }

  const enabledSources = sources.filter(s => s.enabled && s.url)

  return (
    <Card type="inner" title="中继服务器与下载源">
      <Text type="secondary" style={{ display: 'block', marginBottom: 16 }}>
        配置安装包下载源。Agent 部署时按优先级依次尝试：中继服务器 → 其他备用源 → 官方源。
        安装成功后自动上传一份到主中继服务器供后续使用。
      </Text>

      {/* 下载源列表 */}
      <div style={{ marginBottom: 16 }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 8 }}>
          <strong>下载源列表（按优先级排序）</strong>
          <Space>
            <Button size="small" onClick={addSource}>添加源</Button>
            <Button size="small" onClick={handleTestAll} loading={loading} disabled={enabledSources.length === 0}>测试全部</Button>
          </Space>
        </div>
        {sources.map((src, i) => (
          <div key={i} style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 8, padding: '8px 12px', background: src.enabled ? '#f9f9f9' : '#f0f0f0', borderRadius: 4, border: i === 0 ? '1px solid #1677ff' : '1px solid #e8e8e8' }}>
            <span style={{ fontSize: 12, color: '#888', minWidth: 20 }}>{i + 1}</span>
            <Switch size="small" checked={src.enabled} onChange={(v) => updateSource(i, 'enabled', v)} />
            <Input size="small" value={src.url} onChange={(e) => updateSource(i, 'url', e.target.value)} placeholder="http://10.3.67.52:8888" style={{ flex: 1 }} />
            <Input size="small" value={src.label} onChange={(e) => updateSource(i, 'label', e.target.value)} placeholder="标签" style={{ width: 120 }} />
            {i === 0 && <Tag color="blue">主</Tag>}
            <Button size="small" disabled={i === 0} onClick={() => moveSource(i, 'up')}>↑</Button>
            <Button size="small" disabled={i === sources.length - 1} onClick={() => moveSource(i, 'down')}>↓</Button>
            <Button size="small" onClick={() => handleTest(src)} loading={loading}>测试</Button>
            <Button size="small" danger onClick={() => removeSource(i)} disabled={sources.length <= 1}>×</Button>
          </div>
        ))}
      </div>

      <Form form={form} layout="horizontal" labelCol={{ span: 6 }} wrapperCol={{ span: 18 }} style={{ maxWidth: 550 }}>
        <Form.Item name="relay_path" label="包子路径" extra="相对于源地址的子路径，如 'packages'">
          <Input placeholder="留空表示根目录" />
        </Form.Item>
        <Form.Item name="download_priority" label="下载策略">
          <Select
            defaultValue="relay_first"
            options={[
              { value: 'relay_first', label: '中继优先 → 备用源 → 官方源' },
              { value: 'relay_only', label: '仅中继服务器' },
              { value: 'official_only', label: '仅官方源（dev.mysql.com）' },
            ]}
          />
        </Form.Item>
        <Form.Item>
          <Space>
            <Button type="primary" onClick={handleSave}>保存配置</Button>
            <Button onClick={handleScanAll} loading={loading} disabled={enabledSources.length === 0}>扫描全部源</Button>
          </Space>
        </Form.Item>
      </Form>

      {/* 当前活跃源摘要 */}
      {enabledSources.length > 0 && (
        <div style={{ marginTop: 16, padding: 12, background: '#f0f7ff', borderRadius: 4, border: '1px solid #d0e3ff' }}>
          <strong style={{ fontSize: 13 }}>当前活跃下载源：</strong>
          <div style={{ marginTop: 8 }}>
            {enabledSources.map((src, i) => (
              <div key={i} style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 4 }}>
                <Tag color={i === 0 ? 'blue' : 'default'}>{i === 0 ? '优先' : `备用 ${i}`}</Tag>
                <code style={{ fontSize: 12 }}>{buildBaseUrl(src)}</code>
                <span style={{ color: '#888', fontSize: 12 }}>({src.label})</span>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* 扫描结果 */}
      {packages.length > 0 && (
        <div style={{ marginTop: 16 }}>
          <strong>发现 {packages.length} 个安装包</strong>
          <Table
            size="small"
            style={{ marginTop: 8 }}
            pagination={packages.length > 10 ? { pageSize: 10 } : false}
            dataSource={packages.map((p, i) => ({ ...p, key: i }))}
            columns={[
              { title: '来源', dataIndex: 'source', key: 'source', width: 120, render: (v: string) => <Tag>{v}</Tag> },
              { title: '产品', dataIndex: 'flavor', key: 'flavor', width: 90, render: (v: string) => <Tag color={v === 'MySQL' ? 'blue' : v === 'Percona' ? 'purple' : v === 'MariaDB' ? 'orange' : 'default'}>{v}</Tag> },
              { title: '版本', dataIndex: 'version', key: 'version', width: 100 },
              { title: '架构', dataIndex: 'arch', key: 'arch', width: 80, render: (v: string) => <Tag color={v === '通用' ? 'default' : 'blue'}>{v}</Tag> },
              { title: '文件名', dataIndex: 'name', key: 'name', ellipsis: true },
            ]}
          />
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
