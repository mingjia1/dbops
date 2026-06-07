import React, { useEffect, useMemo, useState } from 'react'
import { Card, Table, Button, Space, Tag, message, Modal, Form, Input, Select, Descriptions, Alert, Checkbox } from 'antd'
import { PlusOutlined, EditOutlined, DeleteOutlined, EyeOutlined, ThunderboltOutlined, CheckCircleOutlined, WarningOutlined } from '@ant-design/icons'
import type { ColumnsType } from 'antd/es/table'
import { parameterTemplateApi, instanceApi, type ParameterTemplate, type Instance } from '../services/api'

const RESTART_REQUIRED_PARAMS = [
  'innodb_buffer_pool_size',
  'innodb_log_file_size',
  'max_connections',
  'innodb_thread_concurrency',
  'key_buffer_size',
  'table_open_cache',
  'open_files_limit',
  'innodb_buffer_pool_instances',
]

const ParameterTemplateList: React.FC = () => {
  const [data, setData] = useState<ParameterTemplate[]>([])
  const [loading, setLoading] = useState(false)
  const [editing, setEditing] = useState<ParameterTemplate | null>(null)
  const [viewing, setViewing] = useState<ParameterTemplate | null>(null)
  const [modalOpen, setModalOpen] = useState(false)
  const [applyOpen, setApplyOpen] = useState(false)
  const [recommendOpen, setRecommendOpen] = useState(false)
  const [applying, setApplying] = useState<ParameterTemplate | null>(null)
  const [submitting, setSubmitting] = useState(false)
  const [instances, setInstances] = useState<Instance[]>([])
  const [recommendResult, setRecommendResult] = useState<any>(null)
  const [paramJsonError, setParamJsonError] = useState<string | null>(null)
  const [form] = Form.useForm()
  const [applyForm] = Form.useForm()
  const paramsValue = Form.useWatch('parameters', form)

  const fetchData = () => {
    setLoading(true)
    parameterTemplateApi.list().then((res: any) => {
      setData(res?.data || [])
    }).catch(() => setData([])).finally(() => setLoading(false))
  }

  useEffect(() => {
    fetchData()
    instanceApi.list(100, 0).then((res: any) => setInstances(res?.data || [])).catch(() => {})
  }, [])

  const parsedParams = useMemo(() => {
    if (!paramsValue) return null
    const trimmed = paramsValue.trim()
    if (!trimmed) return null
    try {
      return { ok: true as const, data: JSON.parse(trimmed) }
    } catch (e: any) {
      return { ok: false as const, error: e.message }
    }
  }, [paramsValue])

  useEffect(() => {
    if (!paramsValue) { setParamJsonError(null); return }
    if (parsedParams && !parsedParams.ok) setParamJsonError(parsedParams.error)
    else setParamJsonError(null)
  }, [parsedParams, paramsValue])

  const detectRestart = (params: any): string[] => {
    if (!params || typeof params !== 'object') return []
    return Object.keys(params).filter((k) => RESTART_REQUIRED_PARAMS.includes(k))
  }

  const openCreate = () => {
    setEditing(null)
    form.resetFields()
    setParamJsonError(null)
    setModalOpen(true)
  }

  const openEdit = (r: ParameterTemplate) => {
    setEditing(r)
    form.setFieldsValue({
      name: r.name,
      category: r.category,
      description: r.description,
      parameters: r.parameters,
    })
    setParamJsonError(null)
    setModalOpen(true)
  }

  const handleDelete = (id: string) => {
    Modal.confirm({
      title: '确认删除',
      content: '确定要删除此参数模板吗？',
      okText: '删除',
      cancelText: '取消',
      okButtonProps: { danger: true },
      onOk: () => parameterTemplateApi.delete(id).then(() => {
        message.success('删除成功')
        fetchData()
      }).catch(() => {}),
    })
  }

  const submit = async () => {
    try {
      const values = await form.validateFields()
      if (paramJsonError) {
        message.error('参数 JSON 格式错误, 请先修正')
        return
      }
      setSubmitting(true)
      if (editing) {
        await parameterTemplateApi.update(editing.id, values)
        message.success('更新成功')
      } else {
        await parameterTemplateApi.create(values)
        message.success('创建成功')
      }
      setModalOpen(false)
      fetchData()
    } catch (err: any) {
      message.error(err?.response?.data?.message || '操作失败')
    } finally {
      setSubmitting(false)
    }
  }

  const openApply = (r: ParameterTemplate) => {
    setApplying(r)
    applyForm.resetFields()
    setApplyOpen(true)
  }

  const openRecommend = (r: ParameterTemplate) => {
    setApplying(r)
    applyForm.resetFields()
    setRecommendResult(null)
    setRecommendOpen(true)
  }

  const submitRecommend = async () => {
    if (!applying) return
    try {
      const values = await applyForm.validateFields()
      setSubmitting(true)
      const res: any = await parameterTemplateApi.recommend({
        instance_id: values.instance_id,
        template_id: applying.id,
        workload_type: values.workload_type,
      })
      setRecommendResult(res?.data || res)
      message.success('已生成推荐参数')
    } catch (err: any) {
      message.error(err?.response?.data?.message || '推荐失败')
    } finally {
      setSubmitting(false)
    }
  }

  const submitApply = async () => {
    if (!applying) return
    try {
      const values = await applyForm.validateFields()
      if (!values.confirm_restart) {
        message.warning('请确认已了解重启影响')
        return
      }
      setSubmitting(true)
      try {
        await parameterTemplateApi.apply({
          template_id: applying.id,
          instance_id: values.instance_id,
          parameters: applying.parameters,
          require_restart: values.require_restart,
        })
        message.success(`已将模板 ${applying.name} 推送到实例 ${values.instance_id}`)
      } catch (err: any) {
        if (err?.response?.status === 404) {
          message.warning('后端未实现 apply 接口, 请使用"推荐参数"功能')
        } else {
          throw err
        }
      }
      setApplyOpen(false)
    } catch (err: any) {
      message.error(err?.response?.data?.message || '应用失败')
    } finally {
      setSubmitting(false)
    }
  }

  const restartKeys = useMemo(() => {
    if (!parsedParams || !parsedParams.ok) return [] as string[]
    return detectRestart(parsedParams.data)
  }, [parsedParams])

  const columns: ColumnsType<ParameterTemplate> = [
    { title: '模板名称', dataIndex: 'name', key: 'name' },
    {
      title: '分类',
      dataIndex: 'category',
      key: 'category',
      render: (c: string) => <Tag color="blue">{c || '未分类'}</Tag>,
    },
    {
      title: '是否预设',
      dataIndex: 'is_preset',
      key: 'is_preset',
      render: (p: boolean) => (p ? <Tag color="purple">预设</Tag> : <Tag>自定义</Tag>),
    },
    { title: '描述', dataIndex: 'description', key: 'description' },
    {
      title: '创建时间',
      dataIndex: 'created_at',
      key: 'created_at',
      render: (t: string) => (t ? new Date(t).toLocaleString() : '-'),
    },
    {
      title: '操作',
      key: 'action',
      width: 280,
      render: (_, record) => (
        <Space>
          <Button type="link" size="small" icon={<EyeOutlined />} onClick={() => setViewing(record)}>
            查看
          </Button>
          <Button type="link" size="small" icon={<EditOutlined />} onClick={() => openEdit(record)}>
            编辑
          </Button>
          <Button type="link" size="small" icon={<ThunderboltOutlined />} onClick={() => openRecommend(record)}>
            推荐参数
          </Button>
          <Button type="link" size="small" icon={<CheckCircleOutlined />} onClick={() => openApply(record)}>
            应用
          </Button>
          <Button type="link" size="small" danger icon={<DeleteOutlined />} onClick={() => handleDelete(record.id)}>
            删除
          </Button>
        </Space>
      ),
    },
  ]

  return (
    <div style={{ padding: '24px' }}>
      <Alert
        type="info"
        showIcon
        style={{ marginBottom: 16 }}
        message="操作说明"
        description={
          <div>
            <div>• <b>推荐参数</b>: 基于模板生成建议参数, 不实际修改实例。</div>
            <div>• <b>应用</b>: 将模板参数实际下发到目标实例, 标记为需重启的参数将导致实例重启。</div>
          </div>
        }
      />
      <Card
        title="参数模板"
        extra={
          <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>
            创建模板
          </Button>
        }
      >
        <Table columns={columns} dataSource={data} rowKey="id" loading={loading} />
      </Card>

      <Modal
        title={editing ? '编辑参数模板' : '创建参数模板'}
        open={modalOpen}
        onCancel={() => setModalOpen(false)}
        onOk={submit}
        confirmLoading={submitting}
        width={640}
      >
        <Form form={form} layout="vertical">
          <Form.Item name="name" label="模板名称" rules={[{ required: true, message: '请输入模板名称' }]}>
            <Input placeholder="例如: 生产-OLTP-标准" />
          </Form.Item>
          <Form.Item name="category" label="分类" rules={[{ required: true, message: '请选择分类' }]}>
            <Select
              options={[
                { value: 'oltp', label: 'OLTP 高并发' },
                { value: 'olap', label: 'OLAP 分析型' },
                { value: 'small', label: '小实例' },
                { value: 'medium', label: '中实例' },
                { value: 'large', label: '大实例' },
                { value: 'memory_optimized', label: '内存优化' },
                { value: 'custom', label: '自定义' },
              ]}
            />
          </Form.Item>
          <Form.Item name="description" label="描述">
            <Input.TextArea rows={2} />
          </Form.Item>
          <Form.Item
            name="parameters"
            label="参数 (JSON 格式)"
            validateStatus={paramJsonError ? 'error' : undefined}
            help={paramJsonError ? `JSON 解析失败: ${paramJsonError}` : '例如: {"innodb_buffer_pool_size": "4G", "max_connections": 1000}'}
          >
            <Input.TextArea
              rows={6}
              placeholder='{"innodb_buffer_pool_size": "4G", "max_connections": 1000}'
              // P2: 用户开始改时清掉旧解析错误, 避免 Form.Item 红色边框 +
              // help 红字 + 提交时再次显示错误, 三处红打架.
              onChange={() => paramJsonError && setParamJsonError(null)}
            />
          </Form.Item>
          {restartKeys.length > 0 && (
            <Alert
              type="warning"
              showIcon
              icon={<WarningOutlined />}
              message={`包含 ${restartKeys.length} 个需重启参数`}
              description={restartKeys.join(', ')}
            />
          )}
        </Form>
      </Modal>

      <Modal
        title={viewing ? `模板详情: ${viewing.name}` : '模板详情'}
        open={!!viewing}
        onCancel={() => setViewing(null)}
        footer={<Button onClick={() => setViewing(null)}>关闭</Button>}
        width={600}
      >
        {viewing && (
          <Descriptions bordered column={1}>
            <Descriptions.Item label="ID">{viewing.id}</Descriptions.Item>
            <Descriptions.Item label="名称">{viewing.name}</Descriptions.Item>
            <Descriptions.Item label="分类">{viewing.category}</Descriptions.Item>
            <Descriptions.Item label="是否预设">{viewing.is_preset ? '是' : '否'}</Descriptions.Item>
            <Descriptions.Item label="描述">{viewing.description || '-'}</Descriptions.Item>
            <Descriptions.Item label="参数">
              <pre style={{ margin: 0, whiteSpace: 'pre-wrap' }}>{viewing.parameters || '-'}</pre>
            </Descriptions.Item>
            <Descriptions.Item label="创建时间">
              {viewing.created_at ? new Date(viewing.created_at).toLocaleString() : '-'}
            </Descriptions.Item>
          </Descriptions>
        )}
      </Modal>

      <Modal
        title={`推荐参数: ${applying?.name || ''}`}
        open={recommendOpen}
        onCancel={() => setRecommendOpen(false)}
        onOk={submitRecommend}
        confirmLoading={submitting}
        okText="生成推荐"
        cancelText="取消"
        width={600}
      >
        <Form form={applyForm} layout="vertical">
          <Form.Item name="instance_id" label="目标实例" rules={[{ required: true, message: '请选择实例' }]}>
            <Select
              showSearch
              optionFilterProp="label"
              options={instances.map((i) => ({ value: i.id, label: i.name }))}
              placeholder="选择要推荐的实例"
            />
          </Form.Item>
          <Form.Item name="workload_type" label="工作负载类型" initialValue="oltp">
            <Select
              options={[
                { value: 'oltp', label: 'OLTP 高并发' },
                { value: 'olap', label: 'OLAP 分析' },
                { value: 'mixed', label: '混合负载' },
              ]}
            />
          </Form.Item>
        </Form>
        {recommendResult && (
          <Alert
            type="success"
            showIcon
            message="推荐已生成"
            description={
              <pre style={{ margin: 0, whiteSpace: 'pre-wrap', maxHeight: 300, overflow: 'auto' }}>
                {JSON.stringify(recommendResult, null, 2)}
              </pre>
            }
          />
        )}
      </Modal>

      <Modal
        title={`应用模板: ${applying?.name || ''}`}
        open={applyOpen}
        onCancel={() => setApplyOpen(false)}
        onOk={submitApply}
        confirmLoading={submitting}
        okText="确认应用"
        cancelText="取消"
        okButtonProps={{ danger: true }}
        width={600}
      >
        <Form form={applyForm} layout="vertical">
          <Form.Item name="instance_id" label="目标实例" rules={[{ required: true, message: '请选择实例' }]}>
            <Select
              showSearch
              optionFilterProp="label"
              options={instances.map((i) => ({ value: i.id, label: i.name }))}
              placeholder="选择要应用此模板的实例"
            />
          </Form.Item>
          <Alert
            type="warning"
            showIcon
            style={{ marginBottom: 12 }}
            message="操作警告"
            description={
              <ul style={{ marginBottom: 0, paddingLeft: 18 }}>
                <li>应用参数将<b>立即修改</b>实例的运行参数。</li>
                <li>标记为需重启的参数会导致实例短暂不可用, 影响业务。</li>
                <li>建议在业务低峰期执行, 并提前在测试环境验证。</li>
              </ul>
            }
          />
          {applying && (() => {
            try {
              const parsed = JSON.parse(applying.parameters || '{}')
              const keys = detectRestart(parsed)
              if (keys.length === 0) return null
              return (
                <Alert
                  type="error"
                  showIcon
                  message={`检测到 ${keys.length} 个需重启参数: ${keys.join(', ')}`}
                  description="应用此模板后实例将自动重启"
                />
              )
            } catch { return null }
          })()}
          <Form.Item name="require_restart" valuePropName="checked" initialValue={false}>
            <Checkbox>已确认需要重启实例, 同意执行</Checkbox>
          </Form.Item>
          <Form.Item name="confirm_restart" valuePropName="checked" rules={[{ required: true, message: '请确认' }]}>
            <Checkbox>我已了解参数影响, 确认应用</Checkbox>
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}

export default ParameterTemplateList
