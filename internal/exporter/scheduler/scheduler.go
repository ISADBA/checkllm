package scheduler

import (
	"context"
	"sync/atomic"
	"time"

	exporterconfig "github.com/ISADBA/checkllm/internal/exporter/config"
	"github.com/ISADBA/checkllm/internal/exporter/logging"
	"github.com/ISADBA/checkllm/internal/exporter/runner"
	"github.com/ISADBA/checkllm/internal/exporter/state"
)

type Scheduler struct {
	cfg   exporterconfig.Config
	store *state.Store
	run   *runner.Runner
	log   logging.Logger
	lagNs atomic.Int64
}

func New(cfg exporterconfig.Config, store *state.Store, run *runner.Runner, logger logging.Logger) *Scheduler {
	return &Scheduler{cfg: cfg, store: store, run: run, log: logger}
}

func (s *Scheduler) Start(ctx context.Context) {
	for _, group := range s.cfg.Groups {
		expr, _ := exporterconfig.ParseCron(group.Schedule)
		next := expr.Next(time.Now())
		s.log.Infof("scheduler registered: group=%s schedule=%q next_run=%s timeout=%s max_concurrency=%d targets=%d", group.Name, group.Schedule, next.Format(time.RFC3339), group.Timeout, group.MaxConcurrency, len(group.Targets))
		go s.loop(ctx, group, expr)
	}
}

func (s *Scheduler) SchedulerLag() time.Duration {
	return time.Duration(s.lagNs.Load())
}

func (s *Scheduler) loop(ctx context.Context, group exporterconfig.GroupConfig, expr exporterconfig.CronExpr) {
	for {
		next := expr.Next(time.Now())
		s.log.Debugf("scheduler waiting: group=%s next_run=%s wait=%s", group.Name, next.Format(time.RFC3339), time.Until(next).Round(time.Second))
		timer := time.NewTimer(time.Until(next))
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}

		lag := time.Since(next)
		if lag < 0 {
			lag = 0
		}
		s.lagNs.Store(lag.Nanoseconds())
		for _, target := range group.Targets {
			key := state.TargetKey{Group: group.Name, Target: target.TargetName}
			if !target.Enabled {
				s.store.RecordSkip(key, "disabled")
				s.log.Infof("target skipped: group=%s target=%s reason=disabled", group.Name, target.TargetName)
				continue
			}
			s.run.Submit(ctx, runner.Job{Group: group, Target: target})
		}
		s.log.Infof("scheduled group=%s targets=%d lag=%s", group.Name, len(group.Targets), lag.Round(time.Millisecond))
	}
}
