#!/usr/bin/env bash

set -uo pipefail

PIXSEAL="${PIXSEAL:-dist/pixseal}"
PICS_DIR="${PICS_DIR:-original pics}"
TEST_KEY="${TEST_KEY:-pixseal-test-key}"
TEST_PROFILES="${TEST_PROFILES:-robust balanced capacity}"
TEST_MESSAGE_ROBUST="${TEST_MESSAGE_ROBUST:-PixSeal robust}"
TEST_MESSAGE_BALANCED="${TEST_MESSAGE_BALANCED:-PixSeal balanced profile test}"
TEST_MESSAGE_CAPACITY="${TEST_MESSAGE_CAPACITY:-PixSeal capacity profile test message for regression coverage}"
ROBUST_RESIZES="${ROBUST_RESIZES:-95 85 75 65 55 50}"
ROBUST_CROPS="${ROBUST_CROPS:-90 75 50}"
RANDOM_CROPS="${RANDOM_CROPS:-75 50}"
RANDOM_CROP_COUNT="${RANDOM_CROP_COUNT:-1}"
RANDOM_SEED="${RANDOM_SEED:-20260907}"
JPEG_QUALITY="${JPEG_QUALITY:-82}"
ROBUST_MAX_MPIX="${ROBUST_MAX_MPIX:-100}"
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
if (( ${#profiles[@]} == 0 )); then
	echo "error: TEST_PROFILES is empty" >&2
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
for profile in "${profiles[@]}"; do
	if ! message_for_profile "$profile" >/dev/null; then
		echo "error: unsupported TEST_PROFILES entry: $profile" >&2
		exit 2
	fi
done

tmp_dir="$(mktemp -d)"
trap 'rm -rf -- "$tmp_dir"' EXIT

passes=0
failures=0
skipped=0
errors=0
timeouts=0
cases=0

read -r -a resize_values <<< "$ROBUST_RESIZES"
read -r -a crop_values <<< "$ROBUST_CROPS"
read -r -a random_crop_values <<< "$RANDOM_CROPS"

check_extract() {
	local label="$1"
	local transformed="$2"
	local expected="$3"
	local output extracted status

	((cases += 1))
	output="$(timeout --foreground "${EXTRACT_TIMEOUT}s" \
		"$PIXSEAL" extract -raw -in "$transformed" -key "$TEST_KEY" 2>/dev/null)"
	status=$?
	if (( status == 0 )); then
		extracted="$output"
		if [[ "$extracted" == "$expected" ]]; then
			printf '    PASS  %-16s message recovered\n' "$label"
			((passes += 1))
			return
		fi
	fi
	if (( status == 124 )); then
		printf '    TIMEOUT %-13s exceeded %ss\n' "$label" "$EXTRACT_TIMEOUT"
		((timeouts += 1))
		return
	fi

	printf '    FAIL  %-16s message not recovered\n' "$label"
	((failures += 1))
}

transform_and_check() {
	local label="$1"
	local transformed="$2"
	local expected="$3"
	local transform_error
	shift 3

	if ! transform_error="$("${image_tool[@]}" "$@" 2>&1)"; then
		((cases += 1))
		((errors += 1))
		printf '    ERROR %-16s ImageMagick transformation failed\n' "$label"
		printf '          %s\n' "${transform_error%%$'\n'*}"
		return
	fi
	check_extract "$label" "$transformed" "$expected"
}

index=0
for image in "${images[@]}"; do
	((index += 1))
	name="$(basename "$image")"

	echo
	echo "Image: $name"
	width=""
	height=""
	read -r width height < <(read_image_dimensions "$image") || true
	if [[ ! "$width" =~ ^[0-9]+$ || ! "$height" =~ ^[0-9]+$ ]]; then
		file_info="$(file -b "$image" 2>/dev/null || true)"
		if [[ "$file_info" =~ ([0-9]+)[[:space:]]*x[[:space:]]*([0-9]+) ]]; then
			width="${BASH_REMATCH[1]}"
			height="${BASH_REMATCH[2]}"
		else
			echo "  ERROR dimensions       could not read image dimensions"
			((errors += 1))
			((cases += 1))
			continue
		fi
	fi
	megapixels=$(( (width * height + 999999) / 1000000 ))
	if (( ROBUST_MAX_MPIX > 0 && width * height > ROBUST_MAX_MPIX * 1000000 )); then
		per_profile=$((1 + ${#resize_values[@]} + ${#crop_values[@]} + ${#random_crop_values[@]} * RANDOM_CROP_COUNT))
		transform_count=$((per_profile * ${#profiles[@]}))
		printf '  SKIP  all transforms   %d MP exceeds limit of %d MP (%d profiles)\n' \
			"$megapixels" "$ROBUST_MAX_MPIX" "${#profiles[@]}"
		((skipped += transform_count))
		continue
	fi

	for profile in "${profiles[@]}"; do
		message="$(message_for_profile "$profile")"
		marked="$tmp_dir/marked-$index-$profile.png"
		echo "  Profile: $profile (${#message} bytes)"
		if ! "$PIXSEAL" embed -in "$image" -out "$marked" \
			-key "$TEST_KEY" -message "$message" -profile "$profile" >/dev/null; then
			echo "error: could not create $profile hidden payload for $image" >&2
			exit 2
		fi

		jpeg="$tmp_dir/jpeg-$index-$profile.jpg"
		transform_and_check "jpeg-q$JPEG_QUALITY" "$jpeg" "$message" \
			"$marked" -quality "$JPEG_QUALITY" "$jpeg"

		for percent in "${resize_values[@]}"; do
			resized="$tmp_dir/resize-$index-$profile-$percent.png"
			transform_and_check "resize-${percent}%" "$resized" "$message" \
				"$marked" -resize "${percent}%" "$resized"
		done

		for percent in "${crop_values[@]}"; do
			cropped="$tmp_dir/crop-$index-$profile-$percent.png"
			transform_and_check "crop-${percent}%" "$cropped" "$message" \
				"$marked" -gravity center -crop "${percent}%x${percent}%+0+0" \
				+repage "$cropped"
		done

		# Reset per image/profile so different profiles receive identical crop locations.
		RANDOM=$((RANDOM_SEED + index))
		for percent in "${random_crop_values[@]}"; do
			crop_width=$((width * percent / 100))
			crop_height=$((height * percent / 100))
			max_x=$((width - crop_width))
			max_y=$((height - crop_height))
			for ((random_index = 1; random_index <= RANDOM_CROP_COUNT; random_index++)); do
				random_x=$((RANDOM % (max_x + 1)))
				random_y=$((RANDOM % (max_y + 1)))
				random_crop="$tmp_dir/random-crop-$index-$profile-$percent-$random_index.png"
				transform_and_check "random-${percent}%-$random_index" "$random_crop" "$message" \
					"$marked" -crop "${crop_width}x${crop_height}+${random_x}+${random_y}" \
					+repage "$random_crop"
			done
		done
	done
done

echo
total=$((cases + skipped))
printf 'Robustness summary: %d passed, %d failed, %d timeouts, %d skipped, %d errors, %d total\n' \
	"$passes" "$failures" "$timeouts" "$skipped" "$errors" "$total"

if (( errors > 0 )); then
	exit 2
fi

if [[ "$STRICT" == "1" ]] && (( failures > 0 || timeouts > 0 )); then
	exit 1
fi
