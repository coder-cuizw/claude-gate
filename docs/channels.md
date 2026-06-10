# 通道接入指南

claude-gate 把每种上游抽象为实现 `upstream.Adapter` 接口的 Adapter，差异收敛在 `internal/upstream/{type}/` 内，**禁止泄漏到主链路**（任务书 §10）。

## 通道特性矩阵

| 维度 | Kiro（逆向 / 特殊） | Official（官方） | Bedrock | Vertex AI | Relay（第三方） |
|------|------|------|------|------|------|
| 认证方式 | SSO / 刷新令牌，需后台定期刷新 | API Key（Bearer） | AWS SigV4（AK/SK 或 Role） | GCP 服务账号 / OAuth | 透传或自定义 Bearer |
| 请求/响应协议 | **私有协议，需双向转换** | 原生 Anthropic Messages | Bedrock Runtime（Anthropic schema） | Vertex（Anthropic schema） | Anthropic 兼容 |
| 流式分帧 | **私有事件帧，需重封装为 SSE** | 标准 SSE | event stream → SSE | Vertex 流式 → SSE | 标准 SSE |
| usage 来源 | 可能缺失，需提取 / 估算 | 响应原生返回 | 响应原生返回 | 响应原生返回 | 视上游而定 |
| 凭证维护 | **需后台刷新令牌** | 无 | 凭证 / Role 轮换 | 凭证轮换 | 无 |
| 转换复杂度 | **高** | 低 | 中 | 中 | 低～中 |

一句话差异化策略：**Kiro 是唯一需要在 Adapter 内重写协议与流式、并维护刷新令牌的通道；其余通道尽量复用官方 SDK / 标准 HTTP，只做薄封装**。云厂商通道（Bedrock / Vertex）优先用官方 SDK，不要手搓 SigV4 / OAuth 签名。

## 新增通道的步骤

1. 在 `internal/upstream/{type}/` 实现 `upstream.Adapter` 接口（`Name` / `Send` / `SendStream`）
2. 在 `internal/upstream/wire.go` 的 `DefaultRegistry` 注册工厂
3. 在前端 `web/src/pages/Channels.tsx` 的 `ChannelConfigFields` 增加该类型的差异化配置字段
4. 主链路、transformer、cache **无需改动**

## ⚠️ Kiro 通道（私有逆向）

`KiroAdapter` 当前为**结构骨架**，需在内部完成（不外泄到主链路）：

1. **认证与令牌刷新** —— 维护 Kiro 私有凭证，按需刷新；过期/失效转 Key 池 cooldown
2. **请求转换** —— Anthropic Messages → Kiro 私有请求格式
3. **响应/流式转换** —— Kiro 私有响应帧 → 标准 Anthropic SSE 事件序列（`message_start` / `content_block_delta` / `message_delta` / `message_stop` …）
4. **usage 提取** —— 从 Kiro 响应提取/估算 token 数，回填统一 `Usage` 结构供策略引擎使用

> 任务书 §10 明确要求：**Kiro 的具体协议细节（认证流程、端点、私有 schema、流式分帧格式）由项目方单独提供。遇到 Kiro 相关格式问题先停下来问，不要自行臆测 wire format**。
>
> 因此本仓库的 `internal/upstream/kiro/kiro.go` 在缺少协议时统一返回明确错误（`Kiro 私有协议细节待项目方提供`），预留了 `RefreshFunc` 等可注入扩展点，待协议到位后补全。

## Key 池调度策略（MVP）

- 轮询选取 `status=active` 的 Key
- 收到 429/5xx → `status=cooldown`，`cooldown_until = now + 5min`
- 后台周期检查 cooldown 到期的 Key 自动恢复
- 对刷新型凭证（Kiro）：后台在过期前主动刷新，刷新失败才进入 cooldown

实现见 `internal/upstream/keypool`（含单元测试）。
