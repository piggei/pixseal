# PixSeal engineering history

This file records the technical evolution of PixSeal, including experiments
that were not promoted. It is chronological. Current behavior is specified by
`README.md` and `docs/ALGORITHM.md`.

## v0.1.0 — first public baseline

The first stable line established the classical DCT steganography core, keyed
whitening/authentication, Hamming protection, crop/resize recovery and the local
robustness test workflow. Capacity was fixed at 64 bytes.

## v0.2.0 build 1 — adaptive Format v3

Introduced Format v3 and adaptive profiles:

- `robust` 16 B;
- `balanced` 32 B;
- `capacity` 64 B;
- `auto` selects the most redundant compatible profile.

Shorter fixed frames reuse spare tile positions as repeated observations.

## v0.2.0 build 2 — v3-only runtime

Removed runtime fallback to the unreleased v1/v2 engineering formats. They had
no external installed base, so compatibility was retained only as project
history rather than production complexity.

## v0.2.0 build 3 — arbitrary digital rotation

Added bounded rotation estimation plus lossless 90/180/270-degree paths. The
Format v3 carrier remained unchanged.

## v0.2.0 build 4 — rotation with resize/crop

Extended geometry experiments to selected rotation+resize/crop combinations.
The work exposed scale/phase coupling and established explicit search budgets as
a project requirement.

## v0.2.0 build 5 — bounded axis-aligned affine recovery

Added virtual affine sampling for anisotropic scale and single-axis shear. A
periodic-tile coherence signal was used to rank hypotheses before HMAC.

## v0.2.0 build 6 — first composed affine experiment

Demonstrated a bounded anisotropic-scale + rotation composition on synthetic
cases without a full angle×affine Cartesian product. Real-corpus testing later
showed the rotation-first heuristic did not generalize reliably.

## v0.2.0 build 7 — direct lattice scoring after real-corpus failure

Two real images that produced FAIL/TIMEOUT in build 6 became private regression
cases. Direct transformed-lattice scoring replaced the rotation-first
composition heuristic for the promoted path and recovered both cases.

## v0.2.0 build 8 — symmetric direct lattice-basis bank

Generalized the direct basis representation to both 110×90 and 90×110
anisotropies before rotation. This shifted the design from edit names toward
explicit transformed lattice bases.

## v0.2.0 build 9 — broader lattice bank and comparative test reporting

Added the 105×95 and 95×105 basis shapes and introduced `make all-test` for a
single sequential comparative report.

A more continuous/local `u`,`v` refinement prototype recovered positive cases
but made negative extraction too expensive. It was deliberately **not promoted**.
This is an important negative result: more hypotheses are not a substitute for a
better estimator.

## v0.2.0 build 10 — resize-regression recovery

The first `all-test` reports exposed a regression where false advanced-geometry
candidates could suppress fractional pure-resize recovery. Build 10 restored
pure-resize priority with virtual isotropic sampling and a single targeted
bilinear normalization fallback.

It also added deterministic Format-v3 encoder fingerprints and robust large-PNG
dimension probing in shell tests.

## v0.2.0 build 11 — first bounded projective step

Added a virtual homography sampler with exactly two vertical-keystone
hypotheses: `top-narrow-4` and `bottom-narrow-4`. This was intentionally a small
projective experiment, not general perspective inference.

`perspective-test` was added and passed the initial private digital corpus.

## v0.2.0 build 12 — local-consensus experiment, not promoted

A local orientation-consensus fallback was prototyped after build 11. The
materialized build did not improve the real rotation matrix and increased total
runtime. It was therefore rejected as a production baseline.

This result reinforced the v0.3 direction: geometry estimation should be tested
as a separate measurable component before another heuristic is inserted into
`Extract()`.

## v0.2.0-rc1 — release consolidation

The release candidate returned to the qualified build-11 algorithm and separated
release-baseline status from experimental research-suite status.

RC1 qualification on the private Linux corpus recorded:

```text
Release baseline: PASS
deep-test:         72/72
geometry-test:     65/120 experimental
 affine-test:      22/24 experimental
composition-test:   2/2
lattice-test:       8/8
perspective-test:   4/4
```

Format v3 and the encoder were frozen.

## Pre-release v0.2.0 package audit

Before publishing the nominal final archive, an independent audit was performed
and then re-verified against the actual source package. The algorithmic base was
sound, but the audit found release-hardening issues unrelated to the on-image
format, including:

- `NaN` strength false-success;
- ambiguous explicit `-strength 0` CLI behavior;
- unsafe output replacement edge cases involving directories/symlinks and
  no-force TOCTOU;
- forced 0644 output permissions;
- perspective harness false greens/errors;
- missing projective end-to-end Go regression;
- large-image pre-allocation policy gap;
- alpha mismatch in `analyze`;
- Windows amd64 build naming mismatch;
- substantial current-documentation drift from the real decoder.

Additional verification found that the PNG header fallback trusted a `.png`
extension without validating the PNG signature and that the documented lattice
strong-evaluation budget was too high relative to the actual code.

Because the package had not yet been publicly released, the correct response was
an RC2 rather than a v0.2.1.

## v0.2.0-rc2 — release hardening

RC2 keeps Format v3 and the encoder mapping unchanged while hardening the
surrounding product:

- rejects non-finite CLI/API strengths and explicit CLI zero;
- adds pre-decode/source size guards;
- makes new output files private and no-force commit no-clobber;
- refuses forced replacement of non-regular targets;
- fixes dotfile output naming;
- adds `extract -raw` for exact multiline-safe payload output;
- aligns analyzer alpha compositing with embedding;
- forces Windows amd64 in the Windows helper;
- repairs and strengthens the perspective harness;
- adds a true Go end-to-end projective regression;
- adds VERSION/buildinfo consistency checking;
- removes dead bicubic normalization code;
- rewrites current documentation around the actual v0.2 decoder and search
  bounds.

RC2 must be qualified on the private corpus before the final v0.2.0 tag is
created.

## Direction after v0.2.0

The planned v0.3 research line focuses on continuous/local lattice inference,
general bounded homography estimation, print-camera recovery, and sparse/tiled
large-image processing. Those are research goals, not v0.2 promises.
