#!/usr/bin/env bash

set -uo pipefail

PIXSEAL="${PIXSEAL:-dist/pixseal}"
PICS_DIR="${PICS_DIR:-original pics}"
TEST_KEY="${TEST_KEY:-pixseal-test-key}"
COMPOSITION_PROFILES="${COMPOSITION_PROFILES:-robust}"
TEST_MESSAGE_ROBUST="${TEST_MESSAGE_ROBUST:-PixSeal robust}"
COMPOSITION_ANGLES="${COMPOSITION_ANGLES:-12.3}"
COMPOSITION_MODES="${COMPOSITION_MODES:-scale110x90-rotate}"
COMPOSITION_MAX_MPIX="${COMPOSITION_MAX_MPIX:-50}"
EXTRACT_TIMEOUT="${EXTRACT_TIMEOUT:-60}"
STRICT="${STRICT:-0}"

if [[ ! -x "$PIXSEAL" ]]; then
    echo "error: PixSeal executable not found: $PIXSEAL" >&2
    exit 2
fi
if [[ ! -d "$PICS_DIR" ]]; then
    echo "error: image directory not found: $PICS_DIR" >&2
    exit 2
fi
if ! command -v timeout >/dev/null 2>&1; then
    echo "error: GNU timeout is required" >&2
    exit 2
fi
if command -v magick >/dev/null 2>&1; then
    image_tool=(magick)
    identify_tool=(magick identify)
elif command -v convert >/dev/null 2>&1; then
    image_tool=(convert)
    identify_tool=(identify)
else
    echo "error: ImageMagick is required (command 'magick' or 'convert')" >&2
    exit 2
fi

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=test-common.sh
source "$SCRIPT_DIR/test-common.sh"

mapfile -d '' images < <(find "$PICS_DIR" -maxdepth 1 -type f \
    \( -iname '*.jpg' -o -iname '*.jpeg' -o -iname '*.png' \) -print0 | sort -z)
if (( ${#images[@]} == 0 )); then
    echo "error: no JPEG or PNG images found in $PICS_DIR" >&2
    exit 2
fi
read -r -a profiles <<< "$COMPOSITION_PROFILES"
read -r -a angles <<< "$COMPOSITION_ANGLES"
read -r -a modes <<< "$COMPOSITION_MODES"
if (( ${#profiles[@]} == 0 || ${#angles[@]} == 0 || ${#modes[@]} == 0 )); then
    echo "error: COMPOSITION_PROFILES, COMPOSITION_ANGLES and COMPOSITION_MODES must not be empty" >&2
    exit 2
fi
for profile in "${profiles[@]}"; do
    if [[ "$profile" != "robust" ]]; then
        echo "error: build 7 direct-lattice composition baseline currently supports only profile robust" >&2
        exit 2
    fi
done
for mode in "${modes[@]}"; do
    if [[ "$mode" != "scale110x90-rotate" ]]; then
        echo "error: unsupported COMPOSITION_MODES entry: $mode" >&2
        exit 2
    fi
done

tmp_dir="$(mktemp -d)"
trap 'rm -rf -- "$tmp_dir"' EXIT
passes=0 failures=0 timeouts=0 errors=0 skipped=0 cases=0 index=0

for image in "${images[@]}"; do
    ((index += 1))
    name="$(basename "$image")"
    read -r width height < <(read_image_dimensions "$image") || true
    if [[ ! "$width" =~ ^[0-9]+$ || ! "$height" =~ ^[0-9]+$ ]]; then
        echo "Image: $name"
        echo "  ERROR dimensions       could not read image dimensions"
        ((errors += 1)); ((cases += 1))
        continue
    fi
    megapixels=$(( (width * height + 999999) / 1000000 ))
    if (( COMPOSITION_MAX_MPIX > 0 && width * height > COMPOSITION_MAX_MPIX * 1000000 )); then
        count=$(( ${#profiles[@]} * ${#angles[@]} * ${#modes[@]} ))
        printf 'Image: %s\n  SKIP  composed geometry %d MP exceeds limit of %d MP (%d cases)\n' "$name" "$megapixels" "$COMPOSITION_MAX_MPIX" "$count"
        ((skipped += count))
        continue
    fi

    echo
    echo "Image: $name"
    for profile in "${profiles[@]}"; do
        message="$TEST_MESSAGE_ROBUST"
        marked="$tmp_dir/marked-$index-$profile.png"
        if ! "$PIXSEAL" embed -in "$image" -out "$marked" -key "$TEST_KEY" -message "$message" -profile "$profile" >/dev/null; then
            echo "error: could not create $profile hidden payload for $image" >&2
            exit 2
        fi
        echo "  Profile: $profile (${#message} bytes)"
        for angle in "${angles[@]}"; do
            for mode in "${modes[@]}"; do
                transformed="$tmp_dir/$mode-$angle-$index-$profile.png"
                case "$mode" in
                    scale110x90-rotate)
                        if ! transform_error="$("${image_tool[@]}" "$marked" -resize '110%x90%!' -background white -alpha remove -alpha off -rotate "$angle" "$transformed" 2>&1)"; then
                            printf '    ERROR %-24s ImageMagick transformation failed\n' "$mode@$angle"
                            printf '          %s\n' "${transform_error%%$'\n'*}"
                            ((errors += 1)); ((cases += 1))
                            continue
                        fi
                        ;;
                esac
                ((cases += 1))
                diag_file="$tmp_dir/diag-${cases:-0}.txt"
	output="$(timeout --foreground "${EXTRACT_TIMEOUT}s" "$PIXSEAL" extract -raw -in "$transformed" -key "$TEST_KEY" 2>"$diag_file")"
                status=$?
                if (( status == 0 )) && [[ "$output" == "$message" ]]; then
                    correction="$(cat "$diag_file" | sed -n '/-correction:/p' | paste -sd ';' -)"
                    printf '    PASS  %-24s message recovered (%s)\n' "$mode@$angle" "$correction"
                    ((passes += 1))
                elif (( status == 124 )); then
                    printf '    TIMEOUT %-21s exceeded %ss\n' "$mode@$angle" "$EXTRACT_TIMEOUT"
                    ((timeouts += 1))
                else
                    printf '    FAIL  %-24s message not recovered\n' "$mode@$angle"
                    ((failures += 1))
                fi
            done
        done
    done
done

echo
total=$((cases + skipped))
printf 'Composition summary: %d passed, %d failed, %d timeouts, %d skipped, %d errors, %d total\n' \
    "$passes" "$failures" "$timeouts" "$skipped" "$errors" "$total"
if (( errors > 0 )); then exit 2; fi
if [[ "$STRICT" == "1" ]] && (( failures > 0 || timeouts > 0 )); then exit 1; fi
