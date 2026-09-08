# PixSeal robustness and validation results

This file separates **deterministic format properties**, **automated regression
results** and **corpus-specific experimental measurements**. None of the results
below proves statistical steganographic undetectability or guarantees recovery
for unseen images.

## v0.2.0 build 9 validation status

Development build: **v0.2.0 build 9**, 8 September 2026.

Build 9 keeps format v3 bit-for-bit unchanged and expands the direct
DCT-lattice basis bank from two to four anisotropic shapes. It also introduces
`make all-test` as the preferred sequential comparative test orchestrator.

### Deterministic lattice-bank bounds

The promoted build-9 bank is fixed in source:

```text
4 basis shapes: 110%x90%, 90%x110%, 105%x95%, 95%x105%
361 angles per shape: -45..+45 degrees at 0.25-degree spacing
1444 sparse lattice probes maximum

20 deterministic logical tile samples per sparse probe
48 quick candidates maximum for each +/-10% anchor shape
240 quick candidates maximum for each +/-5% moderate shape
576 stronger repetition/coherence evaluations maximum
4 matrices retained for authenticated aggregation maximum
3 phases per retained matrix maximum
12 full-carrier authenticated probes maximum
```

The sparse and stronger coherence scores are key-independent geometry-ranking
signals only. A recovered frame must still pass v3 CRC32 and HMAC-SHA256
authentication.

### Private real-corpus regression

The two private photographs that exposed the build-6 composition failure remain
external regression inputs and are not distributed. With ImageMagick at 12.3
degrees, development verification recovered both new moderate anisotropies on
both photographs, in addition to the two build-8 anchor anisotropies:

| Private carrier | 110%x90% | 90%x110% | 105%x95% | 95%x105% |
|---|---|---|---|---|
| real photograph A | PASS | PASS | PASS | PASS |
| real photograph B | PASS | PASS | PASS | PASS |

End-to-end `STRICT=1 make lattice-test` on the final build-9 decoder completed
**8 passed, 0 failed, 0 timeouts, 0 skipped, 0 errors**.

The new 105%x95% / 95%x105% cases were observed at roughly 8-10 seconds per
successful extraction on this development machine. These are engineering
observations, not latency guarantees.

A smaller shortlist for the moderate shapes was tested and caused a real
regression miss; the final 240-candidate-per-moderate-shape bound is therefore a
measured correctness/performance compromise.

### Continuous-refinement experiment

A denser/local refinement prototype for the basis vectors recovered additional
positive cases, but negative extraction approached unacceptable runtimes. The
prototype was **not promoted**. Build 9 remains a bounded discrete basis bank.

### all-test reporting

`make all-test` executes the distinct test/check targets sequentially and reports
status plus elapsed seconds for each section. Experimental transformation suites
run with strict semantics by default. Optional `ALL_TEST_REPORT=<path>` captures
the complete output for build-to-build comparison.

The full private `original pics/` corpus is not distributed. Final aggregate
`all-test` results therefore belong to PJ's Linux workstation and should be
attached or summarized separately when comparing builds.

### Final source-tree validation in this environment

The build-9 closeout caught and fixed one small-carrier quarter-turn regression:
the new tile-period gate initially treated an unmeasurable repetition score as a
failed score. After distinguishing "not enough carrier area for two periods"
from low coherence, the 90/180/270-degree regression test passed again.

The following checks completed successfully on the final source tree:

```text
gofmt                                    PASS
go vet ./...                             PASS
go test -count=1 ./cmd/pixseal           PASS
core/profile/frame + baseline tests      PASS
quarter-turn recovery                    PASS
arbitrary-rotation recovery              PASS
rotation + resize/crop recovery          PASS
direct lattice-basis recovery            PASS
fixed lattice-bank bound                 PASS
axis-aligned affine scale recovery       PASS
wrong-key/unmarked bounded tests         PASS
shell syntax checks                      PASS
make                                      PASS
make build-all                            PASS
make core-target-check                    PASS
STRICT=1 make composition-test (2 real)  2/2 PASS
STRICT=1 make lattice-test (2x4 real)      8/8 PASS
```

A smoke invocation of the new orchestrator using `ALL_TEST_TARGETS="vet
core-target-check"` completed with a valid report and `Overall: PASS`, confirming
sequential execution, report capture and final-summary behavior. The complete
default `make all-test` run was also started, but its `make test` section invokes
the monolithic `go test ./...`, which exceeds this environment's execution
window. Therefore the full aggregate run is **not** marked as passed here and
remains a required comparative run on PJ's Linux workstation.

