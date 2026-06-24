import React, { useState, useEffect } from 'react'
import { Button, Card, Col, Divider, Form, Input, InputNumber, message, Row, Select, Space, Switch, Table, Tabs, Tag, Typography } from 'antd'
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
    if (stored) { setEnabled(true); form.setFieldsValue({ password: stored, confirm_password: stored }) }
  }, [settings])

  const handleSave = async () => {
    const values = await form.validateFields()
    if (values.password !== values.confirm_password) { message.error('两次输入的密码不一致'); return }
    localStorage.setItem(STORAGE_KEY, values.password)
    await save('credential_password', values.password)
    setEnabled(true); message.success('二级密码已设置')
  }
  const handleDisable = () => {
    localStorage.removeItem(STORAGE_KEY); sessionStorage.removeItem('dbops_credential_verified')
    save('credential_password', ''); setEnabled(false); form.resetFields(); message.success('二级密码已关闭')
  }

  return (
    <div style={{ padding: 24 }}>
      <Card title={<Space><SettingOutlined /><span>系统设置</span></Space>}>
        <Tabs defaultActiveKey="packages" items={[
          { key: 'packages', label: <Space><CloudOutlined />安装包管理</Space>, children: <PackageManager /> },
          { key: 'security', label: <Space><LockOutlined />安全设置</Space>, children: (
            <Card type="inner" title="实例密码查看保护">
              <div style={{ marginBottom: 16 }}>
                <Space align="center">
                  <Switch checked={enabled} onChange={(c) => {
                    if (c) { handleSave().catch(() => { message.warning('请先填写并确认二级密码'); setEnabled(false) }) }
                    else { handleDisable() }
                  }} />
                  <Text>{enabled ? '已开启' : '未开启'}</Text>
                  {enabled && <Tag color="success">保护中</Tag>}
                </Space>
                <Text type="secondary" style={{ display: 'block', marginTop: 8 }}>
                  开启后在实例管理页面查看密码时需输入二级密码验证，浏览器会话内有效。
                </Text>
              </div>
              <Form form={form} layout="vertical" style={{ maxWidth: 400 }}>
                <Form.Item name="password" label="二级密码" rules={[{ required: true }]}>
                  <Input.Password placeholder="设置二级密码" autoComplete="new-password" />
                </Form.Item>
                <Form.Item name="confirm_password" label="确认二级密码" rules={[{ required: true }]}>
                  <Input.Password placeholder="再次输入" autoComplete="new-password" />
                </Form.Item>
                <Form.Item>
                  <Space>
                    <Button type="primary" icon={<LockOutlined />} onClick={handleSave}>{enabled ? '更新密码' : '保存密码'}</Button>
                    {enabled && <Button danger onClick={handleDisable}>关闭保护</Button>}
                  </Space>
                </Form.Item>
              </Form>
            </Card>
          )},
          { key: 'mysql-credential', label: <Space><DatabaseOutlined />MySQL 账号</Space>, children: <MySQLCredentialConfig /> },
          { key: 'password-policy', label: <Space><LockOutlined />密码策略</Space>, children: <PasswordPolicyConfig /> },
          { key: 'params', label: <Space><ToolOutlined />平台参数</Space>, children: <PlatformParams /> },
          { key: 'metrics', label: <Space><DatabaseOutlined />监控指标</Space>, children: <MetricsConfig /> },
        ]} />
      </Card>
    </div>
  )
}

// ─── 安装包管理 ───────────────────────────────────────────────────────

