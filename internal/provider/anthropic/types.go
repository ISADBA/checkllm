package anthropic

type messagesRequest struct {
	Model         string            `json:"model"`
	MaxTokens     int               `json:"max_tokens"`
	Messages      []message         `json:"messages"`
	Stream        bool              `json:"stream,omitempty"`
	Temperature   float64           `json:"temperature,omitempty"`
	Tools         []toolDefinition  `json:"tools,omitempty"`
	ToolChoice    any               `json:"tool_choice,omitempty"`
	Thinking      *thinkingConfig   `json:"thinking,omitempty"`
	System        string            `json:"system,omitempty"`
	Metadata      map[string]string `json:"metadata,omitempty"`
	StopSequences []string          `json:"stop_sequences,omitempty"`
}

type message struct {
	Role    string    `json:"role"`
	Content []content `json:"content"`
}

type content struct {
	Type      string `json:"type"`
	Text      string `json:"text,omitempty"`
	ID        string `json:"id,omitempty"`
	Name      string `json:"name,omitempty"`
	Input     any    `json:"input,omitempty"`
	Thinking  string `json:"thinking,omitempty"`
	Signature string `json:"signature,omitempty"`
}

type toolDefinition struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	InputSchema map[string]any `json:"input_schema,omitempty"`
}

type thinkingConfig struct {
	Type         string `json:"type"`
	BudgetTokens int    `json:"budget_tokens"`
}

type messagesResponse struct {
	ID         string    `json:"id"`
	Type       string    `json:"type"`
	Role       string    `json:"role"`
	Content    []content `json:"content"`
	StopReason string    `json:"stop_reason"`
	Usage      usage     `json:"usage"`
}

type usage struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens,omitempty"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens,omitempty"`
}

type streamEnvelope struct {
	Type string `json:"type"`
}
