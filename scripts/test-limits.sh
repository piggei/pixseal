#!/usr/bin/env bash

set -uo pipefail

PIXSEAL="${PIXSEAL:-dist/pixseal}"
PICS_DIR="${PICS_DIR:-original pics}"
TEST_KEY="${TEST_KEY:-pixseal-test-key}"
TEST_PROFILES="${TEST_PROFILES:-robust balanced capacity}"
TEST_MESSAGE_ROBUST="${TEST_MESSAGE_ROBUST:-PixSeal robust}"
TEST_MESSAGE_BALANCED="${TEST_MESSAGE_BALANCED:-PixSeal balanced profile test}"
TEST_MESSAGE_CAPACITY="${TEST_MESSAGE_CAPACITY:-PixSeal capacity profile test message for regression coverage}"
ROBUST_MAX_MPIX="${ROBUST_MAX_MPIX:-100}"
EXTRACT_TIMEOUT="${EXTRACT_TIMEOUT:-60}"
LIMIT_START="${LIMIT_START:-95}"
LIMIT_MIN="${LIMIT_MIN:-10}"
LIMIT_STEP="${LIMIT_STEP:-5}"
RANDOM_SEED="${RANDOM_SEED:-20260907}"

if [[ ! -x "$PIXSEAL" ]]; then
	echo "error: PixSeal executable not found: $PIXSEAL" >&2
	exit 2
fi
if [[ ! -d "$PICS_DIR" ]]; then
	echo "error: image directory not found: $PICS_DIR" >&2
	exit 2
fi
if (( LIMIT_STEP <= 0 || LIMIT_START > 100 || LIMIT_START < LIMIT_MIN || LIMIT_MIN <= 0 )); then
	echo "error: invalid LIMIT_START, LIMIT_MIN or LIMIT_STEP" >&2
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

extract_ok() {
	local transformed="$1"
	local expected="$2"
	local output extracted status
	output="$(timeout --foreground "${EXTRACT_TIMEOUT}s" \
		"$PIXSEAL" extract -raw -in "$transformed" -key "$TEST_KEY" 2>/dev/null)"
	status=$?
	if (( status == 124 )); then return 124; fi
	if (( status != 0 )); then return 1; fi
	extracted="$output"
	[[ "$extracted" == "$expected" ]]
}

transform_or_exit() {
	local label="$1"
	shift
	local error_output
	if ! error_output="$("${image_tool[@]}" "$@" 2>&1)"; then
		printf '    ERROR %-18s %s\n' "$label" "${error_output%%$'\n'*}" >&2
		exit 2
	fi
}

report_scales() {
	local label="$1"
	local passes="$2"
	local failures="$3"
	local timeouts="$4"
	printf '    SCALES %-16s passed: %s%%\n' "$label" "${passes// /%, }"
	if [[ -n "$failures" ]]; then
		failures="${failures# }"
		printf '    SCALES %-16s failed: %s%%\n' "$label" "${failures// /%, }"
	else
		printf '    SCALES %-16s failed: none\n' "$label"
	fi
	if [[ -n "$timeouts" ]]; then
		timeouts="${timeouts# }"
		printf '    SCALES %-16s timeout: %s%%\n' "$label" "${timeouts// /%, }"
	else
		printf '    SCALES %-16s timeout: none\n' "$label"
	fi
}

