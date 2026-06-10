import { App, Button, Card, Descriptions, Form, Input, Modal, Segmented, Select, Switch, Table, Tag, Tooltip } from 'antd'
import { PlusOutlined, ReloadOutlined, SettingOutlined } from '@ant-design/icons'
import { useState } from 'react'
import { ChannelTag, KeyStatusTag } from '../components/tags'
import { useChannels, useUpstreamKeys } from '../api/queries'
import type { Channel, ChannelType, UpstreamKey } from '../api/types'
import { fmtDateTime } from '../utils/format'

/** 按通道类型渲染差异化配置字段（任务书 §7 通道配置表单）。 */
function ChannelConfigFields({ type }: { type: ChannelType }) {
  switch (type) {
    case 'kiro':
      return (
        <>
          <Form.Item label="SSO Start URL" name={['config', 'sso_start_url']}><Input placeholder="https://sso.kiro.internal/start" /></Form.Item>
          <Form.Item label="Region" name={['config', 'region']}><Input placeholder="us-east-1" /></Form.Item>
          <Form.Item label="令牌刷新端点" name={['config', 'token_refresh_endpoint']}><Input placeholder="/oauth/token" /></Form.Item>
          <Tag color="warning" style={{ marginBottom: 8 }}>Kiro 为私有逆向通道：认证/协议/流式/令牌刷新由 KiroAdapter 内部处理</Tag>
        </>
      )
    case 'official':
      return (
        <>
          <Form.Item label="Base URL" name="base_url"><Input placeholder="https://api.anthropic.com" /></Form.Item>
          <Form.Item label="anthropic-version" name={['config', 'anthropic_version']}><Input placeholder="2023-06-01" /></Form.Item>
        </>
      )
    case 'bedrock':
      return (
        <>
          <Form.Item label="AWS Region" name={['config', 'region']}><Input placeholder="us-east-1" /></Form.Item>
          <Form.Item label="anthropic-version" name={['config', 'anthropic_version']}><Input placeholder="bedrock-2023-05-31" /></Form.Item>
          <Tag color="default">凭证：AK/SK 或 Role，走 aws-sdk-go-v2 官方签名</Tag>
        </>
      )
    case 'vertex':
      return (
        <>
          <Form.Item label="GCP Project ID" name={['config', 'project_id']}><Input placeholder="claude-gate-prod" /></Form.Item>
          <Form.Item label="Region" name={['config', 'region']}><Input placeholder="us-east5" /></Form.Item>
          <Tag color="default">凭证：服务账号 JSON，走 GCP 官方鉴权</Tag>
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
  }
}

function KeyPool({ channelId, channelType }: { channelId: number; channelType: ChannelType }) {
  const keys = useUpstreamKeys(channelId)
  const columns = [
    { title: 'Key 名称', dataIndex: 'name', render: (v: string) => <span className="cg-mono" style={{ fontSize: 12 }}>{v}</span> },
    { title: '状态', dataIndex: 'status', width: 100, render: (s: UpstreamKey['status']) => <KeyStatusTag status={s} /> },
    { title: '最近使用', dataIndex: 'last_used_at', width: 160, render: (v?: string | null) => (v ? fmtDateTime(v) : '—') },
    ...(channelType === 'kiro'
      ? [{ title: '令牌刷新', dataIndex: 'refreshed_at', width: 160, render: (v?: string | null) => (v ? <Tooltip title="后台主动刷新刷新型凭证"><Tag color="processing">{fmtDateTime(v)}</Tag></Tooltip> : '—') }]
      : []),
    { title: '最近错误', dataIndex: 'last_error', ellipsis: true, render: (v?: string) => (v ? <span style={{ color: '#C0492F', fontSize: 12 }}>{v}</span> : <span style={{ color: 'var(--cg-text-tertiary,#928e85)' }}>—</span>) },
  ]
  return (
    <div style={{ padding: '4px 0' }}>
      <Table size="small" rowKey="id" loading={keys.isLoading} columns={columns} dataSource={keys.data ?? []} pagination={false} />
    </div>
  )
}

export function Channels() {
  const { message } = App.useApp()
  const channels = useChannels()
  const [open, setOpen] = useState(false)
  const [formType, setFormType] = useState<ChannelType>('kiro')
  const [form] = Form.useForm()

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
          {Object.entries(c.config).slice(0, 2).map(([k, v]) => `${k}=${v}`).join('  ')}
        </span>
      ),
    },
    { title: '状态', dataIndex: 'enabled', width: 90, render: (v: boolean) => <Switch checked={v} size="small" /> },
    {
      title: '操作',
      key: 'action',
      width: 80,
      render: (_: unknown, c: Channel) => (
        <Button type="link" icon={<SettingOutlined />} style={{ padding: 0 }} onClick={() => { setFormType(c.type); setOpen(true) }}>配置</Button>
      ),
    },
  ]

  return (
    <Card
      className="cg-soft-card"
      styles={{ body: { padding: 18 } }}
      title={<span style={{ color: 'var(--cg-text-secondary)', fontSize: 13 }}>各通道差异由上游适配层吸收；展开行查看 Key 池与令牌状态</span>}
      extra={<Button type="primary" icon={<PlusOutlined />} onClick={() => { setFormType('kiro'); setOpen(true) }}>新建通道</Button>}
    >
      <Table<Channel>
        rowKey="id"
        loading={channels.isLoading}
        columns={columns}
        dataSource={channels.data ?? []}
        pagination={false}
        expandable={{
          expandedRowRender: (c) => <KeyPool channelId={c.id} channelType={c.type} />,
          defaultExpandedRowKeys: [1],
        }}
      />

      <Modal
        title="通道配置"
        open={open}
        width={560}
        onCancel={() => setOpen(false)}
        onOk={() => { setOpen(false); message.success('通道已保存（演示）') }}
        okText="保存"
      >
        <Segmented<ChannelType>
          block
          value={formType}
          onChange={setFormType}
          options={[
            { label: 'Kiro', value: 'kiro' },
            { label: '官方', value: 'official' },
            { label: 'Bedrock', value: 'bedrock' },
            { label: 'Vertex', value: 'vertex' },
            { label: '中转', value: 'relay' },
          ]}
          style={{ marginBottom: 18 }}
        />
        <Form form={form} layout="vertical">
          <Form.Item label="通道名称" name="name"><Input placeholder="例如：Kiro 主通道" /></Form.Item>
          <ChannelConfigFields type={formType} />
          <Descriptions size="small" column={1} style={{ marginTop: 8 }}>
            <Descriptions.Item label={<span style={{ color: 'var(--cg-text-secondary)' }}>提示</span>}>
              <Button size="small" icon={<ReloadOutlined />} disabled={formType !== 'kiro'}>
                立即刷新令牌
              </Button>
            </Descriptions.Item>
          </Descriptions>
        </Form>
      </Modal>
    </Card>
  )
}
