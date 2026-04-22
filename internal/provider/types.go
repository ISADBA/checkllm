package provider

import (
	"context"
	"time"
)

type Client interface {
	Execute(ctx context.Context, req ProbeRequest) (Result, error)
}

type ProbeRequest struct {
	Name                 string
	Prompt               string
	Stream               bool
	MaxOutputTokens      int
	Temperature          float64
	ReasoningEffort      string
	PromptCacheKey       string
	PromptCacheRetention string
	Tools                []ToolSpec
	ToolResult           string
	ToolResults          map[string]string
}

type Result struct {
	StatusCode           int
	Text                 string
	ErrorBody            string
	RawRequest           string
	RawResponse          string
	Usage                Usage
	Latency              time.Duration
	FirstEventLatency    time.Duration
	StreamEvents         []StreamEvent
	ToolCalls            []ToolCall
	UsageReturned        bool
	PromptCacheKey       string
	PromptCacheRetention string
}

type Usage struct {
	InputTokens  int
	OutputTokens int
	TotalTokens  int
	CachedTokens int
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
