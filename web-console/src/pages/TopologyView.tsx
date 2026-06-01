import React from 'react'
import { Card } from 'antd'

const TopologyView: React.FC = () => {
  return (
    <div style={{ padding: '24px' }}>
      <Card title="拓扑视图">
        <div style={{ height: 600, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
          <span style={{ color: '#8c8c8c' }}>拓扑图加载中...</span>
        </div>
      </Card>
    </div>
  )
}

export default TopologyView