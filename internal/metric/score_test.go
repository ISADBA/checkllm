package metric

import (
	"errors"
	"testing"

	"github.com/ISADBA/checkllm/internal/probe"
)

func TestCalculateCapabilityScoreDownweightsOpenAIFollowUpChecks(t *testing.T) {
	grouped := map[string][]probe.Result{
		"capability-tool-math": {
			{
				Definition: probe.Definition{
					Name:             "capability-tool-math",
					Kind:             probe.KindCapability,
					ExpectToolCall:   true,
					ExpectedToolName: "sum_numbers",
					ExpectedToolArgs: map[string]string{"a": "17", "b": "25"},
				},
				StatusCode: 200,
				ToolCalls: []probe.ToolCall{
					{Name: "sum_numbers", Arguments: map[string]any{"a": 17, "b": 25}},
				},
				UsageReturned: true,
			},
		},
		"capability-tool-math-followup": {
			{
				Definition: probe.Definition{
					Name:                 "capability-tool-math-followup",
					Kind:                 probe.KindCapability,
					ExpectToolCall:       true,
					ExpectedToolName:     "sum_numbers",
					ExpectedToolArgs:     map[string]string{"a": "17", "b": "25"},
					ExpectFinalText:      true,
					ExpectedFinalPhrases: []string{"42"},
				},
				StatusCode:    200,
				UsageReturned: true,
				Err:           errors.New("follow-up not supported"),
			},
		},
	}

	openaiScore := calculateCapabilityScore(Input{Provider: "openai"}, grouped, &Scores{Observations: map[string]float64{}})
	anthropicScore := calculateCapabilityScore(Input{Provider: "anthropic"}, grouped, &Scores{Observations: map[string]float64{}})

	if openaiScore <= anthropicScore {
		t.Fatalf("expected openai score %d to be higher than anthropic score %d after downweighting", openaiScore, anthropicScore)
	}
	if openaiScore != 94 {
		t.Fatalf("expected openai score 94, got %d", openaiScore)
	}
	if anthropicScore != 75 {
		t.Fatalf("expected anthropic score 75, got %d", anthropicScore)
	}
}
