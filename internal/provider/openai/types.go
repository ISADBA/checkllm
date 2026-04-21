package openai

import "time"

type ProbeRequest struct {
	Name            string
	Prompt          string
	Stream          bool
	MaxOutputTokens int
	Temperature     float64
	ReasoningEffort string
	Tools           []ToolSpec
	ToolResult      string
	ToolResults     map[string]string
}

type Result struct {
	StatusCode        int
	Text              string
	ErrorBody         string
	RawRequest        string
	RawResponse       string
	Usage             Usage
	Latency           time.Duration
	FirstEventLatency time.Duration
	StreamEvents      []StreamEvent
	ToolCalls         []ToolCall
	UsageReturned     bool
}

type Usage struct {
	InputTokens  int
	OutputTokens int
	TotalTokens  int
}

type StreamEvent struct {
	Type      string
	Timestamp time.Time
	Bytes     int
}

type ToolSpec struct {
	Name        string
	Description string
	Parameters  map[string]any
}

type ToolCall struct {
	Name      string
	Arguments map[string]any
}

type responsesRequest struct {
	Model              string                 `json:"model"`
	Input              any                    `json:"input"`
	PreviousResponseID string                 `json:"previous_response_id,omitempty"`
	Stream             bool                   `json:"stream,omitempty"`
	MaxOutputTokens    int                    `json:"max_output_tokens,omitempty"`
	Temperature        float64                `json:"temperature,omitempty"`
	Reasoning          map[string]any         `json:"reasoning,omitempty"`
	Text               map[string]interface{} `json:"text,omitempty"`
	ToolChoice         any                    `json:"tool_choice,omitempty"`
	Tools              []toolDefinition       `json:"tools,omitempty"`
}

type responsesResponse struct {
	ID     string        `json:"id"`
	Output []outputItem  `json:"output"`
	Usage  usagePayload  `json:"usage"`
	Error  *errorPayload `json:"error"`
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
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	TotalTokens  int `json:"total_tokens"`
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
