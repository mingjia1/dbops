import React, { useState, useEffect } from 'react'
import { Card, Row, Col, Statistic, Table, Select, Button } from 'antd'
import { LineChart, Line, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer } from 'recharts'
import { monitorApi } from '@/services/api'

interface MetricData {
  name: string
  value: number
  timestamp: string
}

const MonitorDashboard: React.FC = () => {
  const [metrics, setMetrics] = useState<MetricData[]>([])
  const [instanceId, setInstanceId] = useState<string>('instance-001')
  const [loading, setLoading] = useState(false)

  useEffect(() => {
    fetchMetrics()
  }, [instanceId])

  const fetchMetrics = async () => {
    setLoading(true)
    try {
      const response = await monitorApi.queryMetrics(instanceId)
      setMetrics(response.data)
    } catch (err) {
      console.error('Failed to fetch metrics')
    } finally {
      setLoading(false)
    }
  }

  const chartData = metrics.map((m) => ({
    name: m.name,
    value: m.value,
    time: new Date(m.timestamp).toLocaleTimeString(),
  }))

  const qps = metrics.find((m) => m.name === 'qps')?.value || 0
  const tps = metrics.find((m) => m.name === 'tps')?.value || 0
  const connections = metrics.find((m) => m.name === 'threads_connected')?.value || 0
  const slowQueries = metrics.find((m) => m.name === 'slow_queries')?.value || 0

  const columns = [
    {
      title: '指标',
      dataIndex: 'name',
      key: 'name',
    },
    {
      title: '值',
      dataIndex: 'value',
      key: 'value',
      render: (value: number) => value.toFixed(2),
    },
    {
      title: '时间',
      dataIndex: 'timestamp',
      key: 'timestamp',
      render: (timestamp: string) => new Date(timestamp).toLocaleString(),
    },
  ]

  return (
    <div>
      <Card style={{ marginBottom: 16 }}>
        <Row gutter={16}>
          <Col span={4}>
            <Statistic title="实例" value={instanceId} />
          </Col>
          <Col span={4}>
            <Statistic title="QPS" value={qps} precision={2} />
          </Col>
          <Col span={4}>
            <Statistic title="TPS" value={tps} precision={2} />
          </Col>
          <Col span={4}>
            <Statistic title="连接数" value={connections} />
          </Col>
          <Col span={4}>
            <Statistic title="慢查询" value={slowQueries} />
          </Col>
          <Col span={4}>
            <Select
              value={instanceId}
              onChange={setInstanceId}
              style={{ width: '100%' }}
              options={[
                { value: 'instance-001', label: 'instance-001' },
                { value: 'instance-002', label: 'instance-002' },
              ]}
            />
          </Col>
        </Row>
      </Card>

      <Row gutter={16}>
        <Col span={12}>
          <Card title="指标趋势">
            <ResponsiveContainer height={300}>
              <LineChart data={chartData}>
                <CartesianGrid strokeDasharray="3 3" />
                <XAxis dataKey="time" />
                <YAxis />
                <Tooltip />
                <Line type="monotone" dataKey="value" stroke="#1890ff" />
              </LineChart>
            </ResponsiveContainer>
          </Card>
        </Col>
        <Col span={12}>
          <Card title="指标详情">
            <Table
              columns={columns}
              dataSource={metrics}
              rowKey="name"
              loading={loading}
              pagination={false}
              size="small"
            />
          </Card>
        </Col>
      </Row>
    </div>
  )
}

export default MonitorDashboard