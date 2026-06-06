import React, { useEffect, useState } from 'react'
import { Row, Col, Tag, Spin, Empty } from 'antd'
import {
  DesktopOutlined, DatabaseOutlined, CloudOutlined, SettingOutlined,
  CheckCircleOutlined, CloseCircleOutlined, MinusCircleOutlined,
  ArrowRightOutlined, AlertOutlined, AuditOutlined, SafetyCertificateOutlined,
} from '@ant-design/icons'
import { useNavigate } from 'react-router-dom'
import {
  hostApi, instanceApi, alertApi, approvalApi, auditApi,
  dataMigrationApi, hostApi as _h, type Host, type Instance,
} from '../services/api'
import './Home.css'

const Home: React.FC = () => {
  const navigate = useNavigate()
  const [loading, setLoading] = useState(true)
  const [hosts, setHosts] = useState<Host[]>([])
  const [instances, setInstances] = useState<Instance[]>([])
  const [alertCount, setAlertCount] = useState(0)
  const [approvalCount, setApprovalCount] = useState(0)
  const [auditCount, setAuditCount] = useState(0)
  const [dialect, setDialect] = useState<string>('sqlite')

  useEffect(() => {
    Promise.all([fetchHosts(), fetchInstances(), fetchAlerts(), fetchApprovals(), fetchAudit(), fetchDialect()])
      .finally(() => setLoading(false))
  }, [])

  const fetchHosts = async () => {
    try { const r: any = await hostApi.list(100, 0); setHosts(r.data || []) } catch { setHosts([]) }
  }
  const fetchInstances = async () => {
    try { const r: any = await instanceApi.list(100, 0); setInstances(r.data || []) } catch { setInstances([]) }
  }
  const fetchAlerts = async () => {
    try { const r: any = await alertApi.listHistory(); setAlertCount((r.data || []).length) } catch { setAlertCount(0) }
  }
  const fetchApprovals = async () => {
    try { const r: any = await approvalApi.list(); setApprovalCount((r.data || []).length) } catch { setApprovalCount(0) }
  }
  const fetchAudit = async () => {
    try { const r: any = await auditApi.list({}); setAuditCount((r.data || []).length) } catch { setAuditCount(0) }
  }
  const fetchDialect = async () => {
    try { const r: any = await dataMigrationApi.getStatus(); setDialect(r.data?.dialect || 'sqlite') } catch {}
  }

  const hostOk = hosts.filter(h => h.status === 'success').length
  const hostFail = hosts.filter(h => h.status === 'failed').length
  const hostUnknown = hosts.filter(h => h.status !== 'success' && h.status !== 'failed').length
  const instWithHost = instances.filter(i => i.host_id).length

  const stats = [
    {
      label: '主机', value: hosts.length, accent: '#0071E3', icon: <DesktopOutlined />,
      extra: (
        <div style={{ display: 'flex', gap: 6, marginTop: 6, flexWrap: 'wrap' }}>
          {hostOk > 0 && <span className="apple-tag is-green">{hostOk} 可用</span>}
          {hostFail > 0 && <span className="apple-tag is-red">{hostFail} 异常</span>}
          {hostUnknown > 0 && <span className="apple-tag is-gray">{hostUnknown} 未检测</span>}
          {hosts.length === 0 && <span className="apple-tag is-gray">暂无</span>}
        </div>
      ),
    },
    {
      label: '实例', value: instances.length, accent: '#34C759', icon: <DatabaseOutlined />,
      extra: (
        <div style={{ display: 'flex', gap: 6, marginTop: 6, flexWrap: 'wrap' }}>
          {instWithHost > 0 && <span className="apple-tag">{instWithHost} 已关联主机</span>}
          {instances.length === 0 && <span className="apple-tag is-gray">暂无</span>}
        </div>
      ),
    },
    {
      label: '告警历史', value: alertCount, accent: '#FF9500', icon: <AlertOutlined />,
      // P1: 之前写 "近 7 天累计" 但 alertApi.listHistory() 返全表 (无时间过滤),
      // 显示数字永远偏小. 改成 "总记录数" 诚实表达.
      extra: <div style={{ marginTop: 6, fontSize: 12, color: 'var(--apple-text-secondary)' }}>总记录数</div>,
    },
    {
      label: '待审批', value: approvalCount, accent: '#AF52DE', icon: <SafetyCertificateOutlined />,
      extra: <div style={{ marginTop: 6, fontSize: 12, color: 'var(--apple-text-secondary)' }}>待处理数</div>,
    },
    {
      label: '审计日志', value: auditCount, accent: '#5856D6', icon: <AuditOutlined />,
      extra: <div style={{ marginTop: 6, fontSize: 12, color: 'var(--apple-text-secondary)' }}>近 7 天累计</div>,
    },
    {
      label: '存储后端', value: dialect === 'mysql' ? 'MySQL' : 'SQLite', accent: '#5AC8FA',
      icon: <CloudOutlined />, valueIsString: true,
      extra: <div style={{ marginTop: 6, fontSize: 12, color: 'var(--apple-text-secondary)' }}>{dialect === 'mysql' ? '生产级' : '嵌入式'}</div>,
    },
  ]

  const quickActions = [
    { title: '添加主机', desc: '录入资产并测试连接', icon: <DesktopOutlined />, color: '#0071E3', path: '/dashboard/hosts/new' },
    { title: '新建实例', desc: '注册 MySQL 实例', icon: <DatabaseOutlined />, color: '#34C759', path: '/dashboard/instances' },
    { title: '环境检测', desc: '检测部署环境', icon: <SettingOutlined />, color: '#FF9500', path: '/dashboard/env-check' },
    { title: '备份策略', desc: '数据备份与恢复', icon: <CloudOutlined />, color: '#AF52DE', path: '/dashboard/backup' },
    { title: '告警规则', desc: '配置监控告警', icon: <AlertOutlined />, color: '#FF3B30', path: '/dashboard/alert-rules' },
    { title: '数据存储', desc: 'SQLite / MySQL 切换', icon: <CloudOutlined />, color: '#5AC8FA', path: '/dashboard/data-storage' },
  ]

  if (loading) {
    return (
      <div className="home-loading">
        <Spin size="large" />
      </div>
    )
  }

  return (
    <div className="apple-page apple-fade-in">
      <h1 className="apple-page-title">概览</h1>
      <p className="apple-page-subtitle">当前系统运行情况 · {hosts.length} 台主机 · {instances.length} 个实例</p>

      <Row gutter={[16, 16]}>
        {stats.map(s => (
          <Col key={s.label} xs={12} sm={12} md={8} lg={8} xl={4}>
            <div className="apple-stat">
              <div className="apple-stat-icon" style={{ background: s.accent }}>
                {s.icon}
              </div>
              <div className="apple-stat-body">
                <div className="apple-stat-label">{s.label}</div>
                <div className="apple-stat-value" style={{ color: s.accent }}>{s.value}</div>
                {s.extra}
              </div>
            </div>
          </Col>
        ))}
      </Row>

      <div className="apple-section" style={{ marginTop: 32 }}>
        <div className="apple-section-header">
          <h2 className="apple-section-title">快捷操作</h2>
          <span style={{ color: 'var(--apple-text-secondary)', fontSize: 13 }}>一键进入常用功能</span>
        </div>
        {quickActions.length === 0 ? (
          <Empty description="暂无快捷操作" />
        ) : (
          <Row gutter={[14, 14]}>
            {quickActions.map(action => (
              <Col key={action.path} xs={12} sm={12} md={8} lg={8} xl={4}>
                <div className="apple-card is-interactive" onClick={() => navigate(action.path)} style={{ display: 'flex', alignItems: 'center', gap: 14, padding: '18px 20px' }}>
                  <div style={{
                    width: 42, height: 42, borderRadius: 11, background: action.color,
                    display: 'flex', alignItems: 'center', justifyContent: 'center',
                    color: '#fff', fontSize: 20, flexShrink: 0,
                    boxShadow: `0 4px 12px ${action.color}33`,
                  }}>
                    {action.icon}
                  </div>
                  <div style={{ flex: 1, minWidth: 0 }}>
                    <div style={{ fontSize: 15, fontWeight: 600, color: 'var(--apple-text)', letterSpacing: '-0.01em' }}>{action.title}</div>
                    <div style={{ fontSize: 12, color: 'var(--apple-text-secondary)', marginTop: 2 }}>{action.desc}</div>
                  </div>
                  <ArrowRightOutlined style={{ color: 'var(--apple-text-tertiary)', fontSize: 14 }} />
                </div>
              </Col>
            ))}
          </Row>
        )}
      </div>

      <div className="apple-section" style={{ marginTop: 32 }}>
        <div className="apple-section-header">
          <h2 className="apple-section-title">主机状态分布</h2>
          <span style={{ color: 'var(--apple-text-secondary)', fontSize: 13 }}>实时</span>
        </div>
        <div className="apple-card" style={{ padding: '20px 24px' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 32, flexWrap: 'wrap' }}>
            <Stat label="可用" value={hostOk} icon={<CheckCircleOutlined />} color="var(--apple-green)" />
            <Stat label="异常" value={hostFail} icon={<CloseCircleOutlined />} color="var(--apple-red)" />
            <Stat label="未检测" value={hostUnknown} icon={<MinusCircleOutlined />} color="var(--apple-text-tertiary)" />
            <div style={{ flex: 1 }} />
            <Tag color="default" style={{ borderRadius: 6 }}>合计 {hosts.length}</Tag>
          </div>
        </div>
      </div>
    </div>
  )
}

const Stat: React.FC<{ label: string; value: number; icon: React.ReactNode; color: string }> = ({ label, value, icon, color }) => (
  <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
    <span style={{ color, fontSize: 20, display: 'flex' }}>{icon}</span>
    <div>
      <div style={{ fontSize: 13, color: 'var(--apple-text-secondary)' }}>{label}</div>
      <div style={{ fontSize: 22, fontWeight: 600, color, lineHeight: 1.1 }}>{value}</div>
    </div>
  </div>
)

export default Home
