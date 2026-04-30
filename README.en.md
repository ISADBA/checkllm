# checkllm

English | [简体中文](README.md)

`checkllm` is a Go-based command-line tool for validating whether an LLM endpoint really matches the model identity, protocol behavior, and capability profile it claims to expose. In plain terms, it is built to detect model watering-down: wrapping a weaker model and presenting it as a stronger one.

It currently targets two interface styles:

- OpenAI-style `/responses`
- Anthropic-style `/v1/messages`

The goal is not generic benchmarking. The goal is a reproducible authenticity and fidelity check: send a fixed set of probes to a target endpoint, inspect protocol consistency, usage payloads, behavioral fingerprints, tool-calling ability, streaming behavior, and historical runs, then produce a structured risk conclusion.

The project currently provides two runtime modes:

- `checkllm`: a one-shot CLI workflow for manual diagnosis, validating a single endpoint, and generating Markdown reports
- `checkllm-exporter`: a scheduled exporter workflow for continuous probing, Prometheus scraping, and Grafana monitoring

## What It Is For

This tool is useful when you need to:

- verify whether a `base_url + model` pair really corresponds to the claimed official model
- identify OpenAI-compatible endpoints that are replaced, downgraded, or rewritten underneath
- compare behavior differences across vendors, proxy platforms, and relay layers
- store repeated validation results and compare the same endpoint over time
- generate a readable Markdown investigation report for users or internal teams

It focuses on whether an interface behaves like the model family it claims to be, not on benchmark scores.

## How It Works

### 1. Execution Flow

The program runs through this pipeline:

1. Parse CLI arguments.
2. Load the baseline file for the target model.
3. Match the baseline by `model`, infer the `provider`, and build the default probe set.
4. Call the target endpoint probe by probe.
5. Collect raw outputs such as text, status codes, usage data, tool calls, stream events, and latency.
6. Compute multiple scores.
7. Read historical reports from the same directory for comparison.
8. Generate the final interpretation.
9. Output two Markdown reports.

### 2. Probe Design

The default probes are not random Q&A prompts. They are fixed checks designed for authenticity inspection, including:

- `protocol`: protocol compatibility, JSON shape, error behavior, and usage payloads
- `usage`: whether token statistics exist, look reasonable, and vary with input length
- `fingerprint`: writing style, JSON compliance, self-identification consistency, and wrapper traces
- `capability`: whether tool or function calls trigger correctly and whether the model can continue after tool results
- `stream`: stream event count, done event, first-token latency, and event coverage
- `tier`: instruction-following under constraints, context localization, long-context multi-hop behavior, and reasoning stability

Together these probes are much better than a single prompt for identifying:

- fake compatibility layers
- lower-tier models pretending to be higher-tier models from the same vendor
- rewritten output by a relay or wrapper layer
- injected system prompts from a platform layer
- abnormal usage or token reporting

### 3. Scoring and Risk Interpretation

The current implementation computes these core scores:

- `protocol_conformity_score`
- `stream_conformity_score`
- `usage_consistency_score`
- `behavior_fingerprint_score`
- `capability_tool_score`
- `tier_fidelity_score`
- `route_integrity_score`
- `overall_risk_score`

The broad interpretation is:

- higher per-dimension scores mean the endpoint is closer to the expected behavior
- a higher `overall_risk_score` means higher risk
- risk ranges:
- `0-15`: matches the expected capability range of the model
- `16-40`: low overall risk
- `41-69`: medium risk
- `>= 70`: high risk
- hard failures such as protocol breakage or missing usage data will significantly increase total risk
- the interpreter compares measured scores against the baseline ranges and explains the result with historical context

Possible final labels include:

- `high_confidence_official_compatible`
- `compatibility_with_wrapper_risk`
- `suspected_same_brand_downgrade`
- `usage_token_anomaly`
- `suspected_route_or_protocol_mismatch`
- `suspected_wrapper_or_hidden_prompt`

### 4. Baseline File Mechanism

Each target model is associated with a Markdown baseline file, for example:

- [docs/baselines/openai-gpt-5.4.md](docs/baselines/openai-gpt-5.4.md)
- [docs/baselines/anthropic-claude-opus-4-7.md](docs/baselines/anthropic-claude-opus-4-7.md)

