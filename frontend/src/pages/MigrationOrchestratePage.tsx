import React, { useState } from 'react'
import { Card, Button, Select, Space, Alert, message, Steps, Tag, Form, Input } from 'antd'
import { SwapOutlined, CheckCircleOutlined } from '@ant-design/icons'
import api from '../services/api'

export default function MigrationOrchestratePage() {
  const [step, setStep] = useState(0)
  const [loading, setLoading] = useState(false)
  const [result, setResult] = useState<any>(null)
  const [form] = Form.useForm()

  const handleOrchestrate = async (values: any) => {
    setLoading(true)
    try {
      const res = await api.post('/migrations/orchestrate', values)
      setResult(res.data?.data)
      message.success('迁移编排任务已提交')
      setStep(2)
    } catch (err: any) {
      message.error(`编排失败: ${err.message}`)
    } finally { setLoading(false) }
  }

  const steps = [
    { title: '配置', description: '配置迁移参数' },
    { title: '确认', description: '确认执行' },
    { title: '完成', description: '任务已提交' },
  ]

  return (
    <div style={{ padding: 24 }}>
      <Card title={<><SwapOutlined /> 数据迁移编排</>}>
        <Steps current={step} items={steps} style={{ marginBottom: 24 }} />

        {step === 0 && (
          <Form form={form} layout="vertical" onFinish={() => setStep(1)} style={{ maxWidth: 500 }}>
            <Form.Item name="source_instance_id" label="源实例 ID" rules={[{ required: true }]}><Input /></Form.Item>
            <Form.Item name="target_instance_id" label="目标实例 ID" rules={[{ required: true }]}><Input /></Form.Item>
            <Form.Item name="strategy" label="迁移策略" rules={[{ required: true }]}>
              <Select options={[
                { label: '物理迁移 (XtraBackup)', value: 'physical' },
                { label: '逻辑迁移 (mysqldump)', value: 'logical' },
                { label: 'GTID 复制迁移', value: 'gtid' },
              ]} />
            </Form.Item>
            <Form.Item name="parallel_threads" label="并行线程数" initialValue={4}><Input type="number" /></Form.Item>
            <Button type="primary" htmlType="submit">下一步</Button>
          </Form>
        )}

        {step === 1 && (
          <Space direction="vertical" size="middle" style={{ width: '100%' }}>
            <Alert type="warning" message="确认迁移编排"
              description="将按照指定策略执行完整的数据迁移流程：导出 → 验证 → 切换。请确保源和目标实例可连接。" />
            <Space>
              <Button onClick={() => setStep(0)}>返回</Button>
              <Button type="primary" loading={loading} onClick={() => form.validateFields().then(handleOrchestrate)}>确认执行</Button>
            </Space>
          </Space>
        )}

        {step === 2 && (
          <Alert type="success" showIcon icon={<CheckCircleOutlined />}
            message="迁移编排任务已提交" description="请在数据迁移页面查看进度。" />
        )}
      </Card>
    </div>
  )
}
