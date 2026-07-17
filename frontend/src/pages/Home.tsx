import React, { useEffect, useState } from 'react'
import { Row, Col, Tag, Spin, Empty } from 'antd'
import {
  DesktopOutlined, DatabaseOutlined, CloudOutlined, SettingOutlined,
  CheckCircleOutlined, CloseCircleOutlined, MinusCircleOutlined,
  ArrowRightOutlined, AlertOutlined, AuditOutlined, SafetyCertificateOutlined,
  ClusterOutlined, ApartmentOutlined, HeartOutlined, BarChartOutlined, RocketOutlined,
} from '@ant-design/icons'
import { useNavigate } from 'react-router-dom'
import {
  hostApi, instanceApi, alertApi, approvalApi, auditApi,
  dataMigrationApi, hostApi as _h, type Host, type Instance,
} from '../services/api'
import { palette } from '../appTheme'
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
      label: '主机', value: hosts.length, accent: palette.accent.blue, icon: <DesktopOutlined />,
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
      label: '实例', value: instances.length, accent: palette.accent.green, icon: <DatabaseOutlined />,
      extra: (
        <div style={{ display: 'flex', gap: 6, marginTop: 6, flexWrap: 'wrap' }}>
          {instWithHost > 0 && <span className="apple-tag">{instWithHost} 已关联主机</span>}
          {instances.length === 0 && <span className="apple-tag is-gray">暂无</span>}
        </div>
      ),
    },
    {
      label: '告警历史', value: alertCount, accent: palette.accent.orange, icon: <AlertOutlined />,
      // P1: 之前写 "近 7 天累计" 但 alertApi.listHistory() 返全表 (无时间过滤),
      // 显示数字永远偏小. 改成 "总记录数" 诚实表达.
      extra: <div style={{ marginTop: 6, fontSize: 12, color: 'var(--apple-text-secondary)' }}>总记录数</div>,
    },
    {
      label: '待审批', value: approvalCount, accent: palette.accent.purple, icon: <SafetyCertificateOutlined />,
      extra: <div style={{ marginTop: 6, fontSize: 12, color: 'var(--apple-text-secondary)' }}>待处理数</div>,
    },
    {
      label: '审计日志', value: auditCount, accent: '#5856D6', icon: <AuditOutlined />,
      extra: <div style={{ marginTop: 6, fontSize: 12, color: 'var(--apple-text-secondary)' }}>近 7 天累计</div>,
    },
    {
      label: '存储后端', value: dialect === 'mysql' ? 'MySQL' : 'SQLite', accent: palette.accent.cyan,
      icon: <CloudOutlined />, valueIsString: true,
      extra: <div style={{ marginTop: 6, fontSize: 12, color: 'var(--apple-text-secondary)' }}>{dialect === 'mysql' ? '生产级' : '嵌入式'}</div>,
    },
  ]

  const quickActions = [
    { step: '01', title: '部署数据库', desc: '小白向导：选场景、选机器、一键装 MySQL', icon: <RocketOutlined />, color: palette.accent.green, path: '/dashboard/deploy-wizard' },
    { step: '02', title: '添加空主机', desc: '录入一台尚未安装数据库的服务器', icon: <DesktopOutlined />, color: palette.accent.blue, path: '/dashboard/hosts/new' },
    { step: '03', title: '环境检测', desc: '检查 SSH、端口、目录和部署依赖', icon: <SettingOutlined />, color: palette.accent.orange, path: '/dashboard/env-check' },
    { step: '04', title: '高级集群部署', desc: 'MHA / MGR / PXC 流程图编排', icon: <ClusterOutlined />, color: palette.accent.purple, path: '/dashboard/cluster-deploy' },
    { step: '05', title: '确认拓扑', desc: '查看主从、角色和实例关联关系', icon: <ApartmentOutlined />, color: palette.accent.cyan, path: '/dashboard/topology' },
    { step: '06', title: '监控备份', desc: '接入指标告警并配置备份策略', icon: <BarChartOutlined />, color: palette.accent.orange, path: '/dashboard/monitor' },
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
          <span style={{ color: 'var(--apple-text-secondary)', fontSize: 13 }}>从空主机到集群管理的操作路径</span>
        </div>
        {quickActions.length === 0 ? (
          <Empty description="暂无快捷操作" />
        ) : (
          <Row gutter={[14, 14]}>
            {quickActions.map(action => (
              <Col key={action.path} xs={12} sm={12} md={8} lg={8} xl={4}>
                <div className="apple-card is-interactive quick-action-card" onClick={() => navigate(action.path)}>
                  <div className="quick-action-step">{action.step}</div>
                  <div className="quick-action-icon" style={{ background: action.color, boxShadow: `0 4px 12px ${action.color}33` }}>
                    {action.icon}
                  </div>
                  <div className="quick-action-body">
                    <div className="quick-action-title">{action.title}</div>
                    <div className="quick-action-desc">{action.desc}</div>
                  </div>
                  <ArrowRightOutlined className="quick-action-arrow" />
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
