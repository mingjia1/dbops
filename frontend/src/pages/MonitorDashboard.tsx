import React, { useEffect, useMemo, useState } from 'react'
import { Alert, Button, Card, Col, Empty, Row, Select, Space, Spin, Statistic, Tag, Tooltip } from 'antd'
import {
  AlertOutlined,
  ArrowUpOutlined,
  CheckCircleOutlined,
  CloseCircleOutlined,
  ClockCircleOutlined,
  DatabaseOutlined,
  HddOutlined,
  ReloadOutlined,
  ThunderboltOutlined,
} from '@ant-design/icons'
import {
  Area,
  AreaChart,
  Bar,
  BarChart,
  CartesianGrid,
  Legend,
  Line,
  LineChart,
  ResponsiveContainer,
  Tooltip as RTooltip,
  XAxis,
  YAxis,
} from 'recharts'
import { instanceApi, monitorApi } from '../services/api'
import { palette } from '../appTheme'

interface MetricPoint {
  ts: string
  value: number
}

interface MetricBundle {
  tps?: MetricPoint[]
  qps?: MetricPoint[]
  connections?: MetricPoint[]
  cpu?: MetricPoint[]
  memory?: MetricPoint[]
  disk?: MetricPoint[]
  innodb_buffer_pool_hit_ratio?: number
  slow_queries?: number
  uptime?: number
  innodb_rows_inserted?: number
  innodb_rows_read?: number
  innodb_rows_updated?: number
  innodb_rows_deleted?: number
  threads_running?: number
  threads_connected?: number
  bytes_received?: number
  bytes_sent?: number
}

type MonitorStatus = 'configured' | 'not_configured' | 'no_data' | 'failed'

function formatNumber(n: number | undefined) {
  if (n === undefined || n === null || Number.isNaN(n)) return '-'
  if (Math.abs(n) >= 1e9) return `${(n / 1e9).toFixed(2)}G`
  if (Math.abs(n) >= 1e6) return `${(n / 1e6).toFixed(2)}M`
  if (Math.abs(n) >= 1e3) return `${(n / 1e3).toFixed(2)}K`
  return String(n)
}

function formatBytes(n: number | undefined) {
  if (n === undefined || n === null) return '-'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let idx = 0
  let value = n
  while (value >= 1024 && idx < units.length - 1) {
    value /= 1024
    idx += 1
  }
  return `${value.toFixed(2)} ${units[idx]}`
}

function formatDuration(seconds: number | undefined) {
  if (seconds === undefined || seconds === null) return '-'
  const days = Math.floor(seconds / 86400)
  const hours = Math.floor((seconds % 86400) / 3600)
  const minutes = Math.floor((seconds % 3600) / 60)
  if (days > 0) return `${days}d ${hours}h`
  if (hours > 0) return `${hours}h ${minutes}m`
  return `${minutes}m`
}

const seriesData = (series?: MetricPoint[]) =>
  (series || []).map((point) => ({ ...point, time: point.ts ? new Date(point.ts).toLocaleTimeString() : '-' }))

const hasMetricData = (bundle: MetricBundle) =>
  Object.values(bundle).some((value) => (Array.isArray(value) ? value.length > 0 : value !== undefined && value !== null))

const normalizeMetricRows = (rows: any[]): MetricBundle | null => {
  if (rows.length === 0) return null
  const grouped = rows.reduce((acc: Record<string, MetricPoint[]>, row: any) => {
    const name = String(row?.name || row?.metric_name || '').trim()
    if (!name) return acc
    const point = {
      ts: row?.timestamp || row?.ts || row?.time || '',
      value: Number(row?.value ?? 0),
    }
    acc[name] = [...(acc[name] || []), point]
    return acc
  }, {})
  const last = (name: string) => {
    const series = grouped[name]
    return series?.[series.length - 1]?.value
  }
  const bundle: MetricBundle = {
    tps: grouped.tps || grouped.com_tps || grouped.transactions,
    qps: grouped.qps || grouped.questions || grouped.queries,
    connections: grouped.connections || grouped.threads_connected,
    cpu: grouped.cpu || grouped.cpu_usage,
    memory: grouped.memory || grouped.memory_usage,
    disk: grouped.disk || grouped.disk_usage,
    innodb_buffer_pool_hit_ratio: last('innodb_buffer_pool_hit_ratio') ?? last('buffer_pool_hit_ratio'),
    slow_queries: last('slow_queries') ?? last('slow_query_count'),
    uptime: last('uptime') ?? last('uptime_seconds'),
    innodb_rows_inserted: last('innodb_rows_inserted'),
    innodb_rows_read: last('innodb_rows_read'),
    innodb_rows_updated: last('innodb_rows_updated'),
    innodb_rows_deleted: last('innodb_rows_deleted'),
    threads_running: last('threads_running'),
    threads_connected: last('threads_connected'),
    bytes_received: last('bytes_received'),
    bytes_sent: last('bytes_sent'),
  }
  return hasMetricData(bundle) ? bundle : null
}

