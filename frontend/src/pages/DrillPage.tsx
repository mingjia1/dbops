import React, { useState, useEffect } from 'react'
import { Card, Table, Button, Modal, Form, Input, Select, Space, Tag, message, Popconfirm, Descriptions, Progress } from 'antd'
import { ExperimentOutlined, PlusOutlined, PlayCircleOutlined, DeleteOutlined, FileTextOutlined } from '@ant-design/icons'
import api from '../services/api'

interface HADrill {
  id: string
  name: string
  description: string
  status: string
  score: number
  started_at?: string
  completed_at?: string
}

export default function DrillPage() {
  const [drills, setDrills] = useState<HADrill[]>([])
  const [loading, setLoading] = useState(false)
  const [addOpen, setAddOpen] = useState(false)
  const [reportOpen, setReportOpen] = useState(false)
  const [report, setReport] = useState<any>(null)
  const [form] = Form.useForm()

  const fetchDrills = async () => {
    setLoading(true)
    try { const res = await api.get('/drills'); setDrills(res.data || []) }
    catch {}
    finally { setLoading(false) }
  }
  useEffect(() => { fetchDrills() }, [])

  const handleCreate = async (values: any) => {
    try { await api.post('/drills', values); message.success('创建成功'); setAddOpen(false); form.resetFields(); fetchDrills() }
    catch (err: any) { message.error(err.message) }
  }
  const handleStart = async (id: string) => {
    try { await api.post(`/drills/${id}/start`); message.success('演练已启动'); fetchDrills() }
    catch (err: any) { message.error(err.message) }
  }
  const handleDelete = async (id: string) => {
    try { await api.delete(`/drills/${id}`); message.success('已删除'); fetchDrills() }
    catch (err: any) { message.error(err.message) }
  }
  const viewReport = async (id: string) => {
    try { const res = await api.get(`/drills/${id}/report`); setReport(res.data); setReportOpen(true) }
    catch { message.error('获取报告失败') }
  }

  const columns = [
    { title: '名称', dataIndex: 'name', key: 'name' },
    { title: '描述', dataIndex: 'description', key: 'description', ellipsis: true },
    { title: '状态', dataIndex: 'status', key: 'status', render: (v: string) => {
      const colors: Record<string, string> = { draft: 'default', running: 'processing', completed: 'success', failed: 'error' }
      return <Tag color={colors[v] || 'default'}>{v}</Tag>
    }},
    { title: '评分', dataIndex: 'score', key: 'score', render: (v: number) => v > 0 ? <Tag color={v >= 80 ? 'green' : 'orange'}>{v}</Tag> : '-' },
    { title: '操作', key: 'action', render: (_: any, r: HADrill) => (
      <Space>
        {r.status === 'draft' && <Button size="small" type="primary" icon={<PlayCircleOutlined />} onClick={() => handleStart(r.id)}>启动</Button>}
        {r.status === 'completed' && <Button size="small" icon={<FileTextOutlined />} onClick={() => viewReport(r.id)}>报告</Button>}
        <Popconfirm title="确认删除？" onConfirm={() => handleDelete(r.id)}>
          <Button size="small" danger icon={<DeleteOutlined />}>删除</Button>
        </Popconfirm>
      </Space>
    )},
  ]

  return (
    <div style={{ padding: 24 }}>
      <Card title={<><ExperimentOutlined /> HA 演练管理</>}
        extra={<Button type="primary" icon={<PlusOutlined />} onClick={() => { form.resetFields(); setAddOpen(true) }}>新建演练</Button>}>
        <Table columns={columns} dataSource={drills} rowKey="id" loading={loading} size="small" />
      </Card>
      <Modal title="新建 HA 演练" open={addOpen} onCancel={() => setAddOpen(false)} onOk={() => form.validateFields().then(handleCreate)}>
        <Form form={form} layout="vertical">
          <Form.Item name="name" label="名称" rules={[{ required: true }]}><Input /></Form.Item>
          <Form.Item name="description" label="描述"><Input.TextArea rows={2} /></Form.Item>
          <Form.Item name="plan" label="演练计划 (JSON)"><Input.TextArea rows={3} placeholder='{"type":"failover","target":"cluster-01"}' /></Form.Item>
        </Form>
      </Modal>
      <Modal title="演练报告" open={reportOpen} onCancel={() => setReportOpen(false)} footer={null} width={600}>
        {report && (
          <Descriptions bordered size="small" column={1}>
            <Descriptions.Item label="评分"><Progress type="circle" percent={report.score} size={60} /></Descriptions.Item>
            <Descriptions.Item label="摘要">{report.summary}</Descriptions.Item>
            <Descriptions.Item label="时间线"><pre style={{ maxHeight: 200, overflow: 'auto' }}>{report.timeline}</pre></Descriptions.Item>
            <Descriptions.Item label="发现"><pre style={{ maxHeight: 200, overflow: 'auto' }}>{report.findings}</pre></Descriptions.Item>
          </Descriptions>
        )}
      </Modal>
    </div>
  )
}
