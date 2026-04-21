# checkllm

`checkllm` 是一个基于 Go 实现的命令行模型校验工具，用来判断某个 LLM 接口是否真正符合声明的模型身份、协议行为和能力特征，而不是只看它“能不能回答问题”。

它当前面向两类接口：

- OpenAI 风格的 `/responses` 接口
- Anthropic 风格的 `/v1/messages` 接口

项目的目标不是做通用压测平台，而是做一次可复现的“模型真实性 / 保真度”检查：对目标接口发起一组探针请求，分析协议一致性、usage 回包、行为指纹、工具调用能力、流式输出和历史结果，再给出结构化风险结论。

## 软件用途

这个工具主要适合下面几类场景：

- 校验某个 `base_url + model` 是否真的对应声称的官方模型
- 识别“OpenAI 兼容接口但底层被替换、降级或改写”的情况
- 对比不同供应商、代理平台或中转层的返回行为差异
- 将多次校验结果落盘，做同一模型接口的历史趋势比对
- 给用户或内部团队产出一份可阅读的 Markdown 鉴定报告

它关注的不是传统 benchmark 分数，而是“接口行为像不像它声称的那一类模型”。

## 底层逻辑设计

### 1. 执行链路

程序实际执行流程如下：

1. 解析 CLI 参数
2. 加载目标模型的基线文件
3. 根据 `model` 匹配基线并确定 `provider`，再生成默认探针集合
4. 按探针逐个调用目标接口
5. 收集文本、状态码、usage、tool call、stream event、延迟等原始数据
6. 计算多项评分
7. 读取同目录历史报告，做横向比较
8. 生成最终结论
9. 输出两份 Markdown 报告

对应主入口位于 [cmd/checkllm/main.go](cmd/checkllm/main.go)。

### 2. 核心模块

- [cmd/checkllm/main.go](cmd/checkllm/main.go)：程序入口，串联整个执行流程
- [internal/config/config.go](internal/config/config.go)：解析命令行参数并生成运行配置
- [internal/baseline/loader.go](internal/baseline/loader.go)：加载基线 Markdown 中的 YAML 元数据和指标范围
- [internal/probe/catalog.go](internal/probe/catalog.go)：定义默认探针集合
- [internal/probe/executor.go](internal/probe/executor.go)：执行探针并收集原始结果
- [internal/provider/openai/client.go](internal/provider/openai/client.go)：OpenAI 风格接口适配
- [internal/provider/anthropic/client.go](internal/provider/anthropic/client.go)：Anthropic 风格接口适配
- [internal/metric/score.go](internal/metric/score.go)：把探针结果计算成结构化分数
- [internal/judge/interpret.go](internal/judge/interpret.go)：结合分数、基线和历史报告生成风险结论
- [internal/history/loader.go](internal/history/loader.go)：加载同模型历史运行记录
- [internal/report/markdown.go](internal/report/markdown.go)：输出完整档案和用户报告

### 3. 探针设计思路

默认探针不是随机问答，而是围绕“真实性识别”设计的固定检查项，主要包括：

- `protocol`：检查最基本的协议兼容性、JSON 结构、错误行为和 usage 返回
- `usage`：检查 token 统计是否返回、是否合理、是否随输入长度变化
- `fingerprint`：检查文本输出风格、JSON 遵循性、身份自报一致性、是否出现额外包装层痕迹
- `capability`：检查 tool call / function call 是否按预期触发，是否能接上工具结果继续生成
- `stream`：检查流式事件数量、done 事件、首包延迟和事件覆盖率

这些探针组合起来，能比单次问答更容易识别：

- 假兼容接口
- 同品牌低阶模型冒充高阶模型
- 中转层改写输出
- 平台层注入额外系统提示
- usage/token 回包异常

### 4. 评分与判定逻辑

当前实现会计算这些核心分数：

- `protocol_conformity_score`
- `stream_conformity_score`
- `usage_consistency_score`
- `behavior_fingerprint_score`
- `capability_tool_score`
- `tier_fidelity_score`
- `route_integrity_score`
- `overall_risk_score`

大体逻辑是：

- 单项分数越高，说明该维度越接近预期
- `overall_risk_score` 越高，说明风险越大
- 如果出现协议失败、usage 丢失等硬异常，会直接显著提高总体风险
- 判定模块会把分数与基线文件中的范围对比，再结合历史结果做解释

最终结论可能包括：

- `high_confidence_official_compatible`
- `compatibility_with_wrapper_risk`
- `suspected_same_brand_downgrade`
- `usage_token_anomaly`
- `suspected_route_or_protocol_mismatch`
- `suspected_wrapper_or_hidden_prompt`

### 5. 基线文件机制

每个目标模型对应一个 Markdown 基线文件，例如：

- [docs/baselines/openai-gpt-5.4.md](docs/baselines/openai-gpt-5.4.md)
- [docs/baselines/anthropic-claude-opus-4-7.md](docs/baselines/anthropic-claude-opus-4-7.md)

