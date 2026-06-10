import { Card, Input, Segmented, Select, Space, Table, Tag } from 'antd'
import { ThunderboltOutlined } from '@ant-design/icons'
import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { ChannelTag, StatusTag } from '../components/tags'
import { useGroups, useTraces } from '../api/queries'
import type { TraceListItem } from '../api/types'
import { fmtDateTime, fmtInt, fmtMs } from '../utils/format'

export function Traces() {
  const navigate = useNavigate()
  const [status, setStatus] = useState('all')
  const [channelType, setChannelType] = useState<string | undefined>()
  const [groupId, setGroupId] = useState<number | undefined>()
  const [page, setPage] = useState(1)
  const pageSize = 12

  const groups = useGroups()
  const traces = useTraces({ status, channel_type: channelType ?? 'all', group_id: groupId, page, page_size: pageSize })

  const columns = [
    {
      title: '时间',
      dataIndex: 'request_at',
      width: 168,
      render: (v: string) => <span style={{ color: 'var(--cg-text-secondary)' }}>{fmtDateTime(v)}</span>,
    },
    {
      title: 'Trace ID',
      dataIndex: 'trace_id',
      width: 150,
      render: (v: string) => <span className="cg-mono" style={{ fontSize: 12 }}>{v.slice(0, 12)}…</span>,
    },
    { title: '分组', dataIndex: 'group_name', ellipsis: true },
    { title: '通道', dataIndex: 'channel_type', width: 90, render: (t: TraceListItem['channel_type']) => <ChannelTag type={t} /> },
    { title: '模型', dataIndex: 'model', ellipsis: true, render: (m: string) => <span className="cg-mono" style={{ fontSize: 11.5 }}>{m}</span> },
    {
      title: '状态',
      key: 'status',
      width: 118,
      render: (_: unknown, r: TraceListItem) => (
        <Space size={4}>
          <StatusTag success={r.is_success} code={r.status_code} />
          {r.is_streaming && <ThunderboltOutlined style={{ color: 'var(--cg-text-tertiary,#928e85)', fontSize: 12 }} title="流式" />}
        </Space>
      ),
    },
    { title: 'TTFT', dataIndex: 'ttft_ms', width: 92, align: 'right' as const, render: (v: number) => fmtMs(v) },
    { title: '耗时', dataIndex: 'duration_ms', width: 96, align: 'right' as const, render: (v: number) => `${(v / 1000).toFixed(1)} s` },
    { title: 'Tokens', dataIndex: 'total_tokens', width: 100, align: 'right' as const, render: (v: number) => fmtInt(v) },
  ]

  return (
    <Card className="cg-soft-card" styles={{ body: { padding: 18 } }}>
      <div style={{ display: 'flex', flexWrap: 'wrap', gap: 12, marginBottom: 16, alignItems: 'center' }}>
        <Segmented
          value={status}
          onChange={(v) => { setStatus(v as string); setPage(1) }}
          options={[
            { label: '全部', value: 'all' },
            { label: '成功', value: 'success' },
            { label: '失败', value: 'error' },
          ]}
        />
        <Select
          allowClear
          placeholder="按通道类型"
          style={{ width: 150 }}
          value={channelType}
          onChange={(v) => { setChannelType(v); setPage(1) }}
          options={[
            { label: 'Kiro', value: 'kiro' },
            { label: '官方', value: 'official' },
            { label: 'Bedrock', value: 'bedrock' },
            { label: 'Vertex', value: 'vertex' },
            { label: '中转', value: 'relay' },
          ]}
        />
        <Select
          allowClear
          placeholder="按分组"
          style={{ width: 200 }}
          value={groupId}
          onChange={(v) => { setGroupId(v); setPage(1) }}
          options={(groups.data ?? []).map((g) => ({ label: g.name, value: g.id }))}
        />
        <Input.Search placeholder="搜索 Trace ID / 模型" style={{ width: 240, marginLeft: 'auto' }} allowClear />
      </div>

      <Table<TraceListItem>
        rowKey="trace_id"
        size="middle"
        loading={traces.isLoading}
        columns={columns}
        dataSource={traces.data?.items ?? []}
        onRow={(r) => ({ onClick: () => navigate(`/traces/${r.trace_id}`), style: { cursor: 'pointer' } })}
        pagination={{
          current: page,
          pageSize,
          total: traces.data?.total ?? 0,
          onChange: setPage,
          showTotal: (t) => <span style={{ color: 'var(--cg-text-secondary)' }}>共 {fmtInt(t)} 条记录</span>,
        }}
        footer={() => (
          <span style={{ fontSize: 12.5, color: 'var(--cg-text-secondary)' }}>
            <Tag color="error" style={{ borderRadius: 6 }}>错误 100% 留存</Tag>
            失败请求的完整 body 已落 S3，点击任意行查看详情并一键复现
          </span>
        )}
      />
    </Card>
  )
}
