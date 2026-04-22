package probe

import "time"

type Kind string

const (
	KindProtocol    Kind = "protocol"
	KindUsage       Kind = "usage"
	KindFingerprint Kind = "fingerprint"
	KindTier        Kind = "tier"
	KindCapability  Kind = "capability"
	KindThinking    Kind = "thinking"
)

type Definition struct {
	Name                 string
	ReuseResultFrom      string
	Kind                 Kind
	Prompt               string
	Stream               bool
	MaxOutputTokens      int
	Temperature          float64
	ReasoningEffort      string
	PromptCacheKey       string
	PromptCacheRetention string
	ExpectJSON           bool
	ExpectUsage          bool
	ExpectedPhrase       string
	ExpectedSubstrings   []string
	ExpectedJSONKeys     []string
	ExpectedJSONValues   map[string]string
	ExpectedLineSequence []string
	ForbiddenSubstrings  []string
	MinStreamEvents      int
	Tools                []ToolSpec
	ExpectToolCall       bool
	ExpectedToolName     string
	ExpectedToolArgs     map[string]string
	ToolResult           string
	ToolResults          map[string]string
	ExpectFinalText      bool
	ExpectedFinalPhrases []string
	Repeat               int
}

type ToolSpec struct {
	Name        string
	Description string
	Parameters  map[string]any
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

type Result struct {
	Definition           Definition
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
	Err                  error
}

type ToolCall struct {
	Name      string
	Arguments map[string]any
}
