package config

import "time"

var AllowedLabelKeys = map[string]struct{}{
	"env":    {},
	"vendor": {},
	"route":  {},
	"region": {},
	"owner":  {},
	"tier":   {},
}

type Config struct {
	Global GlobalConfig
	Groups []GroupConfig
}

type GlobalConfig struct {
	ListenAddr           string
	ScrapeTimeout        time.Duration
	GlobalMaxConcurrency int
	DefaultTimeout       time.Duration
	DefaultRetry         RetryConfig
}

type GroupConfig struct {
	Name           string
	Schedule       string
	Timeout        time.Duration
	MaxConcurrency int
	Retry          RetryConfig
	Labels         map[string]string
	Targets        []TargetConfig
}

type TargetConfig struct {
	TargetName   string
	Enabled      bool
	Provider     string
	BaseURL      string
	APIKey       string
	APIKeyRef    string
	Model        string
	BaselinePath string
	Labels       map[string]string
}

type RetryConfig struct {
	MaxAttempts int
	Backoff     time.Duration
}

func MergeLabels(groupLabels, targetLabels map[string]string) map[string]string {
	merged := map[string]string{}
	for key := range AllowedLabelKeys {
		merged[key] = ""
	}
	for k, v := range groupLabels {
		if _, ok := AllowedLabelKeys[k]; ok {
			merged[k] = v
		}
	}
	for k, v := range targetLabels {
		if _, ok := AllowedLabelKeys[k]; ok {
			merged[k] = v
		}
	}
	return merged
}
