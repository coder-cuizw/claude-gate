import { App, Button, Card, Form, Input, Modal, Select, Space, Switch, Table, Tag, Typography } from 'antd'
import { CopyOutlined, EyeInvisibleOutlined, EyeOutlined, KeyOutlined, PlusOutlined } from '@ant-design/icons'
import { useState } from 'react'
import { useApiKeys, useGroups } from '../api/queries'
import { api } from '../api/mock'
import type { ApiKey } from '../api/types'
import { fmtDateTime, fmtInt } from '../utils/format'

function genDemoKey() {
  const hex = (n: number) => Array.from({ length: n }, () => '0123456789abcdef'[Math.floor(Math.random() * 16)]).join('')
  return `cg-${hex(8)}-${hex(48)}`
}

/** 把完整 Key 打码：保留 cg- 与前缀，其余以圆点替代。 */
function maskKey(prefix: string) {
  return `cg-${prefix}-${'•'.repeat(24)}`
}

export function ApiKeys() {
  const { message } = App.useApp()
  const apiKeys = useApiKeys()
  const groups = useGroups()
  const [open, setOpen] = useState(false)
  const [created, setCreated] = useState<string | null>(null)

  // 重复查看密钥的状态
  const [revealing, setRevealing] = useState<ApiKey | null>(null)
  const [revealed, setRevealed] = useState<string | null>(null)
  const [showSecret, setShowSecret] = useState(false)
  const [revealLoading, setRevealLoading] = useState(false)

  const openReveal = async (k: ApiKey) => {
    setRevealing(k)
    setRevealed(null)
    setShowSecret(false)
    setRevealLoading(true)
    try {
      const full = await api.revealApiKey(k.id)
      setRevealed(full)
    } catch {
      message.error('获取密钥失败')
    } finally {
      setRevealLoading(false)
    }
  }

  const columns = [
    {
      title: '名称',
      key: 'name',
      render: (_: unknown, k: ApiKey) => (
        <div>
          <div style={{ fontWeight: 600 }}>{k.name}</div>
          <span className="cg-mono" style={{ fontSize: 11.5, color: 'var(--cg-text-secondary)' }}>cg-{k.key_prefix}-••••••••</span>
        </div>
      ),
    },
    { title: '分组', dataIndex: 'group_name', render: (v: string) => <Tag>{v}</Tag> },
    { title: '请求数', dataIndex: 'request_count', width: 110, align: 'right' as const, render: (v: number) => fmtInt(v) },
    { title: '最近使用', dataIndex: 'last_used_at', width: 160, render: (v?: string | null) => (v ? fmtDateTime(v) : '—') },
    { title: '过期时间', dataIndex: 'expires_at', width: 130, render: (v?: string | null) => (v ? new Date(v).toLocaleDateString('zh-CN') : <span style={{ color: 'var(--cg-text-tertiary,#928e85)' }}>永久</span>) },
    { title: '状态', dataIndex: 'enabled', width: 80, render: (v: boolean) => <Switch checked={v} size="small" /> },
    {
      title: '操作',
      key: 'action',
      width: 100,
      render: (_: unknown, k: ApiKey) => (
        <Button type="link" size="small" icon={<EyeOutlined />} style={{ padding: 0 }} onClick={() => openReveal(k)}>
          查看密钥
        </Button>
      ),
    },
  ]

  const handleCreate = () => {
    setCreated(genDemoKey())
  }

  const displayKey = revealed ? (showSecret ? revealed : revealing ? maskKey(revealing.key_prefix) : '') : ''

  return (
    <Card
      className="cg-soft-card"
      styles={{ body: { padding: 18 } }}
      title={<span style={{ color: 'var(--cg-text-secondary)', fontSize: 13 }}>客户 Key 绑定到分组；明文经 AES 加密存储，可随时在此重复查看与复制</span>}
      extra={<Button type="primary" icon={<PlusOutlined />} onClick={() => { setCreated(null); setOpen(true) }}>新建 Key</Button>}
    >
      <Table<ApiKey> rowKey="id" loading={apiKeys.isLoading} columns={columns} dataSource={apiKeys.data ?? []} pagination={false} />

      {/* 新建 Key */}
      <Modal
        title="新建客户 API Key"
        open={open}
        onCancel={() => setOpen(false)}
        footer={created ? [<Button key="done" type="primary" onClick={() => setOpen(false)}>完成</Button>] : undefined}
        onOk={handleCreate}
        okText="生成 Key"
      >
        {!created ? (
          <Form layout="vertical">
            <Form.Item label="名称" required><Input placeholder="例如：生产-客户甲" /></Form.Item>
            <Form.Item label="绑定分组" required>
              <Select options={(groups.data ?? []).map((g) => ({ label: g.name, value: g.id }))} placeholder="选择分组" />
            </Form.Item>
            <Form.Item label="有效期"><Select defaultValue="forever" options={[{ label: '永久', value: 'forever' }, { label: '90 天', value: '90d' }, { label: '1 年', value: '1y' }]} /></Form.Item>
          </Form>
        ) : (
          <div>
            <Typography.Paragraph><KeyOutlined /> 已生成新密钥，可立即复制；之后也可在列表「查看密钥」重复查看：</Typography.Paragraph>
            <Space.Compact style={{ display: 'flex' }}>
              <Input className="cg-mono" readOnly value={created} />
              <Button icon={<CopyOutlined />} onClick={() => { navigator.clipboard?.writeText(created); message.success('已复制') }}>复制</Button>
            </Space.Compact>
          </div>
        )}
      </Modal>

      {/* 重复查看密钥 */}
      <Modal
        title={<Space><KeyOutlined />查看密钥 · {revealing?.name}</Space>}
        open={!!revealing}
        onCancel={() => setRevealing(null)}
        footer={[<Button key="close" onClick={() => setRevealing(null)}>关闭</Button>]}
        destroyOnClose
      >
        <Typography.Paragraph style={{ color: 'var(--cg-text-secondary)', fontSize: 13 }}>
          完整明文由后台 AES-256-GCM 解密获得，可随时重复查看。请勿在不安全场合展示。
        </Typography.Paragraph>
        <Space.Compact style={{ display: 'flex' }}>
          <Input className="cg-mono" readOnly value={displayKey} placeholder={revealLoading ? '解密中…' : ''} />
          <Button
            icon={showSecret ? <EyeInvisibleOutlined /> : <EyeOutlined />}
            onClick={() => setShowSecret((s) => !s)}
            disabled={!revealed}
          >
            {showSecret ? '隐藏' : '显示'}
          </Button>
          <Button
            type="primary"
            icon={<CopyOutlined />}
            disabled={!revealed}
            onClick={() => { if (revealed) { navigator.clipboard?.writeText(revealed); message.success('已复制完整密钥') } }}
          >
            复制
          </Button>
        </Space.Compact>
      </Modal>
    </Card>
  )
}
