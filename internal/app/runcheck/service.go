package runcheck

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/ISADBA/checkllm/internal/baseline"
	"github.com/ISADBA/checkllm/internal/config"
	"github.com/ISADBA/checkllm/internal/history"
	"github.com/ISADBA/checkllm/internal/judge"
	"github.com/ISADBA/checkllm/internal/metric"
	"github.com/ISADBA/checkllm/internal/probe"
	"github.com/ISADBA/checkllm/internal/provider"
	anthropic "github.com/ISADBA/checkllm/internal/provider/anthropic"
	openai "github.com/ISADBA/checkllm/internal/provider/openai"
	"github.com/ISADBA/checkllm/internal/report"
)

type service struct{}

func NewService() Service {
	return service{}
}

func (service) Run(ctx context.Context, input Input) (Result, error) {
	cfg, err := config.Normalize(config.Config{
		Command:      "run",
		BaseURL:      input.BaseURL,
		APIKey:       input.APIKey,
		Model:        input.Model,
		Provider:     input.Provider,
		BaselinePath: input.BaselinePath,
		OutputPath:   input.OutputPath,
		Timeout:      input.Timeout,
		MaxSamples:   input.MaxSamples,
		EnableStream: input.EnableStream,
		ExpectUsage:  input.ExpectUsage,
	})
	if err != nil {
		return Result{}, err
	}

	base, err := baseline.Load(cfg.BaselinePath)
	if err != nil {
		return Result{}, fmt.Errorf("load baseline: %w", err)
	}

	client, err := buildProviderClient(cfg)
	if err != nil {
		return Result{}, fmt.Errorf("build provider client: %w", err)
	}

	results, err := probe.ExecuteAll(ctx, client, probe.DefaultCatalog(cfg.Provider, cfg.Model, cfg.EnableStream, cfg.MaxSamples), cfg.Timeout)
	if err != nil {
		return Result{}, fmt.Errorf("execute probes: %w", err)
	}

	scores := metric.Calculate(metric.Input{
		Provider:      cfg.Provider,
		Model:         cfg.Model,
		ProbeResults:  results,
		Baseline:      base,
		EnableStream:  cfg.EnableStream,
		ExpectedUsage: cfg.ExpectUsage,
	})

	historyDir := strings.TrimSpace(input.HistoryDir)
	if historyDir == "" {
		historyDir = cfg.HistoryDir()
	}
	historyReports, err := history.LoadDir(historyDir, cfg.BaseURL, cfg.Model)
	if err != nil {
		return Result{}, fmt.Errorf("load history: %w", err)
	}

	judgement := judge.Interpret(judge.Input{
		Config:   cfg,
		Baseline: base,
		Scores:   scores,
		History:  historyReports,
	})

	run := report.BuildRunReport(report.BuildInput{
		Config:       cfg,
		Baseline:     base,
		ProbeResults: results,
		Scores:       scores,
		Judgement:    judgement,
	})

	result := Result{
		Summary:        projectSummary(run),
		RunReport:      run,
		ArchivePath:    cfg.OutputPath,
		UserReportPath: cfg.UserReportPath(),
	}

	if !input.WriteReport {
		result.ArchivePath = ""
		result.UserReportPath = ""
		return result, nil
	}

	if err := report.WriteArchiveMarkdown(cfg.OutputPath, run); err != nil {
		return Result{}, fmt.Errorf("write run archive: %w", err)
	}
	if err := report.WriteUserMarkdown(cfg.UserReportPath(), run); err != nil {
		return Result{}, fmt.Errorf("write user report: %w", err)
	}
	return result, nil
}

func buildProviderClient(cfg config.Config) (provider.Client, error) {
	switch cfg.Provider {
	case "openai":
		return openai.NewClient(cfg.BaseURL, cfg.APIKey, cfg.Model), nil
	case "anthropic":
		return anthropic.NewClient(cfg.BaseURL, cfg.APIKey, cfg.Model), nil
	default:
		return nil, fmt.Errorf("unsupported provider %q", cfg.Provider)
	}
}

func projectSummary(run report.RunReport) Summary {
	statuses := make(map[string]string, len(run.Judgement.Statuses))
	for k, v := range run.Judgement.Statuses {
		statuses[k] = v
	}
	return Summary{
		RunAt:      run.RunAt,
		Provider:   run.Config.Provider,
		Model:      run.Config.Model,
		BaseURL:    strings.TrimRight(run.Config.BaseURL, "/"),
		Conclusion: run.Judgement.Conclusion,
		Scores: ScoresSummary{
			Risk:         float64(run.Scores.OverallRiskScore),
			Protocol:     float64(run.Scores.ProtocolConformityScore),
			Stream:       float64(run.Scores.StreamConformityScore),
			Usage:        float64(run.Scores.UsageConsistencyScore),
			Fingerprint:  float64(run.Scores.BehaviorFingerprintScore),
			Capability:   float64(run.Scores.CapabilityToolScore),
			Tier:         float64(run.Scores.TierFidelityScore),
			Route:        float64(run.Scores.RouteIntegrityScore),
			Functional:   float64(run.Categories.Functional.Score),
			Intelligence: float64(run.Categories.Intelligence.Score),
		},
		Statuses: statuses,
		Thinking: ThinkingSummary{Status: run.Thinking.Status},
		PromptCache: PromptCacheSummary{
			Status: run.PromptCache.Status,
		},
		Network: NetworkSummary{
			AvgLatencyMs:         float64(run.Network.AvgLatencyMs),
			P95LatencyMs:         float64(run.Network.P95LatencyMs),
			AvgFirstByteMs:       float64(run.Network.AvgFirstByteMs),
			AvgOutputTokensPerS:  run.Network.AvgOutputTokensPerS,
			TimeoutCount:         float64(run.Network.TimeoutCount),
			SuccessfulProbeCount: float64(run.Network.SuccessfulProbeCount),
		},
	}
}

func DefaultHistoryDir(groupName, targetName string) string {
	return filepath.Join("data", "exporter-history", config.SanitizeFileName(groupName), config.SanitizeFileName(targetName))
}
