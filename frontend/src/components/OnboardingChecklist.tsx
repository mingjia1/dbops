import React, { useEffect, useState } from 'react'
import { Alert, Button, Checkbox, List, Space, Typography } from 'antd'
import { CheckCircleOutlined, CloseCircleOutlined } from '@ant-design/icons'
import api from '../services/api'

const { Text } = Typography

interface OnboardingStep {
  key: string
  title: string
  description: string
  action?: () => void
  actionLabel?: string
  check: () => Promise<boolean>
}

const OnboardingChecklist: React.FC = () => {
  const [visible, setVisible] = useState(false)
  const [loading, setLoading] = useState(false)
  const [steps, setSteps] = useState<OnboardingStep[]>([])
  const [results, setResults] = useState<Record<string, boolean>>({})

  useEffect(() => {
    const dismissed = localStorage.getItem('dbops_onboarding_dismissed')
    if (dismissed) return

    const checks: OnboardingStep[] = [
      {
        key: 'backend_health',
        title: 'Backend 服务正常',
        description: '平台 API 服务已启动且健康检查通过',
        check: async () => {
          try {
            const res: any = await fetch('/health/ready').then(r => r.json())
            return res.code === 200 && res.data?.status === 'ready'
          } catch {
            return false
          }
        },
      },
      {
        key: 'first_host',
        title: '已添加被管理主机',
        description: '至少添加一台 Linux 主机并确认 Agent 可达',
        check: async () => {
          try {
            const res = await api.get('/hosts')
            return (res?.data?.length || 0) > 0
          } catch {
            return false
          }
        },
      },
      {
        key: 'agent_running',
        title: 'Agent 已安装并运行',
        description: '主机上的 Agent 服务已启动且能通过认证',
        check: async () => {
          try {
            const res = await api.get('/hosts')
            const hosts = res?.data || []
            return hosts.some((h: any) => h.agent_status === 'online' || h.agent_status === 'running')
          } catch {
            return false
          }
        },
      },
      {
        key: 'scanned_instances',
        title: '已扫描并登记实例',
        description: '至少一台 MySQL 实例已被发现并纳入平台管理',
        check: async () => {
          try {
            const res = await api.get('/instances')
            return (res?.data?.length || 0) > 0
          } catch {
            return false
          }
        },
      },
    ]
    setSteps(checks)
    setVisible(true)
    runChecks(checks)
  }, [])

  const runChecks = async (checks: OnboardingStep[]) => {
    setLoading(true)
    const results: Record<string, boolean> = {}
    for (const step of checks) {
      results[step.key] = await step.check()
    }
    setResults(results)
    setLoading(false)
  }

  const handleDismiss = () => {
    localStorage.setItem('dbops_onboarding_dismissed', 'true')
    setVisible(false)
  }

  if (!visible) return null

  const completedCount = Object.values(results).filter(Boolean).length
  const allDone = completedCount === steps.length

  return (
    <Alert
      type={allDone ? 'success' : 'info'}
      message={
        <Space direction="vertical" size="small" style={{ width: '100%' }}>
          <Text strong>{allDone ? '平台已就绪' : '快速入门向导'}</Text>
          <Text type="secondary">
            {allDone
              ? '恭喜！你已完成基本配置，可以开始使用平台管理 MySQL 实例。'
              : `已完成 ${completedCount}/${steps.length} 项检查，请按以下步骤完成初始化。`}
          </Text>
          <List
            size="small"
            loading={loading}
            dataSource={steps}
            renderItem={(step) => (
              <List.Item>
                <Space>
                  {results[step.key] ? (
                    <CheckCircleOutlined style={{ color: '#52c41a' }} />
                  ) : (
                    <CloseCircleOutlined style={{ color: '#d9d9d9' }} />
                  )}
                  <div>
                    <Text strong>{step.title}</Text>
                    <br />
                    <Text type="secondary" style={{ fontSize: 12 }}>{step.description}</Text>
                  </div>
                </Space>
              </List.Item>
            )}
          />
          <Button type="primary" size="small" onClick={handleDismiss}>
            {allDone ? '开始使用' : '稍后再说'}
          </Button>
        </Space>
      }
      style={{ marginBottom: 16 }}
    />
  )
}

export default OnboardingChecklist
