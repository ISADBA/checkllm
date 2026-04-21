package report

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/ISADBA/checkllm/internal/judge"
)

func WriteArchiveMarkdown(path string, run RunReport) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	var b strings.Builder
	b.WriteString("# Run Archive\n\n")

	b.WriteString("## Metadata\n\n```yaml\n")
	fmt.Fprintf(&b, "run_at: %s\n", run.RunAt.Format("2006-01-02T15:04:05Z07:00"))
	fmt.Fprintf(&b, "provider: %s\n", run.Config.Provider)
	fmt.Fprintf(&b, "model: %s\n", run.Config.Model)
	fmt.Fprintf(&b, "base_url: %s\n", run.Config.BaseURL)
	fmt.Fprintf(&b, "baseline_path: %s\n", run.Config.BaselinePath)
	fmt.Fprintf(&b, "user_report_path: %s\n", run.Config.UserReportPath())
	b.WriteString("```\n\n")

	b.WriteString("## Scores\n\n```yaml\n")
	fmt.Fprintf(&b, "protocol_conformity_score: %d\n", run.Scores.ProtocolConformityScore)
	fmt.Fprintf(&b, "stream_conformity_score: %d\n", run.Scores.StreamConformityScore)
	fmt.Fprintf(&b, "usage_consistency_score: %d\n", run.Scores.UsageConsistencyScore)
	fmt.Fprintf(&b, "behavior_fingerprint_score: %d\n", run.Scores.BehaviorFingerprintScore)
	fmt.Fprintf(&b, "capability_tool_score: %d\n", run.Scores.CapabilityToolScore)
	fmt.Fprintf(&b, "tier_fidelity_score: %d\n", run.Scores.TierFidelityScore)
	fmt.Fprintf(&b, "route_integrity_score: %d\n", run.Scores.RouteIntegrityScore)
	fmt.Fprintf(&b, "functional_score: %d\n", run.Categories.Functional.Score)
	fmt.Fprintf(&b, "intelligence_score: %d\n", run.Categories.Intelligence.Score)
	fmt.Fprintf(&b, "overall_risk_score: %d\n", run.Scores.OverallRiskScore)
	b.WriteString("```\n\n")

	b.WriteString("## Feature Detection\n\n```yaml\n")
	fmt.Fprintf(&b, "thinking: %s\n", quoteYAML(run.Thinking.Status))
	fmt.Fprintf(&b, "prompt_cache: %s\n", quoteYAML(run.PromptCache.Status))
	fmt.Fprintf(&b, "prompt_cache_key: %s\n", quoteYAML(run.PromptCache.ObservedKey))
	fmt.Fprintf(&b, "prompt_cache_retention: %s\n", quoteYAML(run.PromptCache.Retention))
	fmt.Fprintf(&b, "prompt_cache_cached_tokens: %d\n", run.PromptCache.CachedTokens)
	b.WriteString("```\n\n")

	b.WriteString("## Token Usage\n\n```yaml\n")
	fmt.Fprintf(&b, "input_tokens: %d\n", run.TokenUsage.InputTokens)
	fmt.Fprintf(&b, "output_tokens: %d\n", run.TokenUsage.OutputTokens)
	fmt.Fprintf(&b, "total_tokens: %d\n", run.TokenUsage.TotalTokens)
	b.WriteString("```\n\n")

	b.WriteString("## Network\n\n```yaml\n")
	fmt.Fprintf(&b, "avg_latency_ms: %d\n", run.Network.AvgLatencyMs)
	fmt.Fprintf(&b, "p95_latency_ms: %d\n", run.Network.P95LatencyMs)
	fmt.Fprintf(&b, "avg_first_byte_ms: %d\n", run.Network.AvgFirstByteMs)
	fmt.Fprintf(&b, "avg_output_tokens_per_s: %.2f\n", run.Network.AvgOutputTokensPerS)
	fmt.Fprintf(&b, "avg_stream_events: %.2f\n", run.Network.AvgStreamEvents)
	fmt.Fprintf(&b, "timeout_count: %d\n", run.Network.TimeoutCount)
	fmt.Fprintf(&b, "successful_probe_count: %d\n", run.Network.SuccessfulProbeCount)
	b.WriteString("```\n\n")

	b.WriteString("## Interpretation\n\n")
	fmt.Fprintf(&b, "- Conclusion: `%s`\n", run.Judgement.Conclusion)
	for _, summary := range run.Judgement.Summaries {
		fmt.Fprintf(&b, "- %s\n", summary)
	}

	if len(run.Scores.Observations) > 0 {
		b.WriteString("\n## Observations\n\n")
		keys := make([]string, 0, len(run.Scores.Observations))
		for key := range run.Scores.Observations {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			fmt.Fprintf(&b, "- `%s`: %s\n", key, formatObservation(key, run.Scores.Observations[key]))
		}
	}

	b.WriteString("\n## Probe Archive\n\n")
	for _, result := range run.ProbeResults {
		b.WriteString("### " + result.Definition.Name + "\n\n")
		b.WriteString("```yaml\n")
		fmt.Fprintf(&b, "kind: %s\n", result.Definition.Kind)
		fmt.Fprintf(&b, "status_code: %d\n", result.StatusCode)
		fmt.Fprintf(&b, "stream: %t\n", result.Definition.Stream)
		fmt.Fprintf(&b, "temperature: %s\n", strconv.FormatFloat(result.Definition.Temperature, 'f', -1, 64))
		fmt.Fprintf(&b, "reasoning_effort: %s\n", quoteYAML(result.Definition.ReasoningEffort))
		fmt.Fprintf(&b, "max_output_tokens: %d\n", result.Definition.MaxOutputTokens)
		fmt.Fprintf(&b, "expect_usage: %t\n", result.Definition.ExpectUsage)
		fmt.Fprintf(&b, "expect_json: %t\n", result.Definition.ExpectJSON)
		fmt.Fprintf(&b, "expect_tool_call: %t\n", result.Definition.ExpectToolCall)
		fmt.Fprintf(&b, "expected_tool_name: %s\n", quoteYAML(result.Definition.ExpectedToolName))
		fmt.Fprintf(&b, "latency_ms: %d\n", result.Latency.Milliseconds())
		if result.FirstEventLatency > 0 {
			fmt.Fprintf(&b, "first_event_ms: %d\n", result.FirstEventLatency.Milliseconds())
		}
		fmt.Fprintf(&b, "usage_input_tokens: %d\n", result.Usage.InputTokens)
		fmt.Fprintf(&b, "usage_output_tokens: %d\n", result.Usage.OutputTokens)
		fmt.Fprintf(&b, "usage_total_tokens: %d\n", result.Usage.TotalTokens)
		fmt.Fprintf(&b, "usage_cached_tokens: %d\n", result.Usage.CachedTokens)
		fmt.Fprintf(&b, "usage_returned: %t\n", result.UsageReturned)
		fmt.Fprintf(&b, "prompt_cache_key: %s\n", quoteYAML(result.PromptCacheKey))
		fmt.Fprintf(&b, "prompt_cache_retention: %s\n", quoteYAML(result.PromptCacheRetention))
		fmt.Fprintf(&b, "tool_call_count: %d\n", len(result.ToolCalls))
		fmt.Fprintf(&b, "stream_event_count: %d\n", len(result.StreamEvents))
		if result.Err != nil {
			fmt.Fprintf(&b, "error: %s\n", quoteYAML(result.Err.Error()))
		}
		b.WriteString("```\n\n")

		b.WriteString("#### Input Prompt\n\n```text\n")
		b.WriteString(result.Definition.Prompt)
		b.WriteString("\n```\n\n")

		if len(result.Definition.ExpectedJSONKeys) > 0 || len(result.Definition.ExpectedJSONValues) > 0 || len(result.Definition.ExpectedLineSequence) > 0 || result.Definition.ExpectedPhrase != "" || len(result.Definition.ExpectedToolArgs) > 0 || len(result.Definition.ExpectedFinalPhrases) > 0 {
			b.WriteString("#### Expectations\n\n")
			if result.Definition.ExpectedPhrase != "" {
				fmt.Fprintf(&b, "- expected_phrase: `%s`\n", result.Definition.ExpectedPhrase)
			}
			if len(result.Definition.ExpectedJSONKeys) > 0 {
				fmt.Fprintf(&b, "- expected_json_keys: `%s`\n", strings.Join(result.Definition.ExpectedJSONKeys, ", "))
			}
			if len(result.Definition.ExpectedJSONValues) > 0 {
				fmt.Fprintf(&b, "- expected_json_values: `%s`\n", mustJSON(result.Definition.ExpectedJSONValues))
			}
			if len(result.Definition.ExpectedLineSequence) > 0 {
				fmt.Fprintf(&b, "- expected_line_sequence: `%s`\n", strings.Join(result.Definition.ExpectedLineSequence, " | "))
			}
			if len(result.Definition.ForbiddenSubstrings) > 0 {
				fmt.Fprintf(&b, "- forbidden_substrings: `%s`\n", strings.Join(result.Definition.ForbiddenSubstrings, ", "))
			}
			if len(result.Definition.ExpectedToolArgs) > 0 {
				fmt.Fprintf(&b, "- expected_tool_args: `%s`\n", mustJSON(result.Definition.ExpectedToolArgs))
			}
			if len(result.Definition.ExpectedFinalPhrases) > 0 {
				fmt.Fprintf(&b, "- expected_final_phrases: `%s`\n", strings.Join(result.Definition.ExpectedFinalPhrases, ", "))
			}
			b.WriteString("\n")
		}

		if len(result.Definition.Tools) > 0 {
			b.WriteString("#### Tools\n\n```json\n")
			b.WriteString(mustJSON(result.Definition.Tools))
			b.WriteString("\n```\n\n")
		}

		if len(result.Definition.ToolResults) > 0 {
			b.WriteString("#### Tool Result Inputs\n\n```json\n")
			b.WriteString(mustJSON(result.Definition.ToolResults))
			b.WriteString("\n```\n\n")
		} else if result.Definition.ToolResult != "" {
			b.WriteString("#### Tool Result Input\n\n```json\n")
			b.WriteString(result.Definition.ToolResult)
			b.WriteString("\n```\n\n")
		}

		if result.ErrorBody != "" {
			b.WriteString("#### Error Body\n\n```text\n")
			b.WriteString(result.ErrorBody)
			b.WriteString("\n```\n\n")
		}

		if result.RawRequest != "" {
			b.WriteString("#### Raw Request\n\n```json\n")
			b.WriteString(result.RawRequest)
			b.WriteString("\n```\n\n")
		}

		if result.RawResponse != "" {
			b.WriteString("#### Raw Response\n\n```json\n")
			b.WriteString(result.RawResponse)
			b.WriteString("\n```\n\n")
		}

		if len(result.ToolCalls) > 0 {
			b.WriteString("#### Tool Calls\n\n```json\n")
			b.WriteString(mustJSON(result.ToolCalls))
			b.WriteString("\n```\n\n")
		}

		if len(result.StreamEvents) > 0 {
			b.WriteString("#### Stream Events\n\n```json\n")
			b.WriteString(mustJSON(result.StreamEvents))
			b.WriteString("\n```\n\n")
		}

		b.WriteString("#### Output Text\n\n```text\n")
		b.WriteString(result.Text)
		b.WriteString("\n```\n\n")
	}

	return os.WriteFile(path, []byte(b.String()), 0o644)
}

