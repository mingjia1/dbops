import React, { useEffect, useState } from 'react'
import { Alert, Badge, Button, Card, Space, Table, Tag, Typography } from 'antd'
import { CheckCircleOutlined, DatabaseOutlined, ReloadOutlined, ThunderboltOutlined } from '@ant-design/icons'
import { healthBaseUrl } from '../services/api'

const { Text } = Typography

interface HealthStatus {
  name: string
  status: 'healthy' | 'degraded' | 'down' | 'disabled'
  detail?: string
  latencyMs?: number
}

// 每个组件一段"人话": 干什么用的、挂了影响什么、可选还是必需.
const COMPONENT_DOCS: Record<string, { role: string; impact: string; optional: boolean }> = {
  后端服务: { role: '平台核心 API 服务', impact: '不可用时平台完全无法使用', optional: false },
  Redis: { role: '缓存与告警通道加速', impact: '不可用时查询略慢, 功能不受影响', optional: true },
  ClickHouse: { role: '历史监控数据存储', impact: '不可用时仅能实时采集, 无法查看历史趋势', optional: true },
  Agent: { role: '主机侧指标与任务执行', impact: '不在线的主机无法采集监控和执行部署任务', optional: false },
}

// 把原始错误翻译成可操作的提示.
const humanizeDetail = (name: string, status: HealthStatus['status'], detail?: string): string => {
  if (status === 'disabled') return '未部署或未在配置中启用, 不影响平台核心功能'
  if (status === 'healthy') return `响应正常, ${detail || ''}`.trim()
  const raw = (detail || '').toLowerCase()
  if (raw.includes('unreachable') || raw.includes('fetch')) return '无法连接, 请确认该服务已启动且端口可达'
  if (name === 'Agent') return '部分主机 Agent 不在线, 请到 代理管理 页查看各主机状态'
  return detail || '状态异常, 请查看后端日志'
}

const PlatformHealth: React.FC = () => {
  const [loading, setLoading] = useState(false)
  const [checks, setChecks] = useState<HealthStatus[]>([])
  const [lastChecked, setLastChecked] = useState<string | null>(null)

  const runChecks = async () => {
    setLoading(true)
    const base = healthBaseUrl()

    const check = async (name: string, url: string): Promise<HealthStatus> => {
      const start = performance.now()
      try {
        const res = await fetch(url, { method: 'GET', mode: 'cors' })
        const latency = Math.round(performance.now() - start)
        const payload = await res.json().catch(() => ({}))
        const detail = payload?.data?.error || payload?.message || `HTTP ${res.status}`
        const statusRaw = String(payload?.data?.status || '').toLowerCase()
        const status = res.ok
          ? statusRaw === 'disabled'
            ? 'disabled'
            : statusRaw === 'down'
              ? 'down'
              : 'healthy'
          : 'down'
        return { name, status, detail, latencyMs: latency }
      } catch (err: any) {
        return { name, status: 'down', detail: err?.message || 'unreachable', latencyMs: Math.round(performance.now() - start) }
      }
    }

    const results = await Promise.all([
      check('后端服务', `${base}/health/ready`),
      check('Agent', `${base}/health/agents`),
      check('Redis', `${base}/health/redis`),
      check('ClickHouse', `${base}/health/clickhouse`),
    ])

    setChecks(results)
    setLastChecked(new Date().toLocaleString('zh-CN', { hour12: false }))
    setLoading(false)
  }

  useEffect(() => {
    runChecks()
  }, [])

  const coreDown = checks.some((c) => (c.name === '后端服务' || c.name === 'Agent') && c.status === 'down')
  const optionalDown = checks.filter((c) => c.name !== '后端服务' && c.name !== 'Agent' && c.status === 'down')
  const okCount = checks.filter((c) => c.status === 'healthy' || c.status === 'disabled').length

  const statusTag = (status: HealthStatus['status']) => {
    const map = {
      healthy: { color: 'success', text: '正常' },
      disabled: { color: 'default', text: '未启用' },
      degraded: { color: 'warning', text: '降级' },
      down: { color: 'error', text: '异常' },
    } as const
    return <Tag color={map[status].color}>{map[status].text}</Tag>
  }

  const columns = [
    {
      title: '组件',
      dataIndex: 'name',
      key: 'name',
      width: 160,
      render: (v: string) => (
        <Space>
          {v === '后端服务' ? <DatabaseOutlined /> : <ThunderboltOutlined />}
          <Text strong>{v}</Text>
        </Space>
      ),
    },
    { title: '状态', dataIndex: 'status', key: 'status', width: 90, render: (v: HealthStatus['status']) => statusTag(v) },
    { title: '作用', key: 'role', render: (_: any, r: HealthStatus) => COMPONENT_DOCS[r.name]?.role || '-' },
    { title: '不可用影响', key: 'impact', render: (_: any, r: HealthStatus) => (
      <Text type={COMPONENT_DOCS[r.name]?.optional ? 'secondary' : undefined}>
        {COMPONENT_DOCS[r.name]?.impact || '-'}
      </Text>
    ) },
    { title: '延迟', dataIndex: 'latencyMs', key: 'latencyMs', width: 100, render: (v?: number) => (v !== undefined ? `${v} ms` : '-') },
    { title: '说明', key: 'detail', render: (_: any, r: HealthStatus) => humanizeDetail(r.name, r.status, r.detail) },
  ]

  return (
    <div style={{ padding: 24 }}>
      <Card
        title={<><DatabaseOutlined /> 平台健康</>}
        extra={
          <Space>
            <Text type="secondary">{lastChecked ? `检测于 ${lastChecked}` : ''}</Text>
            <Button icon={<ReloadOutlined />} loading={loading} onClick={runChecks}>重新检测</Button>
          </Space>
        }
      >
        {coreDown ? (
          <Alert
            type="error"
            showIcon
            message="平台核心服务异常"
            description="后端服务或 Agent 探测失败, 请检查对应进程是否存活、端口是否可达。"
            style={{ marginBottom: 16 }}
          />
        ) : optionalDown.length > 0 ? (
          <Alert
            type="warning"
            showIcon
            message={`核心功能正常, ${optionalDown.map((c) => c.name).join('、')}未就绪`}
            description="以下为可选组件, 仅影响对应增强功能, 不影响日常使用。"
            style={{ marginBottom: 16 }}
          />
        ) : (
          <Alert
            type="success"
            showIcon
            icon={<CheckCircleOutlined />}
            message="平台运行正常"
            description={`全部 ${checks.length} 项组件检查通过。`}
            style={{ marginBottom: 16 }}
          />
        )}

        <Table
          columns={columns}
          dataSource={checks}
          rowKey="name"
          loading={loading}
          pagination={false}
          size="middle"
        />

        <div style={{ marginTop: 12 }}>
          <Badge status="success" text="正常：可用" />
          <Badge status="default" text="未启用：可选组件未部署, 不影响核心功能" style={{ marginLeft: 16 }} />
          <Badge status="error" text="异常：请按说明列提示处理" style={{ marginLeft: 16 }} />
        </div>
      </Card>
    </div>
  )
}

export default PlatformHealth
