package openai

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
)

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

func (c *Client) Execute(ctx context.Context, req ProbeRequest) (Result, error) {
	if req.Stream {
		return c.executeStream(ctx, req)
	}
	return c.executeOnce(ctx, req)
}

func (c *Client) executeOnce(ctx context.Context, req ProbeRequest) (Result, error) {
	body, err := json.Marshal(responsesRequest{
		Model:           c.model,
		Input:           buildUserInput(req.Prompt),
		MaxOutputTokens: req.MaxOutputTokens,
		Temperature:     req.Temperature,
		Reasoning:       buildReasoningConfig(req.ReasoningEffort),
		Text:            defaultTextConfig(),
		ToolChoice:      "auto",
		Tools:           toToolDefinitions(req.Tools),
	})
	if err != nil {
		return Result{}, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/responses", bytes.NewReader(body))
	if err != nil {
		return Result{}, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	start := time.Now()
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return Result{}, err
	}
	defer resp.Body.Close()

	payload, readErr := io.ReadAll(resp.Body)
	latency := time.Since(start)
	result := Result{StatusCode: resp.StatusCode, Latency: latency, RawRequest: string(body), RawResponse: string(payload)}
	if readErr != nil {
		return result, readErr
	}
	if resp.StatusCode >= 400 {
		result.ErrorBody = string(payload)
		return result, fmt.Errorf("openai error status %d", resp.StatusCode)
	}
	if err := validateJSONResponse(resp, payload, c.baseURL); err != nil {
		result.ErrorBody = string(payload)
		return result, err
	}
	var decoded responsesResponse
	if err := json.Unmarshal(payload, &decoded); err != nil {
		result.ErrorBody = string(payload)
		return result, err
	}
	result.Text = extractOutputText(decoded.Output)
	result.ToolCalls = extractToolCalls(decoded.Output)
	result.Usage = Usage{
		InputTokens:  decoded.Usage.InputTokens,
		OutputTokens: decoded.Usage.OutputTokens,
		TotalTokens:  decoded.Usage.TotalTokens,
	}
	result.UsageReturned = decoded.Usage.TotalTokens > 0 || decoded.Usage.InputTokens > 0 || decoded.Usage.OutputTokens > 0
	if (req.ToolResult != "" || len(req.ToolResults) > 0) && len(result.ToolCalls) > 0 {
		followUp, err := c.executeToolFollowUp(ctx, req, decoded, result)
		if err == nil {
			return followUp, nil
		}
	}
	return result, nil
}

func (c *Client) executeToolFollowUp(ctx context.Context, req ProbeRequest, current responsesResponse, result Result) (Result, error) {
	for step := 0; step < 4; step++ {
		input, ok := buildToolFollowUpInput(current.Output, req)
		if !ok {
			return result, nil
		}
		body, err := json.Marshal(map[string]any{
			"model":                c.model,
			"previous_response_id": current.ID,
			"input":                input,
			"max_output_tokens":    req.MaxOutputTokens,
			"temperature":          req.Temperature,
			"reasoning":            buildReasoningConfig(req.ReasoningEffort),
			"text":                 defaultTextConfig(),
			"tool_choice":          "auto",
			"tools":                toToolDefinitions(req.Tools),
		})
		if err != nil {
			return result, err
		}
		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/responses", bytes.NewReader(body))
		if err != nil {
			return result, err
		}
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
		httpReq.Header.Set("Content-Type", "application/json")
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
			return result, fmt.Errorf("openai follow-up error status %d", resp.StatusCode)
		}
		if err := validateJSONResponse(resp, payload, c.baseURL); err != nil {
			result.ErrorBody = string(payload)
			return result, err
		}
		var decoded responsesResponse
		if err := json.Unmarshal(payload, &decoded); err != nil {
			return result, err
		}
		text := extractOutputText(decoded.Output)
		if text != "" {
			result.Text = text
		}
		toolCalls := extractToolCalls(decoded.Output)
		if len(toolCalls) > 0 {
			result.ToolCalls = append(result.ToolCalls, toolCalls...)
		}
		result.Usage = Usage{
			InputTokens:  decoded.Usage.InputTokens,
			OutputTokens: decoded.Usage.OutputTokens,
			TotalTokens:  decoded.Usage.TotalTokens,
		}
		result.UsageReturned = decoded.Usage.TotalTokens > 0 || decoded.Usage.InputTokens > 0 || decoded.Usage.OutputTokens > 0
		current = decoded
		if len(toolCalls) == 0 {
			return result, nil
		}
	}
	return result, nil
}

