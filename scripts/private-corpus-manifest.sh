#!/usr/bin/env bash
set -euo pipefail

PRINT_CAMERA_DIR="${PRINT_CAMERA_DIR:-print-camera private}"
PRINT_SCAN_DIR="${PRINT_SCAN_DIR:-print-scan private}"
OUTPUT="${PRIVATE_CORPUS_MANIFEST:-private-corpus-manifest.sha256}"

tmp="$(mktemp)"
trap 'rm -f -- "$tmp"' EXIT
found=0
for entry in "camera|$PRINT_CAMERA_DIR|print-camera private" "scan|$PRINT_SCAN_DIR|print-scan private"; do
    IFS='|' read -r _kind dir label <<<"$entry"
    [[ -d "$dir" ]] || continue
    while IFS= read -r -d '' file; do
        hash="$(sha256sum "$file" | awk '{print $1}')"
        printf '%s  %s/%s\n' "$hash" "$label" "$(basename "$file")" >>"$tmp"
        found=1
    done < <(find "$dir" -maxdepth 1 -type f \( -iname '*.jpg' -o -iname '*.jpeg' -o -iname '*.png' \) -print0 | sort -z)
done
if (( found == 0 )); then
    echo "error: no private physical-corpus images found" >&2
    exit 1
fi
LC_ALL=C sort "$tmp" >"$OUTPUT"
echo "Wrote $OUTPUT"
