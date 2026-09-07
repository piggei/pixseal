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

mapfile -d '' images < <(find "$PICS_DIR" -maxdepth 1 -type f \
	\( -iname '*.jpg' -o -iname '*.jpeg' -o -iname '*.png' \) -print0 | sort -z)
if (( ${#images[@]} == 0 )); then
	echo "error: no JPEG or PNG images found in $PICS_DIR" >&2
	exit 2
fi

read -r -a profiles <<< "$TEST_PROFILES"
read -r -a angles <<< "$GEOMETRY_ANGLES"
if (( ${#profiles[@]} == 0 )); then
	echo "error: TEST_PROFILES is empty" >&2
	exit 2
fi
if (( ${#angles[@]} == 0 )); then
	echo "error: GEOMETRY_ANGLES is empty" >&2
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
for angle in "${angles[@]}"; do
	if [[ ! "$angle" =~ ^-?[0-9]+([.][0-9]+)?$ ]]; then
		echo "error: invalid GEOMETRY_ANGLES entry: $angle" >&2
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

for image in "${images[@]}"; do
	((index += 1))
	name="$(basename "$image")"
	width=""
	height=""
	read -r width height < <("${identify_tool[@]}" -ping -format '%w %h\n' "$image" 2>/dev/null) || true
	if [[ ! "$width" =~ ^[0-9]+$ || ! "$height" =~ ^[0-9]+$ ]]; then
		echo "Image: $name"
		echo "  ERROR dimensions       could not read image dimensions"
		((errors += 1))
		((cases += 1))
		continue
	fi
	megapixels=$(( (width * height + 999999) / 1000000 ))
	if (( GEOMETRY_MAX_MPIX > 0 && width * height > GEOMETRY_MAX_MPIX * 1000000 )); then
		count=$(( ${#profiles[@]} * ${#angles[@]} ))
		printf 'Image: %s\n  SKIP  all rotations    %d MP exceeds limit of %d MP (%d cases)\n' \
			"$name" "$megapixels" "$GEOMETRY_MAX_MPIX" "$count"
		((skipped += count))
		continue
	fi

	echo
	echo "Image: $name"
	for profile in "${profiles[@]}"; do
		message="$(message_for_profile "$profile")"
		marked="$tmp_dir/marked-$index-$profile.png"
		if ! "$PIXSEAL" embed -in "$image" -out "$marked" \
			-key "$TEST_KEY" -message "$message" -profile "$profile" >/dev/null; then
			echo "error: could not create $profile hidden payload for $image" >&2
			exit 2
		fi
		echo "  Profile: $profile (${#message} bytes)"
		for angle in "${angles[@]}"; do
			safe_angle="${angle//-/m}"
			safe_angle="${safe_angle//./p}"
			rotated="$tmp_dir/rotate-$index-$profile-$safe_angle.png"
			((cases += 1))
			if ! transform_error="$("${image_tool[@]}" "$marked" -background white -alpha remove -alpha off -rotate "$angle" "$rotated" 2>&1)"; then
				printf '    ERROR rotate-%-7s ImageMagick transformation failed\n' "${angle}deg"
				printf '          %s\n' "${transform_error%%$'\n'*}"
				((errors += 1))
				continue
			fi
			output="$(timeout --foreground "${EXTRACT_TIMEOUT}s" \
				"$PIXSEAL" extract -in "$rotated" -key "$TEST_KEY" 2>&1)"
			status=$?
			if (( status == 0 )); then
				extracted="${output%%$'\n'*}"
				if [[ "$extracted" == "$message" ]]; then
					correction="$(printf '%s\n' "$output" | sed -n 's/^rotation-correction: //p' | head -n 1)"
					if [[ -n "$correction" ]]; then
						printf '    PASS  rotate-%-7s message recovered (%s)\n' "${angle}deg" "$correction"
					else
						printf '    PASS  rotate-%-7s message recovered\n' "${angle}deg"
					fi
					((passes += 1))
					continue
				fi
			fi
			if (( status == 124 )); then
				printf '    TIMEOUT rotate-%-6s exceeded %ss\n' "${angle}deg" "$EXTRACT_TIMEOUT"
				((timeouts += 1))
			else
				printf '    FAIL  rotate-%-7s message not recovered\n' "${angle}deg"
				((failures += 1))
			fi
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
