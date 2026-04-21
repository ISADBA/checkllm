package history

import (
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Report struct {
	RunAt   time.Time
	BaseURL string
	Model   string
	Scores  map[string]float64
}

func LoadDir(dir, baseURL, model string) ([]Report, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var reports []Report
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		report, ok, err := loadReport(path, baseURL, model)
		if err != nil {
			return nil, err
		}
		if ok {
			reports = append(reports, report)
		}
	}
	sort.Slice(reports, func(i, j int) bool { return reports[i].RunAt.Before(reports[j].RunAt) })
	return reports, nil
}

func loadReport(path, baseURL, model string) (Report, bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Report{}, false, err
	}
	blocks := extractYAMLBlocks(string(data))
	if len(blocks) < 2 {
		return Report{}, false, nil
	}
	meta := parseFlatMap(blocks[0])
	if meta["base_url"] != baseURL || meta["model"] != model {
		return Report{}, false, nil
	}
	t, err := time.Parse(time.RFC3339, meta["run_at"])
	if err != nil {
		t = time.Time{}
	}
	scoreMap := make(map[string]float64)
	for k, v := range parseFlatMap(blocks[1]) {
		fv, err := strconv.ParseFloat(v, 64)
		if err == nil {
			scoreMap[k] = fv
		}
	}
	return Report{RunAt: t, BaseURL: meta["base_url"], Model: meta["model"], Scores: scoreMap}, true, nil
}

func extractYAMLBlocks(s string) []string         { return baselineExtractYAMLBlocks(s) }
func parseFlatMap(block string) map[string]string { return baselineParseFlatMap(block) }

// Package-local indirection avoids exporting parsing helpers from baseline.
var baselineExtractYAMLBlocks = baselineHelperExtractYAMLBlocks
var baselineParseFlatMap = baselineHelperParseFlatMap

func baselineHelperExtractYAMLBlocks(s string) []string { return baselineTestOnlyExtractYAMLBlocks(s) }
func baselineHelperParseFlatMap(block string) map[string]string {
	return baselineTestOnlyParseFlatMap(block)
}

// Duplicated lightweight parsers keep the package dependency direction clean.
func baselineTestOnlyExtractYAMLBlocks(s string) []string {
	lines := strings.Split(s, "\n")
	var blocks []string
	var current []string
	inYAML := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "```yaml" {
			inYAML = true
			current = nil
			continue
		}
		if inYAML && trimmed == "```" {
			blocks = append(blocks, strings.Join(current, "\n"))
			inYAML = false
			continue
		}
		if inYAML {
			current = append(current, line)
		}
	}
	return blocks
}

func baselineTestOnlyParseFlatMap(block string) map[string]string {
	result := make(map[string]string)
	for _, line := range strings.Split(block, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		result[strings.TrimSpace(parts[0])] = strings.Trim(strings.TrimSpace(parts[1]), "\"")
	}
	return result
}
