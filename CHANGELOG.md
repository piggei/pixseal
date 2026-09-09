# Changelog

All notable PixSeal changes are documented here. Historical build entries are
summaries; current behavior is defined by README and ALGORITHM.

## v0.2.0-rc2 - 2026-09-09

Release-hardening candidate produced after an independent audit and a second
verification pass.

### Correctness and safety

- Reject non-finite embedding strengths (`NaN`, `+Inf`, `-Inf`).
- Reject explicit CLI `-strength 0`; omitting the flag still selects default 24.
  The Go API retains zero as its internal default sentinel.
- Add a 300 MP DecodeConfig/core source guard before PixSeal's heavy RGB working
  allocations.
- Preflight output existence before image decode/embed work.
- Use `Lstat` semantics so dangling symlinks count as existing targets.
- Without `-force`, use a no-clobber commit that cannot silently replace a
  racing destination.
- With `-force`, replace regular files only; refuse directories, symlinks, FIFOs
  and devices.
- Stop forcing new output files to 0644; new files keep private temporary-file
  permissions, while forced replacement preserves existing regular-file mode.
- Sync encoded temporary output before commit.
- Restrict the backup/restore rename fallback to Windows and document its
  crash-durability limitation.
- Fix dotfile output naming (`.sealed` -> `.sealed.png`).
- Add `extract -raw`: exact payload bytes on stdout, diagnostics on stderr.
- Align `analyze` alpha handling with the encoder's white compositing.
- Force `GOOS=windows`, `GOARCH=amd64`, `CGO_ENABLED=0` in `build-windows.bat`.

### Tests and tooling

- Repair `test-perspective.sh` ImageMagick identify setup and validation.
- Fail on an empty perspective corpus instead of reporting a false green.
- Make perspective error/case accounting coherent.
- Restrict positive perspective modes to the two decoder hypotheses actually
  shipped in v0.2.
- Require the perspective shell test to observe the matching
  `perspective-correction` path.
- Add a deterministic Go end-to-end mild-perspective recovery regression.
- Validate the PNG signature and IHDR chunk before using the large-PNG header
  fallback.
- Add file-safety, explicit-strength, raw multiline-payload, large-source and
  analyzer-alpha regressions.
- Add `make version-check` and include it in the release baseline/all-test report.
- Split deterministic Go tests into `release-unit` and `research-unit` for release qualification while preserving historical `make test` semantics.
- Correct all-test documentation: `extreme-test` is a non-strict boundary map;
  strict semantics apply only to suites that implement them.

### Cleanup and documentation

- Remove unused `scaleCandidate` and obsolete bicubic normalization code/tests.
- Correct stale two/four-shape comments and test diagnostics.
- Rewrite README around v0.2.0 current behavior rather than build-9/build-10
  historical descriptions.
- Rewrite ALGORITHM as the current Format-v3/decoder specification, including
  exact frame-tag placement and current bounded search budgets.
- Reorganize RESULTS so release qualification is the first status presented.
- Reorder HISTORY chronologically and record rejected experiments explicitly.
- Reduce TODO to open work only.
- Clarify EXIF Orientation, ICC/color-management, Windows replacement durability
  and print-camera limitations.

### Compatibility

- Format v3 is unchanged.
- Profile IDs, capacities, whitening, Hamming protection, tile mapping, DCT
  embedding and encoder fingerprints remain the v0.2 compatibility contract.

## v0.2.0-rc1 - 2026-09-08

- Consolidated the build-11 algorithm as the v0.2 release candidate.
- Added `make release-check` and split release-baseline versus research-suite
  reporting in `make all-test`.
- Froze Format v3 and deterministic encoder fingerprints.
- Qualified the release baseline on the private Linux corpus: `deep-test` 72/72,
  composition 2/2, lattice 8/8, mild perspective 4/4; arbitrary geometry and
  affine retained documented research limits.

## v0.2.0 build 11 - 2026-09-08

- Added virtual homography sampling with two bounded vertical-keystone
  hypotheses (`top-narrow-4`, `bottom-narrow-4`).
- Added `perspective-test` and `perspective-correction` diagnostics.
- Kept general perspective/print-camera recovery explicitly out of scope.

## v0.2.0 build 10 - 2026-09-08

- Restored fractional pure-resize recovery before speculative geometry.
- Added virtual isotropic-scale search plus a single targeted bilinear
  normalization fallback.
- Added deterministic v3 encoder pixel fingerprints.
- Added large-PNG IHDR dimension probing to geometry scripts.

## v0.2.0 build 9 - 2026-09-08

- Expanded the fixed direct lattice-basis bank to 110×90, 90×110, 105×95 and
  95×105 before rotation.
- Added `make all-test` comparative reporting.
- Rejected a more continuous local-basis prototype because negative-case runtime
  was unacceptable.

## v0.2.0 build 8 - 2026-09-08

- Generalized direct lattice composition to symmetric 110×90 / 90×110 bases.

## v0.2.0 build 7 - 2026-09-08

- Replaced a rotation-first composed-geometry heuristic with direct lattice
  scoring after real-corpus FAIL/TIMEOUT results.

## v0.2.0 build 6 - 2026-09-07

- Demonstrated the first bounded anisotropic-scale + rotation composition.

## v0.2.0 build 5 - 2026-09-07

- Added bounded axis-aligned affine recovery via virtual sampling.

## v0.2.0 build 4 - 2026-09-07

- Added selected rotation + resize/crop recovery experiments.

## v0.2.0 build 3 - 2026-09-07

- Added exact quarter-turn and arbitrary digital rotation recovery.

## v0.2.0 build 2 - 2026-09-07

- Made runtime extraction Format-v3-only; unreleased v1/v2 retained as history.

## v0.2.0 build 1 - 2026-09-07

- Introduced adaptive Format v3 profiles and automatic profile selection.

## v0.1.0 - 2026-09-07

- First stable PixSeal line with DCT embedding, authentication/ECC and digital
  crop/resize robustness experiments.
