package anthropic

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/ISADBA/checkllm/internal/provider"
)

const anthropicVersion = "2023-06-01"

type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
	model      string
}

func NewClient(baseURL, apiKey, model string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		model:   model,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

func (c *Client) Execute(ctx context.Context, req provider.ProbeRequest) (provider.Result, error) {
	if req.Stream {
		return c.executeStream(ctx, req)
	}
	return c.executeOnce(ctx, req)
}

func (c *Client) executeOnce(ctx context.Context, req provider.ProbeRequest) (provider.Result, error) {
	body, err := json.Marshal(buildMessagesRequest(req, c.model, false, nil))
	if err != nil {
		return provider.Result{}, err
	}
	httpReq, err := c.newRequest(ctx, body)
	if err != nil {
		return provider.Result{}, err
	}

	start := time.Now()
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return provider.Result{}, err
	}
	defer resp.Body.Close()

	payload, readErr := io.ReadAll(resp.Body)
	latency := time.Since(start)
	result := provider.Result{
		StatusCode:  resp.StatusCode,
		Latency:     latency,
		RawRequest:  string(body),
		RawResponse: string(payload),
	}
	if readErr != nil {
		return result, readErr
	}
	if resp.StatusCode >= 400 {
		result.ErrorBody = string(payload)
		return result, fmt.Errorf("anthropic error status %d", resp.StatusCode)
	}
	if err := validateJSONResponse(resp, payload, c.baseURL); err != nil {
		result.ErrorBody = string(payload)
		return result, err
	}
	var decoded messagesResponse
	if err := json.Unmarshal(payload, &decoded); err != nil {
		result.ErrorBody = string(payload)
		return result, err
	}
	applyResponse(&result, decoded)
	if (req.ToolResult != "" || len(req.ToolResults) > 0) && len(result.ToolCalls) > 0 {
		followUp, err := c.executeToolFollowUp(ctx, req, result, decoded.Content)
		if err != nil {
			return followUp, err
		}
		return followUp, nil
	}
	return result, nil
}

func (c *Client) executeToolFollowUp(ctx context.Context, req provider.ProbeRequest, result provider.Result, assistantContent []content) (provider.Result, error) {
	history := []map[string]any{
		{
			"role": "user",
			"content": []map[string]any{
				{"type": "text", "text": req.Prompt},
			},
		},
		{
			"role":    "assistant",
			"content": assistantContent,
		},
	}
	for step := 0; step < 4; step++ {
		userToolResult, ok := buildToolResultMessage(req, assistantContent)
		if !ok {
			return result, nil
		}
		history = append(history, userToolResult)
		body, err := buildToolFollowUpRequestBody(req, c.model, history)
		if err != nil {
			return result, err
		}
		httpReq, err := c.newRequest(ctx, body)
		if err != nil {
			return result, err
		}
		resp, err := c.httpClient.Do(httpReq)
		if err != nil {
			return result, err
		}
		payload, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			return result, readErr
		}
		result.RawRequest = result.RawRequest + "\n\n--- FOLLOWUP REQUEST ---\n" + string(body)
		result.RawResponse = result.RawResponse + "\n\n--- FOLLOWUP RESPONSE ---\n" + string(payload)
		if resp.StatusCode >= 400 {
			result.ErrorBody = string(payload)
			return result, fmt.Errorf("anthropic follow-up error status %d", resp.StatusCode)
		}
		if err := validateJSONResponse(resp, payload, c.baseURL); err != nil {
			result.ErrorBody = string(payload)
			return result, err
		}
		var decoded messagesResponse
		if err := json.Unmarshal(payload, &decoded); err != nil {
			return result, err
		}
		mergeResponse(&result, decoded)
		assistantContent = decoded.Content
		history = append(history, map[string]any{
			"role":    "assistant",
			"content": assistantContent,
		})
		if decoded.StopReason != "tool_use" {
			return result, nil
		}
	}
	return result, nil
}

