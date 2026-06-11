import { App, Button, Card, Form, Input, Modal, Popconfirm, Segmented, Select, Switch, Table, Tag } from 'antd'
import { DeleteOutlined, PlusOutlined, SettingOutlined } from '@ant-design/icons'
import { useState } from 'react'
import { ChannelTag, KeyStatusTag } from '../components/tags'
import {
  useChannels,
  useCreateUpstreamKey,
  useDeleteUpstreamKey,
  useSaveChannel,
  useToggleChannel,
  useUpdateUpstreamKey,
  useUpstreamKeys,
} from '../api/queries'
import type { Channel, ChannelType, UpstreamKey } from '../api/types'
import { fmtDateTime } from '../utils/format'

/** 按通道类型渲染差异化配置字段。 */
function ChannelConfigFields({ type }: { type: ChannelType }) {
  switch (type) {
    case 'kiro':
      return (
        <>
          <Form.Item label="Base URL" name="base_url"><Input placeholder="https://prod.kiro.internal" /></Form.Item>
          <Form.Item label="鉴权头" name={['config', 'auth_header']} tooltip="透传时凭证写入的请求头，默认 Authorization"><Input placeholder="Authorization" /></Form.Item>
          <Tag color="default" style={{ marginBottom: 8 }}>当前为透传：号池由外部维护，直接填好 key 即可，无需刷新；后续按真实报错再做私有协议适配</Tag>
        </>
      )
    case 'official':
      return (
        <>
          <Form.Item label="Base URL" name="base_url"><Input placeholder="https://api.anthropic.com" /></Form.Item>
          <Form.Item label="anthropic-version" name={['config', 'anthropic_version']}><Input placeholder="2023-06-01" /></Form.Item>
        </>
      )
    case 'relay':
      return (
        <>
          <Form.Item label="Base URL" name="base_url"><Input placeholder="https://relay.example.com" /></Form.Item>
          <Form.Item label="鉴权模式" name={['config', 'auth_mode']}>
            <Select options={[{ label: '透传客户凭证', value: 'passthrough' }, { label: '自定义 Bearer', value: 'bearer' }]} />
          </Form.Item>
        </>
      )
    default:
      return null
  }
}

function KeyPool({ channelId }: { channelId: number }) {
  const { message } = App.useApp()
  const keys = useUpstreamKeys(channelId)
  const createKey = useCreateUpstreamKey()
  const updateKey = useUpdateUpstreamKey()
  const deleteKey = useDeleteUpstreamKey()
  const [open, setOpen] = useState(false)
  const [form] = Form.useForm()

  const addKey = async () => {
    const v = await form.validateFields()
    try {
      await createKey.mutateAsync({ channel_id: channelId, name: v.name, credential: v.credential })
      message.success('已添加 Key')
      setOpen(false)
      form.resetFields()
    } catch (e) {
      message.error(e instanceof Error ? e.message : '添加失败')
    }
  }

  const columns = [
    { title: 'Key 名称', dataIndex: 'name', render: (v: string) => <span className="cg-mono" style={{ fontSize: 12 }}>{v}</span> },
    { title: '状态', dataIndex: 'status', width: 100, render: (s: UpstreamKey['status']) => <KeyStatusTag status={s} /> },
    { title: '最近使用', dataIndex: 'last_used_at', width: 168, render: (v?: string | null) => (v ? fmtDateTime(v) : '—') },
    { title: '最近错误', dataIndex: 'last_error', ellipsis: true, render: (v?: string) => (v ? <span style={{ color: '#C0492F', fontSize: 12 }}>{v}</span> : <span style={{ color: 'var(--cg-text-tertiary,#928e85)' }}>—</span>) },
    {
      title: '操作', key: 'op', width: 150,
      render: (_: unknown, k: UpstreamKey) => (
        <span style={{ display: 'inline-flex', gap: 8, alignItems: 'center' }}>
          <Switch size="small" checked={k.status === 'active'} checkedChildren="启" unCheckedChildren="停" onChange={(c) => updateKey.mutate({ id: k.id, status: c ? 'active' : 'disabled' })} />
          <Popconfirm title="删除该 Key？" onConfirm={() => deleteKey.mutate(k.id)} okText="删除" cancelText="取消">
            <Button type="link" size="small" danger icon={<DeleteOutlined />} style={{ padding: 0 }} />
          </Popconfirm>
        </span>
      ),
    },
  ]
  return (
    <div style={{ padding: '4px 0' }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 8 }}>
        <span style={{ fontSize: 12.5, color: 'var(--cg-text-secondary)' }}>多把 Key 间轮询转发；号池由外部维护，这里只做启用/禁用</span>
        <Button size="small" icon={<PlusOutlined />} onClick={() => { form.resetFields(); setOpen(true) }}>添加 Key</Button>
      </div>
      <Table size="small" rowKey="id" loading={keys.isLoading} columns={columns} dataSource={keys.data ?? []} pagination={false} />
      <Modal title="添加上游 Key" open={open} onCancel={() => setOpen(false)} onOk={addKey} okText="添加" confirmLoading={createKey.isPending} destroyOnClose>
        <Form form={form} layout="vertical">
          <Form.Item name="name" label="名称" rules={[{ required: true, message: '请输入名称' }]}><Input placeholder="例如：kiro-pool-01" /></Form.Item>
          <Form.Item name="credential" label="凭证（明文，将 AES 加密存储）" rules={[{ required: true, message: '请输入凭证' }]}><Input.Password placeholder="sk-..." /></Form.Item>
        </Form>
      </Modal>
    </div>
  )
}

