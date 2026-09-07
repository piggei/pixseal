#!/usr/bin/env bash

set -uo pipefail

PIXSEAL="${PIXSEAL:-dist/pixseal}"
PICS_DIR="${PICS_DIR:-original pics}"
TEST_KEY="${TEST_KEY:-pixseal-test-key}"
TEST_MESSAGE="${TEST_MESSAGE:-PixSeal image test}"
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

mapfile -d '' images < <(find "$PICS_DIR" -maxdepth 1 -type f \
	\( -iname '*.jpg' -o -iname '*.jpeg' -o -iname '*.png' \) -print0 | sort -z)
if (( ${#images[@]} == 0 )); then
	echo "error: no JPEG or PNG images found in $PICS_DIR" >&2
	exit 2
fi

tmp_dir="$(mktemp -d)"
trap 'rm -rf -- "$tmp_dir"' EXIT
RANDOM="$RANDOM_SEED"

extract_ok() {
	local transformed="$1"
	local output extracted status
	output="$(timeout --foreground "${EXTRACT_TIMEOUT}s" \
		"$PIXSEAL" extract -in "$transformed" -key "$TEST_KEY" 2>/dev/null)"
	status=$?
	if (( status == 124 )); then return 124; fi
	if (( status != 0 )); then return 1; fi
	extracted="${output%%$'\n'*}"
	[[ "$extracted" == "$TEST_MESSAGE" ]]
}

transform_or_exit() {
	local label="$1"
	shift
	local error_output
	if ! error_output="$("${image_tool[@]}" "$@" 2>&1)"; then
		printf '  ERROR %-18s %s\n' "$label" "${error_output%%$'\n'*}" >&2
		exit 2
	fi
}

report_boundary() {
	local label="$1"
	local last_pass="$2"
	local first_fail="$3"
	local outcome="${4:-fail}"
	if [[ "$outcome" == "timeout" ]]; then
		printf '  LIMIT %-17s inconclusive: timeout at %s%% after pass at %s%%\n' "$label" "$first_fail" "$last_pass"
	elif [[ -n "$first_fail" ]]; then
		printf '  LIMIT %-17s last pass %s%%, first fail %s%%\n' "$label" "$last_pass" "$first_fail"
	else
		printf '  LIMIT %-17s no failure down to %s%%\n' "$label" "$LIMIT_MIN"
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
	read -r width height < <("${identify_tool[@]}" -ping -format '%w %h\n' "$image" 2>/dev/null) || true
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

	marked="$tmp_dir/marked-$image_index.png"
	if ! "$PIXSEAL" embed -in "$image" -out "$marked" \
		-key "$TEST_KEY" -message "$TEST_MESSAGE" >/dev/null; then
		echo "  ERROR baseline           could not embed watermark" >&2
		exit 2
	fi

	resize_passes="100"
	resize_failures=""
	resize_timeouts=""
	for ((percent = LIMIT_START; percent >= LIMIT_MIN; percent -= LIMIT_STEP)); do
		output="$tmp_dir/resize-$image_index-$percent.png"
		transform_or_exit "resize-${percent}%" "$marked" -resize "${percent}%" "$output"
		if extract_ok "$output"; then
			printf '  PASS  resize-%-10s message recovered\n' "${percent}%"
			resize_passes="$resize_passes $percent"
		else
			status=$?
			if (( status == 124 )); then
				printf '  TIMEOUT resize-%-7s exceeded %ss\n' "${percent}%" "$EXTRACT_TIMEOUT"
				resize_timeouts="$resize_timeouts $percent"
			else
				printf '  FAIL  resize-%-10s message not recovered\n' "${percent}%"
				resize_failures="$resize_failures $percent"
			fi
		fi
	done
	printf '  SCALES resize            passed: %s%%\n' "${resize_passes// /%, }"
	if [[ -n "$resize_failures" ]]; then
		resize_failures="${resize_failures# }"
		printf '  SCALES resize            failed: %s%%\n' "${resize_failures// /%, }"
	else
		echo "  SCALES resize            failed: none"
	fi
	if [[ -n "$resize_timeouts" ]]; then
		resize_timeouts="${resize_timeouts# }"
		printf '  SCALES resize            timeout: %s%%\n' "${resize_timeouts// /%, }"
	else
		echo "  SCALES resize            timeout: none"
	fi

	last_pass=100
	first_fail=""
	boundary_outcome="fail"
	for ((percent = LIMIT_START; percent >= LIMIT_MIN; percent -= LIMIT_STEP)); do
		crop_width=$((width * percent / 100))
		crop_height=$((height * percent / 100))
		x=$(((width - crop_width) / 2))
		y=$(((height - crop_height) / 2))
		output="$tmp_dir/center-$image_index-$percent.png"
		transform_or_exit "center-${percent}%" "$marked" \
			-crop "${crop_width}x${crop_height}+${x}+${y}" +repage "$output"
		if extract_ok "$output"; then
			printf '  PASS  center-crop-%-6s message recovered\n' "${percent}%"
			last_pass="$percent"
		else
			status=$?
			if (( status == 124 )); then
				printf '  TIMEOUT center-crop-%-3s exceeded %ss\n' "${percent}%" "$EXTRACT_TIMEOUT"
				boundary_outcome="timeout"
			else
				printf '  FAIL  center-crop-%-6s message not recovered\n' "${percent}%"
			fi
			first_fail="$percent"
			break
		fi
	done
	report_boundary "center crop" "$last_pass" "$first_fail" "$boundary_outcome"

	last_pass=100
	first_fail=""
	boundary_outcome="fail"
	for ((percent = LIMIT_START; percent >= LIMIT_MIN; percent -= LIMIT_STEP)); do
		crop_width=$((width * percent / 100))
		crop_height=$((height * percent / 100))
		max_x=$((width - crop_width))
		max_y=$((height - crop_height))
		x=$((RANDOM % (max_x + 1)))
		y=$((RANDOM % (max_y + 1)))
		output="$tmp_dir/random-$image_index-$percent.png"
		transform_or_exit "random-${percent}%" "$marked" \
			-crop "${crop_width}x${crop_height}+${x}+${y}" +repage "$output"
		if extract_ok "$output"; then
			printf '  PASS  random-crop-%-6s message recovered at +%d+%d\n' "${percent}%" "$x" "$y"
			last_pass="$percent"
		else
			status=$?
			if (( status == 124 )); then
				printf '  TIMEOUT random-crop-%-3s exceeded %ss at +%d+%d\n' "${percent}%" "$EXTRACT_TIMEOUT" "$x" "$y"
				boundary_outcome="timeout"
			else
				printf '  FAIL  random-crop-%-6s message not recovered at +%d+%d\n' "${percent}%" "$x" "$y"
			fi
			first_fail="$percent"
			break
		fi
	done
	report_boundary "random crop" "$last_pass" "$first_fail" "$boundary_outcome"
done
