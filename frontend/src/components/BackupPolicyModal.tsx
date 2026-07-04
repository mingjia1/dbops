import React from 'react'
import { Form, Input, InputNumber, Modal, Radio, Select, Switch } from 'antd'
import { ScheduleOutlined } from '@ant-design/icons'
import type { BackupPolicy } from '../services/backupHelpers'

interface BackupPolicyModalProps {
  open: boolean
  submitting: boolean
  editingPolicy: BackupPolicy | null
  form: any
  instanceOptions: { value: string; label: string }[]
  onOk: () => void
  onCancel: () => void
}

const BackupPolicyModal: React.FC<BackupPolicyModalProps> = ({
  open, submitting, editingPolicy, form, instanceOptions, onOk, onCancel,
}) => (
  <Modal
    title={editingPolicy ? '编辑备份策略' : '新建备份策略'}
    open={open}
    onCancel={onCancel}
    onOk={onOk}
    confirmLoading={submitting}
    width={620}
  >
    <Form form={form} layout="vertical">
      <Form.Item name="instance_id" label="目标实例" rules={[{ required: true }]}>
        <Select options={instanceOptions} />
      </Form.Item>
      <Form.Item name="backup_type" label="备份类型" rules={[{ required: true }]}>
        <Radio.Group>
          <Radio.Button value="full">全量</Radio.Button>
          <Radio.Button value="incremental">增量</Radio.Button>
          <Radio.Button value="logical">逻辑</Radio.Button>
        </Radio.Group>
      </Form.Item>
      <Form.Item name="schedule" label={<span><ScheduleOutlined /> Cron 表达式</span>} rules={[{ required: true }]}>
        <Input placeholder="0 2 * * *" />
      </Form.Item>
      <Form.Item name="retention_days" label="保留天数" rules={[{ required: true }]}>
        <InputNumber min={1} max={3650} style={{ width: '100%' }} />
      </Form.Item>
      <Form.Item name="storage_type" label="存储类型">
        <Radio.Group>
          <Radio.Button value="local">本地</Radio.Button>
          <Radio.Button value="nfs">NFS</Radio.Button>
          <Radio.Button value="s3">S3</Radio.Button>
        </Radio.Group>
      </Form.Item>
      <Form.Item name="storage_path" label="存储路径">
        <Input placeholder="/backup/mysql" />
      </Form.Item>
      <Form.Item name="enabled" label="启用" valuePropName="checked">
        <Switch checkedChildren="启用" unCheckedChildren="禁用" />
      </Form.Item>
    </Form>
  </Modal>
)

export default BackupPolicyModal
