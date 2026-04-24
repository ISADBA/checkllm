# Baseline: anthropic / claude-opus-4-7

## Metadata

```yaml
provider: anthropic
model: claude-opus-4-7
api_style: messages
updated_at: 2026-04-21
```

## Expected Ranges

```yaml
protocol_conformity_score:
  min: 95
stream_conformity_score:
  min: 95
usage_consistency_score:
  min: 85
behavior_fingerprint_score:
  min: 90
capability_tool_score:
  min: 95
tier_fidelity_score:
  min: 85
route_integrity_score:
  min: 80
overall_risk_score:
  max: 25
```

## Notes

- Baseline is calibrated from the 2026-04-21 Claude Opus 4.7 verification runs against Anthropic-style Messages endpoints.
- Current ranges still need more repeated samples, especially for stream behavior and multi-step tool follow-up.
- Prompt cache is not yet included in the Anthropic-specific baseline interpretation.