func WriteUserMarkdown(path string, run RunReport) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	var b strings.Builder
	b.WriteString("# 模型校验报告\n\n")

	b.WriteString("## 一句话结论\n\n")
	fmt.Fprintf(&b, "%s\n\n", humanConclusion(run))

	b.WriteString("## 风险摘要\n\n")
	for _, line := range summaryBullets(run) {
		fmt.Fprintf(&b, "- %s\n", line)
	}

	b.WriteString("\n## 为什么这么判定\n\n")
	for _, line := range evidenceBullets(run) {
		fmt.Fprintf(&b, "- %s\n", line)
	}

	b.WriteString("\n## 功能指标\n\n")
	for _, line := range categoryBullets(run, run.Categories.Functional) {
		fmt.Fprintf(&b, "- %s\n", line)
	}
	fmt.Fprintf(&b, "- Prompt 缓存支持：%s\n", promptCacheLine(run))

	b.WriteString("\n## 智能指标\n\n")
	for _, line := range categoryBullets(run, run.Categories.Intelligence) {
		fmt.Fprintf(&b, "- %s\n", line)
	}
	fmt.Fprintf(&b, "- 思考支持：%s\n", thinkingSupportLine(run))

	b.WriteString("\n## 网络指标\n\n")
	for _, line := range networkBullets(run) {
		fmt.Fprintf(&b, "- %s\n", line)
	}

	b.WriteString("\n## 指标总览\n\n")
	for _, line := range metricOverviewBullets(run) {
		fmt.Fprintf(&b, "- %s\n", line)
	}

	b.WriteString("\n## 建议动作\n\n")
	for _, line := range actionBullets(run) {
		fmt.Fprintf(&b, "- %s\n", line)
	}

	b.WriteString("\n## 元数据\n\n```yaml\n")
	fmt.Fprintf(&b, "run_at: %s\n", run.RunAt.Format("2006-01-02T15:04:05Z07:00"))
	fmt.Fprintf(&b, "provider: %s\n", run.Config.Provider)
	fmt.Fprintf(&b, "model: %s\n", run.Config.Model)
	fmt.Fprintf(&b, "base_url: %s\n", run.Config.BaseURL)
	fmt.Fprintf(&b, "archive_path: %s\n", run.Config.OutputPath)
	fmt.Fprintf(&b, "total_input_tokens: %d\n", run.TokenUsage.InputTokens)
	fmt.Fprintf(&b, "total_output_tokens: %d\n", run.TokenUsage.OutputTokens)
	fmt.Fprintf(&b, "total_tokens: %d\n", run.TokenUsage.TotalTokens)
	b.WriteString("```\n\n")

	b.WriteString("## 原始分数\n\n```yaml\n")
	fmt.Fprintf(&b, "protocol_conformity_score: %d\n", run.Scores.ProtocolConformityScore)
	fmt.Fprintf(&b, "stream_conformity_score: %d\n", run.Scores.StreamConformityScore)
	fmt.Fprintf(&b, "usage_consistency_score: %d\n", run.Scores.UsageConsistencyScore)
	fmt.Fprintf(&b, "behavior_fingerprint_score: %d\n", run.Scores.BehaviorFingerprintScore)
	fmt.Fprintf(&b, "capability_tool_score: %d\n", run.Scores.CapabilityToolScore)
	fmt.Fprintf(&b, "tier_fidelity_score: %d\n", run.Scores.TierFidelityScore)
	fmt.Fprintf(&b, "route_integrity_score: %d\n", run.Scores.RouteIntegrityScore)
	fmt.Fprintf(&b, "functional_score: %d\n", run.Categories.Functional.Score)
	fmt.Fprintf(&b, "intelligence_score: %d\n", run.Categories.Intelligence.Score)
	fmt.Fprintf(&b, "overall_risk_score: %d\n", run.Scores.OverallRiskScore)
	b.WriteString("```\n\n")

	b.WriteString("## 指标状态\n\n")
	fmt.Fprintf(&b, "- %s：`%d/100`，%s\n", run.Categories.Functional.Name, run.Categories.Functional.Score, judge.FormatStatus(run.Categories.Functional.Status))
	for _, item := range run.Categories.Functional.Items {
		fmt.Fprintf(&b, "- %s：`%d/100`，%s\n", item.Name, item.Score, judge.FormatStatus(item.Status))
	}
	fmt.Fprintf(&b, "- %s：`%d/100`，%s\n", run.Categories.Intelligence.Name, run.Categories.Intelligence.Score, judge.FormatStatus(run.Categories.Intelligence.Status))
	for _, item := range run.Categories.Intelligence.Items {
		fmt.Fprintf(&b, "- %s：`%d/100`，%s\n", item.Name, item.Score, judge.FormatStatus(item.Status))
	}
	fmt.Fprintf(&b, "- 思考支持：%s\n", thinkingSupportLine(run))
	fmt.Fprintf(&b, "- Prompt 缓存支持：%s\n", promptCacheLine(run))

	return os.WriteFile(path, []byte(b.String()), 0o644)
}

