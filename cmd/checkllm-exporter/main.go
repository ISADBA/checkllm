package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ISADBA/checkllm/internal/app/runcheck"
	exportercollector "github.com/ISADBA/checkllm/internal/exporter/collector"
	exporterconfig "github.com/ISADBA/checkllm/internal/exporter/config"
	"github.com/ISADBA/checkllm/internal/exporter/runner"
	"github.com/ISADBA/checkllm/internal/exporter/scheduler"
	"github.com/ISADBA/checkllm/internal/exporter/secrets"
	"github.com/ISADBA/checkllm/internal/exporter/server"
	"github.com/ISADBA/checkllm/internal/exporter/state"
)

type exporterStats struct {
	run   *runner.Runner
	sched *scheduler.Scheduler
}

func (s exporterStats) RunningJobs() int            { return s.run.RunningJobs() }
func (s exporterStats) QueueDepth() int             { return s.run.QueueDepth() }
func (s exporterStats) SchedulerLag() time.Duration { return s.sched.SchedulerLag() }

func main() {
	var configPath string
	var logLevel string
	flag.StringVar(&configPath, "config", "", "exporter config file")
	flag.StringVar(&logLevel, "log-level", "info", "log level")
	flag.Parse()

	if configPath == "" {
		log.Fatal("missing required --config")
	}
	_ = logLevel

	cfg, err := exporterconfig.Load(configPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	store := state.NewStore()
	targetCount := 0
	for _, group := range cfg.Groups {
		for _, target := range group.Targets {
			labels := exporterconfig.MergeLabels(group.Labels, target.Labels)
			store.EnsureTarget(state.TargetKey{Group: group.Name, Target: target.TargetName}, state.MetricLabels{
				Group:    group.Name,
				Target:   target.TargetName,
				Provider: target.Provider,
				Model:    target.Model,
				Env:      labels["env"],
				Vendor:   labels["vendor"],
				Route:    labels["route"],
				Region:   labels["region"],
				Owner:    labels["owner"],
				Tier:     labels["tier"],
			})
			targetCount++
		}
	}

	runSvc := runcheck.NewService()
	resolver := secrets.NewResolver()
	run := runner.New(runSvc, resolver, store, cfg)
	sched := scheduler.New(cfg, store, run)
	metrics := exportercollector.New(store, exporterStats{run: run, sched: sched}, len(cfg.Groups), targetCount)
	httpServer := server.New(cfg.Global.ListenAddr, metrics)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	sched.Start(ctx)
	httpServer.MarkReady()
	go func() {
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("serve http: %v", err)
		}
	}()

	log.Printf("checkllm exporter listening on %s", cfg.Global.ListenAddr)
	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown http server: %v", err)
	}
}