A baseline contains two parts:

- metadata: `provider`, `model`, `api_style`, `updated_at`
- metric ranges: `min/max` bounds for each score

If `--provider` is not provided, the program scans `docs/baselines/`, matches a unique baseline by `model`, and infers the provider from the baseline metadata.

The program does not rely only on a hardcoded model name. It combines baseline matching, probe results, and historical comparisons.

### 5. Request Reuse

The default remains one probe per request, because many checks are contrast experiments or stability samples and must be sent independently.

That said, the implementation already supports probes that reuse existing request results. If a probe only needs to extract or score extra signals from a previous response without generating a new sample, it can use `ReuseResultFrom`.

Good reuse scenarios:

- extracting multiple protocol attributes from one response, such as envelope shape, stop status, or usage format
- adding structural checks on top of one tool-calling response
- reusing one streaming response to inspect coverage, termination events, and usage payloads

Bad reuse scenarios:

- input-length comparison probes such as `usage-short / usage-medium / usage-long`
- parameter contrast probes such as `reasoning-on / reasoning-off`
- any stability, repetition, or multi-sample probe
- capability checks that depend on different prompts, tool results, or context states

## Quick Start With the Binary

### 1. Preparation

- prepare a reachable target endpoint
- prepare a valid API key
- prepare the target model name; default baseline templates will be initialized into `docs/baselines/` automatically on first run

### 2. Build or Use a Binary

If you already have a built binary, run it directly. Otherwise build with the repository `Makefile`:

```bash
make build
```

The binary is generated at `dist/<goos>-<goarch>/checkllm`.

The build also generates:

- `dist/<goos>-<goarch>/checkllm-exporter`

On first run, if `docs/baselines/` or any default baseline file is missing, the program fills in the missing files from templates embedded in the binary. Existing local files are not overwritten.

### 3. Validate an OpenAI-Style Endpoint

```bash
./dist/<goos>-<goarch>/checkllm run \
  --base-url https://api.openai.com/v1 \
  --api-key $OPENAI_API_KEY \
  --model gpt-5.4 \
  --baseline docs/baselines/openai-gpt-5.4.md
```

### 4. Validate an Anthropic-Style Endpoint

```bash
./dist/<goos>-<goarch>/checkllm run \
  --base-url https://api.anthropic.com \
  --api-key $ANTHROPIC_API_KEY \
  --model claude-opus-4-7 \
  --baseline docs/baselines/anthropic-claude-opus-4-7.md
```

If the Anthropic-style interface is exposed through an aggregation platform such as OpenRouter, `--base-url` should still point to the platform base URL. The Anthropic provider in this project appends the message endpoint path on its own.

### 5. Common Flags

- `--base-url`: target API base URL
- `--api-key`: access credential
- `--model`: target model name
- `--provider`: optional; currently `openai` and `anthropic`, usually inferred from the baseline
- `--baseline`: path to the baseline file; if omitted, the program scans `docs/baselines/` and matches a unique baseline by `model`
- `--output`: output path for the run artifact; defaults to `docs/runs/`
- `--timeout`: timeout per probe, default `90s`
- `--max-samples`: repeat count for sampling probes, default `2`
- `--enable-stream`: whether to enable streaming probes, default `true`
- `--expect-usage`: whether the endpoint is expected to return usage data, default `true`

### 5.1 Runtime and Token Cost

With the current default probe set, `--max-samples=2`, and `--enable-stream=true`, one full run usually sends around `40-50` probe requests. Multi-step tool follow-ups can increase that number further.

Typical observations:

- OpenAI-style endpoints usually take about `2-4` minutes in total
- Anthropic-style endpoints usually take about `2-6` minutes in total
- OpenAI-style endpoints often consume around `3000-5000` tokens
- Anthropic-style endpoints vary more widely, often around `4000-16000` tokens

These are not guarantees. They mainly depend on:

- larger `--max-samples` values increase repeated probes almost linearly
- enabling stream probes adds one more streaming request
- tool follow-up probes add subsequent requests within one capability check
- different providers bill and expose usage differently for thinking, tools, and output length
- upstream gateways, proxy layers, and rate limits can significantly affect total runtime

