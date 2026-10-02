import React, { useEffect, useState } from 'react'
import { Alert, Button, Card, Col, Row, Space, Statistic, Tag, Typography } from 'antd'
import { CheckCircleOutlined, CloseCircleOutlined, DatabaseOutlined, ThunderboltOutlined } from '@ant-design/icons'
import api from '../services/api'

const { Text } = Typography

interface HealthStatus {
  name: string
  status: 'healthy' | 'degraded' | 'down' | 'unknown'
  detail?: string
  latencyMs?: number
}

const PlatformHealth: React.FC = () => {
  const [loading, setLoading] = useState(false)
  const [checks, setChecks] = useState<HealthStatus[]>([])
  const [lastChecked, setLastChecked] = useState<string | null>(null)

  const runChecks = async () => {
    setLoading(true)
    const results: HealthStatus[] = []
    const base = api.defaults.baseURL || window.location.origin

    const check = async (name: string, url: string, expect: number): Promise<HealthStatus> => {
      const start = performance.now()
      try {
        const res = await fetch(url, { method: 'GET', mode: 'cors' })
        const latency = Math.round(performance.now() - start)
        if (res.ok) {
          return { name, status: 'healthy', detail: `HTTP ${res.status}`, latencyMs: latency }
        }
        return { name, status: 'degraded', detail: `HTTP ${res.status}`, latencyMs: latency }
      } catch (err: any) {
        return { name, status: 'down', detail: err?.message || 'unreachable', latencyMs: Math.round(performance.now() - start) }
      }
    }

    const backend = await check('Backend', `${base}/health/ready`, 200)
    results.push(backend)

    const redis = await check('Redis', `${base}/health/redis`, 200)
    results.push(redis)

    const clickhouse = await check('ClickHouse', `${base}/health/clickhouse`, 200)
    results.push(clickhouse)

    const agentCount = await check('Agent Connectivity', `${base}/health/agents`, 200)
    results.push(agentCount)

    setChecks(results)
    setLastChecked(new Date().toLocaleString())
    setLoading(false)
  }

  useEffect(() => {
    runChecks()
  }, [])

  const healthyCount = checks.filter((c) => c.status === 'healthy').length
  const degradedCount = checks.filter((c) => c.status === 'degraded').length
  const downCount = checks.filter((c) => c.status === 'down').length

  const statusColor = (status: HealthStatus['status']) => {
    switch (status) {
      case 'healthy':
        return 'success'
      case 'degraded':
        return 'warning'
      case 'down':
        return 'error'
      default:
        return 'default'
    }
  }

  return (
    <div style={{ padding: 24 }}>
      <Card
        title={<><DatabaseOutlined /> 平台健康状态</>}
        extra={
          <Button icon={<ThunderboltOutlined />} loading={loading} onClick={runChecks}>
            刷新检测
          </Button>
        }
      >
        <Row gutter={16}>
          <Col span={6}>
            <Statistic title="Healthy" value={healthyCount} valueStyle={{ color: '#52c41a' }} suffix={`/ ${checks.length}`} />
          </Col>
          <Col span={6}>
            <Statistic title="Degraded" value={degradedCount} valueStyle={{ color: '#faad14' }} />
          </Col>
          <Col span={6}>
            <Statistic title="Down" value={downCount} valueStyle={{ color: '#ff4d4f' }} />
          </Col>
          <Col span={6}>
            <Statistic title="Last Checked" value={lastChecked ? new Date(lastChecked).toLocaleTimeString() : '-'} />
          </Col>
        </Row>

        <Space direction="vertical" style={{ width: '100%', marginTop: 16 }}>
          {checks.map((check) => (
            <Card key={check.name} size="small" type="inner">
              <Space style={{ width: '100%', justifyContent: 'space-between' }}>
                <Space>
                  {check.status === 'healthy' ? <CheckCircleOutlined style={{ color: '#52c41a' }} /> : <CloseCircleOutlined style={{ color: '#ff4d4f' }} />}
                  <Text strong>{check.name}</Text>
                  <Tag color={statusColor(check.status)}>{check.status.toUpperCase()}</Tag>
                </Space>
                <Space>
                  {check.latencyMs !== undefined && <Text type="secondary">{check.latencyMs} ms</Text>}
                  <Text type="secondary">{check.detail}</Text>
                </Space>
              </Space>
            </Card>
          ))}
        </Space>

        {downCount > 0 && (
          <Alert
            type="error"
            message="部分依赖不可用"
            description="平台可能无法正常工作，请检查 Backend、Redis、ClickHouse 和 Agent 服务状态。"
            style={{ marginTop: 16 }}
          />
        )}
      </Card>
    </div>
  )
}

export default PlatformHealth
