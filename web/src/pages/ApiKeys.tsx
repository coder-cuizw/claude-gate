import { App, Button, Card, Form, Input, Modal, Popconfirm, Select, Space, Switch, Table, Tag, Typography } from 'antd'
import { CopyOutlined, DeleteOutlined, EyeInvisibleOutlined, EyeOutlined, KeyOutlined, PlusOutlined } from '@ant-design/icons'
import { useState } from 'react'
import { revealApiKey, useApiKeys, useCreateApiKey, useDeleteApiKey, useGroups, useUpdateApiKey } from '../api/queries'
import type { ApiKey } from '../api/types'
import { fmtInt } from '../utils/format'

/** 把完整 Key 打码：保留 cg- 与前缀，其余以圆点替代。 */
function maskKey(prefix: string) {
  return `cg-${prefix}-${'•'.repeat(24)}`
}

export function ApiKeys() {
  const { message } = App.useApp()
  const apiKeys = useApiKeys()
  const groups = useGroups()
  const [form] = Form.useForm()
  const createMut = useCreateApiKey()
  const updateMut = useUpdateApiKey()
  const deleteMut = useDeleteApiKey()
  const [open, setOpen] = useState(false)
  const [created, setCreated] = useState<string | null>(null)

  // 重复查看密钥
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
      setRevealed(await revealApiKey(k.id))
    } catch {
      message.error('获取密钥失败')
    } finally {
      setRevealLoading(false)
    }
  }

  const handleCreate = async () => {
    const v = await form.validateFields()
    let expires_at: string | null = null
    if (v.expiry === '90d') expires_at = new Date(Date.now() + 90 * 864e5).toISOString()
    if (v.expiry === '1y') expires_at = new Date(Date.now() + 365 * 864e5).toISOString()
    try {
      const res = await createMut.mutateAsync({ name: v.name, group_id: v.group_id, expires_at })
      setCreated(res.plaintext)
    } catch (e) {
      message.error(e instanceof Error ? e.message : '创建失败')
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
    { title: '请求数', dataIndex: 'request_count', width: 100, align: 'right' as const, render: (v: number) => fmtInt(v) },
    { title: '过期时间', dataIndex: 'expires_at', width: 120, render: (v?: string | null) => (v ? new Date(v).toLocaleDateString('zh-CN') : <span style={{ color: 'var(--cg-text-tertiary,#928e85)' }}>永久</span>) },
    {
      title: '状态', dataIndex: 'enabled', width: 80,
      render: (v: boolean, k: ApiKey) => <Switch checked={v} size="small" onChange={(c) => updateMut.mutate({ id: k.id, enabled: c })} />,
    },
    {
      title: '操作',
      key: 'action',
      width: 150,
      render: (_: unknown, k: ApiKey) => (
        <Space size={2}>
          <Button type="link" size="small" icon={<EyeOutlined />} style={{ padding: 0 }} onClick={() => openReveal(k)}>查看</Button>
          <Popconfirm title="确认删除该 Key？" onConfirm={() => deleteMut.mutate(k.id)} okText="删除" cancelText="取消">
            <Button type="link" size="small" danger icon={<DeleteOutlined />} style={{ padding: '0 4px' }}>删除</Button>
          </Popconfirm>
        </Space>
      ),
    },
  ]

  const displayKey = revealed ? (showSecret ? revealed : revealing ? maskKey(revealing.key_prefix) : '') : ''

  return (
    <Card
      className="cg-soft-card"
      styles={{ body: { padding: 18 } }}
      title={<span style={{ color: 'var(--cg-text-secondary)', fontSize: 13 }}>客户 Key 绑定到分组；明文经 AES 加密存储，可随时在此重复查看与复制</span>}
      extra={<Button type="primary" icon={<PlusOutlined />} onClick={() => { setCreated(null); form.resetFields(); setOpen(true) }}>新建 Key</Button>}
    >
      <Table<ApiKey> rowKey="id" loading={apiKeys.isLoading} columns={columns} dataSource={apiKeys.data ?? []} pagination={false} />

      <Modal
        title="新建客户 API Key"
        open={open}
        onCancel={() => setOpen(false)}
        footer={created ? [<Button key="done" type="primary" onClick={() => setOpen(false)}>完成</Button>] : undefined}
        onOk={handleCreate}
        okText="生成 Key"
        confirmLoading={createMut.isPending}
      >
        {!created ? (
          <Form form={form} layout="vertical" initialValues={{ expiry: 'forever' }}>
            <Form.Item name="name" label="名称" rules={[{ required: true, message: '请输入名称' }]}><Input placeholder="例如：生产-客户甲" /></Form.Item>
            <Form.Item name="group_id" label="绑定分组" rules={[{ required: true, message: '请选择分组' }]}>
              <Select options={(groups.data ?? []).map((g) => ({ label: g.name, value: g.id }))} placeholder="选择分组" />
            </Form.Item>
            <Form.Item name="expiry" label="有效期"><Select options={[{ label: '永久', value: 'forever' }, { label: '90 天', value: '90d' }, { label: '1 年', value: '1y' }]} /></Form.Item>
          </Form>
        ) : (
          <div>
            <Typography.Paragraph><KeyOutlined /> 已生成新密钥，可立即复制；之后也可在列表「查看」重复查看：</Typography.Paragraph>
            <Space.Compact style={{ display: 'flex' }}>
              <Input className="cg-mono" readOnly value={created} />
              <Button icon={<CopyOutlined />} onClick={() => { navigator.clipboard?.writeText(created); message.success('已复制') }}>复制</Button>
            </Space.Compact>
          </div>
        )}
      </Modal>

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
          <Button icon={showSecret ? <EyeInvisibleOutlined /> : <EyeOutlined />} onClick={() => setShowSecret((s) => !s)} disabled={!revealed}>
            {showSecret ? '隐藏' : '显示'}
          </Button>
          <Button type="primary" icon={<CopyOutlined />} disabled={!revealed} onClick={() => { if (revealed) { navigator.clipboard?.writeText(revealed); message.success('已复制完整密钥') } }}>
            复制
          </Button>
        </Space.Compact>
      </Modal>
    </Card>
  )
}
