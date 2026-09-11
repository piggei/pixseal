#!/usr/bin/env bash
set -euo pipefail

row() {
    printf '  %-24s %-14s %-18s %s\n' "$1" "$2" "$3" "$4"
}

section() {
    printf '\n%s\n' "$1"
    printf '  %-24s %-14s %-18s %s\n' 'TARGET' 'CLASS' 'INPUT' 'DESCRIPTION'
    printf '  %-24s %-14s %-18s %s\n' '------------------------' '--------------' '------------------' '-----------'
}

printf 'PixSeal available test/check targets\n'
printf 'Legend: INPUT=pics means the local qualification corpus; private means files are never shipped.\n'
printf 'Research/qualification targets may intentionally fail at current robustness boundaries; release-check is the production gate.\n'

section 'Release / baseline'
row version-check RELEASE self 'VERSION/buildinfo consistency'
row vet RELEASE source 'go vet on all packages'
row release-unit RELEASE source 'release-scoped Go unit/regression gate'
row test-images RELEASE pics 'authenticated round-trip on each image/profile'
row deep-test QUALIFICATION pics 'strict baseline JPEG/resize/crop suite; corpus-sensitive'
row core-target-check PORTABILITY source 'compile reusable core for Linux/Windows/Android/iOS'
row release-check AGGREGATE source+pics 'complete production release baseline gate'

section 'Research / diagnostics'
row research-unit RESEARCH source 'experimental geometry Go regressions'
row lattice-estimator-test RESEARCH source 'local lattice estimator regressions'
row homography-test RESEARCH source 'bounded homography/projective regressions'
row photometric-test RESEARCH source 'bounded photometric bank regressions'
row bit-channel-test RESEARCH source 'protected-bit/ECC diagnostics'
row reliability-test RESEARCH source 'soft Hamming/full-grid regressions'
row spatial-channel-test RESEARCH source 'spatial coded-bit stability regressions'
row phase-surface-test RESEARCH source 'confidence-weighted phase surface regressions'
row blind-phase-test RESEARCH source 'repetition + cross-cell blind phase regressions'
row lattice-phase-test RESEARCH source 'local fractional lattice-phase regressions'
row global-unwrap-test RESEARCH source 'exact global +/-1 unwrap + exact top-2/split-repetition/rollback regressions'
row smooth-phase-test RESEARCH source 'bounded smooth phase-field regressions'
row geometry-test RESEARCH pics 'strict rotation/combined geometry suite; corpus-sensitive'
row affine-test RESEARCH pics 'strict axis-aligned affine suite; corpus-sensitive'
row composition-test RESEARCH pics 'strict anisotropic-scale + rotation suite; corpus-sensitive'
row lattice-test RESEARCH pics 'strict direct lattice-basis composition suite; corpus-sensitive'
row perspective-test RESEARCH pics 'strict mild projective perspective suite; corpus-sensitive'

section 'Private physical-channel'
row print-camera-test PRIVATE private/print-camera 'print -> paper -> smartphone; PASS requires Format-v3 HMAC'
row print-scan-test PRIVATE private/print-scan 'print -> scanner; PASS requires Format-v3 HMAC'

section 'Exploratory / compatibility'
row extreme-test QUALIFICATION pics 'progressive resize/crop limit map; non-strict by design'
row test-unit COMPATIBILITY source 'complete go test ./... suite'
row test AGGREGATE source+pics 'build + test-unit + image round-trips'
row all AGGREGATE source+pics 'test + baseline transformation suite'
row all-test AGGREGATE source+pics '24-target qualification matrix with final summary'

printf '\nTip: use make <target>. For all-test, optionally set ALL_TEST_REPORT=path/to/report.txt.\n'
