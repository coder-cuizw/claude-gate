import { Card, Input, Segmented, Select, Table, Tag } from 'antd'
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

  const tokCell = (v: number) => <span className="cg-mono" style={{ fontSize: 12.5 }}>{fmtInt(v)}</span>
  const columns = [
    {
      title: '时间',
      dataIndex: 'request_at',
      width: 156,
      fixed: 'left' as const,
      render: (v: string) => <span style={{ color: 'var(--cg-text-secondary)', fontSize: 12.5 }}>{fmtDateTime(v)}</span>,
    },
    { title: '通道', dataIndex: 'channel_type', width: 78, render: (t: TraceListItem['channel_type']) => <ChannelTag type={t} /> },
    { title: '分组', dataIndex: 'group_name', width: 150, ellipsis: true },
    { title: '模型', dataIndex: 'model', width: 184, ellipsis: true, render: (m: string) => <span className="cg-mono" style={{ fontSize: 11.5 }}>{m}</span> },
    {
      title: '类型',
      key: 'streaming',
      width: 84,
      render: (_: unknown, r: TraceListItem) =>
        r.is_streaming ? (
          <Tag icon={<ThunderboltOutlined />} color="processing" style={{ borderRadius: 6 }}>流式</Tag>
        ) : (
          <Tag style={{ borderRadius: 6 }}>非流</Tag>
        ),
    },
    {
      title: '状态',
      key: 'status',
      width: 116,
      render: (_: unknown, r: TraceListItem) => <StatusTag success={r.is_success} code={r.status_code} />,
    },
    {
      title: '首字',
      dataIndex: 'ttft_ms',
      width: 92,
      align: 'right' as const,
      render: (v: number, r: TraceListItem) =>
        r.is_streaming ? <span style={{ color: 'var(--cg-accent,#C45A35)', fontWeight: 600 }}>{fmtMs(v)}</span> : <span style={{ color: 'var(--cg-text-tertiary,#928e85)' }}>{fmtMs(v)}</span>,
    },
    { title: '输入', dataIndex: ['billed_usage', 'input_tokens'], width: 86, align: 'right' as const, render: tokCell },
    { title: '输出', dataIndex: ['billed_usage', 'output_tokens'], width: 86, align: 'right' as const, render: tokCell },
    { title: '缓存创建', dataIndex: ['billed_usage', 'cache_creation_tokens'], width: 98, align: 'right' as const, render: tokCell },
    { title: '缓存读取', dataIndex: ['billed_usage', 'cache_read_tokens'], width: 98, align: 'right' as const, render: tokCell },
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
        scroll={{ x: 1140 }}
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