The minimal command form is:

```bash
./dist/<goos>-<goarch>/checkllm run \
  --base-url <API_BASE_URL> \
  --api-key <API_KEY> \
  --model <MODEL_NAME>
```

This minimal form works for both OpenAI and Anthropic as long as `docs/baselines/` already contains exactly one matching baseline for the model. If the model has no baseline yet, or you want to force a provider, add `--provider` or `--baseline`.

If you use a built-in model baseline supported by the project, you usually do not need to prepare `docs/baselines/` manually. The program initializes the default templates automatically. You only need to provide your own `--baseline` file when you want a custom model baseline or want to override the default template.

### 6. Output

The program generates two kinds of Markdown output:

- `docs/runs/*.md`: full run archives with probe inputs, raw requests, raw responses, tool calls, and stream events
- `docs/repos/*.md`: simplified user-facing reports

In practice:

- `docs/runs/` is for engineering investigation and reproduction
- `docs/repos/` is suitable for sharing directly with business teams or customers

### 7. Historical Comparison

The program reads existing Markdown reports from the current output directory and filters them by:

- same `base_url`
- same `model`

It then uses those historical results together with the current scores during interpretation. For trend analysis, repeated runs for the same target should be stored in the same output area.

## checkllm-exporter

### 1. Use Cases

If you do not want to inspect an endpoint just once, but want to keep watching it over time, `checkllm-exporter` is the better fit.

Typical scenarios:

- probe a proxy API every 2 hours and watch whether the risk score suddenly rises
- compare official and proxy routes every 6 hours for the same model family
- continuously expose `target_up`, `risk_score`, `tier_score`, and `conclusion` for multiple endpoints
- let Prometheus scrape `/metrics`, then use Grafana for dashboards and alerting

### 2. Execution Model

`checkllm-exporter` works like this:

- the exporter runs checks automatically based on each configured `schedule`
- each completed run updates an in-memory latest snapshot
- when Prometheus scrapes `/metrics`, it gets the latest completed result for each target
- Prometheus itself does not trigger live checks

### 3. Minimal Config Example

```yaml
global:
  listen_addr: ":9108"
  global_max_concurrency: 2
  default_timeout: 15m
  default_retry:
    max_attempts: 2
    backoff: 30s

groups:
  - name: "prod-official"
    schedule: "0 */6 * * *"
    max_concurrency: 1
    labels:
      env: "prod"
      vendor: "official"
      region: "global"
    targets:
      - target_name: "openai-gpt-5-4"
        enabled: true
        provider: "openai"
        base_url: "https://api.openai.com/v1"
        api_key_ref: "env:OPENAI_API_KEY"
        model: "gpt-5.4"
        baseline_path: "./docs/baselines/openai-gpt-5.4.md"
        labels:
          route: "official"
          owner: "platform"
          tier: "flagship"
```

### 4. Start Example

```bash
./dist/<goos>-<goarch>/checkllm-exporter --config ./checkllm_exporter.yaml
```

After startup it exposes:

- `/metrics`
- `/healthz`
- `/readyz`

### 5. Prometheus Scrape Example

```yaml
scrape_configs:
  - job_name: checkllm-exporter
    static_configs:
      - targets:
          - 127.0.0.1:9108
```

### 6. Recommended Metrics to Watch

- `checkllm_target_up`
- `checkllm_target_last_risk_score`
- `checkllm_target_last_tier_score`
- `checkllm_runs_total`
- `checkllm_run_failures_total`
- `checkllm_target_conclusion`

## Scope and Current Limitations

- currently focused on OpenAI `/responses` and Anthropic `/v1/messages`
- current probes are mainly text-task oriented
- `usage` validation is still based on API returns plus coarse local estimation
- streaming analysis is still a first-version implementation
- prompt-cache-specific detection is still being expanded
- baseline quality directly affects judgment stability, so official real samples should be used to keep baselines updated

## Security Notes

- do not commit real API keys into the repository
- do not publish full run archives if they contain sensitive request data
- if you need to share a report, prefer the user-facing report in `docs/repos/` rather than the full archive in `docs/runs/`

For development details, see [developer.md](developer.md).
