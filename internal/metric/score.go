package metric

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"github.com/ISADBA/checkllm/internal/baseline"
	"github.com/ISADBA/checkllm/internal/probe"
)

type Input struct {
	Provider      string
	Model         string
	ProbeResults  []probe.Result
	Baseline      baseline.Baseline
	EnableStream  bool
	ExpectedUsage bool
}

type Scores struct {
	ProtocolConformityScore  int
	StreamConformityScore    int
	UsageConsistencyScore    int
	BehaviorFingerprintScore int
	CapabilityToolScore      int
	TierFidelityScore        int
	RouteIntegrityScore      int
	OverallRiskScore         int
	HardAnomalies            []string
	Observations             map[string]float64
}

func Calculate(input Input) Scores {
	scores := Scores{
		ProtocolConformityScore:  100,
		StreamConformityScore:    100,
		UsageConsistencyScore:    100,
		BehaviorFingerprintScore: 100,
		CapabilityToolScore:      100,
		TierFidelityScore:        100,
		RouteIntegrityScore:      100,
		Observations:             map[string]float64{},
	}

	grouped := groupByName(input.ProbeResults)
	usageRatios := collectUsageRatios(input.ProbeResults)
	scores.ProtocolConformityScore = calculateProtocolScore(input, grouped, &scores)
	scores.StreamConformityScore = calculateStreamScore(input, grouped, &scores)
	scores.UsageConsistencyScore = calculateUsageScore(input, grouped, usageRatios, &scores)
	scores.BehaviorFingerprintScore = calculateFingerprintScore(input, grouped, &scores)
	scores.CapabilityToolScore = calculateCapabilityScore(grouped, &scores)
	scores.TierFidelityScore = calculateTierScore(grouped, &scores)
	scores.RouteIntegrityScore = calculateRouteScore(grouped, scores.StreamConformityScore, scores.ProtocolConformityScore, &scores)

	good := []int{
		scores.ProtocolConformityScore,
		scores.StreamConformityScore,
		scores.UsageConsistencyScore,
		scores.BehaviorFingerprintScore,
		scores.CapabilityToolScore,
		scores.TierFidelityScore,
		scores.RouteIntegrityScore,
	}
	scores.OverallRiskScore = clampInt(100-averageInt(good)-len(scores.HardAnomalies)*10, 0, 100)
	if len(scores.HardAnomalies) > 0 {
		scores.OverallRiskScore = clampInt(100-averageInt(good)+len(scores.HardAnomalies)*12, 0, 100)
	}
	return scores
}

func calculateProtocolScore(input Input, grouped map[string][]probe.Result, scores *Scores) int {
	var checks []float64
	for _, results := range grouped {
		for _, res := range results {
			if res.Definition.Kind != probe.KindProtocol {
				continue
			}
			if res.Err != nil || res.StatusCode >= 400 {
				scores.HardAnomalies = append(scores.HardAnomalies, res.Definition.Name+" failed")
				checks = append(checks, 0)
				continue
			}
			score := 1.0
			if input.ExpectedUsage && res.Definition.ExpectUsage && !res.UsageReturned {
				scores.HardAnomalies = append(scores.HardAnomalies, res.Definition.Name+" missing usage")
				score *= 0.4
			}
			if res.Definition.ExpectedPhrase != "" && !strings.Contains(strings.ToLower(res.Text), strings.ToLower(res.Definition.ExpectedPhrase)) {
				score *= 0.6
			}
			if containsForbiddenSubstring(res.Text, res.Definition.ForbiddenSubstrings) {
				score *= 0.4
			}
			if res.Definition.ExpectJSON {
				if !looksLikeJSON(res.Text) {
					score *= 0.3
				} else if !validateJSONExpectations(res.Text, res.Definition) {
					score *= 0.6
				}
			}
			checks = append(checks, score)
		}
	}
	return ratioScore(checks)
}

