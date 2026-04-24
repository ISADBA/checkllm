# Baseline: openai / gpt-5.5

## Metadata

```yaml
provider: openai
model: gpt-5.5
api_style: responses
updated_at: 2026-04-24
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
  min: 80
tier_fidelity_score:
  min: 85
route_integrity_score:
  min: 90
overall_risk_score:
  max: 20
```

## Notes

- Baseline is calibrated from the 2026-04-24 GPT-5.5 verification run recorded in docs/runs/20260424-102431-gpt-5.5.md.
- Current thresholds reflect observed OpenAI-compatible Responses behavior at the verified endpoint rather than a direct OpenAI first-party endpoint.
- Usage token accounting should be re-tuned after more repeated samples, because the current run shows a clear usage consistency deviation while protocol and stream behavior remain strong.
