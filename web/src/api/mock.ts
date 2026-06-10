// 演示用内置 mock 数据层（任务书 §7 约定：面板无 SEO/SSR，纯内部运维 SPA）。
//
// 所有页面默认走这里的 mock，使面板可脱离后端独立运行与截图。
// 接入真实后端时，把 queries.ts 中的实现换成对 /api/admin/* 的请求即可，页面无需改动。

import type {
  ApiKey,
  ByChannelStat,
  Channel,
  ErrorBucket,
  Group,
  ModelMapping,
  Paged,
  StatsOverview,
  TimeWindow,
  TimeseriesPoint,
  TraceDetail,
  TraceListItem,
  UpstreamKey,
} from './types'

// —— 可复现的伪随机数（让截图稳定） ——
function mulberry32(seed: number) {
  return function () {
    seed |= 0
    seed = (seed + 0x6d2b79f5) | 0
    let t = Math.imul(seed ^ (seed >>> 15), 1 | seed)
    t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296
  }
}
const rnd = mulberry32(20260610)
const pick = <T>(arr: T[]) => arr[Math.floor(rnd() * arr.length)]
const randInt = (a: number, b: number) => Math.floor(rnd() * (b - a + 1)) + a

const delay = (ms = 260) => new Promise((r) => setTimeout(r, ms))
const iso = (d: Date) => d.toISOString()
const now = () => new Date('2026-06-10T20:30:00Z')

const MODELS = [
  'claude-3-5-sonnet-20241022',
  'claude-3-5-haiku-20241022',
  'claude-3-opus-20240229',
  'claude-sonnet-4-20250514',
]

// —— 通道 ——
export const channels: Channel[] = [
  {
    id: 1,
    name: 'Kiro 主通道',
    type: 'kiro',
    base_url: 'https://prod.kiro.internal',
    config: { sso_start_url: 'https://sso.kiro.internal/start', region: 'us-east-1', token_refresh_endpoint: '/oauth/token' },
    enabled: true,
    key_count: 4,
    created_at: '2026-04-02T09:12:00Z',
  },
  {
    id: 2,
    name: 'Anthropic 官方',
    type: 'official',
    base_url: 'https://api.anthropic.com',
    config: { anthropic_version: '2023-06-01' },
    enabled: true,
    key_count: 3,
    created_at: '2026-04-02T09:20:00Z',
  },
  {
    id: 3,
    name: 'AWS Bedrock 美东',
    type: 'bedrock',
    base_url: '',
    config: { region: 'us-east-1', anthropic_version: 'bedrock-2023-05-31' },
    enabled: true,
    key_count: 2,
    created_at: '2026-04-10T11:00:00Z',
  },
  {
    id: 4,
    name: 'Google Vertex',
    type: 'vertex',
    base_url: '',
    config: { project_id: 'claude-gate-prod', region: 'us-east5' },
    enabled: false,
    key_count: 1,
    created_at: '2026-04-18T15:30:00Z',
  },
  {
    id: 5,
    name: '第三方中转 A',
    type: 'relay',
    base_url: 'https://relay-a.example.com',
    config: { auth_mode: 'bearer' },
    enabled: true,
    key_count: 2,
    created_at: '2026-05-01T08:00:00Z',
  },
]

