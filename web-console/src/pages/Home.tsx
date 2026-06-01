import React, { useState, useEffect } from 'react'
import { Row, Col, Card, Statistic, Progress, Table, Tag, Space, Button, Alert } from 'antd'
import {
  DatabaseOutlined, CheckCircleOutlined, CloseCircleOutlined,
  AlertOutlined, ThunderboltOutlined, PartitionOutlined,
  CloudUploadOutlined, SyncOutlined,
} from '@ant-design/icons'
import { useNavigate } from 'react-router-dom'
import axios from 'axios'
import type { ColumnsType } from 'antd/es/table'
import './Home.css'

interface Instance {
  id: string
  name: string
  host: string
  port: number
  status: string
  version: string
}

interface Backup {
  task_id: string
  status: string
  started_at: string
  completed_at: string
  file_path: string
  file_size: number
}

interface Alert {
  id: string
  rule_name: string
  level: string
  message: string
  created_at: string
}

const Home: React.FC = () => {
  const navigate = useNavigate()
  const [loading, setLoading] = useState(true)
  const [instances, setInstances] = useState<Instance[]>([])
  const [backups, setBackups] = useState<Backup[]>([])
  const [alerts, setAlerts] = useState<Alert[]>([])

  useEffect(() => {
    fetchData()
  }, [])

  const fetchData = async () => {
    try {
      setLoading(true)

      const instanceRes = await axios.get('/api/v1/instances')
      setInstances(Array.isArray(instanceRes) ? instanceRes : [])

      const backupRes = await axios.get('/api/v1/backups')
      setBackups(Array.isArray(backupRes) ? backupRes : [])

      const alertRes = await axios.get('/api/v1/alerts/rules')
      setAlerts(Array.isArray(alertRes) ? alertRes : [])
    } catch (error) {
      console.error('Failed to fetch data:', error)
    } finally {
      setLoading(false)
    }
  }

  const backupColumns: ColumnsType<Backup> = [
    {
      title: '备份ID',
      dataIndex: 'task_id',
      key: 'task_id',
    },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      render: (status: string) => (
        <Tag color={status === 'completed' ? 'success' : status === 'failed' ? 'error' : 'processing'} icon={
          status === 'completed' ? <CheckCircleOutlined /> :
          status === 'failed' ? <CloseCircleOutlined /> : <SyncOutlined spin />
        }>
          {status === 'completed' ? '完成' : status === 'failed' ? '失败' : '进行中'}
        </Tag>
      ),
    },
    {
      title: '开始时间',
      dataIndex: 'started_at',
      key: 'started_at',
      render: (time: string) => new Date(time).toLocaleString(),
    },
    {
      title: '文件大小',
      dataIndex: 'file_size',
      key: 'file_size',
      render: (size: number) => `${(size / 1024 / 1024).toFixed(2)} MB`,
    },
  ]

  const alertColumns: ColumnsType<Alert> = [
    {
      title: '告警规则',
      dataIndex: 'rule_name',
      key: 'rule_name',
    },
    {
      title: '级别',
      dataIndex: 'level',
      key: 'level',
      render: (level: string) => (
        <Tag color={level === 'critical' ? 'red' : level === 'warning' ? 'orange' : 'blue'}>
          {level === 'critical' ? '严重' : level === 'warning' ? '警告' : '信息'}
        </Tag>
      ),
    },
    {
      title: '消息',
      dataIndex: 'message',
      key: 'message',
      ellipsis: true,
    },
    {
      title: '时间',
      dataIndex: 'created_at',
      key: 'created_at',
      render: (time: string) => new Date(time).toLocaleString(),
    },
  ]

  const runningInstances = instances.filter(i => i.status === 'running').length
  const stoppedInstances = instances.filter(i => i.status === 'stopped').length
  const criticalAlerts = alerts.filter(a => a.level === 'critical').length

  return (
    <div className="home-container">
      <Alert
        className="home-alert"
        message="系统状态正常"
        description="所有服务运行正常，未发现严重问题"
        type="success"
        showIcon
        closable
      />

      <div className="stats-grid">
        <div className="stat-card">
          <Statistic title="总实例数" value={instances.length} prefix={<DatabaseOutlined />} />
        </div>
        <div className="stat-card running">
          <Statistic title="运行中" value={runningInstances} prefix={<CheckCircleOutlined />} />
        </div>
        <div className="stat-card stopped">
          <Statistic title="已停止" value={stoppedInstances} prefix={<CloseCircleOutlined />} />
        </div>
        <div className="stat-card critical">
          <Statistic title="严重告警" value={criticalAlerts} prefix={<AlertOutlined />} />
        </div>
      </div>

      <div className="progress-section">
        <div className="progress-grid">
          <div className="progress-item">
            <Progress
              type="circle"
              percent={instances.length > 0 ? Math.round((runningInstances / instances.length) * 100) : 0}
              format={(percent) => `${percent}% 运行中`}
              strokeColor="#52c41a"
            />
          </div>
          <div className="progress-item">
            <Progress
              type="circle"
              percent={instances.length > 0 ? Math.round((stoppedInstances / instances.length) * 100) : 0}
              format={(percent) => `${percent}% 已停止`}
              strokeColor="#ff4d4f"
            />
          </div>
        </div>
      </div>

      <div className="actions-grid">
        <Button className="action-button" icon={<DatabaseOutlined />} onClick={() => navigate('/instances')}>
          添加实例
        </Button>
        <Button className="action-button" icon={<CloudUploadOutlined />} onClick={() => navigate('/backup')}>
          创建备份
        </Button>
        <Button className="action-button" icon={<ThunderboltOutlined />} onClick={() => navigate('/upgrade')}>
          版本升级
        </Button>
        <Button className="action-button" icon={<PartitionOutlined />} onClick={() => navigate('/migration')}>
          数据迁移
        </Button>
      </div>

      <div className="tables-grid">
        <div className="table-card">
          <Card title="最近备份" extra={<Button type="link" onClick={() => navigate('/backup')}>查看全部</Button>}>
            <Table
              dataSource={backups.slice(0, 5)}
              columns={backupColumns}
              pagination={false}
              size="small"
              rowKey="task_id"
            />
          </Card>
        </div>
        <div className="table-card">
          <Card title="最新告警" extra={<Button type="link" onClick={() => navigate('/alert-rules')}>查看全部</Button>}>
            <Table
              dataSource={alerts.slice(0, 5)}
              columns={alertColumns}
              pagination={false}
              size="small"
              rowKey="id"
            />
          </Card>
        </div>
      </div>
    </div>
  )
}

export default Home