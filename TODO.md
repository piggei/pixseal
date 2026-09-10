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
- [ ] **IN PROGRESS:** Strengthen fundamental-period/scale disambiguation against
      residual phase-consistent aliases/harmonics.
- [x] Fit a bounded deterministic DLT homography from four local sync-phase
      correspondences when available.
- [ ] **IN PROGRESS:** Add true sub-pixel block-origin refinement and evaluate
      whether residual local warp/lens distortion requires a model beyond one
      global homography.
- [ ] Reduce negative-case cost of the mature arbitrary-geometry research paths;
      `geometry-test` remains the dominant `all-test` runtime.
- [ ] Decide whether/when the diagnostic projective path has enough evidence to
      be promoted into `ExtractWithInfo`; do not integrate before the complete
      synthetic/real/negative/regression promotion sequence.

## v0.3.0 — print-camera research

- [x] Provide a private full-resolution smartphone regression target that never
      ships the corpus, SKIPs when absent and PASSes only on valid v3 HMAC.
- [x] Run the bounded lattice/boundary/projective diagnostics on both original
      real smartphone captures; build2 obtains lattice evidence on both and build3
      adds phase-aware refinement.
- [ ] **IN PROGRESS:** Recover a valid Format-v3 HMAC from the frontal photograph.
- [ ] Recover a valid Format-v3 HMAC from the inclined photograph.
- [ ] Quantify signal survival more strongly than key-known header scores; those
      scores are affected by bounded multiple testing and are not detection.
- [ ] Add a small deterministic photometric-normalization bank only after
      geometry/phase refinement shows it is needed.
- [ ] Add controlled synthetic camera-channel stages: perspective, non-integer
      resampling, blur, gamma/illumination variation, JPEG recompression and noise.
- [ ] Generalize on new print-camera captures not used during development.

## Future platform work

- [ ] Run the no-clobber CLI regressions on a real Windows host when convenient;
      v0.2.0 qualification already includes Windows/amd64 cross-compilation.
- [ ] Decide explicit EXIF Orientation normalization policy.
- [ ] Decide ICC/color-management preservation policy.
- [ ] Consider tiled/lazy pixel access to reduce peak memory on very large images.
