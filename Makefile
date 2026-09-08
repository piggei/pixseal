SHELL := /bin/bash
.DEFAULT_GOAL := build
.NOTPARALLEL:

PIXSEAL := dist/pixseal
ORIGINAL_PICS_DIR := original pics
TEST_KEY ?= pixseal-test-key
TEST_PROFILES ?= robust balanced capacity
TEST_MESSAGE_ROBUST ?= PixSeal robust
TEST_MESSAGE_BALANCED ?= PixSeal balanced profile test
TEST_MESSAGE_CAPACITY ?= PixSeal capacity profile test message for regression coverage
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
GEOMETRY_ANGLES ?= -45 -30 -15 -10 -5 -1 1 5 10 15 30 45 90 180 270
GEOMETRY_COMBINED_ANGLES ?= 12.3
GEOMETRY_COMBINED_MODES ?= rotate-resize75 resize75-rotate rotate-crop80 crop80-rotate rotate-resize75-crop80
GEOMETRY_MAX_MPIX ?= 50
AFFINE_MODES ?= scale110x90 scale90x110 shearx8 sheary8
AFFINE_MAX_MPIX ?= 50
COMPOSITION_PROFILES ?= robust
COMPOSITION_ANGLES ?= 12.3
COMPOSITION_MODES ?= scale110x90-rotate
COMPOSITION_MAX_MPIX ?= 50
LATTICE_ANGLES ?= 12.3
LATTICE_MODES ?= scale110x90-rotate scale90x110-rotate scale105x95-rotate scale95x105-rotate
LATTICE_MAX_MPIX ?= 50
PERSPECTIVE_MODES ?= top-narrow-4 bottom-narrow-4
PERSPECTIVE_MAX_MPIX ?= 50
ALL_TEST_REPORT ?=
ALL_TEST_TARGETS ?=
ALL_TEST_STRICT ?=1
GO_SOURCES := $(shell find cmd internal watermark -type f -name '*.go')

.PHONY: build test test-unit test-images deep-test extreme-test geometry-test affine-test composition-test lattice-test perspective-test all-test all build-all core-target-check vet clean

# Default target: build the native executable for the current platform.
build: $(PIXSEAL)

$(PIXSEAL): $(GO_SOURCES) go.mod
	@echo "Building PixSeal..."
	@mkdir -p dist
	@go build -trimpath -ldflags="-s -w" -o $(PIXSEAL) ./cmd/pixseal
	@echo "Created $(PIXSEAL)"

# Local release test suite: unit tests plus round trips on original pics.
test: build test-unit test-images
	@echo "Local test suite completed."

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
	read -r -a profiles <<< "$(TEST_PROFILES)"; \
	if (( $${#profiles[@]} == 0 )); then \
		echo "error: TEST_PROFILES is empty" >&2; \
		exit 1; \
	fi; \
	for image in "$${images[@]}"; do \
		name="$$(basename "$$image")"; \
		echo "Testing $$image"; \
		for profile in "$${profiles[@]}"; do \
			case "$$profile" in \
				robust) message='$(TEST_MESSAGE_ROBUST)' ;; \
				balanced) message='$(TEST_MESSAGE_BALANCED)' ;; \
				capacity) message='$(TEST_MESSAGE_CAPACITY)' ;; \
				*) echo "error: unsupported TEST_PROFILES entry: $$profile" >&2; exit 1 ;; \
			esac; \
			marked="$$tmp_dir/$$name-$$profile.png"; \
			"$(PIXSEAL)" embed -in "$$image" -out "$$marked" \
				-key "$(TEST_KEY)" -message "$$message" -profile "$$profile" >/dev/null; \
			extracted="$$($(PIXSEAL) extract -in "$$marked" -key "$(TEST_KEY)" | head -n 1)"; \
			if [[ "$$extracted" != "$$message" ]]; then \
				echo "error: extracted message differs for $$image ($$profile)" >&2; \
				exit 1; \
			fi; \
			echo "  OK $$profile"; \
		done; \
	done; \
	echo "Image round-trip tests passed: $${#images[@]} images x $${#profiles[@]} profiles"

