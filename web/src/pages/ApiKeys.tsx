import { App, Button, Card, Form, Input, Modal, Select, Switch, Table, Tag, Typography } from 'antd'
import { CopyOutlined, KeyOutlined, PlusOutlined } from '@ant-design/icons'
import { useState } from 'react'
import { useApiKeys, useGroups } from '../api/queries'
import type { ApiKey } from '../api/types'
import { fmtDateTime, fmtInt } from '../utils/format'

function genDemoKey() {
  const hex = (n: number) => Array.from({ length: n }, () => '0123456789abcdef'[Math.floor(Math.random() * 16)]).join('')
  return `cg-${hex(8)}-${hex(48)}`
}

export function ApiKeys() {
  const { message } = App.useApp()
  const apiKeys = useApiKeys()
  const groups = useGroups()
  const [open, setOpen] = useState(false)
  const [created, setCreated] = useState<string | null>(null)

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
    { title: '请求数', dataIndex: 'request_count', width: 120, align: 'right' as const, render: (v: number) => fmtInt(v) },
    { title: '最近使用', dataIndex: 'last_used_at', width: 170, render: (v?: string | null) => (v ? fmtDateTime(v) : '—') },
    { title: '过期时间', dataIndex: 'expires_at', width: 150, render: (v?: string | null) => (v ? new Date(v).toLocaleDateString('zh-CN') : <span style={{ color: 'var(--cg-text-tertiary,#928e85)' }}>永久</span>) },
    { title: '状态', dataIndex: 'enabled', width: 90, render: (v: boolean) => <Switch checked={v} size="small" /> },
  ]

  const handleCreate = () => {
    setCreated(genDemoKey())
  }

  return (
    <Card
      className="cg-soft-card"
      styles={{ body: { padding: 18 } }}
      title={<span style={{ color: 'var(--cg-text-secondary)', fontSize: 13 }}>客户 Key 绑定到分组；明文仅在创建时展示一次，库内只存前缀与 hash</span>}
      extra={<Button type="primary" icon={<PlusOutlined />} onClick={() => { setCreated(null); setOpen(true) }}>新建 Key</Button>}
    >
      <Table<ApiKey> rowKey="id" loading={apiKeys.isLoading} columns={columns} dataSource={apiKeys.data ?? []} pagination={false} />

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
            <Typography.Paragraph type="warning"><KeyOutlined /> 请立即复制并妥善保存，此明文只展示这一次：</Typography.Paragraph>
            <Input.Group compact style={{ display: 'flex' }}>
              <Input className="cg-mono" readOnly value={created} />
              <Button icon={<CopyOutlined />} onClick={() => { navigator.clipboard?.writeText(created); message.success('已复制') }}>复制</Button>
            </Input.Group>
          </div>
        )}
      </Modal>
    </Card>
  )
}
