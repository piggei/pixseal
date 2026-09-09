# Changelog

## v0.3.0-dev1 — 2026-09-09

First research checkpoint toward print -> paper -> smartphone recovery. Format
v3, the deterministic encoder and the production `ExtractWithInfo` decoder are
unchanged from the qualified v0.2.0 baseline.

### Added

- Separate `watermark.DiagnoseGeometry` API and `pixseal diagnose` CLI command.
- Bounded sampled-luminance diagnostic pyramid for large inputs.
- Local `u/v` basis, phase, DCT differential margin, periodic coherence and
  35x32 v3 tile-repetition measurements.
- Per-region candidate pools with cross-region geometric consensus and explicit
  handling of 90-degree square-lattice basis equivalence.
- Explicit coarse/refined/repetition/phase/sample budgets and per-stage timings
  in JSON output.
- Optional independent baseline HMAC attempt when a key is supplied. Diagnostic
  lattice evidence cannot by itself authenticate a payload.
- `make lattice-estimator-test`.
- `make print-camera-test`, which uses the two private original smartphone
  photographs when present, SKIPs cleanly when absent, and reports PASS only for
  an authenticated Format v3 payload.

### Research status

- Canonical marked synthetic/digital carriers can produce coherent local lattice
  evidence while corresponding unmarked inputs remain negative in the dev1
  regression path.
- Arbitrary-angle automatic local-basis estimation is not yet reliable enough to
  feed homography fitting; this remains the next geometry milestone.
- No claim of real print-camera payload recovery is made by dev1.

## v0.2.0 — 2026-09-09

Final release of the Format v3 line, promoted from RC4 after qualification on
the private two-image corpus. No Format v3, encoder fingerprint or decoder
search-bank change was made after RC4.

### Qualification

- `release-unit`: PASS.
- `test-images`: PASS on both private qualification images and all three explicit
  profiles.
- `deep-test`: 72/72 PASS with zero failures, timeouts, skips or errors.
- `core-target-check`: PASS for linux/amd64, windows/amd64, android/arm64 and
  ios/arm64.
- Experimental reference results: affine 22/24, composition 2/2, lattice 8/8,
  perspective 4/4.
- The first RC4 all-test run produced two 60-second geometry timeouts; a targeted
  rerun with a 120-second geometry extraction timeout recovered both and restored
  the RC2 reference result of 65/120 with zero timeouts.

### Finalization

- Promoted version metadata from `v0.2.0-rc4` to `v0.2.0`.
- Recorded final qualification and retained the release/research separation.
- Set the geometry research harness default extraction timeout to 120 seconds to
  avoid misclassifying known recoverable cases as wall-clock regressions on the
  qualification host.

## v0.2.0-rc4 — 2026-09-09

Release-engineering reconciliation candidate. **Format v3, deterministic encoder
fingerprints and decoder search banks are unchanged.**

### Fixed

- Restored the RC2 `release-unit` / `research-unit` split so experimental Go
  geometry regressions cannot make the stable release baseline red.
- Made `make release-check` self-contained with target-specific `STRICT=1`.
- Made strict baseline qualification fail when every configured transformation
  is skipped and zero baseline transformations are executed.
- Made partial `ALL_TEST_TARGETS` runs report `PARTIAL` / `NOT RUN` instead of a
  misleading full qualification PASS.
- Handle subcommand `flag.ErrHelp` as successful CLI help (exit 0).
- Make `capacity` use `image.DecodeConfig` only; it no longer decodes the full
  bitmap merely to calculate geometry capacity.
- Restored `extract -raw` contract: exact payload bytes on stdout, diagnostics on
  stderr; geometric shell harnesses again capture the two streams separately.
- Restored numeric validation of `PERSPECTIVE_MAX_MPIX`.
- Restored RC2 CLI oversized-config and forced-permission regression coverage and
  added a true 64-bit pixel-count overflow regression.
- Treat post-publication temporary-file removal as best-effort cleanup rather
  than converting a successful no-clobber commit into a false write failure.
- Restore the v0.2 public/core source-size policy to 300,000,000 pixels while
  retaining RC3 overflow-safe arithmetic.
- Correct direct-lattice documentation to a 24 full-grid overall worst case
  (12 per sequential shape group), not 12 globally.
- Restore accurate RC1 → RC2 → RC3 chronology and attribute the qualification
  report to RC2.

### Preserved from RC3

- Cross-platform no-clobber publication with hard-link first and exclusive-create
  copy fallback when hard links are unavailable.
- Regular-file-only forced replacement, umask-safe new files and permission
  preservation on replacement.
- Shared alpha-on-white flattening between encoder and analyzer.
- Overflow-safe source-size checks and strengthened projective E2E regression.

## v0.2.0-rc3 — 2026-09-09

Second hardening candidate. RC3 improved cross-platform output publication,
overflow checking, alpha flattening and the synthetic perspective regression.
A subsequent audit found that assembly of RC3 had also reverted several valid
RC2 release-engineering and shell-harness changes; RC4 reconciles those changes.

## v0.2.0-rc2 — 2026-09-09

Implemented the first independent audit findings without changing Format v3:
finite strength validation, explicit CLI strength contract, dotfile handling,
`extract -raw`, large-image preflight, safer output permissions/no-clobber logic,
perspective harness fixes, an end-to-end projective regression, release/research
Go-test separation and major documentation cleanup.

PJ's RC2 qualification run reported release baseline PASS, `deep-test` 72/72,
composition 2/2, lattice 8/8 and perspective 4/4. Experimental geometry and
affine limits remained visible at 65/120 and 22/24 respectively.

## v0.2.0-rc1 — 2026-09-08

Consolidated the qualified build-11 algorithmic line and froze Format v3 encoder
fingerprints for release hardening.

## Development builds 1–12

See [`HISTORY.md`](HISTORY.md) for the complete technical progression.
