# PixSeal TODO

This file contains open work only. Completed milestones belong in `HISTORY.md`
and `CHANGELOG.md`.

## v0.3.0 — geometry research

- [x] Build a bounded diagnostic direct/local lattice estimator separated from
      `ExtractWithInfo`; dev1 has synthetic and digital marked/unmarked regressions.
- [ ] **IN PROGRESS:** Estimate transformed local lattice vectors `u` and `v` rather than adding
      open-ended angle/scale/shear Cartesian searches. Dev1 exposes `u/v`, phase
      and local consensus; arbitrary-angle promotion still needs improvement.
- [ ] Fit affine/projective geometry from multiple local lattice measurements.
- [ ] Reduce the negative-case cost of arbitrary geometry; `geometry-test`
      currently dominates `all-test` runtime.
- [ ] Generalize projective recovery beyond the two fixed vertical-keystone
      hypotheses.
- [x] Preserve explicit maximum probe/candidate/phase/sample budgets for the dev1
      diagnostic stages and expose them in JSON. Keep this rule for every later
      homography/full-decode stage.

## v0.3.0 — print-camera research

- [ ] **IN PROGRESS:** Provide a private full-resolution smartphone regression target that never
      ships the corpus and cleanly SKIPs when absent. Execute it on the original
      photographs when they are available locally.
- [ ] Determine whether the v3 DCT signal remains statistically measurable after
      print -> paper -> smartphone capture before changing the encoder.
- [ ] Improve arbitrary-angle local-basis candidate generation, then fit a general
      homography and recover through virtual sampling.
- [ ] Add controlled synthetic camera-channel stages: perspective, resize, blur,
      gamma/illumination variation, JPEG recompression and sensor noise.

## Future platform work

- [ ] Run the no-clobber CLI regressions on a real Windows host when convenient;
      v0.2.0 qualification already includes Windows/amd64 cross-compilation.
- [ ] Decide explicit EXIF Orientation normalization policy.
- [ ] Decide ICC/color-management preservation policy.
- [ ] Consider tiled/lazy pixel access to reduce peak memory on very large images.
