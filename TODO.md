# PixSeal TODO

This file contains open work only. Completed milestones belong in `HISTORY.md`
and `CHANGELOG.md`.

## v0.3.0 — geometry research

- [x] Build a bounded diagnostic direct/local lattice estimator separated from
      `ExtractWithInfo`.
- [x] Estimate local transformed lattice vectors `u` and `v` and combine multiple
      regions without allowing one region to set the global geometry.
- [x] Add a coarse print-boundary initializer that is never treated as watermark
      evidence.
- [x] Produce a bounded projective scale shortlist and explicit homographies.
- [x] Add a virtual projective DCT sampler and synthetic authenticated end-to-end
      regression, capped at four complete v3 decode attempts.
- [x] Add bounded spatial phase-consensus ranking and ±1%/±2% scale refinement
      without photo-specific target dimensions.
- [x] Add explicit fundamental-period/scale selection using multilevel and
      multiregion support before phase refinement, without photo-specific target
      dimensions.
- [x] Fit a bounded deterministic DLT homography from four local sync-phase
      correspondences when available.
- [x] Add bounded modulo-8 block-origin/sub-pixel refinement with monotonic
      full-probe verification.
- [x] Add a small deterministic post-geometry photometric bank (`raw`,
      `local-normalize`, `mild-highpass`) with bounded probes and unchanged HMAC
      semantics; build5 shows a modest frontal sync improvement but no HMAC.
- [x] Add a bounded adaptive finer lattice level for large no-boundary borderline
      cases and allow a lattice-positive weak-boundary seed to reach the projective
      decoder without making the physical boundary a detection gate.
- [x] Quantify the protected-bit channel on the already-bounded full-decode
      candidates using the six exactly-known v3 header Hamming words, post-ECC
      known-header errors, syndrome occupancy and DCT reliability margins.
- [x] Test a bounded reliability-aware Hamming decoder on the existing Format v3
      code without increasing the four full-decode/HMAC-slot ceiling. The synthetic
      weak-double-error case authenticates, but no real acquisition does; the simple
      per-word soft model is therefore not promoted to production extraction.
- [x] Use spatial/tile agreement to determine whether remaining errors are local
      or systematic. Build9 finds key-independent tile-sign agreement only about
      0.64--0.68 and almost all known prefix bits mixed across spatial cells.
- [x] Fit/test a **smooth low-DOF spatial phase or residual-warp field** using the
      build9 cell evidence. Build10 uses a six-parameter affine field with separate
      diagnostic/decode gates; one real candidate is promoted but no HMAC succeeds.
- [x] Improve spatial control estimation without increasing independent per-cell
      freedom. Build11 adds fractional phase-surface interpolation, explicit
      confidence, search-boundary penalties and robust weighted fitting.
- [x] Evaluate a regularized low-order extension under the same bounded four-HMAC-slot
      rule. Build11 compares a ridge-regularized quadratic field by leave-one-out; it
      wins one real model comparison but fails the bounded correction gate and is not
      promoted.
- [x] Derive continuous local-phase controls from key-independent structural evidence.
      Build12 uses robust/balanced intra-tile repetition coherence, with pairwise
      cross-cell correlation as a bounded fallback; the real smooth-field path no
      longer uses key-assisted oracle controls.
- [x] Test a second key-independent image-domain observer for **cycle-slip resolution**.
      Build13 uses repetition-guided cross-cell correlation in a bounded +/-1 window;
      real mean pair peaks remain only about 0.053--0.065 versus about 0.61 synthetic,
      so the observer is kept as diagnostic-only and applies no real cycle-slip fixes.
- [x] Add a genuinely complementary **local lattice fractional-phase** observer.
      Build14 projects the existing key-independent 3x3 lattice estimates into the
      candidate canonical plane, measures modulo-8 phase and tests a bounded +/-1
      smooth unwrap. It produces several strong same-candidate diagnostics but also
      degradations; no real field passes all HMAC-promotion gates.
