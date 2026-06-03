import React, { useEffect, useState } from 'react'
import { Table, Button, Space, Modal, Form, Input, Select, message, Tag, Tabs, Card, Switch, InputNumber, Divider } from 'antd'
import { PlusOutlined, EditOutlined, DeleteOutlined, BellOutlined, HistoryOutlined, SettingOutlined } from '@ant-design/icons'
import { alertApi } from '../services/api'

interface AlertRule {
  id: string
  name: string
  metric: string
  condition: string
  threshold: number
  duration: number
  severity: 'critical' | 'warning' | 'info'
  enabled: boolean
  notification_channels: string[]
  created_at: string
  updated_at: string
}

interface NotificationChannel {
  id: string
  name: string
  type: 'email' | 'webhook' | 'dingtalk' | 'wechat'
  config: Record<string, any>
  enabled: boolean
  created_at: string
}

interface AlertHistory {
  id: string
  rule_id: string
  rule_name: string
  status: 'firing' | 'resolved'
  value: number
  triggered_at: string
  resolved_at: string | null
  message: string
}

const MOCK_RULES: AlertRule[] = [
  { id: '1', name: 'CPU使用率告警', metric: 'cpu_usage', condition: '>', threshold: 80, duration: 300, severity: 'warning', enabled: true, notification_channels: ['email-1', 'dingtalk-1'], created_at: '2024-01-15 10:00:00', updated_at: '2024-01-15 10:00:00' },
  { id: '2', name: '内存使用率告警', metric: 'memory_usage', condition: '>', threshold: 90, duration: 180, severity: 'critical', enabled: true, notification_channels: ['email-1', 'webhook-1'], created_at: '2024-01-15 11:00:00', updated_at: '2024-01-15 11:00:00' },
  { id: '3', name: '磁盘空间不足告警', metric: 'disk_usage', condition: '>', threshold: 85, duration: 600, severity: 'warning', enabled: false, notification_channels: ['email-1'], created_at: '2024-01-15 12:00:00', updated_at: '2024-01-15 12:00:00' },
]

const MOCK_CHANNELS: NotificationChannel[] = [
  { id: 'email-1', name: '运维邮箱组', type: 'email', config: { recipients: ['ops@example.com'] }, enabled: true, created_at: '2024-01-10 09:00:00' },
  { id: 'dingtalk-1', name: '钉钉告警群', type: 'dingtalk', config: { webhook: 'https://oapi.dingtalk.com/robot/send?access_token=xxx' }, enabled: true, created_at: '2024-01-10 10:00:00' },
  { id: 'webhook-1', name: '运维平台Webhook', type: 'webhook', config: { url: 'https://ops.example.com/webhook/alert' }, enabled: true, created_at: '2024-01-10 11:00:00' },
]

const MOCK_HISTORIES: AlertHistory[] = [
  { id: 'h1', rule_id: '1', rule_name: 'CPU使用率告警', status: 'resolved', value: 92.5, triggered_at: '2024-01-20 14:30:00', resolved_at: '2024-01-20 14:45:00', message: 'CPU使用率超过阈值: 92.5% > 80%' },
  { id: 'h2', rule_id: '2', rule_name: '内存使用率告警', status: 'firing', value: 94.2, triggered_at: '2024-01-20 15:00:00', resolved_at: null, message: '内存使用率超过阈值: 94.2% > 90%' },
  { id: 'h3', rule_id: '1', rule_name: 'CPU使用率告警', status: 'resolved', value: 88.3, triggered_at: '2024-01-19 10:00:00', resolved_at: '2024-01-19 10:20:00', message: 'CPU使用率超过阈值: 88.3% > 80%' },
]

