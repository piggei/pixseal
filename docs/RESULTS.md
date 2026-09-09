# PixSeal v0.2.0 — qualification and measured results

This document separates the **stable release baseline** from experimental
geometry measurements. Results are corpus-specific observations, not universal
recovery guarantees.

## v0.2.0 release qualification reference

The qualified `v0.2.0-rc1` run on PJ's Linux/WSL2 corpus reported:

```text
Release baseline: PASS
Research suites: ATTENTION (2 experimental targets failed)
Overall: FAIL (2 targets failed)
```

Release baseline:

```text
vet                  PASS
test                 PASS
deep-test             72/72 PASS
core-target-check     PASS
```

Research measurements:

```text
extreme-test          completed progressive limit mapping
geometry-test         65/120 PASS, 55 FAIL
affine-test           22/24 PASS, 2 FAIL
composition-test      2/2 PASS
lattice-test          8/8 PASS
perspective-test      4/4 PASS
```

The research FAIL rows are intentionally visible. They do not imply corruption
or authentication failures on the stable JPEG/resize/crop baseline; they expose
image-dependent limits in experimental geometry recovery.

## Baseline digital robustness

On the release qualification corpus, `deep-test` recovered all 72 tested cases
across two real images and all three profiles. The suite covered:

- JPEG quality 82;
- resize 95, 85, 75, 65, 55 and 50%;
- center crop 90, 75 and 50%;
- deterministic random crop 75 and 50%.

`extreme-test` is intentionally non-strict. It searches progressively for the
failure envelope and reports individual FAIL rows as measurements.

## Adaptive profiles

Deterministic properties:

| Profile | Max payload | Protected bits | Tile redundancy |
|---|---:|---:|---:|
| robust | 16 B | 448 | 2.50x |
| balanced | 32 B | 672 | 1.67x |
| capacity | 64 B | 1120 | 1.00x |

The same 1120-position tile is used for every profile. Shorter protected frames
receive more repeated observations.

## Resize recovery

The v0.2.0 decoder's stable fractional-resize path is virtual and bounded:

```text
13 fixed non-direct scale hypotheses
<= 3 phases per scale
<= 6 full-grid shortlisted scales
<= 18 full-carrier virtual aggregations
```

A decisive scale may receive one physical bilinear fallback over the expected
inverse size and its eight ±1-pixel neighbours. This architecture restored the
fractional-resize regression found during build 9 without changing Format v3.

## Experimental affine measurements

The fixed axis-aligned affine bank contains 36 hypotheses. In the RC1 private
corpus run, 22 of 24 configured cases authenticated successfully. The two
failures were corpus/profile-specific shear cases. This remains research
behavior rather than a release guarantee.

## Experimental lattice composition

The frozen bank contains:

```text
110%x90%
90%x110%
105%x95%
95%x105%
```

at -45..+45 degrees in quarter-degree steps.

Implemented bounds:

```text
1444 sparse probes maximum
192 stronger coherence evaluations maximum
12 full authenticated grids maximum
```

The RC1 qualification run recovered 8/8 configured robust-profile lattice cases.

## Experimental perspective

The frozen v0.2.0 perspective bank contains only:

```text
top-narrow-4
bottom-narrow-4
```

Each is one full projective grid at phase `(0,0)`, for two maximum projective
aggregations. The RC1 private-corpus shell run recovered 4/4 configured cases.

RC3 adds a deterministic Go end-to-end perspective regression that also asserts
that the reported `PerspectiveCorrection` matches the expected projective path.
This closes the earlier gap where only the hypothesis count was unit-tested.

General homography inference and physical print-camera recovery are not claimed.

## v0.2.0-rc3 hardening verification

RC3 keeps the v3 encoder layout and golden fingerprints unchanged. It adds
pre-release hardening verified locally in the packaging environment:

- non-finite and out-of-contract CLI strength rejection;
- directory/symlink output rejection and no-clobber race regression;
- Unix umask regression for newly-created outputs;
- `.sealed -> .sealed.png` naming regression;
- exact multiline extraction via `extract -raw`;
- 250,000,000-pixel working-image preflight regression;
- alpha-detail estimator consistency regression;
- real projective end-to-end Go regression with path assertion;
- JPEG perspective shell test and zero-corpus failure behavior;
- VERSION/buildinfo consistency gate;
- `gofmt`, `go vet`, native build and shell syntax checks.

A full private-corpus `make all-test` remains the final external qualification
step before promoting RC3 to `v0.2.0` final. Do not infer that unexecuted corpus
suites passed merely from the local hardening checks.

## Large images

The v0.2 line has previously round-tripped a private ~200 MP carrier. RC3 keeps
that class within the new 250,000,000-pixel working-image safety limit. Geometry
suites may independently skip transforms above their configured 50/100 MP
research limits.

## Interpretation

- **Deterministic:** format sizes, profile capacities, search-space caps and
  authentication rules.
- **Experimental:** success rates on particular images/transforms.
- **Heuristic:** `analyze` detail score and recommended strength.

No experimental PASS guarantees recovery on an arbitrary future image, and no
failed experimental geometry case invalidates the authenticated digital
baseline.
