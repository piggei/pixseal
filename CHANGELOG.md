# Changelog

All notable changes to PixSeal are documented here.

## v0.2.0 build 4 - 2026-09-07

Development build. Not a final v0.2.0 release.

### Combined geometric recovery

- Kept format v3 unchanged and extended only the decoder geometry layer.
- Added multi-lattice arbitrary-angle detection for the native 8-pixel lattice
  and the 6-pixel lattice produced by 75% resize.
- Normalized orientation contrast by block area before comparing 8-pixel and
  6-pixel candidates.
- Added experimental recovery for rotation combined with 75% resize and crop,
  including both transformation orders and a rotate -> resize75 -> crop chain.
- Extended exact quarter-turn recovery to all existing direct 8/6/4-pixel block
  sizes, retaining 100%, 75% and 50% direct-resize compatibility.
- Deliberately did not claim arbitrary-angle + 50% resize: exact-angle
  development checks still failed after the double interpolation at default
  strength 24.

### Bounded search and failed-extraction control

- Arbitrary-angle estimation now uses at most two zero-degree checks, 720
  non-zero quarter-degree coarse probes and 33 local 0.05-degree refinements.
- At most two refined candidates reach authenticated v3 decoding.
- Added strong-rotation early termination after failed authenticated decode so a
  clearly rotated wrong-key carrier does not fall through into irrelevant
  pure-resize normalization.
- The theoretical full-grid maximum is now 1013 candidates when every gated
  optional branch is counted; normal successful paths exit much earlier.

### Geometry regression suite

- Added Go regression coverage for rotate+resize75 and rotate+crop recovery.
- Extended `make geometry-test` with configurable combined modes via
  `GEOMETRY_COMBINED_ANGLES` and `GEOMETRY_COMBINED_MODES`.
- Default combined modes cover rotate/resize order, rotate/crop order and a
  rotate+resize75+crop chain while keeping generated images temporary.

### Documentation

- Updated README, algorithm specification, results notes, CLI help and version
  metadata to `v0.2.0 build 4`.
- Kept affine, perspective and print-camera recovery as subsequent research
  targets; build 4 is the controlled orientation+scale+phase baseline.

## v0.2.0 build 3 - 2026-09-07

Development build. Not a final v0.2.0 release.

### Bounded digital rotation recovery

- Added automatic recovery for exact 90, 180 and 270 degree rotations using
  lossless quarter-turn correction.
- Added experimental arbitrary-angle recovery without changing format v3.
- Added a two-stage DCT lattice-orientation estimator: a fixed coarse probe from
  -45 to +45 degrees at 0.25-degree increments, followed by local 0.05-degree
  refinement of at most two strong candidates.
- Arbitrary-angle probes measure the phase contrast of the same DCT coefficient
  separation imposed by PixSeal embedding; they do not perform a full payload
  decode for every angle.
- Rotation is estimated modulo 90 degrees and authenticated v3 decoding resolves
  the final quadrant through bounded quarter-turn variants.
- Added a 50-million-pixel safety check to expanded rotation rectification
  canvases.
- `ExtractInfo` now reports `RotationCorrectionDegrees`; the CLI prints the
  applied correction when a rotated carrier is recovered.

### Geometry tests

- Added unit coverage for quarter turns, fractional arbitrary rotations across
  robust/balanced/capacity profiles, unmarked-image probe rejection and bounded
  wrong-key failure on a rotated carrier.
- Added `make geometry-test`, an ImageMagick-backed private-corpus matrix for
  digital rotations. It is intentionally excluded from `make all` so the
  historical JPEG/resize/crop baseline remains unchanged.
- Kept combined arbitrary rotation + resize/crop, affine, perspective and
  print-camera recovery as later research targets.

### Search bounds

- Native crop/resize search remains bounded to the previous 153 candidates.
- The rotation estimator performs at most 361 coarse + 22 refinement sparse
  angle probes.
- At most two arbitrary-angle rectifications are passed to full v3 decoding;
  including optional quarter-turn and scale branches, the theoretical maximum is
  857 full grid candidates plus the fixed sparse orientation probes.
- Successful unrotated extraction still returns before any rotation work.

### Documentation

- Updated README, algorithm specification, results notes and help output for the
  new rotation layer and next-step geometry roadmap.
- Runtime format remains v3-only; no v1/v2 compatibility code was reintroduced.

## v0.2.0 build 2 - 2026-09-07

Development build. Not a final v0.2.0 release.

### v3-only runtime

- Removed the experimental v1 and v2 runtime decoders. Neither format had a known
  external carrier population or interoperability requirement, so format v3 is
  now the sole extraction format.
- Retained the format number `3`; v1 and v2 remain part of the documented
  engineering history rather than supported carrier formats.
- Simplified extraction APIs and CLI options accordingly: `Extract` no longer
  accepts unused decoder options, `capacity` no longer accepts a legacy
  repetition argument, and `-repetition` / extract-side `-strength` were removed.
- Removed legacy-only implementation and regression code, reducing the number of
  branches exercised during failed extraction.

### Repository hygiene

- Added `*Zone.Identifier` and `*:Zone.Identifier` to `.gitignore` to ignore
  Windows/WSL downloaded-file metadata artifacts.
- Renamed the shared modern decoder source to the v3-focused decoder path and
  removed obsolete legacy source files.

### Geometry roadmap

- Documented arbitrary digital rotation as the next synchronization target,
  followed by combined rotation/resize/crop, affine and perspective recovery.
