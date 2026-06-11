import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { http } from './client'
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

// 统一对接真实后端 /api/admin/*。派生字段（key_count、channel_name、group_name 等）
// 后端未直接返回，前端在此 join 关联资源补齐，页面无需改动。

const SPAN: Record<TimeWindow, number> = { '5m': 3e5, '1h': 36e5, '24h': 864e5, '7d': 6048e5 }
const GRAN: Record<TimeWindow, string> = { '5m': '1m', '1h': '1m', '24h': '1h', '7d': '1h' }
function windowQuery(w: TimeWindow): string {
  const now = Date.now()
  return `from=${new Date(now - SPAN[w]).toISOString()}&to=${new Date(now).toISOString()}`
}

// —— 统计 ——
export const useOverview = (w: TimeWindow) =>
  useQuery({ queryKey: ['overview', w], queryFn: () => http.get<StatsOverview>(`/api/admin/stats/overview?${windowQuery(w)}`), refetchInterval: 5000 })

export const useTimeseries = (w: TimeWindow, metric: string) =>
  useQuery({
    queryKey: ['timeseries', w, metric],
    queryFn: () => http.get<TimeseriesPoint[]>(`/api/admin/stats/timeseries?metric=${metric}&granularity=${GRAN[w]}&${windowQuery(w)}`),
    refetchInterval: 5000,
  })

export const useErrors = (w: TimeWindow) =>
  useQuery({ queryKey: ['errors', w], queryFn: () => http.get<ErrorBucket[]>(`/api/admin/stats/errors?${windowQuery(w)}`) })

export const useByChannel = (w: TimeWindow) =>
  useQuery({ queryKey: ['by-channel', w], queryFn: () => http.get<ByChannelStat[]>(`/api/admin/stats/by-channel?${windowQuery(w)}`) })

// —— 通道（补 key_count）——
export const useChannels = () =>
  useQuery({
    queryKey: ['channels'],
    queryFn: async (): Promise<Channel[]> => {
      const [chsRes, keysRes] = await Promise.all([http.get<Channel[]>('/api/admin/channels'), http.get<UpstreamKey[]>('/api/admin/upstream-keys')])
      const chs = chsRes ?? []
      const keys = keysRes ?? []
      return chs.map((c) => ({ ...c, config: c.config ?? {}, key_count: keys.filter((k) => k.channel_id === c.id).length }))
    },
  })

export const useUpstreamKeys = (channelId?: number) =>
  useQuery({
    queryKey: ['upstream-keys', channelId],
    queryFn: () => http.get<UpstreamKey[]>(`/api/admin/upstream-keys${channelId ? `?channel_id=${channelId}` : ''}`),
  })

// —— 分组（补 channel_name / channel_type）——
async function enrichGroups(gs: Group[] | null | undefined): Promise<Group[]> {
  const list = gs ?? []
  const chs = (await http.get<Channel[]>('/api/admin/channels')) ?? []
  const byId = new Map(chs.map((c) => [c.id, c]))
  return list.map((g) => ({ ...g, channel_name: byId.get(g.channel_id)?.name ?? '—', channel_type: byId.get(g.channel_id)?.type ?? 'custom' }))
}

export const useGroups = () => useQuery({ queryKey: ['groups'], queryFn: async () => enrichGroups(await http.get<Group[]>('/api/admin/groups')) })

export const useGroup = (id: number) =>
  useQuery({
    queryKey: ['group', id],
    queryFn: async () => (await enrichGroups([await http.get<Group>(`/api/admin/groups/${id}`)]))[0],
    enabled: Number.isFinite(id),
  })

// —— 客户 Key（补 group_name）——
export const useApiKeys = () =>
  useQuery({
    queryKey: ['api-keys'],
    queryFn: async (): Promise<ApiKey[]> => {
      const [ksRes, gsRes] = await Promise.all([http.get<ApiKey[]>('/api/admin/api-keys'), http.get<Group[]>('/api/admin/groups')])
      const ks = ksRes ?? []
      const gs = gsRes ?? []
      const byId = new Map(gs.map((g) => [g.id, g]))
      return ks.map((k) => ({ ...k, group_name: byId.get(k.group_id)?.name ?? '—', request_count: k.request_count ?? 0, last_used_at: k.last_used_at ?? null }))
    },
  })

export const useModelMappings = () => useQuery({ queryKey: ['model-mappings'], queryFn: () => http.get<ModelMapping[]>('/api/admin/model-mappings') })

// —— 明细（补 group_name / total_tokens）——
function usageTotal(u?: { input_tokens: number; output_tokens: number; cache_creation_tokens: number; cache_read_tokens: number }) {
  return u ? u.input_tokens + u.output_tokens + u.cache_creation_tokens + u.cache_read_tokens : 0
}

export const useTraces = (opts: { status?: string; channel_type?: string; group_id?: number; page: number; page_size: number }) =>
  useQuery({
    queryKey: ['traces', opts],
    queryFn: async (): Promise<Paged<TraceListItem>> => {
      const q = new URLSearchParams({ status: opts.status ?? 'all', page: String(opts.page), page_size: String(opts.page_size) })
      if (opts.channel_type && opts.channel_type !== 'all') q.set('channel_type', opts.channel_type)
      if (opts.group_id) q.set('group_id', String(opts.group_id))
      const [page, gsRes] = await Promise.all([
        http.get<Paged<TraceListItem>>(`/api/admin/traces?${q.toString()}`),
        http.get<Group[]>('/api/admin/groups'),
      ])
      const gs = gsRes ?? []
      const byId = new Map(gs.map((g) => [g.id, g]))
      const items = (page.items ?? []).map((r) => ({ ...r, group_name: byId.get(r.group_id)?.name ?? '—', total_tokens: usageTotal(r.billed_usage) }))
      return { ...page, items }
    },
  })

