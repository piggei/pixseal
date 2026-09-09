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
    # files can be misnamed, and interpreting arbitrary JPEG bytes as IHDR yields
    # nonsense dimensions.
    local header=()
    read -r -a header <<< "$(od -An -v -tx1 -N24 -- "$image" 2>/dev/null)"
    if (( ${#header[@]} == 24 )) &&
       [[ "${header[*]:0:8}" == "89 50 4e 47 0d 0a 1a 0a" ]] &&
       [[ "${header[12]} ${header[13]} ${header[14]} ${header[15]}" == "49 48 44 52" ]]; then
        width=$(( 16#${header[16]} * 16777216 + 16#${header[17]} * 65536 + 16#${header[18]} * 256 + 16#${header[19]} ))
        height=$(( 16#${header[20]} * 16777216 + 16#${header[21]} * 65536 + 16#${header[22]} * 256 + 16#${header[23]} ))
        if (( width > 0 && height > 0 )); then
            printf '%s %s\n' "$width" "$height"
            return 0
        fi
    fi
    return 1
}