const PACKAGE_CATALOG = [
  // MySQL Linux
  { os: 'Linux', product: 'MySQL', version: '8.4.0', branch: 'MySQL-8.4', filename: 'mysql-8.4.0-linux-glibc2.28-x86_64.tar.xz', arch: 'x86_64', glibc: '2.28', note: 'LTS, RHEL 8+/Ubuntu 20+' },
  { os: 'Linux', product: 'MySQL', version: '8.0.36', branch: 'MySQL-8.0', filename: 'mysql-8.0.36-linux-glibc2.17-x86_64.tar.xz', arch: 'x86_64', glibc: '2.17', note: 'CentOS 7' },
  { os: 'Linux', product: 'MySQL', version: '8.0.37', branch: 'MySQL-8.0', filename: 'mysql-8.0.37-linux-glibc2.28-x86_64.tar.xz', arch: 'x86_64', glibc: '2.28', note: 'RHEL 8+/Ubuntu 20+' },
  { os: 'Linux', product: 'MySQL', version: '5.7.44', branch: 'MySQL-5.7', filename: 'mysql-5.7.44-linux-glibc2.12-x86_64.tar.gz', arch: 'x86_64', glibc: '2.12', note: '旧系统' },
  // MySQL Windows
  { os: 'Windows', product: 'MySQL', version: '8.4.0', branch: 'MySQL-8.4', filename: 'mysql-8.4.0-winx64.zip', arch: 'x64', glibc: '-', note: 'Windows x64' },
  { os: 'Windows', product: 'MySQL', version: '8.0.36', branch: 'MySQL-8.0', filename: 'mysql-8.0.36-winx64.zip', arch: 'x64', glibc: '-', note: 'Windows x64' },
  { os: 'Windows', product: 'MySQL', version: '5.7.44', branch: 'MySQL-5.7', filename: 'mysql-5.7.44-winx64.zip', arch: 'x64', glibc: '-', note: 'Windows x64' },
  // Percona Linux
  { os: 'Linux', product: 'Percona', version: '8.0.36-28', branch: 'Percona-Server-8.0', filename: 'Percona-Server-8.0.36-28-Linux.x86_64.glibc2.28.tar.gz', arch: 'x86_64', glibc: '2.28', note: 'MySQL 兼容' },
  { os: 'Linux', product: 'Percona', version: '5.7.44-48', branch: 'Percona-Server-5.7', filename: 'Percona-Server-5.7.44-48-Linux.x86_64.glibc2.17.tar.gz', arch: 'x86_64', glibc: '2.17', note: '旧系统' },
  // MariaDB Linux
  { os: 'Linux', product: 'MariaDB', version: '11.4.2', branch: 'mariadb-11.4.2', filename: 'mariadb-11.4.2-linux-systemd-x86_64.tar.gz', arch: 'x86_64', glibc: '-', note: '最新 LTS' },
  { os: 'Linux', product: 'MariaDB', version: '10.11.4', branch: 'mariadb-10.11.4', filename: 'mariadb-10.11.4-linux-systemd-x86_64.tar.gz', arch: 'x86_64', glibc: '-', note: 'LTS' },
  { os: 'Linux', product: 'MariaDB', version: '10.6.16', branch: 'mariadb-10.6.16', filename: 'mariadb-10.6.16-linux-systemd-x86_64.tar.gz', arch: 'x86_64', glibc: '-', note: '' },
  // MariaDB Windows
  { os: 'Windows', product: 'MariaDB', version: '10.11.4', branch: 'mariadb-10.11.4', filename: 'mariadb-10.11.4-winx64.zip', arch: 'x64', glibc: '-', note: 'Windows x64' },
]

const MIRROR_URLS: Record<string, string> = {
  MySQL: 'https://mirrors.tuna.tsinghua.edu.cn/mysql',
  Percona: 'https://mirrors.tuna.tsinghua.edu.cn/percona',
  MariaDB: 'https://mirrors.tuna.tsinghua.edu.cn/mariadb',
}

