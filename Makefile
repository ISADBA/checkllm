APP_NAME := checkllm
MAIN_PKG := ./cmd/checkllm
OUT_DIR := dist
GO ?= go
GENERATE_TARGET := ./internal/baseline

SUPPORTED_PLATFORMS := \
	darwin-amd64 \
	darwin-arm64 \
	linux-amd64 \
	linux-arm64 \
	windows-amd64 \
	windows-arm64

CURRENT_GOOS := $(shell $(GO) env GOOS)
CURRENT_GOARCH := $(shell $(GO) env GOARCH)
CURRENT_PLATFORM := $(CURRENT_GOOS)-$(CURRENT_GOARCH)

REQUESTED_TARGETS := $(filter-out build,$(MAKECMDGOALS))
REQUESTED_TARGET := $(firstword $(REQUESTED_TARGETS))

platform_os = $(word 1,$(subst -, ,$1))
platform_arch = $(word 2,$(subst -, ,$1))
platform_ext = $(if $(filter windows,$(call platform_os,$1)),.exe,)

.PHONY: build build-all clean help _build-platform all $(SUPPORTED_PLATFORMS)

build:
ifeq ($(strip $(REQUESTED_TARGETS)),)
	@$(MAKE) --no-print-directory _build-platform PLATFORM=$(CURRENT_PLATFORM)
else ifeq ($(words $(REQUESTED_TARGETS)),1)
ifeq ($(REQUESTED_TARGET),all)
	@$(MAKE) --no-print-directory build-all
else ifneq ($(filter $(REQUESTED_TARGET),$(SUPPORTED_PLATFORMS)),)
	@$(MAKE) --no-print-directory _build-platform PLATFORM=$(REQUESTED_TARGET)
else
	@echo "unsupported platform: $(REQUESTED_TARGET)"
	@echo "supported platforms: $(SUPPORTED_PLATFORMS)"
	@exit 1
endif
else
	@echo "usage: make build [all|<platform>]"
	@echo "supported platforms: $(SUPPORTED_PLATFORMS)"
	@exit 1
endif

build-all:
	@set -e; for platform in $(SUPPORTED_PLATFORMS); do \
		$(MAKE) --no-print-directory _build-platform PLATFORM=$$platform; \
	done

_build-platform:
	@mkdir -p "$(OUT_DIR)/$(PLATFORM)"
	@echo "building $(PLATFORM)"
	@$(GO) generate $(GENERATE_TARGET)
	@GOOS=$(call platform_os,$(PLATFORM)) GOARCH=$(call platform_arch,$(PLATFORM)) \
		$(GO) build -o "$(OUT_DIR)/$(PLATFORM)/$(APP_NAME)$(call platform_ext,$(PLATFORM))" $(MAIN_PKG)
	@echo "built $(OUT_DIR)/$(PLATFORM)/$(APP_NAME)$(call platform_ext,$(PLATFORM))"

clean:
	@rm -rf "$(OUT_DIR)"

help:
	@echo "make build              Build the current platform binary"
	@echo "make build all          Build all supported platform binaries"
	@echo "make build <platform>   Build one platform binary"
	@echo "supported platforms: $(SUPPORTED_PLATFORMS)"

all $(SUPPORTED_PLATFORMS):
	@:

%:
	@:
