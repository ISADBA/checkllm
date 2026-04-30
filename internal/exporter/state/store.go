package state

import (
	"sync"
	"time"

	"github.com/ISADBA/checkllm/internal/app/runcheck"
)

type TargetKey struct {
	Group  string
	Target string
}

type MetricLabels struct {
	Group    string
	Target   string
	Provider string
	Model    string
	Env      string
	Vendor   string
	Route    string
	Region   string
	Owner    string
	Tier     string
}

type TargetState struct {
	Labels                   MetricLabels
	LastRunAt                time.Time
	LastSuccessAt            time.Time
	LastDuration             time.Duration
	LastUp                   bool
	LastErrorType            string
	LastConclusion           string
	LastRiskScore            float64
	LastProtocolScore        float64
	LastStreamScore          float64
	LastUsageScore           float64
	LastFingerprintScore     float64
	LastCapabilityScore      float64
	LastTierScore            float64
	LastRouteScore           float64
	LastFunctionalScore      float64
	LastIntelligenceScore    float64
	LastAvgLatencyMs         float64
	LastP95LatencyMs         float64
	LastAvgFirstByteMs       float64
	LastAvgOutputTokensPerS  float64
	LastTimeoutCount         float64
	LastSuccessfulProbeCount float64
	MetricStatuses           map[string]string
	ThinkingStatus           string
	PromptCacheStatus        string
	RunsTotal                map[string]uint64
	FailuresTotal            map[string]uint64
	RetriesTotal             uint64
	SkipsTotal               map[string]uint64
	Running                  bool
}

type SuccessUpdate struct {
	Duration time.Duration
	Summary  runcheck.Summary
	Retries  uint64
}

type FailureUpdate struct {
	Duration  time.Duration
	ErrorType string
	Retries   uint64
}

type Store struct {
	mu      sync.RWMutex
	targets map[TargetKey]*TargetState
}

func NewStore() *Store {
	return &Store{targets: map[TargetKey]*TargetState{}}
}

func (s *Store) EnsureTarget(key TargetKey, labels MetricLabels) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.targets[key]
	if !ok {
		s.targets[key] = &TargetState{
			Labels:         labels,
			MetricStatuses: map[string]string{},
			RunsTotal:      map[string]uint64{},
			FailuresTotal:  map[string]uint64{},
			SkipsTotal:     map[string]uint64{},
		}
		return
	}
	entry.Labels = labels
}

func (s *Store) MarkRunning(key TargetKey, startedAt time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.targets[key]
	if !ok {
		entry = &TargetState{
			MetricStatuses: map[string]string{},
			RunsTotal:      map[string]uint64{},
			FailuresTotal:  map[string]uint64{},
			SkipsTotal:     map[string]uint64{},
		}
		s.targets[key] = entry
	}
	if entry.Running {
		return false
	}
	entry.Running = true
	entry.LastRunAt = startedAt
	return true
}

func (s *Store) FinishSuccess(key TargetKey, update SuccessUpdate) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry := s.targets[key]
	entry.Running = false
	entry.LastSuccessAt = update.Summary.RunAt
	entry.LastDuration = update.Duration
	entry.LastUp = true
	entry.LastErrorType = ""
	entry.LastConclusion = update.Summary.Conclusion
	entry.LastRiskScore = update.Summary.Scores.Risk
	entry.LastProtocolScore = update.Summary.Scores.Protocol
	entry.LastStreamScore = update.Summary.Scores.Stream
	entry.LastUsageScore = update.Summary.Scores.Usage
	entry.LastFingerprintScore = update.Summary.Scores.Fingerprint
	entry.LastCapabilityScore = update.Summary.Scores.Capability
	entry.LastTierScore = update.Summary.Scores.Tier
	entry.LastRouteScore = update.Summary.Scores.Route
	entry.LastFunctionalScore = update.Summary.Scores.Functional
	entry.LastIntelligenceScore = update.Summary.Scores.Intelligence
	entry.LastAvgLatencyMs = update.Summary.Network.AvgLatencyMs
	entry.LastP95LatencyMs = update.Summary.Network.P95LatencyMs
	entry.LastAvgFirstByteMs = update.Summary.Network.AvgFirstByteMs
	entry.LastAvgOutputTokensPerS = update.Summary.Network.AvgOutputTokensPerS
	entry.LastTimeoutCount = update.Summary.Network.TimeoutCount
	entry.LastSuccessfulProbeCount = update.Summary.Network.SuccessfulProbeCount
	entry.ThinkingStatus = update.Summary.Thinking.Status
	entry.PromptCacheStatus = update.Summary.PromptCache.Status
	entry.MetricStatuses = copyStringMap(update.Summary.Statuses)
	entry.RunsTotal["success"]++
	entry.RetriesTotal += update.Retries
}

func (s *Store) FinishFailure(key TargetKey, update FailureUpdate) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry := s.targets[key]
	entry.Running = false
	entry.LastDuration = update.Duration
	entry.LastUp = false
	entry.LastErrorType = update.ErrorType
	entry.RunsTotal["failed"]++
	entry.FailuresTotal[update.ErrorType]++
	entry.RetriesTotal += update.Retries
}

func (s *Store) RecordSkip(key TargetKey, reason string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry := s.targets[key]
	entry.RunsTotal["skipped"]++
	entry.SkipsTotal[reason]++
}

func (s *Store) Snapshot() []TargetState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]TargetState, 0, len(s.targets))
	for _, entry := range s.targets {
		cloned := *entry
		cloned.MetricStatuses = copyStringMap(entry.MetricStatuses)
		cloned.RunsTotal = copyUintMap(entry.RunsTotal)
		cloned.FailuresTotal = copyUintMap(entry.FailuresTotal)
		cloned.SkipsTotal = copyUintMap(entry.SkipsTotal)
		out = append(out, cloned)
	}
	return out
}

func (s *Store) RunningJobs() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	total := 0
	for _, entry := range s.targets {
		if entry.Running {
			total++
		}
	}
	return total
}

func copyStringMap(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func copyUintMap(in map[string]uint64) map[string]uint64 {
	out := make(map[string]uint64, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