func calculateStreamScore(input Input, grouped map[string][]probe.Result, scores *Scores) int {
	if !input.EnableStream {
		return 100
	}
	var checks []float64
	var eventCounts []float64
	var firstEventLatencies []float64
	var typeCoverage []float64
	for _, results := range grouped {
		for _, res := range results {
			if !res.Definition.Stream {
				continue
			}
			if res.Err != nil || res.StatusCode >= 400 {
				checks = append(checks, 0)
				continue
			}
			score := 1.0
			eventCounts = append(eventCounts, float64(len(res.StreamEvents)))
			if len(res.StreamEvents) == 0 {
				score = 0
			} else {
				if res.Definition.MinStreamEvents > 0 && len(res.StreamEvents) < res.Definition.MinStreamEvents {
					score *= 0.5
				}
				if !hasStreamDoneEvent(res.StreamEvents) {
					score *= 0.7
				}
				if coverage, ok := streamTypeCoverage(res.StreamEvents); ok {
					typeCoverage = append(typeCoverage, coverage)
					score *= clampUnit(coverage + 0.2)
				}
				if res.Definition.ExpectUsage && !res.UsageReturned {
					score *= 0.7
				}
				if res.FirstEventLatency > 0 {
					firstEventLatencies = append(firstEventLatencies, float64(res.FirstEventLatency.Milliseconds()))
				}
			}
			checks = append(checks, score)
		}
	}
	if len(eventCounts) > 0 {
		scores.Observations["stream_avg_events"] = average(eventCounts)
	}
	if len(firstEventLatencies) > 0 {
		scores.Observations["stream_first_event_latency_ms"] = average(firstEventLatencies)
	}
	if len(typeCoverage) > 0 {
		scores.Observations["stream_type_coverage_ratio"] = average(typeCoverage)
	}
	return ratioScore(checks)
}

func calculateUsageScore(input Input, grouped map[string][]probe.Result, usageRatios []float64, scores *Scores) int {
	var checks []float64
	for _, results := range grouped {
		for _, res := range results {
			if res.Definition.Kind != probe.KindUsage {
				continue
			}
			if res.Err != nil || res.StatusCode >= 400 {
				checks = append(checks, 0)
				continue
			}
			score := 1.0
			if input.ExpectedUsage && !res.UsageReturned {
				score *= 0.3
			}
			if res.Usage.TotalTokens <= 0 || res.Usage.InputTokens <= 0 || res.Usage.OutputTokens < 0 {
				score *= 0.4
			}
			checks = append(checks, score)
		}
	}
	if len(usageRatios) > 0 {
		avgRatio := average(usageRatios)
		scores.Observations["usage_avg_deviation_ratio"] = avgRatio
		checks = append(checks, clampUnit(1.0-avgRatio*2.5))
	}
	inputMonotonic, totalMonotonic := usageMonotonicity(grouped)
	scores.Observations["usage_input_monotonicity"] = inputMonotonic
	scores.Observations["usage_total_monotonicity"] = totalMonotonic
	checks = append(checks, inputMonotonic, totalMonotonic)
	if variance, ok := repeatedUsageVariance(grouped["usage-short"]); ok {
		scores.Observations["usage_short_variance_ratio"] = variance
		checks = append(checks, clampUnit(1.0-variance*4))
	}
	return ratioScore(checks)
}

