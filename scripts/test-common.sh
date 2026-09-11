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

    # PNG signature + first IHDR chunk. Never trust the extension alone: camera
    # files can be misnamed. Use decimal bytes for the dimensions so this path
    # is independent of shell hexadecimal parsing quirks on older hosts.
    local sig dims=()
    sig="$(od -An -v -tx1 -N8 -- "$image" 2>/dev/null | tr -d ' \n')"
    if [[ "$sig" == "89504e470d0a1a0a" ]]; then
        read -r -a dims <<< "$(od -An -v -tu1 -j16 -N8 -- "$image" 2>/dev/null)"
        if (( ${#dims[@]} == 8 )); then
            width=$(( dims[0] * 16777216 + dims[1] * 65536 + dims[2] * 256 + dims[3] ))
            height=$(( dims[4] * 16777216 + dims[5] * 65536 + dims[6] * 256 + dims[7] ))
            if (( width > 0 && height > 0 )); then
                printf '%s %s\n' "$width" "$height"
                return 0
            fi
        fi
    fi
    return 1
}