// —— 上游 Key 池 ——
export const upstreamKeys: UpstreamKey[] = [
  { id: 11, channel_id: 1, name: 'kiro-sso-01', status: 'active', last_used_at: '2026-06-10T20:29:40Z', refreshed_at: '2026-06-10T20:05:00Z', created_at: '2026-04-02T09:13:00Z' },
  { id: 12, channel_id: 1, name: 'kiro-sso-02', status: 'active', last_used_at: '2026-06-10T20:29:10Z', refreshed_at: '2026-06-10T19:58:00Z', created_at: '2026-04-02T09:13:30Z' },
  { id: 13, channel_id: 1, name: 'kiro-sso-03', status: 'cooldown', cooldown_until: '2026-06-10T20:33:00Z', last_error: '429 Too Many Requests', last_used_at: '2026-06-10T20:24:00Z', refreshed_at: '2026-06-10T19:40:00Z', created_at: '2026-04-02T09:14:00Z' },
  { id: 14, channel_id: 1, name: 'kiro-sso-04', status: 'disabled', last_error: '令牌刷新失败：invalid_grant', refreshed_at: '2026-06-09T11:00:00Z', created_at: '2026-04-02T09:14:30Z' },
  { id: 21, channel_id: 2, name: 'official-key-01', status: 'active', last_used_at: '2026-06-10T20:29:55Z', created_at: '2026-04-02T09:21:00Z' },
  { id: 22, channel_id: 2, name: 'official-key-02', status: 'active', last_used_at: '2026-06-10T20:28:00Z', created_at: '2026-04-02T09:21:30Z' },
  { id: 23, channel_id: 2, name: 'official-key-03', status: 'cooldown', cooldown_until: '2026-06-10T20:31:00Z', last_error: '529 Overloaded', last_used_at: '2026-06-10T20:26:00Z', created_at: '2026-04-02T09:22:00Z' },
  { id: 31, channel_id: 3, name: 'bedrock-role-east', status: 'active', last_used_at: '2026-06-10T20:27:00Z', created_at: '2026-04-10T11:01:00Z' },
  { id: 32, channel_id: 3, name: 'bedrock-ak-backup', status: 'active', last_used_at: '2026-06-10T20:20:00Z', created_at: '2026-04-10T11:02:00Z' },
  { id: 41, channel_id: 4, name: 'vertex-sa-json', status: 'disabled', last_error: '通道已停用', created_at: '2026-04-18T15:31:00Z' },
  { id: 51, channel_id: 5, name: 'relay-a-key-01', status: 'active', last_used_at: '2026-06-10T20:25:00Z', created_at: '2026-05-01T08:01:00Z' },
  { id: 52, channel_id: 5, name: 'relay-a-key-02', status: 'active', last_used_at: '2026-06-10T20:18:00Z', created_at: '2026-05-01T08:02:00Z' },
]

// —— 分组 ——
export const groups: Group[] = [
  {
    id: 101,
    name: '高级套餐-透传',
    description: '直连官方，usage 透传，面向对计量精度敏感的客户',
    channel_id: 2,
    channel_name: 'Anthropic 官方',
    channel_type: 'official',
    cache_strategy: { type: 'passthrough' },
    transformer_config: [
      { name: 'model_mapper', enabled: true },
      { name: 'streaming_event_fixer', enabled: true },
    ],
    rate_limit_config: { rpm: 600, tpm: 800000 },
    retry_config: { max_retries: 2, backoff_ms: 500 },
    enabled: true,
    created_at: '2026-04-03T10:00:00Z',
  },
  {
    id: 102,
    name: 'Kiro-百分比计费',
    description: 'Kiro 通道，按上下文比例计 cache，input 固定 1',
    channel_id: 1,
    channel_name: 'Kiro 主通道',
    channel_type: 'kiro',
    cache_strategy: {
      type: 'percentage',
      params: { cache_creation_ratio: 0.1, cache_read_ratio: 0.9, input_fixed_tokens: 1, output_source: 'upstream' },
    },
    transformer_config: [
      { name: 'tool_call_normalizer', enabled: true },
      { name: 'system_prompt_injector', enabled: true, params: { mode: 'inject', text: '请用简洁中文回答' } },
      { name: 'streaming_event_fixer', enabled: true },
    ],
    rate_limit_config: { rpm: 300, tpm: 400000 },
    retry_config: { max_retries: 3, backoff_ms: 800 },
    enabled: true,
    created_at: '2026-04-05T14:00:00Z',
  },
  {
    id: 103,
    name: '基础套餐-固定值',
    description: '固定计费，便于按次结算',
    channel_id: 5,
    channel_name: '第三方中转 A',
    channel_type: 'relay',
    cache_strategy: {
      type: 'fixed',
      params: { input_tokens: 1, output_tokens: 0, cache_creation_tokens: 1000, cache_read_tokens: 5000 },
    },
    transformer_config: [{ name: 'model_mapper', enabled: true }],
    rate_limit_config: { rpm: 120, tpm: 150000 },
    retry_config: { max_retries: 1, backoff_ms: 400 },
    enabled: true,
    created_at: '2026-04-08T09:00:00Z',
  },
  {
    id: 104,
    name: 'Bedrock-公式计费',
    description: '用公式引擎自定义计量口径',
    channel_id: 3,
    channel_name: 'AWS Bedrock 美东',
    channel_type: 'bedrock',
    cache_strategy: {
      type: 'formula',
      params: {
        input: '1',
        cache_creation: 'total * 0.1',
        cache_read: 'total - cache_creation - input',
        output: 'upstream_output',
      },
    },
    transformer_config: [
      { name: 'model_mapper', enabled: true },
      { name: 'streaming_event_fixer', enabled: false },
    ],
    rate_limit_config: { rpm: 200, tpm: 300000 },
    retry_config: { max_retries: 2, backoff_ms: 600 },
    enabled: true,
    created_at: '2026-04-12T16:00:00Z',
  },
  {
    id: 105,
    name: '测试分组（停用）',
    description: '内部联调使用，已停用',
    channel_id: 2,
    channel_name: 'Anthropic 官方',
    channel_type: 'official',
    cache_strategy: { type: 'passthrough' },
    transformer_config: [],
    rate_limit_config: { rpm: 60, tpm: 100000 },
    retry_config: { max_retries: 0, backoff_ms: 0 },
    enabled: false,
    created_at: '2026-05-20T16:00:00Z',
  },
]

