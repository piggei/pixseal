# PixSeal robustness and validation results

This file separates **deterministic format properties**, **automated regression
results** and **corpus-specific experimental measurements**. None of the results
below proves statistical steganographic undetectability or guarantees recovery
for unseen images.

## v0.2.0 build 4 validation status

Development build: **v0.2.0 build 4**, 7 September 2026.

Build 4 keeps the adaptive v3 on-image format unchanged and extends the bounded
geometry layer from pure rotation to a first combined rotation/resize/crop
baseline.

### Deterministic search bounds

```text
Direct crop/resize grids:              116
Gated quarter-turn grids:     3 x 116 = 348 maximum
Arbitrary-angle sparse probes:
  zero-degree lattice checks:            2
  non-zero coarse probes:              720
  local fine probes:                    33 maximum
Authenticated arbitrary-angle decode:
  2 rectified candidates x 4 quadrants x 64 native grids = 512 maximum
Pure-resize normalization:              13 + 24 candidates
```

The theoretical maximum is therefore **1013 full grid candidates** plus at most
**755 sparse orientation probes** when every optional branch is exercised. The
75% arbitrary-angle path uses 36-offset 6-pixel grids and is cheaper than the
native 64-offset worst case.

### Combined-geometry development checks

Using a temporary 900x700 generated carrier and ImageMagick transformations,
the build-4 geometry harness recovered all of the following at 12.3 degrees for
the tested profiles:

```text
rotate
rotate -> resize 75%
resize 75% -> rotate
rotate -> crop 80%
crop 80% -> rotate
rotate -> resize 75% -> crop 80%
```

Robust and balanced were exercised through the full six-case matrix in this
environment. Capacity was exercised in split runs covering the same five
combined modes plus a 90-degree quarter-turn control. These generated checks do
**not** count as private-corpus robustness results.

The Go regression suite additionally covers rotate+resize75 and rotate+crop
recovery around format v3.

A 50% arbitrary-angle + resize experiment was intentionally treated as a
negative boundary. Even when the exact 12.3-degree correction was supplied to
the decoder, the transformed carrier did not authenticate at default strength
24. This indicates signal loss from double interpolation rather than merely a
failure to estimate the angle.

### Failed-extraction control

Build 4 adds an early stop when a strong non-zero lattice orientation is found
but authenticated decoding fails. This prevents a clearly rotated wrong-key
carrier from falling through into the unrelated pure-resize normalization path.
The dedicated Go wrong-key rotation regression completed in approximately
**3.6 s** in this development environment after that change.

### Validation status in this environment

Successfully executed:

```text
gofmt
go vet ./...
go test ./cmd/pixseal
go test ./watermark
bash -n scripts/test-robustness.sh
bash -n scripts/test-limits.sh
bash -n scripts/test-geometry.sh
make clean
make
make build-all
```

`go test ./watermark` completed with all tests passing in about 34 seconds. The
aggregate `go test ./...` invocation was also attempted, but in this execution
environment it did not complete before the command timeout even though the same
packages pass when run separately. It is therefore **not** reported as passed
here and should be rerun on PJ's Linux system.

The private `original pics/` corpus is not available in this environment. The
complete default `make geometry-test` matrix still requires validation on PJ's
machine. `make all` was attempted, but its internal aggregate `go test ./...` hit
the same environment timeout described above before the corpus check; it is not
reported as passed.

### Current geometry boundary

Build 4 claims an **experimental native/75%-scale rotation + crop baseline**. It
does not claim arbitrary-angle + 50% resize, arbitrary fractional-scale
synchronization, affine deformation, perspective correction or print-camera
recovery.

## v0.2.0 build 3 validation status

Development build: **v0.2.0 build 3**, 7 September 2026.

Build 3 keeps the adaptive v3 on-image format unchanged and adds bounded digital
rotation recovery around the v3-only decoder.

### Deterministic rotation-search bounds

The native crop/resize search is unchanged. Rotation adds two finite stages:

```text
Exact quarter turns:
  up to 3 orientations x 64 direct grids = 192 grids

Arbitrary-angle estimator:
  -45 .. +45 degrees, step 0.25 = 361 sparse coarse probes
  at most 2 peaks x 11 local refinements = 22 sparse refinement probes
  383 sparse angle probes maximum

Authenticated arbitrary-angle decode:
  at most 2 rectified angles x 4 quadrants x 64 grids = 512 grids
```

Including every optional branch, the theoretical maximum is **857 full grid
candidates** plus the fixed 383 sparse orientation probes. This is a worst-case
bound, not the cost of a normal successful extraction. Unrotated valid carriers
still return from the original native fast path before rotation analysis.

Rotation rectification is skipped when the expanded canvas would exceed
50,000,000 pixels.

### Automated validation in this environment

