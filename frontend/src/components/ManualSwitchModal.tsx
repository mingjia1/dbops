import React from 'react'
import { Alert, Button, Form, FormInstance, Input, Modal, Select, Switch } from 'antd'
import { instanceEndpoint } from '../services/haHelpers'
import type { Instance } from '../services/api'

interface ManualSwitchModalProps {
  open: boolean
  confirming: boolean
  form: FormInstance
  slaveInstances: Instance[]
  clusterIsMGR: boolean
  clusterIsPXC: boolean
  forceMode: boolean | undefined
  preflightPass: boolean
  onOk: () => void
  onCancel: () => void
  onPreflight: () => void
}

const archTitle = (isPXC: boolean, isMGR: boolean): string => {
  if (isPXC) return 'PXC 集群角色切换'
  if (isMGR) return 'MGR 集群角色切换'
  return '手动主从切换'
}

const archMessage = (isPXC: boolean, isMGR: boolean): string => {
  if (isPXC) return 'PXC 集群将通过角色切换接口完成主从切换'
  if (isMGR) return 'MGR 集群将通过角色切换接口完成主从切换'
  return '手动切换需要选择一个非主节点作为新主'
}

const archDescription = (isPXC: boolean, isMGR: boolean): string => {
  if (isPXC || isMGR) return '选择一个 secondary 节点，系统将通过角色切换接口将其提升为 primary。'
  return '未选择强制模式时，Pre-flight 通过后即可切换。'
}

export const ManualSwitchModal: React.FC<ManualSwitchModalProps> = ({
  open, confirming, form, slaveInstances,
  clusterIsPXC, clusterIsMGR, forceMode, preflightPass,
  onOk, onCancel, onPreflight,
}) => (
  <Modal
    title={archTitle(clusterIsPXC, clusterIsMGR)}
    open={open}
    onCancel={onCancel}
    onOk={onOk}
    confirmLoading={confirming}
    okText="执行"
    cancelText="取消"
    okButtonProps={{ danger: true }}
  >
    <Form form={form} layout="vertical">
      <Alert
        type="error"
        showIcon
        message={archMessage(clusterIsPXC, clusterIsMGR)}
        description={archDescription(clusterIsPXC, clusterIsMGR)}
        style={{ marginBottom: 12 }}
      />
      <Form.Item
        name="new_master_id"
        label={clusterIsPXC ? '目标节点' : '新主实例'}
        rules={[{ required: true, message: clusterIsPXC ? '请选择目标节点' : '请选择新主实例' }]}
      >
        <Select
          placeholder={clusterIsPXC ? '选择 secondary 节点' : '选择非主节点'}
          options={slaveInstances.map((instance) => ({
            value: instance.id,
            label: `${instance.name} (${instanceEndpoint(instance)}, 延迟: ${instance.status?.seconds_behind_master ?? '-'}s)`,
          }))}
        />
      </Form.Item>
      <Form.Item name="reason" label="切换原因" rules={[{ required: true, message: '请输入切换原因' }]}>
        <Input placeholder="例如：计划内维护" />
      </Form.Item>
      <Form.Item name="force" label="强制执行" valuePropName="checked">
        <Switch checkedChildren="强制" unCheckedChildren="安全模式" />
      </Form.Item>
      {forceMode ? (
        <Alert type="warning" showIcon message="强制模式会绕过 Pre-flight 阻断项，请确认数据风险。" style={{ marginBottom: 12 }} />
      ) : (
        <Alert
          type={preflightPass ? 'success' : 'info'}
          showIcon
          message={preflightPass ? 'Pre-flight 已通过' : '非强制模式需要先通过 Pre-flight 检查'}
          action={<Button size="small" onClick={onPreflight}>检查</Button>}
          style={{ marginBottom: 12 }}
        />
      )}
      <Form.Item name="confirm_impact" valuePropName="checked" rules={[{ required: true, message: '请确认操作影响' }]}>
        <Switch checkedChildren="已确认影响" unCheckedChildren="未确认" />
      </Form.Item>
    </Form>
  </Modal>
)