### Validation boundary

Build 9 does **not** claim continuous/general affine estimation, arbitrary
rotation combined with shear, balanced/capacity composed recovery, reverse edit
order, perspective or print-camera recovery. Those remain explicit research
tasks in `TODO.md`.

## v0.2.0 build 8 validation status

Development build: **v0.2.0 build 8**, 8 September 2026.

Build 8 keeps format v3 bit-for-bit unchanged and generalizes the build-7
direct composed search into the first symmetric **DCT-lattice basis bank**. The
decoder represents each promoted geometry by transformed horizontal and vertical
lattice basis vectors rather than by a rotation-detector result followed by a
scale guess.

### Deterministic lattice-bank bounds

The build-8 bank is fixed in source:

```text
2 basis shapes: 110%x90% and 90%x110%
361 angles per shape: -45..+45 degrees at 0.25-degree spacing
722 sparse lattice probes maximum

20 deterministic logical tile samples per sparse probe
48 quick candidates retained per shape maximum
96 matrices receive stronger periodicity/coherence analysis maximum
4 matrices retained for authenticated aggregation maximum
3 phases per retained matrix maximum
12 full-carrier authenticated probes maximum
```

The sparse and stronger coherence scores are key-independent geometry-ranking
signals only. A recovered frame must still pass v3 CRC32 and HMAC-SHA256
authentication.

### Private real-corpus regression

The two photographs that exposed the build-6 composition failure were reused only
as temporary local regression inputs. They are not part of the repository or
distributed source archive. With ImageMagick and the default build-8
`lattice-test` angle of 12.3 degrees:

| Private carrier | 110%x90% -> rotate | 90%x110% -> rotate |
|---|---|---|
| real photograph A | PASS | PASS |
| real photograph B | PASS | PASS |

End-to-end result: **4 passed, 0 failed, 0 timeouts, 0 skipped, 0 errors**.
Successful extractions reported approximately `rotation-correction: -12.25
degrees`; the inverse X/Y scale correction correctly swapped between the two
basis shapes.

During development, positive-angle spot checks at 5 and 20 degrees succeeded on
both promoted shapes on the smaller real photograph. The first implementation
under-ranked a -15-degree carrier at the sparse stage; retaining an independent
bounded shortlist per basis shape corrected that asymmetry, and manual -15-degree
spot checks then recovered both promoted shapes. These are engineering observations,
not a continuous-angle or unseen-image guarantee.

Same-machine spot checks with the final build-8 decoder on the smaller private
carrier measured approximately:

```text
110%x90% -> 12.3deg, valid key     4.68 s
90%x110% -> 12.3deg, valid key     4.64 s
110%x90% -> 12.3deg, wrong key     9.84 s
unmarked original                  8.03 s
```

These are development observations on one machine and must not be interpreted as
latency guarantees.

### Source validation in this environment

The following checks completed successfully on the final build-8 source tree:

```text
gofmt check                              PASS
go vet ./...                             PASS
go test -count=1 ./cmd/pixseal           PASS
core/profile/image watermark test group  PASS
quarter-turn/rotation test group         PASS
combined-geometry test                   PASS
direct lattice-basis recovery test       PASS
fixed lattice-bank bound test            PASS
axis-aligned affine scale test            PASS
fixed affine-search bound test            PASS
shell syntax checks                       PASS
make                                      PASS
make build-all                            PASS
make core-target-check                    PASS
STRICT=1 make composition-test (2 real)   2/2 PASS
STRICT=1 make lattice-test (2x2 real)     4/4 PASS
```

The monolithic uncached `go test -count=1 ./watermark` invocation exceeded this
environment's 240-second execution window even though the same test groups and
individual long-running geometry tests pass when run separately. The aggregate
`go test -count=1 ./...` / `make test` therefore remains a required validation on
PJ's Linux workstation and is not marked as passed here. A full private
`affine-test` run was also started on the two supplied originals; the robust cases
shown before the environment timeout passed, but the complete matrix was not
finished here and is not reported as a pass.

### Validation boundary

Build 8 does **not** claim continuous/general affine estimation, arbitrary
rotation combined with shear, 105%x95% / 95%x105% composed recovery,
`balanced`/`capacity` composed recovery, reverse edit order, perspective or
print-camera recovery. Those remain explicit research tasks in `TODO.md`.