The following checks were executed successfully for build 3:

```text
gofmt
go vet ./...
go test ./...
bash -n scripts/test-robustness.sh
bash -n scripts/test-limits.sh
bash -n scripts/test-geometry.sh
make clean
make
make build-all
```

A temporary generated 560x512 corpus was also used only to validate shell-suite
plumbing. `make geometry-test` passed 7.5, 12.3 and 90 degree robust-profile
cases, and a reduced `make all` matrix passed round trips plus JPEG, 75% resize,
75% center crop and 75% random crop for all three profiles. These generated
checks do **not** count as private-corpus robustness results.

With `original pics/` removed again, `make all`, `make extreme-test` and
`make geometry-test` were each invoked and stopped at the expected missing-corpus
check. They are therefore not reported as passed on PJ's photographic corpus.

### Automated rotation regression results

The Go regression suite includes independent bilinear image rotation fixtures and
currently verifies:

- lossless 90/180/270-degree recovery across robust/balanced/capacity profiles;
- fractional arbitrary-angle recovery at 7.5, 12.3 and 22.7 degrees across the
  three profiles;
- profile auto-detection after rotation;
- reported rotation-correction angle within 0.15 degree of the expected value in
  those fixtures;
- rejection of an unmarked synthetic image by the arbitrary-angle probe;
- bounded failure for a rotated carrier extracted with the wrong key.

These are generated deterministic regression images, not a substitute for the
private photographic corpus.

### Same-machine performance spot checks

Build 2 and build 3 were compiled in the same environment and run against the
same deterministic synthetic files. Times are engineering spot checks rather
than guarantees:

| Case | Size | Build 2 | Build 3 | Observation |
|---|---:|---:|---:|---|
| valid unrotated robust carrier | 560x512 | ~0.03 s | ~0.02 s | native positive fast path unchanged |
| unmarked carrier | 560x512 | ~4.56 s | ~3.93 s | zero-degree lattice precheck skips the angle sweep |
| valid carrier, wrong key | 560x512 | ~4.20 s | ~4.29 s | aligned wrong-key carrier avoids the angle sweep |
| unmarked carrier | 1024x768 | ~12.19 s | ~13.11 s | larger DCT search still dominates |
| robust carrier rotated 12.3 deg | 560x512 | unsupported | ~2.39 s | recovered, correction about -12.25 deg |
| robust carrier rotated 44.4 deg | 560x512 | unsupported | ~3.09 s | recovered, correction about -44.40 deg |
| robust carrier rotated 90 deg | 560x512 | unsupported | ~0.28 s | lossless quarter-turn fast path |
| rotated 12.3 deg, wrong key | 560x512 | unsupported | ~10.24 s | bounded authenticated failure |

The zero-degree lattice precheck avoids the expensive angle sweep when the carrier
is already geometrically aligned, so ordinary unmarked and wrong-key spot checks
remain close to the build-2 baseline. A genuinely rotated wrong-key carrier does
pay the bounded orientation/rectification cost (~10.24 s in the 12.3-degree
560x512 spot check). Future work should continue reducing that rotated-negative
cost before adding affine/perspective dimensions.

### Private `original pics` geometry suite

Build 3 adds:

```sh
make geometry-test
```

The default matrix tests -45, -30, -15, -10, -5, -1, 1, 5, 10, 15, 30, 45,
90, 180 and 270 degrees for every selected profile, using ImageMagick only to
produce the transformed temporary carrier. The extractor receives no angle.

The private `original pics/` corpus is absent from this development environment,
so `make geometry-test`, `make all` and `make extreme-test` cannot be reported as
passed here. They must be validated on PJ's Linux system.

### Current geometry boundary

Build 3 claims **experimental pure digital rotation recovery**. It does not yet
claim combined arbitrary rotation + fractional resize/crop, affine deformation,
perspective correction or print-camera recovery. Those remain staged research
targets.

## v0.2.0 build 2 validation status

Development build: **v0.2.0 build 2**, 7 September 2026.

Build 2 keeps the adaptive v3 profile math unchanged and removes runtime v1/v2
compatibility. The decoder now evaluates only the three authenticated v3 profile
patterns inside the same bounded geometric search.

### Deterministic v3 profile math

| Profile | Maximum payload | Hamming-coded bits | Logical tile positions | Mean tile observations/coded bit |
|---|---:|---:|---:|---:|
| robust | 16 bytes | 448 | 1120 | 2.50x |
| balanced | 32 bytes | 672 | 1120 | 1.6667x |
| capacity | 64 bytes | 1120 | 1120 | 1.00x |

The geometric search remains bounded to at most 153 candidates before
image-size skips:

```text
116 direct block-size/offset grids
 13 aligned inverse-normalizations
 24 +/-1 dimension neighbours for the three strongest scales
153 maximum geometric candidates
```

