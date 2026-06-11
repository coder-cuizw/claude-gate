# 通道接入指南

claude-gate **只做中间层**：把每种上游抽象为实现 `upstream.Adapter` 接口的 Adapter，差异收敛在 `internal/upstream/{type}/` 内，**禁止泄漏到主链路**（任务书 §10）。

> 定位说明：本项目不做号池管理。上游 Key 由使用方在外部开好后**直接配置**进来，网关只负责在同通道的多把 active Key 间**轮询转发**，不维护刷新令牌、不做冷却/健康调度。

## 当前支持的通道

| 维度 | Kiro | Official（官方） | Relay（第三方中转） | Custom（本地 mock） |
|------|------|------|------|------|
| 认证方式 | 直接配置好的 key（透传） | API Key（`x-api-key`） | 透传或自定义 Bearer | 无（本地合成） |
| 请求/响应协议 | 当前**透传**（Anthropic 兼容） | 原生 Anthropic Messages | Anthropic 兼容 | 本地合成标准响应 |
| 流式分帧 | 透传标准 SSE | 标准 SSE | 标准 SSE | 合成标准 SSE 序列 |
| usage 来源 | 透传上游返回 | 响应原生返回 | 视上游而定 | 本地合成 |
| 转换复杂度 | 低（透传） | 低 | 低～中 | — |

> **Kiro 现阶段先做透传**：上游可能是你自己的号池（已开好 key），直接填入即可，无需刷新。后续如遇到 Kiro 私有协议导致的真实报错，再在 `internal/upstream/kiro/` 内逐步覆盖私有认证 / 协议 / 流式分帧 / usage 提取（任务书 §10：不臆测 wire format）。
>
> **Bedrock / Vertex 已按需移除**。如未来要接入，只需新增 Adapter 并在 registry 注册，主链路不变。

## 通用 HTTP 透传

official / kiro（当前）/ relay 都复用 `internal/upstream/httpproxy` 通用透传适配器，仅在认证头与默认 base_url 上做配置：

| 通道 | 认证头 | 默认 base_url |
|------|--------|---------------|
| official | `x-api-key` | `https://api.anthropic.com` |
| kiro | `Authorization: Bearer`（可在 config.auth_header 覆盖） | 通道 base_url |
| relay | `Authorization: Bearer` | 通道 base_url |

## 新增通道的步骤

1. 在 `internal/upstream/{type}/` 实现 `upstream.Adapter` 接口（`Name` / `Send` / `SendStream`），或直接复用 `httpproxy.New(...)`
2. 在 `internal/upstream/wire.go` 的 `DefaultRegistry` 注册工厂
3. 在前端 `web/src/pages/Channels.tsx` 的 `ChannelConfigFields` 增加该类型的差异化配置字段
4. 主链路、transformer、cache **无需改动**

## 上游 Key 选择（中间层）

- 在同通道的多把 `status=active` Key 间**轮询**（`internal/upstream/keypool`，含单元测试）
- 仅 `active` / `disabled` 两态，由后台手动启用/禁用
- 调用失败仅被动记录 `last_error`，**不做冷却/降级/刷新**（号池生命周期由使用方在外部维护）
