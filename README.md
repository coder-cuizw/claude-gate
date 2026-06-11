<div align="center">

# claude-gate

**面向 Claude 系列模型的可编程中转网关（中间层）**

统一接入 Kiro / 官方 / 第三方中转等上游，对客户暴露同一套 Anthropic Messages 协议。
只做中间层：上游 Key 直接配置、多把轮询，不做号池管理。

</div>

---

## 项目简介

`claude-gate` 把"通道（channel）"抽象为可插拔的 **Adapter**，解决四个核心痛点：

1. **多通道统一接入与隔离** —— 异构上游统一成同一套对外协议，按分组隔离与路由
2. **通道差异化适配** —— 各通道差异由 Adapter 层屏蔽；Kiro 当前先做透传，后续按真实报错再适配私有协议
3. **可定制的缓存计费** —— 按分组配置 `cache_creation` / `cache_read` / `input` 的计算方式（透传 / 百分比 / 固定值 / 公式）
4. **错误请求的可观测与可复现** —— 任何失败请求 100% 还原 + 一键重放

> 本仓库是按《claude-gate 开发任务书 v1.0》推进的实现。当前完成度见下方 **实现状态**。

## 界面预览

控制台采用 **Claude 官网风格**（暖米色 / 赤陶色 + 衬线标题），支持**明亮 / 暗黑 / 跟随系统**三态自适应。

| 实时大盘（明亮） | 实时大盘（暗黑） |
|:---:|:---:|
| ![](docs/screenshots/dashboard-light.png) | ![](docs/screenshots/dashboard-dark.png) |

| 缓存策略编辑器（公式 + 实时试算） | 上游通道与 Key 池 |
|:---:|:---:|
| ![](docs/screenshots/group-edit-dark.png) | ![](docs/screenshots/channels-light.png) |

更多页面截图见 [`docs/screenshots/`](docs/screenshots/)（登录、请求明细、请求详情、分组、客户 Key、系统设置，均含明暗两套）。

## 技术栈

**后端**：Go 1.22 · 原生 net/http（代理路径）· `expr-lang/expr`（公式引擎）· `log/slog`（结构化日志）· PostgreSQL / ClickHouse / Redis / S3(MinIO)

**前端**：Vite + React 18 + TypeScript · **Ant Design 5**（深度定制 Claude 主题）· `@ant-design/charts` · React Router 6 · TanStack Query · Zustand

## 快速开始

### 前端控制台（演示模式，无需后端）

控制台内置 mock 数据，可独立运行与截图：

```bash
cd web
pnpm install
pnpm dev          # 开发模式 http://localhost:5173
# 或
pnpm build && pnpm preview   # 生产构建预览 http://localhost:4173
```

任意账号密码即可登录。生成全部页面明暗截图：

```bash
cd web && pnpm preview &
PLAYWRIGHT_BROWSERS_PATH=/opt/pw-browsers node scripts/screenshots.mjs
```

### 后端网关

```bash
make build        # 编译 bin/gateway 与 bin/migrate
make test         # 运行全部单元测试
make run          # 启动网关，默认 :8791
curl localhost:8791/healthz
```

### 全套依赖（docker-compose）

```bash
cd deploy && docker compose up -d   # PG / ClickHouse / Redis / MinIO + 网关 + 前端
```

## 项目结构

```
claude-gate/
├── cmd/                  网关入口 / 迁移工具
├── internal/
│   ├── domain/           领域模型与统一错误
│   ├── cache/            ⭐ 缓存计费策略引擎（四种策略 + 公式引擎）
│   ├── transformer/      请求改写流水线（model_mapper 等）
│   ├── auth/             API Key 解析 + 分组解析
│   ├── upstream/         上游适配层（official / kiro / keypool / registry）
│   ├── gateway/          HTTP 入口与代理逻辑
│   ├── observ/           trace_id 贯穿 / 明细落库 / body 落盘接口
│   ├── store/            PG / ClickHouse / Redis / S3 存储接口
│   └── config/           配置加载（yaml + env）
├── migrations/           PostgreSQL 与 ClickHouse 建表脚本
├── web/                  Vite + React + antd 控制台
├── deploy/               docker-compose / Dockerfile / nginx
└── docs/                 文档与截图
```

## 实现状态

按任务书里程碑划分，**诚实标注**当前完成度：

