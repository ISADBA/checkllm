package runner

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"sync/atomic"
	"time"

	"github.com/ISADBA/checkllm/internal/app/runcheck"
	exporterconfig "github.com/ISADBA/checkllm/internal/exporter/config"
	"github.com/ISADBA/checkllm/internal/exporter/logging"
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
	log         logging.Logger
	runningJobs atomic.Int64
	queueDepth  atomic.Int64
}

func New(service runcheck.Service, resolver secrets.Resolver, store *state.Store, cfg exporterconfig.Config, logger logging.Logger) *Runner {
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
		log:       logger,
	}
}

func (r *Runner) Submit(ctx context.Context, job Job) {
	key := state.TargetKey{Group: job.Group.Name, Target: job.Target.TargetName}
	startedAt := time.Now()
	if !r.store.MarkRunning(key, startedAt) {
		r.store.RecordSkip(key, "already_running")
		r.log.Warnf("target skipped: group=%s target=%s reason=already_running", job.Group.Name, job.Target.TargetName)
		return
	}
	depth := r.queueDepth.Add(1)
	r.log.Infof("target queued: group=%s target=%s model=%s base_url=%s queue_depth=%d", job.Group.Name, job.Target.TargetName, job.Target.Model, job.Target.BaseURL, depth)
	go r.run(ctx, key, job, startedAt)
}

func (r *Runner) run(ctx context.Context, key state.TargetKey, job Job, startedAt time.Time) {
	running := r.runningJobs.Add(1)
	r.log.Infof("target started: group=%s target=%s running_jobs=%d", job.Group.Name, job.Target.TargetName, running)
	defer func() {
		r.runningJobs.Add(-1)
		r.queueDepth.Add(-1)
	}()

	summary, retries, failureType, err := r.execute(ctx, job)
	duration := time.Since(startedAt)
	if err != nil {
		r.store.FinishFailure(key, state.FailureUpdate{
			Duration:     duration,
			ErrorType:    failureType,
			ErrorMessage: err.Error(),
			Retries:      retries,
		})
		r.log.Errorf("target failed: group=%s target=%s duration=%s retries=%d error_type=%s err=%v", job.Group.Name, job.Target.TargetName, duration.Round(time.Millisecond), retries, failureType, err)
		return
	}

	r.store.FinishSuccess(key, state.SuccessUpdate{
		Duration: duration,
		Summary:  summary,
		Retries:  retries,
	})
	r.log.Infof("target succeeded: group=%s target=%s duration=%s retries=%d conclusion=%s risk=%.0f protocol=%.0f usage=%.0f tier=%.0f route=%.0f", job.Group.Name, job.Target.TargetName, duration.Round(time.Millisecond), retries, summary.Conclusion, summary.Scores.Risk, summary.Scores.Protocol, summary.Scores.Usage, summary.Scores.Tier, summary.Scores.Route)
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
	r.log.Debugf("target execution config: group=%s target=%s timeout=%s attempts=%d backoff=%s", job.Group.Name, job.Target.TargetName, job.Group.Timeout, attempts, job.Group.Retry.Backoff)
	var retries uint64
	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		r.log.Infof("target attempt: group=%s target=%s attempt=%d/%d", job.Group.Name, job.Target.TargetName, attempt, attempts)
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
				r.log.Errorf("target summary invalid: group=%s target=%s attempt=%d err=%v", job.Group.Name, job.Target.TargetName, attempt, err)
				break
			}
			return result.Summary, retries, "", nil
		}
		lastErr = runErr
		r.log.Warnf("target attempt failed: group=%s target=%s attempt=%d/%d error_type=%s err=%v", job.Group.Name, job.Target.TargetName, attempt, attempts, classifyError(runErr), runErr)
		if attempt >= attempts || !isRetryable(runErr) {
			break
		}
		retries++
		if job.Group.Retry.Backoff > 0 {
			r.log.Infof("target retry scheduled: group=%s target=%s next_attempt=%d backoff=%s reason=%s", job.Group.Name, job.Target.TargetName, attempt+1, job.Group.Retry.Backoff, classifyError(runErr))
			timer := time.NewTimer(job.Group.Retry.Backoff)
			select {
			case <-ctx.Done():
				timer.Stop()
				return runcheck.Summary{}, retries, classifyError(ctx.Err()), ctx.Err()
			case <-timer.C:
			}
		}
	}
	return runcheck.Summary{}, retries, classifyError(lastErr), fmt.Errorf("run check failed after %d attempt(s): %w", attempts, lastErr)
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
