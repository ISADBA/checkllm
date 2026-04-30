package secrets

import (
	"fmt"
	"os"
	"strings"

	exporterconfig "github.com/ISADBA/checkllm/internal/exporter/config"
)

type Resolver interface {
	Resolve(target exporterconfig.TargetConfig) (string, error)
}

type resolver struct{}

func NewResolver() Resolver {
	return resolver{}
}

func (resolver) Resolve(target exporterconfig.TargetConfig) (string, error) {
	if target.APIKey != "" {
		return target.APIKey, nil
	}
	ref := strings.TrimSpace(target.APIKeyRef)
	switch {
	case strings.HasPrefix(ref, "env:"):
		name := strings.TrimSpace(strings.TrimPrefix(ref, "env:"))
		if name == "" {
			return "", fmt.Errorf("api_key_ref env name is empty")
		}
		value := strings.TrimSpace(os.Getenv(name))
		if value == "" {
			return "", fmt.Errorf("api_key_ref env %q is empty", name)
		}
		return value, nil
	case strings.HasPrefix(ref, "file:"):
		path := strings.TrimSpace(strings.TrimPrefix(ref, "file:"))
		if path == "" {
			return "", fmt.Errorf("api_key_ref file path is empty")
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("read api_key_ref file: %w", err)
		}
		value := strings.TrimSpace(string(data))
		if value == "" {
			return "", fmt.Errorf("api_key_ref file %q is empty", path)
		}
		return value, nil
	default:
		return "", fmt.Errorf("unsupported api_key_ref %q", ref)
	}
}
