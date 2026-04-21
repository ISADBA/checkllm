# GPT-5-x 模型真伪校验引擎开发设计

更新时间：2026-04-20

## 1. 目标

第一阶段要做的不是一个全功能平台，而是一个可以在命令行直接运行的 Go 校验引擎：

- 用户在 CLI 传入 `base_url`、`api_key`、`model`
- 引擎自动执行一组针对 `gpt-5-x` 的校验任务
- 输出结构化数字指标
- 在数字指标之外，增加一个“评估解读模块”
- 解读模块结合当前结果与历史结果，对弹性指标给出判定
- 仅关注返回的 `usage` token 信息是否符合预期，不对接任何财务或账单系统
- 暂时不依赖数据库
- 如需存储，使用本地 Markdown 文件

第一阶段只支持 `gpt-5-x` 家族，后续再逐步扩展到其他模型供应商。

## 2. 非目标

第一阶段暂时不做：

- 多供应商统一抽象
- Web UI
- 分布式多地域探针网络
- 数据库存储
- watermark / attestation
- 全自动模型家族归因

第一阶段聚焦于：

- 单 endpoint 的真实性与保真度风险评估
- token / usage 合理性校验
- 流式行为与协议一致性校验
- 同品牌降级风险的初步识别
- 历史结果比较与弹性解读

## 3. 输入与输出

## 3.1 CLI 输入

建议提供如下命令形态：

```bash
checkllm run \
  --base-url https://api.openai.com/v1 \
  --api-key $OPENAI_API_KEY \
  --model gpt-5.4 \
  --provider openai \
  --baseline docs/baselines/openai-gpt-5.4.md \
  --output docs/runs/2026-04-20-gpt-5.4.md
```

第一阶段关键参数：

- `--base-url`
- `--api-key`
- `--model`
- `--provider`
- `--baseline`
- `--output`
- `--timeout`
- `--max-samples`
- `--enable-stream`

其中：

- `provider` 第一阶段默认只支持 `openai`
- `baseline` 指向该模型的官方基线文件
- `output` 为本次评测结果 Markdown 路径

## 3.2 输出

输出分成两层：

### A. 数值结果层

机器可读，面向自动比较：

- `protocol_conformity_score`
- `stream_conformity_score`
- `usage_consistency_score`
- `behavior_fingerprint_score`
- `tier_fidelity_score`
- `route_integrity_score`
- `overall_risk_score`

### B. 解读层

面向用户理解与历史比较：

- 当前结果是否落在官方基线范围内
- 与历史评测相比是改善、退化还是波动正常
- 哪些指标属于硬异常
- 哪些指标属于弹性偏差
- 最终结论是：
  - `高置信官方一致`
  - `存在兼容但不保真风险`
  - `疑似同牌降级`
  - `疑似中转改写`
  - `存在 usage token 异常`

## 4. 第一阶段支持范围

第一阶段只支持 `gpt-5-x`，建议先做如下假设：

- 使用 OpenAI 兼容的 Responses API 或 Chat Completions 风格接口
- 输入为文本任务
- 输出包含普通非工具调用场景
- 流式与非流式都支持，但流式是重点

这里的 `gpt-5-x` 指代的是你计划支持的 `gpt-5.*` 系列目标模型集合。  
具体支持哪些型号，不在代码里写死为单个名字，而是通过基线配置声明，例如：

- `gpt-5.4`
- `gpt-5.4-mini`
- `gpt-5.3`

第一阶段不把“模型名是否存在”作为真伪逻辑的一部分，而是把它当作基线注册表中的一个目标对象。

## 5. 总体架构

建议按如下模块拆分：

1. `cmd/checkllm`
   - CLI 入口
2. `internal/config`
   - 参数解析与运行配置
3. `internal/provider/openai`
   - OpenAI 风格请求发送、流式解析、usage 抽取
4. `internal/probe`
   - 探针定义与执行
5. `internal/metric`
   - 指标计算
6. `internal/judge`
   - 评估解读与历史比较
7. `internal/baseline`
   - 基线加载
8. `internal/report`
   - Markdown 报告输出
9. `internal/history`
   - 历史结果扫描与比对

## 6. 目录建议

```text
checkllm/
  cmd/
    checkllm/
      main.go
  internal/
    baseline/
      loader.go
      model.go
    config/
      config.go
    history/
      loader.go
    judge/
      interpret.go
      threshold.go
    metric/
      score.go
      usage.go
      stream.go
      fingerprint.go
    probe/
      catalog.go
      executor.go
      types.go
    provider/
      openai/
        client.go
        responses.go
        stream.go
        types.go
    report/
      markdown.go
      model.go
  docs/
    baselines/
      openai-gpt-5.4.md
    runs/
```

## 7. 评测流程

单次执行建议走以下流程：

1. 读取 CLI 参数
2. 加载目标模型基线文件
3. 构建 probe 任务集
4. 依次执行 non-stream probes
5. 执行 stream probes
6. 收集原始响应、usage、错误、延迟、chunk 时序
7. 计算数值指标
8. 读取历史运行结果
9. 进行阈值判定与趋势解读
10. 生成 Markdown 报告

