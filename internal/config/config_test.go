package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseInitializesDefaultBaselines(t *testing.T) {
	t.Chdir(t.TempDir())

	cfg, err := Parse([]string{
		"run",
		"--base-url", "https://example.com/v1",
		"--api-key", "test-key",
		"--model", "gpt-5.4",
	})
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if cfg.Provider != "openai" {
		t.Fatalf("expected provider openai, got %q", cfg.Provider)
	}

	expectedBaseline := filepath.Join("docs", "baselines", "openai-gpt-5.4.md")
	if cfg.BaselinePath != expectedBaseline {
		t.Fatalf("expected baseline path %q, got %q", expectedBaseline, cfg.BaselinePath)
	}

	if _, err := os.Stat(expectedBaseline); err != nil {
		t.Fatalf("expected baseline file %q to exist: %v", expectedBaseline, err)
	}
}
