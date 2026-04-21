package report

import (
	"strings"
	"time"

	"github.com/ISADBA/checkllm/internal/baseline"
	"github.com/ISADBA/checkllm/internal/config"
	"github.com/ISADBA/checkllm/internal/judge"
	"github.com/ISADBA/checkllm/internal/metric"
	"github.com/ISADBA/checkllm/internal/probe"
)

type BuildInput struct {
	Config       config.Config
	Baseline     baseline.Baseline
	ProbeResults []probe.Result
	Scores       metric.Scores
	Judgement    judge.Interpretation
}

type RunReport struct {
	RunAt        time.Time
	Config       config.Config
	Baseline     baseline.Baseline
	ProbeResults []probe.Result
	Scores       metric.Scores
	Judgement    judge.Interpretation
	TokenUsage   TokenUsage
	Categories   MetricCategories
	Thinking     ThinkingSupport
}

type TokenUsage struct {
	InputTokens  int
	OutputTokens int
	TotalTokens  int
}

type MetricCategories struct {
	Functional   MetricCategory
	Intelligence MetricCategory
}

type MetricCategory struct {
	Name   string
	Score  int
	Items  []MetricItem
	Status string
}

type MetricItem struct {
	Name   string
	Key    string
	Score  int
	Status string
}

type ThinkingSupport struct {
	Status  string
	Summary string
}

func BuildRunReport(input BuildInput) RunReport {
	categories := buildMetricCategories(input.Scores, input.Judgement)
	return RunReport{
		RunAt:        time.Now(),
		Config:       input.Config,
		Baseline:     input.Baseline,
		ProbeResults: input.ProbeResults,
		Scores:       input.Scores,
		Judgement:    input.Judgement,
		TokenUsage:   summarizeTokenUsage(input.ProbeResults),
		Categories:   categories,
		Thinking:     detectThinkingSupport(input.ProbeResults),
	}
}

func summarizeTokenUsage(results []probe.Result) TokenUsage {
	var usage TokenUsage
	for _, result := range results {
		usage.InputTokens += result.Usage.InputTokens
		usage.OutputTokens += result.Usage.OutputTokens
		usage.TotalTokens += result.Usage.TotalTokens
	}
	return usage
}

func buildMetricCategories(scores metric.Scores, judgement judge.Interpretation) MetricCategories {
	functionalItems := []MetricItem{
		buildMetricItem("协议一致性", "protocol_conformity_score", scores.ProtocolConformityScore, judgement.Statuses),
		buildMetricItem("流式一致性", "stream_conformity_score", scores.StreamConformityScore, judgement.Statuses),
		buildMetricItem("Usage 一致性", "usage_consistency_score", scores.UsageConsistencyScore, judgement.Statuses),
		buildMetricItem("路径完整性", "route_integrity_score", scores.RouteIntegrityScore, judgement.Statuses),
	}
	intelligenceItems := []MetricItem{
		buildMetricItem("行为指纹", "behavior_fingerprint_score", scores.BehaviorFingerprintScore, judgement.Statuses),
		buildMetricItem("工具/函数能力", "capability_tool_score", scores.CapabilityToolScore, judgement.Statuses),
		buildMetricItem("档位保真", "tier_fidelity_score", scores.TierFidelityScore, judgement.Statuses),
	}
	return MetricCategories{
		Functional: MetricCategory{
			Name:   "功能指标",
			Score:  averageMetricItems(functionalItems),
			Items:  functionalItems,
			Status: categoryStatus(functionalItems),
		},
		Intelligence: MetricCategory{
			Name:   "智能指标",
			Score:  averageMetricItems(intelligenceItems),
			Items:  intelligenceItems,
			Status: categoryStatus(intelligenceItems),
		},
	}
}

func buildMetricItem(name, key string, score int, statuses map[string]string) MetricItem {
	return MetricItem{
		Name:   name,
		Key:    key,
		Score:  score,
		Status: statuses[key],
	}
}

func averageMetricItems(items []MetricItem) int {
	if len(items) == 0 {
		return 0
	}
	total := 0
	for _, item := range items {
		total += item.Score
	}
	return total / len(items)
}

func categoryStatus(items []MetricItem) string {
	status := "normal"
	for _, item := range items {
		switch item.Status {
		case "significant_deviation":
			return "significant_deviation"
		case "mild_deviation":
			status = "mild_deviation"
		}
	}
	return status
}

func detectThinkingSupport(results []probe.Result) ThinkingSupport {
	sawReasoningField := false
	sawReasoningSummary := false
	sawReasoningTokens := false
	sawReasoningEffort := false
	for _, result := range results {
		raw := strings.ToLower(result.RawResponse)
		if raw == "" {
			continue
		}
		if strings.Contains(raw, `"reasoning":`) {
			sawReasoningField = true
		}
		if strings.Contains(raw, `"reasoning":{"effort":"`) && !strings.Contains(raw, `"reasoning":{"effort":"none"`) {
			sawReasoningEffort = true
		}
		if strings.Contains(raw, `"summary":[{`) || strings.Contains(raw, `"summary":"`) || strings.Contains(raw, `"reasoning":{"effort":"`) && !strings.Contains(raw, `"summary":null`) {
			sawReasoningSummary = true
		}
		if strings.Contains(raw, `"reasoning_tokens":`) && !strings.Contains(raw, `"reasoning_tokens":0`) {
			sawReasoningTokens = true
		}
	}
	switch {
	case sawReasoningTokens || sawReasoningSummary || sawReasoningEffort:
		return ThinkingSupport{
			Status:  "supported_active",
			Summary: "已检测到思考能力信号，本次探测中出现了 reasoning 摘要、非零 reasoning token 或启用中的 reasoning 配置。",
		}
	case sawReasoningField:
		return ThinkingSupport{
			Status:  "supported_exposed",
			Summary: "已检测到 reasoning 接口字段，但当前用例没有主动启用 reasoning，且未观察到实际思考摘要或非零 reasoning token，因此暂时不能判断该 endpoint 是否可稳定提供思考能力。",
		}
	default:
		return ThinkingSupport{
			Status:  "not_detected",
			Summary: "本次探测未检测到明确的 reasoning / thinking 能力信号；这不等于模型一定不支持，也可能是当前兼容层未透出相关字段。",
		}
	}
}
