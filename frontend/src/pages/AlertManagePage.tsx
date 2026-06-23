import React, { useState, useEffect } from 'react'
import { Card, Table, Button, Modal, Form, Input, Select, Space, Tag, message, Popconfirm, Descriptions, Tabs } from 'antd'
import { NotificationOutlined, PlusOutlined, EditOutlined, DeleteOutlined, PauseCircleOutlined, HistoryOutlined } from '@ant-design/icons'
import api from '../services/api'

interface AlertTemplate { id: string; name: string; category: string; metric: string; severity: string; message: string }
interface EscalationRule { id: string; rule_id: string; level: number; severity: string; interval_sec: number; notify_channels: string }
interface Silence { id: string; name: string; rule_ids: string; start_at: string; end_at: string; enabled: boolean }

export default function AlertManagePage() {
  const [templates, setTemplates] = useState<AlertTemplate[]>([])
  const [escalations, setEscalations] = useState<EscalationRule[]>([])
  const [silences, setSilences] = useState<Silence[]>([])
  const [loading, setLoading] = useState(false)
  const [tplOpen, setTplOpen] = useState(false)
  const [escOpen, setEscOpen] = useState(false)
  const [silOpen, setSilOpen] = useState(false)
  const [form] = Form.useForm()

  const fetchAll = async () => {
    setLoading(true)
    try {
      const [t, e, s] = await Promise.all([api.get('/alerts/templates'), api.get('/alerts/escalations'), api.get('/alerts/silences')])
      setTemplates(t.data?.data || []); setEscalations(e.data?.data || []); setSilences(s.data?.data || [])
    } catch {} finally { setLoading(false) }
  }
  useEffect(() => { fetchAll() }, [])

  const handleSaveTemplate = async (v: any) => {
    try { await api.post('/alerts/templates', v); message.success('创建成功'); setTplOpen(false); form.resetFields(); fetchAll() }
    catch (err: any) { message.error(err.message) }
  }
  const handleSaveEscalation = async (v: any) => {
    try { await api.post('/alerts/escalations', v); message.success('创建成功'); setEscOpen(false); form.resetFields(); fetchAll() }
    catch (err: any) { message.error(err.message) }
  }
  const handleSaveSilence = async (v: any) => {
    try { await api.post('/alerts/silences', v); message.success('创建成功'); setSilOpen(false); form.resetFields(); fetchAll() }
    catch (err: any) { message.error(err.message) }
  }
  const handleEvaluate = async () => {
    try { await api.post('/alerts/evaluate'); message.success('告警评估已触发'); fetchAll() }
    catch (err: any) { message.error(err.message) }
  }

  const tplCols = [
    { title: '名称', dataIndex: 'name', key: 'name' },
    { title: '类别', dataIndex: 'category', key: 'category', render: (v: string) => <Tag>{v}</Tag> },
    { title: '指标', dataIndex: 'metric', key: 'metric' },
    { title: '严重性', dataIndex: 'severity', key: 'severity', render: (v: string) => <Tag color={v === 'critical' ? 'red' : v === 'warning' ? 'orange' : 'blue'}>{v}</Tag> },
    { title: '消息', dataIndex: 'message', key: 'message', ellipsis: true },
    { title: '操作', key: 'action', render: (_: any, r: AlertTemplate) => (
      <Popconfirm title="确认删除？" onConfirm={async () => { await api.delete(`/alerts/templates/${r.id}`); fetchAll() }}>
        <Button size="small" danger icon={<DeleteOutlined />}>删除</Button>
      </Popconfirm>
    )},
  ]
  const escCols = [
    { title: '级别', dataIndex: 'level', key: 'level', render: (v: number) => <Tag color="red">L{v}</Tag> },
    { title: '严重性', dataIndex: 'severity', key: 'severity' },
    { title: '间隔(秒)', dataIndex: 'interval_sec', key: 'interval_sec' },
    { title: '通知渠道', dataIndex: 'notify_channels', key: 'notify_channels', ellipsis: true },
  ]
  const silCols = [
    { title: '名称', dataIndex: 'name', key: 'name' },
    { title: '规则IDs', dataIndex: 'rule_ids', key: 'rule_ids', ellipsis: true },
    { title: '开始', dataIndex: 'start_at', key: 'start_at', render: (v: string) => new Date(v).toLocaleString() },
    { title: '结束', dataIndex: 'end_at', key: 'end_at', render: (v: string) => new Date(v).toLocaleString() },
    { title: '状态', dataIndex: 'enabled', key: 'enabled', render: (v: boolean) => v ? <Tag color="green">启用</Tag> : <Tag>禁用</Tag> },
  ]

  return (
    <div style={{ padding: 24 }}>
      <Card title={<><NotificationOutlined /> 告警高级管理</>} extra={<Button onClick={handleEvaluate}>手动评估告警</Button>}>
        <Tabs items={[
          { key: 'templates', label: '告警模板', children: <>
            <Button type="primary" icon={<PlusOutlined />} onClick={() => { form.resetFields(); setTplOpen(true) }} style={{ marginBottom: 16 }}>新建模板</Button>
            <Table columns={tplCols} dataSource={templates} rowKey="id" loading={loading} size="small" />
          </>},
          { key: 'escalations', label: '升级规则', children: <>
            <Button icon={<PlusOutlined />} onClick={() => { form.resetFields(); setEscOpen(true) }} style={{ marginBottom: 16 }}>新建升级规则</Button>
            <Table columns={escCols} dataSource={escalations} rowKey="id" loading={loading} size="small" />
          </>},
          { key: 'silences', label: '静默规则', children: <>
            <Button icon={<PauseCircleOutlined />} onClick={() => { form.resetFields(); setSilOpen(true) }} style={{ marginBottom: 16 }}>新建静默</Button>
            <Table columns={silCols} dataSource={silences} rowKey="id" loading={loading} size="small" />
          </>},
        ]} />
      </Card>
      <Modal title="新建告警模板" open={tplOpen} onCancel={() => setTplOpen(false)} onOk={() => form.validateFields().then(handleSaveTemplate)}>
        <Form form={form} layout="vertical">
          <Form.Item name="name" label="名称" rules={[{ required: true }]}><Input /></Form.Item>
          <Form.Item name="category" label="类别" rules={[{ required: true }]}><Select options={['性能','可用性','安全','容量'].map(v=>({label:v,value:v}))} /></Form.Item>
          <Form.Item name="metric" label="指标" rules={[{ required: true }]}><Input placeholder="如 cpu_usage, memory_usage" /></Form.Item>
          <Form.Item name="severity" label="严重性" rules={[{ required: true }]}><Select options={['info','warning','critical'].map(v=>({label:v,value:v}))} /></Form.Item>
          <Form.Item name="message" label="消息模板"><Input.TextArea rows={2} /></Form.Item>
        </Form>
      </Modal>
      <Modal title="新建升级规则" open={escOpen} onCancel={() => setEscOpen(false)} onOk={() => form.validateFields().then(handleSaveEscalation)}>
        <Form form={form} layout="vertical">
          <Form.Item name="rule_id" label="关联规则ID" rules={[{ required: true }]}><Input /></Form.Item>
          <Form.Item name="level" label="升级级别" initialValue={1}><Input type="number" /></Form.Item>
          <Form.Item name="severity" label="严重性" initialValue="critical"><Select options={['warning','critical'].map(v=>({label:v,value:v}))} /></Form.Item>
          <Form.Item name="interval_sec" label="升级间隔(秒)" initialValue={300}><Input type="number" /></Form.Item>
          <Form.Item name="notify_channels" label="通知渠道"><Input placeholder="email,dingtalk,wecom" /></Form.Item>
        </Form>
      </Modal>
      <Modal title="新建静默规则" open={silOpen} onCancel={() => setSilOpen(false)} onOk={() => form.validateFields().then(handleSaveSilence)}>
        <Form form={form} layout="vertical">
          <Form.Item name="name" label="名称" rules={[{ required: true }]}><Input /></Form.Item>
          <Form.Item name="rule_ids" label="规则IDs"><Input placeholder="逗号分隔" /></Form.Item>
          <Form.Item name="start_at" label="开始时间"><Input type="datetime-local" /></Form.Item>
          <Form.Item name="end_at" label="结束时间"><Input type="datetime-local" /></Form.Item>
        </Form>
      </Modal>
    </div>
  )
}
