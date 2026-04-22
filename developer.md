# Developer Guide

这个文档面向项目维护者和贡献者，主要说明代码组织、源码构建、源码运行和贡献方式。

## 代码组织结构

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
  Makefile
```

核心模块说明：

- [cmd/checkllm/main.go](cmd/checkllm/main.go)：程序入口，串联执行流程
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

## 源码编译

### 使用 Makefile

```bash
make build
```

默认编译当前平台二进制，输出到 `dist/<goos>-<goarch>/checkllm`。
构建前会自动执行 baseline 嵌入代码生成，确保二进制内置的模板与 `docs/baselines/*.md` 保持一致。

```bash
make build all
```

编译全部预设平台二进制。

```bash
make build linux-amd64
```

编译指定平台二进制。当前支持的平台列表可用 `make help` 查看。

### 直接使用 Go

```bash
env GOCACHE=/tmp/go-cache-checkllm-build go build ./...
```

如果只想编译主程序：

```bash
go build -o ./dist/local/checkllm ./cmd/checkllm
```

如果你修改了 `docs/baselines/*.md`，建议先执行：

```bash
go generate ./internal/baseline
```

这样会重新生成内置 baseline 模板代码。

## GitHub Actions 发布

仓库内置了一个 GitHub Actions workflow：

- [release.yml](.github/workflows/release.yml)

当前行为：

- 推送 `v*` tag 时，自动编译全部预设平台二进制
- 自动打包发布产物
- 自动创建或更新对应的 GitHub Release
- 自动上传各平台压缩包和 `checksums.txt`

当前发布产物命名规则：

- Unix 平台：`checkllm_<goos>_<goarch>.tar.gz`
- Windows 平台：`checkllm_<goos>_<goarch>.zip`

如果你只想手动验证 workflow，也可以在 GitHub Actions 页面直接执行 `workflow_dispatch`。

## 源码运行

### 运行 OpenAI 风格接口校验

```bash
go run ./cmd/checkllm run \
  --base-url https://api.openai.com/v1 \
  --api-key $OPENAI_API_KEY \
  --model gpt-5.4 \
  --baseline docs/baselines/openai-gpt-5.4.md \
  --output docs/runs/local-gpt-5.4.md
```

### 运行 Anthropic 风格接口校验

```bash
go run ./cmd/checkllm run \
  --base-url https://api.anthropic.com \
  --api-key $ANTHROPIC_API_KEY \
  --model claude-opus-4-7 \
  --baseline docs/baselines/anthropic-claude-opus-4-7.md \
  --output docs/runs/local-claude-opus-4-7.md
```

最简命令形态如下：

```bash
go run ./cmd/checkllm run \
  --base-url <API_BASE_URL> \
  --api-key <API_KEY> \
  --model <MODEL_NAME>
```

如果 `docs/baselines/` 中存在该模型唯一对应的基线文件，可以不显式传 `--provider` 和 `--baseline`。否则需要手动指定。

当用户运行二进制且本地缺少 `docs/baselines/` 或缺少默认 baseline 文件时，程序会自动把二进制内置模板写入本地默认目录，但不会覆盖已有文件。

## 参数与输出

常用参数：

- `--base-url`：目标接口基础地址
- `--api-key`：调用凭证
- `--model`：目标模型名
- `--provider`：可选；当前支持 `openai` 和 `anthropic`
- `--baseline`：基线文件路径
- `--output`：运行档案输出路径
- `--timeout`：单个探针超时时间，默认 `90s`
- `--max-samples`：重复探针采样次数，默认 `2`
- `--enable-stream`：是否启用流式探针，默认 `true`
- `--expect-usage`：是否要求接口返回 usage 字段，默认 `true`

程序执行后会产生：

- `docs/runs/*.md`：完整运行档案，适合工程排查
- `docs/repos/*.md`：简化解释报告，适合对外分享

## 开发与贡献建议

如果你要扩展这个项目，优先考虑这几个方向：

- 为更多 provider 增加适配层
- 给不同模型家族建立更稳定的 baseline
- 增加更强的身份一致性探针和多轮探针
- 强化 stream / tool call / prompt cache 的专项校验
- 给报告增加 diff 视图和历史趋势汇总

贡献时建议遵循这些原则：

- 新增 provider 时，先补适配层，再补 probe 兼容逻辑，再补 baseline
- 修改评分逻辑时，同时检查 `docs/runs/` 和 `docs/repos/` 的解释是否仍然一致
- 新增 probe 时，明确哪些必须独立请求，哪些可以通过 `ReuseResultFrom` 复用结果
- 提交前至少跑通一次本地构建和一次真实或可复现的示例校验

## 安全与协作注意事项

- 不要把真实 API Key 写入仓库
- 不要把包含敏感请求数据的运行档案直接公开
- 如果要分享报告，优先分享 `docs/repos/`
- 调整 baseline 前，优先保留对应历史样本，避免阈值漂移后难以回溯
