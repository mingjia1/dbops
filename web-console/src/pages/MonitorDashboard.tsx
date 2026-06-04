import React, { useEffect, useMemo, useState } from 'react'
import { Card, Row, Col, Statistic, Select, Spin, Empty, Alert, Tag, Space, Progress, Tooltip, Button } from 'antd'
import {
  DatabaseOutlined, CheckCircleOutlined, CloseCircleOutlined, AlertOutlined,
  ReloadOutlined, ClockCircleOutlined, ThunderboltOutlined, HddOutlined,
  ArrowUpOutlined, ArrowDownOutlined,
} from '@ant-design/icons'
import {
  LineChart, Line, AreaChart, Area, BarChart, Bar, PieChart, Pie, Cell,
  XAxis, YAxis, CartesianGrid, Tooltip as RTooltip, Legend, ResponsiveContainer,
} from 'recharts'
import { instanceApi, monitorApi } from '../services/api'

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

function formatNumber(n: number | undefined) {
  if (n === undefined || n === null || Number.isNaN(n)) return '-'
  if (Math.abs(n) >= 1e9) return `${(n / 1e9).toFixed(2)}G`
  if (Math.abs(n) >= 1e6) return `${(n / 1e6).toFixed(2)}M`
  if (Math.abs(n) >= 1e3) return `${(n / 1e3).toFixed(2)}K`
  return String(n)
}

function formatBytes(n: number | undefined) {
  if (n === undefined || n === null) return '-'
  const u = ['B', 'KB', 'MB', 'GB', 'TB']
  let i = 0
  let v = n
  while (v >= 1024 && i < u.length - 1) { v /= 1024; i++ }
  return `${v.toFixed(2)} ${u[i]}`
}

function formatDuration(seconds: number | undefined) {
  if (seconds === undefined || seconds === null) return '-'
  const d = Math.floor(seconds / 86400)
  const h = Math.floor((seconds % 86400) / 3600)
  const m = Math.floor((seconds % 3600) / 60)
  if (d > 0) return `${d}天${h}小时`
  if (h > 0) return `${h}小时${m}分`
  return `${m}分`
}

