package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	cfg, err := parseConfig(string(data))
	if err != nil {
		return Config{}, err
	}
	return Normalize(cfg, filepath.Dir(path))
}

func Normalize(cfg Config, baseDir string) (Config, error) {
	if cfg.Global.ListenAddr == "" {
		cfg.Global.ListenAddr = ":9108"
	}
	if cfg.Global.ScrapeTimeout <= 0 {
		cfg.Global.ScrapeTimeout = 10 * time.Second
	}
	if cfg.Global.GlobalMaxConcurrency < 1 {
		cfg.Global.GlobalMaxConcurrency = 1
	}
	if cfg.Global.DefaultTimeout <= 0 {
		cfg.Global.DefaultTimeout = 15 * time.Minute
	}
	if cfg.Global.DefaultRetry.MaxAttempts < 1 {
		cfg.Global.DefaultRetry.MaxAttempts = 2
	}
	if cfg.Global.DefaultRetry.Backoff < 0 {
		return Config{}, fmt.Errorf("global.default_retry.backoff must be >= 0")
	}
	if cfg.Global.DefaultRetry.Backoff == 0 {
		cfg.Global.DefaultRetry.Backoff = 30 * time.Second
	}
	if strings.TrimSpace(cfg.Global.ListenAddr) == "" {
		return Config{}, fmt.Errorf("global.listen_addr is required")
	}
	if len(cfg.Groups) == 0 {
		return Config{}, fmt.Errorf("at least one group is required")
	}

	seenGroups := map[string]struct{}{}
	for gi := range cfg.Groups {
		group := &cfg.Groups[gi]
		group.Name = strings.TrimSpace(group.Name)
		group.Schedule = strings.TrimSpace(group.Schedule)
		if group.Name == "" {
			return Config{}, fmt.Errorf("groups[%d].name is required", gi)
		}
		if _, exists := seenGroups[group.Name]; exists {
			return Config{}, fmt.Errorf("duplicate group name %q", group.Name)
		}
		seenGroups[group.Name] = struct{}{}
		if group.Schedule == "" {
			return Config{}, fmt.Errorf("group %q schedule is required", group.Name)
		}
		if _, err := ParseCron(group.Schedule); err != nil {
			return Config{}, fmt.Errorf("group %q schedule: %w", group.Name, err)
		}
		if group.Timeout <= 0 {
			group.Timeout = cfg.Global.DefaultTimeout
		}
		if group.MaxConcurrency < 1 {
			group.MaxConcurrency = 1
		}
		if group.Retry.MaxAttempts < 1 {
			group.Retry = cfg.Global.DefaultRetry
		}
		if group.Retry.Backoff < 0 {
			return Config{}, fmt.Errorf("group %q retry.backoff must be >= 0", group.Name)
		}
		if group.Retry.Backoff == 0 {
			group.Retry.Backoff = cfg.Global.DefaultRetry.Backoff
		}
		if err := validateLabels(fmt.Sprintf("group %q", group.Name), group.Labels); err != nil {
			return Config{}, err
		}
		if len(group.Targets) == 0 {
			return Config{}, fmt.Errorf("group %q must contain at least one target", group.Name)
		}
		seenTargets := map[string]struct{}{}
		for ti := range group.Targets {
			target := &group.Targets[ti]
			target.TargetName = strings.TrimSpace(target.TargetName)
			target.Provider = strings.TrimSpace(strings.ToLower(target.Provider))
			target.BaseURL = strings.TrimRight(strings.TrimSpace(target.BaseURL), "/")
			target.APIKey = strings.TrimSpace(target.APIKey)
			target.APIKeyRef = strings.TrimSpace(target.APIKeyRef)
			target.Model = strings.TrimSpace(target.Model)
			target.BaselinePath = resolvePath(baseDir, strings.TrimSpace(target.BaselinePath))
			if target.TargetName == "" {
				return Config{}, fmt.Errorf("group %q target[%d].target_name is required", group.Name, ti)
			}
			if _, exists := seenTargets[target.TargetName]; exists {
				return Config{}, fmt.Errorf("duplicate target %q in group %q", target.TargetName, group.Name)
			}
			seenTargets[target.TargetName] = struct{}{}
			switch target.Provider {
			case "openai", "anthropic":
			default:
				return Config{}, fmt.Errorf("group %q target %q provider %q is not supported", group.Name, target.TargetName, target.Provider)
			}
			if (target.APIKey == "") == (target.APIKeyRef == "") {
				return Config{}, fmt.Errorf("group %q target %q requires exactly one of api_key or api_key_ref", group.Name, target.TargetName)
			}
			if target.Model == "" || target.BaseURL == "" || target.BaselinePath == "" {
				return Config{}, fmt.Errorf("group %q target %q requires provider, base_url, model, baseline_path", group.Name, target.TargetName)
			}
			if strings.HasPrefix(target.APIKeyRef, "file:") {
				refPath := strings.TrimPrefix(target.APIKeyRef, "file:")
				target.APIKeyRef = "file:" + resolvePath(baseDir, refPath)
			}
			if err := validateLabels(fmt.Sprintf("group %q target %q", group.Name, target.TargetName), target.Labels); err != nil {
				return Config{}, err
			}
		}
	}
	return cfg, nil
}