The complete private `original pics/` corpus is not distributed. PJ should run
`go test -count=1 ./...`, the standard local suites and `STRICT=1 make
lattice-test` on the development workstation before treating this build as a
release candidate.

## v0.2.0 build 7 validation status

Development build: **v0.2.0 build 7**, 8 September 2026.

Build 7 keeps format v3 unchanged and replaces the build-6 rotation-first
composition heuristic with direct scoring of the repeated v3 DCT lattice. The
promoted composed baseline remains intentionally narrow: `robust`, 110%x90%
anisotropic scale followed by arbitrary digital rotation.

### Why the estimator changed

PJ tested build 6 against two private real photographs using the default
`make composition-test` transform. The results were:

```text
private carrier A: TIMEOUT (>60 s)
private carrier B: FAIL
```

The generated development carrier had passed, so these results exposed an
overfitting/generalization problem rather than a simple implementation error.
Instrumentation showed that the correct 110%x90% + 12.3-degree matrix had strong
tile coherence even when the standalone rotation estimator either found no useful
peak or selected a texture-driven peak far from the true orientation.

### Deterministic direct-lattice search bounds

The build-7 baseline search is fixed in source:

```text
1 scale pair: 110%x90%
361 angles: -45..+45 degrees at 0.25-degree spacing
361 sparse lattice probes maximum

20 deterministic logical tile samples per sparse probe
16 candidates receive stronger coherence analysis maximum
4 candidates retained for authenticated aggregation maximum
3 phases per retained candidate maximum
12 full-carrier authenticated probes maximum
```

The sparse and coherence scores are geometric ranking signals only. CRC32 and
HMAC-SHA256 authentication of the v3 frame remain required for success.

### Private real-corpus regression

The two photographs that exposed build 6 were used only as local regression
inputs and are not part of the repository or source archive. With the same
ImageMagick pipeline used by `composition-test`:

```text
embed robust payload
resize 110%x90%
rotate 12.3 degrees on a white expanded canvas
extract
```

build 7 produced:

```text
2 passed
0 failed
0 timeouts
0 skipped
0 errors
```

Both extractions reported approximately:

```text
rotation-correction: -12.25 degrees
scale-correction: x=0.9091 y=1.1111
```

Same-machine CLI spot checks on the already transformed carriers completed in
approximately **5.5 s** and **3.0 s**. These are engineering measurements only;
they are not latency guarantees on other images or systems.

With a wrong key, the same transformed carriers completed in approximately
**5.4 s** and **2.9 s**. Build 7 treats strong direct-lattice coherence followed
by failed HMAC as a bounded authentication failure and does not continue into
unrelated rotation/resize searches. Unmarked-image cost remains image-dependent;
on the larger private original it remained about 27 s, essentially unchanged from
build 6, while the smaller private original completed in about 6 s.

### Validation boundary

Build 7 does not promote balanced/capacity composed recovery, 90%x110%,
105%x95%, 95%x105%, the reverse transform order, rotation+shear, arbitrary affine
matrices, perspective or print-camera recovery. Those remain explicit research
tasks in `TODO.md`.

### Build-7 source validation in this environment

The following checks completed successfully after the direct-lattice integration:

```text
gofmt                                      PASS
go vet ./...                               PASS
go test -count=1 ./cmd/pixseal             PASS
profile/frame/resize/quarter-turn tests     PASS
arbitrary-rotation tests                    PASS
rotation+resize/crop tests                  PASS
direct-lattice composition unit test        PASS
axis-aligned affine scale tests             PASS
wrong-key/unmarked bounded unit tests       PASS
shell syntax checks                         PASS
make                                        PASS
make build-all                              PASS
make core-target-check                      PASS
STRICT=1 make composition-test (2 private)  2/2 PASS
```

The monolithic uncached `go test ./watermark` / `go test ./...` invocation did not
complete within the execution window of this environment even though the same
test groups pass when run separately. It must therefore be rerun on PJ's Linux
system and is **not** marked as passed here. The complete private `original pics/`
corpus is also not available in this environment; only the two regression images
supplied for the build-6 failure were used locally and neither is distributed.

## v0.2.0 build 6 validation status

Development build: **v0.2.0 build 6**, 7 September 2026.

Build 6 keeps format v3 unchanged and adds the first bounded composition of
anisotropic scale and arbitrary rotation. The claimed baseline is intentionally
narrow and currently validated only for the `robust` profile.

### Deterministic composed-search bounds

