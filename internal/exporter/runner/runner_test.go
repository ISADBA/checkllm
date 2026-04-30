package runner

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ISADBA/checkllm/internal/app/runcheck"
	exporterconfig "github.com/ISADBA/checkllm/internal/exporter/config"
	"github.com/ISADBA/checkllm/internal/exporter/state"
)

type fakeService struct {
	results []error
	calls   int
}

func (f *fakeService) Run(_ context.Context, _ runcheck.Input) (runcheck.Result, error) {
	idx := f.calls
	f.calls++
	if idx < len(f.results) && f.results[idx] != nil {
		return runcheck.Result{}, f.results[idx]
	}
	return runcheck.Result{
		Summary: runcheck.Summary{
			RunAt:      time.Now(),
			Provider:   "openai",
			Model:      "gpt-5.4",
			BaseURL:    "https://api.openai.com/v1",
			Conclusion: "high_confidence_official_compatible",
			Scores: runcheck.ScoresSummary{
				Risk:         12,
				Protocol:     95,
				Usage:        94,
				Fingerprint:  93,
				Tier:         92,
				Route:        91,
				Capability:   90,
				Functional:   94,
				Intelligence: 92,
			},
			Statuses: map[string]string{"protocol_conformity_score": "normal"},
			Thinking: runcheck.ThinkingSummary{Status: "not_detected"},
			PromptCache: runcheck.PromptCacheSummary{
				Status: "not_detected",
			},
		},
	}, nil
}

type fakeResolver struct{}

func (fakeResolver) Resolve(target exporterconfig.TargetConfig) (string, error) {
	return target.APIKey, nil
}

func TestRunnerRetriesRetryableErrors(t *testing.T) {
	store := state.NewStore()
	key := state.TargetKey{Group: "g", Target: "t"}
	store.EnsureTarget(key, state.MetricLabels{Group: "g", Target: "t", Provider: "openai", Model: "gpt-5.4"})
	service := &fakeService{results: []error{context.DeadlineExceeded, nil}}
	cfg := exporterconfig.Config{
		Global: exporterconfig.GlobalConfig{GlobalMaxConcurrency: 1},
		Groups: []exporterconfig.GroupConfig{{
			Name:           "g",
			Timeout:        time.Second,
			MaxConcurrency: 1,
			Retry:          exporterconfig.RetryConfig{MaxAttempts: 2},
		}},
	}
	r := New(service, fakeResolver{}, store, cfg)
	r.Submit(context.Background(), Job{
		Group: cfg.Groups[0],
		Target: exporterconfig.TargetConfig{
			TargetName:   "t",
			APIKey:       "secret",
			Provider:     "openai",
			BaseURL:      "https://api.openai.com/v1",
			Model:        "gpt-5.4",
			BaselinePath: "/tmp/base.md",
		},
	})

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		snapshot := store.Snapshot()
		if len(snapshot) == 1 && snapshot[0].RunsTotal["success"] == 1 {
			if snapshot[0].RetriesTotal != 1 {
				t.Fatalf("expected one retry, got %d", snapshot[0].RetriesTotal)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("runner did not finish in time")
}

func TestClassifyError(t *testing.T) {
	if got := classifyError(errors.New("429 too many requests")); got != "rate_limit" {
		t.Fatalf("unexpected rate limit classification: %s", got)
	}
	if got := classifyError(errors.New("load baseline: broken")); got != "config" {
		t.Fatalf("unexpected config classification: %s", got)
	}
}
