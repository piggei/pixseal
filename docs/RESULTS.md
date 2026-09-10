# PixSeal v0.2.0 — qualification and measured results

This document separates the **stable release baseline** from experimental
geometry measurements. Results are corpus-specific observations, not universal
recovery guarantees.

## RC2 qualification reference — 2026-09-09

The preserved `v0.2.0-rc2` all-test report was produced on Linux/WSL2 with Go
1.25.1 and ImageMagick, using strict mode for pass/fail baseline and geometry
suites. It reported:

```text
Release baseline: PASS
Research suites: ATTENTION (2 experimental targets failed)
Overall: FAIL (2 experimental targets failed)
```

Release baseline:

```text
version-check          PASS
vet                    PASS
release-unit           PASS
test-images            PASS
deep-test              72/72 PASS
core-target-check      PASS
```

Research measurements:

```text
extreme-test           completed progressive limit mapping
research-unit          PASS
geometry-test          65/120 PASS, 55 FAIL
affine-test            22/24 PASS, 2 FAIL
composition-test       2/2 PASS
lattice-test           8/8 PASS
perspective-test       4/4 PASS
```

The geometry/affine FAIL rows are intentionally visible measurements of the
experimental search bank. They are not failures of the stable authenticated
JPEG/resize/crop release baseline.

## Baseline digital robustness

On the RC2 qualification corpus, `deep-test` recovered all 72 tested cases
across two real images and all three profiles. The suite covered JPEG quality 82,
resize 95/85/75/65/55/50%, center crop 90/75/50% and deterministic random crop
75/50%. `extreme-test` is intentionally non-strict and maps the progressive
failure envelope.

## Adaptive profiles

| Profile | Max payload | Protected bits | Tile redundancy |
|---|---:|---:|---:|
| robust | 16 B | 448 | 2.50x |
| balanced | 32 B | 672 | 1.67x |
| capacity | 64 B | 1120 | 1.00x |

## Resize recovery budget

```text
13 fixed non-direct scale hypotheses
<= 3 phases per scale
<= 6 full-grid shortlisted scales
<= 18 full-carrier virtual aggregations
```

A decisive scale may receive one physical bilinear fallback over the expected
inverse size and its eight ±1-pixel neighbours.

## Experimental lattice composition budget

The frozen bank contains 110x90, 90x110, 105x95 and 95x105 basis shapes over
-45..+45 degrees at 0.25-degree steps. The search is evaluated in two sequential
shape groups (±10% anchors, then ±5% moderates):

```text
1444 sparse probes maximum
192 stronger coherence evaluations maximum
12 full authenticated grids maximum per group
24 full authenticated grids maximum overall
```

The RC2 qualification recovered 8/8 configured robust-profile lattice cases.

## Experimental perspective

The frozen v0.2 bank contains only `top-narrow-4` and `bottom-narrow-4`, one
aligned full projective aggregation each. RC2 recovered 4/4 configured cases.
RC2 already contained an end-to-end Go recovery regression; RC3 strengthened the
synthetic warp implementation while retaining explicit correction-path assertion.
General homography inference and physical print-camera recovery are not claimed.

## Final v0.2.0 qualification — 2026-09-09

The RC4 source candidate was exercised on the same private two-image corpus used
for the RC2 reference. The complete all-test report recorded:

```text
version-check          PASS
vet                    PASS
release-unit           PASS
test-images            PASS
deep-test              72/72 PASS
extreme-test           completed progressive limit mapping
research-unit          PASS
geometry-test          63 PASS, 55 FAIL, 2 TIMEOUT (60-second run)
affine-test            22/24 PASS, 2 FAIL
composition-test       2/2 PASS
lattice-test           8/8 PASS
perspective-test       4/4 PASS
core-target-check      PASS

Release baseline: PASS
Research suites: ATTENTION
```

The two RC4 geometry timeouts were both `PJ_lingua.PNG`, profile `capacity`, at
-10 and -5 degrees. A targeted rerun with:

```text
STRICT=1 EXTRACT_TIMEOUT=120 make geometry-test
```

recovered both cases and produced:

```text
65 passed, 55 failed, 0 timeouts, 0 skipped, 0 errors, 120 total
```

This exactly reproduces the RC2 experimental geometry reference. The timeout
difference is therefore treated as a wall-clock harness sensitivity, not a
decoder capability regression. The final source keeps the same decoder search
banks and uses a 120-second default only for the geometry research harness.

Final release conclusions:

- stable release baseline fully qualified;
- Format v3 golden fingerprints unchanged;
- no encoder or decoder-search functional change after RC4;
- experimental affine/composition/lattice/perspective counts match the RC2
  reference;
- geometry capability matches the RC2 65/120 reference when given the validated
  120-second research timeout;
- reusable core portability checks pass for linux/amd64, windows/amd64,
  android/arm64 and ios/arm64.

## Large images

The v0.2 line has round-tripped a private ~200 MP carrier. v0.2.0 retains the
300,000,000-pixel CLI/core source policy from RC2 while keeping RC3's
overflow-safe arithmetic. Geometry suites independently use lower research
limits (typically 50/100 MP). The 300 MP guard is a safety policy, not a promise
that every machine has sufficient RAM for every image below it.

