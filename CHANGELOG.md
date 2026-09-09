# Changelog

## v0.2.0-rc3 — 2026-09-09

Pre-release hardening candidate. **Format v3 and the deterministic encoder remain
unchanged.**

### Fixed

- Reject non-finite embedding strengths; the CLI now also rejects explicit
  strength 0 and all values outside 4..120.
- Harden output publication: `-force` replaces regular files only; directories,
  symlinks and other special files are rejected.
- Protect no-force output against late target creation with a no-clobber commit.
- Respect Unix umask for new output files and preserve regular-file permissions
  on replacement.
- Sync encoded temporary files before publication and restrict the
  backup-and-restore replacement fallback to Windows.
- Treat extensionless Unix dotfiles correctly (`.sealed -> .sealed.png`).
- Add `extract -raw` for exact payload-only scripting and multiline payloads;
  normal diagnostics are emitted to stderr.
- Add `image.DecodeConfig` preflight and a 250,000,000-pixel working-image safety
  limit before heavy allocations.
- Make `analyze` flatten alpha against white exactly like embedding.
- Fix perspective test tool initialization, JPEG handling, zero-corpus false
  PASS, error accounting and unsupported left/right positive modes.
- Require the perspective shell regression to report the expected projective
  correction path.
- Validate PNG signature and IHDR before using the large-PNG dimension fallback.
- Force Windows batch builds to `GOOS=windows GOARCH=amd64 CGO_ENABLED=0`.
- Correct stale geometry comments and the lattice-basis test diagnostic.
- Remove unused `scaleCandidate` and obsolete bicubic normalization code.
- Add VERSION/buildinfo consistency checking.

### Added tests

- CLI strength rejection for 0, NaN and infinities.
- Directory, dangling-symlink and late-created-output safety regressions.
- Unix umask regression.
- Exact multiline `extract -raw` regression.
- Oversized working-image rejection before pixel-plane allocation.
- Alpha-analysis consistency regression.
- Deterministic end-to-end mild-perspective recovery with path assertion.
- Perspective JPEG and empty-corpus harness verification during RC3 closeout.

### Documentation

- Rewrote the current decoder description from the implementation rather than
  intermediate-build prose.
- Corrected v3 frame/tag placement (`header + actual payload + tag + padding`).
- Corrected resize, lattice and perspective search budgets.
- Clarified strict vs non-strict suite semantics.
- Documented EXIF Orientation, ICC/color-management and Windows crash-durability
  limitations.
- Reduced TODO to open release/v0.3 work; historical milestones remain in
  HISTORY.

## v0.2.0-rc1 — 2026-09-08

- Consolidated the qualified build-11 algorithmic line.
- Froze Format v3 encoder fingerprints.
- Added `release-check` and split release-baseline vs research-suite reporting.
- Qualification: release baseline PASS; deep-test 72/72; composition 2/2;
  lattice 8/8; perspective 4/4; known experimental geometry/affine limits
  remained visible.

## Development builds 1–11

See [`HISTORY.md`](HISTORY.md) for the complete technical progression from
adaptive Format v3 through rotation, affine/lattice recovery, resize regression
recovery and the first bounded projective experiment.