func humanConclusion(run RunReport) string {
	switch run.Judgement.Conclusion {
	case "high_confidence_official_compatible":
		return "当前 endpoint 整体表现接近官方兼容服务，暂未看到明显的中转改写或能力降级。"
	case "suspected_route_or_protocol_mismatch":
		return "当前 endpoint 存在较明显的协议或路径异常，更像是中转兼容层，而不是稳定的官方直连形态。"
	case "usage_token_anomaly":
		return "当前 endpoint 的 usage token 返回存在异常，token 统计口径需要重点核查。"
	case "suspected_same_brand_downgrade":
		return "当前 endpoint 可能仍属于同品牌模型，但有较强的低配或降级嫌疑。"
	case "suspected_wrapper_or_hidden_prompt":
		return "当前 endpoint 有较强的包装层或隐藏 prompt 嫌疑，用户拿到的可能不是原始官方模型行为。"
	case "compatibility_with_wrapper_risk":
		return "当前 endpoint 基本兼容，但存在一定包装层干预风险。"
	case "suspected_identity_rewrite_layer":
		return "当前 endpoint 有较强的身份改写层嫌疑，自报身份在不同问法或同一交互内不够稳定。"
	case "identity_claim_inconsistency":
		return "当前 endpoint 的自报身份与预期模型画像不完全一致，建议继续复测。"
	default:
		return "当前 endpoint 完成了基础校验，但仍需结合关键观测值进一步判断。"
	}
}