// —— 客户 API Key ——
export const apiKeys: ApiKey[] = [
  { id: 1001, key_prefix: 'a1b2c3d4', name: '生产-客户甲', group_id: 101, group_name: '高级套餐-透传', enabled: true, expires_at: '2026-12-31T00:00:00Z', created_at: '2026-04-04T10:00:00Z', last_used_at: '2026-06-10T20:29:50Z', request_count: 184320 },
  { id: 1002, key_prefix: 'e5f6a7b8', name: '生产-客户乙', group_id: 102, group_name: 'Kiro-百分比计费', enabled: true, expires_at: null, created_at: '2026-04-06T10:00:00Z', last_used_at: '2026-06-10T20:29:30Z', request_count: 92750 },
  { id: 1003, key_prefix: 'c9d0e1f2', name: '测试-内部', group_id: 103, group_name: '基础套餐-固定值', enabled: true, expires_at: '2026-08-01T00:00:00Z', created_at: '2026-04-09T10:00:00Z', last_used_at: '2026-06-10T19:50:00Z', request_count: 12880 },
  { id: 1004, key_prefix: 'b3a4c5d6', name: '生产-客户丙', group_id: 104, group_name: 'Bedrock-公式计费', enabled: true, expires_at: null, created_at: '2026-04-14T10:00:00Z', last_used_at: '2026-06-10T20:15:00Z', request_count: 45200 },
  { id: 1005, key_prefix: 'f7e8d9c0', name: '已停用示例', group_id: 101, group_name: '高级套餐-透传', enabled: false, expires_at: '2026-05-01T00:00:00Z', created_at: '2026-04-20T10:00:00Z', last_used_at: '2026-04-30T10:00:00Z', request_count: 320 },
]

export const modelMappings: ModelMapping[] = [
  { id: 1, channel_id: 3, client_model: 'claude-3-5-sonnet-20241022', upstream_model: 'anthropic.claude-3-5-sonnet-20241022-v2:0' },
  { id: 2, channel_id: 3, client_model: 'claude-3-5-haiku-20241022', upstream_model: 'anthropic.claude-3-5-haiku-20241022-v1:0' },
  { id: 3, channel_id: 4, client_model: 'claude-3-5-sonnet-20241022', upstream_model: 'claude-3-5-sonnet-v2@20241022' },
  { id: 4, channel_id: 1, client_model: 'claude-sonnet-4-20250514', upstream_model: 'kiro-claude-sonnet-4' },
]

