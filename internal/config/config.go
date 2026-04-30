package config

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ISADBA/checkllm/internal/baseline"
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
	fs.StringVar(&cfg.BaseURL, "base-url", "", "provider API base URL")
	fs.StringVar(&cfg.APIKey, "api-key", "", "API key")
	fs.StringVar(&cfg.Model, "model", "", "target model")
	fs.StringVar(&cfg.Provider, "provider", "", "provider")
	fs.StringVar(&cfg.BaselinePath, "baseline", "", "baseline markdown file")
	fs.StringVar(&cfg.OutputPath, "output", "", "output markdown report file")
	fs.DurationVar(&cfg.Timeout, "timeout", 90*time.Second, "per-probe timeout")
	fs.IntVar(&cfg.MaxSamples, "max-samples", 2, "repeat count for repeat probes")
	fs.BoolVar(&cfg.EnableStream, "enable-stream", true, "enable stream probes")
	fs.BoolVar(&cfg.ExpectUsage, "expect-usage", true, "require usage fields to be returned")
	if err := fs.Parse(args[1:]); err != nil {
		return Config{}, err
	}

	cfg.Command = "run"
	return Normalize(cfg)
}

func Normalize(cfg Config) (Config, error) {
	cfg.BaseURL = strings.TrimRight(cfg.BaseURL, "/")
	if cfg.BaseURL == "" || cfg.APIKey == "" || cfg.Model == "" {
		return Config{}, errors.New("missing required flags: --base-url, --api-key, --model")
	}
	if cfg.BaselinePath == "" {
		if err := baseline.EnsureDefaultTemplates(filepath.Join("docs", "baselines")); err != nil {
			return Config{}, fmt.Errorf("initialize baselines: %w", err)
		}
	}

	if cfg.BaselinePath != "" {
		base, err := baseline.Load(cfg.BaselinePath)
		if err != nil {
			return Config{}, fmt.Errorf("load baseline: %w", err)
		}
		if !strings.EqualFold(base.Model, cfg.Model) {
			return Config{}, fmt.Errorf("baseline model %q does not match --model %q", base.Model, cfg.Model)
		}
		if cfg.Provider == "" {
			cfg.Provider = base.Provider
		} else if !strings.EqualFold(cfg.Provider, base.Provider) {
			return Config{}, fmt.Errorf("provider %q does not match baseline provider %q", cfg.Provider, base.Provider)
		}
	}

	if cfg.BaselinePath == "" {
		if cfg.Provider != "" {
			cfg.BaselinePath = filepath.Join("docs", "baselines", fmt.Sprintf("%s-%s.md", cfg.Provider, cfg.Model))
		} else {
			path, provider, err := resolveBaselineForModel(filepath.Join("docs", "baselines"), cfg.Model)
			if err != nil {
				return Config{}, err
			}
			cfg.BaselinePath = path
			cfg.Provider = provider
		}
	}

	if cfg.Provider == "" {
		return Config{}, fmt.Errorf("provider could not be inferred for model %q", cfg.Model)
	}
	if cfg.Provider != "openai" && cfg.Provider != "anthropic" {
		return Config{}, fmt.Errorf("provider %q is not supported yet", cfg.Provider)
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

func SanitizeFileName(v string) string {
	return sanitizeFileName(v)
}

func resolveBaselineForModel(dir, model string) (string, string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return "", "", fmt.Errorf("baseline directory %q does not exist", dir)
		}
		return "", "", err
	}

	var matchedPath string
	var matchedProvider string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		base, err := baseline.Load(path)
		if err != nil {
			return "", "", fmt.Errorf("load baseline %q: %w", path, err)
		}
		if !strings.EqualFold(base.Model, model) {
			continue
		}
		if matchedPath != "" {
			return "", "", fmt.Errorf("multiple baselines found for model %q: %q and %q", model, matchedPath, path)
		}
		matchedPath = path
		matchedProvider = base.Provider
	}
	if matchedPath == "" {
		return "", "", fmt.Errorf("no baseline found for model %q under %q; specify --provider or --baseline", model, dir)
	}
	if matchedProvider == "" {
		return "", "", fmt.Errorf("baseline %q does not declare provider", matchedPath)
	}
	return matchedPath, matchedProvider, nil
}