func calculateFingerprintScore(input Input, grouped map[string][]probe.Result, scores *Scores) int {
	var checks []float64
	var identityReports []identityReport
	var multiTurnChecks []float64
	for name, results := range grouped {
		if len(results) == 0 || results[0].Definition.Kind != probe.KindFingerprint {
			continue
		}
		var success []float64
		var normalized []string
		for _, res := range results {
			if res.Err != nil || res.StatusCode >= 400 {
				success = append(success, 0)
				continue
			}
			score := 1.0
			if res.Definition.ExpectJSON {
				if !looksLikeJSON(res.Text) {
					score *= 0.2
				} else if !validateJSONExpectations(res.Text, res.Definition) {
					score *= 0.5
				}
				if strings.HasPrefix(name, "identity-self-report") {
					if report, ok := parseIdentityReport(res.Text); ok {
						identityReports = append(identityReports, report)
					} else {
						score *= 0.5
					}
				}
				if name == "identity-multiturn-esperanto" {
					if consistency, ok := parseMultiTurnIdentityConsistency(res.Text, []string{"first_vendor", "first_family", "first_model"}, []string{"second_vendor", "second_family", "second_model"}); ok {
						multiTurnChecks = append(multiTurnChecks, consistency)
						score *= clampUnit(0.4 + 0.6*consistency)
					} else {
						score *= 0.4
					}
				}
				if name == "identity-resistance-latin" {
					if resistance, ok := parseMultiTurnIdentityConsistency(res.Text, []string{"initial_vendor", "initial_family", "initial_model"}, []string{"final_vendor", "final_family", "final_model"}); ok {
						multiTurnChecks = append(multiTurnChecks, resistance)
						score *= clampUnit(0.4 + 0.6*resistance)
					} else {
						score *= 0.4
					}
				}
			}
			if res.Definition.ExpectedPhrase != "" && !strings.Contains(strings.ToLower(res.Text), strings.ToLower(res.Definition.ExpectedPhrase)) {
				score *= 0.6
			}
			if len(res.Definition.ExpectedLineSequence) > 0 && !validateLineSequence(res.Text, res.Definition.ExpectedLineSequence) {
				score *= 0.5
			}
			if containsForbiddenSubstring(res.Text, res.Definition.ForbiddenSubstrings) {
				score *= 0.35
			}
			success = append(success, score)
			normalized = append(normalized, normalizeText(res.Text))
		}
		passRatio := average(success)
		scores.Observations[name+"_pass_ratio"] = passRatio
		checks = append(checks, passRatio)
		if len(normalized) > 1 {
			stability := repetitionStability(normalized)
			scores.Observations[name+"_stability"] = stability
			checks = append(checks, stability)
		}
	}
	if wrapperScore, ok := wrapperSuspicionSignal(scores.Observations); ok {
		scores.Observations["wrapper_cleanliness_score"] = wrapperScore
	}
	if len(identityReports) > 0 {
		identityVendorMatch, identityFamilyMatch, identityConsistency := summarizeIdentityReports(input.Provider, input.Model, identityReports)
		scores.Observations["identity_self_report_vendor_match"] = identityVendorMatch
		scores.Observations["identity_self_report_family_match"] = identityFamilyMatch
		scores.Observations["identity_self_report_consistency"] = identityConsistency
		checks = append(checks, 0.3+identityVendorMatch*0.35+identityFamilyMatch*0.35)
		checks = append(checks, identityConsistency)
	}
	if len(multiTurnChecks) > 0 {
		scores.Observations["identity_multiturn_consistency"] = average(multiTurnChecks)
		checks = append(checks, average(multiTurnChecks))
	}
	return ratioScore(checks)
}

func calculateTierScore(grouped map[string][]probe.Result, scores *Scores) int {
	var checks []float64
	var reasoningOffPass []float64
	var reasoningOnPass []float64
	for name, results := range grouped {
		if len(results) == 0 || results[0].Definition.Kind != probe.KindTier {
			continue
		}
		var success []float64
		var normalized []string
		for _, res := range results {
			if res.Err != nil || res.StatusCode >= 400 {
				success = append(success, 0)
				continue
			}
			score := 1.0
			switch name {
			case "tier-multi-constraint":
				score = scoreTierMultiConstraint(res.Text)
			case "tier-context-locate":
				if looksLikeJSON(res.Text) && validateJSONExpectations(res.Text, res.Definition) {
					score = 1.0
				} else {
					score = 0.2
				}
			case "tier-instruction-hard":
				score = scoreTierInstructionHard(res.Text, res.Definition)
			case "tier-negative-constraint":
				score = scoreTierNegativeConstraint(res.Text, res.Definition)
			case "tier-longcontext-multihop", "tier-reasoning-off", "tier-reasoning-on":
				if looksLikeJSON(res.Text) && validateJSONExpectations(res.Text, res.Definition) {
					score = 1.0
				} else {
					score = 0.2
				}
			}
			success = append(success, score)
			normalized = append(normalized, normalizeText(res.Text))
		}
		passRatio := average(success)
		checks = append(checks, passRatio)
		scores.Observations[name+"_pass_ratio"] = passRatio
		switch name {
		case "tier-reasoning-off":
			reasoningOffPass = append(reasoningOffPass, passRatio)
		case "tier-reasoning-on":
			reasoningOnPass = append(reasoningOnPass, passRatio)
		}
		if len(normalized) > 1 {
			stability := repetitionStability(normalized)
			scores.Observations[name+"_stability"] = stability
			checks = append(checks, stability)
		}
	}
	if len(reasoningOffPass) > 0 && len(reasoningOnPass) > 0 {
		offAvg := average(reasoningOffPass)
		onAvg := average(reasoningOnPass)
		scores.Observations["reasoning_activation_gain"] = onAvg - offAvg
		checks = append(checks, clampUnit(0.5+(onAvg-offAvg)))
	}
	return ratioScore(checks)
}

