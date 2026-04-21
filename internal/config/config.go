package config

import (
	"errors"
	"flag"
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

type Config struct {
	Command      string
	BaseURL      string
	APIKey       string
	Model        string
	Provider     string
	BaselinePath string
	OutputPath   string
	Timeout      time.Duration
	MaxSamples   int
	EnableStream bool
	ExpectUsage  bool
}

func Parse(args []string) (Config, error) {
	if len(args) == 0 {
		return Config{}, errors.New("usage: checkllm run --base-url ... --api-key ... --model ...")
	}
	if args[0] != "run" {
		return Config{}, fmt.Errorf("unsupported command %q", args[0])
	}

	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	var cfg Config
	fs.StringVar(&cfg.BaseURL, "base-url", "", "OpenAI-compatible API base URL")
	fs.StringVar(&cfg.APIKey, "api-key", "", "API key")
	fs.StringVar(&cfg.Model, "model", "", "target model")
	fs.StringVar(&cfg.Provider, "provider", "openai", "provider")
	fs.StringVar(&cfg.BaselinePath, "baseline", "", "baseline markdown file")
	fs.StringVar(&cfg.OutputPath, "output", "", "output markdown report file")
	fs.DurationVar(&cfg.Timeout, "timeout", 90*time.Second, "run timeout")
	fs.IntVar(&cfg.MaxSamples, "max-samples", 2, "repeat count for repeat probes")
	fs.BoolVar(&cfg.EnableStream, "enable-stream", true, "enable stream probes")
	fs.BoolVar(&cfg.ExpectUsage, "expect-usage", true, "require usage fields to be returned")
	if err := fs.Parse(args[1:]); err != nil {
		return Config{}, err
	}

	cfg.Command = "run"
	cfg.BaseURL = strings.TrimRight(cfg.BaseURL, "/")
	if cfg.Provider == "" {
		cfg.Provider = "openai"
	}
	if cfg.Provider != "openai" {
		return Config{}, fmt.Errorf("provider %q is not supported yet", cfg.Provider)
	}
	if cfg.BaseURL == "" || cfg.APIKey == "" || cfg.Model == "" {
		return Config{}, errors.New("missing required flags: --base-url, --api-key, --model")
	}
	if cfg.BaselinePath == "" {
		cfg.BaselinePath = filepath.Join("docs", "baselines", fmt.Sprintf("%s-%s.md", cfg.Provider, cfg.Model))
	}
	if cfg.OutputPath == "" {
		cfg.OutputPath = filepath.Join("docs", "runs", fmt.Sprintf("%s-%s.md", time.Now().Format("20060102-150405"), sanitizeFileName(cfg.Model)))
	}
	return cfg, nil
}

func (c Config) HistoryDir() string {
	return filepath.Dir(c.OutputPath)
}

func (c Config) UserReportPath() string {
	return filepath.Join("docs", "repos", filepath.Base(c.OutputPath))
}

func sanitizeFileName(v string) string {
	replacer := strings.NewReplacer("/", "-", " ", "-", ":", "-", "\\", "-")
	return replacer.Replace(v)
}