image_index=0
for image in "${images[@]}"; do
	((image_index += 1))
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
			echo "  ERROR dimensions         could not read image dimensions" >&2
			exit 2
		fi
	fi

	megapixels=$(( (width * height + 999999) / 1000000 ))
	if (( ROBUST_MAX_MPIX > 0 && width * height > ROBUST_MAX_MPIX * 1000000 )); then
		printf '  SKIP  all limits        %d MP exceeds limit of %d MP\n' "$megapixels" "$ROBUST_MAX_MPIX"
		continue
	fi

	for profile in "${profiles[@]}"; do
		message="$(message_for_profile "$profile")"
		marked="$tmp_dir/marked-$image_index-$profile.png"
		echo "  Profile: $profile (${#message} bytes)"
		if ! "$PIXSEAL" embed -in "$image" -out "$marked" \
			-key "$TEST_KEY" -message "$message" -profile "$profile" >/dev/null; then
			echo "    ERROR baseline           could not hide payload" >&2
			exit 2
		fi

		resize_passes="100"
		resize_failures=""
		resize_timeouts=""
		for ((percent = LIMIT_START; percent >= LIMIT_MIN; percent -= LIMIT_STEP)); do
			output="$tmp_dir/resize-$image_index-$profile-$percent.png"
			transform_or_exit "resize-${percent}%" "$marked" -resize "${percent}%" "$output"
			if extract_ok "$output" "$message"; then
				printf '    PASS  resize-%-10s message recovered\n' "${percent}%"
				resize_passes="$resize_passes $percent"
			else
				status=$?
				if (( status == 124 )); then
					printf '    TIMEOUT resize-%-7s exceeded %ss\n' "${percent}%" "$EXTRACT_TIMEOUT"
					resize_timeouts="$resize_timeouts $percent"
				else
					printf '    FAIL  resize-%-10s message not recovered\n' "${percent}%"
					resize_failures="$resize_failures $percent"
				fi
			fi
		done
		report_scales "resize" "$resize_passes" "$resize_failures" "$resize_timeouts"

		center_passes="100"
		center_failures=""
		center_timeouts=""
		for ((percent = LIMIT_START; percent >= LIMIT_MIN; percent -= LIMIT_STEP)); do
			crop_width=$((width * percent / 100))
			crop_height=$((height * percent / 100))
			x=$(((width - crop_width) / 2))
			y=$(((height - crop_height) / 2))
			output="$tmp_dir/center-$image_index-$profile-$percent.png"
			transform_or_exit "center-${percent}%" "$marked" \
				-crop "${crop_width}x${crop_height}+${x}+${y}" +repage "$output"
			if extract_ok "$output" "$message"; then
				printf '    PASS  center-crop-%-6s message recovered\n' "${percent}%"
				center_passes="$center_passes $percent"
			else
				status=$?
				if (( status == 124 )); then
					printf '    TIMEOUT center-crop-%-3s exceeded %ss\n' "${percent}%" "$EXTRACT_TIMEOUT"
					center_timeouts="$center_timeouts $percent"
				else
					printf '    FAIL  center-crop-%-6s message not recovered\n' "${percent}%"
					center_failures="$center_failures $percent"
				fi
			fi
		done
		report_scales "center crop" "$center_passes" "$center_failures" "$center_timeouts"

		random_passes="100"
		random_failures=""
		random_timeouts=""
		RANDOM=$((RANDOM_SEED + image_index))
		for ((percent = LIMIT_START; percent >= LIMIT_MIN; percent -= LIMIT_STEP)); do
			crop_width=$((width * percent / 100))
			crop_height=$((height * percent / 100))
			max_x=$((width - crop_width))
			max_y=$((height - crop_height))
			x=$((RANDOM % (max_x + 1)))
			y=$((RANDOM % (max_y + 1)))
			output="$tmp_dir/random-$image_index-$profile-$percent.png"
			transform_or_exit "random-${percent}%" "$marked" \
				-crop "${crop_width}x${crop_height}+${x}+${y}" +repage "$output"
			if extract_ok "$output" "$message"; then
				printf '    PASS  random-crop-%-6s message recovered at +%d+%d\n' "${percent}%" "$x" "$y"
				random_passes="$random_passes $percent"
			else
				status=$?
				if (( status == 124 )); then
					printf '    TIMEOUT random-crop-%-3s exceeded %ss at +%d+%d\n' "${percent}%" "$EXTRACT_TIMEOUT" "$x" "$y"
					random_timeouts="$random_timeouts $percent"
				else
					printf '    FAIL  random-crop-%-6s message not recovered at +%d+%d\n' "${percent}%" "$x" "$y"
					random_failures="$random_failures $percent"
				fi
			fi
		done
		report_scales "random crop" "$random_passes" "$random_failures" "$random_timeouts"
	done
done
