package server

import (
	"encoding/json"
	"net/http"
	"sort"

	"github.com/ISADBA/checkllm/internal/exporter/state"
)

type debugTarget struct {
	Group                    string            `json:"group"`
	Target                   string            `json:"target"`
	Provider                 string            `json:"provider"`
	Model                    string            `json:"model"`
	BaseLabels               map[string]string `json:"labels"`
	LastRunAt                string            `json:"last_run_at,omitempty"`
	LastSuccessAt            string            `json:"last_success_at,omitempty"`
	LastDurationSeconds      float64           `json:"last_duration_seconds"`
	LastUp                   bool              `json:"last_up"`
	LastErrorType            string            `json:"last_error_type,omitempty"`
	LastErrorMessage         string            `json:"last_error_message,omitempty"`
	LastConclusion           string            `json:"last_conclusion,omitempty"`
	LastRiskScore            float64           `json:"last_risk_score"`
	LastProtocolScore        float64           `json:"last_protocol_score"`
	LastStreamScore          float64           `json:"last_stream_score"`
	LastUsageScore           float64           `json:"last_usage_score"`
	LastFingerprintScore     float64           `json:"last_fingerprint_score"`
	LastCapabilityScore      float64           `json:"last_capability_score"`
	LastTierScore            float64           `json:"last_tier_score"`
	LastRouteScore           float64           `json:"last_route_score"`
	LastFunctionalScore      float64           `json:"last_functional_score"`
	LastIntelligenceScore    float64           `json:"last_intelligence_score"`
	LastAvgLatencyMs         float64           `json:"last_avg_latency_ms"`
	LastP95LatencyMs         float64           `json:"last_p95_latency_ms"`
	LastAvgFirstByteMs       float64           `json:"last_avg_first_byte_ms"`
	LastAvgOutputTokensPerS  float64           `json:"last_avg_output_tokens_per_s"`
	LastTimeoutCount         float64           `json:"last_timeout_count"`
	LastSuccessfulProbeCount float64           `json:"last_successful_probe_count"`
	MetricStatuses           map[string]string `json:"metric_statuses,omitempty"`
	ThinkingStatus           string            `json:"thinking_status,omitempty"`
	PromptCacheStatus        string            `json:"prompt_cache_status,omitempty"`
	RunsTotal                map[string]uint64 `json:"runs_total,omitempty"`
	FailuresTotal            map[string]uint64 `json:"failures_total,omitempty"`
	RetriesTotal             uint64            `json:"retries_total"`
	SkipsTotal               map[string]uint64 `json:"skips_total,omitempty"`
	Running                  bool              `json:"running"`
}

func debugTargetsHandler(store *state.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		snapshots := store.Snapshot()
		sort.Slice(snapshots, func(i, j int) bool {
			if snapshots[i].Labels.Group == snapshots[j].Labels.Group {
				return snapshots[i].Labels.Target < snapshots[j].Labels.Target
			}
			return snapshots[i].Labels.Group < snapshots[j].Labels.Group
		})

		targets := make([]debugTarget, 0, len(snapshots))
		for _, snapshot := range snapshots {
			targets = append(targets, debugTarget{
				Group:                    snapshot.Labels.Group,
				Target:                   snapshot.Labels.Target,
				Provider:                 snapshot.Labels.Provider,
				Model:                    snapshot.Labels.Model,
				BaseLabels:               map[string]string{"env": snapshot.Labels.Env, "vendor": snapshot.Labels.Vendor, "route": snapshot.Labels.Route, "region": snapshot.Labels.Region, "owner": snapshot.Labels.Owner, "tier": snapshot.Labels.Tier},
				LastRunAt:                formatTime(snapshot.LastRunAt),
				LastSuccessAt:            formatTime(snapshot.LastSuccessAt),
				LastDurationSeconds:      snapshot.LastDuration.Seconds(),
				LastUp:                   snapshot.LastUp,
				LastErrorType:            snapshot.LastErrorType,
				LastErrorMessage:         snapshot.LastErrorMessage,
				LastConclusion:           snapshot.LastConclusion,
				LastRiskScore:            snapshot.LastRiskScore,
				LastProtocolScore:        snapshot.LastProtocolScore,
				LastStreamScore:          snapshot.LastStreamScore,
				LastUsageScore:           snapshot.LastUsageScore,
				LastFingerprintScore:     snapshot.LastFingerprintScore,
				LastCapabilityScore:      snapshot.LastCapabilityScore,
				LastTierScore:            snapshot.LastTierScore,
				LastRouteScore:           snapshot.LastRouteScore,
				LastFunctionalScore:      snapshot.LastFunctionalScore,
				LastIntelligenceScore:    snapshot.LastIntelligenceScore,
				LastAvgLatencyMs:         snapshot.LastAvgLatencyMs,
				LastP95LatencyMs:         snapshot.LastP95LatencyMs,
				LastAvgFirstByteMs:       snapshot.LastAvgFirstByteMs,
				LastAvgOutputTokensPerS:  snapshot.LastAvgOutputTokensPerS,
				LastTimeoutCount:         snapshot.LastTimeoutCount,
				LastSuccessfulProbeCount: snapshot.LastSuccessfulProbeCount,
				MetricStatuses:           snapshot.MetricStatuses,
				ThinkingStatus:           snapshot.ThinkingStatus,
				PromptCacheStatus:        snapshot.PromptCacheStatus,
				RunsTotal:                snapshot.RunsTotal,
				FailuresTotal:            snapshot.FailuresTotal,
				RetriesTotal:             snapshot.RetriesTotal,
				SkipsTotal:               snapshot.SkipsTotal,
				Running:                  snapshot.Running,
			})
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		_ = enc.Encode(map[string]any{"targets": targets})
	}
}

func formatTime(t interface {
	IsZero() bool
	Format(string) string
}) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02T15:04:05Z07:00")
}
