import React, { useEffect, useState } from 'react'
import { Row, Col, Card, Statistic, Tag, Spin } from 'antd'
import {
  DesktopOutlined, DatabaseOutlined, CloudOutlined, SettingOutlined,
  CheckCircleOutlined, CloseCircleOutlined, MinusCircleOutlined,
} from '@ant-design/icons'
import { useNavigate } from 'react-router-dom'
import { hostApi, instanceApi, Host, Instance } from '../services/api'
import './Home.css'

const Home: React.FC = () => {
  const navigate = useNavigate()
  const [loading, setLoading] = useState(true)
  const [hosts, setHosts] = useState<Host[]>([])
  const [instances, setInstances] = useState<Instance[]>([])

  useEffect(() => {
    Promise.all([fetchHosts(), fetchInstances()]).finally(() => setLoading(false))
  }, [])

  const fetchHosts = async () => {
    try {
      const res: any = await hostApi.list(100, 0)
      setHosts(res.data || [])
    } catch { setHosts([]) }
  }

  const fetchInstances = async () => {
    try {
      const res: any = await instanceApi.list(100, 0)
      setInstances(res.data || [])
    } catch { setInstances([]) }
  }

  const hostOk = hosts.filter(h => h.status === 'success').length
  const hostFail = hosts.filter(h => h.status === 'failed').length
  const hostUnknown = hosts.filter(h => h.status !== 'success' && h.status !== 'failed').length
  const instWithHost = instances.filter(i => i.host_id).length

  const quickActions = [
    { title: '主机管理', desc: `${hosts.length} 台主机`, icon: <DesktopOutlined />, color: '#1890ff', path: '/dashboard/hosts' },
    { title: '实例管理', desc: `${instances.length} 个实例`, icon: <DatabaseOutlined />, color: '#52c41a', path: '/dashboard/instances' },
    { title: '环境检测', desc: '检测部署环境', icon: <SettingOutlined className="home-icon" />, color: '#722ed1', path: '/dashboard/env-check' },
    { title: '备份管理', desc: '数据备份与恢复', icon: <CloudOutlined />, color: '#fa8c16', path: '/dashboard/backup' },
  ]

  if (loading) {
    return (
      <div className="home-loading">
        <Spin size="large" />
      </div>
    )
  }

  return (
    <div className="home-page">
      <div className="home-welcome">
        <h2>欢迎使用 MySQL 运维平台</h2>
        <p>当前系统运行正常，共管理 {hosts.length} 台主机、{instances.length} 个实例</p>
      </div>

      <Row gutter={[16, 16]} style={{ marginBottom: 24 }}>
        <Col xs={24} sm={12} lg={6}>
          <Card className="home-stat-card" variant="borderless">
            <Statistic title="主机总数" value={hosts.length} prefix={<DesktopOutlined />} valueStyle={{ color: '#1890ff' }} />
            <div className="stat-tags">
              {hostOk > 0 && <Tag icon={<CheckCircleOutlined />} color="success">{hostOk} 可用</Tag>}
              {hostFail > 0 && <Tag icon={<CloseCircleOutlined />} color="error">{hostFail} 异常</Tag>}
              {hostUnknown > 0 && <Tag icon={<MinusCircleOutlined />}>{hostUnknown} 未检测</Tag>}
            </div>
          </Card>
        </Col>
        <Col xs={24} sm={12} lg={6}>
          <Card className="home-stat-card" variant="borderless">
            <Statistic title="实例总数" value={instances.length} prefix={<DatabaseOutlined />} valueStyle={{ color: '#52c41a' }} />
            <div className="stat-tags">
              {instWithHost > 0 && <Tag color="blue">{instWithHost} 已关联主机</Tag>}
            </div>
          </Card>
        </Col>
        <Col xs={24} sm={12} lg={6}>
          <Card className="home-stat-card" variant="borderless">
            <Statistic title="运行模式" value="Standalone" prefix={<SettingOutlined className="home-icon" />} valueStyle={{ color: '#722ed1', fontSize: 20 }} />
            <div className="stat-tags">
              <Tag>内存存储</Tag>
            </div>
          </Card>
        </Col>
        <Col xs={24} sm={12} lg={6}>
          <Card className="home-stat-card" variant="borderless">
            <Statistic title="系统状态" value="运行中" prefix={<CheckCircleOutlined />} valueStyle={{ color: '#52c41a' }} />
            <div className="stat-tags">
              <Tag color="success">所有服务正常</Tag>
            </div>
          </Card>
        </Col>
      </Row>

      <h3 className="home-section-title">快捷操作</h3>
      <Row gutter={[16, 16]}>
        {quickActions.map(action => (
          <Col xs={12} sm={12} lg={6} key={action.path}>
            <Card
              className="home-action-card"
              variant="borderless"
              hoverable
              onClick={() => navigate(action.path)}
            >
              <div className="action-card-icon" style={{ background: `${action.color}15`, color: action.color }}>
                {action.icon}
              </div>
              <div className="action-card-title">{action.title}</div>
              <div className="action-card-desc">{action.desc}</div>
            </Card>
          </Col>
        ))}
      </Row>
    </div>
  )
}

export default Home
