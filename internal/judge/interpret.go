package judge

import (
	"fmt"
	"sort"

	"github.com/ISADBA/checkllm/internal/baseline"
	"github.com/ISADBA/checkllm/internal/config"
	"github.com/ISADBA/checkllm/internal/history"
	"github.com/ISADBA/checkllm/internal/metric"
)

type Input struct {
	Config   config.Config
	Baseline baseline.Baseline
	Scores   metric.Scores
	History  []history.Report
}

type Interpretation struct {
	Conclusion string
	Summaries  []string
	Statuses   map[string]string
}

func Interpret(input Input) Interpretation {
	statuses := map[string]string{}
	current := map[string]float64{
		"protocol_conformity_score":  float64(input.Scores.ProtocolConformityScore),
		"stream_conformity_score":    float64(input.Scores.StreamConformityScore),
		"usage_consistency_score":    float64(input.Scores.UsageConsistencyScore),
		"behavior_fingerprint_score": float64(input.Scores.BehaviorFingerprintScore),
		"capability_tool_score":      float64(input.Scores.CapabilityToolScore),
		"tier_fidelity_score":        float64(input.Scores.TierFidelityScore),
		"route_integrity_score":      float64(input.Scores.RouteIntegrityScore),
		"overall_risk_score":         float64(input.Scores.OverallRiskScore),
	}
	var summaries []string
	wrapperScore, hasWrapperScore := input.Scores.Observations["wrapper_cleanliness_score"]
	wrapperSuspected := hasWrapperScore && wrapperScore < 0.75
	wrapperStrongSuspected := hasWrapperScore && wrapperScore < 0.5
	translationCleanliness, hasTranslationCleanliness := input.Scores.Observations["anthropic_messages_translation_cleanliness"]
	translationSuspected := hasTranslationCleanliness && translationCleanliness < 0.75
	identityVendorMatch, hasIdentityVendorMatch := input.Scores.Observations["identity_self_report_vendor_match"]
	identityFamilyMatch, hasIdentityFamilyMatch := input.Scores.Observations["identity_self_report_family_match"]
	identityConsistency, hasIdentityConsistency := input.Scores.Observations["identity_self_report_consistency"]
	multiTurnConsistency, hasMultiTurnConsistency := input.Scores.Observations["identity_multiturn_consistency"]
	identityConflict := hasIdentityConsistency && identityConsistency < 0.6
	identityMismatch := (hasIdentityVendorMatch && identityVendorMatch < 0.45) || (hasIdentityFamilyMatch && identityFamilyMatch < 0.45)
	identityRewriteSuspected := hasMultiTurnConsistency && multiTurnConsistency < 0.55
	if hasWrapperScore {
		statuses["wrapper_cleanliness_score"] = deviationStatus(wrapperScore, 0.75, nil, len(input.History) > 1)
	}
	if hasTranslationCleanliness {
		statuses["anthropic_messages_translation_cleanliness"] = deviationStatus(translationCleanliness, 0.75, nil, len(input.History) > 1)
	}
	if hasIdentityConsistency {
		statuses["identity_self_report_consistency"] = deviationStatus(identityConsistency, 0.6, nil, len(input.History) > 1)
	}
	if hasMultiTurnConsistency {
		statuses["identity_multiturn_consistency"] = deviationStatus(multiTurnConsistency, 0.55, nil, len(input.History) > 1)
	}
	for metricName, value := range current {
		var r *baseline.Range
		if rr, ok := input.Baseline.Ranges[metricName]; ok {
			r = &rr
		}
		status := deviationStatus(value, 0, r, len(input.History) > 1)
		if status == "normal" && len(input.History) > 1 {
			median, spread := medianAndSpread(input.History, metricName)
			if spread > 0 && abs(value-median) > spread*1.5 {
				status = "mild_deviation"
			}
		}
		statuses[metricName] = status
	}
	for _, anomaly := range input.Scores.HardAnomalies {
		summaries = append(summaries, "Hard anomaly: "+anomaly)
	}
	if statuses["protocol_conformity_score"] == "significant_deviation" || statuses["usage_consistency_score"] == "significant_deviation" {
		summaries = append(summaries, "Protocol or usage token behavior is outside the expected baseline range.")
	}
	if statuses["tier_fidelity_score"] != "normal" {
		summaries = append(summaries, "Tier fidelity is below the current baseline or historical norm.")
	}
	if statuses["capability_tool_score"] != "normal" {
		summaries = append(summaries, "Tool or function-call capability is weaker than the expected baseline.")
	}
	if statuses["route_integrity_score"] != "normal" {
		summaries = append(summaries, "Route integrity is unstable; stream or delivery behavior needs a retest.")
	}
	if translationSuspected {
		summaries = append(summaries, "Anthropic Messages stream shows chat-completions-style translation traces, so the route is unlikely to be a clean native /v1/messages path.")
	}
	if wrapperStrongSuspected {
		summaries = append(summaries, "Wrapper contamination probes strongly suggest extra platform instructions, branding, or output rewriting.")
	} else if wrapperSuspected {
		summaries = append(summaries, "Wrapper probes show mild signs of extra formatting or platform-layer interference.")
	}
	if identityRewriteSuspected {
		summaries = append(summaries, "Multi-turn identity probes indicate the model's self-description can be rewritten too easily within the same interaction.")
	}
	if identityConflict {
		summaries = append(summaries, "Self-reported identity is inconsistent across low-frequency language prompts, which suggests identity rewriting or unstable self-description.")
	} else if identityMismatch {
		summaries = append(summaries, "Self-reported identity does not align cleanly with the expected OpenAI / GPT-5 family profile.")
	}
	if len(summaries) == 0 {
		summaries = append(summaries, "Current results are within the configured baseline range.")
	}

	conclusion := "high_confidence_official_compatible"
	switch {
	case len(input.Scores.HardAnomalies) > 0:
		conclusion = "suspected_route_or_protocol_mismatch"
	case translationSuspected:
		conclusion = "suspected_route_or_protocol_mismatch"
	case wrapperStrongSuspected:
		conclusion = "suspected_wrapper_or_hidden_prompt"
	case identityRewriteSuspected && wrapperSuspected:
		conclusion = "suspected_identity_rewrite_layer"
	case identityConflict && wrapperSuspected:
		conclusion = "suspected_identity_rewrite_layer"
	case statuses["usage_consistency_score"] == "significant_deviation":
		conclusion = "usage_token_anomaly"
	case statuses["capability_tool_score"] == "significant_deviation" && statuses["tier_fidelity_score"] == "significant_deviation":
		conclusion = "suspected_same_brand_downgrade"
	case statuses["tier_fidelity_score"] == "significant_deviation":
		conclusion = "suspected_same_brand_downgrade"
	case identityMismatch && statuses["behavior_fingerprint_score"] != "normal":
		conclusion = "identity_claim_inconsistency"
	case wrapperSuspected:
		conclusion = "compatibility_with_wrapper_risk"
	case statuses["behavior_fingerprint_score"] == "significant_deviation":
		conclusion = "compatibility_without_fidelity_risk"
	}
	return Interpretation{
		Conclusion: conclusion,
		Summaries:  summaries,
		Statuses:   statuses,
	}
}

func deviationStatus(value float64, customMin float64, r *baseline.Range, useHistory bool) string {
	status := "normal"
	if r != nil {
		if r.Min != nil && value < *r.Min {
			status = "significant_deviation"
		}
		if r.Max != nil && value > *r.Max {
			status = "significant_deviation"
		}
	}
	if r == nil && customMin > 0 && value < customMin {
		status = "significant_deviation"
	}
	_ = useHistory
	return status
}

func medianAndSpread(reports []history.Report, metricName string) (float64, float64) {
	var values []float64
	for _, report := range reports {
		if v, ok := report.Scores[metricName]; ok {
			values = append(values, v)
		}
	}
	if len(values) == 0 {
		return 0, 0
	}
	sort.Float64s(values)
	median := values[len(values)/2]
	minV := values[0]
	maxV := values[len(values)-1]
	return median, maxV - minV
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

func FormatStatus(status string) string {
	switch status {
	case "normal":
		return "正常"
	case "mild_deviation":
		return "轻微偏离"
	case "significant_deviation":
		return "显著偏离"
	default:
		return fmt.Sprintf("未知(%s)", status)
	}
}
