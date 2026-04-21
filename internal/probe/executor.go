package probe

import (
	"context"
	"strings"

	openai "github.com/ISADBA/checkllm/internal/provider/openai"
)

func ExecuteAll(ctx context.Context, provider *openai.Client, defs []Definition) ([]Result, error) {
	results := make([]Result, 0, len(defs))
	for _, def := range defs {
		repeat := def.Repeat
		if repeat < 1 {
			repeat = 1
		}
		for i := 0; i < repeat; i++ {
			rawResult, err := provider.Execute(ctx, openai.ProbeRequest{
				Name:            def.Name,
				Prompt:          def.Prompt,
				Stream:          def.Stream,
				MaxOutputTokens: def.MaxOutputTokens,
				Temperature:     def.Temperature,
				ReasoningEffort: def.ReasoningEffort,
				Tools:           toProviderTools(def.Tools),
				ToolResult:      def.ToolResult,
				ToolResults:     def.ToolResults,
			})
			result := Result{
				Definition:  def,
				StatusCode:  rawResult.StatusCode,
				Text:        rawResult.Text,
				ErrorBody:   rawResult.ErrorBody,
				RawRequest:  rawResult.RawRequest,
				RawResponse: rawResult.RawResponse,
				Usage: Usage{
					InputTokens:  rawResult.Usage.InputTokens,
					OutputTokens: rawResult.Usage.OutputTokens,
					TotalTokens:  rawResult.Usage.TotalTokens,
				},
				Latency:           rawResult.Latency,
				FirstEventLatency: rawResult.FirstEventLatency,
				UsageReturned:     rawResult.UsageReturned,
			}
			for _, evt := range rawResult.StreamEvents {
				result.StreamEvents = append(result.StreamEvents, StreamEvent{
					Type:      evt.Type,
					Timestamp: evt.Timestamp,
					Bytes:     evt.Bytes,
				})
			}
			for _, call := range rawResult.ToolCalls {
				result.ToolCalls = append(result.ToolCalls, ToolCall{
					Name:      call.Name,
					Arguments: call.Arguments,
				})
			}
			if err != nil {
				result.Err = err
			}
			results = append(results, result)
			if isFatalProtocolMismatch(def, err) {
				return results, nil
			}
		}
	}
	return results, nil
}

func isFatalProtocolMismatch(def Definition, err error) bool {
	if err == nil {
		return false
	}
	if def.Name != "protocol-basic" {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "OpenAI-compatible /responses data") ||
		strings.Contains(msg, "received HTML page instead of API JSON") ||
		strings.Contains(msg, "web route, not the Responses API")
}

func toProviderTools(specs []ToolSpec) []openai.ToolSpec {
	if len(specs) == 0 {
		return nil
	}
	out := make([]openai.ToolSpec, 0, len(specs))
	for _, spec := range specs {
		out = append(out, openai.ToolSpec{
			Name:        spec.Name,
			Description: spec.Description,
			Parameters:  spec.Parameters,
		})
	}
	return out
}
