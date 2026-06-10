import { Badge, Card, Col, Row, Segmented, Table } from 'antd'
import { useState } from 'react'
import { AreaChart, ColumnChart, DonutChart, LineChart } from '../components/charts'
import { StatCard } from '../components/StatCard'
import { ChannelTag } from '../components/tags'
import { useByChannel, useErrors, useOverview, useTimeseries } from '../api/queries'
import type { ByChannelStat, TimeWindow } from '../api/types'
import { fmtCompact, fmtInt, fmtMs, fmtPct } from '../utils/format'

const windowOptions: { label: string; value: TimeWindow }[] = [
  { label: '近 5 分钟', value: '5m' },
  { label: '近 1 小时', value: '1h' },
  { label: '近 24 小时', value: '24h' },
  { label: '近 7 天', value: '7d' },
]

function ChartCard({ title, extra, children }: { title: string; extra?: React.ReactNode; children: React.ReactNode }) {
  return (
    <Card className="cg-soft-card" styles={{ body: { padding: 18 } }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 10 }}>
        <span style={{ fontWeight: 600, fontSize: 14.5 }}>{title}</span>
        {extra}
      </div>
      {children}
    </Card>
  )
}

export function Dashboard() {
  const [win, setWin] = useState<TimeWindow>('1h')
  const overview = useOverview(win)
  const qps = useTimeseries(win, 'qps')
  const ttft = useTimeseries(win, 'ttft_p95')
  const tokens = useTimeseries(win, 'tokens')
  const errRate = useTimeseries(win, 'error_rate')
  const errors = useErrors(win)
  const byChannel = useByChannel(win)

  const o = overview.data

  const channelColumns = [
    { title: '通道', dataIndex: 'channel_type', render: (t: ByChannelStat['channel_type']) => <ChannelTag type={t} /> },
    { title: '请求数', dataIndex: 'request_count', align: 'right' as const, render: (v: number) => fmtInt(v) },
    {
      title: '成功率',
      dataIndex: 'success_rate',
      align: 'right' as const,
      render: (v: number) => <span style={{ color: v >= 0.97 ? 'var(--cg-success,#5B8C5A)' : '#C9952B', fontWeight: 500 }}>{fmtPct(v, 1)}</span>,
    },
    { title: '平均 TTFT', dataIndex: 'avg_ttft_ms', align: 'right' as const, render: (v: number) => fmtMs(v) },
  ]

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
      {/* 顶部：时间窗口 + 自动刷新 */}
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', flexWrap: 'wrap', gap: 12 }}>
        <Segmented value={win} onChange={(v) => setWin(v as TimeWindow)} options={windowOptions} />
        <Badge status="processing" text={<span style={{ color: 'var(--cg-text-secondary)', fontSize: 13 }}>每 5 秒自动刷新</span>} />
      </div>

      {/* 指标卡 */}
      <Row gutter={[14, 14]}>
        <Col xs={12} md={8} xl={6}>
          <StatCard label="总请求数" value={fmtCompact(o?.request_count ?? 0)} hint="所选时间窗口内的请求总量" trend={{ value: '8.4%', up: true, good: true }} loading={overview.isLoading} />
        </Col>
        <Col xs={12} md={8} xl={6}>
          <StatCard label="成功率" value={o ? fmtPct(o.success_rate, 1) : '—'} trend={{ value: '0.3%', up: true, good: true }} loading={overview.isLoading} accent />
        </Col>
        <Col xs={12} md={8} xl={6}>
          <StatCard label="平均 TTFT" value={o ? fmtInt(o.avg_ttft_ms) : '—'} suffix="ms" hint="首 token 到达时间均值" loading={overview.isLoading} />
        </Col>
        <Col xs={12} md={8} xl={6}>
          <StatCard label="P95 TTFT" value={o ? fmtInt(o.p95_ttft_ms) : '—'} suffix="ms" loading={overview.isLoading} />
        </Col>
        <Col xs={12} md={8} xl={6}>
          <StatCard label="P99 TTFT" value={o ? fmtInt(o.p99_ttft_ms) : '—'} suffix="ms" loading={overview.isLoading} />
        </Col>
        <Col xs={12} md={8} xl={6}>
          <StatCard label="最大 TTFT" value={o ? fmtInt(o.max_ttft_ms) : '—'} suffix="ms" loading={overview.isLoading} />
        </Col>
        <Col xs={12} md={8} xl={6}>
          <StatCard label="Token 消耗" value={o ? fmtCompact(o.total_tokens) : '—'} hint="计费口径的 token 总量" loading={overview.isLoading} />
        </Col>
        <Col xs={12} md={8} xl={6}>
          <StatCard label="错误数" value={o ? fmtCompact(o.error_count) : '—'} trend={{ value: '2.1%', up: false, good: true }} loading={overview.isLoading} />
        </Col>
      </Row>

      {/* 主图表 */}
      <Row gutter={[14, 14]}>
        <Col xs={24} xl={12}>
          <ChartCard title="QPS（每秒请求数）">
            <AreaChart data={qps.data ?? []} color="#C45A35" />
          </ChartCard>
        </Col>
        <Col xs={24} xl={12}>
          <ChartCard title="TTFT 分位数（P50 / P95 / P99）">
            <LineChart data={ttft.data ?? []} multi />
          </ChartCard>
        </Col>
        <Col xs={24} xl={12}>
          <ChartCard title="Token 消耗（输入 / 输出 / 缓存读取）">
            <AreaChart data={tokens.data ?? []} multi />
          </ChartCard>
        </Col>
        <Col xs={24} xl={12}>
          <ChartCard title="错误率（%）">
            <LineChart data={errRate.data ?? []} color="#C0492F" />
          </ChartCard>
        </Col>
      </Row>

      {/* 按通道对比 + 错误分布 */}
      <Row gutter={[14, 14]}>
        <Col xs={24} xl={9}>
          <ChartCard title="各通道请求量对比">
            <ColumnChart data={(byChannel.data ?? []).map((d) => ({ ...d }))} xField="channel_type" yField="request_count" />
          </ChartCard>
        </Col>
        <Col xs={24} xl={8}>
          <ChartCard title="通道健康度">
            <Table
              size="small"
              rowKey="channel_type"
              loading={byChannel.isLoading}
              pagination={false}
              columns={channelColumns}
              dataSource={byChannel.data ?? []}
            />
          </ChartCard>
        </Col>
        <Col xs={24} xl={7}>
          <ChartCard title="错误类型分布">
            <DonutChart data={(errors.data ?? []).map((e) => ({ ...e }))} angleField="count" colorField="error_type" />
          </ChartCard>
        </Col>
      </Row>
    </div>
  )
}
