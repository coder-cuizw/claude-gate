import { Card, Skeleton, Tooltip } from 'antd'
import { InfoCircleOutlined } from '@ant-design/icons'
import type { ReactNode } from 'react'

interface Props {
  label: string
  value: ReactNode
  suffix?: ReactNode
  hint?: string
  trend?: { value: string; up: boolean; good?: boolean }
  loading?: boolean
  accent?: boolean
}

/** 大盘指标卡：大号数值 + 可选趋势 + 说明。 */
export function StatCard({ label, value, suffix, hint, trend, loading, accent }: Props) {
  return (
    <Card
      className="cg-soft-card"
      styles={{ body: { padding: '18px 20px' } }}
      style={accent ? { background: 'var(--cg-accent-soft)' } : undefined}
    >
      <div style={{ display: 'flex', alignItems: 'center', gap: 6, color: 'var(--cg-text-secondary)', fontSize: 13 }}>
        {label}
        {hint && (
          <Tooltip title={hint}>
            <InfoCircleOutlined style={{ fontSize: 12, opacity: 0.6 }} />
          </Tooltip>
        )}
      </div>
      {loading ? (
        <Skeleton.Button active style={{ height: 34, marginTop: 10, width: 120 }} />
      ) : (
        <div style={{ marginTop: 6, display: 'flex', alignItems: 'baseline', gap: 8 }}>
          <span className="cg-serif" style={{ fontSize: 30, fontWeight: 600, lineHeight: 1.1, letterSpacing: '-0.02em' }}>
            {value}
          </span>
          {suffix && <span style={{ color: 'var(--cg-text-secondary)', fontSize: 14 }}>{suffix}</span>}
          {trend && (
            <span
              style={{
                marginLeft: 'auto',
                fontSize: 12.5,
                fontWeight: 500,
                color: trend.good ? 'var(--cg-success, #5B8C5A)' : '#C0492F',
              }}
            >
              {trend.up ? '▲' : '▼'} {trend.value}
            </span>
          )}
        </div>
      )}
    </Card>
  )
}
