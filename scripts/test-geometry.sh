#!/usr/bin/env bash

set -uo pipefail

PIXSEAL="${PIXSEAL:-dist/pixseal}"
PICS_DIR="${PICS_DIR:-original pics}"
TEST_KEY="${TEST_KEY:-pixseal-test-key}"
TEST_PROFILES="${TEST_PROFILES:-robust balanced capacity}"
TEST_MESSAGE_ROBUST="${TEST_MESSAGE_ROBUST:-PixSeal robust}"
TEST_MESSAGE_BALANCED="${TEST_MESSAGE_BALANCED:-PixSeal balanced profile test}"
TEST_MESSAGE_CAPACITY="${TEST_MESSAGE_CAPACITY:-PixSeal capacity profile test message for regression coverage}"
GEOMETRY_ANGLES="${GEOMETRY_ANGLES:--45 -30 -15 -10 -5 -1 1 5 10 15 30 45 90 180 270}"
GEOMETRY_COMBINED_ANGLES="${GEOMETRY_COMBINED_ANGLES:-12.3}"
GEOMETRY_COMBINED_MODES="${GEOMETRY_COMBINED_MODES:-rotate-resize75 resize75-rotate rotate-crop80 crop80-rotate rotate-resize75-crop80}"
GEOMETRY_MAX_MPIX="${GEOMETRY_MAX_MPIX:-50}"
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
if [[ ! "$EXTRACT_TIMEOUT" =~ ^[1-9][0-9]*$ ]]; then
	echo "error: EXTRACT_TIMEOUT must be a positive number of seconds" >&2
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

read -r -a profiles <<< "$TEST_PROFILES"
read -r -a angles <<< "$GEOMETRY_ANGLES"
read -r -a combined_angles <<< "$GEOMETRY_COMBINED_ANGLES"
read -r -a combined_modes <<< "$GEOMETRY_COMBINED_MODES"
if (( ${#profiles[@]} == 0 || ${#angles[@]} == 0 )); then
	echo "error: TEST_PROFILES and GEOMETRY_ANGLES must not be empty" >&2
	exit 2
fi

message_for_profile() {
	case "$1" in
		robust) printf '%s' "$TEST_MESSAGE_ROBUST" ;;
		balanced) printf '%s' "$TEST_MESSAGE_BALANCED" ;;
		capacity) printf '%s' "$TEST_MESSAGE_CAPACITY" ;;
		*) return 1 ;;
	esac
}
valid_mode() {
	case "$1" in
		rotate-resize75|resize75-rotate|rotate-crop80|crop80-rotate|rotate-resize75-crop80) return 0 ;;
		*) return 1 ;;
	esac
}
for profile in "${profiles[@]}"; do
	if ! message_for_profile "$profile" >/dev/null; then
		echo "error: unsupported TEST_PROFILES entry: $profile" >&2
		exit 2
	fi
done
for angle in "${angles[@]}" "${combined_angles[@]}"; do
	if [[ ! "$angle" =~ ^-?[0-9]+([.][0-9]+)?$ ]]; then
		echo "error: invalid geometry angle: $angle" >&2
		exit 2
	fi
done
for mode in "${combined_modes[@]}"; do
	if ! valid_mode "$mode"; then
		echo "error: invalid GEOMETRY_COMBINED_MODES entry: $mode" >&2
		exit 2
	fi
done

tmp_dir="$(mktemp -d)"
trap 'rm -rf -- "$tmp_dir"' EXIT

passes=0
failures=0
timeouts=0
errors=0
skipped=0
cases=0
index=0

run_extract_case() {
	local transformed="$1" message="$2" label="$3"
	local output status extracted correction
	((cases += 1))
	output="$(timeout --foreground "${EXTRACT_TIMEOUT}s" "$PIXSEAL" extract -in "$transformed" -key "$TEST_KEY" 2>&1)"
	status=$?
	if (( status == 0 )); then
		extracted="${output%%$'\n'*}"
		if [[ "$extracted" == "$message" ]]; then
			correction="$(printf '%s\n' "$output" | sed -n 's/^rotation-correction: //p' | head -n 1)"
			if [[ -n "$correction" ]]; then
				printf '    PASS  %-30s message recovered (%s)\n' "$label" "$correction"
			else
				printf '    PASS  %-30s message recovered\n' "$label"
			fi
			((passes += 1))
			return
		fi
	fi
	if (( status == 124 )); then
		printf '    TIMEOUT %-27s exceeded %ss\n' "$label" "$EXTRACT_TIMEOUT"
		((timeouts += 1))
	else
		printf '    FAIL  %-30s message not recovered\n' "$label"
		((failures += 1))
	fi
}

