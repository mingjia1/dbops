import React, { useEffect, useMemo, useState } from 'react'
import { Tour } from 'antd'
import { useNavigate } from 'react-router-dom'

const TOUR_STORAGE_KEY = 'dbops_dashboard_tour_dismissed'

const DashboardTour: React.FC = () => {
  const [open, setOpen] = useState(false)
  const navigate = useNavigate()

  useEffect(() => {
    const dismissed = localStorage.getItem(TOUR_STORAGE_KEY)
    if (dismissed) return

    const timer = setTimeout(() => {
      setOpen(true)
    }, 1000)

    return () => clearTimeout(timer)
  }, [])

  const steps = useMemo(() => {
    const getTarget = (selector: string) => {
      const el = document.querySelector(selector)
      return (el as HTMLElement | null) ?? undefined
    }

    return [
      {
        title: '欢迎使用 MySQL 运维平台',
        description: '这是一个快速导览，帮助你了解平台的主要功能。',
        target: undefined,
      },
      {
        title: '主机与实例',
        description: '在这里管理被管理主机和 MySQL 实例，支持扫描、纳管和生命周期操作。',
        target: getTarget('[data-menu-key="/dashboard/hosts"]'),
        beforeClose: () => navigate('/dashboard/hosts'),
      },
      {
        title: '集群部署',
        description: '通过流程编排或表单方式部署 HA、MHA、MGR、PXC 等架构。',
        target: getTarget('[data-menu-key="/dashboard/cluster-deploy"]'),
      },
      {
        title: '升级管理',
        description: '规划并执行 MySQL 版本升级，支持原地升级、逻辑迁移和滚动升级。',
        target: getTarget('[data-menu-key="/dashboard/upgrade"]'),
      },
      {
        title: '监控与告警',
        description: '查看实例指标、配置告警规则和通知渠道。',
        target: getTarget('[data-menu-key="/dashboard/monitor"]'),
      },
      {
        title: '开始使用',
        description: '你可以随时点击右上角头像 → 「使用引导」重新查看此导览。',
        target: undefined,
      },
    ]
  }, [navigate])

  const handleClose = () => {
    localStorage.setItem(TOUR_STORAGE_KEY, 'true')
    setOpen(false)
  }

  return (
    <Tour
      open={open}
      onClose={handleClose}
      steps={steps}
      mask={{
        style: { backgroundColor: 'rgba(0, 0, 0, 0.3)' },
      }}
      onFinish={handleClose}
    />
  )
}

export default DashboardTour
