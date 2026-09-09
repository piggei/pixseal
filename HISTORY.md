# PixSeal history

This file records technical evolution, including experiments that were later
superseded or deliberately not promoted.

## v0.1.0

First stable public-development baseline: classical DCT embedding for short
messages, bounded extraction, JPEG/resize/crop robustness harnesses and pure-Go
CLI/core architecture.

## v0.2.0 build 1 — adaptive Format v3

Introduced `robust`, `balanced`, `capacity` and `auto`, fixed 35x32 tile,
authenticated v3 headers, adaptive redundancy and the `analyze` command.

## build 2 — v3-only runtime

Removed runtime v1/v2 decoding because those experimental formats had no known
external compatibility population. Their history remained documented.

## build 3 — arbitrary digital rotation

Added bounded quarter-turn and arbitrary-angle recovery without changing the
v3 on-image format.

## build 4 — combined digital geometry

Extended rotation to selected resize/crop combinations and added
`geometry-test`. Aggressive rotation+50% resize remained a documented signal
limit rather than a claimed feature.

## build 5 — axis-aligned affine recovery

Added a fixed 36-hypothesis anisotropic-scale/shear bank with virtual affine
sampling and bounded full-grid promotion. Added `affine-test`.

## build 6 — first composed affine experiment

Demonstrated one bounded anisotropic-scale+rotation composition on a synthetic
case, but the rotation-first heuristic did not generalize to PJ's real corpus.
The failure was retained as a research result.

## build 7 — direct composed-lattice recovery

Reworked the composed case so geometry was scored directly rather than inherited
from a global rotation peak. The two real regression carriers that had produced
FAIL/TIMEOUT in build 6 both authenticated successfully.

## build 8 — explicit lattice basis bank

Generalized the direct bank to both 110x90 and 90x110 anisotropies under
rotation.

## build 9 — broader fixed lattice bank and unified reports

Added 105x95 and 95x105 shapes, giving four fixed direct lattice bases. Added
`make all-test` for sequential comparative reporting. A more continuous local
`u`,`v` refinement prototype improved positive cases but made negative extraction
too expensive and was deliberately not promoted.

## build 10 — resize regression recovery

The build-9 report exposed fractional-resize regressions caused by speculative
geometry suppressing the mature resize path. Build 10 restored ordering with
virtual isotropic-scale recovery and a targeted bilinear fallback. It also fixed
large-PNG dimension probing in shell suites.

## build 11 — first projective step

Added exactly two fixed 4% vertical-keystone homographies using virtual
projective sampling. ImageMagick-generated regression cases authenticated on the
private LQ/MQ corpus. This was recorded as a stepping stone, not general
perspective support.

## build 12 — local-consensus experiment not promoted

A reconstructed local orientation-consensus fallback failed to improve the
private geometry matrix and increased runtime. Build 11 therefore remained the
algorithmic basis for release consolidation.

## v0.2.0-rc1 — first release candidate

Consolidated build 11, froze the Format v3 deterministic encoder and prepared
the v0.2 line for release hardening.

## External audit 1

An independent source/package audit found no reason to change Format v3 but
identified CLI validation, output-path safety/umask, perspective-harness,
large-image-preflight, regression-coverage and documentation issues.

## v0.2.0-rc2 — first audit reconciliation

RC2 implemented those fixes while keeping the on-image format and encoder
fingerprints unchanged. It also separated stable `release-unit` tests from
experimental `research-unit` regressions.

PJ's 2026-09-09 Linux/WSL2 qualification report recorded:

```text
Release baseline: PASS
deep-test:        72/72 PASS
geometry-test:    65/120 PASS, 55 FAIL (experimental)
affine-test:      22/24 PASS, 2 FAIL (experimental)
composition-test: 2/2 PASS
lattice-test:     8/8 PASS
perspective-test: 4/4 PASS
```

## External audit 2

The RC2 audit confirmed the core and most first-round fixes, then identified a
Windows no-clobber assumption, non-strict release-gate semantics, zero-case
qualification, help exit status, capacity full decoding and additional hardening
items.

## v0.2.0-rc3 — second hardening candidate

RC3 introduced a stronger cross-platform no-clobber design using hard links with
an exclusive-create copy fallback, overflow-safe image-size arithmetic, shared
alpha flattening and a stronger projective synthetic regression. During package
assembly, however, several valid RC2 release-engineering/script changes were
reverted and the RC2 qualification history was misattributed.

## External audit 3

The RC3 audit found no Format v3, encoder or decoder regression. It identified
the release-engineering reversions, unresolved strict/zero-case/help/capacity
items, the raw stdout/stderr regression and an incorrect direct-lattice budget.

## v0.2.0-rc4 — reconciliation candidate

RC4 is deliberately non-algorithmic: it keeps RC3 filesystem/overflow/alpha
hardening, restores the valid RC2 release/test-harness changes, fixes the
remaining CLI/release-gate issues, restores the 300 MP v0.2 source policy and
repairs release chronology/documentation. Format v3 and the frozen search banks
remain unchanged.