基线文件里包含两部分：

- 元数据：`provider`、`model`、`api_style`、`updated_at`
- 指标范围：各项分数的 `min/max`

当用户未显式传入 `--provider` 时，程序会扫描 `docs/baselines/`，按 `model` 匹配唯一基线，并从基线元数据中反推出对应 provider。

程序不会把“模型名写死在代码里”作为唯一依据，而是通过“基线匹配 + 探针结果 + 历史比较”综合判断。

## 目录结构

```text
checkllm/
  cmd/
    checkllm/
      main.go
  docs/
    baselines/
    runs/
    repos/
  internal/
    baseline/
    config/
    history/
    judge/
    metric/
    probe/
    provider/
      anthropic/
      openai/
    report/
  go.mod
```

## 使用方法

### 1. 环境要求

- Go 版本需满足 `go.mod` 中定义的要求
- 目标接口可被当前机器访问
- 需要有效的 API Key

### 2. 构建

```bash
env GOCACHE=/tmp/go-cache-checkllm-build go build ./...
```

### 3. 运行 OpenAI 风格接口校验

```bash
go run ./cmd/checkllm run \
  --base-url https://api.openai.com/v1 \
  --api-key $OPENAI_API_KEY \
  --model gpt-5.4 \
  --baseline docs/baselines/openai-gpt-5.4.md \
  --output docs/runs/local-gpt-5.4.md
```

### 4. 运行 Anthropic 风格接口校验

```bash
go run ./cmd/checkllm run \
  --base-url https://api.anthropic.com \
  --api-key $ANTHROPIC_API_KEY \
  --model claude-opus-4-7 \
  --baseline docs/baselines/anthropic-claude-opus-4-7.md \
  --output docs/runs/local-claude-opus-4-7.md
```

如果是 Anthropic 风格但经由 OpenRouter 一类聚合平台，`--base-url` 仍应传平台提供的基础地址。项目中的 Anthropic provider 会按自身逻辑拼接消息接口路径。

### 5. 常用参数

- `--base-url`：目标接口基础地址
- `--api-key`：调用凭证
- `--model`：目标模型名
- `--provider`：可选；当前支持 `openai` 和 `anthropic`，通常可由 baseline 自动推断
- `--baseline`：基线文件路径；不传时会扫描 `docs/baselines/`，按 `model` 匹配唯一基线
- `--output`：运行档案输出路径；不传时会自动输出到 `docs/runs/`
- `--timeout`：单个探针超时时间，默认 `90s`
- `--max-samples`：重复探针采样次数，默认 `2`
- `--enable-stream`：是否启用流式探针，默认 `true`
- `--expect-usage`：是否要求接口返回 usage 字段，默认 `true`

最简命令形态如下：

```bash
go run ./cmd/checkllm run \
  --base-url <API_BASE_URL> \
  --api-key <API_KEY> \
  --model <MODEL_NAME>
```

这个最简命令同时适用于 OpenAI 和 Anthropic，只要 `docs/baselines/` 中存在该模型唯一对应的基线文件。如果模型还没有基线，或你想强制指定 provider，再额外传 `--provider` 或 `--baseline`。

### 6. 输出结果

程序执行后会产生两类结果：

- 终端输出：关键分数和结论摘要
- `docs/runs/*.md`：完整运行档案，保留 probe 输入、原始请求、原始响应、tool call、stream event 等信息
- `docs/repos/*.md`：面向用户阅读的简化解释报告

其中：

- `docs/runs/` 适合工程排查和复现
- `docs/repos/` 适合直接发给业务方或客户阅读

### 7. 历史对比机制

程序会读取当前输出目录下已有的历史 Markdown 报告，并筛选出：

- 相同 `base_url`
- 相同 `model`

然后把本次分数与历史结果一起用于解释阶段判断。因此，同一个目标建议把多次结果落在同一类目录下，便于趋势分析。

## 适用边界与当前限制

- 当前主要支持 OpenAI `/responses` 和 Anthropic `/v1/messages`
- 目前探针以文本类任务为主
- `usage` 判断仍属于接口返回值与本地粗粒度估算的组合校验
- 流式行为分析还是第一版实现
- Prompt Cache 的专项识别仍在持续补充
- 基线文件质量会直接影响判定稳定性，建议基于官方真实样本持续更新

## 开发建议

如果你要扩展这个项目，优先考虑这几个方向：

- 为更多 provider 增加适配层
- 给不同模型家族建立更稳定的 baseline
- 增加更强的身份一致性探针和多轮探针
- 强化 stream / tool call / prompt cache 的专项校验
- 给报告增加 diff 视图和历史趋势汇总

## 安全建议

- 不要把真实 API Key 写入仓库
- 不要把包含敏感请求数据的运行档案直接公开
- 如果要分享报告，优先分享 `docs/repos/` 下的用户报告，而不是完整的 `docs/runs/` 档案