const normalizeMetrics = (raw: any): MetricBundle | null => {
  if (Array.isArray(raw)) return normalizeMetricRows(raw)
  const seriesFrom = (key: string): MetricPoint[] | undefined => {
    const value = raw?.[key]
    if (Array.isArray(value)) {
      return value.map((point: any) => ({ ts: point.ts || point.time || point.timestamp || '', value: Number(point.value ?? 0) }))
    }
    if (typeof value === 'number') {
      return [{ ts: new Date().toISOString(), value }]
    }
    return undefined
  }
  const bundle: MetricBundle = {
    tps: seriesFrom('tps') || seriesFrom('com_tps') || seriesFrom('transactions'),
    qps: seriesFrom('qps') || seriesFrom('questions') || seriesFrom('queries'),
    connections: seriesFrom('connections') || seriesFrom('threads_connected'),
    cpu: seriesFrom('cpu') || seriesFrom('cpu_usage'),
    memory: seriesFrom('memory') || seriesFrom('memory_usage'),
    disk: seriesFrom('disk') || seriesFrom('disk_usage'),
    innodb_buffer_pool_hit_ratio: raw?.innodb_buffer_pool_hit_ratio ?? raw?.buffer_pool_hit_ratio,
    slow_queries: raw?.slow_queries ?? raw?.slow_query_count,
    uptime: raw?.uptime ?? raw?.uptime_seconds,
    innodb_rows_inserted: raw?.innodb_rows_inserted,
    innodb_rows_read: raw?.innodb_rows_read,
    innodb_rows_updated: raw?.innodb_rows_updated,
    innodb_rows_deleted: raw?.innodb_rows_deleted,
    threads_running: raw?.threads_running,
    threads_connected: raw?.threads_connected,
    bytes_received: raw?.bytes_received,
    bytes_sent: raw?.bytes_sent,
  }
  return hasMetricData(bundle) ? bundle : null
}