```text
2 retained rotation peaks maximum
17 quarter-degree refinements per peak (+/-2 degrees)
4 anisotropic scale pairs
136 composed matrix probes maximum

6 composed matrices retained maximum
3 phases per retained matrix maximum
18 full-carrier authenticated composed probes maximum
```

The new stage is not the full angle x 36-affine-matrix Cartesian product. HMAC
authentication remains the only condition for a recovered message.

### ImageMagick composed regression

A temporary generated 900x700 carrier was embedded with the `robust` profile,
resized anisotropically to 110%x90%, then rotated by 12.3 degrees using
ImageMagick. The compiled build-6 CLI recovered the authenticated message and
reported:

```text
rotation-correction: approximately -12.25 degrees
scale-correction: x=0.9091 y=1.1111
```

The new `STRICT=1 make composition-test` target completed the default generated
case with **1 passed, 0 failed, 0 timeouts, 0 errors**. This validates the first
composed path and its shell harness; it is not a guarantee for arbitrary images,
angles or affine matrices.

### Performance spot checks

Same-machine development observations on the generated 900x700 carrier:

```text
unmarked image                         ~6.2 s
aligned carrier, wrong key            ~10.0 s
110%x90% -> 12.3deg, valid robust      ~8-9 s
110%x90% -> 12.3deg, wrong key         ~8.7 s
```

These values are engineering spot checks only. The important property is that the
new composed stage has a fixed search budget and does not multiply all rotation
angles by the full affine hypothesis set. During final build-6 regression testing,
the key-independent rotation probe was also tightened after an unmarked synthetic
image generated weak non-authenticating candidates; the dedicated fast-reject test
now passes while the existing arbitrary-rotation and combined-geometry tests remain
passing.

### Current validation boundary

Build 6 does **not** claim balanced/capacity recovery for the composed path,
90%x110% in combination with rotation, the reverse transform order, rotation +
shear, projective/perspective recovery or print-camera recovery. These remain in
`TODO.md` and must not be inferred from the single development baseline above.

Final build-6 source checks in this environment passed `gofmt`, `go vet ./...`,
`go test -count=1 ./cmd/pixseal`, the full uncached `go test -count=1 ./watermark`
package (about 57 s), shell syntax checks, native/cross-platform CLI builds and
`make core-target-check` for Linux/Windows/Android/arm64/iOS/arm64. The aggregate
`go test -count=1 ./...` invocation exceeded the 90-second command window here when
Go ran packages concurrently, even though the same packages pass separately; it
must therefore be rerun on PJ's Linux system rather than marked as passed here.

The private `original pics/` corpus is not present in this environment. PJ should
run the full local suite, including `STRICT=1 make composition-test`, before the
next release candidate.

## v0.2.0 build 5 validation status

Development build: **v0.2.0 build 5**, 7 September 2026.

Build 5 keeps format v3 bit-for-bit unchanged and adds a separate bounded
axis-aligned affine recovery stage. Rotation, combined rotation/75%-resize/crop
and the historical pure-resize paths remain present; arbitrary rotation composed
with anisotropic scale/shear is intentionally not part of this build.

### Deterministic affine-search bounds

The affine hypothesis set is fixed in source:

```text
20 anisotropic X/Y scale matrices
   X,Y in {0.90, 0.95, 1.00, 1.05, 1.10}, excluding X == Y
16 shear matrices
   X or Y shear at +/-3, +/-5, +/-8, +/-10 degrees
36 matrices total
```

All 36 are evaluated through the virtual inverse-affine DCT sampler. Periodic
tile coherence is key-independent and gates/ranks the candidates. At most six
matrices survive this ranking; at most three phases per retained matrix receive
an authenticated single-tile v3 probe (18 probes maximum), and at most four
strong failed-sync candidates receive full-carrier repetition aggregation.
HMAC-authenticated v3 decoding is the only success condition.

This affine budget is separate from the bounded rotation and resize budgets; the
build deliberately does not multiply angle hypotheses by affine matrices.

### ImageMagick affine regression matrix

A temporary generated 900x700 carrier was transformed with ImageMagick and
extracted through the compiled build-5 CLI. The complete default `affine-test`
matrix passed in this environment:

| Profile | 110%x90% | 90%x110% | shear X 8 deg | shear Y 8 deg |
|---|---|---|---|---|
| robust | PASS | PASS | PASS | PASS |
| balanced | PASS | PASS | PASS | PASS |
| capacity | PASS | PASS | PASS | PASS |

