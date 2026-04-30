package runner

import (
	"context"
	"errors"
	"net"
	"net/url"
	"strings"
	"sync/atomic"
	"time"

	"github.com/ISADBA/checkllm/internal/app/runcheck"
	exporterconfig "github.com/ISADBA/checkllm/internal/exporter/config"
	"github.com/ISADBA/checkllm/internal/exporter/secrets"
	"github.com/ISADBA/checkllm/internal/exporter/state"
)

type Job struct {
	Group  exporterconfig.GroupConfig
	Target exporterconfig.TargetConfig
}

type Runner struct {
	service     runcheck.Service
	resolver    secrets.Resolver
	store       *state.Store
	globalSem   chan struct{}
	groupSem    map[string]chan struct{}
	runningJobs atomic.Int64
	queueDepth  atomic.Int64
}

func New(service runcheck.Service, resolver secrets.Resolver, store *state.Store, cfg exporterconfig.Config) *Runner {
	groupSem := make(map[string]chan struct{}, len(cfg.Groups))
	for _, group := range cfg.Groups {
		groupSem[group.Name] = make(chan struct{}, group.MaxConcurrency)
	}
	return &Runner{
		service:   service,
		resolver:  resolver,
		store:     store,
		globalSem: make(chan struct{}, cfg.Global.GlobalMaxConcurrency),
		groupSem:  groupSem,
	}
}

func (r *Runner) Submit(ctx context.Context, job Job) {
	key := state.TargetKey{Group: job.Group.Name, Target: job.Target.TargetName}
	startedAt := time.Now()
	if !r.store.MarkRunning(key, startedAt) {
		r.store.RecordSkip(key, "already_running")
		return
	}
	r.queueDepth.Add(1)
	go r.run(ctx, key, job, startedAt)
}

func (r *Runner) run(ctx context.Context, key state.TargetKey, job Job, startedAt time.Time) {
	r.runningJobs.Add(1)
	defer r.runningJobs.Add(-1)
	defer r.queueDepth.Add(-1)

	summary, retries, failureType, err := r.execute(ctx, job)
	duration := time.Since(startedAt)
	if err != nil {
		r.store.FinishFailure(key, state.FailureUpdate{
			Duration:  duration,
			ErrorType: failureType,
			Retries:   retries,
		})
		return
	}

	r.store.FinishSuccess(key, state.SuccessUpdate{
		Duration: duration,
		Summary:  summary,
		Retries:  retries,
	})
}

func (r *Runner) execute(ctx context.Context, job Job) (runcheck.Summary, uint64, string, error) {
	groupSem := r.groupSem[job.Group.Name]
	if err := acquire(ctx, r.globalSem); err != nil {
		return runcheck.Summary{}, 0, classifyError(err), err
	}
	defer release(r.globalSem)
	if err := acquire(ctx, groupSem); err != nil {
		return runcheck.Summary{}, 0, classifyError(err), err
	}
	defer release(groupSem)

	apiKey, err := r.resolver.Resolve(job.Target)
	if err != nil {
		return runcheck.Summary{}, 0, "config", err
	}

	attempts := job.Group.Retry.MaxAttempts
	if attempts < 1 {
		attempts = 1
	}
	var retries uint64
	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		runCtx, cancel := context.WithTimeout(ctx, job.Group.Timeout)
		result, runErr := r.service.Run(runCtx, runcheck.Input{
			BaseURL:      job.Target.BaseURL,
			APIKey:       apiKey,
			Model:        job.Target.Model,
			Provider:     job.Target.Provider,
			BaselinePath: job.Target.BaselinePath,
			Timeout:      job.Group.Timeout,
			MaxSamples:   2,
			EnableStream: true,
			ExpectUsage:  true,
			HistoryDir:   runcheck.DefaultHistoryDir(job.Group.Name, job.Target.TargetName),
			WriteReport:  false,
		})
		cancel()
		if runErr == nil {
			if err := validateSummary(result.Summary); err != nil {
				lastErr = err
				break
			}
			return result.Summary, retries, "", nil
		}
		lastErr = runErr
		if attempt >= attempts || !isRetryable(runErr) {
			break
		}
		retries++
		if job.Group.Retry.Backoff > 0 {
			timer := time.NewTimer(job.Group.Retry.Backoff)
			select {
			case <-ctx.Done():
				timer.Stop()
				return runcheck.Summary{}, retries, classifyError(ctx.Err()), ctx.Err()
			case <-timer.C:
			}
		}
	}
	return runcheck.Summary{}, retries, classifyError(lastErr), lastErr
}

func validateSummary(summary runcheck.Summary) error {
	if summary.Provider == "" || summary.Model == "" || summary.Conclusion == "" {
		return errors.New("parse summary: missing required fields")
	}
	return nil
}

func (r *Runner) RunningJobs() int {
	return int(r.runningJobs.Load())
}

func (r *Runner) QueueDepth() int {
	return int(r.queueDepth.Load())
}

func acquire(ctx context.Context, sem chan struct{}) error {
	select {
	case sem <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func release(sem chan struct{}) {
	select {
	case <-sem:
	default:
	}
}

func isRetryable(err error) bool {
	switch classifyError(err) {
	case "timeout", "network", "rate_limit":
		return true
	default:
		return false
	}
}

func classifyError(err error) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, context.DeadlineExceeded) || strings.Contains(strings.ToLower(err.Error()), "deadline exceeded") {
		return "timeout"
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return "network"
	}
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return "network"
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "429"), strings.Contains(msg, "rate limit"):
		return "rate_limit"
	case strings.Contains(msg, "401"), strings.Contains(msg, "403"), strings.Contains(msg, "unauthorized"), strings.Contains(msg, "forbidden"):
		return "auth"
	case strings.Contains(msg, "baseline"), strings.Contains(msg, "missing required"), strings.Contains(msg, "unsupported provider"), strings.Contains(msg, "api_key_ref"), strings.Contains(msg, "load history"):
		return "config"
	case strings.Contains(msg, "parse"), strings.Contains(msg, "json"):
		return "parse"
	default:
		return "unknown"
	}
}