func (c *Client) executeStream(ctx context.Context, req provider.ProbeRequest) (provider.Result, error) {
	body, err := json.Marshal(buildMessagesRequest(req, c.model, true, nil))
	if err != nil {
		return provider.Result{}, err
	}
	httpReq, err := c.newRequest(ctx, body)
	if err != nil {
		return provider.Result{}, err
	}

	start := time.Now()
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return provider.Result{}, err
	}
	defer resp.Body.Close()

	result := provider.Result{
		StatusCode: resp.StatusCode,
		RawRequest: string(body),
	}
	if resp.StatusCode >= 400 {
		payload, _ := io.ReadAll(resp.Body)
		result.ErrorBody = string(payload)
		result.RawResponse = string(payload)
		result.Latency = time.Since(start)
		return result, fmt.Errorf("anthropic stream error status %d", resp.StatusCode)
	}
	if err := validateStreamResponse(resp, c.baseURL); err != nil {
		payload, _ := io.ReadAll(resp.Body)
		result.ErrorBody = string(payload)
		result.RawResponse = string(payload)
		result.Latency = time.Since(start)
		return result, err
	}

	reader := bufio.NewReader(resp.Body)
	var rawBuilder strings.Builder
	var textBuilder strings.Builder
	firstEventSeen := false
	eventType := ""
	var currentTool *provider.ToolCall

	for {
		line, err := reader.ReadString('\n')
		if err != nil && err != io.EOF {
			return result, err
		}
		rawBuilder.WriteString(line)
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "event: "):
			eventType = strings.TrimSpace(strings.TrimPrefix(line, "event: "))
		case strings.HasPrefix(line, "data: "):
			payload := strings.TrimSpace(strings.TrimPrefix(line, "data: "))
			if payload == "" {
				break
			}
			if payload == "[DONE]" {
				result.StreamEvents = append(result.StreamEvents, provider.StreamEvent{
					Type:      "done",
					Timestamp: time.Now(),
					Bytes:     len(payload),
				})
				break
			}
			now := time.Now()
			if !firstEventSeen {
				result.FirstEventLatency = now.Sub(start)
				firstEventSeen = true
			}
			result.StreamEvents = append(result.StreamEvents, provider.StreamEvent{
				Type:      streamEventName(eventType, payload),
				Timestamp: now,
				Bytes:     len(payload),
			})
			consumeStreamData(payload, &textBuilder, &result, &currentTool)
		}
		if err == io.EOF {
			break
		}
	}

	result.RawResponse = rawBuilder.String()
	result.Text = strings.TrimSpace(textBuilder.String())
	result.Latency = time.Since(start)
	return result, nil
}

func (c *Client) newRequest(ctx context.Context, body []byte) (*http.Request, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, anthropicMessagesURL(c.baseURL), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("x-api-key", c.apiKey)
	httpReq.Header.Set("anthropic-version", anthropicVersion)
	httpReq.Header.Set("content-type", "application/json")
	return httpReq, nil
}

func anthropicMessagesURL(baseURL string) string {
	baseURL = strings.TrimRight(baseURL, "/")
	if strings.HasSuffix(baseURL, "/v1") {
		return baseURL + "/messages"
	}
	return baseURL + "/v1/messages"
}

func buildMessagesRequest(req provider.ProbeRequest, model string, stream bool, toolMessages []message) messagesRequest {
	msgs := []message{
		{
			Role: "user",
			Content: []content{
				{Type: "text", Text: req.Prompt},
			},
		},
	}
	if len(toolMessages) > 0 {
		msgs = toolMessages
	}
	out := messagesRequest{
		Model:       model,
		MaxTokens:   req.MaxOutputTokens,
		Messages:    msgs,
		Stream:      stream,
		Temperature: req.Temperature,
		Tools:       toToolDefinitions(req.Tools),
	}
	if len(req.Tools) > 0 {
		out.ToolChoice = map[string]string{"type": "auto"}
	}
	if thinking := buildThinkingConfig(req.ReasoningEffort); thinking != nil {
		out.Thinking = thinking
	}
	return out
}

func buildThinkingConfig(effort string) *thinkingConfig {
	switch strings.ToLower(strings.TrimSpace(effort)) {
	case "":
		return nil
	case "low":
		return &thinkingConfig{Type: "enabled", BudgetTokens: 1024}
	case "medium":
		return &thinkingConfig{Type: "enabled", BudgetTokens: 2048}
	case "high":
		return &thinkingConfig{Type: "enabled", BudgetTokens: 4096}
	default:
		return &thinkingConfig{Type: "enabled", BudgetTokens: 2048}
	}
}