export function Channels() {
  const { message } = App.useApp()
  const channels = useChannels()
  const saveChannel = useSaveChannel()
  const toggleChannel = useToggleChannel()
  const [open, setOpen] = useState(false)
  const [formType, setFormType] = useState<ChannelType>('kiro')
  const [editing, setEditing] = useState<Channel | null>(null)
  const [form] = Form.useForm()

  const openEdit = (c: Channel) => {
    setEditing(c)
    setFormType(c.type)
    form.setFieldsValue({ name: c.name, base_url: c.base_url, config: c.config ?? {} })
    setOpen(true)
  }
  const openCreate = () => {
    setEditing(null)
    setFormType('kiro')
    form.resetFields()
    setOpen(true)
  }
  const submit = async () => {
    const v = await form.validateFields()
    try {
      await saveChannel.mutateAsync({ id: editing?.id, name: v.name, type: formType, base_url: v.base_url ?? '', config: v.config ?? {}, enabled: editing?.enabled ?? true })
      message.success(editing ? '通道已更新' : '通道已创建')
      setOpen(false)
    } catch (e) {
      message.error(e instanceof Error ? e.message : '保存失败')
    }
  }

  const columns = [
    {
      title: '通道',
      key: 'name',
      render: (_: unknown, c: Channel) => (
        <div>
          <div style={{ fontWeight: 600 }}>{c.name}</div>
          {c.base_url && <div className="cg-mono" style={{ fontSize: 11.5, color: 'var(--cg-text-secondary)' }}>{c.base_url}</div>}
        </div>
      ),
    },
    { title: '类型', dataIndex: 'type', width: 100, render: (t: ChannelType) => <ChannelTag type={t} /> },
    { title: 'Key 数量', dataIndex: 'key_count', width: 100, render: (v: number) => <Tag>{v} 把</Tag> },
    {
      title: '配置',
      key: 'config',
      width: 260,
      render: (_: unknown, c: Channel) => (
        <span className="cg-mono" style={{ fontSize: 11, color: 'var(--cg-text-secondary)' }}>
          {Object.entries(c.config ?? {}).slice(0, 2).map(([k, v]) => `${k}=${v}`).join('  ')}
        </span>
      ),
    },
    { title: '状态', dataIndex: 'enabled', width: 90, render: (v: boolean, c: Channel) => <Switch checked={v} size="small" onChange={() => toggleChannel.mutate(c.id)} /> },
    {
      title: '操作',
      key: 'action',
      width: 80,
      render: (_: unknown, c: Channel) => (
        <Button type="link" icon={<SettingOutlined />} style={{ padding: 0 }} onClick={() => openEdit(c)}>配置</Button>
      ),
    },
  ]

  return (
    <Card
      className="cg-soft-card"
      styles={{ body: { padding: 18 } }}
      title={<span style={{ color: 'var(--cg-text-secondary)', fontSize: 13 }}>claude-gate 只做中间层：上游 Key 直接配置、多把轮询；展开行管理 Key</span>}
      extra={<Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>新建通道</Button>}
    >
      <Table<Channel>
        rowKey="id"
        loading={channels.isLoading}
        columns={columns}
        dataSource={channels.data ?? []}
        pagination={false}
        expandable={{ expandedRowRender: (c) => <KeyPool channelId={c.id} />, defaultExpandedRowKeys: [1] }}
      />

      <Modal title={editing ? '编辑通道' : '通道配置'} open={open} width={560} onCancel={() => setOpen(false)} onOk={submit} okText="保存" confirmLoading={saveChannel.isPending}>
        <Segmented<ChannelType>
          block
          value={formType}
          onChange={setFormType}
          options={[{ label: 'Kiro', value: 'kiro' }, { label: '官方', value: 'official' }, { label: '中转', value: 'relay' }]}
          style={{ marginBottom: 18 }}
        />
        <Form form={form} layout="vertical">
          <Form.Item label="通道名称" name="name" rules={[{ required: true, message: '请输入名称' }]}><Input placeholder="例如：Kiro 主通道" /></Form.Item>
          <ChannelConfigFields type={formType} />
        </Form>
      </Modal>
    </Card>
  )
}
