SHELL := /bin/bash
.DEFAULT_GOAL := build
.NOTPARALLEL:

PIXSEAL := dist/pixseal
ORIGINAL_PICS_DIR := original pics
TEST_KEY ?= pixseal-test-key
TEST_MESSAGE ?= PixSeal image test
ROBUST_RESIZES ?= 95 85 75 65 55 50
ROBUST_CROPS ?= 90 75 50
RANDOM_CROPS ?= 75 50
RANDOM_CROP_COUNT ?= 1
RANDOM_SEED ?= 20260907
JPEG_QUALITY ?= 82
ROBUST_MAX_MPIX ?= 100
EXTRACT_TIMEOUT ?= 60
STRICT ?= 0
LIMIT_START ?= 95
LIMIT_MIN ?= 10
LIMIT_STEP ?= 5
GO_SOURCES := $(shell find cmd internal watermark -type f -name '*.go')

.PHONY: build test test-unit test-images deep-test extreme-test all build-all clean

# Default target: build the native executable for the current platform.
build: $(PIXSEAL)

$(PIXSEAL): $(GO_SOURCES) go.mod
	@echo "Building PixSeal..."
	@mkdir -p dist
	@go build -trimpath -ldflags="-s -w" -o $(PIXSEAL) ./cmd/pixseal
	@echo "Created $(PIXSEAL)"

# Self-contained public test suite: no private image corpus is required.
test: build test-unit
	@echo "Self-contained test suite completed."

test-unit:
	@echo "Running Go unit tests..."
	@go test ./...

test-images: build
	@set -euo pipefail; \
	if [[ ! -d "$(ORIGINAL_PICS_DIR)" ]]; then \
		echo "error: image directory not found: $(ORIGINAL_PICS_DIR)" >&2; \
		exit 1; \
	fi; \
	mapfile -d '' images < <(find "$(ORIGINAL_PICS_DIR)" -maxdepth 1 -type f \
		\( -iname '*.jpg' -o -iname '*.jpeg' -o -iname '*.png' \) -print0 | sort -z); \
	if (( $${#images[@]} == 0 )); then \
		echo "error: no JPEG or PNG images found in $(ORIGINAL_PICS_DIR)" >&2; \
		exit 1; \
	fi; \
	tmp_dir="$$(mktemp -d)"; \
	trap 'rm -rf -- "$$tmp_dir"' EXIT; \
	for image in "$${images[@]}"; do \
		name="$$(basename "$$image")"; \
		marked="$$tmp_dir/$$name.png"; \
		echo "Testing $$image"; \
		"$(PIXSEAL)" embed -in "$$image" -out "$$marked" \
			-key "$(TEST_KEY)" -message "$(TEST_MESSAGE)" >/dev/null; \
		extracted="$$($(PIXSEAL) extract -in "$$marked" -key "$(TEST_KEY)" | head -n 1)"; \
		if [[ "$$extracted" != "$(TEST_MESSAGE)" ]]; then \
			echo "error: extracted message differs for $$image" >&2; \
			exit 1; \
		fi; \
		echo "  OK"; \
	done; \
	echo "Image round-trip tests passed: $${#images[@]}"

deep-test: build
	@echo "Running image transformation tests..."
	@PIXSEAL="$(abspath $(PIXSEAL))" \
	PICS_DIR="$(CURDIR)/$(ORIGINAL_PICS_DIR)" \
	TEST_KEY="$(TEST_KEY)" \
	TEST_MESSAGE="$(TEST_MESSAGE)" \
	ROBUST_RESIZES="$(ROBUST_RESIZES)" \
	ROBUST_CROPS="$(ROBUST_CROPS)" \
	RANDOM_CROPS="$(RANDOM_CROPS)" \
	RANDOM_CROP_COUNT="$(RANDOM_CROP_COUNT)" \
	RANDOM_SEED="$(RANDOM_SEED)" \
	JPEG_QUALITY="$(JPEG_QUALITY)" \
	ROBUST_MAX_MPIX="$(ROBUST_MAX_MPIX)" \
	EXTRACT_TIMEOUT="$(EXTRACT_TIMEOUT)" \
	STRICT="$(STRICT)" \
	bash ./scripts/test-robustness.sh

# Explore resize and crop limits; intentionally excluded from make all.
extreme-test: build
	@echo "Running progressive limit tests..."
	@PIXSEAL="$(abspath $(PIXSEAL))" \
	PICS_DIR="$(CURDIR)/$(ORIGINAL_PICS_DIR)" \
	TEST_KEY="$(TEST_KEY)" \
	TEST_MESSAGE="$(TEST_MESSAGE)" \
	ROBUST_MAX_MPIX="$(ROBUST_MAX_MPIX)" \
	EXTRACT_TIMEOUT="$(EXTRACT_TIMEOUT)" \
	LIMIT_START="$(LIMIT_START)" \
	LIMIT_MIN="$(LIMIT_MIN)" \
	LIMIT_STEP="$(LIMIT_STEP)" \
	RANDOM_SEED="$(RANDOM_SEED)" \
	bash ./scripts/test-limits.sh

# Run all tests, including the optional local image corpus.
all: test test-images deep-test
	@echo "Complete local test suite finished."

build-all:
	@echo "Building PixSeal for Linux, Windows and macOS..."
	@mkdir -p dist
	@GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o dist/pixseal-linux-amd64 ./cmd/pixseal
	@GOOS=windows GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o dist/pixseal-windows-amd64.exe ./cmd/pixseal
	@GOOS=darwin GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o dist/pixseal-macos-amd64 ./cmd/pixseal
	@GOOS=darwin GOARCH=arm64 go build -trimpath -ldflags="-s -w" -o dist/pixseal-macos-arm64 ./cmd/pixseal
	@echo "Created cross-platform binaries in dist/"

clean:
	@rm -rf -- dist
	@echo "Removed dist/"