func summaryBullets(run RunReport) []string {
	lines := []string{
		fmt.Sprintf("总风险分 `%d/100`。%s", run.Scores.OverallRiskScore, riskLevelText(run.Scores.OverallRiskScore)),
		fmt.Sprintf("功能指标总分 `%d/100`，智能指标总分 `%d/100`。", run.Categories.Functional.Score, run.Categories.Intelligence.Score),
		fmt.Sprintf("功能侧看协议一致性 `%d`、流式一致性 `%d`、Usage 一致性 `%d`、路径完整性 `%d`。", run.Scores.ProtocolConformityScore, run.Scores.StreamConformityScore, run.Scores.UsageConsistencyScore, run.Scores.RouteIntegrityScore),
		fmt.Sprintf("智能侧看行为指纹 `%d`、工具/函数能力 `%d`、档位保真 `%d`。", run.Scores.BehaviorFingerprintScore, run.Scores.CapabilityToolScore, run.Scores.TierFidelityScore),
		fmt.Sprintf("思考支持判断：%s", thinkingSupportLine(run)),
		fmt.Sprintf("Prompt 缓存支持判断：%s", promptCacheLine(run)),
		fmt.Sprintf("本次鉴定累计消耗 token：输入 `%d`，输出 `%d`，合计 `%d`。", run.TokenUsage.InputTokens, run.TokenUsage.OutputTokens, run.TokenUsage.TotalTokens),
	}
	if len(run.Scores.HardAnomalies) > 0 {
		lines = append(lines, fmt.Sprintf("存在 `%d` 个硬异常，说明至少有部分行为已经偏离预期接口形态。", len(run.Scores.HardAnomalies)))
	}
	return uniq(lines)
}

