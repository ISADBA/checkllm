package baseline

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureDefaultTemplatesCreatesMissingFiles(t *testing.T) {
	dir := t.TempDir()

	if err := EnsureDefaultTemplates(dir); err != nil {
		t.Fatalf("EnsureDefaultTemplates() error = %v", err)
	}

	names := EmbeddedTemplateNames()
	if len(names) == 0 {
		t.Fatal("expected embedded baseline templates")
	}
	for _, name := range names {
		path := filepath.Join(dir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile(%q) error = %v", path, err)
		}
		if string(data) != embeddedTemplates[name] {
			t.Fatalf("baseline %q content mismatch", name)
		}
	}
}

func TestEnsureDefaultTemplatesKeepsExistingFiles(t *testing.T) {
	dir := t.TempDir()
	names := EmbeddedTemplateNames()
	if len(names) == 0 {
		t.Fatal("expected embedded baseline templates")
	}

	path := filepath.Join(dir, names[0])
	if err := os.WriteFile(path, []byte("custom"), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}

	if err := EnsureDefaultTemplates(dir); err != nil {
		t.Fatalf("EnsureDefaultTemplates() error = %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}
	if string(data) != "custom" {
		t.Fatalf("expected existing baseline to be preserved, got %q", string(data))
	}
}
