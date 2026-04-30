package runcheck

import (
	"context"
	"time"

	"github.com/ISADBA/checkllm/internal/report"
)

type Input struct {
	BaseURL      string
	APIKey       string
	Model        string
	Provider     string
	BaselinePath string
	Timeout      time.Duration
	MaxSamples   int
	EnableStream bool
	ExpectUsage  bool
	OutputPath   string
	HistoryDir   string
	WriteReport  bool
}

type ScoresSummary struct {
	Risk         float64
	Protocol     float64
	Stream       float64
	Usage        float64
	Fingerprint  float64
	Capability   float64
	Tier         float64
	Route        float64
	Functional   float64
	Intelligence float64
}

type ThinkingSummary struct {
	Status string
}

type PromptCacheSummary struct {
	Status string
}

type NetworkSummary struct {
	AvgLatencyMs         float64
	P95LatencyMs         float64
	AvgFirstByteMs       float64
	AvgOutputTokensPerS  float64
	TimeoutCount         float64
	SuccessfulProbeCount float64
}

type Summary struct {
	RunAt       time.Time
	Provider    string
	Model       string
	BaseURL     string
	Conclusion  string
	Scores      ScoresSummary
	Statuses    map[string]string
	Thinking    ThinkingSummary
	PromptCache PromptCacheSummary
	Network     NetworkSummary
}

type Result struct {
	Summary        Summary
	RunReport      report.RunReport
	ArchivePath    string
	UserReportPath string
}

type Service interface {
	Run(ctx context.Context, input Input) (Result, error)
}
