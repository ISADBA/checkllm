package probe

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ISADBA/checkllm/internal/provider"
)

func ExecuteAll(ctx context.Context, client provider.Client, defs []Definition, perProbeTimeout time.Duration) ([]Result, error) {
	results := make([]Result, 0, len(defs))
	latestByName := make(map[string]Result, len(defs))
	for _, def := range defs {
		if def.ReuseResultFrom != "" {
			reused, ok := latestByName[def.ReuseResultFrom]
			if !ok {
				return results, fmt.Errorf("probe %s reuses unknown source probe %s", def.Name, def.ReuseResultFrom)
			}
			for i := 0; i < maxRepeat(def.Repeat); i++ {
				cloned := reused
				cloned.Definition = def
				results = append(results, cloned)
			}
			continue
		}
		repeat := maxRepeat(def.Repeat)
		for i := 0; i < repeat; i++ {
			probeCtx := ctx
			cancel := func() {}
			if perProbeTimeout > 0 {
				probeCtx, cancel = context.WithTimeout(ctx, perProbeTimeout)
			}
			rawResult, err := client.Execute(probeCtx, provider.ProbeRequest{
				Name:                 def.Name,
				Prompt:               def.Prompt,
				Stream:               def.Stream,
				MaxOutputTokens:      def.MaxOutputTokens,
				Temperature:          def.Temperature,
				ReasoningEffort:      def.ReasoningEffort,
				PromptCacheKey:       def.PromptCacheKey,
				PromptCacheRetention: def.PromptCacheRetention,
				Tools:                toProviderTools(def.Tools),
				ToolResult:           def.ToolResult,
				ToolResults:          def.ToolResults,
			})
			cancel()
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
					CachedTokens: rawResult.Usage.CachedTokens,
				},
				Latency:              rawResult.Latency,
				FirstEventLatency:    rawResult.FirstEventLatency,
				UsageReturned:        rawResult.UsageReturned,
				PromptCacheKey:       rawResult.PromptCacheKey,
				PromptCacheRetention: rawResult.PromptCacheRetention,
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
			latestByName[def.Name] = result
			if isFatalProtocolMismatch(def, err) {
				return results, nil
			}
		}
	}
	return results, nil
}

func maxRepeat(repeat int) int {
	if repeat < 1 {
		return 1
	}
	return repeat
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

func toProviderTools(specs []ToolSpec) []provider.ToolSpec {
	if len(specs) == 0 {
		return nil
	}
	out := make([]provider.ToolSpec, 0, len(specs))
	for _, spec := range specs {
		out = append(out, provider.ToolSpec{
			Name:        spec.Name,
			Description: spec.Description,
			Parameters:  spec.Parameters,
		})
	}
	return out
}
