import React from 'react'
import { Form, Select, Modal, Card, Tag, Typography } from 'antd'

const { Paragraph } = Typography

interface CompatCheckModalProps {
  open: boolean
  submitting: boolean
  instanceOptions: { value: string; label: string }[]
  versionOptions: { value: string; label: string }[]
  compatResult: any | null
  versionInfo: React.ReactNode
  form: any
  onCancel: () => void
  onFinish: (values: any) => void
}

const CompatCheckModal: React.FC<CompatCheckModalProps> = ({
  open, submitting, instanceOptions, versionOptions, compatResult,
  versionInfo, form, onCancel, onFinish,
}) => {
  return (
    <Modal
      title="兼容性检查"
      open={open}
      onCancel={() => { onCancel(); form.resetFields() }}
      onOk={() => form.submit()}
      confirmLoading={submitting}
      width={760}
    >
      <Form form={form} layout="vertical" onFinish={onFinish}>
        <Form.Item name="instance_id" label="目标实例" rules={[{ required: true, message: '请选择实例' }]}>
          <Select showSearch optionFilterProp="label" options={instanceOptions} placeholder="选择实例" />
        </Form.Item>
        {versionInfo}
        <Form.Item name="target_version" label="目标版本" rules={[{ required: true, message: '请选择目标版本' }]}>
          <Select showSearch optionFilterProp="label" options={versionOptions} placeholder="选择目标版本" />
        </Form.Item>
      </Form>
      {compatResult && (
        <Card size="small" title="检查结果">
          <Tag color={compatResult.is_compatible ? 'success' : 'error'}>
            {compatResult.is_compatible ? '兼容' : '不兼容'}
          </Tag>
          <Paragraph style={{ marginTop: 12 }}>错误: {compatResult.error_count || 0}，警告: {compatResult.warning_count || 0}</Paragraph>
          {(compatResult.incompatibilities || []).map((item: any, index: number) => (
            <div key={index} style={{ marginTop: 8, padding: 8, background: item.level === 'error' ? '#fff2f0' : '#fffbe6', border: `1px solid ${item.level === 'error' ? '#ffccc7' : '#ffe58f'}`, borderRadius: 4 }}>
              <div style={{ fontWeight: 500 }}>{item.description}</div>
              {item.solution && <div style={{ fontSize: 12, color: '#666', marginTop: 4 }}>解决方案: {item.solution}</div>}
            </div>
          ))}
        </Card>
      )}
    </Modal>
  )
}

export default CompatCheckModal
