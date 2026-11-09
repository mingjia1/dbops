import React, { useState, useEffect, useRef } from 'react'
import { Card, Table, Button, Modal, Input, Space, Tag, message, Descriptions, Tabs, Typography } from 'antd'
import { RobotOutlined, SearchOutlined, BulbOutlined } from '@ant-design/icons'
import api from '../services/api'

const { TextArea } = Input
const { Text } = Typography

interface Diagnosis {
  id: string
  instance_id: string
  status: string
  summary: string
  score: number
  created_at: string
}

interface SQLAdvice {
  id: string
  sql_text: string
  explain: string
  advice: string
  score: number
  created_at: string
}

interface ChatMsg {
  role: 'user' | 'assistant'
  content: string
}

const CHAT_WELCOME: ChatMsg = { role: 'assistant', content: '你好，我是 AI 运维助手。可以询问实例诊断、参数调优、备份恢复、故障排查等问题。' }

export default function AIDiagnosisPage() {
  const [diagnoses, setDiagnoses] = useState<Diagnosis[]>([])
  const [advices, setAdvices] = useState<SQLAdvice[]>([])
  const [diagnoseOpen, setDiagnoseOpen] = useState(false)
  const [advisorOpen, setAdvisorOpen] = useState(false)
  const [instanceID, setInstanceID] = useState('')
  const [sqlText, setSqlText] = useState('')
  const [loading, setLoading] = useState(false)
  const [chatMessages, setChatMessages] = useState<ChatMsg[]>([CHAT_WELCOME])
  const [chatInput, setChatInput] = useState('')
  const [chatLoading, setChatLoading] = useState(false)
  const chatBoxRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    chatBoxRef.current?.scrollTo({ top: chatBoxRef.current.scrollHeight })
  }, [chatMessages, chatLoading])

  const fetchDiagnoses = async () => {
    try { const res = await api.get('/ai/diagnoses'); setDiagnoses(res.data || []) } catch {}
  }
  const fetchAdvices = async () => {
    try { const res = await api.get('/ai/sql-advices'); setAdvices(res.data || []) } catch {}
  }
  useEffect(() => { fetchDiagnoses(); fetchAdvices() }, [])

  const handleDiagnose = async () => {
    if (!instanceID) { message.warning('请输入实例ID'); return }
    setLoading(true)
    try { await api.post('/ai/diagnosis', { instance_id: instanceID }); message.success('诊断任务已提交'); setDiagnoseOpen(false); fetchDiagnoses() }
    catch (err: any) { message.error(err.message) }
    finally { setLoading(false) }
  }

  const handleSQLAdvisor = async () => {
    if (!sqlText) { message.warning('请输入 SQL'); return }
    setLoading(true)
    try { await api.post('/ai/sql-advisor', { sql_text: sqlText }); message.success('SQL 分析完成'); setAdvisorOpen(false); fetchAdvices() }
    catch (err: any) { message.error(err.message) }
    finally { setLoading(false) }
  }

  const handleChat = async () => {
    const text = chatInput.trim()
    if (!text || chatLoading) return
    const history = [...chatMessages.filter(m => m !== CHAT_WELCOME || chatMessages.length === 1), { role: 'user' as const, content: text }]
    setChatMessages([...chatMessages, { role: 'user', content: text }])
    setChatInput('')
    setChatLoading(true)
    try {
      const res = await api.post('/ai/chat', { messages: history.slice(-20) })
      const reply = res.data?.reply || '（空回复）'
      setChatMessages(prev => [...prev, { role: 'assistant', content: reply }])
    } catch (err: any) {
      message.error(err?.response?.data?.message || err.message)
      setChatMessages(prev => [...prev, { role: 'assistant', content: '请求失败：' + (err?.response?.data?.message || err.message) }])
    } finally {
      setChatLoading(false)
    }
  }

  const diagColumns = [
    { title: '实例', dataIndex: 'instance_id', key: 'instance_id', ellipsis: true },
    { title: '状态', dataIndex: 'status', key: 'status', render: (v: string) => <Tag color={v === 'completed' ? 'green' : 'blue'}>{v}</Tag> },
    { title: '摘要', dataIndex: 'summary', key: 'summary', ellipsis: true },
    { title: '评分', dataIndex: 'score', key: 'score', render: (v: number) => <Tag color={v >= 80 ? 'green' : v >= 60 ? 'orange' : 'red'}>{v}</Tag> },
    { title: '时间', dataIndex: 'created_at', key: 'created_at', width: 160, render: (v: string) => new Date(v).toLocaleString() },
  ]

  const sqlColumns = [
    { title: 'SQL', dataIndex: 'sql_text', key: 'sql_text', ellipsis: true, width: 300 },
    { title: '建议', dataIndex: 'advice', key: 'advice', ellipsis: true },
    { title: '评分', dataIndex: 'score', key: 'score', render: (v: number) => <Tag color={v >= 80 ? 'green' : v >= 60 ? 'orange' : 'red'}>{v}</Tag> },
    { title: '时间', dataIndex: 'created_at', key: 'created_at', width: 160, render: (v: string) => new Date(v).toLocaleString() },
  ]

  return (
    <div style={{ padding: 24 }}>
      <Card title={<><RobotOutlined /> AI 智能诊断</>}>
        <Tabs items={[
          { key: 'diagnosis', label: '实例诊断', children: (
            <>
              <div style={{ marginBottom: 16 }}>
                <Button type="primary" icon={<SearchOutlined />} onClick={() => setDiagnoseOpen(true)}>发起诊断</Button>
              </div>
              <Table columns={diagColumns} dataSource={diagnoses} rowKey="id" size="small" />
            </>
          )},
          { key: 'advisor', label: 'SQL 优化建议', children: (
            <>
              <div style={{ marginBottom: 16 }}>
                <Button type="primary" icon={<BulbOutlined />} onClick={() => setAdvisorOpen(true)}>SQL 分析</Button>
              </div>
              <Table columns={sqlColumns} dataSource={advices} rowKey="id" size="small" />
            </>
          )},
          { key: 'chat', label: 'AI 运维助手', children: (
            <div style={{ display: 'flex', flexDirection: 'column', height: 480 }}>
              <div ref={chatBoxRef} style={{ flex: 1, overflowY: 'auto', background: '#fafafa', borderRadius: 8, padding: 16, marginBottom: 12 }}>
                {chatMessages.map((m, i) => (
                  <div key={i} style={{ textAlign: m.role === 'user' ? 'right' : 'left', marginBottom: 10 }}>
                    <div style={{ display: 'inline-block', maxWidth: '80%', padding: '8px 12px', borderRadius: 8, background: m.role === 'user' ? '#e6f4ff' : '#fff', border: '1px solid #f0f0f0', whiteSpace: 'pre-wrap', textAlign: 'left' }}>{m.content}</div>
                  </div>
                ))}
                {chatLoading && <div style={{ color: '#999' }}>AI 正在思考…</div>}
              </div>
              <div style={{ display: 'flex', gap: 8 }}>
                <TextArea
                  rows={2}
                  placeholder="向 AI 运维助手提问，Enter 发送，Shift+Enter 换行"
                  value={chatInput}
                  onChange={e => setChatInput(e.target.value)}
                  onKeyDown={e => { if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); handleChat() } }}
                />
                <Button type="primary" icon={<RobotOutlined />} loading={chatLoading} onClick={handleChat}>发送</Button>
              </div>
            </div>
          )},
        ]} />
      </Card>
      <Modal title="实例诊断" open={diagnoseOpen} onCancel={() => setDiagnoseOpen(false)} onOk={handleDiagnose} confirmLoading={loading}>
        <Input placeholder="输入实例 ID" value={instanceID} onChange={e => setInstanceID(e.target.value)} />
      </Modal>
      <Modal title="SQL 优化分析" open={advisorOpen} onCancel={() => setAdvisorOpen(false)} onOk={handleSQLAdvisor} confirmLoading={loading} width={600}>
        <TextArea rows={6} placeholder="粘贴 SQL 语句" value={sqlText} onChange={e => setSqlText(e.target.value)} />
      </Modal>
    </div>
  )
}
