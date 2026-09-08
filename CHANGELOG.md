# Changelog

All notable changes to PixSeal are documented here.

## v0.2.0-rc1 - 2026-09-08

Release candidate for the v0.2.0 line. No new codec or geometric-recovery
algorithm is introduced relative to build 11.

### Release consolidation

- Froze Format v3, adaptive profile semantics and deterministic encoder fingerprints for release validation.
- Promoted the build-10 pure-resize recovery path as part of the release baseline.
- Retained build-11 bounded projective probing as an explicitly experimental capability, not general perspective support.
- Added `make release-check` for the release baseline: vet, unit/image round trips, baseline JPEG/resize/crop transforms and reusable-core portability.
- Updated `make all-test` reporting to distinguish **Release baseline** from **Research suites** while preserving a non-zero overall exit status for any strict suite failure.
- Clarified that arbitrary geometry, affine/lattice composition and perspective results are corpus-specific experimental measurements, not recovery guarantees.
- Moved direct-lattice/homography/print-camera work to the post-v0.2.0 research roadmap.
- Audited README, algorithm specification, results, history, TODO, version metadata and repository contents for RC consistency.

## v0.2.0 build 11 - 2026-09-08

Development build. Not a final v0.2.0 release.

### First bounded projective recovery

- Kept Format v3, adaptive profiles and the encoder bit-for-bit unchanged.
- Added a virtual homography sampler and a deliberately small two-hypothesis vertical-keystone bank: top edge -4% and bottom edge -4%.
- Mature direct, pure-resize and axis-aligned-affine recovery retain priority; projective probing runs before the more expensive lattice/rotation heuristics.
- The projective bank performs no bitmap rectification and authenticates only the aligned phase, keeping the new stage explicitly bounded.
- Added `perspective-correction` metadata to successful extraction output.
- Added `make perspective-test` and included it in `make all-test`.
- ImageMagick-generated LQ/MQ regression tests recovered 4/4 mild-perspective cases in the development environment; the private ~201 MP HQ image is skipped by the configured geometry limit.
- This is explicitly not general perspective/homography recovery; arbitrary camera pose and local projective estimation remain research work.

## v0.2.0 build 10 - 2026-09-08

Development build. Not a final v0.2.0 release.

### Regression recovery

- Kept the adaptive on-image Format v3 and encoder bit-for-bit unchanged.
- Restored pure isotropic resize recovery ahead of speculative rotation/lattice heuristics, preventing false angle candidates from suppressing the established resize path.
- Added a bounded virtual isotropic-scale sampler for 95/90/85/80/70/65/60/55/45/40/35/30/25% hypotheses; only a strongly supported single scale may trigger physical normalization.
- Added a targeted single-scale normalization fallback for cases where geometry sync is strong but the virtual sample does not authenticate.
- Preserved direct 8/6/4-pixel paths for 100/75/50% and kept later affine/rotation/lattice stages available after the resize baseline.
- Added deterministic v3 encoder pixel fingerprints for robust, balanced and capacity profiles so future decoder work cannot silently alter embedding output.

### Test and tooling fixes

- Unified large-PNG dimension probing across geometry, affine, composition and lattice scripts. If ImageMagick cannot inspect a huge PNG under its resource limits, the scripts read PNG IHDR dimensions without decoding pixels.
- The 16320x12288 (~201 MP) private HQ regression image is therefore reported as an intentional 50 MP geometry-suite SKIP instead of an erroneous dimension-read failure.
- Kept `make all-test` reporting from build 9 and documented build 10 as a regression-recovery build rather than a new geometry feature build.

### Real-corpus findings

- On the private 800x757 LQ regression image, robust and balanced again recover 95/85/75/65/55/50% resize cases; capacity recovers through 55%.
- Capacity at 50% on that LQ carrier remains a known signal limit and was independently observed with build 5, so it is not classified as a build-9 regression.
- No private regression images are included in the source archive.

## v0.2.0 build 9 - 2026-09-08

Development build. Not a final v0.2.0 release.

### Expanded direct lattice-basis recovery