## 8. Probe 设计

第一阶段不要追求大而全，应优先覆盖最能反映真实性风险的 4 类 probe。

## 8.1 Protocol Probes

用于校验接口是否真的像官方服务。

建议包含：

- 非流式普通文本请求
- 流式普通文本请求
- 非法参数请求
- 结构化输出请求
- stop / max_output_tokens 边界请求

关注点：

- HTTP 状态码
- 错误体结构
- usage 字段完整性
- finish reason / event 类型
- 流式事件顺序

## 8.2 Usage Probes

用于校验 token 统计与 usage 回包的合理性。

建议包含：

- 超短 prompt
- 中等长度 prompt
- 长 prompt
- 长输出请求
- 流式输出请求

关注点：

- prompt token 变化斜率
- completion token 变化斜率
- 本地估算与服务端 usage 偏差
- 相邻长度任务的 usage 单调性

## 8.3 Fingerprint Probes

用于识别“是不是这个家族的模型行为”。

建议包含：

- refusal 风格任务
- 格式遵循任务
- 多约束指令任务
- 冲突指令任务
- 多轮承接任务

第一阶段不做很重的自动对抗搜索，而是先做一批人工设计的高区分 probe。

## 8.4 Tier Probes

用于识别“是不是同品牌低配冒充高配”。

建议包含：

- 长约束代码修复
- 多条件结构化抽取
- 长上下文定位题
- 复杂输出稳定性题
- 重复采样一致性题

## 9. 指标设计

第一阶段所有结果都应数字化，并保留原始观测值。

## 9.1 协议一致性类

- `protocol_conformity_score`
  - 0-100
  - 检查 schema、status code、error body、字段完整性
- `stream_conformity_score`
  - 0-100
  - 检查事件顺序、chunk 结构、终止事件、usage 收尾

## 9.2 usage 类

- `usage_consistency_score`
  - 0-100
  - 检查 usage 与本地估算的偏差

## 9.3 行为类

- `behavior_fingerprint_score`
  - 0-100
  - 检查 refusal、格式偏好、冲突指令服从、风格稳定性
- `tier_fidelity_score`
  - 0-100
  - 检查高阶能力任务的表现与基线差距

## 9.4 路径类

- `route_integrity_score`
  - 0-100
  - 第一阶段只基于单次执行内的流式节奏、重试一致性、包装层痕迹做初步估计

## 9.5 总体类

- `overall_risk_score`
  - 0-100
  - 分数越高表示风险越高

建议：

- 分项分数大多使用“越高越好”
- 总风险分使用“越高越危险”

这样对外展示时更直观。

## 10. 基线文件设计

由于不接数据库，建议把官方基线存在 Markdown 文件中，但 Markdown 内要带结构化区块，方便 Go 解析。

建议格式：

~~~md
# Baseline: openai / gpt-5.4

## Metadata

```yaml
provider: openai
model: gpt-5.4
api_style: responses
updated_at: 2026-04-20
```

## Expected Ranges

```yaml
protocol_conformity_score:
  min: 95
stream_conformity_score:
  min: 92
usage_prompt_deviation_ratio:
  max: 0.08
usage_completion_deviation_ratio:
  max: 0.10
behavior_fingerprint_score:
  min: 85
tier_fidelity_score:
  min: 88
```

## Notes

- 基于官方直连环境测得
- 流式事件遵循当前 OpenAI 官方行为窗口
~~~

核心原则：

- 文件对人类可读
- YAML 区块对程序可解析
- 后续可以版本化管理

## 11. 历史结果文件设计

每次运行生成一个 Markdown 结果文件，同时在文件中嵌入结构化 YAML 摘要。

建议格式：

~~~md
# Run Report

## Metadata

```yaml
run_at: 2026-04-20T15:00:00+08:00
provider: openai
model: gpt-5.4
base_url: https://api.openai.com/v1
```

## Scores

```yaml
protocol_conformity_score: 98
stream_conformity_score: 96
usage_consistency_score: 91
behavior_fingerprint_score: 89
tier_fidelity_score: 87
route_integrity_score: 84
overall_risk_score: 12
```

## Interpretation

- 当前结果总体处于官方基线范围内
- `tier_fidelity_score` 低于历史中位数 6 分，属于轻微退化
- `route_integrity_score` 波动偏大，建议复测
~~~

## 12. 评估解读模块设计

你特别提到“通过与历史指标数值比较，给出结果判定”，这部分应单独建模，不能只是简单阈值判断。

## 12.1 解读模块输入

- 当前运行指标
- 对应模型的官方基线
- 同一 `base_url + model` 的历史结果

## 12.2 解读模块输出

- 每个指标的状态：
  - `正常`
  - `轻微偏离`
  - `显著偏离`
  - `硬异常`
- 趋势状态：
  - `稳定`
  - `轻微退化`
  - `显著退化`
  - `可疑波动`
- 总结论

## 12.3 判定逻辑

建议把指标分两类：

### A. 硬指标

一旦异常，直接给硬告警：

- schema 不兼容
- 流式终止事件缺失
- usage 字段缺失
- 错误体结构不符