## Interpretation

- **Deterministic:** format sizes, profile capacities, search-space caps and
  authentication rules.
- **Experimental:** success rates on particular images/transforms.
- **Heuristic:** `analyze` detail score and recommended strength.

No experimental PASS guarantees recovery on an arbitrary future image, and no
failed experimental geometry case invalidates the authenticated digital baseline.

## v0.3.0-build1 research checkpoint — 2026-09-09

The build1 change is isolated from Format v3, embedding and `ExtractWithInfo`; the
v0.2 qualification numbers above therefore remain the stable comparison
baseline. New diagnostic tests add two explicit properties:

- a deterministic marked synthetic carrier must produce bounded local-lattice
  evidence while its corresponding unmarked source remains negative;
- optional correct-key authentication uses the existing HMAC path, while a wrong
  key cannot turn lattice evidence into an authenticated payload.

Local manual qualification on the private digital corpus is recorded only as a
research observation, not a release guarantee. With robust-profile marked copies
of the two qualification images, the 2026-09-09 build1 run measured:

| Input | Lattice evidence | Global consistency | Global `u` | Global `v` |
|---|---:|---:|---:|---:|
| marked `PJ_piccolo.png` | true | 0.939 | (7.95, -0.02) | (-0.02, 8.03) |
| unmarked `PJ_piccolo.png` | false | 0.414 | (5.50, -5.30) | (4.70, 5.23) |
| marked `PJ_lingua.PNG` | true | 0.981 | (8.07, 0.00) | (0.00, 8.00) |
| unmarked `PJ_lingua.PNG` | false | 0.442 | (6.31, -1.91) | (1.76, 6.50) |

The marked canonical carriers therefore resolve very close to the native 8-pixel
lattice while the corresponding unmarked originals remain below the evidence
gate. Exact values are content dependent and are not frozen as interoperability
requirements.

The real `foto stampa.PNG` / `foto stampa storta.PNG` corpus is intentionally not
distributed. `make print-camera-test` therefore SKIPs when that private corpus is
not installed. When it is installed, only an authenticated Format v3 HMAC can
produce PASS; useful lattice evidence without HMAC is reported as research, not
success.

Known build1 limit: automatic arbitrary-angle local-basis estimation is not yet
reliable enough to fit a general homography. Synthetic probing shows measurable
repetition evidence near the true transformed lattice, but candidate generation
and bounded refinement need further work before the homography milestone.
## v0.3.0-build2 projective research checkpoint — 2026-09-09

Build2 remains isolated from Format v3, embedding and production
`ExtractWithInfo`. Its deterministic additions are tested separately through the
research targets. The synthetic projective test verifies that the virtual
homography sampler can reach a valid existing Format-v3 HMAC with the correct
key and that a wrong key fails.

On the two undistributed original smartphone photographs, the current automatic
diagnostic measured:

| Input | Boundary confidence | Global lattice consistency | Lattice evidence | Real HMAC |
|---|---:|---:|---:|---:|
| `foto stampa.PNG` | 0.728 | 0.760 | true | not recovered |
| `foto stampa storta.PNG` | 0.652 | 0.812 | true | not recovered |

For comparison, build1 measured about 0.637/true on the frontal capture and
0.505/false on the inclined capture. Build2 therefore materially improves the
geometric initializer on the harder perspective case. These values are research
observations on two images, not frozen thresholds or interoperability promises.

The bounded projective path probes at most eight canonical-size candidates and
permits at most four full virtual v3 decodes. On the real captures it reaches
that path without materializing a full rectified image, but no candidate yet
authenticates. Key-known header fractions/z-scores are deliberately not reported
as watermark detection because the bounded candidate search still creates a
multiple-testing effect. The hidden message remains unknown.

The qualified v0.2 release baseline remains the reference: build2 changes only
research diagnostics and does not promote these hypotheses into
`ExtractWithInfo`.


## v0.3.0-build3 phase-refinement checkpoint — 2026-09-10

Build3 keeps the build2 boundary/lattice results and adds spatial phase
consensus plus bounded scale/DLT refinement. On the two private ~200 MP
smartphone captures, the automatic diagnostic still reports global lattice
consistency about 0.760 (frontal) and 0.812 (inclined), with lattice evidence on
both.

A notable cross-capture observation is a closely matching coarse scale family:
about 1668x1250 on the frontal capture and 1653x1254 on the inclined capture.
These values are produced independently by the estimator and are not coded as
target dimensions. The frontal member has phase consistency about 0.716; on the
inclined capture the corresponding family is about 0.575 and a bounded height
refinement near 1653x1241 reaches about 0.615. Other local maxima remain and are
retained rather than silently discarded.

For the frontal family, a four-point phase-DLT refinement raises its sparse
known-header probe from roughly z=3.50 to z=4.45. On the inclined capture, the
bounded refinement bank raises the best sparse probe from the build2 value near
z=3.70 to about z=3.97. These are multiple-testing-affected research scores, not
watermark detection.

Both photographs still exhaust at most four virtual full-grid attempts without
a valid Format-v3 HMAC. `print-camera-test` therefore remains a deliberate
research failure and the hidden message remains unknown.