func calculateCapabilityScore(grouped map[string][]probe.Result, scores *Scores) int {
	var checks []float64
	var toolCallHits []float64
	var toolArgMatches []float64
	var finalAnswerMatches []float64
	for name, results := range grouped {
		if len(results) == 0 || results[0].Definition.Kind != probe.KindCapability {
			continue
		}
		var success []float64
		for _, res := range results {
			if res.Err != nil || res.StatusCode >= 400 {
				success = append(success, 0)
				continue
			}
			score := 1.0
			if res.Definition.ExpectToolCall {
				if len(res.ToolCalls) == 0 {
					score = 0
					toolCallHits = append(toolCallHits, 0)
				} else {
					toolCallHits = append(toolCallHits, 1)
					best := bestMatchingToolCall(res.ToolCalls, res.Definition.ExpectedToolName, res.Definition.ExpectedToolArgs)
					score *= best.Score
					toolArgMatches = append(toolArgMatches, best.ArgMatch)
				}
			}
			if res.Definition.ExpectFinalText {
				match := finalTextMatch(res.Text, res.Definition.ExpectedFinalPhrases)
				finalAnswerMatches = append(finalAnswerMatches, match)
				score *= clampUnit(0.4 + 0.6*match)
			}
			if res.Definition.ExpectUsage && !res.UsageReturned {
				score *= 0.7
			}
			success = append(success, score)
		}
		passRatio := average(success)
		scores.Observations[name+"_pass_ratio"] = passRatio
		checks = append(checks, passRatio)
	}
	if len(toolCallHits) > 0 {
		scores.Observations["capability_tool_call_hit_ratio"] = average(toolCallHits)
		checks = append(checks, average(toolCallHits))
	}
	if len(toolArgMatches) > 0 {
		scores.Observations["capability_tool_argument_match"] = average(toolArgMatches)
		checks = append(checks, average(toolArgMatches))
	}
	if len(finalAnswerMatches) > 0 {
		scores.Observations["capability_tool_followup_match"] = average(finalAnswerMatches)
		checks = append(checks, average(finalAnswerMatches))
	}
	return ratioScore(checks)
}

func calculateRouteScore(grouped map[string][]probe.Result, streamScore, protocolScore int, scores *Scores) int {
	var latencyRatios []float64
	for _, results := range grouped {
		if len(results) < 2 {
			continue
		}
		if ratio, ok := latencyStability(results); ok {
			latencyRatios = append(latencyRatios, ratio)
		}
	}
	routeChecks := []float64{
		float64(streamScore) / 100.0,
		float64(protocolScore) / 100.0,
	}
	if len(latencyRatios) > 0 {
		avgRatio := average(latencyRatios)
		scores.Observations["latency_instability_ratio"] = avgRatio
		routeChecks = append(routeChecks, clampUnit(1.0-avgRatio))
	}
	return ratioScore(routeChecks)
}

func estimateTokens(text string) int {
	fields := strings.Fields(text)
	if len(fields) == 0 {
		return 0
	}
	return int(math.Ceil(float64(len(fields)) * 1.3))
}

func collectUsageRatios(results []probe.Result) []float64 {
	var ratios []float64
	for _, res := range results {
		if res.Err != nil || res.StatusCode >= 400 {
			continue
		}
		estimated := estimateTokens(res.Definition.Prompt + " " + res.Text)
		if estimated > 0 && res.Usage.TotalTokens > 0 {
			ratio := math.Abs(float64(res.Usage.TotalTokens-estimated)) / float64(estimated)
			ratios = append(ratios, ratio)
		}
	}
	return ratios
}

func groupByName(results []probe.Result) map[string][]probe.Result {
	grouped := make(map[string][]probe.Result)
	for _, res := range results {
		grouped[res.Definition.Name] = append(grouped[res.Definition.Name], res)
	}
	return grouped
}

func looksLikeJSON(text string) bool {
	var dst any
	return json.Unmarshal([]byte(strings.TrimSpace(text)), &dst) == nil
}

func validateJSONExpectations(text string, def probe.Definition) bool {
	var dst map[string]any
	if json.Unmarshal([]byte(strings.TrimSpace(text)), &dst) != nil {
		return false
	}
	for _, key := range def.ExpectedJSONKeys {
		if _, ok := dst[key]; !ok {
			return false
		}
	}
	for key, expected := range def.ExpectedJSONValues {
		value, ok := dst[key]
		if !ok {
			return false
		}
		if strings.TrimSpace(strings.ToLower(toString(value))) != strings.TrimSpace(strings.ToLower(expected)) {
			return false
		}
	}
	return true
}

