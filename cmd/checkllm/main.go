package main

import (
	"context"
	"fmt"
	"os"

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

func main() {
	cfg, err := config.Parse(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	ctx := context.Background()

	base, err := baseline.Load(cfg.BaselinePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load baseline: %v\n", err)
		os.Exit(1)
	}

	client, err := buildProviderClient(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "build provider client: %v\n", err)
		os.Exit(1)
	}
	results, err := probe.ExecuteAll(ctx, client, probe.DefaultCatalog(cfg.Provider, cfg.Model, cfg.EnableStream, cfg.MaxSamples), cfg.Timeout)
	if err != nil {
		fmt.Fprintf(os.Stderr, "execute probes: %v\n", err)
		os.Exit(1)
	}

	scores := metric.Calculate(metric.Input{
		Provider:      cfg.Provider,
		Model:         cfg.Model,
		ProbeResults:  results,
		Baseline:      base,
		EnableStream:  cfg.EnableStream,
		ExpectedUsage: cfg.ExpectUsage,
	})

	historyReports, err := history.LoadDir(cfg.HistoryDir(), cfg.BaseURL, cfg.Model)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load history: %v\n", err)
		os.Exit(1)
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

	if err := report.WriteArchiveMarkdown(cfg.OutputPath, run); err != nil {
		fmt.Fprintf(os.Stderr, "write run archive: %v\n", err)
		os.Exit(1)
	}

	userReportPath := cfg.UserReportPath()
	if err := report.WriteUserMarkdown(userReportPath, run); err != nil {
		fmt.Fprintf(os.Stderr, "write user report: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("run archive written to %s\n", cfg.OutputPath)
	fmt.Printf("user report written to %s\n", userReportPath)
	fmt.Printf("protocol=%d usage=%d fingerprint=%d tier=%d route=%d risk=%d conclusion=%s\n",
		scores.ProtocolConformityScore,
		scores.UsageConsistencyScore,
		scores.BehaviorFingerprintScore,
		scores.TierFidelityScore,
		scores.RouteIntegrityScore,
		scores.OverallRiskScore,
		judgement.Conclusion,
	)
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
