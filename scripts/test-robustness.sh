#!/usr/bin/env bash

set -uo pipefail

PIXSEAL="${PIXSEAL:-dist/pixseal}"
PICS_DIR="${PICS_DIR:-original pics}"
TEST_KEY="${TEST_KEY:-pixseal-test-key}"
TEST_MESSAGE="${TEST_MESSAGE:-PixSeal image test}"
ROBUST_RESIZES="${ROBUST_RESIZES:-75 50}"
ROBUST_CROPS="${ROBUST_CROPS:-90 75}"
JPEG_QUALITY="${JPEG_QUALITY:-82}"
ROBUST_MAX_MPIX="${ROBUST_MAX_MPIX:-100}"
STRICT="${STRICT:-0}"

if [[ ! -x "$PIXSEAL" ]]; then
	echo "error: PixSeal executable not found: $PIXSEAL" >&2
	exit 2
fi
if [[ ! -d "$PICS_DIR" ]]; then
	echo "error: image directory not found: $PICS_DIR" >&2
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

tmp_dir="$(mktemp -d)"
trap 'rm -rf -- "$tmp_dir"' EXIT

passes=0
failures=0
skipped=0
errors=0
cases=0

read -r -a resize_values <<< "$ROBUST_RESIZES"
read -r -a crop_values <<< "$ROBUST_CROPS"

check_extract() {
	local label="$1"
	local transformed="$2"
	local output extracted

	((cases += 1))
	if output="$("$PIXSEAL" extract -in "$transformed" -key "$TEST_KEY" 2>&1)"; then
		extracted="${output%%$'\n'*}"
		if [[ "$extracted" == "$TEST_MESSAGE" ]]; then
			printf '  PASS  %-16s message recovered\n' "$label"
			((passes += 1))
			return
		fi
	fi

	printf '  FAIL  %-16s message not recovered\n' "$label"
	((failures += 1))
}

transform_and_check() {
	local label="$1"
	local transformed="$2"
	local transform_error
	shift 2

	if ! transform_error="$("${image_tool[@]}" "$@" 2>&1)"; then
		((cases += 1))
		((errors += 1))
		printf '  ERROR %-16s ImageMagick transformation failed\n' "$label"
		printf '        %s\n' "${transform_error%%$'\n'*}"
		return
	fi
	check_extract "$label" "$transformed"
}

index=0
for image in "${images[@]}"; do
	((index += 1))
	name="$(basename "$image")"
	marked="$tmp_dir/marked-$index.png"

	echo
	echo "Image: $name"
	if ! read -r width height < <("${identify_tool[@]}" -format '%w %h\n' "$image"); then
		echo "  ERROR dimensions       could not read image dimensions"
		((errors += 1))
		((cases += 1))
		continue
	fi
	megapixels=$(( (width * height + 999999) / 1000000 ))
	if (( ROBUST_MAX_MPIX > 0 && width * height > ROBUST_MAX_MPIX * 1000000 )); then
		transform_count=$((1 + ${#resize_values[@]} + ${#crop_values[@]}))
		printf '  SKIP  all transforms   %d MP exceeds limit of %d MP\n' \
			"$megapixels" "$ROBUST_MAX_MPIX"
		((skipped += transform_count))
		continue
	fi
	if ! "$PIXSEAL" embed -in "$image" -out "$marked" \
		-key "$TEST_KEY" -message "$TEST_MESSAGE" >/dev/null; then
		echo "error: could not create baseline watermark for $image" >&2
		exit 2
	fi

	jpeg="$tmp_dir/jpeg-$index.jpg"
	transform_and_check "jpeg-q$JPEG_QUALITY" "$jpeg" \
		"$marked" -quality "$JPEG_QUALITY" "$jpeg"

	for percent in "${resize_values[@]}"; do
		resized="$tmp_dir/resize-$index-$percent.png"
		transform_and_check "resize-${percent}%" "$resized" \
			"$marked" -resize "${percent}%" "$resized"
	done

	for percent in "${crop_values[@]}"; do
		cropped="$tmp_dir/crop-$index-$percent.png"
		transform_and_check "crop-${percent}%" "$cropped" \
			"$marked" -gravity center -crop "${percent}%x${percent}%+0+0" \
			+repage "$cropped"
	done
done

echo
total=$((cases + skipped))
printf 'Robustness summary: %d passed, %d failed, %d skipped, %d errors, %d total\n' \
	"$passes" "$failures" "$skipped" "$errors" "$total"

if (( errors > 0 )); then
	exit 2
fi

if [[ "$STRICT" == "1" && "$failures" -gt 0 ]]; then
	exit 1
fi
