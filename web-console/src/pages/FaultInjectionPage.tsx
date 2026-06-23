import React, { useState, useEffect } from 'react'
import { Card, Table, Button, Modal, Form, Input, Select, Space, Tag, message, Popconfirm, Descriptions } from 'antd'
import { ThunderboltOutlined, PlusOutlined, DeleteOutlined, PlayCircleOutlined, RollbackOutlined } from '@ant-design/icons'
import api from '../services/api'

interface FaultTemplate {
  id: string
  name: string
  category: string
  fault_type: string
  description: string
  severity: string
  duration_sec: number
}

interface FaultExecution {
  id: string
  template_id: string
  target_type: string
  target_id: string
  fault_type: string
  status: string
  started_at: string
  completed_at?: string
}

export default function FaultInjectionPage() {
  const [templates, setTemplates] = useState<FaultTemplate[]>([])
  const [executions, setExecutions] = useState<FaultExecution[]>([])
  const [loading, setLoading] = useState(false)
  const [addOpen, setAddOpen] = useState(false)
  const [execOpen, setExecOpen] = useState(false)
  const [form] = Form.useForm()

  const fetchAll = async () => {
    setLoading(true)
    try {
      const [tRes, eRes] = await Promise.all([api.get('/faults/templates'), api.get('/faults/executions')])
      setTemplates(tRes.data?.data || [])
      setExecutions(eRes.data?.data || [])
    } catch {}
    finally { setLoading(false) }
  }
  useEffect(() => { fetchAll() }, [])

  const handleAdd = async (values: any) => {
    try { await api.post('/faults/templates', values); message.success('创建成功'); setAddOpen(false); form.resetFields(); fetchAll() }
    catch (err: any) { message.error(err.message) }
  }
  const handleExecute = async (values: any) => {
    try { await api.post('/faults/execute', values); message.success('故障注入已触发'); setExecOpen(false); form.resetFields(); fetchAll() }
    catch (err: any) { message.error(err.message) }
  }
  const handleRollback = async (id: string) => {
    try { await api.post(`/faults/${id}/rollback`); message.success('已回滚'); fetchAll() }
    catch (err: any) { message.error(err.message) }
  }
  const handleDelete = async (id: string) => {
    try { await api.delete(`/faults/templates/${id}`); message.success('已删除'); fetchAll() }
    catch (err: any) { message.error(err.message) }
  }

  const tplCols = [
    { title: '名称', dataIndex: 'name', key: 'name' },
    { title: '类别', dataIndex: 'category', key: 'category', render: (v: string) => <Tag>{v}</Tag> },
    { title: '故障类型', dataIndex: 'fault_type', key: 'fault_type', render: (v: string) => <Tag color="red">{v}</Tag> },
    { title: '严重性', dataIndex: 'severity', key: 'severity', render: (v: string) => <Tag color={v === 'critical' ? 'red' : v === 'warning' ? 'orange' : 'blue'}>{v}</Tag> },
    { title: '持续(秒)', dataIndex: 'duration_sec', key: 'duration_sec', width: 80 },
    { title: '操作', key: 'action', render: (_: any, r: FaultTemplate) => (
      <Popconfirm title="确认删除？" onConfirm={() => handleDelete(r.id)}>
        <Button size="small" danger icon={<DeleteOutlined />}>删除</Button>
      </Popconfirm>
    )},
  ]

  const execCols = [
    { title: '目标类型', dataIndex: 'target_type', key: 'target_type', render: (v: string) => <Tag>{v}</Tag> },
    { title: '目标 ID', dataIndex: 'target_id', key: 'target_id', ellipsis: true },
    { title: '故障类型', dataIndex: 'fault_type', key: 'fault_type', render: (v: string) => <Tag color="red">{v}</Tag> },
    { title: '状态', dataIndex: 'status', key: 'status', render: (v: string) => {
      const colors: Record<string, string> = { pending: 'default', running: 'processing', completed: 'success', rolled_back: 'warning' }
      return <Tag color={colors[v] || 'default'}>{v}</Tag>
    }},
    { title: '操作', key: 'action', render: (_: any, r: FaultExecution) => (
      r.status === 'completed' ? <Button size="small" icon={<RollbackOutlined />} onClick={() => handleRollback(r.id)}>回滚</Button> : null
    )},
  ]

  return (
    <div style={{ padding: 24 }}>
      <Card title={<><ThunderboltOutlined /> 故障注入</>}>
        <Space style={{ marginBottom: 16 }}>
          <Button type="primary" icon={<PlusOutlined />} onClick={() => { form.resetFields(); setAddOpen(true) }}>新建模板</Button>
          <Button icon={<PlayCircleOutlined />} onClick={() => { form.resetFields(); setExecOpen(true) }}>执行故障注入</Button>
        </Space>
        <Table columns={tplCols} dataSource={templates} rowKey="id" loading={loading} size="small" title={() => '故障模板'} style={{ marginBottom: 24 }} />
        <Table columns={execCols} dataSource={executions} rowKey="id" loading={loading} size="small" title={() => '执行记录'} />
      </Card>
      <Modal title="新建故障模板" open={addOpen} onCancel={() => setAddOpen(false)} onOk={() => form.validateFields().then(handleAdd)}>
        <Form form={form} layout="vertical">
          <Form.Item name="name" label="名称" rules={[{ required: true }]}><Input /></Form.Item>
          <Form.Item name="category" label="类别" rules={[{ required: true }]}>
            <Select options={['网络', '磁盘', 'CPU', '内存', '进程'].map(v => ({ label: v, value: v }))} />
          </Form.Item>
          <Form.Item name="fault_type" label="故障类型" rules={[{ required: true }]}>
            <Select options={['network_delay', 'disk_full', 'cpu_stress', 'oom_kill', 'process_kill'].map(v => ({ label: v, value: v }))} />
          </Form.Item>
          <Form.Item name="duration_sec" label="持续时间(秒)" initialValue={30}><Input type="number" /></Form.Item>
          <Form.Item name="severity" label="严重性" initialValue="warning">
            <Select options={['info', 'warning', 'critical'].map(v => ({ label: v, value: v }))} />
          </Form.Item>
        </Form>
      </Modal>
      <Modal title="执行故障注入" open={execOpen} onCancel={() => setExecOpen(false)} onOk={() => form.validateFields().then(handleExecute)}>
        <Form form={form} layout="vertical">
          <Form.Item name="template_id" label="选择模板" rules={[{ required: true }]}>
            <Select options={templates.map(t => ({ label: `${t.name} (${t.fault_type})`, value: t.id }))} />
          </Form.Item>
          <Form.Item name="target_type" label="目标类型" rules={[{ required: true }]}>
            <Select options={[{ label: '实例', value: 'instance' }, { label: '主机', value: 'host' }, { label: '集群', value: 'cluster' }]} />
          </Form.Item>
          <Form.Item name="target_id" label="目标 ID" rules={[{ required: true }]}><Input /></Form.Item>
        </Form>
      </Modal>
    </div>
  )
}
