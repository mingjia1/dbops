import React, { useCallback, useEffect, useState } from 'react'
import {
  Button, Card, Form, Input, InputNumber, Modal, Popconfirm, Select,
  Space, Table, Tabs, Tag, Typography,
} from 'antd'
import {
  ApiOutlined, AppstoreOutlined, CloudServerOutlined, DeleteOutlined,
  DeploymentUnitOutlined, PlusOutlined, QuestionCircleOutlined, ThunderboltOutlined,
} from '@ant-design/icons'
import { message } from '../components/antdMessage'
import { pluginApi, type PluginInfo, type PluginRegisterRequest } from '../services/api'

const typeColors: Record<string, string> = {
  kernel: 'blue',
  arch: 'green',
  middleware: 'orange',
}

const typeLabels: Record<string, string> = {
  kernel: '内核插件',
  arch: '架构插件',
  middleware: '中间件插件',
}

const typeIcons: Record<string, React.ReactNode> = {
  kernel: <CloudServerOutlined />,
  arch: <DeploymentUnitOutlined />,
  middleware: <ApiOutlined />,
}

const pluginDescriptions: Record<string, string> = {
  'mysql-core': 'MySQL 单机部署、配置渲染、Systemd 托管与实例启停',
  'percona-core': 'Percona Server 单机部署（基于 MySQL Core）',
  'mariadb-core': 'MariaDB 单机部署，自动适配 gtid_domain_id',
  'replica-addon': '主从异步复制搭建',
  'mha-addon': 'MHA 高可用集群搭建（Manager + Primary + Replicas）',
  'mgr-addon': 'MySQL Group Replication 集群搭建',
  'pxc-galera-addon': 'Percona XtraDB Cluster / Galera 集群搭建',
  'keepalived-addon': 'Keepalived VIP 漂移中间件部署',
  'proxysql-addon': 'ProxySQL 读写分离负载均衡部署',
}

// 接入说明: 平台把 Plugin 接口映射为 POST <endpoint>/<action>.
const PLUGIN_PROTOCOL = `请求
  POST <endpoint>/<action>   Content-Type: application/json
  action = ping | prepare | execute | rollback | teardown | join | leave

请求体
  {
    "action": "execute",
    "plugin": "demo-addon",
    "env": { "cluster_id": "c-1", "nodes": [], "credentials": {}, "custom": {} },
    "params": { "flavor": "mysql" },
    "node": null            // join / leave 时为变更的节点
  }

响应
  HTTP 2xx 且空 body            → 成功
  {"success": true, "message": "ok", "data": {}, "warnings": []}
  {"success": false, "message": "disk full"}  → 失败
  execute 失败时平台会自动调用 rollback；非 2xx 一律视为失败。

鉴权
  配置了 Token 时平台会带 Authorization: Bearer <token>，请在插件侧校验。
  Endpoint 必须是 http/https 绝对地址，且不带查询串或片段。`

const PLUGIN_EXAMPLE = `from http.server import BaseHTTPRequestHandler, HTTPServer
import json

TOKEN = "change-me"

class Handler(BaseHTTPRequestHandler):
    def do_POST(self):
        action = self.path.rstrip("/").split("/")[-1]
        length = int(self.headers.get("Content-Length", 0))
        body = json.loads(self.rfile.read(length) or b"{}")
        if self.headers.get("Authorization") != "Bearer " + TOKEN:
            self.send_response(401)
            self.end_headers()
            return
        result = {"success": True, "message": "ok",
                  "data": {"action": action, "params": body.get("params", {})}}
        payload = json.dumps(result).encode()
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(payload)))
        self.end_headers()
        self.wfile.write(payload)

HTTPServer(("0.0.0.0", 9100), Handler).serve_forever()`

const codeBlockStyle: React.CSSProperties = {
  background: 'var(--apple-bg-grouped)',
  color: 'var(--apple-text-primary)',
  border: '0.5px solid var(--apple-separator)',
  borderRadius: 8,
  padding: 12,
  fontSize: 12,
  lineHeight: 1.6,
  overflow: 'auto',
  whiteSpace: 'pre',
  margin: 0,
}