type parsedLine struct {
	indent int
	text   string
}

func parseConfig(input string) (Config, error) {
	lines := cleanLines(input)
	var cfg Config
	i := 0
	for i < len(lines) {
		line := lines[i]
		switch {
		case line.indent == 0 && line.text == "global:":
			i++
			if err := parseGlobal(lines, &i, &cfg.Global); err != nil {
				return Config{}, err
			}
		case line.indent == 0 && line.text == "groups:":
			i++
			groups, err := parseGroups(lines, &i)
			if err != nil {
				return Config{}, err
			}
			cfg.Groups = groups
		default:
			return Config{}, fmt.Errorf("unsupported top-level config line %q", line.text)
		}
	}
	return cfg, nil
}

func cleanLines(input string) []parsedLine {
	var lines []parsedLine
	for _, raw := range strings.Split(input, "\n") {
		if strings.TrimSpace(raw) == "" {
			continue
		}
		trimmed := strings.TrimSpace(raw)
		if strings.HasPrefix(trimmed, "#") {
			continue
		}
		indent := len(raw) - len(strings.TrimLeft(raw, " "))
		lines = append(lines, parsedLine{indent: indent, text: trimmed})
	}
	return lines
}

func parseGlobal(lines []parsedLine, idx *int, global *GlobalConfig) error {
	for *idx < len(lines) {
		line := lines[*idx]
		if line.indent < 2 {
			return nil
		}
		if line.indent != 2 {
			return fmt.Errorf("invalid global indentation near %q", line.text)
		}
		switch {
		case strings.HasPrefix(line.text, "listen_addr:"):
			global.ListenAddr = parseString(valuePart(line.text))
		case strings.HasPrefix(line.text, "scrape_timeout:"):
			d, err := time.ParseDuration(parseString(valuePart(line.text)))
			if err != nil {
				return fmt.Errorf("parse global.scrape_timeout: %w", err)
			}
			global.ScrapeTimeout = d
		case strings.HasPrefix(line.text, "global_max_concurrency:"):
			v, err := strconv.Atoi(parseString(valuePart(line.text)))
			if err != nil {
				return fmt.Errorf("parse global.global_max_concurrency: %w", err)
			}
			global.GlobalMaxConcurrency = v
		case strings.HasPrefix(line.text, "default_timeout:"):
			d, err := time.ParseDuration(parseString(valuePart(line.text)))
			if err != nil {
				return fmt.Errorf("parse global.default_timeout: %w", err)
			}
			global.DefaultTimeout = d
		case line.text == "default_retry:":
			*idx = *idx + 1
			if err := parseRetry(lines, idx, 4, &global.DefaultRetry); err != nil {
				return fmt.Errorf("parse global.default_retry: %w", err)
			}
			continue
		default:
			return fmt.Errorf("unsupported global field %q", line.text)
		}
		*idx = *idx + 1
	}
	return nil
}

func parseGroups(lines []parsedLine, idx *int) ([]GroupConfig, error) {
	var groups []GroupConfig
	for *idx < len(lines) {
		line := lines[*idx]
		if line.indent < 2 {
			return groups, nil
		}
		if line.indent != 2 || !strings.HasPrefix(line.text, "- ") {
			return nil, fmt.Errorf("invalid groups entry near %q", line.text)
		}
		group := GroupConfig{Labels: map[string]string{}}
		inline := strings.TrimPrefix(line.text, "- ")
		if inline != "" {
			if err := assignGroupScalar(&group, inline); err != nil {
				return nil, err
			}
		}
		*idx = *idx + 1
		for *idx < len(lines) {
			current := lines[*idx]
			if current.indent < 4 {
				break
			}
			if current.indent != 4 {
				return nil, fmt.Errorf("invalid group indentation near %q", current.text)
			}
			switch {
			case current.text == "retry:":
				*idx = *idx + 1
				if err := parseRetry(lines, idx, 6, &group.Retry); err != nil {
					return nil, fmt.Errorf("group %q retry: %w", group.Name, err)
				}
			case current.text == "labels:":
				*idx = *idx + 1
				labels, err := parseLabels(lines, idx, 6)
				if err != nil {
					return nil, err
				}
				group.Labels = labels
			case current.text == "targets:":
				*idx = *idx + 1
				targets, err := parseTargets(lines, idx)
				if err != nil {
					return nil, err
				}
				group.Targets = targets
			default:
				if err := assignGroupScalar(&group, current.text); err != nil {
					return nil, err
				}
				*idx = *idx + 1
			}
		}
		groups = append(groups, group)
	}
	return groups, nil
}