- [x] Add a first **global discrete integer phase unwrap** without using the known header.
      Build15 introduced the axis-separable bounded solver and transactional rollback.
- [x] Replace build15 beam ranking with an **exact bounded top-2 certificate**.
      Build16 enumerates at most 3^9 assignments per axis, closes a deterministic beam
      false-accept regression, reports X/Y status separately and keeps no-eligible JSON finite.
- [x] Add a first **split-repetition top-1/top-2 consistency diagnostic**.
      Build16 partitions key-independent repetition pairs deterministically into two folds.
      It remains diagnostic-only because the primary repetition controls used the full pair set.
- [ ] Build a genuinely **held-out integer-cycle discriminator** that can resolve, not merely
      reject, real ambiguous top-2 fields. The fit/proposal evidence and validation evidence
      must be disjoint by construction (for example fit-set vs held-out repetition groups),
      and the fixed rule must generalize across all six physical acquisitions before promotion.
- [ ] Test whether a jointly regularized spatial model can improve several acquisitions
      from blind controls without per-cell oracle freedom; do not increase HMAC slots or
      model order unless cross-validation improves out-of-sample controls.
- [ ] Reduce negative-case cost of the mature arbitrary-geometry research paths;
      `geometry-test` remains the dominant `all-test` runtime.
- [ ] Decide whether/when the diagnostic projective path has enough evidence to
      be promoted into `ExtractWithInfo`; do not integrate before the complete
      synthetic/real/negative/regression promotion sequence.

## v0.3.0 — print-camera research

- [x] Remove the private physical-corpus key from source defaults/documentation and ignore both private acquisition directories.
- [x] Add a Git-ignored local SHA-256 corpus manifest target and canonical `.jpg` smartphone names.
- [ ] Re-run build16 on the complete six-image physical corpus; the current build16 session had only four acquisitions available (the two bicycle photographs were absent).

- [x] Provide a private full-resolution smartphone regression target that never
      ships the corpus, SKIPs when absent and PASSes only on valid v3 HMAC.
- [x] Run the bounded lattice/boundary/projective diagnostics on both original
      real smartphone captures; build2 obtains lattice evidence on both, build3 adds
      phase-aware refinement and build4 separates the fundamental scale from
      supported aliases/harmonics.
- [ ] **IN PROGRESS:** Recover a valid Format-v3 HMAC from the frontal photograph.
- [ ] Recover a valid Format-v3 HMAC from the inclined photograph.
- [ ] Quantify signal survival more strongly than key-known header scores; those
      scores are affected by bounded multiple testing and are not detection.
- [x] Add known-header protected-bit/ECC diagnostics without using the hidden
      payload as an oracle.
- [x] Add a small deterministic photometric-normalization bank only after
      geometry/phase refinement shows it is needed.
- [ ] Add controlled synthetic camera-channel stages: perspective, non-integer
      resampling, blur, gamma/illumination variation, JPEG recompression and noise.
- [x] Exercise a frozen build on a new held-out print-camera pair; build6 recovers
      the projective path on the previously boundary-blocked frontal case without
      photo-specific tuning.
- [x] Make the private print-camera harness automatically test every PNG/JPEG in
      the corpus rather than hardcoding the first two filenames.
- [x] Generalize on an additional held-out smartphone pair and add two scanner
      acquisitions of the same printed-image set as a distinct physical channel.
- [x] Bound diagnostic authentication on scanner-sized inputs; build8 reduces both
      supplied 34.8 MP scans from >120 s to about 10--11 s without changing the
      production extractor.
- [ ] Add more independently printed/acquired material before changing Format v3.

## Future platform work

- [ ] Run the no-clobber CLI regressions on a real Windows host when convenient;
      v0.2.0 qualification already includes Windows/amd64 cross-compilation.
- [ ] Decide explicit EXIF Orientation normalization policy.
- [ ] Decide ICC/color-management preservation policy.
- [ ] Consider tiled/lazy pixel access to reduce peak memory on very large images.
