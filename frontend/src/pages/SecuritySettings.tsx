import React, { useState, useEffect } from 'react'
import { Button, Card, Col, Form, Input, InputNumber, message, Row, Select, Space, Switch, Table, Tabs, Tag, Typography } from 'antd'
import { CheckCircleOutlined, CloudOutlined, DatabaseOutlined, DownloadOutlined, LockOutlined, ReloadOutlined, SettingOutlined, ToolOutlined } from '@ant-design/icons'
import { hostApi } from '../services/api'
import { usePlatformSettings } from '../services/useSettings'

const { Text } = Typography
const token = () => localStorage.getItem('token') || ''

// ─── 安全设置 ─────────────────────────────────────────────────────────────

const SecurityTab: React.FC = () => {
  const [enabled, setEnabled] = useState(false)
  const [form] = Form.useForm()
  const { settings, save } = usePlatformSettings()
  const CRED_KEY = 'dbops_credential_password'
  useEffect(() => {
    const s = settings.credential_password || localStorage.getItem(CRED_KEY)
    if (s) { setEnabled(true); form.setFieldsValue({ password: s, confirm_password: s }) }
  }, [settings])
  const handleSave = async () => {
    const v = await form.validateFields()
    if (v.password !== v.confirm_password) { message.error('两次密码不一致'); return }
    localStorage.setItem(CRED_KEY, v.password)
    await save('credential_password', v.password)
    setEnabled(true); message.success('二级密码已设置')
  }
  const handleDisable = () => {
    localStorage.removeItem(CRED_KEY); sessionStorage.removeItem('dbops_credential_verified')
    save('credential_password', ''); setEnabled(false); form.resetFields(); message.success('已关闭')
  }
  return (
    <Row gutter={16}>
      <Col span={12}>
        <Card type="inner" title="查看密码保护" size="small">
          <Space align="center" style={{ marginBottom: 8 }}>
            <Switch checked={enabled} onChange={(c) => {
              if (c) { handleSave().catch(() => { message.warning('请先填写密码'); setEnabled(false) }) }
              else { handleDisable() }
            }} />
            <Tag color={enabled ? 'success' : 'default'}>{enabled ? '已开启' : '未开启'}</Tag>
          </Space>
          <Text type="secondary" style={{ display: 'block', marginBottom: 12, fontSize: 12 }}>开启后查看实例密码需输入二级密码验证（浏览器会话内有效）。</Text>
          <Form form={form} layout="vertical" size="small">
            <Form.Item name="password" label="二级密码" rules={[{ required: true }]}><Input.Password size="small" autoComplete="new-password" /></Form.Item>
            <Form.Item name="confirm_password" label="确认" rules={[{ required: true }]}><Input.Password size="small" autoComplete="new-password" /></Form.Item>
            <Form.Item>
              <Button size="small" type="primary" icon={<LockOutlined />} onClick={handleSave}>{enabled ? '更新' : '保存'}</Button>
              {enabled && <Button size="small" danger style={{ marginLeft: 8 }} onClick={handleDisable}>关闭</Button>}
            </Form.Item>
          </Form>
        </Card>
      </Col>
      <Col span={12}>
        <Card type="inner" title="密码策略" size="small"><PasswordPolicyForm /></Card>
      </Col>
    </Row>
  )
}

const POLICY_KEY = 'dbops_password_policy'
const defaultPolicy = { min_length: 8, require_uppercase: true, require_lowercase: true, require_digit: true, require_special: true, max_age_days: 90, history_count: 5 }