deep-test: build
	@echo "Running image transformation tests..."
	@PIXSEAL="$(abspath $(PIXSEAL))" \
	PICS_DIR="$(CURDIR)/$(ORIGINAL_PICS_DIR)" \
	TEST_KEY="$(TEST_KEY)" \
	TEST_PROFILES="$(TEST_PROFILES)" \
	TEST_MESSAGE_ROBUST="$(TEST_MESSAGE_ROBUST)" \
	TEST_MESSAGE_BALANCED="$(TEST_MESSAGE_BALANCED)" \
	TEST_MESSAGE_CAPACITY="$(TEST_MESSAGE_CAPACITY)" \
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
	TEST_PROFILES="$(TEST_PROFILES)" \
	TEST_MESSAGE_ROBUST="$(TEST_MESSAGE_ROBUST)" \
	TEST_MESSAGE_BALANCED="$(TEST_MESSAGE_BALANCED)" \
	TEST_MESSAGE_CAPACITY="$(TEST_MESSAGE_CAPACITY)" \
	ROBUST_MAX_MPIX="$(ROBUST_MAX_MPIX)" \
	EXTRACT_TIMEOUT="$(EXTRACT_TIMEOUT)" \
	LIMIT_START="$(LIMIT_START)" \
	LIMIT_MIN="$(LIMIT_MIN)" \
	LIMIT_STEP="$(LIMIT_STEP)" \
	RANDOM_SEED="$(RANDOM_SEED)" \
	bash ./scripts/test-limits.sh

# Experimental geometric robustness suite; intentionally excluded from make all.
geometry-test: build
	@echo "Running digital rotation and combined geometry tests..."
	@PIXSEAL="$(abspath $(PIXSEAL))" \
	PICS_DIR="$(CURDIR)/$(ORIGINAL_PICS_DIR)" \
	TEST_KEY="$(TEST_KEY)" \
	TEST_PROFILES="$(TEST_PROFILES)" \
	TEST_MESSAGE_ROBUST="$(TEST_MESSAGE_ROBUST)" \
	TEST_MESSAGE_BALANCED="$(TEST_MESSAGE_BALANCED)" \
	TEST_MESSAGE_CAPACITY="$(TEST_MESSAGE_CAPACITY)" \
	GEOMETRY_ANGLES="$(GEOMETRY_ANGLES)" \
	GEOMETRY_COMBINED_ANGLES="$(GEOMETRY_COMBINED_ANGLES)" \
	GEOMETRY_COMBINED_MODES="$(GEOMETRY_COMBINED_MODES)" \
	GEOMETRY_MAX_MPIX="$(GEOMETRY_MAX_MPIX)" \
	EXTRACT_TIMEOUT="$(EXTRACT_TIMEOUT)" \
	STRICT="$(STRICT)" \
	bash ./scripts/test-geometry.sh

# Experimental axis-aligned affine suite; intentionally excluded from make all.
affine-test: build
	@echo "Running axis-aligned affine tests..."
	@PIXSEAL="$(abspath $(PIXSEAL))" \
	PICS_DIR="$(CURDIR)/$(ORIGINAL_PICS_DIR)" \
	TEST_KEY="$(TEST_KEY)" \
	TEST_PROFILES="$(TEST_PROFILES)" \
	TEST_MESSAGE_ROBUST="$(TEST_MESSAGE_ROBUST)" \
	TEST_MESSAGE_BALANCED="$(TEST_MESSAGE_BALANCED)" \
	TEST_MESSAGE_CAPACITY="$(TEST_MESSAGE_CAPACITY)" \
	AFFINE_MODES="$(AFFINE_MODES)" \
	AFFINE_MAX_MPIX="$(AFFINE_MAX_MPIX)" \
	EXTRACT_TIMEOUT="$(EXTRACT_TIMEOUT)" \
	STRICT="$(STRICT)" \
	bash ./scripts/test-affine.sh

