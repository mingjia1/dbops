import React from 'react'
import { Form, Select, Button, Input, InputNumber, Row, Col } from 'antd'
import { PlayCircleOutlined, SyncOutlined } from '@ant-design/icons'

export type MigrationType = 'physical' | 'replication' | 'gtid'

interface MigrationFormSectionProps {
  type: MigrationType
  instances: any[]
  loading: boolean
  onSubmit: (values: any) => void
}

const iconMap: Record<MigrationType, React.ReactNode> = {
  physical: <PlayCircleOutlined />,
  replication: <SyncOutlined />,
  gtid: <PlayCircleOutlined />,
}

const buttonTextMap: Record<MigrationType, string> = {
  physical: '启动物理迁移',
  replication: '启动复制迁移',
  gtid: '启动GTID迁移',
}

const PhysicalFields: React.FC = () => (
  <>
    <Col xs={24} md={8}>
      <Form.Item name="compress" label="压缩方式" initialValue="gzip">
        <Select>
          <Select.Option value="gzip">gzip</Select.Option>
          <Select.Option value="lz4">lz4</Select.Option>
          <Select.Option value="none">不压缩</Select.Option>
        </Select>
      </Form.Item>
    </Col>
    <Col xs={24} md={8}>
      <Form.Item name="parallel_threads" label="并行线程数" initialValue={4}>
        <InputNumber min={1} max={16} style={{ width: '100%' }} />
      </Form.Item>
    </Col>
  </>
)

const ReplicationFields: React.FC = () => (
  <>
    <Col xs={24} md={8}>
      <Form.Item name="replication_user" label="复制用户" rules={[{ required: true }]}>
        <Input placeholder="repl_user" />
      </Form.Item>
    </Col>
    <Col xs={24} md={8}>
      <Form.Item name="replication_password" label="复制密码" rules={[{ required: true }]}>
        <Input.Password placeholder="输入密码" />
      </Form.Item>
    </Col>
    <Col xs={24} md={8}>
      <Form.Item name="sync_delay_threshold" label="延迟阈值" initialValue={10}>
        <InputNumber min={0} max={3600} style={{ width: '100%' }} />
      </Form.Item>
    </Col>
  </>
)

const GTIDFields: React.FC = () => (
  <>
    <Col xs={24} md={8}>
      <Form.Item name="gtid_purged" label="清除GTID">
        <Input placeholder="GTID集合(可选)" />
      </Form.Item>
    </Col>
    <Col xs={24} md={8}>
      <Form.Item name="gtid_executed" label="执行GTID">
        <Input placeholder="GTID集合(可选)" />
      </Form.Item>
    </Col>
    <Col xs={24} md={8}>
      <Form.Item name="transaction_batch_size" label="事务批次" initialValue={100}>
        <InputNumber min={10} max={10000} style={{ width: '100%' }} />
      </Form.Item>
    </Col>
  </>
)

const fieldComponents: Record<MigrationType, React.FC> = {
  physical: PhysicalFields,
  replication: ReplicationFields,
  gtid: GTIDFields,
}

const MigrationFormSection: React.FC<MigrationFormSectionProps> = ({ type, instances, loading, onSubmit }) => {
  const [form] = Form.useForm()
  const FieldsComponent = fieldComponents[type]

  return (
    <Form form={form} layout="vertical" onFinish={onSubmit}>
      <Row gutter={16}>
        <Col xs={24} md={8}>
          <Form.Item name="source_instance" label="源实例" rules={[{ required: true, message: '请选择源实例' }]}>
            <Select placeholder="选择源实例" options={instances.map((i: any) => ({ label: i.name, value: i.id }))} />
          </Form.Item>
        </Col>
        <Col xs={24} md={8}>
          <Form.Item name="target_instance" label="目标实例" rules={[{ required: true, message: '请选择目标实例' }]}>
            <Select placeholder="选择目标实例" options={instances.map((i: any) => ({ label: i.name, value: i.id }))} />
          </Form.Item>
        </Col>
        <FieldsComponent />
        <Col xs={24} md={8}>
          <Form.Item label=" ">
            <Button type="primary" icon={iconMap[type]} htmlType="submit" loading={loading}>
              {buttonTextMap[type]}
            </Button>
          </Form.Item>
        </Col>
      </Row>
    </Form>
  )
}

export default MigrationFormSection