const PasswordPolicyForm: React.FC = () => {
  const [form] = Form.useForm()
  const { settings, save } = usePlatformSettings()
  useEffect(() => {
    const raw = settings.password_policy || localStorage.getItem(POLICY_KEY)
    if (raw) { try { form.setFieldsValue(typeof raw === 'string' ? JSON.parse(raw) : raw) } catch {} } else { form.setFieldsValue(defaultPolicy) }
  }, [settings.password_policy])
  const handleSave = () => { const v = form.getFieldsValue(); const j = JSON.stringify(v); localStorage.setItem(POLICY_KEY, j); save('password_policy', j); message.success('密码策略已保存') }
  return (
    <Form form={form} layout="vertical" size="small">
      <Row gutter={8}>
        <Col span={8}><Form.Item name="min_length" label="最小长度"><InputNumber min={4} max={32} size="small" style={{ width: '100%' }} /></Form.Item></Col>
        <Col span={8}><Form.Item name="max_age_days" label="有效期(天)"><InputNumber min={0} size="small" style={{ width: '100%' }} /></Form.Item></Col>
        <Col span={8}><Form.Item name="history_count" label="历史密码数"><InputNumber min={0} size="small" style={{ width: '100%' }} /></Form.Item></Col>
      </Row>
      <Space wrap style={{ marginBottom: 8 }}>
        <Form.Item name="require_uppercase" valuePropName="checked" style={{ margin: 0 }}><Switch size="small" /> 大写</Form.Item>
        <Form.Item name="require_lowercase" valuePropName="checked" style={{ margin: 0 }}><Switch size="small" /> 小写</Form.Item>
        <Form.Item name="require_digit" valuePropName="checked" style={{ margin: 0 }}><Switch size="small" /> 数字</Form.Item>
        <Form.Item name="require_special" valuePropName="checked" style={{ margin: 0 }}><Switch size="small" /> 特殊字符</Form.Item>
      </Space>
      <Button size="small" type="primary" onClick={handleSave}>保存策略</Button>
    </Form>
  )
}

// ─── 监控指标 ─────────────────────────────────────────────────────────────

const METRICS_KEY = 'dbops_metrics_thresholds'
const CRED_KEY2 = 'dbops_default_mysql_credential'
const defaultMetrics = {
  connection_warn: 80, connection_critical: 95, qps_warn: 5000, qps_critical: 10000,
  slow_query_warn: 5, slow_query_critical: 20, repl_lag_warn: 10, repl_lag_critical: 60,
  buffer_pool_warn: 85, deadlock_warn_per_hour: 3, tmp_table_warn: 1024, thread_running_warn: 100,
  disk_warn: 80, disk_critical: 90, mem_warn: 85, cpu_warn: 80,
}

const MetricsTab: React.FC = () => {
  const [form] = Form.useForm()
  const [credForm] = Form.useForm()
  const { settings, save } = usePlatformSettings()
  useEffect(() => {
    const raw = settings.metrics_thresholds || localStorage.getItem(METRICS_KEY)
    if (raw) { try { form.setFieldsValue(typeof raw === 'string' ? JSON.parse(raw) : raw) } catch {} } else { form.setFieldsValue(defaultMetrics) }
    const rawC = settings.mysql_credential || localStorage.getItem(CRED_KEY2)
    if (rawC) { try { const c = typeof rawC === 'string' ? JSON.parse(rawC) : rawC; credForm.setFieldsValue({ mysql_user: c.username || 'root', mysql_password: c.password || '' }) } catch {} }
  }, [settings])
  const saveMetrics = () => { const v = form.getFieldsValue(); const j = JSON.stringify(v); localStorage.setItem(METRICS_KEY, j); save('metrics_thresholds', j); message.success('监控阈值已保存') }
  const saveCred = async () => { const v = await credForm.validateFields(); const j = JSON.stringify({ username: v.mysql_user, password: v.mysql_password }); localStorage.setItem(CRED_KEY2, j); save('mysql_credential', j); message.success('MySQL 账号已保存') }
  const F = ({ name, label }: { name: string; label: string }) => (
    <Col span={6}><Form.Item name={name} label={label} style={{ marginBottom: 8 }}><InputNumber min={0} size="small" style={{ width: '100%' }} /></Form.Item></Col>
  )
  return (
    <Row gutter={16}>
      <Col span={5}>
        <Card type="inner" title="MySQL 账号" size="small">
          <Form form={credForm} layout="vertical" size="small" initialValues={{ mysql_user: 'root' }}>
            <Form.Item name="mysql_user" label="用户名" rules={[{ required: true }]}><Input size="small" /></Form.Item>
            <Form.Item name="mysql_password" label="密码" rules={[{ required: true, min: 8 }]}><Input.Password size="small" autoComplete="new-password" /></Form.Item>
            <Button size="small" type="primary" onClick={saveCred}>保存</Button>
          </Form>
        </Card>
      </Col>
      <Col span={19}>
        <Card type="inner" title="监控指标阈值" size="small">
          <Form form={form} layout="horizontal" labelCol={{ span: 10 }} wrapperCol={{ span: 14 }} size="small">
            <Text strong style={{ fontSize: 12 }}>通用</Text>
            <Row gutter={8} style={{ marginBottom: 4 }}>
              <F name="connection_warn" label="连接数警告%" /><F name="qps_warn" label="QPS警告" /><F name="slow_query_warn" label="慢查询/秒" /><F name="repl_lag_warn" label="延迟警告(秒)" />
            </Row>
            <Row gutter={8} style={{ marginBottom: 12 }}>
              <F name="connection_critical" label="连接数严重%" /><F name="qps_critical" label="QPS严重" /><F name="slow_query_critical" label="慢查询严重" /><F name="repl_lag_critical" label="延迟严重(秒)" />
            </Row>
            <Text strong style={{ fontSize: 12 }}>MySQL 8.0+</Text>
            <Row gutter={8} style={{ marginBottom: 12 }}>
              <F name="buffer_pool_warn" label="BufferPool%" /><F name="deadlock_warn_per_hour" label="死锁/小时" /><F name="tmp_table_warn" label="临时表(MB)" /><F name="thread_running_warn" label="活跃线程" />
            </Row>
            <Text strong style={{ fontSize: 12 }}>资源</Text>
            <Row gutter={8} style={{ marginBottom: 12 }}>
              <F name="disk_warn" label="磁盘警告%" /><F name="disk_critical" label="磁盘严重%" /><F name="mem_warn" label="内存警告%" /><F name="cpu_warn" label="CPU警告%" />
            </Row>
            <Button size="small" type="primary" onClick={saveMetrics}>保存阈值</Button>
          </Form>
        </Card>
      </Col>
    </Row>
  )
}

