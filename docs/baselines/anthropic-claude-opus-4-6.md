# Baseline: anthropic / claude-opus-4-6

## Metadata

```yaml
provider: anthropic
model: claude-opus-4-6
api_style: messages
updated_at: 2026-04-22
```

## Expected Ranges

```yaml
protocol_conformity_score:
  min: 80
stream_conformity_score:
  min: 95
usage_consistency_score:
  min: 85
behavior_fingerprint_score:
  min: 60
capability_tool_score:
  min: 95
tier_fidelity_score:
  min: 65
route_integrity_score:
  min: 90
overall_risk_score:
  max: 25
```

## Notes

- Baseline is calibrated from the 2026-04-22 Claude Opus 4.6 verification run recorded in docs/runs/20260422-154304-claude-opus-4-6.md.
- Current thresholds reflect observed Anthropic-compatible Messages behavior behind OpenRouter rather than a direct Anthropic first-party endpoint.
- Protocol and tier fidelity should be re-tuned after more repeated samples, because the current run shows visible deviation while tool follow-up remains strong.
