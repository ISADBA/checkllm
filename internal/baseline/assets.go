package baseline

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

const DefaultTemplateDir = "docs/baselines"

func EnsureDefaultTemplates(dir string) error {
	if dir == "" {
		dir = DefaultTemplateDir
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create baseline directory %q: %w", dir, err)
	}

	for _, name := range EmbeddedTemplateNames() {
		path := filepath.Join(dir, name)
		if _, err := os.Stat(path); err == nil {
			continue
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("stat baseline template %q: %w", path, err)
		}
		if err := os.WriteFile(path, []byte(embeddedTemplates[name]), 0o644); err != nil {
			return fmt.Errorf("write baseline template %q: %w", path, err)
		}
	}

	return nil
}

func EmbeddedTemplateNames() []string {
	names := make([]string, 0, len(embeddedTemplates))
	for name := range embeddedTemplates {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
