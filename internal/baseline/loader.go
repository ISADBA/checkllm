package baseline

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

func Load(path string) (Baseline, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Baseline{}, err
	}
	blocks := extractYAMLBlocks(string(data))
	if len(blocks) < 2 {
		return Baseline{}, fmt.Errorf("baseline file %s must include metadata and ranges YAML blocks", path)
	}
	meta := parseFlatMap(blocks[0])
	ranges := parseRanges(blocks[1])
	return Baseline{
		Provider:  meta["provider"],
		Model:     meta["model"],
		APIStyle:  meta["api_style"],
		UpdatedAt: meta["updated_at"],
		Ranges:    ranges,
		Notes:     extractNotes(string(data)),
	}, nil
}

func extractYAMLBlocks(s string) []string {
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

func parseFlatMap(block string) map[string]string {
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

func parseRanges(block string) map[string]Range {
	result := make(map[string]Range)
	var current string
	for _, raw := range strings.Split(block, "\n") {
		if strings.TrimSpace(raw) == "" {
			continue
		}
		if !strings.HasPrefix(raw, " ") && strings.HasSuffix(strings.TrimSpace(raw), ":") {
			current = strings.TrimSuffix(strings.TrimSpace(raw), ":")
			result[current] = Range{}
			continue
		}
		if current == "" {
			continue
		}
		line := strings.TrimSpace(raw)
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		value, err := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
		if err != nil {
			continue
		}
		r := result[current]
		switch strings.TrimSpace(parts[0]) {
		case "min":
			r.Min = floatPtr(value)
		case "max":
			r.Max = floatPtr(value)
		}
		result[current] = r
	}
	return result
}

func extractNotes(s string) []string {
	lines := strings.Split(s, "\n")
	var notes []string
	inNotes := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "## ") {
			inNotes = trimmed == "## Notes"
			continue
		}
		if inNotes && strings.HasPrefix(trimmed, "- ") {
			notes = append(notes, strings.TrimPrefix(trimmed, "- "))
		}
	}
	return notes
}

func floatPtr(v float64) *float64 { return &v }
