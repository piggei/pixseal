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

## RC4 local hardening verification

RC4 is a release-engineering reconciliation build, not an algorithm revision.
Local package verification confirms:

- Format v3 golden regression remains unchanged;
- release-scoped Go tests pass;
- source-size arithmetic includes a true multiplication-overflow regression;
- subcommand `-help` exits successfully;
- `capacity` can operate from `DecodeConfig` without full bitmap decoding;
- strict robustness qualification rejects a run with zero executed baseline
  transformations;
- partial `ALL_TEST_TARGETS` runs report `PARTIAL` / `NOT RUN`;
- `extract -raw` keeps exact payload bytes on stdout and diagnostics on stderr;
- invalid `PERSPECTIVE_MAX_MPIX` is rejected cleanly;
- reusable core cross-target checks remain part of the release gate.

A full RC4 private-corpus `make all-test` is still required before promotion to
`v0.2.0` final. The acceptance reference is the RC2 report above; RC4 must not
regress the baseline or experimental counts.

## Large images

The v0.2 line has round-tripped a private ~200 MP carrier. RC4 retains the
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