const PluginManage: React.FC = () => {
  const [form] = Form.useForm()
  const [plugins, setPlugins] = useState<PluginInfo[]>([])
  const [loading, setLoading] = useState(true)
  const [registerOpen, setRegisterOpen] = useState(false)
  const [guideOpen, setGuideOpen] = useState(false)
  const [submitting, setSubmitting] = useState(false)
  const [probing, setProbing] = useState(false)

  const fetchPlugins = useCallback(async () => {
    try {
      setLoading(true)
      const res: any = await pluginApi.list()
      setPlugins(res?.data || [])
    } catch {
      // ignore - plugin list failure is non-critical
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => { fetchPlugins() }, [fetchPlugins])

  const handleUnregister = async (name: string) => {
    try {
      await pluginApi.unregister(name)
      message.success(`已取消注册 ${name}`)
      await fetchPlugins()
    } catch {
      // 全局拦截器已提示
    }
  }

  const handleProbe = async () => {
    const endpoint = String(form.getFieldValue('endpoint') || '').trim().replace(/\/+$/, '')
    const token = String(form.getFieldValue('token') || '').trim()
    if (!endpoint) {
      message.warning('请先填写接入地址')
      return
    }
    setProbing(true)
    try {
      await pluginApi.probe({ endpoint, token: token || undefined, timeout_ms: 5000 })
      message.success('连接测试成功')
    } catch {
      // 全局拦截器已提示
    } finally {
      setProbing(false)
    }
  }

  const handleRegister = async () => {
    let values: any
    try {
      values = await form.validateFields()
    } catch {
      return
    }
    const payload: PluginRegisterRequest = {
      name: String(values.name).trim(),
      type: values.type,
      version: String(values.version || '').trim() || '1.0.0',
      endpoint: String(values.endpoint).trim().replace(/\/+$/, ''),
      description: String(values.description || '').trim() || undefined,
      token: String(values.token || '').trim() || undefined,
      timeout_ms: values.timeout_ms || undefined,
    }
    setSubmitting(true)
    try {
      await pluginApi.register(payload)
      message.success('插件接入成功')
      setRegisterOpen(false)
      form.resetFields()
      await fetchPlugins()
    } catch {
      // 全局拦截器已提示
    } finally {
      setSubmitting(false)
    }
  }

  const columns = [
    {
      title: '插件名称',
      dataIndex: 'name',
      key: 'name',
      render: (name: string) => <Typography.Text strong>{name}</Typography.Text>,
    },
    {
      title: '类型',
      dataIndex: 'type',
      key: 'type',
      render: (type: string) => (
        <Tag color={typeColors[type] || 'default'} icon={typeIcons[type]}>
          {typeLabels[type] || type}
        </Tag>
      ),
    },
    {
      title: '版本',
      dataIndex: 'version',
      key: 'version',
      render: (ver: string) => <Tag>{ver}</Tag>,
    },
    {
      title: '说明',
      key: 'description',
      render: (_: any, record: PluginInfo) => (
        <Space direction="vertical" size={0}>
          <Typography.Text type="secondary">
            {pluginDescriptions[record.name] || record.description || '-'}
          </Typography.Text>
          {record.endpoint && (
            <Typography.Text type="secondary" style={{ fontSize: 12 }} copyable={{ text: record.endpoint }}>
              {record.endpoint}
            </Typography.Text>
          )}
        </Space>
      ),
    },
    {
      title: '状态',
      key: 'status',
      render: (_: any, record: PluginInfo) => (
        record.source === 'dynamic'
          ? <Tag color="processing">动态接入</Tag>
          : <Tag color="success">内置已注册</Tag>
      ),
    },
    {
      title: '操作',
      key: 'action',
      render: (_: any, record: PluginInfo) => (
        record.source === 'dynamic' ? (
          <Popconfirm
            title="取消注册该插件？"
            description="取消后部署链路将不再能调用它。"
            okText="取消注册"
            okButtonProps={{ danger: true }}
            cancelText="返回"
            onConfirm={() => handleUnregister(record.name)}
          >
            <Button type="link" danger size="small" icon={<DeleteOutlined />}>取消注册</Button>
          </Popconfirm>
        ) : <Typography.Text type="secondary">-</Typography.Text>
      ),
    },
  ]

  // 系统默认分页策略: 每页 10 条.
  const renderTable = (data: PluginInfo[]) => (
    <Table
      columns={columns}
      dataSource={data}
      rowKey="name"
      loading={loading}
      pagination={{ pageSize: 10 }}
      size="middle"
    />
  )

  const filterByType = (type: string) => plugins.filter((p) => p.type === type)
  const tabLabel = (icon: React.ReactNode, text: string, count: number) => (
    <span>{icon} {text} ({count})</span>
  )

  const tabItems = [
    { key: 'all', label: tabLabel(<AppstoreOutlined />, '全部', plugins.length), children: renderTable(plugins) },
    { key: 'kernel', label: tabLabel(<CloudServerOutlined />, '内核插件', filterByType('kernel').length), children: renderTable(filterByType('kernel')) },
    { key: 'arch', label: tabLabel(<DeploymentUnitOutlined />, '架构插件', filterByType('arch').length), children: renderTable(filterByType('arch')) },
    { key: 'middleware', label: tabLabel(<ApiOutlined />, '中间件插件', filterByType('middleware').length), children: renderTable(filterByType('middleware')) },
  ]

  return (
    <Card
      title="插件管理"
      style={{ margin: 16 }}
      extra={
        <Space>
          <Button icon={<QuestionCircleOutlined />} onClick={() => setGuideOpen(true)}>接入说明</Button>
          <Button type="primary" icon={<PlusOutlined />} onClick={() => setRegisterOpen(true)}>接入插件</Button>
        </Space>
      }
    >
      <Typography.Paragraph type="secondary" style={{ marginBottom: 16 }}>
        内置插件随平台编译，负责单机部署、集群装配与中间件能力；动态插件通过 HTTP 协议在运行时接入，无需重新编译平台，可随时取消注册。
      </Typography.Paragraph>
      <Tabs items={tabItems} />

      <Modal
        title="接入动态插件"
        open={registerOpen}
        onCancel={() => setRegisterOpen(false)}
        width={640}
        destroyOnClose
        footer={[
          <Button key="cancel" onClick={() => setRegisterOpen(false)}>取消</Button>,
          <Button key="probe" icon={<ThunderboltOutlined />} loading={probing} onClick={handleProbe}>测试连接</Button>,
          <Button key="submit" type="primary" loading={submitting} onClick={handleRegister}>接入</Button>,
        ]}
      >
        <Form
          form={form}
          layout="vertical"
          initialValues={{ type: 'middleware', version: '1.0.0', timeout_ms: 30000 }}
          style={{ marginTop: 16 }}
        >
          <Form.Item
            name="name"
            label="插件名称"
            extra="小写字母/数字开头，仅允许 a-z 0-9 . _ -，不可与内置插件重名"
            rules={[{ required: true, message: '请输入插件名称' }, { pattern: /^[a-z0-9][a-z0-9._-]{1,63}$/, message: '名称格式不合法' }]}
          >
            <Input placeholder="demo-addon" />
          </Form.Item>
          <Space size="middle" style={{ display: 'flex' }}>
            <Form.Item name="type" label="类型" rules={[{ required: true }]} style={{ flex: 1 }}>
              <Select options={[
                { value: 'arch', label: '架构插件' },
                { value: 'middleware', label: '中间件插件' },
                { value: 'kernel', label: '内核插件' },
                { value: 'custom', label: '自定义' },
              ]} />
            </Form.Item>
            <Form.Item name="version" label="版本" style={{ flex: 1 }}>
              <Input placeholder="1.0.0" />
            </Form.Item>
          </Space>
          <Form.Item
            name="endpoint"
            label="接入地址 (Endpoint)"
            extra="平台会向 <endpoint>/<action> 发起 POST，详见「接入说明」"
            rules={[{ required: true, message: '请输入接入地址' }]}
          >
            <Input placeholder="http://127.0.0.1:9100/hook" />
          </Form.Item>
          <Form.Item name="description" label="说明">
            <Input placeholder="该插件负责什么" />
          </Form.Item>
          <Space size="middle" style={{ display: 'flex' }}>
            <Form.Item name="token" label="Token（可选）" style={{ flex: 1 }} extra="作为 Bearer Token 发送">
              <Input.Password placeholder="留空则不鉴权" />
            </Form.Item>
            <Form.Item name="timeout_ms" label="超时（毫秒）" style={{ flex: 1 }}>
              <InputNumber min={1000} max={600000} step={1000} style={{ width: '100%' }} />
            </Form.Item>
          </Space>
        </Form>
      </Modal>

      <Modal
        title="动态插件接入说明"
        open={guideOpen}
        onCancel={() => setGuideOpen(false)}
        width={760}
        footer={[<Button key="close" onClick={() => setGuideOpen(false)}>关闭</Button>]}
      >
        <Typography.Paragraph>
          平台内置插件由 Go 代码在启动时注册；动态插件通过 <Typography.Text code>HTTP Webhook</Typography.Text> 接入，外部服务可以用任意语言实现，无需重新编译或重启平台。注册后会持久化到 <Typography.Text code>plugin_registry</Typography.Text> 表，服务重启自动恢复；只有动态插件可以取消注册，内置插件受保护。
        </Typography.Paragraph>
        <Typography.Title level={5}>协议约定</Typography.Title>
        <Typography.Paragraph copyable={{ text: PLUGIN_PROTOCOL }}>
          <pre style={codeBlockStyle}>{PLUGIN_PROTOCOL}</pre>
        </Typography.Paragraph>
        <Typography.Title level={5}>最小可运行示例（Python 标准库）</Typography.Title>
        <Typography.Paragraph type="secondary">
          保存为 plugin.py 后运行 <Typography.Text code>python plugin.py</Typography.Text>，在接入弹窗里填写 <Typography.Text code>http://&lt;host&gt;:9100/hook</Typography.Text> 与对应 Token，点「测试连接」验证。
        </Typography.Paragraph>
        <Typography.Paragraph copyable={{ text: PLUGIN_EXAMPLE }}>
          <pre style={codeBlockStyle}>{PLUGIN_EXAMPLE}</pre>
        </Typography.Paragraph>
      </Modal>
    </Card>
  )
}

export default PluginManage
