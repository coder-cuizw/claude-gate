# 更新日志

本项目遵循约定式提交（Conventional Commits）。

## [未发布]

### 新增

- **真实存储落地（数据不再丢）**：PostgreSQL（pgx）/ Redis（go-redis）/ ClickHouse（clickhouse-go）/
  S3·MinIO（minio-go）四套真实驱动，各自连真实服务的集成测试通过；`CG_STORE=real` 切换，
  readyz 真探活，首次启动自动建管理员并播种。
- **生产级代理治理（§2.1）**：全局+每通道并发上限与 429 背压、分组 rpm/tpm 限流（Redis/内存双实现）、
  上游重试、异步落库 worker pool（队列满降级 + S3 重试 + 优雅 flush）。
- **计费精确化（§5.3）**：计费 total 改用上游真实输入侧 token，字节估算仅作回退。
- **迁移执行工具 `cmd/migrate`**：按序应用 PG/CH 迁移，`schema_migrations` 记录版本，幂等。
- **docker-compose 升级**：新增 migrate 服务（建表后退出），gateway 切真实模式，一键起全套并自动迁移。
- 修复：明细详情把 S3 取回的非法 JSON body 安全降级为字符串（真测错误请求暴露）。

- **端到端代理主链路打通**（§3 / §5.1）：认证→分组→改写→选 Adapter+取 Key→调上游→
  缓存计费改写 usage→流式/非流回写→明细与 body 落库，curl 实测流式与非流均通过。
- **存储层落地**：`store.ConfigStore` 全量 CRUD、`store.Cache`、`observ.Sink/BodyStore/MetricsReader`
  接口 + **内存实现**（零外部依赖即可端到端运行与自测）；`authstore` 带缓存的 auth.Store 适配。
- **管理 API**（§6 / §5.7）：JWT 登录鉴权、全资源 CRUD、统计（overview/timeseries/errors/by-channel）、
  明细列表/详情、跨分组请求复现（Replay，含 dry_run）、客户 Key reveal。
- **装配根 `app.BuildMemory`**：网关 + 管理 API + 静态前端组装为单一 Handler；`make run` 即起。
- **请求明细增强**：列表按 new-api 风格分列展示 输入/输出/缓存创建/缓存读取；区分流式/非流并展示首字 TTFT。
- **本地 mock 通道**：离线合成标准 Anthropic 响应与 SSE，用于端到端自测与演示。
- 端到端测试（`internal/app`）+ 流式 goroutine 取消测试（§11.5）。

### 变更 / 移除

- **只做中间层**：移除号池管理（冷却状态机、令牌刷新、健康调度），`keypool` 简化为 active Key 轮询；
  `upstream_keys` 移除 `cooldown_until` / `refreshed_at` 列，仅保留 active/disabled。
- **Kiro 改为透传**：不再臆测私有协议，抽出通用 `httpproxy` 适配器供 official/kiro/relay 复用；
  待真实报错后再在 `internal/upstream/kiro/` 内适配。
- **移除 Bedrock / Vertex 通道**（含适配器目录、domain 常量、迁移与前端引用）。
- **默认端口改为 `:8791`**，避免与常见服务（8080 等）冲突。

- **客户 API Key 支持重复查看**：明文经 AES-256-GCM 可逆加密存储（`api_keys.key_encrypted`），
  管理后台可随时「查看密钥」并复制，热路径仍用 `key_hash` 校验不受影响。
  - 后端：`internal/auth/crypto.go`（AES-256-GCM 加解密 + 单测）、新增迁移 `0002_apikey_encrypted`、
    配置项 `CG_ENCRYPTION_KEY`
  - 前端：客户 Key 列表新增「查看密钥」操作，弹窗支持打码/显示切换与复制
  - 文档：`docs/configuration.md` 补充重复查看的安全取舍说明

## [0.1.0] - 2026-06-10

首个里程碑：骨架 + 核心逻辑 + 完整前端控制台。

### 新增

- **后端核心**
  - `domain`：领域模型与统一错误类型 `domain.Error{Code, HTTPStatus, UserMessage, Internal}`
  - `cache`：缓存计费策略引擎，四种策略（透传 / 百分比 / 固定值 / 公式）+ `expr` 公式引擎，单测覆盖率 96.7%
  - `transformer`：改写流水线 + `model_mapper` / `system_prompt_injector` / `tool_call_normalizer` / `streaming_event_fixer`
  - `auth`：API Key 解析/校验 + `GroupResolver`，区分 key 不存在 / 过期 / 禁用 / group 禁用
  - `upstream`：`Adapter` 接口 + registry + `OfficialAdapter`（标准 SSE）+ `KiroAdapter` 骨架 + 轮询 Key 池（cooldown + 自动恢复）
  - `gateway`：HTTP 入口、trace_id 贯穿、健康/就绪检查、结构化错误信封
  - `observ` / `config` / `store`：可观测性、配置加载（yaml + env）、存储接口
  - `migrations`：PostgreSQL 与 ClickHouse 建表脚本
- **前端控制台**（Vite + React + antd 5）
  - Claude 官网风格主题（暖米色 / 赤陶色 + 衬线标题），明亮 / 暗黑 / 跟随系统三态自适应
  - 9 个页面：登录、实时大盘、请求明细、请求详情、分组列表、分组编辑、上游通道、客户 Key、系统设置
  - 缓存策略可视化编辑器（Segmented 切换 + 动态表单 + 公式实时试算）
  - 按通道差异化渲染的通道配置表单；内置 mock 数据可独立运行与截图
- **工程化**
  - docker-compose（PG / ClickHouse / Redis / MinIO + 网关 + 前端）
  - 多阶段 Dockerfile（网关静态编译；前端 nginx 托管）
  - Makefile、`.env.example`、中文文档（配置说明 / 通道接入指南）

### 待办（依赖外部输入或后续里程碑）

- KiroAdapter 私有协议实现（待项目方提供 wire format，§10 要求不臆测）
- 存储层真实驱动接入（PG / ClickHouse / Redis / S3）
- 管理 API CRUD handler 接入存储
- Bedrock / Vertex / Relay 适配器（M5）
- 性能压测达标与默认推荐配置沉淀（M6）
