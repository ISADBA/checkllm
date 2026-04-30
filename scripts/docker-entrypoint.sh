#!/bin/sh
set -eu

if [ "$#" -eq 0 ]; then
	set -- checkllm-exporter
fi

if [ "$1" = "checkllm-exporter" ]; then
	shift
	exec /usr/local/bin/checkllm-exporter --config "${CHECKLLM_EXPORTER_CONFIG}" "$@"
fi

if [ "$1" = "checkllm" ]; then
	shift
	exec /usr/local/bin/checkllm "$@"
fi

exec "$@"