// —— 请求明细（生成 ~220 条） ——
const ERROR_TYPES = ['upstream_timeout', 'rate_limited', 'invalid_request', 'upstream_5xx', 'context_length_exceeded']
function genTraces(): TraceDetail[] {
  const list: TraceDetail[] = []
  const base = now().getTime()
  for (let i = 0; i < 220; i++) {
    const g = pick(groups.filter((x) => x.enabled))
    const success = rnd() > 0.12
    const streaming = rnd() > 0.3
    const ttft = randInt(180, 2600)
    const duration = ttft + randInt(400, 9000)
    const at = new Date(base - i * randInt(8000, 45000))
    const inTok = randInt(800, 60000)
    const outTok = success ? randInt(60, 3200) : randInt(0, 50)
    const upUsage = { input_tokens: inTok, output_tokens: outTok, cache_creation_tokens: randInt(0, 4000), cache_read_tokens: randInt(0, 40000) }
    const errType = success ? undefined : pick(ERROR_TYPES)
    const status = success ? 200 : pick([429, 500, 502, 504, 400])
    list.push({
      trace_id: `01J${Array.from({ length: 23 }, () => '0123456789ABCDEFGHJKMNPQRSTVWXYZ'[Math.floor(rnd() * 32)]).join('')}`,
      request_at: iso(at),
      group_id: g.id,
      group_name: g.name,
      channel_type: g.channel_type,
      model: pick(MODELS),
      status_code: status,
      is_success: success,
      is_streaming: streaming,
      error_type: errType,
      ttft_ms: ttft,
      duration_ms: duration,
      total_tokens: inTok + outTok,
      api_key_name: pick(apiKeys).name,
      upstream_key_name: pick(upstreamKeys.filter((k) => k.channel_id === g.channel_id))?.name ?? 'n/a',
      error_message: success ? undefined : `${errType}: 上游返回 ${status}，详见复现 body`,
      billed_usage:
        g.cache_strategy.type === 'fixed'
          ? { input_tokens: 1, output_tokens: outTok, cache_creation_tokens: 1000, cache_read_tokens: 5000 }
          : g.cache_strategy.type === 'percentage'
            ? { input_tokens: 1, output_tokens: outTok, cache_creation_tokens: Math.floor(inTok * 0.1), cache_read_tokens: Math.floor(inTok * 0.9) }
            : upUsage,
      upstream_usage: upUsage,
      request_body: {
        model: pick(MODELS),
        max_tokens: 4096,
        stream: streaming,
        system: '你是一个乐于助人的助手。',
        messages: [{ role: 'user', content: '帮我把这段话翻译成英文，并解释其中的难点。' }],
      },
      response_body: success
        ? { id: 'msg_01XYZ', type: 'message', role: 'assistant', model: pick(MODELS), stop_reason: 'end_turn', content: [{ type: 'text', text: 'Here is the translation ...' }], usage: upUsage }
        : { type: 'error', error: { type: errType, message: `上游返回 ${status}` } },
      meta: { client_ip: `203.0.113.${randInt(1, 254)}`, user_agent: 'anthropic-sdk-python/0.39.0', channel_type: g.channel_type },
    })
  }
  return list
}
export const traces = genTraces()

