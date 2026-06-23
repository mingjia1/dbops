import React, { useState, useEffect } from 'react'
import { Card, Table, Button, Modal, Form, Input, Select, Space, Tag, message, Popconfirm, Tabs, Descriptions } from 'antd'
import { FileSearchOutlined, PlusOutlined, DeleteOutlined, PlayCircleOutlined, ReloadOutlined } from '@ant-design/icons'
import api from '../services/api'

interface InspectionTemplate {
  id: string
  name: string
  category: string
  schedule?: string
  enabled: boolean
}

interface InspectionReport {
  id: string
  template_id: string
  instance_id: string
  status: string
  summary: string
  score: number
  generated_at: string
}

export default function InspectionPage() {
  const [templates, setTemplates] = useState<InspectionTemplate[]>([])
  const [reports, setReports] = useState<InspectionReport[]>([])
  const [loading, setLoading] = useState(false)
  const [addOpen, setAddOpen] = useState(false)
  const [generateOpen, setGenerateOpen] = useState(false)
  const [form] = Form.useForm()

  const fetchAll = async () => {
    setLoading(true)
    try {
      const [tRes, rRes] = await Promise.all([api.get('/alerts/inspection/templates'), api.get('/alerts/inspection/reports')])
      setTemplates(tRes.data?.data || [])
      setReports(rRes.data?.data || [])
    } catch {}
    finally { setLoading(false) }
  }
  useEffect(() => { fetchAll() }, [])

  const handleAddTemplate = async (values: any) => {
    try { await api.post('/alerts/inspection/templates', values); message.success('创建成功'); setAddOpen(false); form.resetFields(); fetchAll() }
    catch (err: any) { message.error(err.message) }
  }

  const handleGenerate = async (values: any) => {
    try { await api.post('/alerts/inspection/generate', values); message.success('巡检任务已提交'); setGenerateOpen(false); fetchAll() }
    catch (err: any) { message.error(err.message) }
  }

  const handleDeleteTemplate = async (id: string) => {
    try { await api.delete(`/alerts/inspection/templates/${id}`); message.success('已删除'); fetchAll() }
    catch (err: any) { message.error(err.message) }
  }

  const tplCols = [
    { title: '名称', dataIndex: 'name', key: 'name' },
    { title: '类别', dataIndex: 'category', key: 'category', render: (v: string) => <Tag>{v}</Tag> },
    { title: '调度', dataIndex: 'schedule', key: 'schedule', render: (v: string) => v || '手动' },
    { title: '状态', dataIndex: 'enabled', key: 'enabled', render: (v: boolean) => v ? <Tag color="green">启用</Tag> : <Tag>禁用</Tag> },
    { title: '操作', key: 'action', render: (_: any, r: InspectionTemplate) => (
      <Popconfirm title="确认删除？" onConfirm={() => handleDeleteTemplate(r.id)}>
        <Button size="small" danger icon={<DeleteOutlined />}>删除</Button>
      </Popconfirm>
    )},
  ]

  const rptCols = [
    { title: '实例', dataIndex: 'instance_id', key: 'instance_id', ellipsis: true },
    { title: '状态', dataIndex: 'status', key: 'status', render: (v: string) => <Tag color={v === 'completed' ? 'green' : 'blue'}>{v}</Tag> },
    { title: '摘要', dataIndex: 'summary', key: 'summary', ellipsis: true },
    { title: '评分', dataIndex: 'score', key: 'score', render: (v: number) => <Tag color={v >= 80 ? 'green' : v >= 60 ? 'orange' : 'red'}>{v}</Tag> },
    { title: '生成时间', dataIndex: 'generated_at', key: 'generated_at', width: 160, render: (v: string) => new Date(v).toLocaleString() },
  ]

  return (
    <div style={{ padding: 24 }}>
      <Card title={<><FileSearchOutlined /> 巡检管理</>} extra={<Button icon={<ReloadOutlined />} onClick={fetchAll}>刷新</Button>}>
        <Tabs items={[
          { key: 'templates', label: '巡检模板', children: (
            <>
              <Button type="primary" icon={<PlusOutlined />} onClick={() => { form.resetFields(); setAddOpen(true) }} style={{ marginBottom: 16 }}>新建模板</Button>
              <Table columns={tplCols} dataSource={templates} rowKey="id" loading={loading} size="small" />
            </>
          )},
          { key: 'reports', label: '巡检报告', children: (
            <>
              <Button icon={<PlayCircleOutlined />} onClick={() => setGenerateOpen(true)} style={{ marginBottom: 16 }}>执行巡检</Button>
              <Table columns={rptCols} dataSource={reports} rowKey="id" loading={loading} size="small" />
            </>
          )},
        ]} />
      </Card>
      <Modal title="新建巡检模板" open={addOpen} onCancel={() => setAddOpen(false)} onOk={() => form.validateFields().then(handleAddTemplate)}>
        <Form form={form} layout="vertical">
          <Form.Item name="name" label="名称" rules={[{ required: true }]}><Input /></Form.Item>
          <Form.Item name="category" label="类别" rules={[{ required: true }]}>
            <Select options={['性能', '安全', '可用性', '配置'].map(v => ({ label: v, value: v }))} />
          </Form.Item>
          <Form.Item name="schedule" label="调度 (cron)"><Input placeholder="留空表示手动触发" /></Form.Item>
          <Form.Item name="config" label="配置 JSON"><Input.TextArea rows={3} placeholder='{"checks":["cpu","memory","disk"]}' /></Form.Item>
        </Form>
      </Modal>
      <Modal title="执行巡检" open={generateOpen} onCancel={() => setGenerateOpen(false)} onOk={() => form.validateFields().then(handleGenerate)}>
        <Form form={form} layout="vertical">
          <Form.Item name="template_id" label="选择模板" rules={[{ required: true }]}>
            <Select options={templates.map(t => ({ label: t.name, value: t.id }))} />
          </Form.Item>
          <Form.Item name="instance_id" label="实例 ID" rules={[{ required: true }]}><Input /></Form.Item>
        </Form>
      </Modal>
    </div>
  )
}
