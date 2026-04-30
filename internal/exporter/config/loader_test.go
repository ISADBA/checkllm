package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadParsesAndNormalizesConfig(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "exporter.yaml")
	content := `
global:
  listen_addr: ":9108"
  scrape_timeout: 10s
  global_max_concurrency: 4
  default_timeout: 15m
  default_retry:
    max_attempts: 2
    backoff: 30s
groups:
  - name: "prod-official"
    schedule: "0 */6 * * *"
    timeout: 15m
    max_concurrency: 2
    labels:
      env: "prod"
      vendor: "official"
    targets:
      - target_name: "openai-gpt-5-4"
        enabled: true
        provider: "openai"
        base_url: "https://api.openai.com/v1"
        api_key_ref: "env:OPENAI_API_KEY"
        model: "gpt-5.4"
        baseline_path: "./docs/baselines/openai-gpt-5.4.md"
        labels:
          route: "official"
          owner: "platform"
          tier: "flagship"
`
	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Global.GlobalMaxConcurrency != 4 {
		t.Fatalf("unexpected concurrency: %d", cfg.Global.GlobalMaxConcurrency)
	}
	group := cfg.Groups[0]
	if group.Retry.MaxAttempts != 2 || group.Retry.Backoff != 30*time.Second {
		t.Fatalf("unexpected retry config: %+v", group.Retry)
	}
	target := group.Targets[0]
	wantBaseline := filepath.Join(dir, "docs", "baselines", "openai-gpt-5.4.md")
	if target.BaselinePath != wantBaseline {
		t.Fatalf("unexpected baseline path: %s", target.BaselinePath)
	}
}

func TestNormalizeRejectsInvalidLabels(t *testing.T) {
	_, err := Normalize(Config{
		Global: GlobalConfig{
			ListenAddr:           ":9108",
			GlobalMaxConcurrency: 1,
			DefaultTimeout:       time.Minute,
			DefaultRetry:         RetryConfig{MaxAttempts: 1},
		},
		Groups: []GroupConfig{{
			Name:           "prod",
			Schedule:       "0 * * * *",
			Timeout:        time.Minute,
			MaxConcurrency: 1,
			Retry:          RetryConfig{MaxAttempts: 1},
			Targets: []TargetConfig{{
				TargetName:   "a",
				Enabled:      true,
				Provider:     "openai",
				BaseURL:      "https://api.openai.com/v1",
				APIKey:       "x",
				Model:        "gpt-5.4",
				BaselinePath: "/tmp/base.md",
				Labels:       map[string]string{"bad": "value"},
			}},
		}},
	}, t.TempDir())
	if err == nil {
		t.Fatal("expected invalid label error")
	}
}