func evidenceBullets(run RunReport) []string {
	var lines []string
	lines = append(lines, run.Judgement.Summaries...)
	if v, ok := run.Scores.Observations["wrapper_cleanliness_score"]; ok {
		lines = append(lines, fmt.Sprintf("包装层洁净度为 `%s`，越接近 1 越像未被额外包装。", formatObservation("wrapper_cleanliness_score", v)))
	}
	if v, ok := run.Scores.Observations["identity_self_report_consistency"]; ok {
		lines = append(lines, fmt.Sprintf("低频语言身份自报一致性为 `%s`，越低越可疑。", formatObservation("identity_self_report_consistency", v)))
	}
	if v, ok := run.Scores.Observations["identity_multiturn_consistency"]; ok {
		lines = append(lines, fmt.Sprintf("同一交互内身份稳定性为 `%s`，越低越说明身份容易被改写。", formatObservation("identity_multiturn_consistency", v)))
	}
	if v, ok := run.Scores.Observations["usage_avg_deviation_ratio"]; ok {
		lines = append(lines, fmt.Sprintf("Usage 偏差率为 `%s`，越高越说明返回的 token 口径与本地估算差距越大。", formatObservation("usage_avg_deviation_ratio", v)))
	}
	if v, ok := run.Scores.Observations["stream_first_event_latency_ms"]; ok {
		lines = append(lines, fmt.Sprintf("平均首事件延迟为 `%s ms`。", formatObservation("stream_first_event_latency_ms", v)))
	}
	if v, ok := run.Scores.Observations["capability_tool_call_hit_ratio"]; ok {
		lines = append(lines, fmt.Sprintf("工具调用命中率为 `%s`。", formatObservation("capability_tool_call_hit_ratio", v)))
	}
	if v, ok := run.Scores.Observations["capability_tool_argument_match"]; ok {
		lines = append(lines, fmt.Sprintf("工具参数匹配度为 `%s`。", formatObservation("capability_tool_argument_match", v)))
	}
	if v, ok := run.Scores.Observations["capability_tool_followup_match"]; ok {
		lines = append(lines, fmt.Sprintf("工具结果后续回答匹配度为 `%s`。这项指标当前主要在看模型拿到 tool result 后，能否继续产出正确最终答案；若值异常，请优先查看 run archive 中是否记录了 follow-up request/response/error，以区分 endpoint 链路问题与 follow-up 交互兼容性问题。", formatObservation("capability_tool_followup_match", v)))
	}
	if v, ok := run.Scores.Observations["tier-longcontext-multihop_pass_ratio"]; ok {
		lines = append(lines, fmt.Sprintf("长上下文多跳定位通过率为 `%s`。", formatObservation("tier-longcontext-multihop_pass_ratio", v)))
	}
	if v, ok := run.Scores.Observations["reasoning_activation_gain"]; ok {
		lines = append(lines, fmt.Sprintf("reasoning 启用增益为 `%s`，正值说明 reasoning-on 用例整体优于 reasoning-off。", formatObservation("reasoning_activation_gain", v)))
	}
	lines = append(lines, run.Thinking.Summary)
	lines = append(lines, run.PromptCache.Summary)
	return uniq(lines)
}

