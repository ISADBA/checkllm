package judge

import (
	"testing"

	"github.com/ISADBA/checkllm/internal/baseline"
	"github.com/ISADBA/checkllm/internal/config"
	"github.com/ISADBA/checkllm/internal/metric"
)

func TestInterpretFlagsAnthropicMessagesTranslationAsRouteMismatch(t *testing.T) {
	got := Interpret(Input{
		Config:   config.Config{},
		Baseline: baseline.Baseline{},
		Scores: metric.Scores{
			ProtocolConformityScore:  100,
			StreamConformityScore:    100,
			UsageConsistencyScore:    90,
			BehaviorFingerprintScore: 70,
			CapabilityToolScore:      90,
			TierFidelityScore:        90,
			RouteIntegrityScore:      88,
			OverallRiskScore:         20,
			Observations: map[string]float64{
				"anthropic_messages_translation_cleanliness": 0.45,
			},
		},
	})

	if got.Conclusion != "suspected_route_or_protocol_mismatch" {
		t.Fatalf("expected route mismatch conclusion, got %q", got.Conclusion)
	}
	if got.Statuses["anthropic_messages_translation_cleanliness"] != "significant_deviation" {
		t.Fatalf("expected significant deviation status, got %q", got.Statuses["anthropic_messages_translation_cleanliness"])
	}
}