- Kept the adaptive on-image format v3 bit-for-bit unchanged.
- Expanded the direct DCT-lattice basis bank from two to four promoted anisotropic shapes: 110%x90%, 90%x110%, 105%x95% and 95%x105%, each followed by arbitrary rotation from -45 to +45 degrees at 0.25-degree spacing.
- Preserved the basis-vector representation (`u`, `v`) rather than reintroducing separate rotation/scale parameter searches.
- Kept the search bounded at 1444 sparse probes, at most 576 stronger repetition/coherence evaluations, and at most four matrices with three phases each for full authenticated aggregation.
- Retained smaller 48-candidate shortlists for the original +/-10% anchors and introduced bounded 240-candidate shortlists for the +/-5% shapes after real-image tests showed substantially greater pixel-phase sensitivity there.
- Improved exact-quarter-turn gating using the repeated tile periods: native orientation is checked as 35x32 blocks while 90/270-degree candidates are identified by the swapped 32x35 periodicity.
- Fixed the quarter-turn gate for small carriers that contain a decodable v3 tile but not two complete tile periods: repetition coherence is now treated as unavailable rather than negative evidence, preserving 90/180/270-degree recovery without weakening the gate on larger carriers.
- Continued to require a valid v3 CRC32/HMAC-SHA256 frame for every successful extraction; all lattice metrics are geometry-ranking signals only.

### Test orchestration and reporting

- Added `make all-test` to execute the distinct test/check suites sequentially, continue after individual failures and print one final status/timing summary.
- Added `ALL_TEST_REPORT=<path>` to tee the complete run to a report file suitable for build-to-build comparison.
- Added `ALL_TEST_TARGETS="..."` for targeted comparative/smoke runs and `ALL_TEST_STRICT` to control strict semantics for experimental suites.
- `all-test` records PixSeal version, host, Go version and ImageMagick version when available.
- Expanded `make lattice-test` to exercise all four promoted basis shapes.

### Experimental findings

- Reused the two private real photographs from builds 7/8 as external regression material; neither source image is included in the repository or source archive.
- Verified the complete final `STRICT=1 make lattice-test` matrix on the two private regression photographs: four promoted basis shapes per image, **8/8 authenticated recoveries with no failures or timeouts**.
- Tested a more continuous/local `u`,`v` refinement prototype. Positive cases recovered, but negative extraction became too expensive, so that strategy was deliberately **not promoted** into build 9.
- Continuous/general affine estimation, rotation+shear composition, balanced/capacity composed recovery and reverse edit order remain research tasks.

### Documentation and versioning

- Updated README, algorithm specification, results, HISTORY, TODO and version metadata to `v0.2.0 build 9`.

## v0.2.0 build 8 - 2026-09-08

Development build. Not a final v0.2.0 release.

### Symmetric DCT-lattice basis bank

- Kept the adaptive on-image format v3 bit-for-bit unchanged.
- Generalized the build-7 direct composed search from one hard-coded 110%x90% geometry into a small bank expressed as transformed horizontal/vertical DCT-lattice basis vectors (`u`, `v`).
- Added the symmetric promoted basis shapes 110%x90% and 90%x110%, each searched from -45 to +45 degrees at 0.25-degree spacing.
- Kept the search explicitly bounded at 722 sparse basis probes, at most 48 quick candidates per shape (96 stronger-coherence evaluations total), and at most four matrices with three phases each for full authenticated aggregation.
- Changed the first-stage ranking to preserve a bounded shortlist independently for each basis shape. Real-image testing showed that the 20-position sparse score can under-rank a correct negative-angle matrix even when stronger tile periodicity is high.
- Continued to require a valid v3 CRC32/HMAC-SHA256 frame for every successful extraction; lattice coherence is used only to rank geometry.
- Added `make lattice-test` / `STRICT=1 make lattice-test` as a separate experimental corpus target; it intentionally remains outside `make all`.

### Real-corpus validation

- Reused the two private photographs that exposed the build-6 failure as local regression material; neither image is included in the repository or source archive.
- End-to-end ImageMagick testing at 12.3 degrees recovered both photographs under both promoted anisotropic orientations: **4 passed, 0 failed, 0 timeouts, 0 errors**.
- Additional development spot checks recovered both promoted basis shapes at positive and negative rotations, including -15 degrees on the smaller real photograph after the per-shape shortlist change.
- Preserved the existing build-7 110%x90% composition path as a historical regression target while making the build-8 lattice bank the new experimental basis-recovery target.

