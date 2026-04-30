FROM golang:1.22 AS builder

WORKDIR /src

COPY go.mod ./
COPY cmd ./cmd
COPY internal ./internal
COPY docs ./docs

RUN go generate ./internal/baseline
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/checkllm ./cmd/checkllm
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/checkllm-exporter ./cmd/checkllm-exporter

FROM debian:bookworm-slim

RUN apt-get update \
	&& apt-get install -y --no-install-recommends ca-certificates \
	&& rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY --from=builder /out/checkllm /usr/local/bin/checkllm
COPY --from=builder /out/checkllm-exporter /usr/local/bin/checkllm-exporter
COPY --from=builder /src/docs/baselines /app/docs/baselines
COPY scripts/docker-entrypoint.sh /usr/local/bin/docker-entrypoint.sh

RUN mkdir -p /app/docs/runs /app/docs/repos /app/data/exporter-history /etc/checkllm \
	&& chmod +x /usr/local/bin/docker-entrypoint.sh

ENV CHECKLLM_EXPORTER_CONFIG=/etc/checkllm/checkllm_exporter.yaml

ENTRYPOINT ["/usr/local/bin/docker-entrypoint.sh"]
CMD ["checkllm-exporter"]
