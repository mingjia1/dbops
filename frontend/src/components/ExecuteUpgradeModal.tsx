import React from 'react'
import { Form, Select, Input, InputNumber, Switch, Modal, Row, Col } from 'antd'
import type { VersionEntry } from '../services/api'
import { strategyOptions } from '../services/upgradeHelpers'

interface ExecuteUpgradeModalProps {
  open: boolean
  submitting: boolean
  instanceOptions: { value: string; label: string }[]
  clusterOptions: { value: string; label: string }[]
  versionOptions: { value: string; label: string }[]
  versionsLoading: boolean
  versionInfo: React.ReactNode
  form: any
  onCancel: () => void
  onFinish: (values: any) => void
}

const ExecuteUpgradeModal: React.FC<ExecuteUpgradeModalProps> = ({
  open, submitting, instanceOptions, clusterOptions, versionOptions,
  versionsLoading, versionInfo, form, onCancel, onFinish,
}) => {
  const executeStrategy = Form.useWatch('strategy', form)

  return (
    <Modal
      title="启动升级任务"
      open={open}
      onCancel={() => { onCancel(); form.resetFields() }}
      onOk={() => form.submit()}
      confirmLoading={submitting}
      okButtonProps={{ danger: true }}
      width={720}
    >
      <Form form={form} layout="vertical" onFinish={onFinish} initialValues={{
        backup_enabled: false, strategy: 'inplace', parallelism: 4,
        batch_size: 1000, max_in_parallel: 1, health_check_interval: 30,
      }}>
        <Form.Item name="strategy" label="升级策略" rules={[{ required: true }]}>
          <Select options={strategyOptions} />
        </Form.Item>
        {executeStrategy === 'rolling' ? (
          <Form.Item name="cluster_id" label="目标集群" rules={[{ required: true, message: '请选择集群' }]}>
            <Select showSearch optionFilterProp="label" options={clusterOptions} placeholder="选择集群" />
          </Form.Item>
        ) : (
          <>
            <Form.Item name="instance_id" label="目标实例" rules={[{ required: true, message: '请选择实例' }]}>
              <Select showSearch optionFilterProp="label" options={instanceOptions} placeholder="选择实例" />
            </Form.Item>
            {versionInfo}
          </>
        )}
        <Form.Item name="plan_id" label="升级计划ID" rules={[{ required: true, message: '请输入规划后生成的计划ID' }]}>
          <Input placeholder="先规划升级路径，再填写计划ID" />
        </Form.Item>
        <Form.Item name="target_version" label="目标版本" rules={[{ required: true, message: '请选择目标版本' }]}>
          <Select showSearch optionFilterProp="label" loading={versionsLoading} options={versionOptions} placeholder="选择目标版本" />
        </Form.Item>
        <Form.Item name="backup_enabled" label="数据是否已备份" valuePropName="checked">
          <Switch checkedChildren="已备份" unCheckedChildren="未备份" />
        </Form.Item>
        {executeStrategy === 'logical' && (
          <>
            <Form.Item name="parallelism" label="并行度">
              <InputNumber min={1} max={32} style={{ width: '100%' }} />
            </Form.Item>
            <Form.Item name="batch_size" label="批次大小">
              <InputNumber min={100} max={100000} style={{ width: '100%' }} />
            </Form.Item>
          </>
        )}
        {executeStrategy === 'rolling' ? (
          <>
            <Form.Item name="max_in_parallel" label="最大并行实例数">
              <InputNumber min={1} max={8} style={{ width: '100%' }} />
            </Form.Item>
            <Form.Item name="health_check_interval" label="健康检查间隔(秒)">
              <InputNumber min={5} max={600} style={{ width: '100%' }} />
            </Form.Item>
          </>
        ) : (
          <Form.Item name="stop_app_timeout" label="停止应用超时(秒)" initialValue={300}>
            <InputNumber min={30} max={3600} style={{ width: '100%' }} />
          </Form.Item>
        )}
      </Form>
    </Modal>
  )
}

export default ExecuteUpgradeModal
