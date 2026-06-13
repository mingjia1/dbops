import React, { useEffect, useMemo, useState } from 'react'
import { Button, Card, Checkbox, Descriptions, Form, Input, Modal, Select, Space, Table, Tag, message } from 'antd'
import { CheckCircleOutlined, DeleteOutlined, EditOutlined, EyeOutlined, PlusOutlined, ThunderboltOutlined } from '@ant-design/icons'
import type { ColumnsType } from 'antd/es/table'
import {
  instanceApi,
  parameterTemplateApi,
  parameterTemplateParamsToJson,
  type Instance,
  type ParameterTemplate,
} from '../services/api'

const restartRequired = new Set([
  'innodb_buffer_pool_size',
  'innodb_log_file_size',
  'max_connections',
  'table_open_cache',
  'open_files_limit',
  'innodb_buffer_pool_instances',
])

const parseParams = (text?: string) => {
  if (!text?.trim()) return []
  const parsed = JSON.parse(text)
  if (Array.isArray(parsed)) return parsed
  return Object.entries(parsed).map(([name, value]) => ({
    parameter_name: name,
    value: String(value),
    data_type: /^\d+$/.test(String(value)) ? 'int' : /^\d+(k|m|g)$/i.test(String(value)) ? 'size' : 'string',
    is_dynamic: true,
    category: 'custom',
  }))
}