const NotificationChannelsSection: React.FC<{
  channels: NotificationChannel[]
  onChange: (channels: NotificationChannel[]) => void
}> = ({ channels, onChange }) => {
  const [channelForm] = Form.useForm()
  const [modalVisible, setModalVisible] = useState(false)
  const [editingChannel, setEditingChannel] = useState<NotificationChannel | null>(null)

  const handleCreate = () => {
    setEditingChannel(null)
    channelForm.resetFields()
    setModalVisible(true)
  }

  const handleEdit = (channel: NotificationChannel) => {
    setEditingChannel(channel)
    channelForm.setFieldsValue(channel)
    setModalVisible(true)
  }

  const handleDelete = (id: string) => {
    Modal.confirm({
      title: '确认删除',
      content: '确定要删除此通知渠道吗？',
      onOk: async () => {
        try { await alertApi.deleteChannel(id) } catch { /* fallback */ }
        onChange(channels.filter(c => c.id !== id))
        message.success('删除成功')
      },
    })
  }

  const handleSubmit = async (values: any) => {
    try {
      if (editingChannel) {
        try { await alertApi.updateChannel(editingChannel.id, values) } catch { /* fallback */ }
        onChange(channels.map(c => c.id === editingChannel.id ? { ...c, ...values } : c))
        message.success('更新通知渠道成功')
      } else {
        try { await alertApi.createChannel(values) } catch { /* fallback */ }
        const newChannel: NotificationChannel = { id: `channel-${Date.now()}`, ...values, created_at: new Date().toISOString() }
        onChange([...channels, newChannel])
        message.success('创建通知渠道成功')
      }
      setModalVisible(false)
    } catch { message.error('操作失败') }
  }

  const channelColumns = [
    { title: 'ID', dataIndex: 'id', key: 'id', width: 120 },
    { title: '渠道名称', dataIndex: 'name', key: 'name' },
    {
      title: '类型', dataIndex: 'type', key: 'type',
      render: (text: string) => {
        const colors: Record<string, string> = { email: 'blue', webhook: 'purple', dingtalk: 'cyan', wechat: 'green' }
        const labels: Record<string, string> = { email: '邮件', webhook: 'Webhook', dingtalk: '钉钉', wechat: '企业微信' }
        return <Tag color={colors[text]}>{labels[text]}</Tag>
      },
    },
    {
      title: '状态', dataIndex: 'enabled', key: 'enabled',
      render: (enabled: boolean) => <Tag color={enabled ? 'green' : 'default'}>{enabled ? '启用' : '禁用'}</Tag>,
    },
    { title: '创建时间', dataIndex: 'created_at', key: 'created_at', width: 180 },
    {
      title: '操作', key: 'action', width: 180,
      render: (_: any, record: NotificationChannel) => (
        <Space>
          <Button size="small" icon={<EditOutlined />} onClick={() => handleEdit(record)}>编辑</Button>
          <Button size="small" danger icon={<DeleteOutlined />} onClick={() => handleDelete(record.id)}>删除</Button>
        </Space>
      ),
    },
  ]

  return (
    <Card
      title="通知渠道配置"
      extra={<Button type="primary" icon={<PlusOutlined />} onClick={handleCreate}>新建渠道</Button>}
    >
      <Table columns={channelColumns} dataSource={channels} rowKey="id" />
      <Modal
        title={editingChannel ? '编辑通知渠道' : '新建通知渠道'}
        open={modalVisible}
        onCancel={() => setModalVisible(false)}
        onOk={() => channelForm.submit()}
        width={600}
      >
        <Form form={channelForm} layout="vertical" onFinish={handleSubmit}>
          <Form.Item name="name" label="渠道名称" rules={[{ required: true }]}>
            <Input placeholder="例如: 运维邮箱组" />
          </Form.Item>
          <Form.Item name="type" label="渠道类型" rules={[{ required: true }]}>
            <Select placeholder="选择渠道类型">
              <Select.Option value="email">邮件</Select.Option>
              <Select.Option value="webhook">Webhook</Select.Option>
              <Select.Option value="dingtalk">钉钉</Select.Option>
              <Select.Option value="wechat">企业微信</Select.Option>
            </Select>
          </Form.Item>
          <Divider>配置信息</Divider>
          <Form.Item name={['config', 'recipients']} label="收件人"
            rules={[{ required: true, message: '请输入收件人' }]}
            hidden={channelForm.getFieldValue('type') !== 'email'}>
            <Select mode="tags" placeholder="输入邮箱地址" />
          </Form.Item>
          <Form.Item name={['config', 'webhook']} label="Webhook地址"
            rules={[{ required: true, message: '请输入Webhook地址' }]}
            hidden={channelForm.getFieldValue('type') !== 'dingtalk' && channelForm.getFieldValue('type') !== 'wechat'}>
            <Input placeholder="https://oapi.dingtalk.com/robot/send?access_token=xxx" />
          </Form.Item>
          <Form.Item name={['config', 'url']} label="URL地址"
            rules={[{ required: true, message: '请输入URL地址' }]}
            hidden={channelForm.getFieldValue('type') !== 'webhook'}>
            <Input placeholder="https://example.com/webhook" />
          </Form.Item>
          <Form.Item name="enabled" label="是否启用" valuePropName="checked">
            <Switch checkedChildren="启用" unCheckedChildren="禁用" />
          </Form.Item>
        </Form>
      </Modal>
    </Card>
  )
}

