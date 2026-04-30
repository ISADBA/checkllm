package scheduler

import (
	"context"
	"log"
	"sync/atomic"
	"time"

	exporterconfig "github.com/ISADBA/checkllm/internal/exporter/config"
	"github.com/ISADBA/checkllm/internal/exporter/runner"
	"github.com/ISADBA/checkllm/internal/exporter/state"
)

type Scheduler struct {
	cfg   exporterconfig.Config
	store *state.Store
	run   *runner.Runner
	lagNs atomic.Int64
}

func New(cfg exporterconfig.Config, store *state.Store, run *runner.Runner) *Scheduler {
	return &Scheduler{cfg: cfg, store: store, run: run}
}

func (s *Scheduler) Start(ctx context.Context) {
	for _, group := range s.cfg.Groups {
		expr, _ := exporterconfig.ParseCron(group.Schedule)
		go s.loop(ctx, group, expr)
	}
}

func (s *Scheduler) SchedulerLag() time.Duration {
	return time.Duration(s.lagNs.Load())
}

func (s *Scheduler) loop(ctx context.Context, group exporterconfig.GroupConfig, expr exporterconfig.CronExpr) {
	for {
		next := expr.Next(time.Now())
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
				continue
			}
			s.run.Submit(ctx, runner.Job{Group: group, Target: target})
		}
		log.Printf("scheduled group=%s targets=%d", group.Name, len(group.Targets))
	}
}
