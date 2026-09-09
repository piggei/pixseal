#!/usr/bin/env bash

# Shared helpers for PixSeal's local ImageMagick-based test suites.
# The scripts normally ask ImageMagick for dimensions. Very large PNG files can
# exceed ImageMagick resource-policy limits even with -ping; in that case read
# the PNG IHDR width/height directly without decoding pixel data.
read_image_dimensions() {
    local image="$1" output width height
    if output="$("${identify_tool[@]}" -ping -format '%w %h\n' "$image" 2>/dev/null)"; then
        read -r width height <<< "$output"
        if [[ "$width" =~ ^[0-9]+$ && "$height" =~ ^[0-9]+$ ]]; then
            printf '%s %s\n' "$width" "$height"
            return 0
        fi
    fi

    # Large PNGs can exceed ImageMagick policy limits. Fall back to the PNG
    # header only after verifying both the eight-byte signature and the IHDR
    # chunk type; never trust the filename extension alone.
    local bytes=()
    read -r -a bytes <<< "$(od -An -v -tx1 -N24 -- "$image" 2>/dev/null)"
    if (( ${#bytes[@]} == 24 )) &&
       [[ "${bytes[0]} ${bytes[1]} ${bytes[2]} ${bytes[3]} ${bytes[4]} ${bytes[5]} ${bytes[6]} ${bytes[7]}" == "89 50 4e 47 0d 0a 1a 0a" ]] &&
       [[ "${bytes[12]} ${bytes[13]} ${bytes[14]} ${bytes[15]}" == "49 48 44 52" ]]; then
        width=$(( 16#${bytes[16]} * 16777216 + 16#${bytes[17]} * 65536 + 16#${bytes[18]} * 256 + 16#${bytes[19]} ))
        height=$(( 16#${bytes[20]} * 16777216 + 16#${bytes[21]} * 65536 + 16#${bytes[22]} * 256 + 16#${bytes[23]} ))
        if (( width > 0 && height > 0 )); then
            printf '%s %s\n' "$width" "$height"
            return 0
        fi
    fi
    return 1
}