func parseTargets(lines []parsedLine, idx *int) ([]TargetConfig, error) {
	var targets []TargetConfig
	for *idx < len(lines) {
		line := lines[*idx]
		if line.indent < 6 {
			return targets, nil
		}
		if line.indent != 6 || !strings.HasPrefix(line.text, "- ") {
			return nil, fmt.Errorf("invalid target entry near %q", line.text)
		}
		target := TargetConfig{Enabled: true, Labels: map[string]string{}}
		inline := strings.TrimPrefix(line.text, "- ")
		if inline != "" {
			if err := assignTargetScalar(&target, inline); err != nil {
				return nil, err
			}
		}
		*idx = *idx + 1
		for *idx < len(lines) {
			current := lines[*idx]
			if current.indent < 8 {
				break
			}
			if current.indent != 8 {
				return nil, fmt.Errorf("invalid target indentation near %q", current.text)
			}
			if current.text == "labels:" {
				*idx = *idx + 1
				labels, err := parseLabels(lines, idx, 10)
				if err != nil {
					return nil, err
				}
				target.Labels = labels
				continue
			}
			if err := assignTargetScalar(&target, current.text); err != nil {
				return nil, err
			}
			*idx = *idx + 1
		}
		targets = append(targets, target)
	}
	return targets, nil
}

func parseLabels(lines []parsedLine, idx *int, indent int) (map[string]string, error) {
	labels := map[string]string{}
	for *idx < len(lines) {
		line := lines[*idx]
		if line.indent < indent {
			return labels, nil
		}
		if line.indent != indent {
			return nil, fmt.Errorf("invalid labels indentation near %q", line.text)
		}
		key, value, ok := splitKV(line.text)
		if !ok {
			return nil, fmt.Errorf("invalid label line %q", line.text)
		}
		labels[key] = parseString(value)
		*idx = *idx + 1
	}
	return labels, nil
}

func parseRetry(lines []parsedLine, idx *int, indent int, retry *RetryConfig) error {
	for *idx < len(lines) {
		line := lines[*idx]
		if line.indent < indent {
			return nil
		}
		if line.indent != indent {
			return fmt.Errorf("invalid retry indentation near %q", line.text)
		}
		switch {
		case strings.HasPrefix(line.text, "max_attempts:"):
			v, err := strconv.Atoi(parseString(valuePart(line.text)))
			if err != nil {
				return fmt.Errorf("parse max_attempts: %w", err)
			}
			retry.MaxAttempts = v
		case strings.HasPrefix(line.text, "backoff:"):
			d, err := time.ParseDuration(parseString(valuePart(line.text)))
			if err != nil {
				return fmt.Errorf("parse backoff: %w", err)
			}
			retry.Backoff = d
		default:
			return fmt.Errorf("unsupported retry field %q", line.text)
		}
		*idx = *idx + 1
	}
	return nil
}

func assignGroupScalar(group *GroupConfig, line string) error {
	key, value, ok := splitKV(line)
	if !ok {
		return fmt.Errorf("invalid group field %q", line)
	}
	switch key {
	case "name":
		group.Name = parseString(value)
	case "schedule":
		group.Schedule = parseString(value)
	case "timeout":
		d, err := time.ParseDuration(parseString(value))
		if err != nil {
			return err
		}
		group.Timeout = d
	case "max_concurrency":
		v, err := strconv.Atoi(parseString(value))
		if err != nil {
			return err
		}
		group.MaxConcurrency = v
	default:
		return fmt.Errorf("unsupported group field %q", key)
	}
	return nil
}

func assignTargetScalar(target *TargetConfig, line string) error {
	key, value, ok := splitKV(line)
	if !ok {
		return fmt.Errorf("invalid target field %q", line)
	}
	switch key {
	case "target_name":
		target.TargetName = parseString(value)
	case "enabled":
		v, err := strconv.ParseBool(parseString(value))
		if err != nil {
			return err
		}
		target.Enabled = v
	case "provider":
		target.Provider = parseString(value)
	case "base_url":
		target.BaseURL = parseString(value)
	case "api_key":
		target.APIKey = parseString(value)
	case "api_key_ref":
		target.APIKeyRef = parseString(value)
	case "model":
		target.Model = parseString(value)
	case "baseline_path":
		target.BaselinePath = parseString(value)
	default:
		return fmt.Errorf("unsupported target field %q", key)
	}
	return nil
}

func splitKV(line string) (string, string, bool) {
	parts := strings.SplitN(line, ":", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]), true
}

func valuePart(line string) string {
	_, value, _ := splitKV(line)
	return value
}

func parseString(v string) string {
	v = strings.TrimSpace(v)
	v = strings.Trim(v, "\"")
	v = strings.Trim(v, "'")
	return v
}

func resolvePath(baseDir, path string) string {
	if path == "" || filepath.IsAbs(path) {
		return path
	}
	return filepath.Clean(filepath.Join(baseDir, path))
}