export const useTrace = (traceId: string) =>
  useQuery({
    queryKey: ['trace', traceId],
    queryFn: async (): Promise<TraceDetail> => {
      const [d, gsRes] = await Promise.all([
        http.get<{ record: TraceListItem & { api_key_id: number; upstream_key_id: number; error_message?: string; billed_usage: TraceDetail['billed_usage']; upstream_usage: TraceDetail['upstream_usage'] }; request_body?: unknown; response_body?: unknown }>(
          `/api/admin/traces/${traceId}`,
        ),
        http.get<Group[]>('/api/admin/groups'),
      ])
      const gs = gsRes ?? []
      const r = d.record
      const g = gs.find((x) => x.id === r.group_id)
      return {
        ...r,
        group_name: g?.name ?? '—',
        total_tokens: usageTotal(r.billed_usage),
        api_key_name: `#${r.api_key_id}`,
        upstream_key_name: r.upstream_key_id ? `#${r.upstream_key_id}` : 'n/a',
        request_body: d.request_body ?? {},
        response_body: d.response_body ?? {},
        meta: { channel_type: r.channel_type, status_code: r.status_code, upstream_key_id: r.upstream_key_id, trace_id: r.trace_id },
      } as TraceDetail
    },
    enabled: !!traceId,
  })

// —— 命令式调用 ——
export const revealApiKey = (id: number) => http.get<{ plaintext: string }>(`/api/admin/api-keys/${id}/reveal`).then((r) => r.plaintext)

export const replayTrace = (id: string, body: { target_group_id?: number; dry_run?: boolean; override_model?: string }) =>
  http.post(`/api/admin/traces/${id}/replay`, body)

// —— Mutations（写操作；成功后失效相关缓存）——
function useInvalidate(keys: string[]) {
  const qc = useQueryClient()
  return () => keys.forEach((k) => qc.invalidateQueries({ queryKey: [k] }))
}

export const useSaveChannel = () => {
  const inv = useInvalidate(['channels'])
  return useMutation({
    mutationFn: (c: Partial<Channel>) => (c.id ? http.put(`/api/admin/channels/${c.id}`, c) : http.post('/api/admin/channels', c)),
    onSuccess: inv,
  })
}
export const useToggleChannel = () => {
  const inv = useInvalidate(['channels'])
  return useMutation({ mutationFn: (id: number) => http.post(`/api/admin/channels/${id}/toggle`), onSuccess: inv })
}
export const useDeleteChannel = () => {
  const inv = useInvalidate(['channels'])
  return useMutation({ mutationFn: (id: number) => http.del(`/api/admin/channels/${id}`), onSuccess: inv })
}

export const useCreateUpstreamKey = () => {
  const inv = useInvalidate(['upstream-keys', 'channels'])
  return useMutation({ mutationFn: (b: { channel_id: number; name: string; credential: string }) => http.post('/api/admin/upstream-keys', b), onSuccess: inv })
}
export const useUpdateUpstreamKey = () => {
  const inv = useInvalidate(['upstream-keys'])
  return useMutation({ mutationFn: (b: { id: number; name?: string; credential?: string; status?: string }) => http.put(`/api/admin/upstream-keys/${b.id}`, b), onSuccess: inv })
}
export const useDeleteUpstreamKey = () => {
  const inv = useInvalidate(['upstream-keys', 'channels'])
  return useMutation({ mutationFn: (id: number) => http.del(`/api/admin/upstream-keys/${id}`), onSuccess: inv })
}

export const useSaveGroup = () => {
  const inv = useInvalidate(['groups', 'group'])
  return useMutation({
    mutationFn: (g: Partial<Group>) => (g.id ? http.put(`/api/admin/groups/${g.id}`, g) : http.post('/api/admin/groups', g)),
    onSuccess: inv,
  })
}
export const useDeleteGroup = () => {
  const inv = useInvalidate(['groups'])
  return useMutation({ mutationFn: (id: number) => http.del(`/api/admin/groups/${id}`), onSuccess: inv })
}

export const useCreateApiKey = () => {
  const inv = useInvalidate(['api-keys'])
  return useMutation({
    mutationFn: (b: { name: string; group_id: number; expires_at?: string | null }) =>
      http.post<{ api_key: ApiKey; plaintext: string }>('/api/admin/api-keys', b),
    onSuccess: inv,
  })
}
export const useUpdateApiKey = () => {
  const inv = useInvalidate(['api-keys'])
  return useMutation({ mutationFn: (b: { id: number; name?: string; group_id?: number; enabled?: boolean }) => http.put(`/api/admin/api-keys/${b.id}`, b), onSuccess: inv })
}
export const useDeleteApiKey = () => {
  const inv = useInvalidate(['api-keys'])
  return useMutation({ mutationFn: (id: number) => http.del(`/api/admin/api-keys/${id}`), onSuccess: inv })
}

export const useCreateModelMapping = () => {
  const inv = useInvalidate(['model-mappings'])
  return useMutation({ mutationFn: (b: { channel_id: number; client_model: string; upstream_model: string }) => http.post('/api/admin/model-mappings', b), onSuccess: inv })
}
export const useDeleteModelMapping = () => {
  const inv = useInvalidate(['model-mappings'])
  return useMutation({ mutationFn: (id: number) => http.del(`/api/admin/model-mappings/${id}`), onSuccess: inv })
}

// —— 管理员账号 ——
export const useChangePassword = () =>
  useMutation({ mutationFn: (b: { old_password: string; new_password: string }) => http.post('/api/admin/me/password', b) })