func toToolDefinitions(specs []provider.ToolSpec) []toolDefinition {
	if len(specs) == 0 {
		return nil
	}
	out := make([]toolDefinition, 0, len(specs))
	for _, spec := range specs {
		out = append(out, toolDefinition{
			Name:        spec.Name,
			Description: spec.Description,
			InputSchema: spec.Parameters,
		})
	}
	return out
}

func applyResponse(result *provider.Result, decoded messagesResponse) {
	result.Text = extractOutputText(decoded.Content)
	result.ToolCalls = extractToolCalls(decoded.Content)
	result.Usage = provider.Usage{
		InputTokens:  decoded.Usage.InputTokens,
		OutputTokens: decoded.Usage.OutputTokens,
		TotalTokens:  decoded.Usage.InputTokens + decoded.Usage.OutputTokens,
		CachedTokens: decoded.Usage.CacheReadInputTokens,
	}
	result.UsageReturned = decoded.Usage.InputTokens > 0 || decoded.Usage.OutputTokens > 0
}

func mergeResponse(result *provider.Result, decoded messagesResponse) {
	text := extractOutputText(decoded.Content)
	if text != "" {
		result.Text = text
	}
	toolCalls := extractToolCalls(decoded.Content)
	if len(toolCalls) > 0 {
		result.ToolCalls = appendUniqueToolCalls(result.ToolCalls, toolCalls)
	}
	result.Usage = provider.Usage{
		InputTokens:  decoded.Usage.InputTokens,
		OutputTokens: decoded.Usage.OutputTokens,
		TotalTokens:  decoded.Usage.InputTokens + decoded.Usage.OutputTokens,
		CachedTokens: decoded.Usage.CacheReadInputTokens,
	}
	result.UsageReturned = decoded.Usage.InputTokens > 0 || decoded.Usage.OutputTokens > 0
}

func extractOutputText(items []content) string {
	var b strings.Builder
	for _, item := range items {
		if item.Type == "text" && item.Text != "" {
			b.WriteString(item.Text)
		}
		if item.Type == "thinking" && item.Thinking != "" {
			if b.Len() > 0 {
				b.WriteByte('\n')
			}
			b.WriteString(item.Thinking)
		}
	}
	return strings.TrimSpace(b.String())
}

func extractToolCalls(items []content) []provider.ToolCall {
	var calls []provider.ToolCall
	for _, item := range items {
		if item.Type != "tool_use" {
			continue
		}
		args, _ := item.Input.(map[string]any)
		calls = append(calls, provider.ToolCall{
			Name:      item.Name,
			Arguments: args,
		})
	}
	return calls
}

