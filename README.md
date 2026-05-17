# rc_wujunqi

AI Coding 作业：API 通知系统设计与实现。

这是一个内部 API 通知服务 MVP。系统接收业务系统提交的外部 HTTP 通知任务，先持久化到 MySQL，再通过 outbox pattern 可靠发布到 RabbitMQ，由 Go consumer 异步投递到外部供应商 API。

## 核心判断

- 技术栈选择：Golang + MySQL + RabbitMQ。
- Go Web 层采用 MVC 风格分层：Controller、Service、Model、Repository、Worker。
- 投递语义：至少一次投递，不承诺 exactly once。
- 可靠性：MySQL 事务 + outbox + RabbitMQ durable queue + ack + retry queue + dead-letter。
- 幂等：使用 `vendor + idempotency_key` 的 MySQL 唯一索引避免重复创建任务。

## 整体架构

图中的“消息队列”是架构抽象，本项目 MVP 使用 RabbitMQ 实现；“通知投递服务”在架构层面是独立 Worker 服务，负责消费任务并投递外部 API。

![alt text](doc/image/架构图.png)

## 核心时序

### 成功投递

```mermaid
sequenceDiagram
    autonumber
    participant Biz as 业务系统
    participant API as 通知接入服务
    participant Store as 任务存储(MySQL)
    participant Pub as 消息发布组件
    participant MQ as 消息队列(RabbitMQ)
    participant W as 通知投递服务
    participant V as 外部供应商 API

    Biz->>API: POST /notifications
    API->>Store: 同一事务写入任务和 Outbox
    Store-->>API: task_id
    API-->>Biz: 202 Accepted

    Pub->>Store: 读取待发布任务
    Pub->>MQ: 发布持久化消息
    MQ-->>Pub: 发布确认
    Pub->>Store: 标记任务已入队

    MQ-->>W: 投递消息
    W->>Store: 标记 processing
    W->>V: 发送 HTTP 请求
    V-->>W: 2xx
    W->>Store: 标记 succeeded
    W-->>MQ: ack
```

### 失败重试

```mermaid
sequenceDiagram
    autonumber
    participant MQ as 消息队列(RabbitMQ)
    participant W as 通知投递服务
    participant Store as 任务存储(MySQL)
    participant R as 重试策略
    participant RetryQ as Retry Queue
    participant V as 外部供应商 API

    MQ-->>W: 投递消息
    W->>Store: 标记 processing
    W->>V: 发送 HTTP 请求
    V--x W: 超时 / 网络异常 / 5xx / 429
    W->>R: 判断是否可重试

    alt 未超过最大重试次数
        W->>Store: 标记 retrying，retry_count + 1
        W->>RetryQ: 发布延迟重试消息
        W-->>MQ: ack 原消息
        RetryQ-->>MQ: 到期后回到 delivery queue
    else 超过最大重试次数
        W->>Store: 标记 failed
        W-->>MQ: ack 原消息
    end
```

## 快速启动

依赖：

- Docker / Docker Compose
- 本地运行测试需要 Go 1.25+

```bash
docker compose up --build
```

服务地址：

- API: `http://localhost:8080`
- Mock Vendor: `http://localhost:9000`
- RabbitMQ 管理台: `http://localhost:15672`，账号密码 `guest / guest`

健康检查：

```bash
curl http://localhost:8080/healthz
```

## 创建通知任务

```bash
curl -X POST http://localhost:8080/notifications \
  -H 'Content-Type: application/json' \
  -d '{
    "vendor": "crm",
    "target_url": "http://mock-vendor:9000/ok",
    "method": "POST",
    "headers": {
      "Content-Type": "application/json"
    },
    "payload": {
      "contact_id": "c_123",
      "status": "paid"
    },
    "idempotency_key": "payment_evt_123"
  }'
```

返回示例：

```json
{
  "created": true,
  "status": "pending",
  "task_id": "ntf_xxx"
}
```

查询任务：

```bash
curl http://localhost:8080/notifications/{task_id}
```

## 本地测试

```bash
go test ./...
```

## 目录结构

```text
cmd/
  server/        # API 服务入口
  mockvendor/    # 本地 mock 外部供应商
internal/
  controller/    # MVC Controller
  service/       # 业务编排
  model/         # 任务和 outbox 模型
  domain/        # 状态和重试规则
  repository/    # MySQL 读写、事务、幂等
  worker/        # outbox publisher 和 delivery consumer
  infra/         # MySQL、RabbitMQ、HTTP client
doc/
  需要理解.md
  需求分析.md
  架构图.md
  设计文档.md
  接口文档.md
  AI使用说明.md
```

## 文档入口

- [需要理解.md](doc/需要理解.md)
- [需求分析.md](doc/需求分析.md)
- [架构图.md](doc/架构图.md)
- [设计文档.md](doc/设计文档.md)
- [接口文档.md](doc/接口文档.md)
- [AI使用说明.md](doc/AI使用说明.md)
