import React, { useState, useEffect } from 'react'
import { Card, Table, Button, Select, Space, Tag, message, Alert, Descriptions, Tabs, Input, Form } from 'antd'
import { SettingOutlined, CheckCircleOutlined, StarOutlined, ExperimentOutlined } from '@ant-design/icons'
import api from '../services/api'

interface PresetTemplate {
  id: string
  name: string
  description: string
  category: string
}

interface ValidationResult {
  valid: boolean
  errors: string[]
  warnings: string[]
}

export default function ParameterEnhancedPage() {
  const [presets, setPresets] = useState<PresetTemplate[]>([])
  const [loading, setLoading] = useState(false)
  const [selectedPreset, setSelectedPreset] = useState<string>('')
  const [parameters, setParameters] = useState<any[]>([])
  const [validationResult, setValidationResult] = useState<ValidationResult | null>(null)
  const [validating, setValidating] = useState(false)
  const [instanceID, setInstanceID] = useState('')

  const fetchPresets = async () => {
    setLoading(true)
    try {
      const res = await api.get('/parameter-templates/presets')
      setPresets(res.data?.data || [])
    } catch {}
    finally { setLoading(false) }
  }
  useEffect(() => { fetchPresets() }, [])

  const fetchParameters = async (templateID: string) => {
    try {
      const res = await api.get(`/parameter-templates/${templateID}/parameters`)
      setParameters(res.data?.data || [])
    } catch { message.error('获取参数失败') }
  }

  const handleValidate = async (templateID: string) => {
    setValidating(true)
    try {
      const res = await api.post(`/parameter-templates/${templateID}/validate`, { instance_id: instanceID || undefined })
      setValidationResult(res.data?.data || null)
    } catch (err: any) { message.error(err.message) }
    finally { setValidating(false) }
  }

  const presetCols = [
    { title: '名称', dataIndex: 'name', key: 'name' },
    { title: '类别', dataIndex: 'category', key: 'category', render: (v: string) => <Tag>{v}</Tag> },
    { title: '描述', dataIndex: 'description', key: 'description', ellipsis: true },
    { title: '操作', key: 'action', render: (_: any, r: PresetTemplate) => (
      <Space>
        <Button size="small" onClick={() => { setSelectedPreset(r.id); fetchParameters(r.id) }}>查看参数</Button>
        <Button size="small" icon={<ExperimentOutlined />} loading={validating}
          onClick={() => { setSelectedPreset(r.id); handleValidate(r.id) }}>验证</Button>
      </Space>
    )},
  ]

  const paramCols = [
    { title: '参数名', dataIndex: 'parameter_name', key: 'parameter_name' },
    { title: '值', dataIndex: 'value', key: 'value', render: (v: string) => <Tag color="blue">{v}</Tag> },
    { title: '类型', dataIndex: 'data_type', key: 'data_type' },
    { title: '必填', dataIndex: 'is_mandatory', key: 'is_mandatory', render: (v: boolean) => v ? <Tag color="red">是</Tag> : <Tag>否</Tag> },
    { title: '动态', dataIndex: 'is_dynamic', key: 'is_dynamic', render: (v: boolean) => v ? <Tag color="green">是</Tag> : <Tag>否</Tag> },
    { title: '描述', dataIndex: 'description', key: 'description', ellipsis: true },
  ]

  return (
    <div style={{ padding: 24 }}>
      <Card title={<><SettingOutlined /> 参数模板增强</>}>
        <Tabs items={[
          { key: 'presets', label: '预设模板', children: (
            <>
              <Table columns={presetCols} dataSource={presets} rowKey="id" loading={loading} size="small" />
            </>
          )},
          { key: 'parameters', label: '模板参数', children: (
            <>
              {selectedPreset ? (
                <>
                  <Descriptions size="small" bordered style={{ marginBottom: 16 }}>
                    <Descriptions.Item label="模板 ID">{selectedPreset}</Descriptions.Item>
                    <Descriptions.Item label="参数数量">{parameters.length}</Descriptions.Item>
                  </Descriptions>
                  <Table columns={paramCols} dataSource={parameters} rowKey="parameter_name" size="small" />
                </>
              ) : (
                <Alert type="info" message="请先在预设模板列表中选择一个模板查看参数" />
              )}
            </>
          )},
          { key: 'validate', label: '参数验证', children: (
            <Space direction="vertical" size="middle" style={{ width: '100%' }}>
              <Space>
                <Select style={{ width: 300 }} placeholder="选择模板" value={selectedPreset || undefined}
                  onChange={(v) => setSelectedPreset(v)}
                  options={presets.map(p => ({ label: p.name, value: p.id }))} />
                <Input placeholder="实例 ID (可选)" value={instanceID} onChange={e => setInstanceID(e.target.value)} style={{ width: 200 }} />
                <Button type="primary" icon={<CheckCircleOutlined />} loading={validating}
                  disabled={!selectedPreset}
                  onClick={() => handleValidate(selectedPreset)}>验证参数</Button>
              </Space>
              {validationResult && (
                <Card type="inner" title="验证结果">
                  {validationResult.valid ? (
                    <Alert type="success" showIcon message="参数验证通过" />
                  ) : (
                    <Alert type="error" showIcon message="参数验证失败"
                      description={validationResult.errors.join('; ')} />
                  )}
                  {validationResult.warnings.length > 0 && (
                    <Alert type="warning" showIcon message="警告"
                      description={validationResult.warnings.join('; ')} style={{ marginTop: 8 }} />
                  )}
                </Card>
              )}
            </Space>
          )},
        ]} />
      </Card>
    </div>
  )
}