func validateLineSequence(text string, expected []string) bool {
	lines := strings.Split(strings.TrimSpace(text), "\n")
	if len(lines) != len(expected) {
		return false
	}
	for i := range expected {
		if strings.TrimSpace(lines[i]) != expected[i] {
			return false
		}
	}
	return true
}

func scoreTierMultiConstraint(text string) float64 {
	var dst map[string]string
	if json.Unmarshal([]byte(strings.TrimSpace(text)), &dst) != nil {
		return 0.2
	}
	keys := []string{"alpha", "beta", "gamma"}
	seen := map[string]bool{}
	score := 1.0
	for _, key := range keys {
		value, ok := dst[key]
		if !ok {
			return 0.2
		}
		if wordCount(value) != 5 {
			score *= 0.65
		}
		normalized := normalizeText(value)
		if seen[normalized] {
			score *= 0.6
		}
		seen[normalized] = true
	}
	return clampUnit(score)
}

func scoreTierInstructionHard(text string, def probe.Definition) float64 {
	var dst map[string]string
	if json.Unmarshal([]byte(strings.TrimSpace(text)), &dst) != nil {
		return 0.2
	}
	score := 1.0
	for _, key := range def.ExpectedJSONKeys {
		if _, ok := dst[key]; !ok {
			return 0.2
		}
	}
	for key, expected := range def.ExpectedJSONValues {
		if normalizeText(dst[key]) != normalizeText(expected) {
			score *= 0.4
		}
	}
	note := strings.TrimSpace(dst["note"])
	if wordCount(note) != 6 {
		score *= 0.5
	}
	if keywordCount(note, "stable") != 1 {
		score *= 0.5
	}
	if containsForbiddenSubstring(note, def.ForbiddenSubstrings) {
		score *= 0.2
	}
	return clampUnit(score)
}

func scoreTierNegativeConstraint(text string, def probe.Definition) float64 {
	score := 1.0
	if !validateLineSequence(text, def.ExpectedLineSequence) {
		score *= 0.2
	}
	if containsForbiddenSubstring(text, def.ForbiddenSubstrings) {
		score *= 0.2
	}
	return clampUnit(score)
}

func wordCount(s string) int {
	return len(strings.Fields(strings.TrimSpace(s)))
}

func keywordCount(s, keyword string) int {
	normalized := normalizeText(s)
	if normalized == "" || keyword == "" {
		return 0
	}
	var count int
	for _, field := range strings.Fields(normalized) {
		if field == normalizeText(keyword) {
			count++
		}
	}
	return count
}

func repetitionStability(values []string) float64 {
	if len(values) == 0 {
		return 0
	}
	unique := map[string]struct{}{}
	for _, value := range values {
		unique[value] = struct{}{}
	}
	return clampUnit(1.0 - float64(len(unique)-1)/float64(len(values)))
}

func usageMonotonicity(grouped map[string][]probe.Result) (float64, float64) {
	names := []string{"usage-short", "usage-medium", "usage-long"}
	var inputValues, totalValues []float64
	for _, name := range names {
		results := grouped[name]
		if len(results) == 0 {
			continue
		}
		var inputs, totals []float64
		for _, res := range results {
			if res.Err != nil || res.StatusCode >= 400 {
				continue
			}
			inputs = append(inputs, float64(res.Usage.InputTokens))
			totals = append(totals, float64(res.Usage.TotalTokens))
		}
		if len(inputs) > 0 {
			inputValues = append(inputValues, average(inputs))
		}
		if len(totals) > 0 {
			totalValues = append(totalValues, average(totals))
		}
	}
	return monotonicRatio(inputValues), monotonicRatio(totalValues)
}

func monotonicRatio(values []float64) float64 {
	if len(values) < 2 {
		return 1
	}
	var good int
	for i := 1; i < len(values); i++ {
		if values[i] >= values[i-1] {
			good++
		}
	}
	return float64(good) / float64(len(values)-1)
}