func metricOverviewBullets(run RunReport) []string {
	return []string{
		fmt.Sprintf("功能指标总分：`%d/100`，%s", run.Categories.Functional.Score, metricMeaning(run.Categories.Functional.Score)),
		fmt.Sprintf("智能指标总分：`%d/100`，%s", run.Categories.Intelligence.Score, metricMeaning(run.Categories.Intelligence.Score)),
		fmt.Sprintf("协议一致性：`%d/100`，%s", run.Scores.ProtocolConformityScore, metricMeaning(run.Scores.ProtocolConformityScore)),
		fmt.Sprintf("流式一致性：`%d/100`，%s", run.Scores.StreamConformityScore, metricMeaning(run.Scores.StreamConformityScore)),
		fmt.Sprintf("Usage 一致性：`%d/100`，%s", run.Scores.UsageConsistencyScore, metricMeaning(run.Scores.UsageConsistencyScore)),
		fmt.Sprintf("行为指纹：`%d/100`，%s", run.Scores.BehaviorFingerprintScore, metricMeaning(run.Scores.BehaviorFingerprintScore)),
		fmt.Sprintf("工具/函数能力：`%d/100`，%s", run.Scores.CapabilityToolScore, metricMeaning(run.Scores.CapabilityToolScore)),
		fmt.Sprintf("档位保真：`%d/100`，%s", run.Scores.TierFidelityScore, metricMeaning(run.Scores.TierFidelityScore)),
		fmt.Sprintf("路径完整性：`%d/100`，%s", run.Scores.RouteIntegrityScore, metricMeaning(run.Scores.RouteIntegrityScore)),
		fmt.Sprintf("思考支持：%s", thinkingSupportLine(run)),
		fmt.Sprintf("Prompt 缓存支持：%s", promptCacheLine(run)),
	}
}