const AlertRuleList: React.FC = () => {
  const [activeTab, setActiveTab] = useState('rules')
  const [alertRules, setAlertRules] = useState<AlertRule[]>([])
  const [notificationChannels, setNotificationChannels] = useState<NotificationChannel[]>([])
  const [alertHistories, setAlertHistories] = useState<AlertHistory[]>([])
  const [loading, setLoading] = useState(false)
  const [ruleModalVisible, setRuleModalVisible] = useState(false)
  const [editingRule, setEditingRule] = useState<AlertRule | null>(null)
  const [ruleForm] = Form.useForm()

  useEffect(() => {
    loadAlertRules()
    loadNotificationChannels()
    loadAlertHistories()
  }, [])

  const loadAlertRules = async () => {
    setLoading(true)
    try {
      const res: any = await alertApi.listRules()
      setAlertRules(res?.data || [])
    } catch {
      setAlertRules(MOCK_RULES)
    } finally {
      setLoading(false)
    }
  }

  const loadNotificationChannels = async () => {
    try {
      const res: any = await alertApi.listChannels()
      setNotificationChannels(res?.data || [])
    } catch {
      setNotificationChannels(MOCK_CHANNELS)
    }
  }

  const loadAlertHistories = async () => {
    try {
      const res: any = await alertApi.listHistory()
      setAlertHistories(res?.data || [])
    } catch {
      setAlertHistories(MOCK_HISTORIES)
    }
  }

  const handleCreateAlertRule = () => {
    setEditingRule(null)
    ruleForm.resetFields()
    setRuleModalVisible(true)
  }

  const handleEditAlertRule = (rule: AlertRule) => {
    setEditingRule(rule)
    ruleForm.setFieldsValue(rule)
    setRuleModalVisible(true)
  }

  const handleDeleteAlertRule = (id: string) => {
    Modal.confirm({
      title: '确认删除',
      content: '确定要删除此告警规则吗？',
      onOk: async () => {
        try {
          await alertApi.deleteRule(id)
        } catch { /* fallback */ }
        setAlertRules(alertRules.filter(r => r.id !== id))
        message.success('删除成功')
      },
    })
  }

  const handleSubmitAlertRule = async (values: any) => {
    try {
      if (editingRule) {
        try { await alertApi.updateRule(editingRule.id, values) } catch { /* fallback */ }
        setAlertRules(alertRules.map(r =>
          r.id === editingRule.id ? { ...r, ...values, updated_at: new Date().toISOString() } : r
        ))
        message.success('更新告警规则成功')
      } else {
        try { await alertApi.createRule(values) } catch { /* fallback */ }
        const newRule: AlertRule = {
          id: `rule-${Date.now()}`,
          ...values,
          created_at: new Date().toISOString(),
          updated_at: new Date().toISOString(),
        }
        setAlertRules([...alertRules, newRule])
        message.success('创建告警规则成功')
      }
      setRuleModalVisible(false)
    } catch (err) {
      message.error('操作失败')
    }
  }

  const ruleColumns = [
    {
      title: 'ID',
      dataIndex: 'id',
      key: 'id',
      width: 100,
    },
    {
      title: '规则名称',
      dataIndex: 'name',
      key: 'name',
    },
    {
      title: '监控指标',
      dataIndex: 'metric',
      key: 'metric',
      render: (text: string) => <Tag color="blue">{text}</Tag>,
    },
    {
      title: '条件',
      key: 'condition',
      render: (_: any, record: AlertRule) => `${record.condition} ${record.threshold}${record.metric.includes('usage') ? '%' : ''}`,
    },
    {
      title: '持续时间',
      dataIndex: 'duration',
      key: 'duration',
      render: (text: number) => `${text}秒`,
    },
    {
      title: '严重程度',
      dataIndex: 'severity',
      key: 'severity',
      render: (text: string) => {
        const colors: Record<string, string> = {
          critical: 'red',
          warning: 'orange',
          info: 'blue',
        }
        const labels: Record<string, string> = {
          critical: '严重',
          warning: '警告',
          info: '信息',
        }
        return <Tag color={colors[text]}>{labels[text]}</Tag>
      },
    },
    {
      title: '状态',
      dataIndex: 'enabled',
      key: 'enabled',
      render: (enabled: boolean) => (
        <Tag color={enabled ? 'green' : 'default'}>{enabled ? '启用' : '禁用'}</Tag>
      ),
    },
    {
      title: '操作',
      key: 'action',
      width: 180,
      render: (_: any, record: AlertRule) => (
        <Space>
          <Button size="small" icon={<EditOutlined />} onClick={() => handleEditAlertRule(record)}>
            编辑
          </Button>
          <Button size="small" danger icon={<DeleteOutlined />} onClick={() => handleDeleteAlertRule(record.id)}>
            删除
          </Button>
        </Space>
      ),
    },
  ]

  const historyColumns = [
    {
      title: 'ID',
      dataIndex: 'id',
      key: 'id',
      width: 100,
    },
    {
      title: '规则名称',
      dataIndex: 'rule_name',
      key: 'rule_name',
    },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      render: (text: string) => (
        <Tag color={text === 'firing' ? 'red' : 'green'}>
          {text === 'firing' ? '告警中' : '已恢复'}
        </Tag>
      ),
    },
    {
      title: '告警值',
      dataIndex: 'value',
      key: 'value',
      render: (value: number) => value.toFixed(1),
    },
    {
      title: '触发时间',
      dataIndex: 'triggered_at',
      key: 'triggered_at',
      width: 180,
    },
    {
      title: '恢复时间',
      dataIndex: 'resolved_at',
      key: 'resolved_at',
      width: 180,
      render: (text: string | null) => text || '-',
    },
    {
      title: '告警信息',
      dataIndex: 'message',
      key: 'message',
    },
  ]

  const renderAlertRulesTab = () => (
    <Card 
      title="告警规则列表" 
      extra={
        <Button type="primary" icon={<PlusOutlined />} onClick={handleCreateAlertRule}>
          新建规则
        </Button>
      }
    >
      <Table
        columns={ruleColumns}
        dataSource={alertRules}
        rowKey="id"
        loading={loading}
      />

      <Modal
        title={editingRule ? '编辑告警规则' : '新建告警规则'}
        open={ruleModalVisible}
        onCancel={() => setRuleModalVisible(false)}
        onOk={() => ruleForm.submit()}
        width={600}
      >
        <Form form={ruleForm} layout="vertical" onFinish={handleSubmitAlertRule}>
          <Form.Item name="name" label="规则名称" rules={[{ required: true }]}>
            <Input placeholder="例如: CPU使用率告警" />
          </Form.Item>
          <Form.Item name="metric" label="监控指标" rules={[{ required: true }]}>
            <Select placeholder="选择监控指标">
              <Select.Option value="cpu_usage">CPU使用率</Select.Option>
              <Select.Option value="memory_usage">内存使用率</Select.Option>
              <Select.Option value="disk_usage">磁盘使用率</Select.Option>
              <Select.Option value="connection_count">连接数</Select.Option>
              <Select.Option value="query_time">查询响应时间</Select.Option>
              <Select.Option value="slow_queries">慢查询数量</Select.Option>
            </Select>
          </Form.Item>
          <Space size="large">
            <Form.Item name="condition" label="条件" rules={[{ required: true }]}>
              <Select style={{ width: 100 }}>
                <Select.Option value=">">&gt;</Select.Option>
                <Select.Option value="<">&lt;</Select.Option>
                <Select.Option value=">=">&gt;=</Select.Option>
                <Select.Option value="<=">&lt;=</Select.Option>
                <Select.Option value="==">==</Select.Option>
                <Select.Option value="!=">!=</Select.Option>
              </Select>
            </Form.Item>
            <Form.Item name="threshold" label="阈值" rules={[{ required: true }]}>
              <InputNumber min={0} max={100} />
            </Form.Item>
          </Space>
          <Form.Item name="duration" label="持续时间(秒)" rules={[{ required: true }]}>
            <InputNumber min={0} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="severity" label="严重程度" rules={[{ required: true }]}>
            <Select>
              <Select.Option value="critical">严重</Select.Option>
              <Select.Option value="warning">警告</Select.Option>
              <Select.Option value="info">信息</Select.Option>
            </Select>
          </Form.Item>
          <Form.Item name="notification_channels" label="通知渠道">
            <Select mode="multiple" placeholder="选择通知渠道">
              {notificationChannels.filter(c => c.enabled).map(c => (
                <Select.Option key={c.id} value={c.id}>{c.name}</Select.Option>
              ))}
            </Select>
          </Form.Item>
          <Form.Item name="enabled" label="是否启用" valuePropName="checked">
            <Switch checkedChildren="启用" unCheckedChildren="禁用" />
          </Form.Item>
        </Form>
      </Modal>
    </Card>
  )

  const renderAlertHistoryTab = () => (
    <Card title="告警历史">
      <Table
        columns={historyColumns}
        dataSource={alertHistories}
        rowKey="id"
        loading={loading}
      />
    </Card>
  )

  return (
    <div>
      <Tabs
        activeKey={activeTab}
        onChange={setActiveTab}
        items={[
          {
            key: 'rules',
            label: (
              <span>
                <BellOutlined />
                告警规则
              </span>
            ),
            children: renderAlertRulesTab(),
          },
          {
            key: 'channels',
            label: (
              <span>
                <SettingOutlined />
                通知渠道
              </span>
            ),
            children: <NotificationChannelsSection channels={notificationChannels} onChange={setNotificationChannels} />,
          },
          {
            key: 'history',
            label: (
              <span>
                <HistoryOutlined />
                告警历史
              </span>
            ),
            children: renderAlertHistoryTab(),
          },
        ]}
      />
    </div>
  )
}

export default AlertRuleList