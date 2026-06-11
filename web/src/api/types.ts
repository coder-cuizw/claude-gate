// 与后端 domain 对齐的前端类型定义（任务书 §4 / §5.7）。

export type ChannelType = 'kiro' | 'official' | 'relay' | 'custom'

export type KeyStatus = 'active' | 'disabled'

export interface Channel {
  id: number
  name: string
  type: ChannelType
  base_url: string
  config: Record<string, unknown>
  enabled: boolean
  key_count: number
  created_at: string
}

export interface UpstreamKey {
  id: number
  channel_id: number
  name: string
  status: KeyStatus
  last_error?: string
  last_used_at?: string | null
  created_at: string
}

export type CacheStrategyType = 'passthrough' | 'percentage' | 'fixed' | 'formula'

export interface CacheStrategy {
  type: CacheStrategyType
  params?: Record<string, unknown>
}

export interface TransformerItem {
  name: string
  enabled: boolean
  params?: Record<string, unknown>
}

export interface Group {
  id: number
  name: string
  description: string
  channel_id: number
  channel_name: string
  channel_type: ChannelType
  cache_strategy: CacheStrategy
  transformer_config: TransformerItem[]
  rate_limit_config: { rpm?: number; tpm?: number }
  retry_config: { max_retries?: number; backoff_ms?: number }
  enabled: boolean
  created_at: string
}

export interface ApiKey {
  id: number
  key_prefix: string
  name: string
  group_id: number
  group_name: string
  enabled: boolean
  expires_at?: string | null
  created_at: string
  last_used_at?: string | null
  request_count: number
}

export interface ModelMapping {
  id: number
  channel_id: number
  client_model: string
  upstream_model: string
}

export interface Usage {
  input_tokens: number
  output_tokens: number
  cache_creation_tokens: number
  cache_read_tokens: number
}

export interface TraceListItem {
  trace_id: string
  request_at: string
  group_id: number
  group_name: string
  channel_type: ChannelType
  model: string
  status_code: number
  is_success: boolean
  is_streaming: boolean
  error_type?: string
  ttft_ms: number
  duration_ms: number
  total_tokens: number
  // 返回给客户的计费 usage（明细列表按 new-api 风格分列展示输入/输出/缓存创建/读取）
  billed_usage: Usage
}

export interface TraceDetail extends TraceListItem {
  api_key_id: number
  api_key_name: string
  upstream_key_id: number
  upstream_key_name: string
  error_message?: string
  billed_usage: Usage
  upstream_usage: Usage
  request_body: unknown
  response_body: unknown
  meta: Record<string, unknown>
}

export interface StatsOverview {
  request_count: number
  success_rate: number
  avg_ttft_ms: number
  p95_ttft_ms: number
  p99_ttft_ms: number
  max_ttft_ms: number
  total_tokens: number
  error_count: number
}

export interface TimeseriesPoint {
  timestamp: string
  value: number
  series?: string
}

export interface ErrorBucket {
  error_type: string
  count: number
}

export interface ByChannelStat {
  channel_type: ChannelType
  request_count: number
  success_rate: number
  avg_ttft_ms: number
}

export type TimeWindow = '5m' | '1h' | '24h' | '7d'

export interface Paged<T> {
  items: T[]
  total: number
  page: number
  page_size: number
}