// ─── 平台参数 ─────────────────────────────────────────────────────────────

const PARAMS_KEY = 'dbops_platform_params'
const defaultParams = {
  agent_port: 9090, default_mysql_port: 3306, default_os_user: 'mysql',
  datadir_prefix: '/data/mysql', basedir_prefix: '/opt/mysql',
  health_check_interval_sec: 30, deploy_timeout_min: 30,
  backup_retention_days: 7, backup_schedule: '0 2 * * *',
  slow_query_threshold_sec: 1, max_connections_default: 500,
  innodb_buffer_pool_ratio: 0.7, relay_timeout_sec: 300,
}

const PlatformTab: React.FC = () => {
  const [form] = Form.useForm()
  const { settings, save } = usePlatformSettings()
  useEffect(() => {
    const raw = settings.platform_params || localStorage.getItem(PARAMS_KEY)
    if (raw) { try { form.setFieldsValue(typeof raw === 'string' ? JSON.parse(raw) : raw) } catch {} } else { form.setFieldsValue(defaultParams) }
  }, [settings.platform_params])
  const handleSave = () => { const v = form.getFieldsValue(); const j = JSON.stringify(v); localStorage.setItem(PARAMS_KEY, j); save('platform_params', j); message.success('平台参数已保存') }
  return (
    <Form form={form} layout="horizontal" labelCol={{ span: 8 }} wrapperCol={{ span: 16 }} size="small">
      <Row gutter={16}>
        <Col span={12}>
          <Card type="inner" title="连接与部署" size="small">
            <Form.Item name="agent_port" label="Agent 端口"><InputNumber min={1} max={65535} size="small" style={{ width: '100%' }} /></Form.Item>
            <Form.Item name="default_mysql_port" label="MySQL 端口"><InputNumber min={1} max={65535} size="small" style={{ width: '100%' }} /></Form.Item>
            <Form.Item name="default_os_user" label="OS 用户"><Input size="small" placeholder="mysql" /></Form.Item>
            <Form.Item name="datadir_prefix" label="数据目录前缀"><Input size="small" placeholder="/data/mysql" /></Form.Item>
            <Form.Item name="basedir_prefix" label="安装目录前缀"><Input size="small" placeholder="/opt/mysql" /></Form.Item>
            <Form.Item name="deploy_timeout_min" label="部署超时(分钟)"><InputNumber min={5} max={120} size="small" style={{ width: '100%' }} /></Form.Item>
            <Form.Item name="relay_timeout_sec" label="中继下载超时(秒)"><InputNumber min={30} size="small" style={{ width: '100%' }} /></Form.Item>
          </Card>
        </Col>
        <Col span={12}>
          <Card type="inner" title="监控与备份" size="small">
            <Form.Item name="health_check_interval_sec" label="健康检查(秒)"><InputNumber min={10} max={3600} size="small" style={{ width: '100%' }} /></Form.Item>
            <Form.Item name="backup_retention_days" label="备份保留(天)"><InputNumber min={1} max={365} size="small" style={{ width: '100%' }} /></Form.Item>
            <Form.Item name="backup_schedule" label="备份计划(cron)"><Input size="small" placeholder="0 2 * * *" /></Form.Item>
            <Form.Item name="slow_query_threshold_sec" label="慢查询阈值(秒)"><InputNumber min={0} size="small" style={{ width: '100%' }} /></Form.Item>
            <Form.Item name="max_connections_default" label="默认最大连接数"><InputNumber min={10} size="small" style={{ width: '100%' }} /></Form.Item>
            <Form.Item name="innodb_buffer_pool_ratio" label="BufferPool比例"><InputNumber min={0.1} max={0.9} step={0.1} size="small" style={{ width: '100%' }} /></Form.Item>
          </Card>
        </Col>
      </Row>
      <Form.Item style={{ marginTop: 12 }}><Button size="small" type="primary" onClick={handleSave}>保存参数</Button></Form.Item>
    </Form>
  )
}

