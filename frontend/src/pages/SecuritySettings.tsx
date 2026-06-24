import React, { useState, useEffect } from 'react'
import { Alert, Button, Card, Col, Divider, Form, Input, InputNumber, message, Row, Select, Space, Switch, Table, Tabs, Tag, Typography } from 'antd'
import { CloudOutlined, DatabaseOutlined, LockOutlined, SettingOutlined, ToolOutlined } from '@ant-design/icons'
import { usePlatformSettings } from '../services/useSettings'

const { Text } = Typography
const STORAGE_KEY = 'dbops_credential_password'

const SecuritySettings: React.FC = () => {
  const [enabled, setEnabled] = useState(false)
  const [form] = Form.useForm()
  const { settings, save } = usePlatformSettings()

  useEffect(() => {
    const stored = settings.credential_password || localStorage.getItem(STORAGE_KEY)
    if (stored) {
      setEnabled(true)
      form.setFieldsValue({ password: stored, confirm_password: stored })
    }
  }, [settings])

  const handleSave = async () => {
    const values = await form.validateFields()
    if (values.password !== values.confirm_password) {
      message.error('两次输入的密码不一致')
      return
    }
    localStorage.setItem(STORAGE_KEY, values.password)
    await save('credential_password', values.password)
    setEnabled(true)
    message.success('二级密码已设置')
  }

  const handleDisable = () => {
    localStorage.removeItem(STORAGE_KEY)
    sessionStorage.removeItem('dbops_credential_verified')
    save('credential_password', '')
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
  isTemplate?: boolean // URL contains ${version} variables
}

const PRESET_SOURCES: RelaySource[] = [
  { url: 'http://${platform_ip}:8888', label: '本平台中继', enabled: true },
  { url: 'https://mirrors.tuna.tsinghua.edu.cn/mysql', label: '清华镜像', enabled: false },
  { url: 'https://mirrors.aliyun.com/mysql', label: '阿里云镜像', enabled: false },
  { url: 'https://mirrors.huaweicloud.com/mysql', label: '华为云镜像', enabled: false },
  { url: 'https://mirrors.ustc.edu.cn/mysql-ftp', label: '中科大镜像', enabled: false },
  { url: 'https://dev.mysql.com/get/Downloads', label: 'MySQL 官方', enabled: false },
  { url: 'https://archive.mariadb.org', label: 'MariaDB 官方', enabled: false },
]

const resolveSourceUrl = (src: RelaySource, version: string, platformIp: string) => {
  let url = src.url
  // Replace platform IP placeholder
  url = url.replace(/\$\{platform_ip\}/g, platformIp)
  // Replace version variables: 8.0.36 -> major_minor=8.0, version=8.0.36
  const parts = version.split('.')
  const majorMinor = parts.length >= 2 ? `${parts[0]}.${parts[1]}` : version
  url = url.replace(/\$\{version\}/g, version)
  url = url.replace(/\$\{major_minor\}/g, majorMinor)
  url = url.replace(/\$\{major\}/g, parts[0] || version)
  url = url.replace(/\$\{minor\}/g, parts[1] || '')
  return url
}

const RelayServerConfig: React.FC = () => {
  const [form] = Form.useForm()
  const [sources, setSources] = useState<RelaySource[]>([])
  const [packages, setPackages] = useState<Array<{ name: string; version: string; arch: string; flavor: string; path: string; source: string }>>([])
  const [loading, setLoading] = useState(false)
  const platformIp = window.location.hostname || '10.3.67.52'
  const { settings, save: saveSetting } = usePlatformSettings()

  useEffect(() => {
    const raw = settings.relay_config || localStorage.getItem(RELAY_STORAGE_KEY)
    if (raw) {
      try {
        const cfg = typeof raw === 'string' ? JSON.parse(raw) : raw
        form.setFieldsValue({ relay_path: cfg.relay_path || '', download_priority: cfg.download_priority || 'relay_first', default_version: cfg.default_version || '8.0.36' })
        if (cfg.sources && cfg.sources.length > 0) {
          setSources(cfg.sources)
          return
        }
      } catch { /* ignore */ }
    }
    // Default: auto-detect platform relay + show presets
    setSources(PRESET_SOURCES.map(s => ({
      ...s,
      url: s.url.replace(/\$\{platform_ip\}/g, platformIp),
    })))
    form.setFieldsValue({ default_version: '8.0.36' })
  }, [settings.relay_config])

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
      default_version: values.default_version || '8.0.36',
      sources,
    }
    const json = JSON.stringify(cfg)
    localStorage.setItem(RELAY_STORAGE_KEY, json)
    saveSetting('relay_config', json)
    message.success('中继服务器配置已保存')
  }

  const buildBaseUrl = (source: RelaySource, versionOverride?: string) => {
    const version = versionOverride || form.getFieldValue('default_version') || '8.0.36'
    let url = resolveSourceUrl(source, version, platformIp).replace(/\/+$/, '')
    const path = form.getFieldValue('relay_path') || ''
    if (path) {
      url += '/' + path.replace(/^\/+/, '').replace(/\/+$/, '')
    }
    return url
  }

  const handleTest = async (source: RelaySource) => {
    if (!source.url) { message.warning('请先填写地址'); return }
    const resolvedUrl = buildBaseUrl(source)
    setLoading(true)
    try {
      const res = await fetch(resolvedUrl, { method: 'HEAD', signal: AbortSignal.timeout(5000) })
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
    <div>
      {/* 顶部说明 */}
      <Alert
        type="info"
        showIcon
        message="安装包下载源配置"
        description="部署时按优先级依次尝试下载：主中继服务器 → 备用源 → 官方源 (dev.mysql.com)。安装成功后自动上传一份到主中继服务器。"
        style={{ marginBottom: 16 }}
      />

      <Row gutter={16}>
        {/* 左列：下载源列表 */}
        <Col span={16}>
          <Card type="inner" title="下载源列表" size="small"
            extra={
              <Space>
                <Button size="small" onClick={addSource}>+ 添加源</Button>
                <Button size="small" type="dashed" onClick={() => {
                  setSources(PRESET_SOURCES.map(s => ({ ...s, url: s.url.replace(/\$\{platform_ip\}/g, platformIp) })))
                  message.success('已重置为推荐配置')
                }}>重置推荐</Button>
                <Button size="small" onClick={handleTestAll} loading={loading} disabled={enabledSources.length === 0}>测试全部</Button>
              </Space>
            }
          >
            {sources.map((src, i) => (
              <div key={i} style={{
                display: 'grid',
                gridTemplateColumns: '28px 40px 1fr 130px 32px 32px 50px 32px',
                alignItems: 'center',
                gap: 6,
                marginBottom: 6,
                padding: '6px 10px',
                background: i === 0 ? '#f0f7ff' : (src.enabled ? '#fafafa' : '#f5f5f5'),
                borderRadius: 6,
                border: i === 0 ? '1px solid #91caff' : '1px solid #f0f0f0',
                opacity: src.enabled ? 1 : 0.6,
              }}>
                <span style={{ fontSize: 11, color: '#999', textAlign: 'center' }}>{i + 1}</span>
                <Switch size="small" checked={src.enabled} onChange={(v) => updateSource(i, 'enabled', v)} />
                <div style={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
                  <Input size="small" value={src.url} onChange={(e) => updateSource(i, 'url', e.target.value)}
                    placeholder="http://ip:port 或 https://mirror/.../${version}" style={{ width: '100%' }} />
                  {src.url.includes('${') && src.enabled && (
                    <span style={{ fontSize: 10, color: '#1677ff' }}>
                      → {buildBaseUrl(src)}
                    </span>
                  )}
                </div>
                <Input size="small" value={src.label} onChange={(e) => updateSource(i, 'label', e.target.value)}
                  placeholder="标签名" style={{ width: '100%' }} />
                {i === 0
                  ? <Tag color="blue" style={{ fontSize: 10 }}>主</Tag>
                  : <Button size="small" type="text" onClick={() => moveSource(i, 'up')} disabled={i === 0} style={{ padding: '0 4px' }}>↑</Button>
                }
                <Button size="small" type="text" onClick={() => moveSource(i, 'down')} disabled={i === sources.length - 1} style={{ padding: '0 4px' }}>↓</Button>
                <Button size="small" onClick={() => handleTest(src)} loading={loading}>测试</Button>
                <Button size="small" type="text" danger onClick={() => removeSource(i)} disabled={sources.length <= 1} style={{ padding: 0 }}>✕</Button>
              </div>
            ))}
          </Card>
        </Col>

        {/* 右列：全局配置 */}
        <Col span={8}>
          <Card type="inner" title="全局配置" size="small">
            <Form form={form} layout="vertical" size="small">
              <Form.Item name="default_version" label="默认版本" extra="用于解析 ${version} 变量">
                <Input placeholder="8.0.36" />
              </Form.Item>
              <Form.Item name="relay_path" label="包子路径" extra="追加到每个源 URL 末尾">
                <Input placeholder="/" />
              </Form.Item>
              <Form.Item name="download_priority" label="下载策略">
                <Select
                  size="small"
                  defaultValue="relay_first"
                  options={[
                    { value: 'relay_first', label: '中继优先' },
                    { value: 'relay_only', label: '仅中继' },
                    { value: 'official_only', label: '仅官方源' },
                  ]}
                />
              </Form.Item>
              <Space>
                <Button type="primary" size="small" onClick={handleSave}>保存</Button>
                <Button size="small" onClick={handleScanAll} loading={loading} disabled={enabledSources.length === 0}>扫描</Button>
              </Space>
            </Form>
          </Card>

          {/* 变量说明 */}
          <Card type="inner" title="变量说明" size="small" style={{ marginTop: 12 }}>
            <div style={{ fontSize: 11, lineHeight: 1.8 }}>
              <div><code>{'${version}'}</code> → <code>8.0.36</code> 完整版本</div>
              <div><code>{'${major_minor}'}</code> → <code>8.0</code></div>
              <div><code>{'${major}'}</code> → <code>8</code></div>
              <div><code>{'${minor}'}</code> → <code>0</code></div>
              <div><code>{'${platform_ip}'}</code> → <code>{platformIp}</code></div>
            </div>
          </Card>

          {/* 活跃源摘要 */}
          {enabledSources.length > 0 && (
            <Card type="inner" title="活跃源" size="small" style={{ marginTop: 12 }}>
              {enabledSources.map((src, i) => (
                <div key={i} style={{ display: 'flex', alignItems: 'center', gap: 6, marginBottom: 4, fontSize: 12 }}>
                  <Tag color={i === 0 ? 'blue' : 'default'} style={{ fontSize: 10, margin: 0 }}>{i === 0 ? 'P1' : `P${i + 1}`}</Tag>
                  <code style={{ fontSize: 11, wordBreak: 'break-all' }}>{buildBaseUrl(src)}</code>
                </div>
              ))}
            </Card>
          )}
        </Col>
      </Row>

      {/* 扫描结果 */}
      {packages.length > 0 && (
        <Card type="inner" title={`扫描结果 (${packages.length} 个安装包)`} size="small" style={{ marginTop: 16 }}>
          <Table
            size="small"
            pagination={packages.length > 10 ? { pageSize: 10, size: 'small' } : false}
            dataSource={packages.map((p, i) => ({ ...p, key: i }))}
            columns={[
              { title: '来源', dataIndex: 'source', key: 'source', width: 110, render: (v: string) => <Tag style={{ fontSize: 10 }}>{v}</Tag> },
              { title: '产品', dataIndex: 'flavor', key: 'flavor', width: 80, render: (v: string) => <Tag color={v === 'MySQL' ? 'blue' : v === 'Percona' ? 'purple' : v === 'MariaDB' ? 'orange' : 'default'}>{v}</Tag> },
              { title: '版本', dataIndex: 'version', key: 'version', width: 100 },
              { title: '架构', dataIndex: 'arch', key: 'arch', width: 70, render: (v: string) => <Tag color={v === '通用' ? 'default' : 'blue'}>{v}</Tag> },
              { title: '文件名', dataIndex: 'name', key: 'name', ellipsis: true },
            ]}
          />
        </Card>
      )}
    </div>
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
  const { settings, save: saveSetting } = usePlatformSettings()

  useEffect(() => {
    const raw = settings.platform_params || localStorage.getItem(PARAMS_STORAGE_KEY)
    if (raw) {
      try { form.setFieldsValue(typeof raw === 'string' ? JSON.parse(raw) : raw) } catch { form.setFieldsValue(defaultParams) }
    } else {
      form.setFieldsValue(defaultParams)
    }
  }, [settings.platform_params])

  const handleSave = () => {
    const values = form.getFieldsValue()
    const json = JSON.stringify(values)
    localStorage.setItem(PARAMS_STORAGE_KEY, json)
    saveSetting('platform_params', json)
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
  const { settings, save: saveSetting } = usePlatformSettings()

  useEffect(() => {
    const raw = settings.metrics_thresholds || localStorage.getItem(METRICS_STORAGE_KEY)
    if (raw) {
      try { form.setFieldsValue(typeof raw === 'string' ? JSON.parse(raw) : raw) } catch { form.setFieldsValue(defaultMetrics) }
    } else {
      form.setFieldsValue(defaultMetrics)
    }
  }, [settings.metrics_thresholds])

  const handleSave = () => {
    const values = form.getFieldsValue()
    const json = JSON.stringify(values)
    localStorage.setItem(METRICS_STORAGE_KEY, json)
    saveSetting('metrics_thresholds', json)
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
  const { settings, save: saveSetting } = usePlatformSettings()

  useEffect(() => {
    const raw = settings.mysql_credential || localStorage.getItem(CREDENTIAL_STORAGE_KEY)
    if (raw) {
      try {
        const cred = typeof raw === 'string' ? JSON.parse(raw) : raw
        form.setFieldsValue({ mysql_user: cred.username || 'root', mysql_password: cred.password || '' })
      } catch { /* ignore */ }
    }
  }, [settings.mysql_credential])

  const handleSave = async () => {
    const values = await form.validateFields()
    const cred = { username: values.mysql_user, password: values.mysql_password }
    const json = JSON.stringify(cred)
    localStorage.setItem(CREDENTIAL_STORAGE_KEY, json)
    saveSetting('mysql_credential', json)
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
  const { settings, save: saveSetting } = usePlatformSettings()

  useEffect(() => {
    const raw = settings.password_policy || localStorage.getItem(POLICY_STORAGE_KEY)
    if (raw) {
      try { form.setFieldsValue(typeof raw === 'string' ? JSON.parse(raw) : raw) } catch { form.setFieldsValue(defaultPolicy) }
    } else {
      form.setFieldsValue(defaultPolicy)
    }
  }, [settings.password_policy])

  const handleSave = () => {
    const values = form.getFieldsValue()
    const json = JSON.stringify(values)
    localStorage.setItem(POLICY_STORAGE_KEY, json)
    saveSetting('password_policy', json)
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
