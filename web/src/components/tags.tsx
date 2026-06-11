import { Tag } from 'antd'
import { channelColors } from '../theme/tokens'
import type { CacheStrategyType, ChannelType, KeyStatus } from '../api/types'

const channelLabel: Record<ChannelType, string> = {
  kiro: 'Kiro',
  official: '官方',
  relay: '中转',
  custom: '本地',
}

/** 通道类型标签，颜色与大盘一致。 */
export function ChannelTag({ type }: { type: ChannelType }) {
  return (
    <Tag color={channelColors[type]} style={{ borderRadius: 6, fontWeight: 500 }}>
      {channelLabel[type]}
    </Tag>
  )
}

const strategyLabel: Record<CacheStrategyType, string> = {
  passthrough: '透传',
  percentage: '百分比',
  fixed: '固定值',
  formula: '公式',
}
const strategyColor: Record<CacheStrategyType, string> = {
  passthrough: 'default',
  percentage: 'geekblue',
  fixed: 'gold',
  formula: 'purple',
}

/** 缓存策略标签。 */
export function StrategyTag({ type }: { type: CacheStrategyType }) {
  return <Tag color={strategyColor[type]}>{strategyLabel[type]}</Tag>
}

/** 请求状态标签（成功/失败 + 状态码）。 */
export function StatusTag({ success, code }: { success: boolean; code: number }) {
  return (
    <Tag color={success ? 'success' : 'error'} style={{ borderRadius: 6 }}>
      {success ? '成功' : '失败'} · {code}
    </Tag>
  )
}

const keyStatusMap: Record<KeyStatus, { color: string; label: string }> = {
  active: { color: 'success', label: '可用' },
  disabled: { color: 'default', label: '已禁用' },
}

/** 上游 Key 状态标签。 */
export function KeyStatusTag({ status }: { status: KeyStatus }) {
  const m = keyStatusMap[status]
  return <Tag color={m.color}>{m.label}</Tag>
}