// ─── 安装包管理 ───────────────────────────────────────────────────────────

// 每个大版本(x.x)至少一条，覆盖 MySQL 5.7~8.4 + Percona + MariaDB + Windows
const CATALOG = [
  { os: 'Linux', p: 'MySQL',   v: '8.4.0', url: 'https://dev.mysql.com/get/Downloads/MySQL-8.4/mysql-8.4.0-linux-glibc2.28-x86_64.tar.xz', fn: 'mysql-8.4.0-linux-glibc2.28-x86_64.tar.xz', arch: 'x86_64', n: 'LTS glibc2.28' },
  { os: 'Win',   p: 'MySQL',   v: '8.4.0', url: 'https://dev.mysql.com/get/Downloads/MySQL-8.4/mysql-8.4.0-winx64.zip', fn: 'mysql-8.4.0-winx64.zip', arch: 'x64', n: 'Windows' },
  { os: 'Linux', p: 'MySQL',   v: '8.3.0', url: 'https://dev.mysql.com/get/Downloads/MySQL-8.3/mysql-8.3.0-linux-glibc2.28-x86_64.tar.xz', fn: 'mysql-8.3.0-linux-glibc2.28-x86_64.tar.xz', arch: 'x86_64', n: 'glibc2.28' },
  { os: 'Win',   p: 'MySQL',   v: '8.3.0', url: 'https://dev.mysql.com/get/Downloads/MySQL-8.3/mysql-8.3.0-winx64.zip', fn: 'mysql-8.3.0-winx64.zip', arch: 'x64', n: 'Windows' },
  { os: 'Linux', p: 'MySQL',   v: '8.2.0', url: 'https://dev.mysql.com/get/Downloads/MySQL-8.2/mysql-8.2.0-linux-glibc2.28-x86_64.tar.xz', fn: 'mysql-8.2.0-linux-glibc2.28-x86_64.tar.xz', arch: 'x86_64', n: 'glibc2.28' },
  { os: 'Win',   p: 'MySQL',   v: '8.2.0', url: 'https://dev.mysql.com/get/Downloads/MySQL-8.2/mysql-8.2.0-winx64.zip', fn: 'mysql-8.2.0-winx64.zip', arch: 'x64', n: 'Windows' },
  { os: 'Linux', p: 'MySQL',   v: '8.1.0', url: 'https://dev.mysql.com/get/Downloads/MySQL-8.1/mysql-8.1.0-linux-glibc2.28-x86_64.tar.xz', fn: 'mysql-8.1.0-linux-glibc2.28-x86_64.tar.xz', arch: 'x86_64', n: 'glibc2.28' },
  { os: 'Win',   p: 'MySQL',   v: '8.1.0', url: 'https://dev.mysql.com/get/Downloads/MySQL-8.1/mysql-8.1.0-winx64.zip', fn: 'mysql-8.1.0-winx64.zip', arch: 'x64', n: 'Windows' },
  { os: 'Linux', p: 'MySQL',   v: '8.0.36', url: 'https://dev.mysql.com/get/Downloads/MySQL-8.0/mysql-8.0.36-linux-glibc2.17-x86_64.tar.xz', fn: 'mysql-8.0.36-linux-glibc2.17-x86_64.tar.xz', arch: 'x86_64', n: 'glibc2.17' },
  { os: 'Linux', p: 'MySQL',   v: '8.0.37', url: 'https://dev.mysql.com/get/Downloads/MySQL-8.0/mysql-8.0.37-linux-glibc2.28-x86_64.tar.xz', fn: 'mysql-8.0.37-linux-glibc2.28-x86_64.tar.xz', arch: 'x86_64', n: 'glibc2.28' },
  { os: 'Win',   p: 'MySQL',   v: '8.0.36', url: 'https://dev.mysql.com/get/Downloads/MySQL-8.0/mysql-8.0.36-winx64.zip', fn: 'mysql-8.0.36-winx64.zip', arch: 'x64', n: 'Windows' },
  { os: 'Linux', p: 'MySQL',   v: '5.7.44', url: 'https://dev.mysql.com/get/Downloads/MySQL-5.7/mysql-5.7.44-linux-glibc2.12-x86_64.tar.gz', fn: 'mysql-5.7.44-linux-glibc2.12-x86_64.tar.gz', arch: 'x86_64', n: 'glibc2.12' },
  { os: 'Win',   p: 'MySQL',   v: '5.7.44', url: 'https://dev.mysql.com/get/Downloads/MySQL-5.7/mysql-5.7.44-winx64.zip', fn: 'mysql-5.7.44-winx64.zip', arch: 'x64', n: 'Windows' },
  { os: 'Linux', p: 'Percona', v: '8.0.36-28', url: 'https://downloads.percona.com/downloads/Percona-Server-8.0/Percona-Server-8.0.36-28/binary/tarball/Percona-Server-8.0.36-28-Linux.x86_64.glibc2.28.tar.gz', fn: 'Percona-Server-8.0.36-28-Linux.x86_64.glibc2.28.tar.gz', arch: 'x86_64', n: 'MySQL兼容' },
  { os: 'Linux', p: 'Percona', v: '5.7.44-48', url: 'https://downloads.percona.com/downloads/Percona-Server-5.7/Percona-Server-5.7.44-48/binary/tarball/Percona-Server-5.7.44-48-Linux.x86_64.glibc2.17.tar.gz', fn: 'Percona-Server-5.7.44-48-Linux.x86_64.glibc2.17.tar.gz', arch: 'x86_64', n: '旧系统' },
  { os: 'Linux', p: 'MariaDB', v: '11.4.2', url: 'https://archive.mariadb.org/mariadb-11.4.2/bintar-linux-systemd-x86_64/mariadb-11.4.2-linux-systemd-x86_64.tar.gz', fn: 'mariadb-11.4.2-linux-systemd-x86_64.tar.gz', arch: 'x86_64', n: 'LTS' },
  { os: 'Linux', p: 'MariaDB', v: '10.11.4', url: 'https://archive.mariadb.org/mariadb-10.11.4/bintar-linux-systemd-x86_64/mariadb-10.11.4-linux-systemd-x86_64.tar.gz', fn: 'mariadb-10.11.4-linux-systemd-x86_64.tar.gz', arch: 'x86_64', n: 'LTS' },
  { os: 'Win',   p: 'MariaDB', v: '10.11.4', url: 'https://archive.mariadb.org/mariadb-10.11.4/bintar-linux-systemd-x86_64/mariadb-10.11.4-winx64.zip', fn: 'mariadb-10.11.4-winx64.zip', arch: 'x64', n: 'Windows' },
]