// —— 统计 ——
function windowToBuckets(w: TimeWindow): { count: number; stepMs: number; fmt: (d: Date) => string } {
  switch (w) {
    case '5m':
      return { count: 30, stepMs: 10_000, fmt: (d) => d.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit', second: '2-digit' }) }
    case '1h':
      return { count: 60, stepMs: 60_000, fmt: (d) => d.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' }) }
    case '24h':
      return { count: 48, stepMs: 30 * 60_000, fmt: (d) => d.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' }) }
    case '7d':
      return { count: 56, stepMs: 3 * 3600_000, fmt: (d) => d.toLocaleDateString('zh-CN', { month: '2-digit', day: '2-digit' }) + ' ' + d.toLocaleTimeString('zh-CN', { hour: '2-digit' }) + '时' }
  }
}

export function buildTimeseries(w: TimeWindow, metric: string): TimeseriesPoint[] {
  const { count, stepMs, fmt } = windowToBuckets(w)
  const end = now().getTime()
  const out: TimeseriesPoint[] = []
  const local = mulberry32(99 + metric.length * 7 + w.length)
  for (let i = count - 1; i >= 0; i--) {
    const d = new Date(end - i * stepMs)
    const t = fmt(d)
    const wave = Math.sin((count - i) / 4) * 0.5 + 0.5
    if (metric === 'ttft_p95') {
      out.push({ timestamp: t, value: Math.round(620 + wave * 700 + local() * 260), series: 'P95' })
      out.push({ timestamp: t, value: Math.round(380 + wave * 360 + local() * 120), series: 'P50' })
      out.push({ timestamp: t, value: Math.round(1100 + wave * 1200 + local() * 500), series: 'P99' })
    } else if (metric === 'qps') {
      out.push({ timestamp: t, value: Math.round(120 + wave * 90 + local() * 40) })
    } else if (metric === 'error_rate') {
      out.push({ timestamp: t, value: Number((2 + wave * 6 + local() * 3).toFixed(2)) })
    } else if (metric === 'tokens') {
      out.push({ timestamp: t, value: Math.round(120000 + wave * 90000 + local() * 40000), series: '输入' })
      out.push({ timestamp: t, value: Math.round(20000 + wave * 18000 + local() * 9000), series: '输出' })
      out.push({ timestamp: t, value: Math.round(60000 + wave * 50000 + local() * 20000), series: '缓存读取' })
    }
  }
  return out
}

// 演示用：从前缀确定性派生完整 Key 明文（真实环境为后台 AES 解密后的明文）。
function fullKeyFor(prefix: string): string {
  let h = 2166136261
  const out: string[] = []
  for (let i = 0; i < 48; i++) {
    h = (Math.imul(h ^ (prefix.charCodeAt(i % prefix.length) + i * 7), 16777619)) >>> 0
    out.push('0123456789abcdef'[h & 15])
  }
  return `cg-${prefix}-${out.join('')}`
}

export const api = {
  async overview(): Promise<StatsOverview> {
    await delay()
    return { request_count: 1_284_930, success_rate: 0.973, avg_ttft_ms: 612, p95_ttft_ms: 1180, p99_ttft_ms: 2240, max_ttft_ms: 5120, total_tokens: 4_920_000_000, error_count: 34720 }
  },
  async timeseries(w: TimeWindow, metric: string): Promise<TimeseriesPoint[]> {
    await delay(180)
    return buildTimeseries(w, metric)
  },
  async errors(): Promise<ErrorBucket[]> {
    await delay(180)
    return [
      { error_type: 'upstream_timeout', count: 12840 },
      { error_type: 'rate_limited', count: 9230 },
      { error_type: 'upstream_5xx', count: 6110 },
      { error_type: 'context_length_exceeded', count: 4020 },
      { error_type: 'invalid_request', count: 2520 },
    ]
  },
  async byChannel(): Promise<ByChannelStat[]> {
    await delay(180)
    return [
      { channel_type: 'kiro', request_count: 612000, success_rate: 0.961, avg_ttft_ms: 720 },
      { channel_type: 'official', request_count: 438000, success_rate: 0.989, avg_ttft_ms: 540 },
      { channel_type: 'bedrock', request_count: 156000, success_rate: 0.976, avg_ttft_ms: 610 },
      { channel_type: 'relay', request_count: 72000, success_rate: 0.948, avg_ttft_ms: 880 },
      { channel_type: 'vertex', request_count: 6930, success_rate: 0.97, avg_ttft_ms: 650 },
    ]
  },
  async channels(): Promise<Channel[]> {
    await delay()
    return channels
  },
  async upstreamKeys(channelId?: number): Promise<UpstreamKey[]> {
    await delay(160)
    return channelId ? upstreamKeys.filter((k) => k.channel_id === channelId) : upstreamKeys
  },
  async groups(): Promise<Group[]> {
    await delay()
    return groups
  },
  async group(id: number): Promise<Group | undefined> {
    await delay(160)
    return groups.find((g) => g.id === id)
  },
  async apiKeys(): Promise<ApiKey[]> {
    await delay()
    return apiKeys
  },
  // 重复查看客户 Key 明文（真实环境对应 GET /api/admin/api-keys/:id/reveal，
  // 后台用 CG_ENCRYPTION_KEY 解密 key_encrypted，建议配合操作审计）。
  async revealApiKey(id: number): Promise<string> {
    await delay(200)
    const k = apiKeys.find((x) => x.id === id)
    if (!k) throw new Error('Key 不存在')
    return fullKeyFor(k.key_prefix)
  },
  async modelMappings(): Promise<ModelMapping[]> {
    await delay(160)
    return modelMappings
  },
  async traces(opts: { status?: string; channel_type?: string; group_id?: number; page: number; page_size: number }): Promise<Paged<TraceListItem>> {
    await delay(220)
    let filtered = traces as TraceListItem[]
    if (opts.status && opts.status !== 'all') filtered = filtered.filter((t) => (opts.status === 'success' ? t.is_success : !t.is_success))
    if (opts.channel_type && opts.channel_type !== 'all') filtered = filtered.filter((t) => t.channel_type === opts.channel_type)
    if (opts.group_id) filtered = filtered.filter((t) => t.group_id === opts.group_id)
    const start = (opts.page - 1) * opts.page_size
    return { items: filtered.slice(start, start + opts.page_size), total: filtered.length, page: opts.page, page_size: opts.page_size }
  },
  async trace(traceId: string): Promise<TraceDetail | undefined> {
    await delay(200)
    return traces.find((t) => t.trace_id === traceId) ?? traces[0]
  },
}
