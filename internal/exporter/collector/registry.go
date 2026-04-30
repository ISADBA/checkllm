package collector

import (
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ISADBA/checkllm/internal/exporter/state"
)

var metricStatuses = []string{"normal", "mild_deviation", "significant_deviation"}
var thinkingStatuses = []string{"supported_active", "supported_exposed", "not_detected"}
var promptCacheStatuses = []string{"supported_hit", "supported_exposed", "not_detected", "not_applicable"}
var runResults = []string{"success", "failed", "skipped"}
var failureTypes = []string{"timeout", "network", "rate_limit", "auth", "config", "parse", "unknown"}
var skipReasons = []string{"already_running", "disabled", "scheduler_backpressure"}

type StatsProvider interface {
	RunningJobs() int
	QueueDepth() int
	SchedulerLag() time.Duration
}

type Collector struct {
	store         *state.Store
	stats         StatsProvider
	configGroups  int
	configTargets int
}

func New(store *state.Store, stats StatsProvider, configGroups, configTargets int) *Collector {
	return &Collector{
		store:         store,
		stats:         stats,
		configGroups:  configGroups,
		configTargets: configTargets,
	}
}

func (c *Collector) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	_, _ = w.Write([]byte(c.Render()))
}

func (c *Collector) Render() string {
	var b strings.Builder
	writeMetricDoc(&b, "checkllm_target_up", "gauge", "Whether the last run succeeded.")
	writeMetricDoc(&b, "checkllm_target_last_run_timestamp_seconds", "gauge", "Unix timestamp of the last run start.")
	writeMetricDoc(&b, "checkllm_target_last_success_timestamp_seconds", "gauge", "Unix timestamp of the last successful run.")
	writeMetricDoc(&b, "checkllm_target_last_duration_seconds", "gauge", "Duration of the last run.")
	writeMetricDoc(&b, "checkllm_target_last_risk_score", "gauge", "Last overall risk score.")
	writeMetricDoc(&b, "checkllm_target_last_protocol_score", "gauge", "Last protocol score.")
	writeMetricDoc(&b, "checkllm_target_last_stream_score", "gauge", "Last stream score.")
	writeMetricDoc(&b, "checkllm_target_last_usage_score", "gauge", "Last usage score.")
	writeMetricDoc(&b, "checkllm_target_last_fingerprint_score", "gauge", "Last fingerprint score.")
	writeMetricDoc(&b, "checkllm_target_last_capability_score", "gauge", "Last capability score.")
	writeMetricDoc(&b, "checkllm_target_last_tier_score", "gauge", "Last tier score.")
	writeMetricDoc(&b, "checkllm_target_last_route_score", "gauge", "Last route score.")
	writeMetricDoc(&b, "checkllm_target_last_functional_score", "gauge", "Last functional score.")
	writeMetricDoc(&b, "checkllm_target_last_intelligence_score", "gauge", "Last intelligence score.")
	writeMetricDoc(&b, "checkllm_target_last_avg_latency_ms", "gauge", "Last average latency in milliseconds.")
	writeMetricDoc(&b, "checkllm_target_last_p95_latency_ms", "gauge", "Last p95 latency in milliseconds.")
	writeMetricDoc(&b, "checkllm_target_last_avg_first_byte_ms", "gauge", "Last average first-byte latency in milliseconds.")
	writeMetricDoc(&b, "checkllm_target_last_avg_output_tokens_per_s", "gauge", "Last average output throughput.")
	writeMetricDoc(&b, "checkllm_target_last_timeout_count", "gauge", "Last timeout count.")
	writeMetricDoc(&b, "checkllm_target_last_successful_probe_count", "gauge", "Last successful probe count.")
	writeMetricDoc(&b, "checkllm_target_conclusion", "gauge", "One-hot current conclusion.")
	writeMetricDoc(&b, "checkllm_target_metric_status", "gauge", "One-hot current metric status.")
	writeMetricDoc(&b, "checkllm_target_thinking_status", "gauge", "One-hot thinking status.")
	writeMetricDoc(&b, "checkllm_target_prompt_cache_status", "gauge", "One-hot prompt cache status.")
	writeMetricDoc(&b, "checkllm_runs_total", "counter", "Total number of runs.")
	writeMetricDoc(&b, "checkllm_run_failures_total", "counter", "Total failures by error type.")
	writeMetricDoc(&b, "checkllm_run_retries_total", "counter", "Total retry attempts.")
	writeMetricDoc(&b, "checkllm_run_skips_total", "counter", "Total skipped runs.")
	writeMetricDoc(&b, "checkllm_exporter_config_groups", "gauge", "Configured group count.")
	writeMetricDoc(&b, "checkllm_exporter_config_targets", "gauge", "Configured target count.")
	writeMetricDoc(&b, "checkllm_exporter_running_jobs", "gauge", "Running jobs.")
	writeMetricDoc(&b, "checkllm_exporter_queue_depth", "gauge", "Queue depth.")
	writeMetricDoc(&b, "checkllm_exporter_scheduler_lag_seconds", "gauge", "Latest scheduler lag in seconds.")

	states := c.store.Snapshot()
	sort.Slice(states, func(i, j int) bool {
		if states[i].Labels.Group == states[j].Labels.Group {
			return states[i].Labels.Target < states[j].Labels.Target
		}
		return states[i].Labels.Group < states[j].Labels.Group
	})

	for _, snapshot := range states {
		baseLabels := labelMap(snapshot.Labels)
		writeSample(&b, "checkllm_target_up", baseLabels, boolFloat(snapshot.LastUp))
		writeSample(&b, "checkllm_target_last_run_timestamp_seconds", baseLabels, timeFloat(snapshot.LastRunAt))
		writeSample(&b, "checkllm_target_last_success_timestamp_seconds", baseLabels, timeFloat(snapshot.LastSuccessAt))
		writeSample(&b, "checkllm_target_last_duration_seconds", baseLabels, snapshot.LastDuration.Seconds())
		writeSample(&b, "checkllm_target_last_risk_score", baseLabels, snapshot.LastRiskScore)
		writeSample(&b, "checkllm_target_last_protocol_score", baseLabels, snapshot.LastProtocolScore)
		writeSample(&b, "checkllm_target_last_stream_score", baseLabels, snapshot.LastStreamScore)
		writeSample(&b, "checkllm_target_last_usage_score", baseLabels, snapshot.LastUsageScore)
		writeSample(&b, "checkllm_target_last_fingerprint_score", baseLabels, snapshot.LastFingerprintScore)
		writeSample(&b, "checkllm_target_last_capability_score", baseLabels, snapshot.LastCapabilityScore)
		writeSample(&b, "checkllm_target_last_tier_score", baseLabels, snapshot.LastTierScore)
		writeSample(&b, "checkllm_target_last_route_score", baseLabels, snapshot.LastRouteScore)
		writeSample(&b, "checkllm_target_last_functional_score", baseLabels, snapshot.LastFunctionalScore)
		writeSample(&b, "checkllm_target_last_intelligence_score", baseLabels, snapshot.LastIntelligenceScore)
		writeSample(&b, "checkllm_target_last_avg_latency_ms", baseLabels, snapshot.LastAvgLatencyMs)
		writeSample(&b, "checkllm_target_last_p95_latency_ms", baseLabels, snapshot.LastP95LatencyMs)
		writeSample(&b, "checkllm_target_last_avg_first_byte_ms", baseLabels, snapshot.LastAvgFirstByteMs)
		writeSample(&b, "checkllm_target_last_avg_output_tokens_per_s", baseLabels, snapshot.LastAvgOutputTokensPerS)
		writeSample(&b, "checkllm_target_last_timeout_count", baseLabels, snapshot.LastTimeoutCount)
		writeSample(&b, "checkllm_target_last_successful_probe_count", baseLabels, snapshot.LastSuccessfulProbeCount)
		if snapshot.LastConclusion != "" {
			writeSample(&b, "checkllm_target_conclusion", mergeLabels(baseLabels, map[string]string{"conclusion": snapshot.LastConclusion}), 1)
		}
		for metric, current := range snapshot.MetricStatuses {
			for _, status := range metricStatuses {
				value := 0.0
				if current == status {
					value = 1
				}
				writeSample(&b, "checkllm_target_metric_status", mergeLabels(baseLabels, map[string]string{"metric": metric, "status": status}), value)
			}
		}
		for _, status := range thinkingStatuses {
			value := 0.0
			if snapshot.ThinkingStatus == status {
				value = 1
			}
			writeSample(&b, "checkllm_target_thinking_status", mergeLabels(baseLabels, map[string]string{"status": status}), value)
		}
		for _, status := range promptCacheStatuses {
			value := 0.0
			if snapshot.PromptCacheStatus == status {
				value = 1
			}
			writeSample(&b, "checkllm_target_prompt_cache_status", mergeLabels(baseLabels, map[string]string{"status": status}), value)
		}
		for _, result := range runResults {
			writeSample(&b, "checkllm_runs_total", mergeLabels(baseLabels, map[string]string{"result": result}), float64(snapshot.RunsTotal[result]))
		}
		for _, errorType := range failureTypes {
			writeSample(&b, "checkllm_run_failures_total", mergeLabels(baseLabels, map[string]string{"error_type": errorType}), float64(snapshot.FailuresTotal[errorType]))
		}
		writeSample(&b, "checkllm_run_retries_total", baseLabels, float64(snapshot.RetriesTotal))
		for _, reason := range skipReasons {
			writeSample(&b, "checkllm_run_skips_total", mergeLabels(baseLabels, map[string]string{"reason": reason}), float64(snapshot.SkipsTotal[reason]))
		}
	}

	writeSample(&b, "checkllm_exporter_config_groups", nil, float64(c.configGroups))
	writeSample(&b, "checkllm_exporter_config_targets", nil, float64(c.configTargets))
	writeSample(&b, "checkllm_exporter_running_jobs", nil, float64(c.stats.RunningJobs()))
	writeSample(&b, "checkllm_exporter_queue_depth", nil, float64(c.stats.QueueDepth()))
	writeSample(&b, "checkllm_exporter_scheduler_lag_seconds", nil, c.stats.SchedulerLag().Seconds())
	return b.String()
}