| 模块 | 状态 | 说明 |
|------|------|------|
| 端到端代理主链路（§3 / §5.1） | ✅ 完成 | 认证→分组→改写→选 Adapter+取 Key→调上游→计费改写→流式/非流回写→落库，curl 实测通过 |
| 缓存计费策略引擎（§5.3 ⭐） | ✅ 完成 | 四种策略 + 公式引擎，单测覆盖率 **96.7%** |
| Transformer 流水线（§5.4） | ✅ 完成 | 流水线 + 四个改写器 + 单测 |
| API Key 解析 / 分组解析（§5.2） | ✅ 完成 | 区分四类失败原因 + 热路径缓存 + 单测 |
| 上游 Key 选择（§5.5，中间层） | ✅ 完成 | active Key 轮询转发（**不做号池管理**）+ 单测 |
| OfficialAdapter / KiroAdapter（透传）/ RelayAdapter | ✅ 完成 | 复用通用 httpproxy；Kiro 当前透传 |
| 本地 mock 通道 | ✅ 完成 | 离线合成响应，端到端自测/演示用 |
| 管理 API（§6 / §5.7） | ✅ 完成 | JWT + 全资源 CRUD + 统计 + 明细/详情 + 复现 + Key reveal |
| 存储真实驱动（PG/Redis/CH/S3） | ✅ 完成 | pgx / go-redis / clickhouse-go / minio-go 落地，**各驱动真实服务集成测试通过** |
| 真实模式装配 + 切换（§8） | ✅ 完成 | `CG_STORE=real` 连真库；readyz 真探活；首次启动自动建管理员并播种 |
| 并发治理 / 限流 / 重试（§2.1 / §5.2） | ✅ 完成 | 全局+通道并发上限 429 背压、rpm/tpm 限流、上游重试，**均有单测** |
| 异步落库 worker pool（§2.1 / §5.6） | ✅ 完成 | body+明细投递队列、队列满降级、S3 重试、优雅关闭 flush |
| 计费 token 精确口径（§5.3） | ✅ 完成 | total 优先用上游真实输入侧 token，字节估算仅回退 |
| 数据库迁移工具（§8） | ✅ 完成 | `cmd/migrate` 执行 PG/CH 迁移，版本记录幂等 |
| 前端控制台（§7，9 个页面） | ✅ 完成 | Claude 风格 + 明暗自适应 + mock 数据可独立运行 |
| 前端对接真实后端 | 🚧 待接 | 当前前端走内置 mock；后端 API 已就绪，替换 `web/src/api/queries.ts` 数据源即可 |
| **KiroAdapter 私有协议** | 🚧 透传中 | 当前先透传；按真实报错再适配（任务书 §10 不臆测 wire format） |

> 说明：`CG_STORE=real` 时连真实 PG/Redis/ClickHouse/S3（`docker compose up` 一键起全套并自动迁移）；
> 默认 `memory` 模式零依赖即可 `make run` 端到端自测。两模式实现同一组接口，主链路与管理 API 无差异。
>
> **Bedrock / Vertex 已按需移除**；**号池管理、令牌刷新、冷却调度**按"只做中间层"定位移除。

## 测试 / 自测

```bash
go test ./... -cover                       # 单元 + 端到端 + 治理测试
# 真实存储集成测试（需 PG/Redis/CH/MinIO；本地起好后设环境变量）
CG_TEST_PG_DSN=... CG_TEST_REDIS_ADDR=... CG_TEST_CH_ADDR=... CG_TEST_S3_ENDPOINT=... go test ./internal/store/...
make run                                    # 内存模式启动（默认 :8791）
docker compose -f deploy/docker-compose.yml up   # 真实模式一键起全套
```

- 核心逻辑（cache / transformer / auth / keypool）单测，缓存引擎覆盖率 **96.7%**
- **存储集成测试**：PG（JSONB 往返/前缀查找/可逆密文）、Redis（TTL）、ClickHouse（批写+统计）、S3（body 往返）均连真实服务通过
- **治理单测**：并发背压 429 / rpm 限流 / 重试成功 / 计费基于上游 token
- `internal/app` 端到端测试 + 流式 goroutine 取消测试（§11.5，无泄漏）
- **real 模式真测**：配置落 PG、明细落 CH、body 落 S3、热路径走 Redis；20 并发异步落库不崩；重启数据保留、seed 幂等；优雅关闭 flush

## 文档

- [配置说明](docs/configuration.md) —— 全部可调参数与性能调优基线
- [通道接入指南](docs/channels.md) —— 各通道差异矩阵与接入要点

## 许可

内部项目，未开源授权。