func buildToolFollowUpRequestBody(req provider.ProbeRequest, model string, history []map[string]any) ([]byte, error) {
	payload := map[string]any{
		"model":      model,
		"max_tokens": req.MaxOutputTokens,
		"messages":   history,
	}
	if len(req.Tools) > 0 {
		payload["tools"] = toToolDefinitions(req.Tools)
		payload["tool_choice"] = map[string]string{"type": "auto"}
	}
	if req.Temperature != 0 {
		payload["temperature"] = req.Temperature
	}
	if thinking := buildThinkingConfig(req.ReasoningEffort); thinking != nil {
		payload["thinking"] = thinking
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return body, nil
}

func buildToolResultMessage(req provider.ProbeRequest, assistantContent []content) (map[string]any, bool) {
	var userContent []map[string]any
	for _, item := range assistantContent {
		if item.Type != "tool_use" {
			continue
		}
		toolOutput, ok := lookupToolResult(req, item.Name)
		if !ok {
			continue
		}
		block := map[string]any{
			"type":        "tool_result",
			"tool_use_id": item.ID,
			"content":     toolOutput,
		}
		if strings.Contains(strings.ToLower(toolOutput), `"error"`) {
			block["is_error"] = true
		}
		userContent = append(userContent, block)
	}
	if len(userContent) == 0 {
		return nil, false
	}
	return map[string]any{
		"role":    "user",
		"content": userContent,
	}, true
}

func appendUniqueToolCalls(existing, next []provider.ToolCall) []provider.ToolCall {
	seen := map[string]struct{}{}
	for _, call := range existing {
		seen[toolCallKey(call)] = struct{}{}
	}
	for _, call := range next {
		key := toolCallKey(call)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		existing = append(existing, call)
	}
	return existing
}

func toolCallKey(call provider.ToolCall) string {
	body, _ := json.Marshal(call.Arguments)
	return call.Name + ":" + string(body)
}

func lookupToolResult(req provider.ProbeRequest, toolName string) (string, bool) {
	if len(req.ToolResults) > 0 {
		if output, ok := req.ToolResults[toolName]; ok {
			return output, true
		}
	}
	if req.ToolResult != "" {
		return req.ToolResult, true
	}
	return "", false
}

func validateJSONResponse(resp *http.Response, payload []byte, baseURL string) error {
	contentType := strings.ToLower(resp.Header.Get("Content-Type"))
	trimmed := bytes.TrimSpace(payload)
	if strings.Contains(contentType, "application/json") {
		return nil
	}
	if len(trimmed) > 0 && (trimmed[0] == '{' || trimmed[0] == '[') {
		return nil
	}
	return fmt.Errorf("endpoint did not return Anthropic-compatible /v1/messages data (content-type=%q, base_url=%q)", contentType, baseURL)
}

func validateStreamResponse(resp *http.Response, baseURL string) error {
	contentType := strings.ToLower(resp.Header.Get("Content-Type"))
	if strings.Contains(contentType, "text/event-stream") {
		return nil
	}
	return fmt.Errorf("endpoint did not return Anthropic-compatible SSE stream (content-type=%q, base_url=%q)", contentType, baseURL)
}

func streamEventName(eventType, payload string) string {
	if eventType != "" {
		return eventType
	}
	var env streamEnvelope
	if err := json.Unmarshal([]byte(payload), &env); err == nil && env.Type != "" {
		return env.Type
	}
	return "unknown"
}

func consumeStreamData(payload string, textBuilder *strings.Builder, result *provider.Result, currentTool **provider.ToolCall) {
	var raw map[string]any
	if err := json.Unmarshal([]byte(payload), &raw); err != nil {
		return
	}
	switch asString(raw["type"]) {
	case "content_block_start":
		block, _ := raw["content_block"].(map[string]any)
		switch asString(block["type"]) {
		case "tool_use":
			call := &provider.ToolCall{
				Name:      asString(block["name"]),
				Arguments: map[string]any{},
			}
			if input, ok := block["input"].(map[string]any); ok {
				call.Arguments = input
			}
			result.ToolCalls = append(result.ToolCalls, *call)
			*currentTool = &result.ToolCalls[len(result.ToolCalls)-1]
		}
	case "content_block_delta":
		delta, _ := raw["delta"].(map[string]any)
		switch asString(delta["type"]) {
		case "text_delta":
			textBuilder.WriteString(asString(delta["text"]))
		case "input_json_delta":
			if *currentTool == nil {
				return
			}
			partial := asString(delta["partial_json"])
			if partial == "" {
				return
			}
			var args map[string]any
			if err := json.Unmarshal([]byte(partial), &args); err == nil {
				(*currentTool).Arguments = args
			}
		}
	case "message_delta":
		delta, _ := raw["delta"].(map[string]any)
		if stopReason := asString(delta["stop_reason"]); stopReason != "" {
			result.StreamEvents = append(result.StreamEvents, provider.StreamEvent{
				Type:      "stop_reason:" + stopReason,
				Timestamp: time.Now(),
				Bytes:     len(payload),
			})
		}
		if usage, ok := raw["usage"].(map[string]any); ok {
			result.Usage = provider.Usage{
				InputTokens:  asInt(usage["input_tokens"]),
				OutputTokens: asInt(usage["output_tokens"]),
				TotalTokens:  asInt(usage["input_tokens"]) + asInt(usage["output_tokens"]),
				CachedTokens: asInt(usage["cache_read_input_tokens"]),
			}
			result.UsageReturned = true
		}
	case "message_start":
		message, _ := raw["message"].(map[string]any)
		if usage, ok := message["usage"].(map[string]any); ok {
			result.Usage.InputTokens = asInt(usage["input_tokens"])
			result.Usage.CachedTokens = asInt(usage["cache_read_input_tokens"])
			result.Usage.TotalTokens = result.Usage.InputTokens + result.Usage.OutputTokens
			result.UsageReturned = result.Usage.InputTokens > 0
		}
	}
}

func asString(v any) string {
	s, _ := v.(string)
	return s
}

func asInt(v any) int {
	switch typed := v.(type) {
	case float64:
		return int(typed)
	case int:
		return typed
	default:
		return 0
	}
}