Profile detection is fixed work inside each grid and does not multiply this
candidate count.

### Automated validation in this environment

The following checks were executed successfully during build 2 development:

```text
gofmt
go vet ./...
go test ./...
bash -n scripts/test-robustness.sh
bash -n scripts/test-limits.sh
make clean
make
make build-all
```

The Go suite covers:

- automatic profile selection at 16/17/32/33/64 bytes and 65-byte rejection;
- explicit-profile overflow errors;
- robust, balanced and capacity round trips;
- JPEG quality 82, 75% resize and 75% crop for every v3 profile;
- automatic profile recovery without extractor profile input;
- wrong-key and unmarked-image bounded failure paths;
- v3 frame/header authentication and Hamming correction;
- adaptive tile mapping and synchronization observation counts;
- `analyze` option validation and `analyze`/`capacity`/`embed` consistency;
- rejection of removed compatibility flags;
- optimized bicubic normalization equivalence to the reference implementation.

Formats v1 and v2 are no longer regression targets. Their runtime decoders and
compatibility fixtures were deliberately removed in build 2 because no known external
carrier population exists.

### Build 1 versus build 2 failed-extraction spot check

A same-machine comparison was run from clean build 1 and build 2 source trees on
the same deterministic synthetic carriers. The values are engineering spot
checks, not performance guarantees:

| Case | Size | Build 1 | Build 2 | Observation |
|---|---:|---:|---:|---|
| unmarked carrier | 560x512 | ~8.26 s | ~5.29 s | v3-only probing reduces failed-work overhead |
| valid robust carrier, wrong key | 560x512 | ~5.40 s | ~3.90 s | v3-only probing reduces failed-work overhead |
| valid robust carrier | 560x512 | ~0.02 s | ~0.02 s | positive fast path unchanged |
| unmarked carrier | 1024x768 | ~13.27 s | ~12.92 s | DCT/grid work dominates at larger size |

The larger-image result is important: removing v2 probing helps, but it does not
remove the fundamental cost of exhausting geometric DCT candidates. Further
performance work should therefore focus on cheaper synchronization rejection,
not on reintroducing additional brute-force dimensions.

### Private `original pics` corpus

The private `original pics/` corpus is intentionally absent from this source
archive and from the build environment used here. `make all` and
`make extreme-test` were actually invoked: the Go tests inside `make all` passed,
then both commands stopped at the expected missing-corpus check. They are
therefore **not** reported as passed.

The local corpus commands must still be validated on PJ's Linux system:

```sh
make clean
make test
make deep-test
make extreme-test
make all
```

`make test` means unit tests plus round trips on `original pics`; `deep-test` and
`extreme-test` require that corpus plus ImageMagick and GNU `timeout`. No corpus
result is claimed here unless the command was actually executed against those
images.

### Geometry roadmap

Build 2 does **not** claim arbitrary rotation, affine or perspective recovery.
The next controlled experiment is digital rotation, followed by combined
rotation/resize/crop. Those tests are intended to develop bounded geometric
synchronization toward the longer-term print-camera goal.

## Historical: v0.2.0 build 1 validation status

Development build: **v0.2.0 build 1**, 7 September 2026.

### Deterministic v3 profile math

| Profile | Maximum payload | Hamming-coded bits | Logical tile positions | Mean tile observations/coded bit |
|---|---:|---:|---:|---:|
| robust | 16 bytes | 448 | 1120 | 2.50x |
| balanced | 32 bytes | 672 | 1120 | 1.6667x |
| capacity | 64 bytes | 1120 | 1120 | 1.00x |

The unit suite verifies the automatic threshold boundaries at 16, 17, 32, 33
and 64 bytes and rejects 65 bytes. It also verifies that the v3 tile mapping
gives every protected bit either `floor(redundancy)` or `ceil(redundancy)`
observations; no protected bit is omitted from a complete logical tile.

### Automated tests executed in the development environment

The following commands were actually executed successfully for build 1:

```text
gofmt (all Go sources)
go vet ./...
go test ./...
make clean
make
make build-all
```

The cross-build produced Linux amd64, Windows amd64, macOS amd64 and macOS
arm64 binaries successfully; those generated binaries are removed before the
development source archive is packaged.

The Go regression suite includes:

- automatic profile selection at the 16/17/32/33/64-byte boundaries;
- rejection of a 65-byte payload;
- explicit-profile capacity rejection and minimum-compatible-profile reporting;
- maximum-size round trips for robust (16 B), balanced (32 B) and capacity (64 B);
- automatic extraction without a supplied profile;
- JPEG quality 82, 75% resize and 75% crop round trips for each v3 profile on
  generated deterministic test imagery;
