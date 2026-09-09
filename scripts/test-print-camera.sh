#!/usr/bin/env bash

set -uo pipefail

PIXSEAL="${PIXSEAL:?PIXSEAL is required}"
PRINT_CAMERA_DIR="${PRINT_CAMERA_DIR:-print-camera private}"
PRINT_CAMERA_KEY="${PRINT_CAMERA_KEY:-Piccotti}"
PRINT_CAMERA_TIMEOUT="${PRINT_CAMERA_TIMEOUT:-180}"

files=("foto stampa.PNG" "foto stampa storta.PNG")

if [[ ! -d "$PRINT_CAMERA_DIR" ]]; then
    echo "SKIP  print-camera private corpus not present: $PRINT_CAMERA_DIR"
    exit 0
fi

missing=0
for name in "${files[@]}"; do
    if [[ ! -f "$PRINT_CAMERA_DIR/$name" ]]; then
        echo "SKIP  missing private regression case: $PRINT_CAMERA_DIR/$name"
        missing=1
    fi
done
if (( missing != 0 )); then
    echo "SKIP  print-camera test requires both original private photographs"
    exit 0
fi

if ! [[ "$PRINT_CAMERA_TIMEOUT" =~ ^[0-9]+$ ]] || (( PRINT_CAMERA_TIMEOUT <= 0 )); then
    echo "error: PRINT_CAMERA_TIMEOUT must be a positive integer number of seconds" >&2
    exit 2
fi

work="$(mktemp -d)"
trap 'rm -rf -- "$work"' EXIT

failed=0
for name in "${files[@]}"; do
    input="$PRINT_CAMERA_DIR/$name"
    output="$work/${name// /_}.json"
    echo "Image: $name"
    timeout "${PRINT_CAMERA_TIMEOUT}s" "$PIXSEAL" diagnose \
        -in "$input" -key "$PRINT_CAMERA_KEY" -json >"$output"
    status=$?
    if (( status != 0 )); then
        if (( status == 124 )); then
            echo "  TIMEOUT diagnostic exceeded ${PRINT_CAMERA_TIMEOUT}s"
        else
            echo "  ERROR diagnostic command failed (status $status)"
        fi
        failed=1
        continue
    fi

    grep -E '"(width|height|global_consistency|consensus_fraction|lattice_evidence|authentication_status|authenticated_payload|total_ms)"' "$output" \
        | sed 's/^/  /'

    if grep -Eq '"authenticated_payload"[[:space:]]*:[[:space:]]*true' "$output"; then
        echo "  PASS  authenticated Format v3 payload recovered"
    else
        echo "  RESEARCH  authenticated payload not recovered; this is not a PASS"
        failed=1
    fi
done

if (( failed != 0 )); then
    exit 1
fi
