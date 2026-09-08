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

    # PNG signature + IHDR: width and height are big-endian uint32 at bytes 16..23.
    local ext="${image##*.}"
    if [[ "${ext,,}" == "png" ]]; then
        local bytes=()
        read -r -a bytes <<< "$(od -An -v -tx1 -j16 -N8 -- "$image" 2>/dev/null)"
        if (( ${#bytes[@]} == 8 )); then
            width=$(( 16#${bytes[0]} * 16777216 + 16#${bytes[1]} * 65536 + 16#${bytes[2]} * 256 + 16#${bytes[3]} ))
            height=$(( 16#${bytes[4]} * 16777216 + 16#${bytes[5]} * 65536 + 16#${bytes[6]} * 256 + 16#${bytes[7]} ))
            if (( width > 0 && height > 0 )); then
                printf '%s %s\n' "$width" "$height"
                return 0
            fi
        fi
    fi
    return 1
}
