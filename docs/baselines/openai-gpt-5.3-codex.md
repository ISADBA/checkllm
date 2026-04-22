# Baseline: openai / gpt-5.4

## Metadata

```yaml
provider: openai
model: gpt-5.3-codex
api_style: responses
updated_at: 2026-04-20
```

## Expected Ranges

```yaml
protocol_conformity_score:
  min: 95
stream_conformity_score:
  min: 95
usage_consistency_score:
  min: 90
behavior_fingerprint_score:
  min: 75
capability_tool_score:
  min: 80
tier_fidelity_score:
  min: 80
route_integrity_score:
  min: 90
overall_risk_score:
  max: 35
```

## Notes

- Baseline is intended for direct OpenAI-compatible GPT-5.3-codex responses API behavior.
- Usage token ranges are approximate and should be tuned with real official samples.
