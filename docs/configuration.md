# 配置说明

claude-gate 的配置优先级：**默认值 < YAML 文件 < 环境变量（前缀 `CG_`）**。
所有可调参数集中在 `internal/config`，禁止散落硬编码（任务书 §10）。

## 配置项一览

| 配置 | YAML 路径 | 环境变量 | 默认值 | 说明 |
|------|-----------|----------|--------|------|
| 监听地址 | `server.addr` | `CG_SERVER_ADDR` | `:8791` | HTTP 监听 |
| 全链路超时 | `server.request_timeout_seconds` | — | `600` | 单请求 context 超时 |
| PG DSN | `postgres.dsn` | `CG_POSTGRES_DSN` | 见 `.env.example` | 配置数据库 |
| ClickHouse DSN | `clickhouse.dsn` | `CG_CLICKHOUSE_DSN` | — | 明细库 |
| Redis 地址 | `redis.addr` | `CG_REDIS_ADDR` | `localhost:6379` | 热路径缓存 |
| S3 端点 | `s3.endpoint` | `CG_S3_ENDPOINT` | `localhost:9100` | body 落盘 |
| S3 桶 | `s3.bucket` | `CG_S3_BUCKET` | `claude-gate` | |
| JWT 密钥 | `auth.jwt_secret` | `CG_JWT_SECRET` | — | 管理后台签发 |
| 加密口令 | `auth.encryption_key` | `CG_ENCRYPTION_KEY` | — | 可逆存储客户 Key 明文，支持后台重复查看（见下） |
| API Key 缓存秒数 | `auth.apikey_cache_ttl_seconds` | — | `60` | §5.2 |

## 客户 API Key 的"重复查看"与安全取舍

默认设计只存 `key_hash`（不可逆），明文创建时展示一次。但中转站运营常需运营者
**事后重新查看 / 分发**客户 Key，因此 claude-gate 额外存一份 **AES-256-GCM** 加密的明文
（`api_keys.key_encrypted`），由管理后台按权限解密展示，支持随时重复查看：

- 热路径校验仍走 `key_hash`，不解密、不影响代理性能；
- `key_encrypted` 仅在管理后台"查看密钥"时解密，建议配合操作审计；
- 加密口令经 `CG_ENCRYPTION_KEY` 配置，**生产务必设置为高熵随机串并妥善保管**；口令丢失将无法再解出历史明文。

这是面向中转站产品的有意取舍：相比纯 hash 存储略微降低了密钥保密强度，换取运营可重复查看的能力。实现见 `internal/auth/crypto.go`。
| 日志级别 | `log.level` | `CG_LOG_LEVEL` | `info` | debug/info/warn/error |

## 性能与并发（任务书 §2.1）

目标：16 核 / 32 GB 单机上持续 ≥ 10,000 rpm（约 167 RPS 稳态），P99 代理开销 < 30ms。

| 配置 | YAML 路径 | 环境变量 | 默认值 | 调优建议 |
|------|-----------|----------|--------|----------|
| 全局并发上限 | `concurrency.global_max_in_flight` | `CG_GLOBAL_MAX_IN_FLIGHT` | `6000` | 超出快速返回 429，不无限堆积 |
| 每通道并发上限 | `concurrency.per_channel_in_flight` | — | `2000` | 防单通道打满 |
| 落盘 Worker 数 | `concurrency.worker_pool_size` | — | `16` | 按核数设置 |
| 上游连接池上限 | `concurrency.upstream_pool_size` | — | `256` | keep-alive 复用 |

> claude-gate 是 I/O 密集型代理，绝大多数 goroutine 阻塞在等待上游响应。**设计瓶颈是并发连接数与内存，而非 CPU**。按峰值 ~5,000 并发流式连接估算，单连接常驻控制在数十 KB，连接本身约占 1～2 GB。

## 落盘采样与磁盘规划（任务书 §5.6 / §2.1）

| 配置 | YAML 路径 | 默认值 | 说明 |
|------|-----------|--------|------|
| 成功采样率 | `sampling.success_rate` | `0.01` | 成功请求 1% 落盘 |
| 错误全留存 | `sampling.error_always` | `true` | 错误请求 100% 落盘 |
| S3 写入重试 | `sampling.s3_write_retry` | `3` | 失败重试后丢弃并打 metric |

**磁盘容量估算公式**（500 GB 基线）：

```
可容纳天数 ≈ 磁盘容量 / (日请求数 × (错误率 + 成功采样率) × 平均 body 大小 × 2)
```

例：10k rpm ≈ 1440 万/天，错误率 3% + 采样 1% = 4%，平均 body 20 KB（请求+响应各算一份）：

```
1440万 × 0.04 × 20KB × 2 ≈ 23 GB/天 → 500 GB 约可存 21 天
```

ClickHouse 明细已设 **90 天 TTL**，物化视图聚合后单价很低。务必开启**磁盘水位监控 + 自动清理/告警**，避免 body 落盘把磁盘写满。