func (c *Client) executeStream(ctx context.Context, req ProbeRequest) (Result, error) {
	body, err := json.Marshal(responsesRequest{
		Model:           c.model,
		Input:           buildUserInput(req.Prompt),
		MaxOutputTokens: req.MaxOutputTokens,
		Temperature:     req.Temperature,
		Reasoning:       buildReasoningConfig(req.ReasoningEffort),
		Stream:          true,
		Text:            defaultTextConfig(),
		ToolChoice:      "auto",
	})
	if err != nil {
		return Result{}, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/responses", bytes.NewReader(body))
	if err != nil {
		return Result{}, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	start := time.Now()
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return Result{}, err
	}
	defer resp.Body.Close()

	result := Result{StatusCode: resp.StatusCode}
	if resp.StatusCode >= 400 {
		payload, _ := io.ReadAll(resp.Body)
		result.ErrorBody = string(payload)
		result.Latency = time.Since(start)
		return result, fmt.Errorf("openai stream error status %d", resp.StatusCode)
	}
	if err := validateStreamResponse(resp, c.baseURL); err != nil {
		payload, _ := io.ReadAll(resp.Body)
		result.ErrorBody = string(payload)
		result.RawResponse = string(payload)
		result.Latency = time.Since(start)
		return result, err
	}

	reader := bufio.NewReader(resp.Body)
	var textBuilder strings.Builder
	firstEventSeen := false
	for {
		line, err := reader.ReadString('\n')
		if err != nil && err != io.EOF {
			return result, err
		}
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "data: ") {
			payload := strings.TrimPrefix(line, "data: ")
			if payload == "[DONE]" {
				result.StreamEvents = append(result.StreamEvents, StreamEvent{Type: "done", Timestamp: time.Now()})
				break
			}
			if evt, ok := parseStreamEnvelope([]byte(payload)); ok {
				now := time.Now()
				if !firstEventSeen {
					result.FirstEventLatency = now.Sub(start)
					firstEventSeen = true
				}
				result.StreamEvents = append(result.StreamEvents, StreamEvent{
					Type:      evt.Type,
					Timestamp: now,
					Bytes:     len(payload),
				})
				if evt.Delta != "" {
					textBuilder.WriteString(evt.Delta)
				}
				if evt.Usage != nil {
					result.Usage = Usage{
						InputTokens:  evt.Usage.InputTokens,
						OutputTokens: evt.Usage.OutputTokens,
						TotalTokens:  evt.Usage.TotalTokens,
					}
					result.UsageReturned = true
				}
			}
		}
		if err == io.EOF {
			break
		}
	}
	result.Text = textBuilder.String()
	result.Latency = time.Since(start)
	return result, nil
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
	return describeProtocolMismatch(baseURL, contentType, string(payload))
}

func validateStreamResponse(resp *http.Response, baseURL string) error {
	contentType := strings.ToLower(resp.Header.Get("Content-Type"))
	if strings.Contains(contentType, "text/event-stream") {
		return nil
	}
	return describeProtocolMismatch(baseURL, contentType, "")
}

func describeProtocolMismatch(baseURL, contentType, payload string) error {
	msg := fmt.Sprintf("endpoint did not return OpenAI-compatible /responses data (content-type=%q)", contentType)
	lowerPayload := strings.ToLower(payload)
	switch {
	case strings.Contains(lowerPayload, "<!doctype html") || strings.Contains(lowerPayload, "<html"):
		msg += "; received HTML page instead of API JSON"
	case payload != "":
		msg += "; received non-JSON payload"
	}
	if strings.Contains(payload, `The model "api/responses" is not available`) {
		msg += "; upstream appears to treat '/api/responses' as a web route, not the Responses API"
	}
	if strings.Contains(baseURL, "openrouter.ai/api") && !strings.Contains(baseURL, "/api/v1") {
		msg += "; for OpenRouter, verify whether the correct base URL should be 'https://openrouter.ai/api/v1'"
	}
	return fmt.Errorf(msg)
}