const PackageManager: React.FC = () => {
  const [relayDir, setRelayDir] = useState('')
  const [relayUrl, setRelayUrl] = useState('')
  const [relayFiles, setRelayFiles] = useState<Set<string>>(new Set())
  const [downloading, setDownloading] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)
  const [customUrl, setCustomUrl] = useState('')
  const [customFilename, setCustomFilename] = useState('')
  const [osFilter, setOsFilter] = useState('all')
  const [productFilter, setProductFilter] = useState('all')
  const { settings, save: saveSetting } = usePlatformSettings()
  const token = () => localStorage.getItem('token') || ''

  useEffect(() => {
    const raw = settings.relay_config
    if (raw) {
      try {
        const cfg = typeof raw === 'string' ? JSON.parse(raw) : raw
        setRelayDir(cfg.scan_path || '')
        setRelayUrl(cfg.relay_url || 'http://10.3.67.52:8888')
      } catch { /* ignore */ }
    }
    handleRefresh()
  }, [])

  const saveRelayCfg = () => { saveSetting('relay_config', JSON.stringify({ relay_url: relayUrl, scan_path: relayDir })) }

  const handleRefresh = async () => {
    setLoading(true)
    try {
      const res = await fetch('/api/v1/relay/scan-remote', {
        method: 'POST', headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token()}` },
        body: JSON.stringify({ path: '' }),
      })
      const json = await res.json()
      setRelayFiles(new Set<string>((json?.data?.packages || []).map((p: any) => p.name)))
    } catch { /* ignore */ }
    finally { setLoading(false) }
  }

  const handleDownload = async (pkg: typeof PACKAGE_CATALOG[0]) => {
    const base = MIRROR_URLS[pkg.product] || 'https://mirrors.tuna.tsinghua.edu.cn/mysql'
    const url = `${base.replace(/\/+$/, '')}/${pkg.branch}/${pkg.filename}`
    setDownloading(pkg.filename)
    try {
      const res = await fetch('/api/v1/relay/download-to-relay', {
        method: 'POST', headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token()}` },
        body: JSON.stringify({ url, filename: pkg.filename, target_path: `${pkg.product.toLowerCase()}/${pkg.version}` }),
      })
      const json = await res.json()
      if (json?.code === 200) { message.success(`${pkg.filename} 下载完成`); setRelayFiles(prev => new Set(prev).add(pkg.filename)) }
      else message.error(`下载失败: ${json?.message}`)
    } catch (e: any) { message.error(`下载失败: ${e?.message}`) }
    finally { setDownloading(null) }
  }

  const handleDownloadCustom = async () => {
    if (!customUrl) { message.warning('请输入下载 URL'); return }
    const filename = customFilename || customUrl.split('/').pop() || 'package'
    setDownloading(filename)
    try {
      const res = await fetch('/api/v1/relay/download-to-relay', {
        method: 'POST', headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token()}` },
        body: JSON.stringify({ url: customUrl, filename, target_path: relayDir }),
      })
      const json = await res.json()
      if (json?.code === 200) { message.success(`${filename} 下载完成`); setRelayFiles(prev => new Set(prev).add(filename)) }
      else message.error(`下载失败: ${json?.message}`)
    } catch (e: any) { message.error(`下载失败: ${e?.message}`) }
    finally { setDownloading(null) }
  }

  const handleUpload = async (file: File) => {
    const fd = new FormData(); fd.append('file', file); fd.append('path', relayDir)
    try {
      const res = await fetch('/api/v1/relay/upload', { method: 'POST', headers: { Authorization: `Bearer ${token()}` }, body: fd })
      const json = await res.json()
      if (json?.code === 200) { message.success(`${file.name} 上传成功`); setRelayFiles(prev => new Set(prev).add(file.name)) }
      else message.error(`上传失败: ${json?.message}`)
    } catch (e: any) { message.error(`上传失败: ${e?.message}`) }
  }

  const handleDelete = async (filename: string) => {
    try {
      const res = await fetch('/api/v1/relay/delete-package', {
        method: 'POST', headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token()}` },
        body: JSON.stringify({ path: filename }),
      })
      const json = await res.json()
      if (json?.code === 200) { message.success('已删除'); setRelayFiles(prev => { const n = new Set(prev); n.delete(filename); return n }) }
      else message.error(`删除失败: ${json?.message}`)
    } catch (e: any) { message.error(`删除失败: ${e?.message}`) }
  }

  const filtered = PACKAGE_CATALOG.filter(p =>
    (osFilter === 'all' || p.os === osFilter) && (productFilter === 'all' || p.product === productFilter)
  )
  const missingCount = filtered.filter(p => !relayFiles.has(p.filename)).length

  return (
    <div>
      {/* 中继服务器配置 */}
      <Card type="inner" title="中继服务器配置" size="small" style={{ marginBottom: 12 }}>
        <Row gutter={12} align="middle">
          <Col span={8}>
            <Input size="small" value={relayUrl} onChange={(e) => setRelayUrl(e.target.value)}
              onBlur={saveRelayCfg} placeholder="http://10.3.67.52:8888" addonBefore="中继 URL" />
          </Col>
          <Col span={6}>
            <Input size="small" value={relayDir} onChange={(e) => setRelayDir(e.target.value)}
              onBlur={saveRelayCfg} placeholder="子路径（可选）" addonBefore="存储路径" />
          </Col>
          <Col span={4}>
            <Button size="small" onClick={handleRefresh} loading={loading}>扫描中继</Button>
          </Col>
          <Col span={6} style={{ textAlign: 'right' }}>
            {missingCount > 0
              ? <Tag color="warning">{missingCount} 个包缺失</Tag>
              : <Tag color="success">所有包已就绪</Tag>}
          </Col>
        </Row>
      </Card>

      {/* 包目录表格 */}
      <Card type="inner" title="安装包目录" size="small"
        extra={
          <Space>
            <Select size="small" value={osFilter} onChange={setOsFilter} style={{ width: 90 }}
              options={[{ value: 'all', label: '全部OS' }, { value: 'Linux', label: 'Linux' }, { value: 'Windows', label: 'Windows' }]} />
            <Select size="small" value={productFilter} onChange={setProductFilter} style={{ width: 100 }}
              options={[{ value: 'all', label: '全部产品' }, { value: 'MySQL', label: 'MySQL' }, { value: 'Percona', label: 'Percona' }, { value: 'MariaDB', label: 'MariaDB' }]} />
            <input type="file" id="pkg-upload" style={{ display: 'none' }} accept=".tar.gz,.tar.xz,.tgz,.tar.bz2,.rpm,.deb,.zip"
              onChange={(e) => { if (e.target.files?.[0]) handleUpload(e.target.files[0]); e.target.value = '' }} />
            <Button size="small" onClick={() => document.getElementById('pkg-upload')?.click()}>上传</Button>
            <Button size="small" onClick={handleRefresh} loading={loading}>刷新</Button>
          </Space>
        }
      >
        <Table size="small" pagination={false} scroll={{ y: 400 }}
          dataSource={filtered.map((p, i) => ({ ...p, key: i, hasIt: relayFiles.has(p.filename) }))}
          columns={[
            { title: 'OS', dataIndex: 'os', key: 'os', width: 65, render: (v: string) => <Tag>{v}</Tag> },
            { title: '产品', dataIndex: 'product', key: 'product', width: 80, render: (v: string) => <Tag color={v === 'MySQL' ? 'blue' : v === 'Percona' ? 'purple' : 'orange'}>{v}</Tag> },
            { title: '版本', dataIndex: 'version', key: 'version', width: 95 },
            { title: 'arch', dataIndex: 'arch', key: 'arch', width: 60 },
            { title: 'glibc', dataIndex: 'glibc', key: 'glibc', width: 50 },
            { title: '说明', dataIndex: 'note', key: 'note', width: 120, ellipsis: true },
            { title: '文件名', dataIndex: 'filename', key: 'filename', ellipsis: true },
            { title: '中继', dataIndex: 'hasIt', key: 'status', width: 50, render: (v: boolean) => v ? <Tag color="success">✓</Tag> : <Tag color="warning">缺</Tag> },
            { title: '', key: 'act', width: 80, render: (_: any, r: any) => r.hasIt
                ? <Button size="small" danger onClick={() => handleDelete(r.filename)}>删除</Button>
                : <Button size="small" type="primary" loading={downloading === r.filename} disabled={!!downloading} onClick={() => handleDownload(r)}>下载</Button> },
          ]}
        />
      </Card>

      {/* 自定义下载 */}
      <Card type="inner" title="自定义下载" size="small" style={{ marginTop: 12 }}>
        <Space.Compact style={{ width: '100%' }}>
          <Input size="small" value={customUrl} onChange={(e) => setCustomUrl(e.target.value)} placeholder="https://..." style={{ flex: 1 }} />
          <Input size="small" value={customFilename} onChange={(e) => setCustomFilename(e.target.value)} placeholder="文件名" style={{ width: 200 }} />
          <Button size="small" type="primary" onClick={handleDownloadCustom} loading={!!downloading}>下载</Button>
        </Space.Compact>
      </Card>

      {/* Repo/Apt 源管理 */}
      <RepoSourceManager />
    </div>
  )
}