const REPO_SOURCES = [
  { id: 'mysql80-yum', label: 'MySQL 8.0 (yum/RHEL7)', type: 'yum', content: '[mysql80-server]\nname=MySQL 8.0\nbaseurl=https://mirrors.tuna.tsinghua.edu.cn/mysql/yum/mysql80-community/el/7/x86_64/\ngpgcheck=0\nenabled=1' },
  { id: 'mysql84-yum', label: 'MySQL 8.4 LTS (yum/RHEL9)', type: 'yum', content: '[mysql84-server]\nname=MySQL 8.4 LTS\nbaseurl=https://mirrors.tuna.tsinghua.edu.cn/mysql/yum/mysql84-community/el/9/x86_64/\ngpgcheck=0\nenabled=1' },
  { id: 'mysql80-apt', label: 'MySQL 8.0 (apt/Ubuntu22.04)', type: 'apt', content: 'deb https://mirrors.tuna.tsinghua.edu.cn/mysql/apt/ubuntu jammy mysql-8.0' },
  { id: 'mysql84-apt', label: 'MySQL 8.4 LTS (apt/Ubuntu24.04)', type: 'apt', content: 'deb https://mirrors.tuna.tsinghua.edu.cn/mysql/apt/ubuntu noble mysql-8.4-lts' },
  { id: 'percona-yum', label: 'Percona (yum)', type: 'yum', content: '[percona]\nname=Percona\nbaseurl=https://mirrors.tuna.tsinghua.edu.cn/percona/yum/release/8/RPMS/x86_64/\ngpgcheck=0\nenabled=1' },
  { id: 'percona-apt', label: 'Percona (apt)', type: 'apt', content: 'deb https://mirrors.tuna.tsinghua.edu.cn/percona/apt jammy main' },
  { id: 'mariadb-apt', label: 'MariaDB 10.11 (apt)', type: 'apt', content: 'deb https://mirrors.tuna.tsinghua.edu.cn/mariadb/repo/10.11/ubuntu jammy main' },
]

