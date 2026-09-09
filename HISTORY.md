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

## v0.2.0-rc1 — release qualification candidate

Consolidated build 11, froze the Format v3 encoder and separated stable release
gates from research-suite reporting. PJ's qualification run reported release
baseline PASS, deep-test 72/72, composition 2/2, lattice 8/8 and perspective
4/4; arbitrary geometry and affine suites retained documented experimental
limits.

## External pre-release audit

An independent source/package audit found no reason to change Format v3, but it
identified several release-hardening issues: non-finite strength acceptance,
output-path safety and umask handling, perspective harness false-green/error
accounting, missing projective end-to-end coverage, large-image preflight gaps,
dead code and substantial documentation drift from intermediate builds.

## v0.2.0-rc3 — audit hardening candidate

RC3 keeps the on-image Format v3 and encoder fingerprints unchanged while
hardening the release surface:

- finite CLI/API strength validation and explicit CLI range contract;
- safe regular-file-only replacement, symlink/directory rejection and
  race-resistant no-clobber publication;
- umask-respecting new outputs, synced temporary files and Windows-only backup
  replacement fallback;
- Unix dotfile output naming;
- `extract -raw` for exact/multiline payload scripting;
- pre-decode/working-image safety limit at 250 million pixels;
- alpha-consistent analysis;
- robust perspective shell harness and deterministic projective Go E2E test;
- VERSION/buildinfo gate;
- removal of obsolete bicubic/dead structures;
- current-code rewrite of README/ALGORITHM/RESULTS/TODO.

The final `v0.2.0` promotion depends on PJ's RC3 release qualification report.
