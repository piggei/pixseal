#!/usr/bin/env bash

set -uo pipefail

PIXSEAL="${PIXSEAL:-dist/pixseal}"
PICS_DIR="${PICS_DIR:-original pics}"
TEST_KEY="${TEST_KEY:-pixseal-test-key}"
TEST_MESSAGE_ROBUST="${TEST_MESSAGE_ROBUST:-PixSeal robust}"
LATTICE_ANGLES="${LATTICE_ANGLES:-12.3}"
LATTICE_MODES="${LATTICE_MODES:-scale110x90-rotate scale90x110-rotate scale105x95-rotate scale95x105-rotate}"
LATTICE_MAX_MPIX="${LATTICE_MAX_MPIX:-50}"
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

mapfile -d '' images < <(find "$PICS_DIR" -maxdepth 1 -type f \
    \( -iname '*.jpg' -o -iname '*.jpeg' -o -iname '*.png' \) -print0 | sort -z)
if (( ${#images[@]} == 0 )); then
    echo "error: no JPEG or PNG images found in $PICS_DIR" >&2
    exit 2
fi
read -r -a angles <<< "$LATTICE_ANGLES"
read -r -a modes <<< "$LATTICE_MODES"
if (( ${#angles[@]} == 0 || ${#modes[@]} == 0 )); then
    echo "error: LATTICE_ANGLES and LATTICE_MODES must not be empty" >&2
    exit 2
fi
for mode in "${modes[@]}"; do
    case "$mode" in
        scale110x90-rotate|scale90x110-rotate|scale105x95-rotate|scale95x105-rotate) ;;
        *) echo "error: unsupported LATTICE_MODES entry: $mode" >&2; exit 2 ;;
    esac
done

tmp_dir="$(mktemp -d)"
trap 'rm -rf -- "$tmp_dir"' EXIT
passes=0 failures=0 timeouts=0 errors=0 skipped=0 cases=0 index=0

for image in "${images[@]}"; do
    ((index += 1))
    name="$(basename "$image")"
    read -r width height < <("${identify_tool[@]}" -ping -format '%w %h\n' "$image" 2>/dev/null) || true
    if [[ ! "$width" =~ ^[0-9]+$ || ! "$height" =~ ^[0-9]+$ ]]; then
        echo "Image: $name"
        echo "  ERROR dimensions       could not read image dimensions"
        ((errors += 1)); ((cases += 1))
        continue
    fi
    megapixels=$(( (width * height + 999999) / 1000000 ))
    if (( LATTICE_MAX_MPIX > 0 && width * height > LATTICE_MAX_MPIX * 1000000 )); then
        count=$(( ${#angles[@]} * ${#modes[@]} ))
        printf 'Image: %s\n  SKIP  lattice geometry %d MP exceeds limit of %d MP (%d cases)\n' "$name" "$megapixels" "$LATTICE_MAX_MPIX" "$count"
        ((skipped += count))
        continue
    fi

    echo
    echo "Image: $name"
    message="$TEST_MESSAGE_ROBUST"
    marked="$tmp_dir/marked-$index-robust.png"
    if ! "$PIXSEAL" embed -in "$image" -out "$marked" -key "$TEST_KEY" -message "$message" -profile robust >/dev/null; then
        echo "error: could not create robust hidden payload for $image" >&2
        exit 2
    fi
    echo "  Profile: robust (${#message} bytes)"

    for angle in "${angles[@]}"; do
        for mode in "${modes[@]}"; do
            transformed="$tmp_dir/$mode-$angle-$index.png"
            case "$mode" in
                scale110x90-rotate)
                    geometry='110%x90%'
                    ;;
                scale90x110-rotate)
                    geometry='90%x110%'
                    ;;
                scale105x95-rotate)
                    geometry='105%x95%'
                    ;;
                scale95x105-rotate)
                    geometry='95%x105%'
                    ;;
            esac
            if ! transform_error="$("${image_tool[@]}" "$marked" -resize "$geometry!" -background white -alpha remove -alpha off -rotate "$angle" "$transformed" 2>&1)"; then
                printf '    ERROR %-24s ImageMagick transformation failed\n' "$mode@$angle"
                printf '          %s\n' "${transform_error%%$'\n'*}"
                ((errors += 1)); ((cases += 1))
                continue
            fi

            ((cases += 1))
            output="$(timeout --foreground "${EXTRACT_TIMEOUT}s" "$PIXSEAL" extract -in "$transformed" -key "$TEST_KEY" 2>&1)"
            status=$?
            if (( status == 0 )) && [[ "${output%%$'\n'*}" == "$message" ]]; then
                correction="$(printf '%s\n' "$output" | sed -n '/-correction:/p' | paste -sd ';' -)"
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

echo
total=$((cases + skipped))
printf 'Lattice summary: %d passed, %d failed, %d timeouts, %d skipped, %d errors, %d total\n' \
    "$passes" "$failures" "$timeouts" "$skipped" "$errors" "$total"
if (( errors > 0 )); then exit 2; fi
if [[ "$STRICT" == "1" ]] && (( failures > 0 || timeouts > 0 )); then exit 1; fi
