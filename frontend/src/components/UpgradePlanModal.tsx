import React from 'react'
import { Form, Select, Switch, DatePicker, Modal, Card, Tag, Spin, Empty, Typography } from 'antd'

const { Paragraph } = Typography
import { strategyOptions } from '../services/upgradeHelpers'

interface UpgradePlanModalProps {
  open: boolean
  submitting: boolean
  versionsLoading: boolean
  instanceOptions: { value: string; label: string }[]
  versionOptions: { value: string; label: string }[]
  planResult: any | null
  versionInfo: React.ReactNode
  form: any
  onCancel: () => void
  onFinish: (values: any) => void
}

const UpgradePlanModal: React.FC<UpgradePlanModalProps> = ({
  open, submitting, versionsLoading, instanceOptions, versionOptions,
  planResult, versionInfo, form, onCancel, onFinish,
}) => {

  return (
    <Modal
      title="规划升级路径"
      open={open}
      onCancel={onCancel}
      onOk={() => form.submit()}
      confirmLoading={submitting}
      width={720}
    >
      <Form form={form} layout="vertical" onFinish={onFinish} initialValues={{ check_data_exists: true, backup_confirmed: false, strategy: 'inplace' }}>
        <Form.Item name="instance_id" label="目标实例" rules={[{ required: true, message: '请选择实例' }]}>
          <Select showSearch optionFilterProp="label" options={instanceOptions} placeholder="选择实例" />
        </Form.Item>
        {versionInfo}
        <Form.Item name="target_version" label="目标版本" rules={[{ required: true, message: '请选择目标版本' }]}>
          <Select
            showSearch optionFilterProp="label" loading={versionsLoading}
            notFoundContent={versionsLoading ? <Spin size="small" /> : <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} />}
            options={versionOptions} placeholder="选择目标版本"
          />
        </Form.Item>
        <Form.Item name="strategy" label="升级策略" rules={[{ required: true }]}>
          <Select options={strategyOptions} />
        </Form.Item>
        <Form.Item name="check_data_exists" label="检测是否存在数据" valuePropName="checked">
          <Switch checkedChildren="检测" unCheckedChildren="跳过" />
        </Form.Item>
        <Form.Item name="backup_confirmed" label="数据是否已备份" valuePropName="checked">
          <Switch checkedChildren="已备份" unCheckedChildren="未备份" />
        </Form.Item>
        <Form.Item name="backup_method" label="备份方式">
          <Select allowClear options={[
            { value: 'full', label: '全量备份' },
            { value: 'incremental', label: '增量备份' },
            { value: 'external', label: '外部备份已完成' },
          ]} />
        </Form.Item>
        <Form.Item name="scheduled_time" label="计划执行时间">
          <DatePicker showTime style={{ width: '100%' }} />
        </Form.Item>
      </Form>
      {planResult && (
        <Card size="small" title="规划结果">
          <Paragraph>源版本: {planResult.source_flavor} {planResult.source_version}</Paragraph>
          <Paragraph>目标版本: {planResult.target_flavor} {planResult.target_version}</Paragraph>
          <Paragraph>风险等级: <Tag>{planResult.risk_level}</Tag></Paragraph>
          <Paragraph>预计耗时: {planResult.estimated_time || '-'} 分钟</Paragraph>
        </Card>
      )}
    </Modal>
  )
}

export default UpgradePlanModal