func writeMetricDoc(b *strings.Builder, name, typ, help string) {
	fmt.Fprintf(b, "# HELP %s %s\n", name, help)
	fmt.Fprintf(b, "# TYPE %s %s\n", name, typ)
}

func writeSample(b *strings.Builder, name string, labels map[string]string, value float64) {
	b.WriteString(name)
	b.WriteString(renderLabels(labels))
	b.WriteByte(' ')
	b.WriteString(strconv.FormatFloat(value, 'f', -1, 64))
	b.WriteByte('\n')
}

func renderLabels(labels map[string]string) string {
	if len(labels) == 0 {
		return ""
	}
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var parts []string
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf(`%s="%s"`, key, escapeLabelValue(labels[key])))
	}
	return "{" + strings.Join(parts, ",") + "}"
}

func escapeLabelValue(v string) string {
	replacer := strings.NewReplacer(`\`, `\\`, "\n", `\n`, `"`, `\"`)
	return replacer.Replace(v)
}

func labelMap(labels state.MetricLabels) map[string]string {
	return map[string]string{
		"group":    labels.Group,
		"target":   labels.Target,
		"provider": labels.Provider,
		"model":    labels.Model,
		"env":      labels.Env,
		"vendor":   labels.Vendor,
		"route":    labels.Route,
		"region":   labels.Region,
		"owner":    labels.Owner,
		"tier":     labels.Tier,
	}
}

func mergeLabels(base map[string]string, extra map[string]string) map[string]string {
	out := make(map[string]string, len(base)+len(extra))
	for k, v := range base {
		out[k] = v
	}
	for k, v := range extra {
		out[k] = v
	}
	return out
}

func boolFloat(v bool) float64 {
	if v {
		return 1
	}
	return 0
}

func timeFloat(v time.Time) float64 {
	if v.IsZero() {
		return 0
	}
	return float64(v.Unix())
}
