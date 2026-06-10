import { useQuery } from '@tanstack/react-query'
import { api } from './mock'
import type { TimeWindow } from './types'

// 统一的 TanStack Query 封装。演示模式下数据源是内置 mock；
// 接入真实后端时只需把 queryFn 换成对 /api/admin/* 的 fetch（任务书 §6 / §5.7）。

export const useOverview = (w: TimeWindow) =>
  useQuery({ queryKey: ['overview', w], queryFn: () => api.overview(), refetchInterval: 5000 })

export const useTimeseries = (w: TimeWindow, metric: string) =>
  useQuery({ queryKey: ['timeseries', w, metric], queryFn: () => api.timeseries(w, metric), refetchInterval: 5000 })

export const useErrors = (w: TimeWindow) =>
  useQuery({ queryKey: ['errors', w], queryFn: () => api.errors() })

export const useByChannel = (w: TimeWindow) =>
  useQuery({ queryKey: ['by-channel', w], queryFn: () => api.byChannel() })

export const useChannels = () => useQuery({ queryKey: ['channels'], queryFn: () => api.channels() })

export const useUpstreamKeys = (channelId?: number) =>
  useQuery({ queryKey: ['upstream-keys', channelId], queryFn: () => api.upstreamKeys(channelId) })

export const useGroups = () => useQuery({ queryKey: ['groups'], queryFn: () => api.groups() })

export const useGroup = (id: number) =>
  useQuery({ queryKey: ['group', id], queryFn: () => api.group(id), enabled: Number.isFinite(id) })

export const useApiKeys = () => useQuery({ queryKey: ['api-keys'], queryFn: () => api.apiKeys() })

export const useModelMappings = () =>
  useQuery({ queryKey: ['model-mappings'], queryFn: () => api.modelMappings() })

export const useTraces = (opts: {
  status?: string
  channel_type?: string
  group_id?: number
  page: number
  page_size: number
}) => useQuery({ queryKey: ['traces', opts], queryFn: () => api.traces(opts) })

export const useTrace = (traceId: string) =>
  useQuery({ queryKey: ['trace', traceId], queryFn: () => api.trace(traceId), enabled: !!traceId })