const PackageTab: React.FC = () => {
  const [hosts, setHosts] = useState<Array<{id: string; name: string; address: string}>>([])
  const [relayHostId, setRelayHostId] = useState('')
  const [relayPath, setRelayPath] = useState('/opt')
  const [relayFiles, setRelayFiles] = useState<Map<string, {size: string; path: string}>>(new Map())
  const [remoteFiles, setRemoteFiles] = useState<Map<string, {size: string; path: string}>>(new Map())
  const [downloading, setDownloading] = useState<string | null>(null)
  const [progress, setProgress] = useState<Record<string, string>>({})
  const [loading, setLoading] = useState(false)
  const [osFilter, setOsFilter] = useState('all')
  const [productFilter, setProductFilter] = useState('all')
  const [showRepo, setShowRepo] = useState(false)
  const { settings, save } = usePlatformSettings()

  useEffect(() => {
    hostApi.list(100, 0).then((r: any) => setHosts(r?.data || [])).catch(e => console.warn('Failed to load hosts:', e))
    const raw = settings.relay_config
    if (raw) {
      try { const cfg = typeof raw === 'string' ? JSON.parse(raw) : raw; if (cfg.relay_host_id) setRelayHostId(cfg.relay_host_id); if (cfg.relay_path) setRelayPath(cfg.relay_path) } catch {}
    }
    handleScanRelay()
  }, [])

  const saveCfg = () => save('relay_config', JSON.stringify({ relay_host_id: relayHostId, relay_path: relayPath }))

  const handleScanRelay = async () => {
    setLoading(true)
    try {
      const res = await fetch('/api/v1/relay/scan-remote', { method: 'POST', headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token()}` }, body: JSON.stringify({ path: '' }) })
      const json = await res.json()
      const m = new Map<string, {size: string; path: string}>()
      for (const p of (json?.data?.packages || [])) m.set(p.name, { size: p.size || '-', path: p.path || p.name })
      setRelayFiles(m)
    } catch {} finally { setLoading(false) }
  }

  const handleScanRemote = async () => {
    if (!relayHostId) { message.warning('请先选择中继主机'); return }
    setLoading(true)
    try {
      const res = await fetch('/api/v1/relay/scan-remote-ssh', { method: 'POST', headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token()}` }, body: JSON.stringify({ host_id: relayHostId, path: relayPath }) })
      const json = await res.json()
      const m = new Map<string, {size: string; path: string}>()
      for (const p of (json?.data?.packages || [])) m.set(p.name, { size: p.size || '-', path: p.path || p.name })
      setRemoteFiles(m)
      message.success(`远程扫描完成，${(json?.data?.packages || []).length} 个包`)
    } catch (e: any) { message.error(`扫描失败: ${e?.message}`) } finally { setLoading(false) }
  }

  // 统一"补充"操作：存在则从远程拉取，不存在则从镜像下载
  const handleSupply = async (c: typeof CATALOG[0]) => {
    const remoteInfo = remoteFiles.get(c.fn)
    if (remoteInfo && relayHostId) {
      // 从远程主机拉取到中继服务器
      setDownloading(c.fn); setProgress(prev => ({ ...prev, [c.fn]: '拉取中...' }))
      try {
        const res = await fetch('/api/v1/relay/pull-from-relay', { method: 'POST', headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token()}` }, body: JSON.stringify({ host_id: relayHostId, file_path: remoteInfo.path }) })
        const json = await res.json()
        if (json?.code === 200) { message.success(`${c.fn} 已拉取`); handleScanRelay(); setProgress(prev => ({ ...prev, [c.fn]: '完成' })) }
        else { message.error(`拉取失败: ${json?.message}`); setProgress(prev => ({ ...prev, [c.fn]: '失败' })) }
      } catch (e: any) { message.error(`拉取失败: ${e?.message}`); setProgress(prev => ({ ...prev, [c.fn]: '失败' })) }
      finally { setDownloading(null) }
    } else {
      // 从镜像下载
      setDownloading(c.fn); setProgress(prev => ({ ...prev, [c.fn]: '下载中...' }))
      try {
        const res = await fetch('/api/v1/relay/download-to-relay', { method: 'POST', headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token()}` }, body: JSON.stringify({ url: c.url, filename: c.fn, target_path: `${c.p.toLowerCase()}/${c.v}` }) })
        const json = await res.json()
        if (json?.code === 200) { message.success(`${c.fn} 下载完成`); handleScanRelay(); setProgress(prev => ({ ...prev, [c.fn]: '完成' })) }
        else { message.error(`下载失败: ${json?.message}`); setProgress(prev => ({ ...prev, [c.fn]: '失败' })) }
      } catch (e: any) { message.error(`下载失败: ${e?.message}`); setProgress(prev => ({ ...prev, [c.fn]: '失败' })) }
      finally { setDownloading(null) }
    }
  }

  const handleUpload = async (file: File) => {
    const fd = new FormData(); fd.append('file', file)
    try {
      const res = await fetch('/api/v1/relay/upload', { method: 'POST', headers: { Authorization: `Bearer ${token()}` }, body: fd })
      const json = await res.json()
      if (json?.code === 200) { message.success(`${file.name} 上传成功`); handleScanRelay() } else message.error(`上传失败: ${json?.message}`)
    } catch (e: any) { message.error(`上传失败: ${e?.message}`) }
  }

  const handleDelete = async (fn: string) => {
    try {
      const res = await fetch('/api/v1/relay/delete-package', { method: 'POST', headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token()}` }, body: JSON.stringify({ path: fn }) })
      const json = await res.json()
      if (json?.code === 200) { message.success('已删除'); handleScanRelay() } else message.error(`删除失败: ${json?.message}`)
    } catch (e: any) { message.error(`删除失败: ${e?.message}`) }
  }

  const filtered = CATALOG.filter(c => (osFilter === 'all' || c.os === osFilter) && (productFilter === 'all' || c.p === productFilter))
  const missing = filtered.filter(c => !relayFiles.has(c.fn)).length

  return (
    <div>
      <Card type="inner" size="small" style={{ marginBottom: 12 }}>
        <Row gutter={8} align="middle">
          <Col span={5}><Select size="small" style={{ width: '100%' }} value={relayHostId} onChange={(v) => { setRelayHostId(v); saveCfg() }} placeholder="中继主机" options={hosts.map(h => ({ value: h.id, label: `${h.name} (${h.address})` }))} /></Col>
          <Col span={3}><Input size="small" value={relayPath} onChange={(e) => setRelayPath(e.target.value)} onBlur={saveCfg} placeholder="/opt" addonBefore="路径" /></Col>
          <Col span={2}><Button size="small" icon={<ReloadOutlined />} onClick={handleScanRelay} loading={loading}>刷新</Button></Col>
          <Col span={3}><Button size="small" icon={<DownloadOutlined />} onClick={handleScanRemote} loading={loading} disabled={!relayHostId}>扫描远程</Button></Col>
          <Col span={3}>
            <input type="file" id="pkg-upload" style={{ display: 'none' }} accept=".tar.gz,.tar.xz,.tgz,.tar.bz2,.rpm,.deb,.zip" onChange={(e) => { if (e.target.files?.[0]) handleUpload(e.target.files[0]); e.target.value = '' }} />
            <Button size="small" onClick={() => document.getElementById('pkg-upload')?.click()}>上传包</Button>
          </Col>
          <Col span={8} style={{ textAlign: 'right' }}>
            <Space>
              <Tag>中继: {relayFiles.size}</Tag>
              {missing > 0 ? <Tag color="warning">缺 {missing}</Tag> : <Tag color="success" icon={<CheckCircleOutlined />}>齐全</Tag>}
            </Space>
          </Col>
        </Row>
      </Card>

      <Card type="inner" title="安装包目录" size="small"
        extra={<Space size={8}>
          <Select size="small" value={osFilter} onChange={setOsFilter} style={{ width: 70 }} options={[{ value: 'all', label: 'OS' }, { value: 'Linux', label: 'Linux' }, { value: 'Win', label: 'Win' }]} />
          <Select size="small" value={productFilter} onChange={setProductFilter} style={{ width: 80 }} options={[{ value: 'all', label: '产品' }, { value: 'MySQL', label: 'MySQL' }, { value: 'Percona', label: 'Percona' }, { value: 'MariaDB', label: 'MariaDB' }]} />
        </Space>}>
        <Table size="small" pagination={false} scroll={{ y: 400 }}
          dataSource={filtered.map((c, i) => ({ ...c, key: i, hasIt: relayFiles.has(c.fn), remoteIt: remoteFiles.has(c.fn) }))}
          rowClassName={(r: any) => r.hasIt ? '' : 'ant-table-row-warning'}
          columns={[
            { title: 'OS', dataIndex: 'os', key: 'os', width: 45, render: (v: string) => <Tag style={{ fontSize: 10 }}>{v}</Tag> },
            { title: '产品', dataIndex: 'p', key: 'p', width: 65, render: (v: string) => <Tag color={v === 'MySQL' ? 'blue' : v === 'Percona' ? 'purple' : 'orange'}>{v}</Tag> },
            { title: '版本', dataIndex: 'v', key: 'v', width: 80 },
            { title: 'arch', dataIndex: 'arch', key: 'arch', width: 50 },
            { title: '说明', dataIndex: 'n', key: 'n', width: 80, ellipsis: true },
            { title: '文件名', dataIndex: 'fn', key: 'fn', ellipsis: true },
            { title: '状态', dataIndex: 'hasIt', key: 'st', width: 70,
              render: (v: boolean, r: any) => v
                ? <Tag color="success" style={{ fontSize: 10 }}>已有</Tag>
                : r.remoteIt
                  ? <Tag color="processing" style={{ fontSize: 10 }}>远程有</Tag>
                  : <Tag color="warning" style={{ fontSize: 10 }}>缺失</Tag> },
            { title: '', key: 'act', width: 90, render: (_: any, r: any) => {
              if (r.hasIt) return <Button size="small" danger onClick={() => handleDelete(r.fn)}>删除</Button>
              return <Button size="small" type="primary" loading={downloading === r.fn} disabled={!!downloading}
                onClick={() => handleSupply(r)}>{progress[r.fn] || (r.remoteIt && relayHostId ? '拉取' : '下载')}</Button>
            }},
          ]}
        />
      </Card>

      <Card type="inner" title="Yum/Apt 源配置" size="small" style={{ marginTop: 12 }}
        extra={<Button size="small" onClick={() => setShowRepo(!showRepo)}>{showRepo ? '收起' : '展开'}</Button>}>
        {showRepo && (
          <Table size="small" pagination={false}
            dataSource={REPO_SOURCES.map((r, i) => ({ ...r, key: i }))}
            columns={[
              { title: '标签', dataIndex: 'label', key: 'label', width: 200 },
              { title: '类型', dataIndex: 'type', key: 'type', width: 50 },
              { title: '源内容', dataIndex: 'content', key: 'content', ellipsis: true, render: (v: string) => <code style={{ fontSize: 11 }}>{v}</code> },
            ]}
          />
        )}
        {!showRepo && <Text type="secondary">{REPO_SOURCES.length} 个预置源</Text>}
      </Card>
    </div>
  )
}

// ─── 主组件 ───────────────────────────────────────────────────────────────

const SecuritySettings: React.FC = () => {
  return (
    <div style={{ padding: 24 }}>
      <Card title={<Space><SettingOutlined /><span>系统设置</span></Space>}>
        <Tabs defaultActiveKey="packages" items={[
          { key: 'packages', label: <Space><CloudOutlined />安装包管理</Space>, children: <PackageTab /> },
          { key: 'security', label: <Space><LockOutlined />安全设置</Space>, children: <SecurityTab /> },
          { key: 'metrics', label: <Space><DatabaseOutlined />监控指标</Space>, children: <MetricsTab /> },
          { key: 'params', label: <Space><ToolOutlined />平台参数</Space>, children: <PlatformTab /> },
        ]} />
      </Card>
    </div>
  )
}

export default SecuritySettings
