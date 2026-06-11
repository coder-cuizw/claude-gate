import { App, Button, Card, Space, Switch, Table } from 'antd'
import { EditOutlined, PlusOutlined } from '@ant-design/icons'
import { useNavigate } from 'react-router-dom'
import { ChannelTag, StrategyTag } from '../components/tags'
import { useChannels, useGroups, useSaveGroup } from '../api/queries'
import type { Group } from '../api/types'
import { fmtInt } from '../utils/format'

export function Groups() {
  const navigate = useNavigate()
  const { message } = App.useApp()
  const groups = useGroups()
  const channels = useChannels()
  const saveGroup = useSaveGroup()

  const onCreate = async () => {
    const channelId = channels.data?.[0]?.id
    if (!channelId) {
      message.warning('请先创建上游通道')
      return
    }
    try {
      const res = (await saveGroup.mutateAsync({
        name: `新分组-${Date.now() % 10000}`,
        description: '',
        channel_id: channelId,
        enabled: true,
        cache_strategy: { type: 'passthrough' },
        transformer_config: [],
        rate_limit_config: { rpm: 0, tpm: 0 },
        retry_config: { max_retries: 0, backoff_ms: 0 },
      })) as { id: number }
      navigate(`/groups/${res.id}`)
    } catch (e) {
      message.error(e instanceof Error ? e.message : '创建失败')
    }
  }

  const columns = [
    {
      title: '分组',
      key: 'name',
      render: (_: unknown, g: Group) => (
        <div>
          <div style={{ fontWeight: 600 }}>{g.name}</div>
          <div style={{ fontSize: 12, color: 'var(--cg-text-secondary)' }}>{g.description}</div>
        </div>
      ),
    },
    {
      title: '绑定通道',
      key: 'channel',
      width: 220,
      render: (_: unknown, g: Group) => (
        <Space size={6}>
          <ChannelTag type={g.channel_type} />
          <span style={{ fontSize: 12.5 }}>{g.channel_name ?? `#${g.channel_id}`}</span>
        </Space>
      ),
    },
    { title: '缓存策略', key: 'strategy', width: 110, render: (_: unknown, g: Group) => <StrategyTag type={g.cache_strategy.type} /> },
    {
      title: '限流 (rpm / tpm)',
      key: 'rate',
      width: 180,
      render: (_: unknown, g: Group) => {
        const rpm = g.rate_limit_config.rpm ?? 0
        const tpm = g.rate_limit_config.tpm ?? 0
        const fmt = (v: number) => (v > 0 ? fmtInt(v) : <span style={{ color: 'var(--cg-text-tertiary,#928e85)' }}>不限</span>)
        return (
          <span className="cg-mono" style={{ fontSize: 12 }}>
            {fmt(rpm)} / {fmt(tpm)}
          </span>
        )
      },
    },
    {
      title: 'Transformer',
      key: 'tf',
      width: 120,
      render: (_: unknown, g: Group) => (
        <span style={{ color: 'var(--cg-text-secondary)' }}>{g.transformer_config.filter((t) => t.enabled).length} 个启用</span>
      ),
    },
    {
      title: '状态',
      dataIndex: 'enabled',
      width: 90,
      render: (v: boolean) => <Switch checked={v} size="small" />,
    },
    {
      title: '操作',
      key: 'action',
      width: 90,
      render: (_: unknown, g: Group) => (
        <Button type="link" icon={<EditOutlined />} onClick={() => navigate(`/groups/${g.id}`)} style={{ padding: 0 }}>
          编辑
        </Button>
      ),
    },
  ]

  return (
    <Card
      className="cg-soft-card"
      styles={{ body: { padding: 18 } }}
      title={<span style={{ color: 'var(--cg-text-secondary)', fontSize: 13 }}>分组是 claude-gate 最核心的配置单元：绑定通道、缓存计费策略与改写流水线</span>}
      extra={
        <Button type="primary" icon={<PlusOutlined />} loading={saveGroup.isPending} onClick={onCreate}>
          新建分组
        </Button>
      }
    >
      <Table<Group>
        rowKey="id"
        loading={groups.isLoading}
        columns={columns}
        dataSource={groups.data ?? []}
        pagination={false}
        onRow={(g) => ({ onClick: () => navigate(`/groups/${g.id}`), style: { cursor: 'pointer' } })}
      />
    </Card>
  )
}