### Scope and documentation

- Build 8 is still a bounded discrete basis bank, not a continuous/general affine estimator. Continuous/local inference of `u` and `v`, rotation+shear composition, additional anisotropic pairs, balanced/capacity composed recovery and reverse edit order remain research tasks.
- Updated README, algorithm specification, results, HISTORY, TODO and version metadata for build 8.

## v0.2.0 build 7 - 2026-09-08

Development build. Not a final v0.2.0 release.

### Direct DCT-lattice composition recovery

- Kept the adaptive on-image format v3 bit-for-bit unchanged.
- Replaced the build-6 rotation-first composition heuristic with a direct estimator that scores the repeated v3 DCT lattice under the composed transform itself.
- Promoted only the already-declared `robust`, `110%x90% -> arbitrary rotation` baseline; broader scale pairs and profiles remain research work rather than increasing the negative-case search cost prematurely.
- Added a sparse first stage using 20 deterministic tile positions and a fixed 0.25-degree sweep from -45 to +45 degrees: **361 composed lattice probes maximum**.
- Retained at most 16 candidates for stronger periodicity measurement, then at most four matrices and three phases per matrix for full-carrier authenticated aggregation.
- Continued to use CRC32 and HMAC-SHA256 authentication as the sole success criterion; geometric coherence only ranks hypotheses.
- Removed the build-6 runtime dependency on the standalone rotation estimator for composed-scale recovery. This addresses the observed case where anisotropic scale displaced the apparent orientation peak.

### Real-corpus regression

- Added two private real-image regression cases supplied by PJ during development. They are used only locally and are **not** included in the repository or source archive.
- Under the existing ImageMagick `110%x90% -> 12.3 degrees` composition test, build 6 produced one timeout and one failed extraction on those images.
- The build-7 CLI recovered both authenticated messages and reported approximately `rotation-correction: -12.25 degrees` and `scale-correction: x=0.9091 y=1.1111`.
- End-to-end `STRICT=1 make composition-test` on those two private originals completed **2 passed, 0 failed, 0 timeouts, 0 errors**.
- Same-machine CLI spot checks on the transformed private carriers completed in about **5.5 s** and **3.0 s** respectively. These timings are engineering observations, not guarantees.
- Added an authenticated-failure fast exit when strong direct-lattice coherence identifies the composed geometry but HMAC fails. The same two transformed carriers with a wrong key completed in about **5.4 s** and **2.9 s**, avoiding the build-6 timeout-style fallback cascade.

### Tests and documentation

- Added a pure-Go regression test for direct-lattice composition recovery and a fixed-search-space test that freezes the 361-angle baseline.
- Updated README, algorithm notes, results, HISTORY, TODO, version metadata and composition-suite wording for build 7.
- Preserved `HISTORY.md`, `TODO.md`, the repository banner and the GUI/mobile portability direction introduced in build 6.

## v0.2.0 build 6 - 2026-09-07

Development build. Not a final v0.2.0 release.

### First bounded composed affine geometry

- Kept format v3 bit-for-bit unchanged and extended only the decoder geometry layer.
- Added an experimental composed path for anisotropic X/Y scaling followed by arbitrary digital rotation.
- Avoided the full angle x affine Cartesian product: at most two rotation peaks are retained, each is quantized to 0.25 degrees and refined only within +/-2 degrees, then combined with four fixed anisotropic scale pairs (110%x90%, 90%x110%, 105%x95%, 95%x105%).
- Capped the new stage at 136 composed matrix probes. Only the six strongest candidates survive geometric ranking and each contributes at most three full-carrier phase aggregates, for at most 18 authenticated composed probes.
- HMAC-authenticated v3 decoding remains the sole success criterion.
- `ExtractInfo` and CLI output can report rotation and anisotropic scale correction together when the composed path succeeds.
- Kept the first claimed composition deliberately narrow: the validated regression baseline is `robust`, 110%x90% -> 12.3-degree rotation. Balanced/capacity, 90%x110%, broader angle envelopes, the reverse edit order and rotation+shear remain research tasks.

### Composition regression suite