### B. 弹性指标

不能只看单次值，需要结合历史：

- latency 类
- stream cadence 类
- behavior 指纹相似度
- tier 能力分

对弹性指标建议同时参考：

- 官方基线阈值
- 历史中位数
- 历史波动范围

可采用以下简化规则：

- 落在基线范围内且接近历史中位数：`正常`
- 超出历史常态但未越过基线硬阈值：`轻微偏离`
- 越过基线硬阈值但无硬异常：`显著偏离`
- 触发硬规则：`硬异常`

## 13. 打分策略建议

第一阶段不建议上复杂机器学习模型，先用“规则 + 归一化分数”。

## 13.1 分项评分

每个分项得分来自：

- 若干原子检查项
- 每项有权重
- 产出 0-100 分

例如：

`protocol_conformity_score` 可由以下项组成：

- HTTP status 行为
- 响应字段完整性
- 错误字段完整性
- finish reason 合法性
- 结构化输出字段合法性

## 13.2 总风险分

总风险分建议不是简单平均，而是按风险加权：

- 协议硬异常权重最高
- usage 异常次高
- tier_fidelity 与 behavior 为中高
- route_integrity 初期权重中等

示意：

```text
overall_risk_score =
  100 - weighted_sum_of_good_scores
  + hard_anomaly_penalty
```

这样可以确保：

- 一个协议硬异常不会被其他高分掩盖
- 轻微能力波动不会被误判成高风险造假

## 14. OpenAI Provider 抽象建议

虽然第一阶段只支持 OpenAI，但仍然建议保留 provider 接口，避免后续重构。

建议接口：

```go
type Provider interface {
    Name() string
    Execute(ctx context.Context, req ProbeRequest) (ProbeResult, error)
}
```

`ProbeRequest` 至少包括：

- model
- prompt/messages
- stream
- temperature
- max_output_tokens
- expected_mode

`ProbeResult` 至少包括：

- raw_response
- parsed_response
- usage
- stream_events
- latency
- error_info

## 15. OpenAI 兼容性实现策略

第一阶段默认优先适配 OpenAI 官方接口行为。  
对于三方 OpenAI-compatible 服务，按“兼容度”评分，而不是假设其一定完全一致。

建议重点实现：

- non-stream request
- stream request
- usage 抽取
- 错误体抽取
- 事件序列记录

注意事项：

- 很多中转平台会兼容最基本字段，但会在 stream event、usage、error body 上露出差异
- 这些差异应保留为原始证据，而不是只保留最终分数

## 16. Markdown 存储策略

虽然用 Markdown 存储是可行的，但要防止“只有文本描述，缺乏程序可读性”。

因此约束如下：

- 每个 Markdown 文件必须包含 YAML 代码块
- YAML 代码块用于结构化解析
- 人类可读解释写在普通 Markdown 段落中

第一阶段建议有两类文件：

- `docs/baselines/*.md`
- `docs/runs/*.md`

后续如果量变大，再平滑迁移到 SQLite 或其他存储。

## 17. 最小可交付版本

第一版建议只交付一个 `run` 命令，完成以下能力：

1. 接收 `base_url`、`api_key`、`model`
2. 调用一组固定 probe
3. 计算 6-7 个核心数值指标
4. 读取一个官方基线 Markdown
5. 读取同模型历史结果 Markdown
6. 输出一份带数字和解读的 Markdown 报告

只要这一步做扎实，就已经具备 review 和迭代基础。

## 18. 第一阶段开发顺序

建议按这个顺序做：

1. 初始化 Go 项目结构
2. 实现 CLI 参数解析
3. 实现 OpenAI provider 最小调用能力
4. 实现 probe 执行框架
5. 实现 protocol / usage 两类指标
6. 实现 Markdown 基线解析
7. 实现历史结果解析
8. 实现 judge 解读模块
9. 实现 Markdown 报告输出
10. 最后再补 fingerprint / tier probes

原因是：

- protocol 和 usage 最容易形成稳定评测闭环
- judge 模块要尽早定义数据结构
- fingerprint 和 tier probe 最容易消耗调参时间，应放后面

## 19. Review 时最需要确认的几个决策

这份设计里有几个需要你重点 review：

1. 第一阶段是否明确只支持 OpenAI `gpt-5-x`
2. 基线文件是否接受“Markdown + YAML 区块”形式
3. 是否明确把“费用”范围限定为 `usage token` 合理性校验
4. CLI 是否只做单次执行，不做 daemon / server
5. 历史对比是否以“同 base_url + 同 model”为聚合键
6. 是否先以规则评分为主，暂不上 ML 判定

## 20. 我的建议

如果是我来推进第一版，我会把 MVP 约束得更硬一些：

- 只支持 `openai`
- 只支持一个主命令 `checkllm run`
- 只实现 12-20 个固定 probe
- 只输出 Markdown 报告
- 只做规则化评分和历史比较

不要第一步就扩成通用评测平台。  
先把“单 endpoint 的真实性、保真度、usage 风险”做成一个能重复运行、能比较历史、能输出明确结论的 Go 工具，这条路径最稳。
