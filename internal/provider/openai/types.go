package openai

type responsesRequest struct {
	Model                string                 `json:"model"`
	Input                any                    `json:"input"`
	PreviousResponseID   string                 `json:"previous_response_id,omitempty"`
	Stream               bool                   `json:"stream,omitempty"`
	MaxOutputTokens      int                    `json:"max_output_tokens,omitempty"`
	Temperature          float64                `json:"temperature,omitempty"`
	Reasoning            map[string]any         `json:"reasoning,omitempty"`
	PromptCacheKey       string                 `json:"prompt_cache_key,omitempty"`
	PromptCacheRetention string                 `json:"prompt_cache_retention,omitempty"`
	Text                 map[string]interface{} `json:"text,omitempty"`
	ToolChoice           any                    `json:"tool_choice,omitempty"`
	Tools                []toolDefinition       `json:"tools,omitempty"`
}

type responsesResponse struct {
	ID                   string        `json:"id"`
	Output               []outputItem  `json:"output"`
	Usage                usagePayload  `json:"usage"`
	Error                *errorPayload `json:"error"`
	PromptCacheKey       string        `json:"prompt_cache_key"`
	PromptCacheRetention string        `json:"prompt_cache_retention"`
}

type outputItem struct {
	Type      string          `json:"type"`
	ID        string          `json:"id,omitempty"`
	CallID    string          `json:"call_id,omitempty"`
	Name      string          `json:"name,omitempty"`
	Arguments string          `json:"arguments,omitempty"`
	Text      string          `json:"text,omitempty"`
	Summary   []contentRecord `json:"summary,omitempty"`
	Content   []contentRecord `json:"content,omitempty"`
}

type contentRecord struct {
	Type      string `json:"type"`
	Text      string `json:"text"`
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
}

type responseInputItem struct {
	Type    string             `json:"type,omitempty"`
	Role    string             `json:"role,omitempty"`
	Content []inputContentItem `json:"content,omitempty"`
	CallID  string             `json:"call_id,omitempty"`
	Output  string             `json:"output,omitempty"`
}

type inputContentItem struct {
	Type string `json:"type,omitempty"`
	Text string `json:"text,omitempty"`
}

type usagePayload struct {
	InputTokens         int                `json:"input_tokens"`
	OutputTokens        int                `json:"output_tokens"`
	TotalTokens         int                `json:"total_tokens"`
	InputTokensDetails  usageInputDetails  `json:"input_tokens_details"`
	OutputTokensDetails usageOutputDetails `json:"output_tokens_details"`
}

type usageInputDetails struct {
	CachedTokens int `json:"cached_tokens"`
}

type usageOutputDetails struct {
	ReasoningTokens int `json:"reasoning_tokens"`
}

type errorPayload struct {
	Message string `json:"message"`
	Type    string `json:"type"`
}

type toolDefinition struct {
	Type        string         `json:"type"`
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Parameters  map[string]any `json:"parameters,omitempty"`
}

type streamEnvelope struct {
	Type  string        `json:"type"`
	Usage *usagePayload `json:"usage,omitempty"`
	Delta string        `json:"delta,omitempty"`
}
