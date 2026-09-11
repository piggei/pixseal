#!/usr/bin/env bash
set -uo pipefail

PIXSEAL="${PIXSEAL:-dist/pixseal}"
PICS_DIR="${PICS_DIR:-original pics}"
TEST_KEY="${TEST_KEY:-pixseal-test-key}"
TEST_MESSAGE_ROBUST="${TEST_MESSAGE_ROBUST:-PixSeal robust}"
PERSPECTIVE_MODES="${PERSPECTIVE_MODES:-top-narrow-4 bottom-narrow-4}"
PERSPECTIVE_MAX_MPIX="${PERSPECTIVE_MAX_MPIX:-50}"
EXTRACT_TIMEOUT="${EXTRACT_TIMEOUT:-60}"
STRICT="${STRICT:-0}"

if [[ ! -x "$PIXSEAL" ]]; then echo "error: PixSeal executable not found: $PIXSEAL" >&2; exit 2; fi
if [[ ! -d "$PICS_DIR" ]]; then echo "error: image directory not found: $PICS_DIR" >&2; exit 2; fi
if [[ ! "$EXTRACT_TIMEOUT" =~ ^[1-9][0-9]*$ ]]; then echo "error: EXTRACT_TIMEOUT must be a positive number of seconds" >&2; exit 2; fi
if [[ ! "$PERSPECTIVE_MAX_MPIX" =~ ^[0-9]+$ ]]; then echo "error: PERSPECTIVE_MAX_MPIX must be a non-negative integer" >&2; exit 2; fi
if ! command -v timeout >/dev/null 2>&1; then echo "error: GNU timeout is required" >&2; exit 2; fi

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

read -r -a modes <<< "$PERSPECTIVE_MODES"
if (( ${#modes[@]} == 0 )); then echo "error: PERSPECTIVE_MODES is empty" >&2; exit 2; fi
for mode in "${modes[@]}"; do
    case "$mode" in
        top-narrow-4|bottom-narrow-4) ;;
        *) echo "error: unsupported positive perspective mode: $mode (v0.2 supports top-narrow-4 and bottom-narrow-4)" >&2; exit 2 ;;
    esac
done

mapfile -d '' images < <(find "$PICS_DIR" -maxdepth 1 -type f \( -iname '*.jpg' -o -iname '*.jpeg' -o -iname '*.png' \) -print0 | sort -z)
if (( ${#images[@]} == 0 )); then
    echo "error: no JPEG or PNG images found in $PICS_DIR" >&2
    exit 2
fi

tmp_dir="$(mktemp -d)"
trap 'rm -rf -- "$tmp_dir"' EXIT
passes=0 failures=0 timeouts=0 errors=0 skipped=0 cases=0 index=0

for image in "${images[@]}"; do
    ((index+=1))
    name="$(basename "$image")"
    width=""; height=""
    read -r width height < <(read_image_dimensions "$image") || true
    if [[ ! "$width" =~ ^[0-9]+$ || ! "$height" =~ ^[0-9]+$ ]]; then
        echo "Image: $name"
        printf '  SKIP  mild perspective dimensions unavailable (%d cases)\n' "${#modes[@]}"
        ((skipped+=${#modes[@]}))
        continue
    fi
    mp=$(( (width*height+999999)/1000000 ))
    if (( PERSPECTIVE_MAX_MPIX>0 && width*height>PERSPECTIVE_MAX_MPIX*1000000 )); then
        printf 'Image: %s\n  SKIP  mild perspective %d MP exceeds limit of %d MP (%d cases)\n' "$name" "$mp" "$PERSPECTIVE_MAX_MPIX" "${#modes[@]}"
        ((skipped+=${#modes[@]}))
        continue
    fi

    echo; echo "Image: $name"
    marked="$tmp_dir/marked-$index.png"
    if ! "$PIXSEAL" embed -in "$image" -out "$marked" -key "$TEST_KEY" -message "$TEST_MESSAGE_ROBUST" -profile robust >/dev/null; then
        printf '  ERROR embed            could not create robust carrier (%d cases)\n' "${#modes[@]}"
        ((errors+=${#modes[@]})); ((cases+=${#modes[@]}))
        continue
    fi
    echo "  Profile: robust (${#TEST_MESSAGE_ROBUST} bytes)"

    x1=$(( (width-1)*4/100 )); x2=$(( (width-1)-x1 )); maxx=$((width-1)); maxy=$((height-1))
    for mode in "${modes[@]}"; do
        ((cases+=1))
        case "$mode" in
            top-narrow-4) pts="0,0 $x1,0 $maxx,0 $x2,0 0,$maxy 0,$maxy $maxx,$maxy $maxx,$maxy" ;;
            bottom-narrow-4) pts="0,0 0,0 $maxx,0 $maxx,0 0,$maxy $x1,$maxy $maxx,$maxy $x2,$maxy" ;;
        esac
        transformed="$tmp_dir/$mode-$index.png"
        if ! "${image_tool[@]}" "$marked" -alpha off -virtual-pixel white -define "distort:viewport=${width}x${height}+0+0" -distort Perspective "$pts" "$transformed" >/dev/null 2>&1; then
            printf '    ERROR %-18s ImageMagick transformation failed\n' "$mode"; ((errors+=1)); continue
        fi

        diag="$tmp_dir/diag-$index-$mode.txt"
        output="$(timeout --foreground "${EXTRACT_TIMEOUT}s" "$PIXSEAL" extract -raw -in "$transformed" -key "$TEST_KEY" 2>"$diag")"
        status=$?
        if (( status==0 )) && [[ "$output" == "$TEST_MESSAGE_ROBUST" ]] && grep -Fxq "perspective-correction: $mode" "$diag"; then
            printf '    PASS  %-18s message recovered (perspective-correction: %s)\n' "$mode" "$mode"; ((passes+=1))
        elif (( status==124 )); then
            printf '    TIMEOUT %-15s exceeded %ss\n' "$mode" "$EXTRACT_TIMEOUT"; ((timeouts+=1))
        else
            printf '    FAIL  %-18s message/path not recovered\n' "$mode"; ((failures+=1))
        fi
    done
done

total=$((cases+skipped))
echo
printf 'Perspective summary: %d passed, %d failed, %d timeouts, %d skipped, %d errors, %d total\n' "$passes" "$failures" "$timeouts" "$skipped" "$errors" "$total"
if ((cases==0 && skipped==0)); then echo "error: perspective suite executed zero test cases" >&2; exit 2; fi
if ((errors>0)); then exit 2; fi
if [[ "$STRICT" == 1 ]] && ((failures>0 || timeouts>0)); then exit 1; fi