func repeatedUsageVariance(results []probe.Result) (float64, bool) {
	if len(results) < 2 {
		return 0, false
	}
	var totals []float64
	for _, res := range results {
		if res.Err != nil || res.StatusCode >= 400 || res.Usage.TotalTokens <= 0 {
			continue
		}
		totals = append(totals, float64(res.Usage.TotalTokens))
	}
	if len(totals) < 2 {
		return 0, false
	}
	avg := average(totals)
	if avg == 0 {
		return 0, false
	}
	return standardDeviation(totals) / avg, true
}

func latencyStability(results []probe.Result) (float64, bool) {
	var latencies []float64
	for _, res := range results {
		if res.Err != nil || res.StatusCode >= 400 || res.Latency <= 0 {
			continue
		}
		latencies = append(latencies, float64(res.Latency.Milliseconds()))
	}
	if len(latencies) < 2 {
		return 0, false
	}
	avg := average(latencies)
	if avg == 0 {
		return 0, false
	}
	return standardDeviation(latencies) / avg, true
}

func streamTypeCoverage(events []probe.StreamEvent) (float64, bool) {
	if len(events) == 0 {
		return 0, false
	}
	hasResponse := false
	hasDelta := false
	hasCompleted := false
	for _, evt := range events {
		lower := strings.ToLower(evt.Type)
		if strings.HasPrefix(lower, "response.") || strings.Contains(lower, "message") || strings.Contains(lower, "output") {
			hasResponse = true
		}
		if strings.Contains(lower, "delta") || strings.Contains(lower, "added") {
			hasDelta = true
		}
		if strings.Contains(lower, "done") || strings.Contains(lower, "completed") || strings.Contains(lower, "stop") {
			hasCompleted = true
		}
	}
	var hit int
	for _, ok := range []bool{hasResponse, hasDelta, hasCompleted} {
		if ok {
			hit++
		}
	}
	return float64(hit) / 3.0, true
}

func standardDeviation(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	avg := average(values)
	var sum float64
	for _, value := range values {
		diff := value - avg
		sum += diff * diff
	}
	return math.Sqrt(sum / float64(len(values)))
}

func hasStreamDoneEvent(events []probe.StreamEvent) bool {
	for _, evt := range events {
		lower := strings.ToLower(evt.Type)
		if strings.EqualFold(evt.Type, "done") || strings.Contains(lower, "completed") || strings.Contains(lower, "message_stop") || strings.Contains(lower, "stop_reason:") {
			return true
		}
	}
	return false
}

func containsForbiddenSubstring(text string, forbidden []string) bool {
	lower := strings.ToLower(text)
	for _, item := range forbidden {
		if item != "" && strings.Contains(lower, strings.ToLower(item)) {
			return true
		}
	}
	return false
}

func wrapperSuspicionSignal(obs map[string]float64) (float64, bool) {
	keys := []string{
		"fingerprint-wrapper-clean-json_pass_ratio",
		"fingerprint-no-branding_pass_ratio",
	}
	var values []float64
	for _, key := range keys {
		if v, ok := obs[key]; ok {
			values = append(values, v)
		}
	}
	if len(values) == 0 {
		return 0, false
	}
	return average(values), true
}

func ratioScore(checks []float64) int {
	if len(checks) == 0 {
		return 0
	}
	return clampInt(int(math.Round(average(checks)*100)), 0, 100)
}

func clampUnit(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func toString(v any) string {
	switch typed := v.(type) {
	case string:
		return typed
	case float64:
		return fmt.Sprintf("%.0f", typed)
	default:
		return fmt.Sprintf("%v", v)
	}
}

func normalizeText(s string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(s))), " ")
}

type identityReport struct {
	Vendor string
	Family string
	Model  string
	Role   string
}

type toolCallMatch struct {
	Score    float64
	ArgMatch float64
}

func parseIdentityReport(text string) (identityReport, bool) {
	var dst map[string]any
	if json.Unmarshal([]byte(strings.TrimSpace(text)), &dst) != nil {
		return identityReport{}, false
	}
	report := identityReport{
		Vendor: normalizeText(toString(dst["vendor"])),
		Family: normalizeText(toString(dst["family"])),
		Model:  normalizeText(toString(dst["model"])),
		Role:   normalizeText(toString(dst["role"])),
	}
	if report.Vendor == "" && report.Family == "" && report.Model == "" {
		return identityReport{}, false
	}
	return report, true
}