const MonitorDashboard: React.FC = () => {
  const [instances, setInstances] = useState<any[]>([])
  const [selectedInstance, setSelectedInstance] = useState<string | undefined>(undefined)
  const [metrics, setMetrics] = useState<MetricBundle | null>(null)
  const [loading, setLoading] = useState(false)
  const [stats, setStats] = useState({ total: 0, healthy: 0, unhealthy: 0, stopped: 0 })

  const fetchInstances = async () => {
    try {
      const res: any = await instanceApi.list(100, 0)
      const list = res?.data || []
      setInstances(list)
      setStats({
        total: list.length,
        healthy: list.filter((i: any) => {
          const h = i.status?.health_status
          return h === 'healthy' || h === 'ok'
        }).length,
        unhealthy: list.filter((i: any) => {
          const h = i.status?.health_status
          return h === 'unhealthy' || h === 'failed'
        }).length,
        stopped: list.filter((i: any) => i.status?.run_status === 'stopped').length,
      })
      if (!selectedInstance && list.length > 0) {
        setSelectedInstance(list[0].id)
      }
    } catch { /* ignore */ }
  }

  useEffect(() => {
    fetchInstances()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  const fetchMetrics = async () => {
    if (!selectedInstance) return
    setLoading(true)
    try {
      const res: any = await monitorApi.queryMetrics(selectedInstance)
      const data = res?.data || res || {}
      setMetrics(normalizeMetrics(data))
    } catch {
      setMetrics(null)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    fetchMetrics()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [selectedInstance])

  const normalizeMetrics = (raw: any): MetricBundle => {
    const seriesFrom = (k: string): MetricPoint[] | undefined => {
      const v = raw?.[k]
      if (Array.isArray(v)) {
        return v.map((p: any) => ({ ts: p.ts || p.time || p.timestamp || '', value: Number(p.value ?? 0) }))
      }
      if (typeof v === 'number') {
        return [{ ts: new Date().toLocaleTimeString(), value: v }]
      }
      return undefined
    }
    return {
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
  }

  const seriesData = (s?: MetricPoint[]) =>
    (s || []).map((p) => ({ ...p, time: p.ts ? new Date(p.ts).toLocaleTimeString() : '-' }))

  const trafficData = useMemo(() => {
    if (!metrics) return []
    return [
      { name: '接收', value: metrics.bytes_received || 0, color: '#1890ff' },
      { name: '发送', value: metrics.bytes_sent || 0, color: '#52c41a' },
    ]
  }, [metrics])

  const dmlData = useMemo(() => {
    if (!metrics) return []
    return [
      { name: 'INSERT', value: metrics.innodb_rows_inserted || 0, color: '#1890ff' },
      { name: 'UPDATE', value: metrics.innodb_rows_updated || 0, color: '#fa8c16' },
      { name: 'DELETE', value: metrics.innodb_rows_deleted || 0, color: '#f5222d' },
      { name: 'SELECT', value: metrics.innodb_rows_read || 0, color: '#52c41a' },
    ]
  }, [metrics])

  const selectedInstanceObj = instances.find((i) => i.id === selectedInstance)
  const bufferHit = metrics?.innodb_buffer_pool_hit_ratio ?? null
  const noData = !loading && !metrics

  return (
    <div style={{ padding: '24px' }}>
      <Alert
        type="info"
        showIcon
        style={{ marginBottom: 16 }}
        message="监控仪表盘"
        description="实时显示 MySQL 实例的运行指标: QPS/TPS、连接数、CPU/内存/磁盘使用率、InnoDB 缓冲池命中率、DML 流量统计、慢查询数等。"
      />

      <Row gutter={[16, 16]}>
        <Col span={6}>
          <Card>
            <Statistic title="总实例数" value={stats.total} prefix={<DatabaseOutlined />} />
          </Card>
        </Col>
        <Col span={6}>
          <Card>
            <Statistic title="健康" value={stats.healthy} prefix={<CheckCircleOutlined />} valueStyle={{ color: '#3f8600' }} />
          </Card>
        </Col>
        <Col span={6}>
          <Card>
            <Statistic title="异常" value={stats.unhealthy} prefix={<AlertOutlined />} valueStyle={{ color: '#cf1322' }} />
          </Card>
        </Col>
        <Col span={6}>
          <Card>
            <Statistic title="已停止" value={stats.stopped} prefix={<CloseCircleOutlined />} valueStyle={{ color: '#8c8c8c' }} />
          </Card>
        </Col>
      </Row>

      <Card
        title={
          <Space>
            <span>实例指标</span>
            {selectedInstanceObj && <Tag color="blue">{selectedInstanceObj.name}</Tag>}
          </Space>
        }
        style={{ marginTop: 16 }}
        extra={
          <Space>
            <Select
              placeholder="选择实例查看指标"
              style={{ width: 240 }}
              value={selectedInstance}
              onChange={setSelectedInstance}
              options={instances.map((i: any) => ({ label: i.name, value: i.id }))}
            />
            <Tooltip title="刷新">
              <Button
                type="text"
                icon={<ReloadOutlined spin={loading} />}
                onClick={fetchMetrics}
              />
            </Tooltip>
          </Space>
        }
      >
        {loading ? (
          <div style={{ height: 300, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
            <Spin tip="加载中..." />
          </div>
        ) : noData || !metrics ? (
          <Empty description="暂无监控数据" />
        ) : (
          <>
            <Row gutter={[16, 16]} style={{ marginBottom: 16 }}>
              <Col span={6}>
                <Card size="small">
                  <Statistic
                    title="QPS"
                    value={metrics.qps?.[metrics.qps.length - 1]?.value || 0}
                    precision={1}
                    prefix={<ThunderboltOutlined />}
                    valueStyle={{ color: '#1890ff' }}
                  />
                </Card>
              </Col>
              <Col span={6}>
                <Card size="small">
                  <Statistic
                    title="TPS"
                    value={metrics.tps?.[metrics.tps.length - 1]?.value || 0}
                    precision={1}
                    prefix={<ArrowUpOutlined />}
                    valueStyle={{ color: '#52c41a' }}
                  />
                </Card>
              </Col>
              <Col span={6}>
                <Card size="small">
                  <Statistic
                    title="活跃连接"
                    value={metrics.threads_running || metrics.connections?.[metrics.connections.length - 1]?.value || 0}
                    prefix={<ArrowDownOutlined />}
                  />
                </Card>
              </Col>
              <Col span={6}>
                <Card size="small">
                  <Statistic
                    title="慢查询累计"
                    value={metrics.slow_queries || 0}
                    prefix={<ClockCircleOutlined />}
                    valueStyle={{ color: (metrics.slow_queries || 0) > 100 ? '#cf1322' : '#fa8c16' }}
                  />
                </Card>
              </Col>
            </Row>

            <Row gutter={[16, 16]}>
              <Col span={12}>
                <Card size="small" title="QPS 趋势">
                  <ResponsiveContainer width="100%" height={240}>
                    <LineChart data={seriesData(metrics.qps)}>
                      <CartesianGrid strokeDasharray="3 3" />
                      <XAxis dataKey="time" tick={{ fontSize: 11 }} />
                      <YAxis tick={{ fontSize: 11 }} />
                      <RTooltip />
                      <Legend />
                      <Line type="monotone" dataKey="value" name="QPS" stroke="#1890ff" dot={false} />
                    </LineChart>
                  </ResponsiveContainer>
                </Card>
              </Col>
              <Col span={12}>
                <Card size="small" title="TPS 趋势">
                  <ResponsiveContainer width="100%" height={240}>
                    <AreaChart data={seriesData(metrics.tps)}>
                      <defs>
                        <linearGradient id="tpsGrad" x1="0" y1="0" x2="0" y2="1">
                          <stop offset="5%" stopColor="#52c41a" stopOpacity={0.8} />
                          <stop offset="95%" stopColor="#52c41a" stopOpacity={0.1} />
                        </linearGradient>
                      </defs>
                      <CartesianGrid strokeDasharray="3 3" />
                      <XAxis dataKey="time" tick={{ fontSize: 11 }} />
                      <YAxis tick={{ fontSize: 11 }} />
                      <RTooltip />
                      <Area type="monotone" dataKey="value" name="TPS" stroke="#52c41a" fill="url(#tpsGrad)" />
                    </AreaChart>
                  </ResponsiveContainer>
                </Card>
              </Col>

              <Col span={12}>
                <Card size="small" title="连接数趋势">
                  <ResponsiveContainer width="100%" height={240}>
                    <LineChart data={seriesData(metrics.connections)}>
                      <CartesianGrid strokeDasharray="3 3" />
                      <XAxis dataKey="time" tick={{ fontSize: 11 }} />
                      <YAxis tick={{ fontSize: 11 }} />
                      <RTooltip />
                      <Line type="monotone" dataKey="value" name="连接数" stroke="#722ed1" dot={false} />
                    </LineChart>
                  </ResponsiveContainer>
                </Card>
              </Col>
              <Col span={12}>
                <Card size="small" title="InnoDB DML 行数累计">
                  <ResponsiveContainer width="100%" height={240}>
                    <BarChart data={dmlData}>
                      <CartesianGrid strokeDasharray="3 3" />
                      <XAxis dataKey="name" tick={{ fontSize: 11 }} />
                      <YAxis tick={{ fontSize: 11 }} tickFormatter={formatNumber} />
                      <RTooltip formatter={(v: any) => formatNumber(Number(v))} />
                      <Bar dataKey="value" name="行数">
                        {dmlData.map((entry, idx) => (
                          <Cell key={idx} fill={entry.color} />
                        ))}
                      </Bar>
                    </BarChart>
                  </ResponsiveContainer>
                </Card>
              </Col>

              <Col span={12}>
                <Card size="small" title="CPU 使用率">
                  <ResponsiveContainer width="100%" height={240}>
                    <AreaChart data={seriesData(metrics.cpu)}>
                      <defs>
                        <linearGradient id="cpuGrad" x1="0" y1="0" x2="0" y2="1">
                          <stop offset="5%" stopColor="#fa8c16" stopOpacity={0.8} />
                          <stop offset="95%" stopColor="#fa8c16" stopOpacity={0.1} />
                        </linearGradient>
                      </defs>
                      <CartesianGrid strokeDasharray="3 3" />
                      <XAxis dataKey="time" tick={{ fontSize: 11 }} />
                      <YAxis tick={{ fontSize: 11 }} domain={[0, 100]} unit="%" />
                      <RTooltip />
                      <Area type="monotone" dataKey="value" name="CPU%" stroke="#fa8c16" fill="url(#cpuGrad)" />
                    </AreaChart>
                  </ResponsiveContainer>
                </Card>
              </Col>
              <Col span={12}>
                <Card size="small" title="网络流量 (累计)">
                  <ResponsiveContainer width="100%" height={240}>
                    <PieChart>
                      <Pie
                        data={trafficData}
                        cx="50%"
                        cy="50%"
                        labelLine={false}
                        outerRadius={80}
                        dataKey="value"
                        label={(e: any) => `${e.name}: ${formatBytes(e.value)}`}
                      >
                        {trafficData.map((entry, idx) => (
                          <Cell key={idx} fill={entry.color} />
                        ))}
                      </Pie>
                      <RTooltip formatter={(v: any) => formatBytes(Number(v))} />
                    </PieChart>
                  </ResponsiveContainer>
                </Card>
              </Col>
            </Row>

            <Row gutter={[16, 16]} style={{ marginTop: 16 }}>
              <Col span={8}>
                <Card size="small" title={<Space><HddOutlined />InnoDB 缓冲池命中率</Space>}>
                  {bufferHit !== null ? (
                    <>
                      <Progress
                        type="circle"
                        percent={Math.round((bufferHit > 1 ? bufferHit : bufferHit * 100))}
                        format={(p) => `${p}%`}
                        strokeColor={bufferHit > 0.95 ? '#52c41a' : bufferHit > 0.85 ? '#fa8c16' : '#f5222d'}
                      />
                      <div style={{ marginTop: 8, color: '#8c8c8c', fontSize: 12 }}>
                        健康范围: ≥ 95%
                      </div>
                    </>
                  ) : <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="无数据" />}
                </Card>
              </Col>
              <Col span={8}>
                <Card size="small" title="MySQL 运行时间">
                  <Statistic
                    value={formatDuration(metrics.uptime)}
                    valueStyle={{ color: '#1890ff' }}
                    prefix={<ClockCircleOutlined />}
                  />
                </Card>
              </Col>
              <Col span={8}>
                <Card size="small" title="连接数详情">
                  <Row gutter={8}>
                    <Col span={12}>
                      <Statistic title="活跃" value={metrics.threads_running || 0} valueStyle={{ fontSize: 18, color: '#fa8c16' }} />
                    </Col>
                    <Col span={12}>
                      <Statistic title="已连接" value={metrics.threads_connected || 0} valueStyle={{ fontSize: 18 }} />
                    </Col>
                  </Row>
                </Card>
              </Col>
            </Row>
          </>
        )}
      </Card>
    </div>
  )
}

export default MonitorDashboard