Summary: **12 passed, 0 failed, 0 timeouts, 0 skipped, 0 errors**.

The reported corrections matched the tested discrete hypotheses. Example outputs
included `scale-correction: x=0.9091 y=1.1111` for a 110%x90% carrier and an
8-degree shear correction for the corresponding shear cases. These generated
results validate the implementation and shell harness; they are not a guarantee
for arbitrary photographic content or every transform inside the hypothesis
range.

### Affine timing spot checks

A freshly embedded robust 900x700 generated carrier transformed to 110%x90% was
recovered in approximately **2.21 s** in this environment. The same transformed
carrier with an incorrect key exhausted the bounded search in approximately
**10.46 s**. These are machine-specific engineering spot checks, not performance
guarantees.

The important structural property is that the affine stage is finite and uses
coherence gating plus capped authenticated probes rather than a full
angle x scale x shear brute force.

### Validation status in this environment

The following checks were executed successfully during final build-5 closure:

```text
gofmt check
go vet ./...
go test -count=1 ./cmd/pixseal
bash -n scripts/test-robustness.sh
bash -n scripts/test-limits.sh
bash -n scripts/test-geometry.sh
bash -n scripts/test-affine.sh
make clean
make
make build-all
STRICT=1 make affine-test        (temporary generated 900x700 corpus; 12/12 PASS)
```

A full **uncached** `go test -count=1 ./watermark` was also attempted, but the
package exceeded the command window of this environment after the geometry/affine
suite grew. The aggregate `go test ./...` likewise cannot be claimed as an
uncached final pass here. Cached package results were successful, but they are not
used as the final validation claim. Both commands must therefore be rerun on
PJ's Linux system.

The private `original pics/` corpus is absent here, so `make test`, `make all`,
`make extreme-test`, `make geometry-test` and `make affine-test` are **not**
reported as private-corpus passes. The generated affine corpus above exists only
for development validation and is not included in the source archive.

Recommended build-5 validation on PJ's system:

```sh
make clean
gofmt -w cmd internal watermark
go vet ./...
go test ./...
make test
make deep-test
make extreme-test
STRICT=1 make geometry-test
STRICT=1 make affine-test
make all
```

### Current geometry boundary

Build 5 claims an **experimental bounded axis-aligned affine baseline** in
addition to the build-4 rotation/75%-resize/crop baseline. It does not claim
arbitrary rotation composed with anisotropic scale/shear, projective/perspective
recovery or print-camera recovery. Those remain staged research targets.

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


## v0.2.0 build 10 regression recovery

Build 10 was driven by the build-9 `all-test` report rather than by a new geometry feature. On the private 800x757 LQ carrier, robust and balanced recover 95/85/75/65/55/50% pure resize again; capacity recovers through 55%. Capacity at 50% remains a known carrier/profile limit also reproduced with build 5.

The private 16320x12288 (~201 MP) HQ PNG is above the 50 MP geometry-suite limit. Geometry, affine, composition and lattice scripts now identify its dimensions through PNG IHDR fallback and report policy SKIPs rather than dimension-read errors. The image itself is not distributed.

These observations are experimental results on the private corpus, not recovery guarantees.


## v0.2.0 build 11 perspective experiment

Using ImageMagick-generated fixed-canvas perspective warps on the private real-image corpus, the build-11 two-hypothesis vertical-keystone decoder recovered 4/4 tested robust-profile cases across the LQ and MQ carriers (top edge narrowed by 4%, bottom edge narrowed by 4%). The ~201 MP HQ carrier was intentionally skipped by the 50 MP geometry-suite limit. These results are experimental and do not imply general print-camera or arbitrary-perspective robustness.
## v0.2.0-rc1 validation status

The RC contains the build-11 codec/decoder behavior with release-facing consolidation only. Format v3 and deterministic encoder fingerprints are frozen. The release baseline is defined as static analysis, authenticated unit/image round trips, baseline JPEG/resize/crop robustness and reusable-core portability. Advanced geometry suites remain strict research measurements and may expose corpus-specific limits without being represented as universal release guarantees.

The last pre-RC PJ-corpus report for build 11 recorded a fully passing baseline (`test`, 72/72 `deep-test`, `composition-test`, `lattice-test`, `perspective-test`, and core portability) while arbitrary geometry and affine suites retained known corpus-specific failures. RC-specific corpus validation remains to be run on PJ's Linux system; results must not be inferred from the build-11 report merely because the algorithm is unchanged.

