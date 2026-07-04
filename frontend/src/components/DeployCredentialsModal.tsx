import React from 'react'
import { Alert, Button, Descriptions, Modal, Space, Table, Typography } from 'antd'
import { KeyOutlined } from '@ant-design/icons'
import { formatClusterRole } from '../services/roleDisplay'
import type { ArchType } from '../services/deployHelpers'

interface CredentialNode {
  host: string
  port?: number
  role?: string
  username?: string
  password?: string
}

interface DeployCredentialsModalProps {
  visible: boolean
  mysql_user: string
  mysql_password: string
  nodes?: CredentialNode[]
  arch: ArchType
  onClose: () => void
}

const { Text } = Typography

const DeployCredentialsModal: React.FC<DeployCredentialsModalProps> = ({
  visible,
  mysql_user,
  mysql_password,
  nodes,
  arch,
  onClose,
}) => {
  return (
    <Modal
      title={
        <Space>
          <KeyOutlined />
          <span>部署成功 - MySQL 凭证信息</span>
        </Space>
      }
      open={visible}
      onCancel={onClose}
      footer={
        <Button type="primary" onClick={onClose}>
          我已保存
        </Button>
      }
      width={600}
    >
      <Alert
        type="warning"
        showIcon
        message="请立即保存以下 MySQL 连接信息！此信息关闭后将不再显示。"
        description="部署后的 MySQL root 密码仅在此处展示一次，请保存到安全位置。可在实例详情页通过「强制修改密码」功能重置密码。"
        style={{ marginBottom: 16 }}
      />
      <Descriptions size="small" column={1} bordered style={{ marginBottom: 16 }}>
        <Descriptions.Item label="用户名">{mysql_user}</Descriptions.Item>
        <Descriptions.Item label="密码">
          <Text copyable={{ text: mysql_password }}>
            {mysql_password}
          </Text>
        </Descriptions.Item>
      </Descriptions>
      {nodes && nodes.length > 0 && (
        <Table
          size="small"
          pagination={false}
          columns={[
            { title: '节点', dataIndex: 'host', key: 'host' },
            { title: '端口', dataIndex: 'port', key: 'port' },
            { title: '角色', dataIndex: 'role', key: 'role', render: (role: string) => formatClusterRole(arch, role) },
            { title: '用户名', dataIndex: 'username', key: 'username' },
            {
              title: '密码',
              dataIndex: 'password',
              key: 'password',
              render: (pw: string) => <Text copyable={{ text: pw }}>{pw}</Text>,
            },
          ]}
          dataSource={nodes}
          rowKey={(row) => `${row.host}:${row.port}`}
        />
      )}
    </Modal>
  )
}

export default DeployCredentialsModal