transform_combined() {
	local mode="$1" angle="$2" source="$3" output="$4"
	case "$mode" in
		rotate-resize75)
			"${image_tool[@]}" "$source" -background white -alpha remove -alpha off -rotate "$angle" -resize 75% "$output" ;;
		resize75-rotate)
			"${image_tool[@]}" "$source" -resize 75% -background white -alpha remove -alpha off -rotate "$angle" "$output" ;;
		rotate-crop80)
			"${image_tool[@]}" "$source" -background white -alpha remove -alpha off -rotate "$angle" -gravity center -crop 80%x80%+0+0 +repage "$output" ;;
		crop80-rotate)
			"${image_tool[@]}" "$source" -gravity center -crop 80%x80%+0+0 +repage -background white -alpha remove -alpha off -rotate "$angle" "$output" ;;
		rotate-resize75-crop80)
			"${image_tool[@]}" "$source" -background white -alpha remove -alpha off -rotate "$angle" -resize 75% -gravity center -crop 80%x80%+0+0 +repage "$output" ;;
	esac
}

for image in "${images[@]}"; do
	((index += 1))
	name="$(basename "$image")"
	width=""
	height=""
	read -r width height < <(read_image_dimensions "$image") || true
	if [[ ! "$width" =~ ^[0-9]+$ || ! "$height" =~ ^[0-9]+$ ]]; then
		echo "Image: $name"
		echo "  ERROR dimensions       could not read image dimensions"
		((errors += 1))
		((cases += 1))
		continue
	fi
	megapixels=$(( (width * height + 999999) / 1000000 ))
	per_profile=$(( ${#angles[@]} + ${#combined_angles[@]} * ${#combined_modes[@]} ))
	if (( GEOMETRY_MAX_MPIX > 0 && width * height > GEOMETRY_MAX_MPIX * 1000000 )); then
		count=$(( ${#profiles[@]} * per_profile ))
		printf 'Image: %s\n  SKIP  all geometry     %d MP exceeds limit of %d MP (%d cases)\n' \
			"$name" "$megapixels" "$GEOMETRY_MAX_MPIX" "$count"
		((skipped += count))
		continue
	fi

	echo
	echo "Image: $name"
	for profile in "${profiles[@]}"; do
		message="$(message_for_profile "$profile")"
		marked="$tmp_dir/marked-$index-$profile.png"
		if ! "$PIXSEAL" embed -in "$image" -out "$marked" -key "$TEST_KEY" -message "$message" -profile "$profile" >/dev/null; then
			echo "error: could not create $profile hidden payload for $image" >&2
			exit 2
		fi
		echo "  Profile: $profile (${#message} bytes)"

		for angle in "${angles[@]}"; do
			safe_angle="${angle//-/m}"
			safe_angle="${safe_angle//./p}"
			rotated="$tmp_dir/rotate-$index-$profile-$safe_angle.png"
			if ! transform_error="$("${image_tool[@]}" "$marked" -background white -alpha remove -alpha off -rotate "$angle" "$rotated" 2>&1)"; then
				printf '    ERROR rotate-%-22s ImageMagick transformation failed\n' "${angle}deg"
				printf '          %s\n' "${transform_error%%$'\n'*}"
				((errors += 1)); ((cases += 1))
				continue
			fi
			run_extract_case "$rotated" "$message" "rotate-${angle}deg"
		done

		for angle in "${combined_angles[@]}"; do
			safe_angle="${angle//-/m}"
			safe_angle="${safe_angle//./p}"
			for mode in "${combined_modes[@]}"; do
				transformed="$tmp_dir/$mode-$index-$profile-$safe_angle.png"
				if ! transform_error="$(transform_combined "$mode" "$angle" "$marked" "$transformed" 2>&1)"; then
					printf '    ERROR %-29s ImageMagick transformation failed\n' "$mode-${angle}deg"
					printf '          %s\n' "${transform_error%%$'\n'*}"
					((errors += 1)); ((cases += 1))
					continue
				fi
				run_extract_case "$transformed" "$message" "$mode-${angle}deg"
			done
		done
	done
done

echo
total=$((cases + skipped))
printf 'Geometry summary: %d passed, %d failed, %d timeouts, %d skipped, %d errors, %d total\n' \
	"$passes" "$failures" "$timeouts" "$skipped" "$errors" "$total"

if (( errors > 0 )); then
	exit 2
fi
if [[ "$STRICT" == "1" ]] && (( failures > 0 || timeouts > 0 )); then
	exit 1
fi
