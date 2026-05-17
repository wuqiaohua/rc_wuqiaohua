# AI 使用说明

本文档记录 AI 在本作业中的辅助作用、未采纳建议，以及由面试者主动做出的关键决策。

## AI 提供的帮助

- 帮助梳理 API 通知系统的常见设计点，如异步投递、失败重试、幂等、死信、状态机和可观测性。
- 帮助整理作业要求、能力考察点和完成计划。
- 帮助优化文档骨架和 Mermaid 架构图、时序图。
- 帮助写出 API 通知系统的 MVP 代码。

## 未采纳的 AI 建议

- 未采纳 AI 默认推荐的 Python + FastAPI + PostgreSQL 技术栈，因为本作业的实现目标更适合用 Go 编写 HTTP 服务和后台 worker。
- 未采纳“数据库任务表 + 定时扫描 worker 就足够”的更轻方案，因为 RabbitMQ 在异步削峰、失败隔离、消费确认和后续扩展上更清晰。
- 未采纳“让业务方直接传 `target_url`、method 和 headers，把系统做成通用 HTTP 转发器”的建议，因为这会放大安全风险，也会让通知系统边界变得模糊。
- 未采纳“一开始就拆成 API 服务、publisher 服务、consumer 服务多个微服务”的建议，因为 MVP 阶段更需要降低部署和调试复杂度；当前先用单进程承载多个职责，但代码边界按可拆分方式组织。
- 未采纳使用 Kafka 作为消息队列的建议，因为当前需求更偏任务队列和可靠消费确认，而不是高吞吐事件流；RabbitMQ 的 ack、retry queue 和 dead-letter 更贴合本场景。
- 未采纳“一开始就把供应商配置做进数据库和管理后台”的建议，因为第一版供应商数量有限，用 TOML 配置文件可以先做到配置和代码分开上线，同时避免过早建设配置平台。
- 未采纳“请求线程内同步调用供应商 API 并失败重试”的建议，因为业务系统不需要同步等待供应商结果，且同步重试会拖慢调用方并降低故障隔离能力。
- 未采纳过早引入规则引擎、管理后台、复杂供应商配置平台、服务网格或 exactly once 的方案，因为这些会让 MVP 偏离可靠投递主链路。

## 面试者主动做出的关键决策

- 主动决定 MVP 不允许调用方在请求体中传入 `target_url`，而是通过 TOML 配置文件维护 `vendor` 对应的目标地址、HTTP 方法和默认 Header，避免系统变成任意 HTTP 转发器，并让供应商配置后续可以和代码分开上线。
- 主动决定先为每个调用方分配 `app_id` 和 `app_secret`，通过 Header 携带时间戳、随机串和 HMAC-SHA256 签名完成 API 认证和基础权限控制。
- 主动采用 outbox pattern，而不是在业务事务里直接调用 MQ 或第三方服务，避免数据库写成功但消息发送失败的问题。
- 选择 MySQL，是因为 MySQL 事务适合保证 notification task 和 outbox event 原子写入，唯一索引适合处理 `app_id + vendor + idempotency_key` 维度的幂等。
- 主动决定 MVP 先采用单体分层架构，而不是一开始拆微服务，降低部署、调试和事务一致性复杂度。
- 主动要求 Golang 采用 MVC 风格框架和分层组织，避免把 HTTP handler、业务逻辑、存储逻辑和 worker 混在一起。
- 主动把供应商差异先收敛到 TOML 形式的 `vendor` 配置，并预留后续 `NotifierAdapter` 扩展方向，让需要签名、字段映射或特殊成功判定的复杂渠道可以独立演进。
- 主动选择 Golang + MySQL + RabbitMQ 作为 MVP 技术栈。
- 选择 RabbitMQ，是因为它适合业务异步和任务队列，能提供 ack、retry queue、dead-letter、削峰和失败隔离能力。
- 主动说明 MVP 不选择 Kafka，因为当前更需要任务队列、ack、retry、DLQ，而不是高吞吐事件流。
- 选择 Golang，是因为 Go 适合异步任务、worker、网络 IO，goroutine 能低成本启动 outbox publisher 和 delivery consumer。

## 关键取舍

- MVP 采用消息队列，因为通知系统天然是异步任务系统，RabbitMQ 是核心路径，不是外围复杂度。
- 系统采用至少一次投递，不承诺 exactly once。
- 使用 outbox pattern 解决任务落库和 MQ 发布之间的双写一致性问题。
- 重复通知通过 `app_id + vendor + idempotency_key`、MySQL 唯一索引和外部供应商幂等能力共同降低影响。
- `target_url` 不放在请求体中，而是放到 TOML 配置文件中；MVP 先不做供应商管理后台，但保留后续演进为数据库配置或管理后台的空间。
- API 认证先采用 `app_id + app_secret + HMAC 签名`，在不引入完整账号系统的前提下，解决调用方身份识别、请求防篡改、防重放和 vendor 授权边界问题。
- 供应商配置平台、管理后台、多租户权限、复杂工作流编排放到后续演进。
