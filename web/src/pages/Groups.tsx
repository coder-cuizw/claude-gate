import { Button, Card, Switch, Table } from 'antd'
import { EditOutlined, PlusOutlined } from '@ant-design/icons'
import { useNavigate } from 'react-router-dom'
import { ChannelTag, StrategyTag } from '../components/tags'
import { useGroups } from '../api/queries'
import type { Group } from '../api/types'
import { fmtInt } from '../utils/format'

export function Groups() {
  const navigate = useNavigate()
  const groups = useGroups()

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
    { title: '通道', dataIndex: 'channel_type', width: 100, render: (t: Group['channel_type']) => <ChannelTag type={t} /> },
    { title: '缓存策略', key: 'strategy', width: 110, render: (_: unknown, g: Group) => <StrategyTag type={g.cache_strategy.type} /> },
    {
      title: '限流 (rpm / tpm)',
      key: 'rate',
      width: 180,
      render: (_: unknown, g: Group) => (
        <span className="cg-mono" style={{ fontSize: 12 }}>
          {fmtInt(g.rate_limit_config.rpm ?? 0)} / {fmtInt(g.rate_limit_config.tpm ?? 0)}
        </span>
      ),
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
        <Button type="primary" icon={<PlusOutlined />} onClick={() => navigate('/groups/101')}>
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
