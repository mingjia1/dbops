import React, { useState } from 'react'
import { Card, Button, Select, Space, Steps, message, Alert, Tag } from 'antd'
import { ArrowUpOutlined, CheckCircleOutlined } from '@ant-design/icons'
import api from '../services/api'

type ArchTarget = 'mha' | 'mgr' | 'pxc'

export default function ArchUpgradePage() {
  const [targetArch, setTargetArch] = useState<ArchTarget>('mha')
  const [step, setStep] = useState(0)
  const [loading, setLoading] = useState(false)
  const [result, setResult] = useState<any>(null)

  const handleUpgrade = async () => {
    setLoading(true)
    try {
      const res = await api.post(`/switch/single-to-${targetArch}`)
      setResult(res.data?.data)
      message.success(`已升级为 ${targetArch.toUpperCase()} 架构`)
      setStep(2)
    } catch (err: any) {
      message.error(`升级失败: ${err.message}`)
    } finally { setLoading(false) }
  }

  const steps = [
    { title: '选择目标架构', description: '选择要升级的 HA 架构' },
    { title: '确认', description: '确认升级操作' },
    { title: '完成', description: '升级完成' },
  ]

  return (
    <div style={{ padding: 24 }}>
      <Card title={<><ArrowUpOutlined /> 单实例架构升级</>}>
        <Steps current={step} items={steps} style={{ marginBottom: 24 }} />

        {step === 0 && (
          <Space direction="vertical" size="middle" style={{ width: '100%' }}>
            <div>选择目标 HA 架构类型：</div>
            <Select style={{ width: '100%' }} value={targetArch} onChange={setTargetArch} options={[
              { label: 'MHA (Master High Availability) — 传统主从+自动故障切换', value: 'mha' },
              { label: 'MGR (MySQL Group Replication) — 多主/单主组复制', value: 'mgr' },
              { label: 'PXC (Percona XtraDB Cluster) — Galera 多主集群', value: 'pxc' },
            ]} />
            <Alert type="info" message="此操作将当前单实例升级为选定的 HA 架构，自动添加从节点并建立复制关系。" />
            <Button type="primary" onClick={() => setStep(1)}>下一步</Button>
          </Space>
        )}

        {step === 1 && (
          <Space direction="vertical" size="middle" style={{ width: '100%' }}>
            <Alert type="warning" message="确认升级" description={`将升级为 ${targetArch.toUpperCase()} 架构，此操作将添加从节点并建立复制。`} />
            <Space>
              <Button onClick={() => setStep(0)}>返回</Button>
              <Button type="primary" loading={loading} onClick={handleUpgrade}>确认升级</Button>
            </Space>
          </Space>
        )}

        {step === 2 && (
          <Space direction="vertical" size="middle" style={{ width: '100%' }}>
            <Alert type="success" showIcon icon={<CheckCircleOutlined />}
              message={`成功升级为 ${targetArch.toUpperCase()} 架构`} />
            {result && (
              <Card type="inner" title="升级结果" size="small">
                <pre style={{ maxHeight: 300, overflow: 'auto' }}>{JSON.stringify(result, null, 2)}</pre>
              </Card>
            )}
          </Space>
        )}
      </Card>
    </div>
  )
}
