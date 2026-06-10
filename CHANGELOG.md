# 更新日志

本项目遵循约定式提交（Conventional Commits）。

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