- Added `make composition-test`, intentionally outside `make all`, plus `STRICT=1 make composition-test`.
- The default suite uses only `original pics/`, generates transformed carriers in a temporary directory and validates the currently claimed robust baseline with ImageMagick.
- A temporary generated 900x700 development carrier passed the default composition case with authenticated recovery and reported both rotation and scale corrections.

### Project documentation

- Added `HISTORY.md` for architectural/build history and `TODO.md` for the live engineering roadmap.
- Added the PixSeal project banner under `docs/assets/` and displayed it at the top of the README.
- Updated README, algorithm notes, results notes, version metadata and test instructions for build 6.
- Kept the reusable `watermark` core free of CLI/UI dependencies and added `make core-target-check` to compile-check it for Linux, Windows, Android/arm64 and iOS/arm64 in preparation for future Windows/Linux/Android graphical frontends.

### Performance notes

- Tightened the reference arbitrary-rotation contrast gate after a regression test found weak rotation candidates on an unmarked synthetic image; authenticated behavior was already safe, but the stronger fast-reject gate avoids unnecessary geometry work.
- Same-machine spot checks remained close to build 5 for ordinary negative inputs: an unmarked 900x700 image completed in about 6.2 s and an aligned carrier with a wrong key in about 10.0 s.
- The generated robust 110%x90% -> 12.3-degree rotation case recovered in about 8-9 s in this environment; the same transformed carrier with a wrong key completed in about 8.7 s. These are engineering observations, not guarantees.

## v0.2.0 build 5 - 2026-09-07

Development build. Not a final v0.2.0 release.

### Bounded axis-aligned affine recovery

- Kept the on-image format v3 unchanged and extended only the decoder geometry layer.
- Added a finite 36-matrix affine hypothesis set: 20 non-uniform X/Y scale pairs drawn from 90/95/100/105/110%, excluding uniform pairs already covered by the resize path, plus X/Y shear at +/-3, +/-5, +/-8 and +/-10 degrees.
- Added a virtual inverse-affine sampler so hypotheses can be evaluated directly in DCT space without rendering dozens of full-resolution corrected images.
- Added key-independent periodic tile-coherence scoring to reject weak geometry and rank affine hypotheses before authenticated probing.
- Limited authenticated single-tile probing to at most six affine matrices and three retained phases per matrix. Only the four strongest failed sync candidates may receive full-carrier repetition aggregation.
- Kept HMAC-authenticated v3 decoding as the sole success criterion.
- Added correction metadata for anisotropic X/Y scale and X/Y shear to `ExtractInfo` and CLI extraction output.
- Deliberately kept arbitrary-angle rotation and the new affine stage separate; rotation composed with anisotropic scale/shear remains the next research step rather than introducing an angle x affine Cartesian search.

### Affine regression suite

- Added `make affine-test`, intentionally outside `make all`, using only the private `original pics/` corpus and temporary generated transforms.
- Added `STRICT=1 make affine-test` for fail-fast CI-style validation.
- The default ImageMagick matrix covers 110%x90%, 90%x110%, shear X 8 degrees and shear Y 8 degrees for `robust`, `balanced` and `capacity`.
- A generated 900x700 development corpus completed the full default matrix with 12/12 authenticated recoveries. This is a development regression result, not a guarantee for arbitrary photographs.

### Performance and validation

- Added a native-lattice repetition-coherence gate so aligned wrong-key or unmarked inputs do not automatically pay the full affine search cost.
- Same-machine spot checks showed the affine stage remained bounded; a freshly embedded 900x700 robust carrier transformed to 110%x90% recovered in about 2.21 s, while the same transformed carrier with a wrong key completed in about 10.46 s. These timings are engineering observations, not guarantees.
- `gofmt`, `go vet ./...`, native build, cross-builds and shell syntax checks pass in the development environment. The full uncached `watermark` package test and aggregate `go test ./...` exceed the command window of this environment and are therefore left for explicit validation on PJ's Linux system.

### Documentation

- Updated README, algorithm specification, results notes, CLI output descriptions and version metadata to `v0.2.0 build 5`.
- Documented the affine search bounds and the staged path toward rotation+affine composition, perspective recovery and the longer-term print-camera channel.

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