func networkBullets(run RunReport) []string {
	lines := []string{
		fmt.Sprintf("平均总延时：`%d ms`。", run.Network.AvgLatencyMs),
		fmt.Sprintf("P95 总延时：`%d ms`。", run.Network.P95LatencyMs),
		fmt.Sprintf("平均首包时间：`%d ms`。", run.Network.AvgFirstByteMs),
		fmt.Sprintf("平均输出吞吐：`%.2f tokens/s`。", run.Network.AvgOutputTokensPerS),
		fmt.Sprintf("平均流式事件数：`%.2f`。", run.Network.AvgStreamEvents),
		fmt.Sprintf("超时次数：`%d`。", run.Network.TimeoutCount),
		fmt.Sprintf("成功探测数：`%d`。", run.Network.SuccessfulProbeCount),
	}
	return uniq(lines)
}

func actionBullets(run RunReport) []string {
	var lines []string
	if run.Scores.ProtocolConformityScore < 85 || run.Scores.StreamConformityScore < 85 {
		lines = append(lines, "优先复测协议与流式行为，确认是否确实存在中转兼容层。")
	}
	if run.Scores.UsageConsistencyScore < 85 {
		lines = append(lines, "重点核查 usage token 口径，不建议直接信任当前 endpoint 的 token 返回。")
	}
	if run.Scores.TierFidelityScore < 80 {
		lines = append(lines, "补做更高难度的长上下文、结构化输出和代码任务，确认是否存在同品牌降级。")
	}
	if run.Scores.CapabilityToolScore < 80 {
		lines = append(lines, "补做 tools / function call 相关复测，并区分确认是工具调用执行问题，还是 tool result 回填后的续答链路问题。")
	}
	if v, ok := run.Scores.Observations["wrapper_cleanliness_score"]; ok && v < 0.75 {
		lines = append(lines, "建议增加包装层探针复测，重点看是否存在隐藏 prompt、固定免责声明或品牌注入。")
	}
	if len(lines) == 0 {
		lines = append(lines, "当前没有明显高风险项，可以继续做周期性复测，观察长期漂移。")
	}
	return uniq(lines)
}

func riskLevelText(score int) string {
	switch {
	case score >= 70:
		return "风险较高。"
	case score >= 40:
		return "存在中等风险。"
	default:
		return "整体风险较低。"
	}
}