func summarizeIdentityReports(provider, model string, reports []identityReport) (float64, float64, float64) {
	var vendorMatches []float64
	var familyMatches []float64
	var normalized []string
	for _, report := range reports {
		vendorMatches = append(vendorMatches, matchIdentityVendor(provider, report))
		familyMatches = append(familyMatches, matchIdentityFamily(model, report))
		normalized = append(normalized, normalizeText(report.Vendor+"|"+report.Family+"|"+report.Model+"|"+report.Role))
	}
	return average(vendorMatches), average(familyMatches), repetitionStability(normalized)
}

func parseMultiTurnIdentityConsistency(text string, leftKeys, rightKeys []string) (float64, bool) {
	var dst map[string]any
	if json.Unmarshal([]byte(strings.TrimSpace(text)), &dst) != nil {
		return 0, false
	}
	if len(leftKeys) != len(rightKeys) || len(leftKeys) == 0 {
		return 0, false
	}
	var matches float64
	for i := range leftKeys {
		left := normalizeText(toString(dst[leftKeys[i]]))
		right := normalizeText(toString(dst[rightKeys[i]]))
		if left == "" || right == "" {
			continue
		}
		if left == right {
			matches++
		} else if strings.Contains(left, "unknown") || strings.Contains(right, "unknown") {
			matches += 0.5
		}
	}
	return matches / float64(len(leftKeys)), true
}

func matchIdentityVendor(provider string, report identityReport) float64 {
	text := normalizeText(report.Vendor + " " + report.Role)
	expected := normalizeText(provider)
	if expected == "" {
		expected = "openai"
	}
	switch {
	case strings.Contains(text, expected):
		return 1
	case expected == "anthropic" && strings.Contains(text, "claude"):
		return 0.9
	case strings.Contains(text, "unknown"):
		return 0.55
	case text == "":
		return 0.35
	default:
		return 0
	}
}

func matchIdentityFamily(model string, report identityReport) float64 {
	text := normalizeText(report.Family + " " + report.Model)
	model = normalizeText(model)
	switch {
	case model != "" && strings.Contains(text, model):
		return 1
	case strings.Contains(model, "claude-opus") && strings.Contains(text, "claude opus"):
		return 0.95
	case strings.Contains(model, "claude") && strings.Contains(text, "claude"):
		return 0.85
	case strings.Contains(model, "gpt-5") && strings.Contains(text, "gpt-5"):
		return 1
	case strings.Contains(model, "gpt") && strings.Contains(text, "gpt"):
		return 0.8
	case strings.Contains(text, "claude opus"):
		return 0.8
	case strings.Contains(text, "claude"):
		return 0.65
	case strings.Contains(text, "unknown"):
		return 0.5
	case text == "":
		return 0.3
	default:
		return 0
	}
}

func bestMatchingToolCall(calls []probe.ToolCall, expectedName string, expectedArgs map[string]string) toolCallMatch {
	best := toolCallMatch{}
	for _, call := range calls {
		nameScore := 0.0
		if normalizeText(call.Name) == normalizeText(expectedName) {
			nameScore = 1.0
		} else if expectedName == "" {
			nameScore = 1.0
		}
		argMatch := argumentMatch(call.Arguments, expectedArgs)
		score := clampUnit(nameScore*0.6 + argMatch*0.4)
		if score > best.Score {
			best = toolCallMatch{Score: score, ArgMatch: argMatch}
		}
	}
	return best
}

func argumentMatch(actual map[string]any, expected map[string]string) float64 {
	if len(expected) == 0 {
		return 1
	}
	var hits float64
	for key, expectedValue := range expected {
		actualValue, ok := actual[key]
		if !ok {
			continue
		}
		if normalizeText(toString(actualValue)) == normalizeText(expectedValue) {
			hits++
		}
	}
	return hits / float64(len(expected))
}

func finalTextMatch(text string, expected []string) float64 {
	if len(expected) == 0 {
		return 1
	}
	lower := strings.ToLower(text)
	var hits float64
	for _, item := range expected {
		if item != "" && strings.Contains(lower, strings.ToLower(item)) {
			hits++
		}
	}
	return hits / float64(len(expected))
}

func average(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	var sum float64
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

func averageInt(values []int) int {
	if len(values) == 0 {
		return 0
	}
	var sum int
	for _, v := range values {
		sum += v
	}
	return sum / len(values)
}

func clampInt(v, minV, maxV int) int {
	if v < minV {
		return minV
	}
	if v > maxV {
		return maxV
	}
	return v
}