const MonitorDashboard: React.FC = () => {
  const [instances, setInstances] = useState<any[]>([])
  const [selectedInstance, setSelectedInstance] = useState<string | undefined>(undefined)
  const [metrics, setMetrics] = useState<MetricBundle | null>(null)
  const [monitorStatus, setMonitorStatus] = useState<MonitorStatus>('configured')
  const [monitorMessage, setMonitorMessage] = useState('')
  const [loading, setLoading] = useState(false)
  const [stats, setStats] = useState({ total: 0, healthy: 0, unhealthy: 0, stopped: 0 })

  const fetchInstances = async () => {
    try {
      const res: any = await instanceApi.list(100, 0)
      const list = res?.data || []
      setInstances(list)
      setStats({
        total: list.length,
        healthy: list.filter((item: any) => ['healthy', 'ok'].includes(item.status?.health_status)).length,
        unhealthy: list.filter((item: any) => ['unhealthy', 'failed'].includes(item.status?.health_status)).length,
        stopped: list.filter((item: any) => item.status?.run_status === 'stopped').length,
      })
      if (!selectedInstance && list.length > 0) setSelectedInstance(list[0].id)
    } catch {
      // Keep dashboard usable when the instance list is temporarily unavailable.
    }
  }

  const fetchMetrics = async () => {
    if (!selectedInstance) return
    setLoading(true)
    try {
      const res: any = await monitorApi.queryMetrics(selectedInstance)
      const data = res?.data || res || {}
      const status = (data?.status || (Array.isArray(data) && data.length === 0 ? 'no_data' : 'configured')) as MonitorStatus
      const normalized = normalizeMetrics(data?.metrics ?? data)
      setMonitorStatus(normalized ? status : (status === 'configured' ? 'no_data' : status))
      setMonitorMessage(data?.message || '')
      setMetrics(normalized)
    } catch (err: any) {
      setMonitorStatus('failed')
      setMonitorMessage(err?.response?.data?.message || err?.message || 'Monitoring query failed')
      setMetrics(null)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    fetchInstances()
  }, [])

  useEffect(() => {
    fetchMetrics()
  }, [selectedInstance])

  const selectedInstanceObj = instances.find((item) => item.id === selectedInstance)
  const trafficData = useMemo(() => [
    { name: 'Received', value: metrics?.bytes_received || 0, color: palette.series.primary },
    { name: 'Sent', value: metrics?.bytes_sent || 0, color: palette.series.success },
  ], [metrics])
  const dmlData = useMemo(() => [
    { name: 'INSERT', value: metrics?.innodb_rows_inserted || 0, color: palette.series.primary },
    { name: 'UPDATE', value: metrics?.innodb_rows_updated || 0, color: palette.series.warning },
    { name: 'DELETE', value: metrics?.innodb_rows_deleted || 0, color: palette.series.danger },
    { name: 'SELECT', value: metrics?.innodb_rows_read || 0, color: palette.series.success },
  ], [metrics])
  const emptyDescription = monitorStatus === 'not_configured'
    ? 'Monitoring storage is not configured. Configure ClickHouse and agent metrics ingest.'
    : monitorStatus === 'failed'
      ? (monitorMessage || 'Monitoring query failed.')
      : 'No monitoring data has been ingested for this instance.'

  return (
    <div style={{ padding: 24 }}>
      <Row gutter={[16, 16]}>
        <Col xs={24} md={6}><Card><Statistic title="Instances" value={stats.total} prefix={<DatabaseOutlined />} /></Card></Col>
        <Col xs={24} md={6}><Card><Statistic title="Healthy" value={stats.healthy} prefix={<CheckCircleOutlined />} valueStyle={{ color: palette.text.healthy }} /></Card></Col>
        <Col xs={24} md={6}><Card><Statistic title="Unhealthy" value={stats.unhealthy} prefix={<AlertOutlined />} valueStyle={{ color: palette.text.unhealthy }} /></Card></Col>
        <Col xs={24} md={6}><Card><Statistic title="Stopped" value={stats.stopped} prefix={<CloseCircleOutlined />} valueStyle={{ color: palette.text.stopped }} /></Card></Col>
      </Row>

      <Card
        title={<Space><span>Instance Metrics</span>{selectedInstanceObj && <Tag color="blue">{selectedInstanceObj.name}</Tag>}</Space>}
        style={{ marginTop: 16 }}
        extra={(
          <Space>
            <Select
              placeholder="Select instance"
              style={{ width: 240 }}
              value={selectedInstance}
              onChange={setSelectedInstance}
              options={instances.map((item: any) => ({ label: item.name, value: item.id }))}
            />
            <Tooltip title="Refresh">
              <Button type="text" icon={<ReloadOutlined spin={loading} />} onClick={fetchMetrics} />
            </Tooltip>
          </Space>
        )}
      >
        {loading ? (
          <div style={{ height: 300, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
            <Spin tip="Loading..." />
          </div>
        ) : !metrics ? (
          <Space direction="vertical" style={{ width: '100%' }}>
            <Alert
              type={monitorStatus === 'failed' ? 'error' : monitorStatus === 'not_configured' ? 'warning' : 'info'}
              showIcon
              message={monitorStatus === 'not_configured' ? 'Monitoring not configured' : monitorStatus === 'failed' ? 'Monitoring query failed' : 'No monitoring data'}
              description={emptyDescription}
            />
            <Empty description={emptyDescription} />
          </Space>
        ) : (
          <Space direction="vertical" size={16} style={{ width: '100%' }}>
            <Row gutter={[16, 16]}>
              <Col xs={24} md={6}><Card size="small"><Statistic title="QPS" value={metrics.qps?.[metrics.qps.length - 1]?.value || 0} precision={1} prefix={<ThunderboltOutlined />} valueStyle={{ color: palette.series.primary }} /></Card></Col>
              <Col xs={24} md={6}><Card size="small"><Statistic title="TPS" value={metrics.tps?.[metrics.tps.length - 1]?.value || 0} precision={1} prefix={<ArrowUpOutlined />} valueStyle={{ color: palette.series.success }} /></Card></Col>
              <Col xs={24} md={6}><Card size="small"><Statistic title="Running Threads" value={metrics.threads_running || metrics.connections?.[metrics.connections.length - 1]?.value || 0} prefix={<DatabaseOutlined />} /></Card></Col>
              <Col xs={24} md={6}><Card size="small"><Statistic title="Slow Queries" value={metrics.slow_queries || 0} prefix={<AlertOutlined />} valueStyle={{ color: (metrics.slow_queries || 0) > 100 ? '#cf1322' : '#fa8c16' }} /></Card></Col>
            </Row>

            <Row gutter={[16, 16]}>
              <Col xs={24} xl={12}>
                <Card title="QPS Trend" size="small">
                  <ResponsiveContainer width="100%" height={260}>
                    <LineChart data={seriesData(metrics.qps)}>
                      <CartesianGrid strokeDasharray="3 3" />
                      <XAxis dataKey="time" />
                      <YAxis />
                      <RTooltip />
                      <Line type="monotone" dataKey="value" stroke={palette.series.primary} dot={false} />
                    </LineChart>
                  </ResponsiveContainer>
                </Card>
              </Col>
              <Col xs={24} xl={12}>
                <Card title="TPS Trend" size="small">
                  <ResponsiveContainer width="100%" height={260}>
                    <AreaChart data={seriesData(metrics.tps)}>
                      <CartesianGrid strokeDasharray="3 3" />
                      <XAxis dataKey="time" />
                      <YAxis />
                      <RTooltip />
                      <Area type="monotone" dataKey="value" stroke={palette.series.success} fill={palette.series.success} fillOpacity={0.16} />
                    </AreaChart>
                  </ResponsiveContainer>
                </Card>
              </Col>
              <Col xs={24} xl={12}>
                <Card title="Connections" size="small">
                  <ResponsiveContainer width="100%" height={260}>
                    <LineChart data={seriesData(metrics.connections)}>
                      <CartesianGrid strokeDasharray="3 3" />
                      <XAxis dataKey="time" />
                      <YAxis />
                      <RTooltip />
                      <Line type="monotone" dataKey="value" stroke={palette.series.warning} dot={false} />
                    </LineChart>
                  </ResponsiveContainer>
                </Card>
              </Col>
              <Col xs={24} xl={12}>
                <Card title="Resource Usage" size="small">
                  <ResponsiveContainer width="100%" height={260}>
                    <AreaChart data={seriesData(metrics.cpu || metrics.memory || metrics.disk)}>
                      <CartesianGrid strokeDasharray="3 3" />
                      <XAxis dataKey="time" />
                      <YAxis />
                      <RTooltip />
                      <Legend />
                      <Area type="monotone" dataKey="value" stroke={palette.series.danger} fill={palette.series.danger} fillOpacity={0.14} />
                    </AreaChart>
                  </ResponsiveContainer>
                </Card>
              </Col>
              <Col xs={24} xl={12}>
                <Card title="Traffic" size="small">
                  <ResponsiveContainer width="100%" height={220}>
                    <BarChart data={trafficData}>
                      <CartesianGrid strokeDasharray="3 3" />
                      <XAxis dataKey="name" />
                      <YAxis tickFormatter={(value) => formatBytes(Number(value))} />
                      <RTooltip formatter={(value: any) => formatBytes(Number(value))} />
                      <Bar dataKey="value" fill={palette.series.primary} />
                    </BarChart>
                  </ResponsiveContainer>
                </Card>
              </Col>
              <Col xs={24} xl={12}>
                <Card title="DML Rows" size="small">
                  <ResponsiveContainer width="100%" height={220}>
                    <BarChart data={dmlData}>
                      <CartesianGrid strokeDasharray="3 3" />
                      <XAxis dataKey="name" />
                      <YAxis tickFormatter={(value) => formatNumber(Number(value))} />
                      <RTooltip formatter={(value: any) => formatNumber(Number(value))} />
                      <Bar dataKey="value" fill={palette.series.success} />
                    </BarChart>
                  </ResponsiveContainer>
                </Card>
              </Col>
            </Row>

            <Row gutter={[16, 16]}>
              <Col xs={24} md={8}><Card size="small"><Statistic title="Uptime" value={formatDuration(metrics.uptime)} prefix={<ClockCircleOutlined />} /></Card></Col>
              <Col xs={24} md={8}><Card size="small"><Statistic title="Buffer Pool Hit" value={metrics.innodb_buffer_pool_hit_ratio ?? 0} suffix="%" precision={2} prefix={<HddOutlined />} /></Card></Col>
              <Col xs={24} md={8}><Card size="small"><Statistic title="Threads Connected" value={metrics.threads_connected || 0} /></Card></Col>
            </Row>
          </Space>
        )}
      </Card>
    </div>
  )
}

export default MonitorDashboard