// ─── Repo/Apt 源管理 ─────────────────────────────────────────────────
interface RepoSource { id: string; label: string; os: 'Linux' | 'Windows'; type: 'yum' | 'apt' | 'zypper'; content: string; enabled: boolean }

const DEFAULT_REPO_SOURCES: RepoSource[] = [
  { id: 'mysql80-yum', label: 'MySQL 8.0 (yum/RHEL 7)', os: 'Linux', type: 'yum', enabled: true, content: `[mysql80-server]
name=MySQL 8.0 Server
baseurl=https://mirrors.tuna.tsinghua.edu.cn/mysql/yum/mysql80-community/el/7/x86_64/
gpgcheck=0
enabled=1` },
  { id: 'mysql84-yum', label: 'MySQL 8.4 LTS (yum/RHEL 9)', os: 'Linux', type: 'yum', enabled: true, content: `[mysql84-server]
name=MySQL 8.4 LTS
baseurl=https://mirrors.tuna.tsinghua.edu.cn/mysql/yum/mysql84-community/el/9/x86_64/
gpgcheck=0
enabled=1` },
  { id: 'mysql80-apt', label: 'MySQL 8.0 (apt/Ubuntu 22.04)', os: 'Linux', type: 'apt', enabled: true, content: `deb https://mirrors.tuna.tsinghua.edu.cn/mysql/apt/ubuntu jammy mysql-8.0` },
  { id: 'mysql84-apt', label: 'MySQL 8.4 LTS (apt/Ubuntu 24.04)', os: 'Linux', type: 'apt', enabled: true, content: `deb https://mirrors.tuna.tsinghua.edu.cn/mysql/apt/ubuntu noble mysql-8.4-lts` },
  { id: 'percona-yum', label: 'Percona (yum)', os: 'Linux', type: 'yum', enabled: false, content: `[percona]
name=Percona
baseurl=https://mirrors.tuna.tsinghua.edu.cn/percona/yum/release/8/RPMS/x86_64/
gpgcheck=0
enabled=1` },
  { id: 'percona-apt', label: 'Percona (apt)', os: 'Linux', type: 'apt', enabled: false, content: `deb https://mirrors.tuna.tsinghua.edu.cn/percona/apt jammy main` },
  { id: 'mariadb-apt', label: 'MariaDB 10.11 (apt)', os: 'Linux', type: 'apt', enabled: false, content: `deb https://mirrors.tuna.tsinghua.edu.cn/mariadb/repo/10.11/ubuntu jammy main` },
]