- Documented the longer-term experimental print-camera goal: recover an
  authenticated PixSeal message after printing the carrier and photographing it
  with a phone.
- These geometry features are roadmap items only and are not claimed as
  supported by build 2.

### Validation

- Kept the fixed geometric search bound at a maximum of 153 candidates before
  image-size skips.
- Retained adaptive profile round trips, transformation tests, wrong-key and
  unmarked-image timing guards, CLI analysis tests and cross-platform builds.
- Updated documentation and version metadata consistently to `v0.2.0 build 2`.

## v0.2.0 build 1 - 2026-09-07

Development build. Not a final v0.2.0 release.

### Adaptive v3 format

- Added format v3 with adaptive `robust`, `balanced` and `capacity` profiles.
- `auto` selects the most robust profile that can contain the requested payload:
  1-16 bytes -> robust, 17-32 -> balanced, 33-64 -> capacity.
- Maximum payload remains 64 bytes.
- Preserved the 8-byte header and 8-byte truncated HMAC overhead while packing
  v3 format and profile identity into the former version byte.
- Preserved CRC32, truncated HMAC-SHA256 authentication, key-derived whitening
  and Hamming(7,4) error correction.
- Reused the fixed 35x32 / 1120-position tile. Protected v3 bits are dispersed
  with modular stride 251, producing 2.50x, 1.67x and 1.00x mean per-tile
  observation ratios for robust, balanced and capacity respectively.
- Repeated v3 observations are soft-combined before Hamming decoding.
- Profile identification is automatic and authenticated; extraction never
  requires the user to supply a profile.

### Extraction and compatibility

- Modern extraction now probes v3 before v2 on each shared geometric candidate
  and falls back to v1 only after modern extraction fails.
- The expensive DCT grid/normalization work is shared between v3 and v2 rather
  than duplicated as two geometric searches.
- Geometric search remains explicitly bounded: at most 116 direct grids, 13
  aligned inverse-normalizations and 24 +/-1 dimension neighbors before
  image-size skips.
- Replaced full sorting of 1120 phase candidates with bounded top-4 selection,
  restoring failed-extraction performance to approximately the v0.1 level in
  development spot checks.
- Added explicit regression coverage for v3, v2 and v1 extraction.

### CLI

- Added `embed -profile auto|robust|balanced|capacity`; default is `auto`.
- `embed` reports the concrete selected profile.
- Explicit-profile capacity errors report payload bytes, profile capacity and
  the minimum compatible profile when one exists.
- Added `analyze -in ... -message ...` and `analyze -in ... -bytes N`.
- `analyze` reports deterministic profile/geometry math separately from
  heuristic image-detail and strength recommendations.
- Added `capacity -details` for per-profile capacities and image dimensions.
- Preserved the historical default `capacity` output (`N bytes`) for script
  compatibility.

### Analysis heuristic

- Added bounded local-luminance-gradient sampling with no runtime dependencies.
- Build 1 classifies detail as low/medium/high and recommends strengths 20/24/28
  respectively.
- Documentation explicitly marks the detail/strength model as heuristic rather
  than a robustness guarantee.

### Tests and development workflow

- Added profile threshold tests at 16, 17, 32, 33 and 64 bytes plus 65-byte
  rejection.
- Added maximum-capacity round trips for all three profiles.
- Added JPEG, resize and crop tests for each v3 profile.
- Added wrong-key, unmarked-image, automatic-profile and CLI-analysis tests.
- Added consistency tests across `analyze`, `capacity` and `embed`.
- Local shell suites now exercise all concrete v3 profiles with
  profile-appropriate payloads.
- Restored the requested development semantics: `make` builds only; `make test`
  runs unit tests plus local image round trips; `make deep-test` runs the baseline
  transformation matrix; `make extreme-test` explores progressive limits; and
  `make all` combines build/test/deep-test.

### Documentation

- Reframed PixSeal consistently as robust steganography for short messages; DCT
  watermarking remains the technical embedding mechanism rather than the goal.
- Documented the exact v3 frame/profile byte layout, redundancy math, modular
  tile mapping, sync scoring and bounded search space.
- Reiterated that whitening is not encryption and sensitive payloads should be
  encrypted before embedding.
- Reiterated experimental status and the absence of independent cryptographic or
  steganalytic audit.

## v0.1.0 - 2026-09-07

Initial public release.

### Features

- Pure-Go command-line application with `embed`, `extract` and `capacity`.
- Robust image steganography for hiding a small authenticated payload in DCT
  luminance coefficients.
- PNG and JPEG input, with lossless PNG output for newly sealed carrier images.
- Authenticated payloads up to 64 bytes using CRC32 and truncated HMAC-SHA256.
- Key-derived whitening and Hamming(7,4) error correction.
- Periodic two-dimensional DCT tile with geometric synchronization.
- Recovery after JPEG recompression, supported resizing and off-grid cropping.
- Legacy v1 extraction compatibility, covered by an explicit regression test.
- Protected output writes and cross-platform build targets.

### Release hardening

- Project terminology changed from ownership watermarking to robust image
  steganography.
- The relationship to the original SynthID inspiration was clarified: PixSeal is
  independent and does not implement Google's SynthID algorithm.
- Crop geometry, normalization limits, 8-bit output conversion and metadata loss
  were documented.
- Failed extraction was optimized through compact cached pixel data and bounded
  scale candidate ranking.

See `docs/RESULTS.md` for the historical v0.1 baseline measurements.
