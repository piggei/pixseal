#!/usr/bin/env bash

set -uo pipefail

REPORT="${ALL_TEST_REPORT:-}"
if [[ -n "$REPORT" ]]; then
    mkdir -p "$(dirname "$REPORT")"
    exec > >(tee "$REPORT") 2>&1
fi

default_targets="version-check vet release-unit test-images deep-test extreme-test research-unit lattice-estimator-test homography-test geometry-test affine-test composition-test lattice-test perspective-test core-target-check"
read -r -a targets <<< "${ALL_TEST_TARGETS:-$default_targets}"
if (( ${#targets[@]} == 0 )); then
    echo "error: ALL_TEST_TARGETS resolved to an empty list" >&2
    exit 2
fi

label_for() {
    case "$1" in
        version-check) echo "version consistency" ;;
        vet) echo "go vet" ;;
        release-unit) echo "release-gate Go tests" ;;
        test-images) echo "local image round-trip" ;;
        deep-test) echo "baseline transformations" ;;
        extreme-test) echo "progressive limits" ;;
        research-unit) echo "experimental geometry Go regressions" ;;
        lattice-estimator-test) echo "v0.3 local lattice estimator" ;;
        homography-test) echo "v0.3 bounded homography/projective decoder" ;;
        geometry-test) echo "rotation/combined geometry" ;;
        affine-test) echo "axis-aligned affine" ;;
        composition-test) echo "build-7 composition regression" ;;
        lattice-test) echo "direct lattice bank" ;;
        perspective-test) echo "mild projective perspective" ;;
        core-target-check) echo "desktop/mobile core portability" ;;
        *) echo "$1" ;;
    esac
}

release_targets=(version-check vet release-unit test-images deep-test core-target-check)
research_targets=(extreme-test research-unit lattice-estimator-test homography-test geometry-test affine-test composition-test lattice-test perspective-test)

in_list() {
    local needle="$1"; shift
    local item
    for item in "$@"; do
        [[ "$item" == "$needle" ]] && return 0
    done
    return 1
}

statuses=()
durations=()
started="$(date '+%Y-%m-%d %H:%M:%S %z')"
start_epoch="$(date +%s)"

printf 'PixSeal all-test\n'
if [[ -f VERSION ]]; then printf 'Version: %s\n' "$(head -n 1 VERSION)"; fi
printf 'Started: %s\n' "$started"
printf 'Host: %s\n' "$(uname -srm 2>/dev/null || echo unknown)"
printf 'Go: %s\n' "$(go version 2>/dev/null || echo unavailable)"
if command -v magick >/dev/null 2>&1; then
    printf 'ImageMagick: %s\n' "$(magick -version 2>/dev/null | head -n 1)"
elif command -v convert >/dev/null 2>&1; then
    printf 'ImageMagick: %s\n' "$(convert -version 2>/dev/null | head -n 1)"
fi
printf 'STRICT=%s is used for deep/geometry/affine/composition/lattice/perspective; extreme-test remains non-strict.\n' "${ALL_TEST_STRICT:-1}"
if [[ -n "$REPORT" ]]; then printf 'Report: %s\n' "$REPORT"; fi
printf '\n'

for i in "${!targets[@]}"; do
    target="${targets[$i]}"
    label="$(label_for "$target")"
    printf '%s\n' '========================================================================'
    printf '[%d/%d] %s (%s)\n' "$((i+1))" "${#targets[@]}" "$label" "$target"
    printf '%s\n' '------------------------------------------------------------------------'
    section_start="$(date +%s)"

    case "$target" in
        deep-test|geometry-test|affine-test|composition-test|lattice-test|perspective-test)
            make --no-print-directory STRICT="${ALL_TEST_STRICT:-1}" "$target"
            status=$?
            ;;
        *)
            make --no-print-directory "$target"
            status=$?
            ;;
    esac

    elapsed=$(( $(date +%s) - section_start ))
    durations[$i]="$elapsed"
    if (( status == 0 )); then statuses[$i]="PASS"; else statuses[$i]="FAIL($status)"; fi
    printf '\n[%s] %s — %ss\n\n' "${statuses[$i]}" "$target" "$elapsed"
done

total_elapsed=$(( $(date +%s) - start_epoch ))
printf '%s\n' '========================================================================'
printf 'ALL-TEST SUMMARY\n'
printf '%s\n' '========================================================================'
printf '%-22s %-12s %10s\n' 'Target' 'Result' 'Seconds'
printf '%-22s %-12s %10s\n' '----------------------' '------------' '----------'

failed=0
release_failed=0
research_failed=0
release_selected=0
research_selected=0
for i in "${!targets[@]}"; do
    target="${targets[$i]}"
    status="${statuses[$i]}"
    printf '%-22s %-12s %10s\n' "$target" "$status" "${durations[$i]}"
    if in_list "$target" "${release_targets[@]}"; then
        ((release_selected += 1))
        [[ "$status" == PASS ]] || ((release_failed += 1))
    elif in_list "$target" "${research_targets[@]}"; then
        ((research_selected += 1))
        [[ "$status" == PASS ]] || ((research_failed += 1))
    fi
    [[ "$status" == PASS ]] || ((failed += 1))
done

printf '\nTotal elapsed: %ss\n' "$total_elapsed"

if (( release_selected == 0 )); then
    echo 'Release baseline: NOT RUN'
elif (( release_selected < ${#release_targets[@]} )); then
    if (( release_failed > 0 )); then
        printf 'Release baseline: PARTIAL (%d/%d targets run; %d failed)\n' "$release_selected" "${#release_targets[@]}" "$release_failed"
    else
        printf 'Release baseline: PARTIAL (%d/%d targets run)\n' "$release_selected" "${#release_targets[@]}"
    fi
elif (( release_failed == 0 )); then
    echo 'Release baseline: PASS'
else
    printf 'Release baseline: FAIL (%d release-gate target%s failed)\n' "$release_failed" "$([[ $release_failed -eq 1 ]] && echo '' || echo 's')"
fi

if (( research_selected == 0 )); then
    echo 'Research suites: NOT RUN'
elif (( research_selected < ${#research_targets[@]} )); then
    if (( research_failed > 0 )); then
        printf 'Research suites: PARTIAL/ATTENTION (%d/%d targets run; %d failed)\n' "$research_selected" "${#research_targets[@]}" "$research_failed"
    else
        printf 'Research suites: PARTIAL (%d/%d targets run)\n' "$research_selected" "${#research_targets[@]}"
    fi
elif (( research_failed == 0 )); then
    echo 'Research suites: PASS'
else
    printf 'Research suites: ATTENTION (%d experimental target%s failed)\n' "$research_failed" "$([[ $research_failed -eq 1 ]] && echo '' || echo 's')"
fi

if (( failed > 0 )); then
    printf 'Overall: FAIL (%d target%s failed)\n' "$failed" "$([[ $failed -eq 1 ]] && echo '' || echo 's')"
    exit 1
fi
if (( release_selected < ${#release_targets[@]} || research_selected < ${#research_targets[@]} )); then
    echo 'Overall: PARTIAL (requested target set did not run the complete qualification matrix)'
    exit 0
fi
echo 'Overall: PASS'
