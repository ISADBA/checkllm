# Baseline: openai / gpt-5.2

## Metadata

```yaml
provider: openai
model: gpt-5.2
api_style: responses
updated_at: 2026-04-22
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
  min: 65
capability_tool_score:
  min: 75
tier_fidelity_score:
  min: 80
route_integrity_score:
  min: 90
overall_risk_score:
  max: 25
```

## Notes

- Baseline is calibrated from the 2026-04-22 GPT-5.2 verification run recorded in docs/runs/20260422-101724-gpt-5.2.md.
- Current thresholds reflect observed OpenAI-compatible Responses behavior behind OpenRouter rather than a direct OpenAI first-party endpoint.
- Behavior fingerprint and multi-step tool follow-up should be re-tuned after more repeated samples or an official direct-endpoint verification run.
