# catllm
A llm gateway write by golang. Inspire litellm and aigateway.

## Quick Start (Phase 1 MVP)

The Phase 1 MVP provides a minimal viable data plane with:
- Simple file-based configuration
- OpenAI provider support
- `/v1/chat/completions` endpoint (streaming and non-streaming)
- `/responses` endpoint (streaming and non-streaming)
- Basic retry logic

### Installation

```bash
go build -o catllm .
```

### Configuration

Create a `config.yaml` file (see `config.example.yaml`):

```yaml
server:
  port: 8080

providers:
  - name: openai
    base_url: https://api.openai.com
    api_key: ${OPENAI_API_KEY}
    timeout: 30

routes:
  - model: gpt-4o
    provider: openai
  - model: gpt-4o-mini
    provider: openai
```

Set your OpenAI API key:
```bash
export OPENAI_API_KEY=sk-...
```

### Running

```bash
./catllm -config config.yaml
```

### Usage

Send requests to the gateway:

```bash
# Non-streaming request to /v1/chat/completions
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4o-mini",
    "messages": [{"role": "user", "content": "Hello!"}]
  }'

# Streaming request to /v1/chat/completions
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4o-mini",
    "messages": [{"role": "user", "content": "Hello!"}],
    "stream": true
  }'

# Request to /responses endpoint
curl http://localhost:8080/responses \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4o-mini",
    "messages": [{"role": "user", "content": "Hello!"}]
  }'
```

Health check:
```bash
curl http://localhost:8080/health
```

## design

这个是一个 golang llm gateway 开源项目，你现在是llm gateway 领域的专家，我们共同设计项目需求和架构设计。该项目代码质量和性能为第一优先级。

架构分成控制面和数据面，优先交付数据面并保证与主流 LLM API 兼容。

### 目标与范围
- 统一对外 LLM API，兼容 OpenAI 风格接口，便于客户端无感迁移。
- 屏蔽不同厂商差异，支持多模型、多供应商路由与弹性策略。
- 高性能、低延迟、可观测、可扩展、易运维。

### 总体架构
- 数据面：无状态请求处理服务，负责统一 API、路由、策略、编解码与转发。
- 控制面：配置与策略管理、路由发布、运行状态与可观测性入口。

### Repository layout
- `docs/architecture.md`：数据面模块设计与工作流说明。
- `internal/config`：配置加载模块（设计文档占位）。
- `internal/codec`：协议转换模块（设计文档占位）。
- `internal/forwarder`：转发模块（设计文档占位）。

## 数据面
第一阶段目标是实现统一 LLM API 数据面，开放 API 参考：
https://aigateway.envoyproxy.io/docs/capabilities/llm-integrations/supported-endpoints

### 请求处理流程
1. 认证与租户识别：API Key/JWT/mTLS。
2. 参数校验与规范化：统一模型/参数字段，校验额度与范围。
3. 路由决策：按模型、租户、区域、优先级或标签选择供应商。
4. 策略执行：限流、配额、超时、重试、熔断、fallback。
5. 转换与转发：请求/响应映射到具体供应商协议。
6. 结果回写：统一响应格式，支持流式响应。
7. 观测与审计：指标、日志、追踪、账单数据。

### 数据面输入与配置来源
数据面以 K8s 资源作为主要配置输入，监听并解析：
- Gateway API 资源（如 `Gateway`、`HTTPRoute`）。
- AIGateway 相关 CRD 资源。
实现与资源模型参考 https://github.com/envoyproxy/gateway.git

### 协议兼容与端点
优先支持最常用端点，例如：
- `/v1/chat/completions`
- `/v1/completions`
- `/v1/embeddings`
- `/v1/models`
- `/v1/images/generations`
- `/v1/audio/transcriptions`

### 核心模块设计
- Router：基于模型与规则的路由引擎，支持权重与优先级。
- Provider Adapter：屏蔽各厂商差异，负责鉴权、字段映射与错误标准化。
- Policy Engine：限流/配额/熔断/重试等策略统一编排。
- Usage Meter：token/字符统计与计费维度输出。
- Stream Handler：SSE/HTTP chunk 流式传输与背压控制。

### 模块设计（第一阶段数据面）
#### 模块一：配置加载模块（参考 envoyproxy/gateway）
目标：以 Gateway API + AIGateway CRD 为主要输入，生成数据面可消费的统一配置快照，支持热更新与一致性保障。

职责边界：
- 监听资源变化：Gateway API（Gateway、HTTPRoute、BackendRefs 等）与 AIGateway CRD。
- 资源解析与校验：结构校验、跨资源关联检查、默认值填充。
- 语义融合与编译：将多来源配置编译为内部统一快照（Listeners、Routes、Providers、Policies）。
- 版本化与发布：生成不可变快照，支持增量/全量更新与回滚。
- 运行时分发：在控制面/数据面之间提供订阅或推送机制。

内部接口与数据结构（概念）：
- `ConfigSource`：负责对接不同来源（K8s、文件、环境变量）。
  - `List()` / `Watch()` 提供资源事件流。
- `ConfigCompiler`：资源编译器。
  - `Compile(resources) -> ConfigSnapshot`
- `ConfigSnapshot`：不可变、版本化配置，包含：
  - `listeners[]`、`routes[]`、`providers[]`、`policies[]`、`secrets[]`
  - `version`、`checksum`、`generated_at`