const RepoSourceManager: React.FC = () => {
  const [sources, setSources] = useState<RepoSource[]>(DEFAULT_REPO_SOURCES)
  const [editId, setEditId] = useState<string | null>(null)
  const [editContent, setEditContent] = useState('')
  const [pushLoading, setPushLoading] = useState<string | null>(null)
  const { settings, save: saveSetting } = usePlatformSettings()
  const token = () => localStorage.getItem('token') || ''

  useEffect(() => {
    const raw = settings.repo_sources
    if (raw) {
      try { const saved: RepoSource[] = typeof raw === 'string' ? JSON.parse(raw) : raw; if (saved.length > 0) setSources(saved) } catch { /* ignore */ }
    }
  }, [])

  const handleSave = () => { saveSetting('repo_sources', JSON.stringify(sources)); message.success('源配置已保存') }

  const handlePush = async (src: RepoSource) => {
    setPushLoading(src.id)
    try {
      const res = await fetch('/api/v1/relay/push-repo-source', {
        method: 'POST', headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token()}` },
        body: JSON.stringify({ label: src.label, type: src.type, content: src.content }),
      })
      const json = await res.json()
      if (json?.code === 200) message.success(`${src.label}: 推送成功`)
      else message.error(`推送失败: ${json?.message}`)
    } catch (e: any) { message.error(`推送失败: ${e?.message}`) }
    finally { setPushLoading(null) }
  }

  const addSource = () => {
    const newId = `custom-${Date.now()}`
    setSources([...sources, { id: newId, label: '自定义源', os: 'Linux', type: 'yum', enabled: true, content: '' }])
    setEditId(newId); setEditContent('')
  }
  const updateSource = (id: string, field: keyof RepoSource, val: any) => { setSources(sources.map(s => s.id === id ? { ...s, [field]: val } : s)) }
  const removeSource = (id: string) => { setSources(sources.filter(s => s.id !== id)) }

  return (
    <Card type="inner" title="Yum/Apt 源配置" size="small" style={{ marginTop: 12 }}
      extra={<Space>
        <Button size="small" onClick={addSource}>添加源</Button>
        <Button size="small" type="primary" onClick={handleSave}>保存配置</Button>
      </Space>}>
      <Text type="secondary" style={{ display: 'block', marginBottom: 8 }}>
        配置 yum/apt 源，部署时自动推送到远程主机。支持 RHEL/CentOS (yum) 和 Ubuntu/Debian (apt)。
      </Text>
      {sources.map((src) => (
        <div key={src.id} style={{ marginBottom: 6, padding: 6, border: '1px solid #f0f0f0', borderRadius: 4, background: src.enabled ? '#fafafa' : '#f5f5f5' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
            <Switch size="small" checked={src.enabled} onChange={(v) => updateSource(src.id, 'enabled', v)} />
            <Input size="small" value={src.label} onChange={(e) => updateSource(src.id, 'label', e.target.value)} style={{ width: 180 }} />
            <Select size="small" value={src.type} onChange={(v) => updateSource(src.id, 'type', v)} style={{ width: 80 }}
              options={[{ value: 'yum', label: 'yum' }, { value: 'apt', label: 'apt' }, { value: 'zypper', label: 'zypper' }]} />
            <Select size="small" value={src.os} onChange={(v) => updateSource(src.id, 'os', v)} style={{ width: 90 }}
              options={[{ value: 'Linux', label: 'Linux' }, { value: 'Windows', label: 'Windows' }]} />
            <Button size="small" onClick={() => { setEditId(editId === src.id ? null : src.id); setEditContent(src.content) }}>
              {editId === src.id ? '收起' : '编辑'}
            </Button>
            <Button size="small" type="primary" onClick={() => handlePush(src)} loading={pushLoading === src.id} disabled={!src.enabled}>
              推送到主机
            </Button>
            <Button size="small" danger onClick={() => removeSource(src.id)}>×</Button>
          </div>
          {editId === src.id && (
            <Input.TextArea value={editContent} onChange={(e) => { setEditContent(e.target.value); updateSource(src.id, 'content', e.target.value) }}
              rows={4} style={{ marginTop: 6, fontFamily: 'monospace', fontSize: 12 }} />
          )}
        </div>
      ))}
    </Card>
  )
}

// ─── MySQL 账号配置 ──────────────────────────────────────────────────
const CREDENTIAL_STORAGE_KEY = 'dbops_default_mysql_credential'

const MySQLCredentialConfig: React.FC = () => {
  const [form] = Form.useForm()
  const { settings, save: saveSetting } = usePlatformSettings()
  useEffect(() => {
    const raw = settings.mysql_credential || localStorage.getItem(CREDENTIAL_STORAGE_KEY)
    if (raw) { try { const c = typeof raw === 'string' ? JSON.parse(raw) : raw; form.setFieldsValue({ mysql_user: c.username || 'root', mysql_password: c.password || '' }) } catch {} }
  }, [settings.mysql_credential])
  const handleSave = async () => {
    const values = await form.validateFields()
    const json = JSON.stringify({ username: values.mysql_user, password: values.mysql_password })
    localStorage.setItem(CREDENTIAL_STORAGE_KEY, json); saveSetting('mysql_credential', json); message.success('默认 MySQL 账号已保存')
  }
  return (
    <Card type="inner" title="集群部署默认 MySQL 账号">
      <Text type="secondary" style={{ display: 'block', marginBottom: 16 }}>集群部署时使用的默认 MySQL root 账号和密码。此设置会同步到集群部署页面。</Text>
      <Form form={form} layout="vertical" style={{ maxWidth: 400 }} initialValues={{ mysql_user: 'root' }}>
        <Form.Item name="mysql_user" label="用户名" rules={[{ required: true }]}><Input placeholder="root" /></Form.Item>
        <Form.Item name="mysql_password" label="密码" rules={[{ required: true, min: 8 }]}><Input.Password placeholder="MySQL root 密码" autoComplete="new-password" /></Form.Item>
        <Form.Item><Button type="primary" onClick={handleSave}>保存账号</Button></Form.Item>
      </Form>
    </Card>
  )
}

// ─── 密码策略配置 ────────────────────────────────────────────────────
const POLICY_STORAGE_KEY = 'dbops_password_policy'
const defaultPolicy = { min_length: 8, require_uppercase: true, require_lowercase: true, require_digit: true, require_special: true }

const PasswordPolicyConfig: React.FC = () => {
  const [form] = Form.useForm()
  const { settings, save: saveSetting } = usePlatformSettings()
  useEffect(() => {
    const raw = settings.password_policy || localStorage.getItem(POLICY_STORAGE_KEY)
    if (raw) { try { form.setFieldsValue(typeof raw === 'string' ? JSON.parse(raw) : raw) } catch {} } else { form.setFieldsValue(defaultPolicy) }
  }, [settings.password_policy])
  const handleSave = () => { const v = form.getFieldsValue(); const j = JSON.stringify(v); localStorage.setItem(POLICY_STORAGE_KEY, j); saveSetting('password_policy', j); message.success('密码策略已保存') }
  return (
    <Card type="inner" title="全平台密码复杂度要求">
      <Text type="secondary" style={{ display: 'block', marginBottom: 16 }}>设置平台上所有密码的最低要求。</Text>
      <Form form={form} layout="horizontal" labelCol={{ span: 8 }} wrapperCol={{ span: 16 }} style={{ maxWidth: 450 }}>
        <Form.Item name="min_length" label="最小长度"><InputNumber min={4} max={32} style={{ width: '100%' }} /></Form.Item>
        <Form.Item name="require_uppercase" label="需要大写字母" valuePropName="checked"><Switch /></Form.Item>
        <Form.Item name="require_lowercase" label="需要小写字母" valuePropName="checked"><Switch /></Form.Item>
        <Form.Item name="require_digit" label="需要数字" valuePropName="checked"><Switch /></Form.Item>
        <Form.Item name="require_special" label="需要特殊字符" valuePropName="checked"><Switch /></Form.Item>
        <Form.Item><Button type="primary" onClick={handleSave}>保存策略</Button></Form.Item>
      </Form>
    </Card>
  )
}

// ─── 平台参数配置 ────────────────────────────────────────────────────
const PARAMS_STORAGE_KEY = 'dbops_platform_params'
const defaultParams = { agent_port: 9090, default_mysql_port: 3306, default_os_user: 'mysql', default_datadir_prefix: '/data/mysql', health_check_interval_sec: 30, deploy_timeout_min: 30, backup_retention_days: 7 }

const PlatformParams: React.FC = () => {
  const [form] = Form.useForm()
  const { settings, save: saveSetting } = usePlatformSettings()
  useEffect(() => {
    const raw = settings.platform_params || localStorage.getItem(PARAMS_STORAGE_KEY)
    if (raw) { try { form.setFieldsValue(typeof raw === 'string' ? JSON.parse(raw) : raw) } catch {} } else { form.setFieldsValue(defaultParams) }
  }, [settings.platform_params])
  const handleSave = () => { const v = form.getFieldsValue(); const j = JSON.stringify(v); localStorage.setItem(PARAMS_STORAGE_KEY, j); saveSetting('platform_params', j); message.success('平台参数已保存') }
  return (
    <Card type="inner" title="平台默认参数">
      <Text type="secondary" style={{ display: 'block', marginBottom: 16 }}>这些参数在创建实例和集群部署时作为默认值使用。</Text>
      <Form form={form} layout="horizontal" labelCol={{ span: 8 }} wrapperCol={{ span: 16 }} style={{ maxWidth: 500 }}>
        <Form.Item name="agent_port" label="Agent 端口"><InputNumber min={1} max={65535} style={{ width: '100%' }} /></Form.Item>
        <Form.Item name="default_mysql_port" label="默认 MySQL 端口"><InputNumber min={1} max={65535} style={{ width: '100%' }} /></Form.Item>
        <Form.Item name="default_os_user" label="默认 OS 用户"><Input placeholder="mysql" /></Form.Item>
        <Form.Item name="default_datadir_prefix" label="数据目录前缀"><Input placeholder="/data/mysql" /></Form.Item>
        <Form.Item name="health_check_interval_sec" label="健康检查间隔(秒)"><InputNumber min={10} max={3600} style={{ width: '100%' }} /></Form.Item>
        <Form.Item name="deploy_timeout_min" label="部署超时(分钟)"><InputNumber min={5} max={120} style={{ width: '100%' }} /></Form.Item>
        <Form.Item name="backup_retention_days" label="备份保留天数"><InputNumber min={1} max={365} style={{ width: '100%' }} /></Form.Item>
        <Form.Item><Button type="primary" onClick={handleSave}>保存参数</Button></Form.Item>
      </Form>
    </Card>
  )
}

// ─── 监控指标阈值配置 ────────────────────────────────────────────────
const METRICS_STORAGE_KEY = 'dbops_metrics_thresholds'
const defaultMetrics = { replication_lag_warn_sec: 10, replication_lag_critical_sec: 60, connection_usage_warn_pct: 80, connection_usage_critical_pct: 95, disk_usage_warn_pct: 80, disk_usage_critical_pct: 90, memory_usage_warn_pct: 85, qps_threshold: 10000 }

const MetricsConfig: React.FC = () => {
  const [form] = Form.useForm()
  const { settings, save: saveSetting } = usePlatformSettings()
  useEffect(() => {
    const raw = settings.metrics_thresholds || localStorage.getItem(METRICS_STORAGE_KEY)
    if (raw) { try { form.setFieldsValue(typeof raw === 'string' ? JSON.parse(raw) : raw) } catch {} } else { form.setFieldsValue(defaultMetrics) }
  }, [settings.metrics_thresholds])
  const handleSave = () => { const v = form.getFieldsValue(); const j = JSON.stringify(v); localStorage.setItem(METRICS_STORAGE_KEY, j); saveSetting('metrics_thresholds', j); message.success('监控阈值已保存') }
  return (
    <Card type="inner" title="监控指标阈值">
      <Text type="secondary" style={{ display: 'block', marginBottom: 16 }}>设置告警阈值，当指标超过阈值时在监控仪表盘中显示警告。</Text>
      <Form form={form} layout="horizontal" labelCol={{ span: 10 }} wrapperCol={{ span: 14 }} style={{ maxWidth: 520 }}>
        <Divider plain>复制延迟</Divider>
        <Form.Item name="replication_lag_warn_sec" label="警告阈值(秒)"><InputNumber min={1} style={{ width: '100%' }} /></Form.Item>
        <Form.Item name="replication_lag_critical_sec" label="严重阈值(秒)"><InputNumber min={1} style={{ width: '100%' }} /></Form.Item>
        <Divider plain>连接数</Divider>
        <Form.Item name="connection_usage_warn_pct" label="警告阈值(%)"><InputNumber min={1} max={100} style={{ width: '100%' }} /></Form.Item>
        <Form.Item name="connection_usage_critical_pct" label="严重阈值(%)"><InputNumber min={1} max={100} style={{ width: '100%' }} /></Form.Item>
        <Divider plain>资源使用</Divider>
        <Form.Item name="disk_usage_warn_pct" label="磁盘警告(%)"><InputNumber min={1} max={100} style={{ width: '100%' }} /></Form.Item>
        <Form.Item name="disk_usage_critical_pct" label="磁盘严重(%)"><InputNumber min={1} max={100} style={{ width: '100%' }} /></Form.Item>
        <Form.Item name="memory_usage_warn_pct" label="内存警告(%)"><InputNumber min={1} max={100} style={{ width: '100%' }} /></Form.Item>
        <Form.Item name="qps_threshold" label="QPS 阈值"><InputNumber min={100} style={{ width: '100%' }} /></Form.Item>
        <Form.Item><Button type="primary" onClick={handleSave}>保存阈值</Button></Form.Item>
      </Form>
    </Card>
  )
}

export default SecuritySettings