- v2 extraction compatibility, including the v0.1 transform regression path;
- v1 extraction compatibility;
- wrong-key and unmarked-image failure paths;
- bounded failed-extraction regression timing guard;
- `analyze` option validation;
- consistency among `analyze`, detailed `capacity` and auto `embed` profile
  selection;
- pixel-plane bicubic normalization equivalence to the reference implementation;
- Hamming single-bit correction.

Generated images in the Go unit suite are temporary test fixtures and are not
part of source archives.

### Failed-extraction performance spot checks

A local timing spot check was performed after the v3 phase-ranking optimization.
These numbers are machine/build specific and are **not** performance guarantees:

| Case | Carrier size | Result | Observed elapsed time |
|---|---:|---|---:|
| unmarked image | 560x512 | rejected | ~3.41 s |
| unmarked image | 1024x768 | rejected | ~9.11 s |
| v3 robust carrier, wrong key | 560x512 | rejected | ~3.74 s |
| valid v3 robust carrier | 560x512 | recovered | ~0.01 s |

The important regression property is structural: the modern geometric search is
bounded to at most 153 candidates before image-size skips, and v3 shares the
expensive grid aggregation with v2 rather than running a second geometric scan.

### Local `original pics` corpus: unavailable here

The private `original pics/` corpus is intentionally absent from the development
archive/environment used for this build. `make all` and `make extreme-test` were
actually invoked to verify their control flow, but both stopped at the expected
missing-corpus check:

```text
make all          -> unit tests passed, then test-images stopped: original pics missing
make extreme-test -> stopped immediately: original pics missing
```

Therefore `make test`, `make deep-test`, `make all` and `make extreme-test` are
**not** claimed as corpus-validated for this environment.

`make test` now means Go unit tests **plus** round trips on `original pics`, as
required for the v0.2 development workflow. `deep-test` and `extreme-test` also
require that corpus plus ImageMagick and GNU `timeout`.

On PJ's Linux system, build 1 should be validated with:

```sh
gofmt -w cmd internal watermark
go vet ./...
go test ./...
make clean
make all
make extreme-test
```

The shell suites now exercise `robust`, `balanced` and `capacity` separately,
using profile-appropriate messages and the same deterministic random-crop
locations for cross-profile comparison.

## Stable v0.1.0 baseline

The measurements below were recorded on 7 September 2026 during the final v0.1
cycle. They remain useful as the baseline that v3 must not regress unexpectedly.
They describe recovery of an authenticated v2 payload after transformations,
not general guarantees.

The historical tests used the private `original pics` corpus, default strength
24, a short authenticated message, ImageMagick transformations and five-point
percentage steps. The source images are excluded from Git and release archives.

### Progressive v0.1 measurements

| Source | Smallest recovered resize tested | Last passing center crop before first failure | Last passing random crop before first failure |
|---|---:|---:|---:|
| Immagine LQ | 45% | 40% | 40% |
| Immagine MQ | 40% | 20% | 20% |
| Immagine HQ | Not tested | Not tested | Not tested |

`Immagine HQ` was skipped because its approximately 201 megapixels exceeded the
100-megapixel test-harness limit.

For LQ, resize extraction passed at every tested step from 95% through 45% and
failed from 40% through 10%. For MQ, it passed from 95% through 40%; an earlier
development build timed out at 35% and 30%, then reported failures from 25%
through 10%. The timeout cases were inconclusive.

The historical crop run stopped each sequence at its first failure. Therefore
40% (LQ) and 20% (MQ) mean **last passing step before the first observed
failure**, not a proof that every smaller crop fails. Later scripts were changed
to continue through all configured crop percentages.

### v0.1 deep-test baseline

The v0.1 default regression matrix was:

- JPEG recompression at quality 82;
- resize at 95%, 85%, 75%, 65%, 55% and 50%;
- centered crop at 90%, 75% and 50%;
- deterministic off-center crop at 75% and 50%.

All applicable LQ and MQ transformations in that baseline were recovered during
the final v0.1 development cycle. The HQ image remained skipped by the 100-MP
test-harness bound.

That harness limit is separate from the decoder's internal rule that skips an
individual inverse-normalization reconstruction above 50,000,000 pixels.

## How to interpret future v0.2 measurements

For the adaptive format, results should be reported **per profile**. A single
`auto` result can hide the very trade-off v3 is intended to expose.

Recommended reporting fields are:

```text
source image
profile
payload byte length
strength
transformation and parameters
PASS / FAIL / TIMEOUT / SKIP
elapsed extraction time when relevant
```

`PASS` proves recovery for that exact carrier/transformation/key. `FAIL` means
the bounded search completed without an authenticated payload. `TIMEOUT` is an
inconclusive harness result. `SKIP` means a configured test or decoder size bound
prevented the candidate from being attempted.