- `ConfigStore`：缓存最近 N 个快照，支持 `GetLatest()` 与 `Get(version)`。

关键流程：
1. 监听资源事件，构建资源索引。
2. 编译为快照：解析 -> 校验 -> 关联 -> 默认值 -> 生成内部模型。
3. 快照发布：只有校验通过的快照才发布；失败则保留旧版本。
4. 数据面热更新：数据面订阅新快照并原子切换。

容错与一致性：
- 资源缺失或不一致时拒绝发布新快照，保持旧版本。
- 快照内带校验哈希，用于数据面比对和幂等更新。
- 支持灰度发布：按监听器或路由粒度发布（可选）。

#### 模块二：协议转换模块
目标：在统一 LLM API 与各厂商协议之间进行请求/响应映射，保证兼容性与可扩展性。

职责边界：
- 统一请求模型 -> 供应商请求模型的字段映射。
- 供应商响应 -> 统一响应模型的标准化与错误归一。
- 流式协议适配：SSE、chunked、WebSocket（可选）。
- 参数与能力协商：模型能力、参数支持范围、默认值。

内部接口（概念）：
- `Codec`：`Encode(req UnifiedRequest) -> ProviderRequest`
  - `Decode(resp ProviderResponse) -> UnifiedResponse`
  - `DecodeStream(chunk) -> StreamEvent`
- `CapabilityMatrix`：模型能力声明，驱动参数白名单与降级策略。
- `ErrorMapper`：将供应商错误统一映射为标准错误码与语义。

关键流程：
1. 请求入站：校验统一请求参数 -> 规范化 -> 编码为目标协议。
2. 响应回写：解码供应商响应 -> 标准化 -> 补齐 usage 与 metadata。
3. 流式回写：分片解码并标准化事件类型（delta、finish、usage）。

扩展点：
- 每个供应商实现独立 `Codec` 与 `CapabilityMatrix`。
- 通过注册表按 `provider` 与 `endpoint` 选择 codec。

#### 模块三：转发模块
目标：高性能、可观测的上游请求执行层，负责连接管理、重试与故障处理。

职责边界：
- 连接复用：HTTP/2、Keep-Alive、连接池。
- 传输策略：超时、重试、熔断、并发限制。
- 认证注入：按 provider 注入 API Key/JWT/mTLS。
- 可靠性：失败重试、幂等保障、fallback。
- 观测：请求/响应指标、链路追踪、审计日志。

内部组件（概念）：
- `UpstreamClient`：对单一供应商的 HTTP 客户端与连接池。
- `TransportPolicy`：超时、重试、熔断配置。
- `RetryPlanner`：基于错误码与语义选择重试/切换上游。
- `Forwarder`：对外暴露 `Do(ctx, ProviderRequest) -> ProviderResponse`。

关键流程：
1. 选择上游：路由结果 + 策略（权重、健康度）。
2. 请求发送：签名/鉴权注入 -> 发送 -> 处理重试/超时。
3. 响应处理：统一错误语义，记录 metrics/traces。

性能与安全：
- 优先使用标准库 `net/http` + 自定义 Transport 参数。
- 支持请求体复用或缓存，避免重复编码。
- 对敏感头与请求体做脱敏日志。

### 数据结构与接口
定义内部统一请求/响应模型（Model、Prompt、Messages、Params、Usage），并提供：
- `Provider` 接口：`Prepare(req)`, `Do(req)`, `Parse(resp)`
- `Router` 接口：`Route(req)` 返回目标 Provider 列表

## 控制面
第二阶段完善控制面能力：
- 配置管理：集中式配置存储与热加载，支持灰度发布。
- 管理 API：模型/路由/供应商/策略的增删改查。
- 运行态观测：健康检查、路由命中率、错误分布、延迟分布。

## 配置设计
配置设计参考 https://aigateway.envoyproxy.io/docs/api/

建议采用 YAML/JSON，核心结构包含：
- `listeners`：对外监听与鉴权方式
- `providers`：上游 LLM 供应商与模型映射
- `routes`：路由规则与权重
- `policies`：限流、配额、重试、超时

示例（概念草案）：
```yaml
providers:
  - name: openai
    base_url: https://api.openai.com
    api_key: ${OPENAI_KEY}
routes:
  - model: gpt-4o
    provider: openai
    weight: 100
policies:
  timeout_ms: 30000
  retries: 2
  rate_limit_rps: 100
```

## 非功能要求
- 性能：高并发、低延迟、连接复用、零拷贝优先。
- 可靠性：超时、重试、熔断、降级与可回退策略。
- 安全：密钥管理、请求脱敏、最小权限。
- 可观测：OpenTelemetry、结构化日志与指标输出。

## api

```http
POST /v1/chat/completions
Content-Type: application/json
Host: localhost:8080
{
  "model": "gpt-4o",
  "messages": [
    {"role": "system", "content": "You are a helpful assistant."},
    {"role": "user", "content": "Hello, world!"}
  ],
  "max_tokens": 100,
  "temperature": 0.7,
  "stream": false
}
```

```http
POST /responses
Content-Type: application/json
Host: localhost:8080
{
  "model": "gpt-5.2",
  "input": "Tell me a three sentence bedtime story about a unicorn."
}
```
