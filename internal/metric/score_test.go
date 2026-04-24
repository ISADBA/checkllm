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

func TestCalculateRouteScorePenalizesAnthropicMessagesTranslationTrace(t *testing.T) {
	grouped := map[string][]probe.Result{
		"protocol-stream-basic": {
			{
				Definition: probe.Definition{Name: "protocol-stream-basic", Stream: true},
				StatusCode: 200,
				RawResponse: `event: message_start
data: {"type":"message_start","message":{"id":"chatcmpl-msg_123","type":"message","role":"assistant","content":[]}}

event: content_block_start
data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"hi"}}

event: message_delta
data: {"type":"message_delta","delta":{"stop_reason":"","stop_sequence":null},"usage":{"input_tokens":5,"output_tokens":3}}

event: message_stop
data: {"type":"message_stop"}`,
				StreamEvents: []probe.StreamEvent{
					{Type: "message_start"},
					{Type: "content_block_start"},
					{Type: "content_block_delta"},
					{Type: "message_delta"},
					{Type: "message_stop"},
				},
			},
			{
				Definition: probe.Definition{Name: "protocol-stream-basic", Stream: true},
				StatusCode: 200,
				RawResponse: `event: message_start
data: {"type":"message_start","message":{"id":"chatcmpl-msg_456","type":"message","role":"assistant","content":[]}}

event: content_block_start
data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"hi"}}

event: message_delta
data: {"type":"message_delta","delta":{"stop_reason":"","stop_sequence":null},"usage":{"input_tokens":5,"output_tokens":3}}

event: message_stop
data: {"type":"message_stop"}`,
				StreamEvents: []probe.StreamEvent{
					{Type: "message_start"},
					{Type: "content_block_start"},
					{Type: "content_block_delta"},
					{Type: "message_delta"},
					{Type: "message_stop"},
				},
			},
		},
	}

	scores := &Scores{Observations: map[string]float64{}}
	routeScore := calculateRouteScore(Input{Provider: "anthropic"}, grouped, 100, 100, scores)

	if routeScore >= 90 {
		t.Fatalf("expected translated anthropic route score to be penalized, got %d", routeScore)
	}
	if cleanliness := scores.Observations["anthropic_messages_translation_cleanliness"]; cleanliness >= 0.75 {
		t.Fatalf("expected low translation cleanliness, got %.3f", cleanliness)
	}
}
