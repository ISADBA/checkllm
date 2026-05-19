package probe

import "testing"

func TestDefaultCatalogTunesOpenAIProbeBudgets(t *testing.T) {
	defs := DefaultCatalog("openai", "gpt-5.5", false, 1)
	got := map[string]int{}
	for _, def := range defs {
		got[def.Name] = def.MaxOutputTokens
	}

	tests := map[string]int{
		"fingerprint-wrapper-clean-json": 96,
		"fingerprint-no-branding":        48,
		"identity-self-report-esperanto": 192,
		"identity-self-report-latin":     192,
		"identity-multiturn-esperanto":   384,
		"identity-resistance-latin":      384,
		"tier-multi-constraint":          192,
		"tier-instruction-hard":          160,
		"thinking-basic":                 128,
	}

	for name, want := range tests {
		if got[name] != want {
			t.Fatalf("%s max_output_tokens = %d, want %d", name, got[name], want)
		}
	}
}

func TestDefaultCatalogKeepsAnthropicSpecificTuning(t *testing.T) {
	defs := DefaultCatalog("anthropic", "claude-opus-4-7", false, 1)
	got := map[string]int{}
	for _, def := range defs {
		got[def.Name] = def.MaxOutputTokens
	}

	tests := map[string]int{
		"capability-tool-weather":           160,
		"capability-tool-weather-followup":  192,
		"capability-tool-two-step-order-status": 256,
		"thinking-basic":                    128,
		"fingerprint-wrapper-clean-json":    40,
	}

	for name, want := range tests {
		if got[name] != want {
			t.Fatalf("%s max_output_tokens = %d, want %d", name, got[name], want)
		}
	}
}