func parseStreamEnvelope(payload []byte) (streamEnvelope, bool) {
	var raw map[string]any
	if err := json.Unmarshal(payload, &raw); err != nil {
		return streamEnvelope{}, false
	}
	eventType := asString(raw["type"])
	if eventType == "" {
		eventType = "unknown"
	}
	var usage *usagePayload
	if extracted, ok := extractUsageFromStreamMap(raw); ok {
		usage = extracted
	}
	return streamEnvelope{
		Type:  eventType,
		Usage: usage,
		Delta: extractDeltaTextFromEvent(eventType, raw),
	}, true
}

func extractUsage(payload []byte) (*usagePayload, bool) {
	var raw map[string]any
	if err := json.Unmarshal(payload, &raw); err != nil {
		return nil, false
	}
	return extractUsageFromStreamMap(raw)
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

func extractUsageFromStreamMap(raw map[string]any) (*usagePayload, bool) {
	if usage, ok := parseUsageMap(raw["usage"]); ok {
		return usage, true
	}
	responseMap, _ := raw["response"].(map[string]any)
	if usage, ok := parseUsageMap(responseMap["usage"]); ok {
		return usage, true
	}
	return nil, false
}

func parseUsageMap(value any) (*usagePayload, bool) {
	usageMap, ok := value.(map[string]any)
	if !ok {
		return nil, false
	}
	return &usagePayload{
		InputTokens:  asInt(usageMap["input_tokens"]),
		OutputTokens: asInt(usageMap["output_tokens"]),
		TotalTokens:  asInt(usageMap["total_tokens"]),
	}, true
}

func extractDeltaTextFromEvent(eventType string, raw map[string]any) string {
	switch strings.ToLower(eventType) {
	case "response.output_text.delta":
		return asString(raw["delta"])
	case "response.output_text.done":
		return ""
	default:
		return ""
	}
}

func buildUserInput(prompt string) []responseInputItem {
	return []responseInputItem{
		{
			Role: "user",
			Content: []inputContentItem{
				{
					Type: "input_text",
					Text: prompt,
				},
			},
		},
	}
}

func buildReasoningConfig(effort string) map[string]any {
	effort = strings.TrimSpace(effort)
	if effort == "" {
		return nil
	}
	return map[string]any{"effort": effort}
}

func buildToolFollowUpInput(output []outputItem, req ProbeRequest) ([]responseInputItem, bool) {
	var input []responseInputItem
	for _, call := range extractToolCalls(output) {
		callID := findToolCallID(output, call.Name)
		if callID == "" {
			continue
		}
		toolOutput, ok := lookupToolResult(req, call.Name)
		if !ok {
			continue
		}
		input = append(input, responseInputItem{
			Type:   "function_call_output",
			CallID: callID,
			Output: toolOutput,
		})
	}
	return input, len(input) > 0
}

func lookupToolResult(req ProbeRequest, toolName string) (string, bool) {
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

func defaultTextConfig() map[string]interface{} {
	return map[string]interface{}{
		"format": map[string]interface{}{
			"type": "text",
		},
	}
}

func extractOutputText(output []outputItem) string {
	var b strings.Builder
	for _, item := range output {
		if item.Text != "" {
			b.WriteString(item.Text)
		}
		for _, content := range item.Content {
			if content.Text != "" {
				b.WriteString(content.Text)
			}
		}
		for _, summary := range item.Summary {
			if summary.Text != "" {
				b.WriteString(summary.Text)
			}
		}
	}
	return strings.TrimSpace(b.String())
}

func extractToolCalls(output []outputItem) []ToolCall {
	var calls []ToolCall
	for _, item := range output {
		if item.Type != "function_call" {
			continue
		}
		call := ToolCall{Name: item.Name, Arguments: map[string]any{}}
		if item.Arguments != "" {
			_ = json.Unmarshal([]byte(item.Arguments), &call.Arguments)
		}
		calls = append(calls, call)
	}
	return calls
}

func findToolCallID(output []outputItem, name string) string {
	for _, item := range output {
		if item.Type == "function_call" && (name == "" || item.Name == name) {
			if item.CallID != "" {
				return item.CallID
			}
			return item.ID
		}
	}
	return ""
}

func toToolDefinitions(specs []ToolSpec) []toolDefinition {
	if len(specs) == 0 {
		return nil
	}
	out := make([]toolDefinition, 0, len(specs))
	for _, spec := range specs {
		out = append(out, toolDefinition{
			Type:        "function",
			Name:        spec.Name,
			Description: spec.Description,
			Parameters:  spec.Parameters,
		})
	}
	return out
}