func metricMeaning(score int) string {
	switch {
	case score >= 90:
		return "表现稳定，接近预期。"
	case score >= 75:
		return "有一定偏差，但还不算严重。"
	case score >= 60:
		return "偏差已经比较明显。"
	default:
		return "存在较强异常信号。"
	}
}

func formatObservation(key string, value float64) string {
	switch {
	case strings.HasSuffix(key, "_ms"):
		return strconv.FormatFloat(value, 'f', 0, 64)
	case strings.Contains(key, "ratio") || strings.Contains(key, "score") || strings.Contains(key, "consistency") || strings.Contains(key, "match") || strings.Contains(key, "cleanliness") || strings.Contains(key, "monotonicity") || strings.Contains(key, "stability"):
		return strconv.FormatFloat(value, 'f', 3, 64)
	default:
		return strconv.FormatFloat(value, 'f', 4, 64)
	}
}

func mustJSON(v any) string {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "{}"
	}
	return string(data)
}

func quoteYAML(s string) string {
	if s == "" {
		return `""`
	}
	return strconv.Quote(s)
}

func uniq(items []string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, item := range items {
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	return out
}

func categoryBullets(run RunReport, category MetricCategory) []string {
	lines := []string{
		fmt.Sprintf("%s总分：`%d/100`，%s。", category.Name, category.Score, metricMeaning(category.Score)),
	}
	for _, item := range category.Items {
		lines = append(lines, fmt.Sprintf("%s：`%d/100`，%s。", item.Name, item.Score, metricMeaning(item.Score)))
		if item.Key == "capability_tool_score" {
			lines = append(lines, capabilityBreakdownLine(run))
		}
	}
	return lines
}

func thinkingSupportLine(run RunReport) string {
	switch run.Thinking.Status {
	case "supported_active":
		if strings.EqualFold(run.Config.Provider, "anthropic") {
			return "已检测到，本次探测出现了活跃的 thinking 信号。"
		}
		return "已检测到，本次探测出现了活跃的 reasoning 信号。"
	case "supported_exposed":
		if strings.EqualFold(run.Config.Provider, "anthropic") {
			return "检测到 thinking 字段，但当前用例未稳定观察到实际 thinking 内容，暂不能判断是否可稳定使用。"
		}
		return "检测到 reasoning 字段，但当前用例未主动启用，暂不能判断是否可稳定使用。"
	default:
		return "本次未检测到明确信号。"
	}
}

func promptCacheLine(run RunReport) string {
	switch run.PromptCache.Status {
	case "supported_hit":
		return fmt.Sprintf("已检测到缓存命中，最高 cached tokens 为 `%d`。", run.PromptCache.CachedTokens)
	case "supported_exposed":
		return "已检测到 prompt cache 相关字段，但本次未观察到实际缓存命中。"
	case "not_applicable":
		return "当前 provider 尚未接入该专项探测，本次不下结论。"
	default:
		return "本次未检测到明确信号。"
	}
}

func capabilityBreakdownLine(run RunReport) string {
	parts := []string{"这项总分包含两层含义：工具调用执行，以及拿到 tool result 后的最终续答。"}
	if v, ok := run.Scores.Observations["capability_tool_call_hit_ratio"]; ok {
		parts = append(parts, fmt.Sprintf("工具调用命中率 `%s`", formatObservation("capability_tool_call_hit_ratio", v)))
	}
	if v, ok := run.Scores.Observations["capability_tool_argument_match"]; ok {
		parts = append(parts, fmt.Sprintf("参数匹配度 `%s`", formatObservation("capability_tool_argument_match", v)))
	}
	if v, ok := run.Scores.Observations["capability_tool_followup_match"]; ok {
		parts = append(parts, fmt.Sprintf("结果续答匹配度 `%s`", formatObservation("capability_tool_followup_match", v)))
	}
	return strings.Join(parts, "，") + "。"
}
