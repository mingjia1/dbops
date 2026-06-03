import React, { useEffect, useState } from 'react'
import { Card, Row, Col, Statistic, Select, Spin } from 'antd'
import {
  DatabaseOutlined,
  CheckCircleOutlined,
  CloseCircleOutlined,
  AlertOutlined,
} from '@ant-design/icons'
import { instanceApi, monitorApi } from '../services/api'

const MonitorDashboard: React.FC = () => {
  const [instances, setInstances] = useState<any[]>([])
  const [selectedInstance, setSelectedInstance] = useState<string | undefined>(undefined)
  const [metrics, setMetrics] = useState<any>(null)
  const [loading, setLoading] = useState(false)
  const [stats, setStats] = useState({ total: 0, running: 0, stopped: 0, abnormal: 0 })

  useEffect(() => {
    instanceApi.list(100, 0).then((res: any) => {
      const list = res?.data || []
      setInstances(list)
      setStats({
        total: list.length,
        running: list.filter((i: any) => i.status === 'running').length,
        stopped: list.filter((i: any) => i.status === 'stopped').length,
        abnormal: list.filter((i: any) => i.status === 'abnormal').length,
      })
    }).catch(() => {})
  }, [])

  useEffect(() => {
    if (!selectedInstance) return
    setLoading(true)
    monitorApi.queryMetrics(selectedInstance).then((res: any) => {
      setMetrics(res?.data || null)
    }).catch(() => {}).finally(() => setLoading(false))
  }, [selectedInstance])

  return (
    <div style={{ padding: '24px' }}>
      <Row gutter={[16, 16]}>
        <Col span={6}>
          <Card>
            <Statistic title="总实例数" value={stats.total} prefix={<DatabaseOutlined />} />
          </Card>
        </Col>
        <Col span={6}>
          <Card>
            <Statistic title="运行中" value={stats.running} prefix={<CheckCircleOutlined />} valueStyle={{ color: '#3f8600' }} />
          </Card>
        </Col>
        <Col span={6}>
          <Card>
            <Statistic title="已停止" value={stats.stopped} prefix={<CloseCircleOutlined />} valueStyle={{ color: '#8c8c8c' }} />
          </Card>
        </Col>
        <Col span={6}>
          <Card>
            <Statistic title="异常" value={stats.abnormal} prefix={<AlertOutlined />} valueStyle={{ color: '#cf1322' }} />
          </Card>
        </Col>
      </Row>
      <Card
        title="监控图表"
        style={{ marginTop: 16 }}
        extra={
          <Select
            placeholder="选择实例查看指标"
            style={{ width: 200 }}
            allowClear
            value={selectedInstance}
            onChange={setSelectedInstance}
            options={instances.map((i: any) => ({ label: i.name, value: i.id }))}
          />
        }
      >
        {loading ? (
          <div style={{ height: 300, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
            <Spin tip="加载中..." />
          </div>
        ) : metrics ? (
          <pre style={{ maxHeight: 300, overflow: 'auto', margin: 0 }}>{JSON.stringify(metrics, null, 2)}</pre>
        ) : (
          <div style={{ height: 300, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
            <span style={{ color: '#8c8c8c' }}>请选择一个实例查看监控指标</span>
          </div>
        )}
      </Card>
    </div>
  )
}

export default MonitorDashboard