# Experimental composed geometry suite; intentionally excluded from make all.
composition-test: build
	@echo "Running composed anisotropic-scale + rotation tests..."
	@PIXSEAL="$(abspath $(PIXSEAL))" \
	PICS_DIR="$(CURDIR)/$(ORIGINAL_PICS_DIR)" \
	TEST_KEY="$(TEST_KEY)" \
	TEST_MESSAGE_ROBUST="$(TEST_MESSAGE_ROBUST)" \
	COMPOSITION_PROFILES="$(COMPOSITION_PROFILES)" \
	COMPOSITION_ANGLES="$(COMPOSITION_ANGLES)" \
	COMPOSITION_MODES="$(COMPOSITION_MODES)" \
	COMPOSITION_MAX_MPIX="$(COMPOSITION_MAX_MPIX)" \
	EXTRACT_TIMEOUT="$(EXTRACT_TIMEOUT)" \
	STRICT="$(STRICT)" \
	bash ./scripts/test-composition.sh

# Experimental direct lattice-basis suite; intentionally excluded from make all.
lattice-test: build
	@echo "Running direct lattice-basis composition tests..."
	@PIXSEAL="$(abspath $(PIXSEAL))" \
	PICS_DIR="$(CURDIR)/$(ORIGINAL_PICS_DIR)" \
	TEST_KEY="$(TEST_KEY)" \
	TEST_MESSAGE_ROBUST="$(TEST_MESSAGE_ROBUST)" \
	LATTICE_ANGLES="$(LATTICE_ANGLES)" \
	LATTICE_MODES="$(LATTICE_MODES)" \
	LATTICE_MAX_MPIX="$(LATTICE_MAX_MPIX)" \
	EXTRACT_TIMEOUT="$(EXTRACT_TIMEOUT)" \
	STRICT="$(STRICT)" \
	bash ./scripts/test-lattice.sh

# Experimental mild projective/keystone suite; intentionally excluded from make all.
perspective-test: build
	@echo "Running mild perspective tests..."
	@PIXSEAL="$(abspath $(PIXSEAL))" \
	PICS_DIR="$(CURDIR)/$(ORIGINAL_PICS_DIR)" \
	TEST_KEY="$(TEST_KEY)" TEST_MESSAGE_ROBUST="$(TEST_MESSAGE_ROBUST)" \
	PERSPECTIVE_MODES="$(PERSPECTIVE_MODES)" PERSPECTIVE_MAX_MPIX="$(PERSPECTIVE_MAX_MPIX)" \
	EXTRACT_TIMEOUT="$(EXTRACT_TIMEOUT)" STRICT="$(STRICT)" bash ./scripts/test-perspective.sh

# Run every test/check target sequentially, continue after individual failures,
# and print one comparable summary at the end. Experimental suites are forced
# into STRICT=1 so a reported FAIL/TIMEOUT is reflected in the target status.
# Set ALL_TEST_REPORT=path/to/report.txt to tee the complete run to a file.
all-test:
	@ALL_TEST_REPORT="$(ALL_TEST_REPORT)" \
	ALL_TEST_TARGETS="$(ALL_TEST_TARGETS)" \
	ALL_TEST_STRICT="$(ALL_TEST_STRICT)" \
	bash ./scripts/test-all.sh

vet:
	@echo "Running go vet..."
	@go vet ./...

# Run build + local round-trip tests + baseline transformation tests.
all: test deep-test
	@echo "Complete local test suite finished."

build-all:
	@echo "Building PixSeal for Linux, Windows and macOS..."
	@mkdir -p dist
	@GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o dist/pixseal-linux-amd64 ./cmd/pixseal
	@GOOS=windows GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o dist/pixseal-windows-amd64.exe ./cmd/pixseal
	@GOOS=darwin GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o dist/pixseal-macos-amd64 ./cmd/pixseal
	@GOOS=darwin GOARCH=arm64 go build -trimpath -ldflags="-s -w" -o dist/pixseal-macos-arm64 ./cmd/pixseal
	@echo "Created cross-platform binaries in dist/"

# Compile the reusable steganography core for representative future frontend targets.
# This creates no distributable binaries; it is an architectural portability check.
core-target-check:
	@echo "Checking reusable core on desktop/mobile targets..."
	@CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build ./watermark
	@CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build ./watermark
	@CGO_ENABLED=0 GOOS=android GOARCH=arm64 go build ./watermark
	@CGO_ENABLED=0 GOOS=ios GOARCH=arm64 go build ./watermark
	@echo "Core target checks passed: linux/amd64, windows/amd64, android/arm64, ios/arm64"

clean:
	@rm -rf -- dist
	@echo "Removed dist/"