const ParameterTemplateList: React.FC = () => {
  const [data, setData] = useState<ParameterTemplate[]>([])
  const [instances, setInstances] = useState<Instance[]>([])
  const [loading, setLoading] = useState(false)
  const [submitting, setSubmitting] = useState(false)
  const [editing, setEditing] = useState<ParameterTemplate | null>(null)
  const [viewing, setViewing] = useState<ParameterTemplate | null>(null)
  const [applying, setApplying] = useState<ParameterTemplate | null>(null)
  const [recommendResult, setRecommendResult] = useState<any>(null)
  const [modalOpen, setModalOpen] = useState(false)
  const [applyOpen, setApplyOpen] = useState(false)
  const [recommendOpen, setRecommendOpen] = useState(false)
  const [form] = Form.useForm()
  const [applyForm] = Form.useForm()
  const paramsText = Form.useWatch('parameters', form)

  const fetchData = async () => {
    setLoading(true)
    try {
      const [tplRes, instRes]: any[] = await Promise.all([
        parameterTemplateApi.list(),
        instanceApi.list(1000, 0),
      ])
      setData(tplRes?.data || [])
      setInstances(instRes?.data || [])
    } catch {
      setData([])
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    fetchData()
  }, [])

  const parsedParams = useMemo(() => {
    try {
      return { ok: true, rows: parseParams(paramsText) }
    } catch (err: any) {
      return { ok: false, error: err.message, rows: [] }
    }
  }, [paramsText])

  const restartKeys = parsedParams.rows
    .map((row: any) => row.parameter_name)
    .filter((name: string) => restartRequired.has(name))

  const openCreate = () => {
    setEditing(null)
    form.setFieldsValue({ category: 'custom', parameters: '{\n  "max_connections": "500"\n}' })
    setModalOpen(true)
  }

  const openEdit = (template: ParameterTemplate) => {
    setEditing(template)
    form.setFieldsValue({
      name: template.name,
      category: template.category,
      description: template.description,
      parameters: parameterTemplateParamsToJson(template.parameters),
    })
    setModalOpen(true)
  }

  const submitTemplate = async () => {
    const values = await form.validateFields()
    if (!parsedParams.ok) {
      message.error(`参数 JSON 格式错误: ${parsedParams.error}`)
      return
    }
    setSubmitting(true)
    try {
      if (editing) {
        await parameterTemplateApi.update(editing.id, values)
        message.success('模板已更新')
      } else {
        await parameterTemplateApi.create(values)
        message.success('模板已创建')
      }
      setModalOpen(false)
      fetchData()
    } finally {
      setSubmitting(false)
    }
  }

  const deleteTemplate = (template: ParameterTemplate) => {
    Modal.confirm({
      title: '删除参数模板',
      content: `确定删除 ${template.name} 吗？`,
      okText: '删除',
      cancelText: '取消',
      okButtonProps: { danger: true },
      onOk: async () => {
        await parameterTemplateApi.delete(template.id)
        message.success('模板已删除')
        fetchData()
      },
    })
  }

  const submitRecommend = async () => {
    const values = await applyForm.validateFields()
    setSubmitting(true)
    try {
      const res: any = await parameterTemplateApi.recommend(values)
      setRecommendResult(res?.data || res)
      message.success('推荐参数已生成')
    } finally {
      setSubmitting(false)
    }
  }

  const submitApply = async () => {
    if (!applying) return
    const values = await applyForm.validateFields()
    if (!values.confirm) {
      message.warning('请先确认变更影响')
      return
    }
    setSubmitting(true)
    try {
      const res: any = await parameterTemplateApi.apply({
        template_id: applying.id,
        instance_id: values.instance_id,
        parameters: applying.parameters,
        require_restart: values.require_restart,
      })
      const result = res?.data || {}
      const failed = result.failed || 0
      const applied = result.applied || 0
      if (failed > 0) {
        const rows = (result.results || [])
          .filter((row: any) => row.status !== 'completed' && row.status !== 'success')
          .map((row: any) => `${row.name || '-'}=${row.value || ''}: ${row.message || row.status || 'failed'}`)
          .join('\n')
        Modal.warning({
          title: `参数模板应用存在失败：成功 ${applied} 个，失败 ${failed} 个`,
          content: <div style={{ maxHeight: 260, overflow: 'auto', whiteSpace: 'pre-wrap' }}>{rows || '存在失败参数，但接口未返回失败明细'}</div>,
        })
      } else {
        message.success(`参数模板已应用：成功 ${applied} 个`)
      }
      setApplyOpen(false)
    } finally {
      setSubmitting(false)
    }
  }

  const columns: ColumnsType<ParameterTemplate> = [
    { title: '模板名称', dataIndex: 'name', key: 'name' },
    { title: '分类', dataIndex: 'category', key: 'category', render: (v) => <Tag color="blue">{v || 'custom'}</Tag> },
    { title: '参数数', key: 'count', render: (_, item) => item.parameters?.length || 0 },
    { title: '预设', dataIndex: 'is_preset', key: 'is_preset', render: (v) => (v ? <Tag color="purple">预设</Tag> : <Tag>自定义</Tag>) },
    { title: '描述', dataIndex: 'description', key: 'description' },
    { title: '更新时间', dataIndex: 'updated_at', key: 'updated_at', render: (v) => (v ? new Date(v).toLocaleString() : '-') },
    {
      title: '操作',
      key: 'action',
      width: 330,
      render: (_, item) => (
        <Space>
          <Button size="small" icon={<EyeOutlined />} onClick={() => setViewing(item)}>详情</Button>
          <Button size="small" icon={<EditOutlined />} disabled={item.is_preset} onClick={() => openEdit(item)}>编辑</Button>
          <Button size="small" icon={<ThunderboltOutlined />} onClick={() => { setApplying(item); setRecommendResult(null); applyForm.resetFields(); setRecommendOpen(true) }}>推荐</Button>
          <Button size="small" icon={<CheckCircleOutlined />} onClick={() => { setApplying(item); applyForm.resetFields(); setApplyOpen(true) }}>应用</Button>
          <Button size="small" danger icon={<DeleteOutlined />} disabled={item.is_preset} onClick={() => deleteTemplate(item)}>删除</Button>
        </Space>
      ),
    },
  ]

  return (
    <div style={{ padding: 24 }}>
      <Card title="参数模板" extra={<Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>创建模板</Button>}>
        <Table columns={columns} dataSource={data} rowKey="id" loading={loading} />
      </Card>

      <Modal title={editing ? '编辑参数模板' : '创建参数模板'} open={modalOpen} onCancel={() => setModalOpen(false)} onOk={submitTemplate} confirmLoading={submitting} width={720}>
        <Form form={form} layout="vertical">
          <Form.Item name="name" label="模板名称" rules={[{ required: true }]}>
            <Input placeholder="例如: MGR-OLTP-standard" />
          </Form.Item>
          <Form.Item name="category" label="分类" rules={[{ required: true }]}>
            <Select options={['oltp', 'olap', 'mgr', 'mha', 'pxc', 'ha', 'custom'].map((v) => ({ value: v, label: v }))} />
          </Form.Item>
          <Form.Item name="description" label="描述"><Input.TextArea rows={2} /></Form.Item>
          <Form.Item
            name="parameters"
            label="参数 JSON"
            validateStatus={!parsedParams.ok ? 'error' : restartKeys.length > 0 ? 'warning' : undefined}
            help={!parsedParams.ok ? parsedParams.error : restartKeys.length > 0 ? `包含需重启参数: ${restartKeys.join(', ')}` : '支持对象格式，例如 {"max_connections":"500"}'}
          >
            <Input.TextArea rows={8} />
          </Form.Item>
        </Form>
      </Modal>

      <Modal title="模板详情" open={!!viewing} onCancel={() => setViewing(null)} footer={<Button onClick={() => setViewing(null)}>关闭</Button>} width={720}>
        {viewing && (
          <Descriptions bordered column={1}>
            <Descriptions.Item label="ID">{viewing.id}</Descriptions.Item>
            <Descriptions.Item label="名称">{viewing.name}</Descriptions.Item>
            <Descriptions.Item label="分类">{viewing.category}</Descriptions.Item>
            <Descriptions.Item label="参数">
              <pre style={{ margin: 0, whiteSpace: 'pre-wrap' }}>{parameterTemplateParamsToJson(viewing.parameters)}</pre>
            </Descriptions.Item>
          </Descriptions>
        )}
      </Modal>

      <Modal title={`推荐参数: ${applying?.name || ''}`} open={recommendOpen} onCancel={() => setRecommendOpen(false)} onOk={submitRecommend} confirmLoading={submitting} width={720}>
        <Form form={applyForm} layout="vertical" initialValues={{ workload_type: 'oltp', cpu_cores: 4, memory_gb: 16, disk_gb: 100 }}>
          <Form.Item name="workload_type" label="负载类型"><Select options={['oltp', 'olap', 'mixed'].map((v) => ({ value: v, label: v }))} /></Form.Item>
          <Space>
            <Form.Item name="cpu_cores" label="CPU 核数"><Input type="number" /></Form.Item>
            <Form.Item name="memory_gb" label="内存 GB"><Input type="number" /></Form.Item>
            <Form.Item name="disk_gb" label="磁盘 GB"><Input type="number" /></Form.Item>
          </Space>
        </Form>
        {recommendResult && <pre style={{ maxHeight: 280, overflow: 'auto' }}>{JSON.stringify(recommendResult, null, 2)}</pre>}
      </Modal>

      <Modal title={`应用模板: ${applying?.name || ''}`} open={applyOpen} onCancel={() => setApplyOpen(false)} onOk={submitApply} confirmLoading={submitting} okText="确认应用" width={640}>
        <Form form={applyForm} layout="vertical">
          <Form.Item name="instance_id" label="目标实例" rules={[{ required: true }]}>
            <Select showSearch optionFilterProp="label" options={instances.map((i) => ({ value: i.id, label: `${i.name} (${i.connection?.host}:${i.connection?.port})` }))} />
          </Form.Item>
          <Form.Item name="require_restart" valuePropName="checked">
            <Checkbox>包含需重启参数，已安排维护窗口</Checkbox>
          </Form.Item>
          <Form.Item name="confirm" valuePropName="checked">
            <Checkbox>确认将模板参数应用到目标实例</Checkbox>
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}

export default ParameterTemplateList
