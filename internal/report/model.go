package report

import (
	"math"
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
	PromptCache  PromptCacheSupport
	Network      NetworkMetrics
}

type TokenUsage struct {
	InputTokens  int
	OutputTokens int
	TotalTokens  int
}

type NetworkMetrics struct {
	AvgLatencyMs         int
	P95LatencyMs         int
	AvgFirstByteMs       int
	AvgOutputTokensPerS  float64
	AvgStreamEvents      float64
	TimeoutCount         int
	SuccessfulProbeCount int
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

type PromptCacheSupport struct {
	Status       string
	Summary      string
	ObservedKey  string
	Retention    string
	CachedTokens int
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
		Thinking:     detectThinkingSupport(input.Config.Provider, input.ProbeResults),
		PromptCache:  detectPromptCacheSupport(input.Config.Provider, input.ProbeResults),
		Network:      summarizeNetworkMetrics(input.ProbeResults),
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

func detectThinkingSupport(provider string, results []probe.Result) ThinkingSupport {
	sawReasoningField := false
	sawReasoningSummary := false
	sawReasoningTokens := false
	sawReasoningEffort := false
	sawThinkingField := false
	sawThinkingText := false
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
		if strings.Contains(raw, `"thinking"`) {
			sawThinkingField = true
		}
		if strings.Contains(raw, `"type":"thinking"`) || strings.Contains(raw, `"thinking":"`) || strings.Contains(raw, `"thinking_delta"`) {
			sawThinkingText = true
		}
	}
	switch {
	case sawReasoningTokens || sawReasoningSummary || sawReasoningEffort || sawThinkingText:
		return ThinkingSupport{
			Status:  "supported_active",
			Summary: thinkingActiveSummary(provider),
		}
	case sawReasoningField || sawThinkingField:
		return ThinkingSupport{
			Status:  "supported_exposed",
			Summary: thinkingExposedSummary(provider),
		}
	default:
		return ThinkingSupport{
			Status:  "not_detected",
			Summary: thinkingNotDetectedSummary(provider),
		}
	}
}

func detectPromptCacheSupport(provider string, results []probe.Result) PromptCacheSupport {
	if strings.EqualFold(provider, "anthropic") {
		return PromptCacheSupport{
			Status:  "not_applicable",
			Summary: "当前 provider 尚未接入 Anthropic prompt cache 专项探测，因此本次报告不对其支持性下结论。",
		}
	}
	sawCacheField := false
	observedKey := ""
	retention := ""
	maxCachedTokens := 0
	for _, result := range results {
		if result.PromptCacheKey != "" || result.PromptCacheRetention != "" {
			sawCacheField = true
		}
		if observedKey == "" && result.PromptCacheKey != "" {
			observedKey = result.PromptCacheKey
		}
		if retention == "" && result.PromptCacheRetention != "" {
			retention = result.PromptCacheRetention
		}
		if result.Usage.CachedTokens > maxCachedTokens {
			maxCachedTokens = result.Usage.CachedTokens
		}
	}
	switch {
	case maxCachedTokens > 0:
		return PromptCacheSupport{
			Status:       "supported_hit",
			Summary:      "已检测到 prompt cache 命中，本次探测中出现了非零 cached tokens。",
			ObservedKey:  observedKey,
			Retention:    retention,
			CachedTokens: maxCachedTokens,
		}
	case sawCacheField:
		return PromptCacheSupport{
			Status:      "supported_exposed",
			Summary:     "已检测到 prompt cache 相关字段，但本次探测未观察到实际缓存命中。",
			ObservedKey: observedKey,
			Retention:   retention,
		}
	default:
		return PromptCacheSupport{
			Status:  "not_detected",
			Summary: "本次探测未检测到明确的 prompt cache 支持信号。",
		}
	}
}

func thinkingActiveSummary(provider string) string {
	if strings.EqualFold(provider, "anthropic") {
		return "已检测到思考能力信号，本次探测中出现了 thinking 内容块、thinking delta 或启用中的 thinking 配置。"
	}
	return "已检测到思考能力信号，本次探测中出现了 reasoning 摘要、非零 reasoning token 或启用中的 reasoning 配置。"
}

func thinkingExposedSummary(provider string) string {
	if strings.EqualFold(provider, "anthropic") {
		return "已检测到 thinking 接口字段，但当前用例未稳定观察到实际 thinking 内容，因此暂时不能判断该 endpoint 是否可稳定提供思考能力。"
	}
	return "已检测到 reasoning 接口字段，但当前用例没有主动启用 reasoning，且未观察到实际思考摘要或非零 reasoning token，因此暂时不能判断该 endpoint 是否可稳定提供思考能力。"
}

func thinkingNotDetectedSummary(provider string) string {
	if strings.EqualFold(provider, "anthropic") {
		return "本次探测未检测到明确的 thinking 能力信号；这不等于模型一定不支持，也可能是当前接口未透出相关字段或探针尚未完全适配。"
	}
	return "本次探测未检测到明确的 reasoning / thinking 能力信号；这不等于模型一定不支持，也可能是当前兼容层未透出相关字段。"
}

func summarizeNetworkMetrics(results []probe.Result) NetworkMetrics {
	var latencyValues []float64
	var firstByteValues []float64
	var throughputValues []float64
	var streamEventValues []float64
	var timeoutCount int
	var successfulCount int

	for _, result := range results {
		if result.Err != nil && strings.Contains(strings.ToLower(result.Err.Error()), "deadline exceeded") {
			timeoutCount++
		}
		if result.Err != nil || result.StatusCode >= 400 {
			continue
		}
		successfulCount++
		if result.Latency > 0 {
			latencyMs := float64(result.Latency.Milliseconds())
			latencyValues = append(latencyValues, latencyMs)
			if result.Usage.OutputTokens > 0 && latencyMs > 0 {
				throughputValues = append(throughputValues, float64(result.Usage.OutputTokens)/(latencyMs/1000.0))
			}
		}
		if result.FirstEventLatency > 0 {
			firstByteValues = append(firstByteValues, float64(result.FirstEventLatency.Milliseconds()))
		}
		if len(result.StreamEvents) > 0 {
			streamEventValues = append(streamEventValues, float64(len(result.StreamEvents)))
		}
	}

	return NetworkMetrics{
		AvgLatencyMs:         int(math.Round(averageFloat(latencyValues))),
		P95LatencyMs:         int(math.Round(percentileFloat(latencyValues, 0.95))),
		AvgFirstByteMs:       int(math.Round(averageFloat(firstByteValues))),
		AvgOutputTokensPerS:  averageFloat(throughputValues),
		AvgStreamEvents:      averageFloat(streamEventValues),
		TimeoutCount:         timeoutCount,
		SuccessfulProbeCount: successfulCount,
	}
}

func averageFloat(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	var sum float64
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

func percentileFloat(values []float64, p float64) float64 {
	if len(values) == 0 {
		return 0
	}
	cp := make([]float64, len(values))
	copy(cp, values)
	sortFloat64s(cp)
	if p <= 0 {
		return cp[0]
	}
	if p >= 1 {
		return cp[len(cp)-1]
	}
	idx := int(math.Ceil(p*float64(len(cp)))) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(cp) {
		idx = len(cp) - 1
	}
	return cp[idx]
}

func sortFloat64s(values []float64) {
	for i := 0; i < len(values); i++ {
		for j := i + 1; j < len(values); j++ {
			if values[j] < values[i] {
				values[i], values[j] = values[j], values[i]
			}
		}
	}
}
